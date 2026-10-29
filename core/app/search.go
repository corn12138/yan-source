package app

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/search/glob"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// 在文件中搜索，移植自 Mayoi（MIT，cmd/mayoi/search.go）：在快速打开索引里的文件中找，几个 goroutine
// 一起读，每 60 毫秒把找到的交回来一批，按路径排好；有未保存修改的文件按编辑器里的文字找。
// 区分大小写、全字匹配、正则，「包含的文件」「排除的文件」用逗号分隔的通配符。
// 替换：一处、一个文件、全部；开着的文件作为一步撤销，没开的直接写回磁盘（保留编码、换行符和权限）。

const (
	maxSearchMatches = 20000
	maxSearchBytes   = 4 << 20
	searchPause      = 150 * time.Millisecond // 停止打字多久后开始搜索
	previewBytes     = 240                    // 匹配前后各显示多少字节
)

type searchMatch struct {
	start, end int    // 文件里的字节偏移
	text       string // 匹配到的文字
	line       int
	pre, hit   string // 这一行在匹配前的部分、匹配在这一行里的部分
	post       string // 这一行在匹配后的部分
}

type searchFile struct {
	path, rel string
	matches   []searchMatch
	// 找的是哪个版本：编辑器里未保存的文字（doc 和它的快照版本），或者磁盘上的文件（disk）。
	// 替换一处时版本对不上就不换，免得按旧位置改了别的字。
	doc    *document
	ver    uint64
	disk   edit.Stamp
	digest [sha256.Size]byte
}

// searchText 是开着且有未保存修改的文档：按编辑器里的文字找。
type searchText struct {
	text string
	doc  *document
	ver  uint64
}

// searchFieldID 是搜索视图里有键盘焦点的地方。
type searchFieldID uint8

const (
	sfQuery searchFieldID = iota
	sfReplace
	sfInclude
	sfExclude
	sfResults
)

type fileSearch struct {
	query, with, include, exclude string
	matchCase, word, regex        bool
	replacing, details            bool
	focus                         searchFieldID

	key       string // 结果是按哪个查询找的
	gen       int
	cancel    *atomic.Bool
	running   bool
	timer     *time.Timer
	re        *regexp.Regexp
	err       string
	files     []*searchFile // 按 rel 排序
	count     int
	stopped   bool // 到了 maxSearchMatches 就停
	sel       int  // 选中的结果行（文件标题和匹配连起来数）
	top       int
	collapsed map[string]bool

	// again holds each file's latest refresh request (searchAgain); edited gathers documents
	// typed into until editTimer finds them again.
	again     map[string]int
	againSeq  int
	edited    map[string]bool
	editTimer *time.Timer
}

func (s *fileSearch) searchKey() string {
	return fmt.Sprintf("%v|%v|%v|%s|%s|%s", s.matchCase, s.word, s.regex, s.include, s.exclude, s.query)
}

// showSearch 显示搜索视图，键盘到查询框；选区在一行之内时用它做查询。
func (e *Editor) showSearch(replace bool) error {
	if e.ws == nil {
		return errors.New("没有打开文件夹")
	}
	s := &e.fs
	if t := e.cur(); t != nil {
		if r := t.doc.buf.Selections().Main(); !r.Empty() {
			if txt := t.doc.buf.Snapshot().String(r.From(), r.To()); !strings.Contains(txt, "\n") && len(txt) < 200 {
				s.query = txt
			}
		}
	}
	s.replacing = s.replacing || replace
	s.focus = sfQuery
	if replace && s.query != "" {
		s.focus = sfReplace
	}
	e.showSide(sideSearch)
	e.searchSoon(0)
	return nil
}

// searchSoon 在 d 之后开始搜索（打字时等一下，免得每个字都搜一遍）。查询没变就不搜。
func (e *Editor) searchSoon(d time.Duration) {
	s := &e.fs
	if s.searchKey() == s.key {
		return
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	if d == 0 {
		e.startSearch()
		return
	}
	key := s.searchKey()
	post := e.postHere()
	s.timer = time.AfterFunc(d, func() {
		post(func(e *Editor) {
			if e.fs.searchKey() == key {
				e.startSearch()
			}
		})
	})
}

// stop 停下正在跑的搜索和等着开始的搜索。
func (s *fileSearch) stop() {
	if s.cancel != nil {
		s.cancel.Store(true)
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	if s.editTimer != nil {
		s.editTimer.Stop()
	}
	s.edited, s.editTimer = nil, nil
	s.running = false
}

func (e *Editor) startSearch() {
	s := &e.fs
	s.key = s.searchKey()
	if s.cancel != nil {
		s.cancel.Store(true)
	}
	s.gen++
	s.re, s.files, s.count, s.running, s.stopped, s.err = nil, nil, 0, false, false, ""
	s.sel, s.top = 0, 0
	if s.query == "" || e.ws == nil {
		return
	}
	re, err := compileQuery(s.query, s.matchCase, s.word, s.regex)
	if err != nil {
		s.err = "正则写错了"
		return
	}
	s.re = re
	inc, exc := glob.ParseGlobs(s.include), glob.ParseGlobs(s.exclude)
	var rels []string
	for _, rel := range e.ws.index {
		if (len(inc) == 0 || inc.Match(rel)) && !exc.Match(rel) {
			rels = append(rels, rel)
		}
	}
	// 开着且有未保存修改的文件按编辑器里的文字找。
	texts := map[string]searchText{}
	for _, d := range e.docs {
		if d.buf.Dirty() {
			if rel := e.rel(d.buf.Path()); !filepath.IsAbs(rel) {
				snap := d.buf.Snapshot()
				texts[rel] = searchText{snap.String(0, snap.Len()), d, snap.Version()}
			}
		}
	}
	cancel := &atomic.Bool{}
	s.cancel, s.running = cancel, true
	e.runSearch(s.gen, re, rels, texts, cancel)
}

// runSearch 在后台几个 goroutine 一起找，每 60 毫秒把找到的寄回所有者。
func (e *Editor) runSearch(gen int, re *regexp.Regexp, rels []string, texts map[string]searchText, cancel *atomic.Bool) {
	root := e.root
	post := e.postHere()
	go func() {
		var (
			mu    sync.Mutex
			batch []*searchFile
			found atomic.Int64
			next  atomic.Int64
			wg    sync.WaitGroup
		)
		flush := func(done bool) {
			mu.Lock()
			b := batch
			batch = nil
			mu.Unlock()
			stopped := found.Load() >= maxSearchMatches
			if len(b) > 0 || done {
				post(func(e *Editor) { e.takeResults(gen, b, done, stopped) })
			}
		}
		for range max(1, min(runtime.GOMAXPROCS(0), 8, len(rels))) {
			wg.Go(func() {
				for !cancel.Load() {
					i := int(next.Add(1) - 1)
					if i >= len(rels) {
						return
					}
					limit := maxSearchMatches - int(found.Load())
					if limit <= 0 {
						cancel.Store(true)
						return
					}
					f := searchFileText(root, rels[i], texts, re, limit)
					if f == nil {
						continue
					}
					found.Add(int64(len(f.matches)))
					mu.Lock()
					batch = append(batch, f)
					mu.Unlock()
				}
			})
		}
		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		tick := time.NewTicker(60 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				flush(false)
				continue
			case <-done:
			}
			break
		}
		flush(true)
	}()
}

// searchFileText 在一个文件（或它在 texts 里的文字）中找 re；没找到、读不了（太大、二进制）时返回 nil。
func searchFileText(root, rel string, texts map[string]searchText, re *regexp.Regexp, limit int) *searchFile {
	path := filepath.Join(root, filepath.FromSlash(rel))
	f := &searchFile{path: path, rel: rel}
	t, ok := texts[rel]
	txt := t.text
	if ok {
		f.doc, f.ver = t.doc, t.ver
	} else {
		txt, f.disk, ok = readSearchText(path)
	}
	if !ok {
		return nil
	}
	if f.doc == nil {
		f.digest = sha256.Sum256([]byte(txt))
	}
	f.matches = matchesAt(txt, re.FindAllStringIndex(txt, limit))
	if len(f.matches) == 0 {
		return nil
	}
	return f
}

// readSearchText 读文件的文字和读的是哪个版本；太大、二进制、读不了时返回 false。
func readSearchText(path string) (string, edit.Stamp, bool) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0) // 不在 FIFO 上等
	if err != nil {
		return "", edit.Stamp{}, false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxSearchBytes {
		return "", edit.Stamp{}, false
	}
	data := make([]byte, info.Size())
	n, err := io.ReadFull(f, data)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", edit.Stamp{}, false
	}
	data = data[:n]
	if bytes.IndexByte(data[:min(len(data), 8192)], 0) >= 0 {
		return "", edit.Stamp{}, false
	}
	// Search offsets must share the editor snapshot's BOM-free coordinate space.
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	return string(data), edit.Stamp{ModTime: info.ModTime(), Size: info.Size()}, true
}

// matchesAt 返回 text 在 locs 处的匹配，带上匹配所在行显示得下的部分。
func matchesAt(txt string, locs [][]int) []searchMatch {
	var out []searchMatch
	line, lineStart, at := 0, 0, 0
	for _, m := range locs {
		if m[0] == m[1] {
			continue
		}
		if n := strings.Count(txt[at:m[0]], "\n"); n > 0 {
			line += n
			lineStart = at + strings.LastIndexByte(txt[at:m[0]], '\n') + 1
		}
		at = m[0]
		lineEnd := len(txt)
		if i := strings.IndexByte(txt[m[0]:], '\n'); i >= 0 {
			lineEnd = m[0] + i
		}
		if lineEnd > lineStart && txt[lineEnd-1] == '\r' {
			lineEnd--
		}
		hitEnd := min(m[1], lineEnd)
		from := runeStart(txt, max(lineStart, m[0]-previewBytes))
		to := runeStart(txt, min(lineEnd, hitEnd+previewBytes))
		out = append(out, searchMatch{start: m[0], end: m[1], text: txt[m[0]:m[1]], line: line,
			pre: txt[from:m[0]], hit: txt[m[0]:max(hitEnd, m[0])], post: txt[max(hitEnd, m[0]):max(to, hitEnd)]})
	}
	return out
}

func runeStart(s string, i int) int {
	for i > 0 && i < len(s) && !utf8.RuneStart(s[i]) {
		i--
	}
	return i
}

// takeResults 收下后台找到的一批文件（另一次搜索已经开始时丢掉）。
func (e *Editor) takeResults(gen int, files []*searchFile, done, stopped bool) {
	s := &e.fs
	if gen != s.gen {
		return
	}
	for _, f := range files {
		i, ok := slices.BinarySearchFunc(s.files, f.rel, func(x *searchFile, rel string) int { return strings.Compare(x.rel, rel) })
		if ok {
			s.count -= len(s.files[i].matches)
			s.files[i] = f
		} else {
			s.files = slices.Insert(s.files, i, f)
		}
		if room := max(maxSearchMatches-s.count, 0); len(f.matches) > room {
			f.matches, stopped = f.matches[:room], true
		}
		s.count += len(f.matches)
	}
	s.stopped = s.stopped || stopped
	if done {
		s.running = false
	}
}

// searchRow 是结果的一行：match 为 -1 时是文件标题。
type searchRow struct{ file, match int }

func (e *Editor) searchRows() []searchRow {
	s := &e.fs
	var rows []searchRow
	for i, f := range s.files {
		rows = append(rows, searchRow{i, -1})
		if s.collapsed[f.rel] {
			continue
		}
		for j := range f.matches {
			rows = append(rows, searchRow{i, j})
		}
	}
	return rows
}

// openMatch 在预览标签里显示匹配并选中它；keep 为真时转正并把键盘交给编辑器。
func (e *Editor) openMatch(r searchRow, keep bool) error {
	s := &e.fs
	if r.file >= len(s.files) {
		return nil
	}
	f := s.files[r.file]
	if r.match < 0 {
		if s.collapsed == nil {
			s.collapsed = map[string]bool{}
		}
		s.collapsed[f.rel] = !s.collapsed[f.rel]
		return nil
	}
	m := f.matches[r.match]
	var err error
	if keep {
		c := e.fileColumn()
		err = e.openIn(c, c.active+1, f.path, false)
	} else {
		err = e.previewFile(f.path)
	}
	if err != nil {
		return err
	}
	e.showSearchMatch(m)
	if keep {
		e.area = areaEditor
	}
	return nil
}

// showSearchMatch selects a search match in the current tab and brings it to the middle of the pane.
func (e *Editor) showSearchMatch(m searchMatch) {
	t := e.cur()
	b := t.doc.buf
	n := b.Snapshot().Len()
	b.SetSelections(edit.Single(edit.Range{Anchor: min(m.start, n), Head: min(m.end, n), GoalX: -1}))
	e.centerLine(t, b.Snapshot().LineAt(min(m.start, n)))
	e.reveal()
}

// DropSearchMatch opens search result row i where it was dropped: in column col, or with split
// in a new column before (-1) or after (1) it; a drop targets a locked column on purpose. The
// match is selected. line is the row's line when the drag began: a result list that changed
// meanwhile drops nothing.
func (e *Editor) DropSearchMatch(col, i, line, split int) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	e.sync()
	defer e.sync()
	rows := e.searchRows()
	if i < 0 || i >= len(rows) || rows[i].match < 0 {
		return
	}
	f := e.fs.files[rows[i].file]
	m := f.matches[rows[i].match]
	if m.line != line {
		e.status = "搜索结果刚更新过：请重新拖动"
		return
	}
	c := e.cols[col]
	if split != 0 {
		c = e.splitColumn(col, split < 0)
	}
	if err := e.openIn(c, c.active+1, f.path, false); err != nil {
		e.status = err.Error()
		return
	}
	e.showSearchMatch(m)
	e.area = areaEditor
}

// replaceIn 在文件里替换：only 不为 nil 时只替换那一处（它在文件里的位置变了就不换）。
// 开着的文件作为一步撤销；没开的读出来、替换、原子写回。返回替换了几处。
func (e *Editor) replaceIn(f *searchFile, only *searchMatch) (int, error) {
	s := &e.fs
	tmpl := replaceTemplate(s.with, s.regex)
	if d := e.docOf(f.path); d != nil {
		snap := d.buf.Snapshot()
		cs := replacements(snap.String(0, snap.Len()), s.re, tmpl, only)
		if len(cs) == 0 {
			return 0, nil
		}
		d.buf.Apply(edit.Transaction{Changes: cs, Origin: edit.OriginCommand})
		d.buf.BreakGroup()
		return len(cs), nil
	}
	snap, err := text.Load(f.path)
	if err != nil {
		return 0, err
	}
	if snap.Meta().ReadOnly {
		return 0, fmt.Errorf("%s 是只读的", filepath.Base(f.path))
	}
	// The file can change between validating a result and loading its replacement.
	if only != nil && sha256.Sum256([]byte(snap.String(0, snap.Len()))) != f.digest {
		return 0, edit.ErrConflict
	}
	cs := replacements(snap.String(0, snap.Len()), s.re, tmpl, only)
	if len(cs) == 0 {
		return 0, nil
	}
	buffer := edit.NewBuffer(snap)
	buffer.Apply(edit.Transaction{Changes: cs, Origin: edit.OriginCommand})
	return len(cs), buffer.Save()
}

// replacements 在文字里重新找一遍匹配，算出替换；only 只换落在同一位置、文字相同的那一处。
func replacements(txt string, re *regexp.Regexp, tmpl string, only *searchMatch) text.ChangeSet {
	var cs text.ChangeSet
	for _, m := range re.FindAllStringSubmatchIndex(txt, -1) {
		if m[0] == m[1] {
			continue
		}
		if only != nil && (m[0] != only.start || m[1] != only.end || txt[m[0]:m[1]] != only.text) {
			continue
		}
		var dst []byte
		dst = re.ExpandString(dst, tmpl, txt, m)
		cs = append(cs, text.Change{Start: m[0], End: m[1], Text: string(dst)})
	}
	return cs
}

// current 报告 f 还是不是搜索时的那个版本：文档没再改过，磁盘上的文件没被换过。
func (e *Editor) current(f *searchFile) bool {
	d := e.docOf(f.path)
	if f.doc != nil {
		return d == f.doc && d.buf.Snapshot().Version() == f.ver
	}
	if d != nil && (d.buf.Dirty() || d.buf.DiskStamp() != f.disk || d.buf.DiskState() != edit.DiskSame) {
		return false
	}
	// Metadata can be preserved by sync tools; the same match at the same offset may belong
	// to a different file version. A single-result action requires the whole searched version.
	txt, stamp, ok := readSearchText(f.path)
	return ok && stamp == f.disk && sha256.Sum256([]byte(txt)) == f.digest
}

// replaceSearch 替换选中的匹配、选中文件的全部匹配，或者（all）所有文件的全部匹配。
// 结果和现在的查询、文件夹、文件版本对不上时不替换，重新搜索。
func (e *Editor) replaceSearch(all bool) error {
	s := &e.fs
	if s.re == nil || len(s.files) == 0 {
		return nil
	}
	if s.key != s.searchKey() {
		e.status = "搜索条件改了：等新的结果出来再替换"
		e.searchSoon(0)
		return nil
	}
	var files []*searchFile
	var only *searchMatch
	switch rows := e.searchRows(); {
	case all:
		files = s.files
	case s.sel < len(rows):
		r := rows[s.sel]
		files = []*searchFile{s.files[r.file]}
		if r.match >= 0 {
			only = &s.files[r.file].matches[r.match]
		}
	}
	for _, f := range files {
		if filepath.IsAbs(e.rel(f.path)) {
			return userError(f.path + " 不在当前文件夹里：不替换")
		}
	}
	if only != nil && !e.current(files[0]) {
		e.status = "这一处所在的文件已经变了：已重新搜索，确认后再替换"
		s.key = ""
		e.searchSoon(0)
		return nil
	}
	n, nf := 0, 0
	for _, f := range files {
		k, err := e.replaceIn(f, only)
		if err != nil {
			return err
		}
		if k > 0 {
			n, nf = n+k, nf+1
		}
	}
	switch {
	case only != nil && n == 0:
		e.status = "这一处已经变了：重新搜索一下"
	case only != nil:
		e.status = "替换了 1 处"
	default:
		e.status = fmt.Sprintf("在 %d 个文件里替换了 %d 处", nf, n)
	}
	e.checkDisk()
	s.key = "" // 重新搜索
	e.searchSoon(0)
	return nil
}

// searchFrame 把搜索视图算成显示模型。
func (e *Editor) searchFrame(dst *view.Search) {
	s := &e.fs
	rows := dst.Rows[:0]
	*dst = view.Search{
		Query: s.query, With: s.with, Include: s.include, Exclude: s.exclude,
		MatchCase: s.matchCase, Word: s.word, Regex: s.regex, Replacing: s.replacing, Details: s.details,
		Running: s.running || s.query != "" && s.searchKey() != s.key, // 等着打字停下来的也算在搜
		Error:   s.err != "",
	}
	if e.area == areaTree && e.side == sideSearch {
		dst.Focus = view.SearchField(s.focus) + 1
	}
	switch {
	case s.err != "":
		dst.Summary = s.err
	case s.query == "":
	case s.count == 0 && !dst.Running:
		dst.Summary = "没有结果"
	case s.count > 0:
		dst.Summary = fmt.Sprintf("%d 个文件中有 %d 个结果", len(s.files), s.count)
		if s.stopped {
			dst.Summary += "（只显示前 20000 个）"
		}
	}
	for _, r := range e.searchRows() {
		f := s.files[r.file]
		if r.match < 0 {
			name, dir := splitPath(f.rel)
			rows = append(rows, view.SearchRow{Path: f.path, Name: name, Dir: dir, Count: len(f.matches), Collapsed: s.collapsed[f.rel]})
			continue
		}
		m := f.matches[r.match]
		row := view.SearchRow{Match: true, Line: m.line, Pre: m.pre, Hit: m.hit, Post: m.post}
		if s.replacing && s.re != nil && s.count <= 2000 {
			row.With, row.Replacing = s.re.ReplaceAllString(m.text, replaceTemplate(s.with, s.regex)), true
		}
		rows = append(rows, row)
	}
	dst.Rows = rows
	s.sel = max(min(s.sel, len(rows)-1), 0)
	if s.sel < s.top {
		s.top = s.sel
	} else if s.sel >= s.top+e.sideRows {
		s.top = s.sel - e.sideRows + 1
	}
	dst.Selected, dst.Top = s.sel, s.top
}

// handleSearchKey 处理搜索视图有焦点时的按键；返回 false 表示交给 keymap。
func (e *Editor) handleSearchKey(c keymap.Chord) bool {
	s := &e.fs
	if c.Mods&^keymap.ModShift != 0 {
		return false
	}
	if s.focus == sfResults {
		switch c.Key {
		case keymap.KeyUp:
			if s.sel == 0 {
				s.focus = sfQuery
				return true
			}
			e.searchMove(-1)
		case keymap.KeyDown:
			e.searchMove(1)
		case keymap.KeyPgUp:
			e.searchMove(-e.sideRows)
		case keymap.KeyPgDn:
			e.searchMove(e.sideRows)
		case keymap.KeyEnter:
			if rows := e.searchRows(); s.sel < len(rows) {
				if err := e.openMatch(rows[s.sel], true); err != nil {
					e.status = err.Error()
				}
			}
		case keymap.KeyEsc, keymap.KeyTab:
			s.focus = sfQuery
		default:
			return c.Key == keymap.KeyRune
		}
		return true
	}
	field := e.searchField()
	switch c.Key {
	case keymap.KeyEsc:
		e.area = areaEditor
	case keymap.KeyDown:
		if len(e.searchRows()) > 0 {
			s.focus = sfResults
			e.searchMove(0)
		}
	case keymap.KeyTab:
		order := []searchFieldID{sfQuery}
		if s.replacing {
			order = append(order, sfReplace)
		}
		if s.details {
			order = append(order, sfInclude, sfExclude)
		}
		i := slices.Index(order, s.focus)
		if c.Mods == keymap.ModShift {
			s.focus = order[(i+len(order)-1)%len(order)]
		} else {
			s.focus = order[(i+1)%len(order)]
		}
	case keymap.KeyEnter:
		if s.focus == sfReplace {
			return e.replaceSearch(false) == nil
		}
		s.key = ""
		e.searchSoon(0)
	case keymap.KeyBackspace:
		if *field != "" {
			_, n := utf8.DecodeLastRuneInString(*field)
			*field = (*field)[:len(*field)-n]
			e.searchSoon(searchPause)
		}
	default:
		return c.Key == keymap.KeyRune
	}
	return true
}

// searchField 返回有焦点的输入框的文字。
func (e *Editor) searchField() *string {
	s := &e.fs
	switch s.focus {
	case sfReplace:
		return &s.with
	case sfInclude:
		return &s.include
	case sfExclude:
		return &s.exclude
	}
	return &s.query
}

// searchMove 在结果里移动，并在预览标签里显示选中的匹配（键盘留在结果里）。
func (e *Editor) searchMove(n int) {
	s := &e.fs
	rows := e.searchRows()
	if len(rows) == 0 {
		return
	}
	s.sel = max(min(s.sel+n, len(rows)-1), 0)
	if rows[s.sel].match >= 0 {
		e.openMatch(rows[s.sel], false)
		e.area = areaTree
	}
}

// ---- 前端用的语义动作 ----

// SetSearchField 用前端输入框里的文字替换搜索视图的一个输入框（0 查询、1 替换、2 包含、3 排除）。
func (e *Editor) SetSearchField(field int, value string) {
	s := &e.fs
	switch searchFieldID(field) {
	case sfQuery:
		s.query = value
	case sfReplace:
		s.with = value
		return
	case sfInclude:
		s.include = value
	case sfExclude:
		s.exclude = value
	}
	e.searchSoon(searchPause)
}

// FocusSearch 记下键盘在搜索视图的哪个输入框（-1 表示不在搜索视图里）。
func (e *Editor) FocusSearch(field int) {
	if field < 0 {
		if e.side == sideSearch && e.area == areaTree {
			e.area = areaEditor
		}
		return
	}
	e.fs.focus = searchFieldID(field)
	e.side, e.area = sideSearch, areaTree
}

// SearchClick 选中第 i 行结果：文件标题折叠或展开；匹配在预览标签里打开，keep（双击）时转正。
func (e *Editor) SearchClick(i int, keep bool) {
	e.sync()
	defer e.sync()
	rows := e.searchRows()
	if i < 0 || i >= len(rows) {
		return
	}
	e.fs.sel, e.fs.focus = i, sfResults
	e.side, e.area = sideSearch, areaTree
	if err := e.openMatch(rows[i], keep); err != nil {
		e.status = err.Error()
	}
	if !keep {
		e.area = areaTree
	}
}

// ToggleSearchOption 切换搜索选项：0 区分大小写、1 全字、2 正则、3 替换框、4 包含排除框。
func (e *Editor) ToggleSearchOption(opt int) {
	s := &e.fs
	switch opt {
	case 0:
		s.matchCase = !s.matchCase
	case 1:
		s.word = !s.word
	case 2:
		s.regex = !s.regex
	case 3:
		s.replacing = !s.replacing
		return
	case 4:
		s.details = !s.details
		return
	}
	e.searchSoon(0)
}

// ReplaceSearch 替换第 i 行（文件标题：整个文件；匹配：那一处）；i < 0 时全部替换。
func (e *Editor) ReplaceSearch(i int) {
	e.sync()
	defer e.sync()
	all := i < 0
	if !all {
		e.fs.sel = i
	}
	if err := e.replaceSearch(all); err != nil {
		e.status = err.Error()
	}
}
