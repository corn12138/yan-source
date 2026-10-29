package app

import (
	"errors"
	"fmt"
	"github.com/corn12138/yan-source/internal/process"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/git/linediff"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/syntax/highlight"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/text/width"
	"github.com/corn12138/yan-source/core/view"
)

// 改动视图，照着 Mayoi 的 diff 标签（cmd/mayoi/diffview.go）的单栏样子：一个只读标签显示文件相对 HEAD
// 的改动——删掉的行红底、加上的行绿底，改过的行里变了的词颜色更深；没改的行只在改动前后各留三行，
// 其余折成「⋯ 12 行没改」，回车或双击展开；在其他行上回车或双击，打开文件跳到那一行。

const diffContext = 3 // 打开时光标上面留几行（上下文行数见 diffOpts.context）

// diffOpts 是排改动视图的设置：并排时一边几格（0 是一栏）、制表符宽、改动前后留几行没改的、
// 改过的行里怎么标出改了的部分（""：按词，"char"：按字，"line"：不标），和 @@ 后面写的作用域（新文件第几行所在的块）。
type diffOpts struct {
	splitW, tabW, context int
	highlight             string
	scope                 func(line int) string
}

func (e *Editor) diffOpts(splitW int) diffOpts {
	return diffOpts{splitW: splitW, tabW: e.cfg.TabWidth, context: e.cfg.DiffContext, highlight: e.cfg.DiffHighlight}
}

// diffOptsFor 是改动视图 dt 排文件 path（新的一边是 cur）用的设置：显示整个文件时不折，@@ 后面写上作用域。
func (e *Editor) diffOptsFor(dt *diffTab, path string, cur []string) diffOpts {
	o := e.diffOpts(dt.splitW)
	if dt.full {
		o.context = 1 << 30
	}
	o.scope = e.scopeNamer(path, cur)
	return o
}

// maxScopeLines 是给 @@ 找作用域的文件最多几行：再长就不找了。
const maxScopeLines = 50000

// scopeNamer 给 @@ 标题找新文件第 line 行所在的（最里面的）块，同面包屑：按缩进找开始它的那一行。
func (e *Editor) scopeNamer(path string, cur []string) func(int) string {
	if len(cur) == 0 || len(cur) > maxScopeLines || !isCodePath(path) {
		return nil
	}
	var s text.Snapshot
	ready := false
	cache := map[int]int{}
	return func(line int) string {
		if !ready {
			s, ready = text.FromString(strings.Join(cur, "\n"), text.Meta{Path: path}), true
		}
		sc := e.scopes(s, line, cache)
		if len(sc) == 0 {
			return ""
		}
		return scopeLabel(s, sc[len(sc)-1])
	}
}

// header 是一处改动的 @@ 标题：两边的行号和行数，有作用域时跟在后面。
func (o diffOpts) header(h linediff.Hunk) string {
	head := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.A+1, h.ALen, h.B+1, h.BLen)
	if o.scope != nil {
		if sc := o.scope(h.B); sc != "" {
			head += " " + sc
		}
	}
	return head
}

// words 是一对改过的行里改了的部分。
func (o diffOpts) words(a, b string) (as, bs []linediff.Span) {
	switch o.highlight {
	case "line":
		return nil, nil
	case "char":
		return linediff.Chars(a, b)
	}
	return linediff.Words(a, b)
}

type diffKind uint8

const (
	diffSame diffKind = iota
	diffAdded
	diffRemoved
	diffHeader
	diffFold
	diffReview
	diffFile // 所有改动里一个文件的标题
)

// diffLine 是改动视图的一行：它是什么、对应文件的第几行（删掉的行是 HEAD 里的第几行）、
// 词级别的改动、折起来的那段是第几个。
type diffLine struct {
	copyHash string
	noteID   int
	oldLine  int // The left side has its own coordinate in a paired split row.
	kind     diffKind
	line     int // 文件里（或 HEAD 里）的行，从 0 数
	words    []linediff.Span
	fold     int
	sec      int // 所有改动里是第几个文件（只看一个文件时是 0）
	hunk     int // 这个文件的第几处改动（不在改动里时是 -1）
	// 并排时：左边是旧的（left 为真表示这一行左边有字，它删掉了时 leftGone），右边是新的；
	// 右边从第 rightAt 个字节开始，左边到 leftEnd 为止。words 里左边的词以 0 起，右边的以 rightAt 起。
	split             bool
	left, leftGone    bool
	right, rightAdded bool
	leftEnd, rightAt  int
	rwords            []linediff.Span
	// 所有改动里文件标题行的增减条：从第 barAt 个字节起，加的 addB 格、删的 delB 格，其余空格（共 statBarW 格）。
	barAt, addB, delB int
	ltext, rtext      string // 并排：两边这一行原本的文字（没截、没展开制表符），两个窗格画它们
	bodyToggle        bool   // 提交的标签里折起、展开说明的那一行
}

// diffTab 是显示一个文件改动的标签的状态。
type diffTab struct {
	repoRoot string

	reviewGen  uint64
	commitHash string
	path       string // 文件
	version    uint64 // 按文件的哪个版本算的（文件没开时为 0）
	statusOf   uintptr
	lines      []diffLine
	expanded   map[int]bool // 展开了的折叠
	base       []string     // HEAD 里的文本
	hasBase    bool
	// fixed 的标签看的是一个提交里的改动：新的一边是 after（那个提交里的文本），不跟着文件变。
	fixed bool
	after []string
	// peer is the old side of a comparison of two files (CompareFiles): base is its text, read
	// like the new side's, and peerVersion its document's version (0 when it is not open).
	peer        string
	peerVersion uint64
	boundary    *diffBoundary // the end the change navigation last said it reached
	editTarget  *document     // the document the diff's block actions edited last: its undo is the diff's
	full        bool          // 显示整个文件：没改的行都不折起来
	syncedTop   int           // 所有改动：上次按顶上的文件选中源代码管理里那一行时的滚动位置
	bodyOpen    bool          // 提交的标签：长的说明展开了
	// 并排两窗格（窗口版，splitdiff.go）：两边的文字与高亮、右边各自滚动时的顶行、
	// 下一帧让右边跟上左边、平滑滑过去，和块操作刚写进去要亮一下的行。
	sides      *splitSides
	rightTop   int
	alignRight bool
	glide      bool
	glow       splitGlow
	glowGen    uint64
	copyLeft   bool            // 并排：上次点的是左边，复制时复制左边的行
	autoFolded map[string]bool // 所有改动里因为大而默认折起来过的文件（分组:路径）：之后照用户的意思
	loadGen    uint64          // 所有改动第几次读；读完时不是最新的一次就丢掉
	// splitW 是这次按几格宽一边排的（0 是一栏）：窗格宽度或者布局变了就重排。
	splitW int
	// changes 的标签是「所有改动」：源代码管理里每个改了的文件一段（changes.go）；commit 不为空时是一个提交的标签。
	changes     bool
	commit      *git.Commit
	fileFolders map[string]bool
	revealFile  string
	compareFrom *git.Commit
	awaitRepo   bool
	loadError   string
	restore     *config.SessionTab
	secs        []changeSec
	collapsed   map[string]bool // 折起来的文件（分组:路径）
	loading     bool
}

// openChange 在标签里显示文件相对 HEAD 的改动；preview 为真时用预览标签。
func (e *Editor) openChange(path string, preview bool) error {
	defer e.bindRepository(e.repositoryForPath(path))()
	if e.repository().git.repo == nil {
		return errors.New("不在 git 仓库里")
	}
	name := filepath.Base(path) + " (改动)"
	if e.showingDiff(filepath.Join(filepath.Dir(path), name), preview) {
		return nil
	}
	dt := &diffTab{path: path, expanded: map[int]bool{}}
	out, err := process.GitCommand("-C", filepath.Dir(path), "show", "HEAD:./"+filepath.Base(path)).Output()
	if err == nil {
		dt.base, dt.hasBase = strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n"), true
	}
	e.showDiff(name, dt, preview)
	return nil
}

// showingDiff 在当前列里已经有这个改动视图时切过去，返回 true。
func (e *Editor) showingDiff(vpath string, preview bool) bool {
	c := e.fileColumn()
	i := c.showing(vpath)
	if i < 0 {
		return false
	}
	if !preview {
		e.keep(c.tabs[i])
	}
	e.activate(c, i)
	return true
}

// showDiff 在当前列里开一个叫 name 的只读标签显示 dt（同名的已经开着就切过去），光标停在第一处改动。
func (e *Editor) showDiff(name string, dt *diffTab, preview bool) {
	if dt.repoRoot == "" && e.repository().git.repo != nil {
		dt.repoRoot = e.repository().git.repo.Root
	}
	path := dt.path
	vpath := filepath.Join(filepath.Dir(path), name)
	if e.showingDiff(vpath, preview) {
		return
	}
	c := e.fileColumn()
	d := &document{buf: edit.NewBuffer(text.FromString("", text.Meta{Path: vpath, ReadOnly: true}))}
	if lx := highlight.Lexer(path); lx != nil {
		d.hl = highlight.New(lx, func() { e.Post(func(*Editor) {}) })
	}
	e.refreshDiff(d, dt)
	t := &tab{doc: d, sels: edit.Single(edit.Cursor(0)), preview: preview, diff: dt}
	d.tabs = append(d.tabs, t)
	if preview {
		if old := slicesIndex(c.tabs, func(t *tab) bool { return t.preview }); old >= 0 && !c.tabs[old].doc.buf.Dirty() {
			e.dropTab(c, old)
		}
	}
	at := min(c.active+1, len(c.tabs))
	c.tabs = append(c.tabs[:at], append([]*tab{t}, c.tabs[at:]...)...)
	if c.active >= at {
		c.active++
	}
	e.activate(c, at)
	if n := firstChange(dt.lines); n > 0 {
		t.scrollY = max(n-diffContext-2, 0) // 改动前面的几行和 @@ 标题也看得见
		d.buf.SetSelections(edit.Single(edit.Cursor(d.buf.Snapshot().LineStart(n))))
	}
}

func slicesIndex[T any](s []T, f func(T) bool) int {
	for i, v := range s {
		if f(v) {
			return i
		}
	}
	return -1
}

func firstChange(lines []diffLine) int {
	for i, l := range lines {
		if l.kind == diffAdded || l.kind == diffRemoved {
			return i
		}
	}
	return 0
}

// currentLines 返回文件现在的行：开着就用编辑器里的文字（未保存的修改也算）。
func (e *Editor) currentLines(path string) ([]string, uint64) {
	if d := e.docOf(path); d != nil {
		s := d.buf.Snapshot()
		return strings.Split(strings.ReplaceAll(s.String(0, s.Len()), "\r\n", "\n"), "\n"), s.Version()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, 0 // 删掉了
	}
	return strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n"), 0
}

// refreshDiff 按文件现在的样子重新算改动，写进只读文档。
func (e *Editor) refreshDiff(d *document, dt *diffTab) {
	defer e.bindDiff(dt)()
	if dt.changes {
		e.buildChanges(d, dt)
		return
	}
	defer e.keepDiffPlace(d, dt)()
	cur, ver := dt.after, uint64(0)
	if !dt.fixed {
		cur, ver = e.currentLines(dt.path)
	}
	dt.version = ver
	if dt.peer != "" {
		dt.base, dt.peerVersion = e.currentLines(dt.peer)
		dt.hasBase = dt.base != nil
	}
	var base []string
	if dt.hasBase {
		base = dt.base
	}
	hunks := e.diffHunks(base, cur)
	var b strings.Builder
	dt.lines = dt.lines[:0]
	put := func(s string, l diffLine) {
		b.WriteString(s)
		b.WriteByte('\n')
		dt.lines = append(dt.lines, l)
	}
	if len(hunks) == 0 {
		msg := "（和 HEAD 一样，没有改动）"
		if dt.fixed {
			msg = "（这个提交没有改这个文件的文字）"
		}
		if dt.peer != "" {
			msg = "（两个文件的文字一样）"
		}
		if e.cfg.DiffWhitespace != "" {
			msg = strings.TrimSuffix(msg, "）") + "；" + diffWhitespaceLabel(e.cfg.DiffWhitespace) + "）"
		}
		put(msg, diffLine{kind: diffHeader, line: -1})
	}
	fold := 0
	dt.splitW = e.diffSplitWidthFor(e.columnOf(d))
	writeHunks(put, base, cur, hunks, 0, &fold, dt.expanded, e.diffOptsFor(dt, dt.path, cur))
	txt := e.reviewDiffText(dt, strings.TrimSuffix(b.String(), "\n"))
	d.buf.Reload(text.FromString(txt, d.buf.Snapshot().Meta()))
}

// writeHunks 写出 base 到 cur 的改动：每处改动前面一行 @@，没改的行在改动前后各留三行、其余折起来。
// fold 是折叠的编号（几个文件接着往下数），expanded 里的折叠写全。
func writeHunks(put func(string, diffLine), base, cur []string, hunks []linediff.Hunk, sec int, fold *int, expanded map[int]bool, o diffOpts) {
	if o.splitW > 0 {
		writeSplit(put, base, cur, hunks, sec, fold, expanded, o)
		return
	}
	at := 0 // cur 里写到了第几行
	same := func(from, to int, lead, trail bool) {
		// cur[from:to] 没改：开头、结尾各留三行，中间折起来
		n := to - from
		keepHead, keepTail := o.context, o.context
		if lead {
			keepHead = 0
		}
		if trail {
			keepTail = 0
		}
		if n <= keepHead+keepTail+1 || expanded[*fold] {
			for i := from; i < to; i++ {
				put(cur[i], diffLine{kind: diffSame, line: i, sec: sec, hunk: -1})
			}
			*fold++
			return
		}
		for i := from; i < from+keepHead; i++ {
			put(cur[i], diffLine{kind: diffSame, line: i, sec: sec, hunk: -1})
		}
		put(fmt.Sprintf("⋯ %d 行没改", n-keepHead-keepTail), diffLine{kind: diffFold, line: from + keepHead, fold: *fold, sec: sec, hunk: -1})
		*fold++
		for i := to - keepTail; i < to; i++ {
			put(cur[i], diffLine{kind: diffSame, line: i, sec: sec, hunk: -1})
		}
	}
	for k, h := range hunks {
		same(at, h.B, k == 0, false)
		put(o.header(h), diffLine{kind: diffHeader, line: h.B, sec: sec, hunk: k})
		for i := 0; i < h.ALen; i++ {
			l := diffLine{kind: diffRemoved, line: h.A + i, sec: sec, hunk: k}
			if i < h.BLen {
				l.words, _ = o.words(base[h.A+i], cur[h.B+i])
			}
			put(base[h.A+i], l)
		}
		for i := 0; i < h.BLen; i++ {
			l := diffLine{kind: diffAdded, line: h.B + i, sec: sec, hunk: k}
			if i < h.ALen {
				_, l.words = o.words(base[h.A+i], cur[h.B+i])
			}
			put(cur[h.B+i], l)
		}
		at = h.B + h.BLen
	}
	if len(hunks) > 0 {
		same(at, len(cur), false, true)
	}
}

// blockOf is what block actions the change of header line l takes.
func (dt *diffTab) blockOf(l diffLine) view.DiffBlock {
	switch {
	case dt.commit != nil || dt.fixed:
		return view.DiffBlockNone
	case dt.peer != "":
		return view.DiffBlockCompare
	case dt.changes:
		if l.sec < 0 || l.sec >= len(dt.secs) {
			return view.DiffBlockNone
		}
		s := dt.secs[l.sec]
		if l.kind == diffFile {
			switch {
			case s.group == changeConflict:
				return view.DiffBlockFileConflict
			case s.group == changeStaged:
				return view.DiffBlockFileStaged
			case s.group == changeUntracked:
				return view.DiffBlockFileUntracked
			case s.group == changeUnstaged && s.baseOK && len(s.cur) == 0 && !s.lazy:
				return view.DiffBlockFileDeleted
			}
			return view.DiffBlockFile
		}
		if s.group == changeStaged {
			return view.DiffBlockStaged
		}
		if s.group != changeUnstaged || !s.baseOK || len(s.cur) == 0 {
			return view.DiffBlockNone
		}
	}
	if l.kind == diffFile {
		return view.DiffBlockNone
	}
	return view.DiffBlockRevert
}

// diffRow 给改动视图的一行加上样式和行号。
func (dt *diffTab) row(row *view.RowFrame, i int) {
	if i < 0 || i >= len(dt.lines) {
		return
	}
	l := dt.lines[i]
	row.Num = int32(l.line + 1)
	if l.split {
		dt.splitRow(row, l)
		return
	}
	switch l.kind {
	case diffAdded:
		row.Base = style.DiffAdded
	case diffRemoved:
		row.Base = style.DiffRemoved
	case diffHeader, diffFold, diffFile, diffReview:
		row.Base, row.Num = style.DiffHeader, -1
		row.Spans = row.Spans[:0] // 不要语法高亮
	}
	if l.kind == diffHeader && l.hunk >= 0 || l.kind == diffFile {
		row.Block = dt.blockOf(l)
	}
	if g := dt.glow; g.path != "" && (l.kind == diffAdded || l.kind == diffSame) && l.line >= g.from && l.line < g.to && g.path == dt.pathOf(l) {
		row.Glow = true
	}
	if l.kind == diffFile && l.barAt >= 0 && l.barAt < len(row.Text) {
		at := int32(l.barAt)
		for k := 0; k < statBarW && int(at)+len("■") <= len(row.Text); k++ {
			st := style.Dim
			switch {
			case k < l.addB:
				st = style.Inserted
			case k < l.addB+l.delB:
				st = style.Deleted
			}
			row.Spans = append(row.Spans, style.Span{Start: at, End: at + int32(len("■")), Style: st})
			at += int32(len("■"))
		}
	}
	word := style.DiffAddedWord
	if l.kind == diffRemoved {
		word = style.DiffRemovedWord
	}
	for _, w := range l.words {
		if w.Start < w.End && w.End <= len(row.Text) {
			row.Spans = append(row.Spans, style.Span{Start: int32(w.Start), End: int32(w.End), Style: word})
		}
	}
}

// diffOpen 在改动视图里回车、双击：折起来的行展开；其他行打开文件跳到那一行。
func (e *Editor) diffOpen(t *tab) error {
	dt := t.diff
	s := t.doc.buf.Snapshot()
	i := s.LineAt(t.doc.buf.Selections().Main().Head)
	if i >= len(dt.lines) {
		return nil
	}
	l := dt.lines[i]
	if l.copyHash != "" {
		return e.copyCommitHash(l.copyHash)
	}
	switch l.kind {
	case diffReview:
		return e.editReview(l.noteID)
	case diffFold:
		if l.bodyToggle {
			dt.bodyOpen = !dt.bodyOpen
		} else {
			dt.expanded[l.fold] = true
		}
		e.refreshDiff(t.doc, dt)
		return nil
	case diffFile:
		if dt.changes && l.sec >= 0 && l.sec < len(dt.secs) && dt.secs[l.sec].group == changeConflict {
			return e.openMerge(dt.secs[l.sec].path)
		}
		e.toggleChangesFile(t, l.sec)
		return nil
	case diffRemoved:
		if dt.peer == "" { // 删掉的行：打开文件到这处改动所在的行
			for j := i; j >= 0; j-- {
				if h := dt.lines[j]; h.kind == diffHeader && h.sec == l.sec && h.hunk == l.hunk {
					l = h
					break
				}
			}
			if l.kind == diffRemoved {
				return errors.New("这一行已经删掉了")
			}
			break
		}
		c := e.fileColumn() // 比较两个文件：删掉的行在左边的文件里
		if err := e.openIn(c, c.active+1, dt.peer, false); err != nil {
			return err
		}
		e.GotoLine(l.line + 1)
		return nil
	}
	if l.line < 0 {
		return nil
	}
	if dt.fixed || dt.commit != nil { // 行号是那个提交里的，跟现在的文件对不上，文件也可能已经不在了
		return errors.New("这是提交里的版本：要编辑现在的文件，从资源管理器打开它")
	}
	c := e.fileColumn()
	if err := e.openIn(c, c.active+1, dt.pathOf(l), false); err != nil {
		return err
	}
	e.GotoLine(l.line + 1)
	return nil
}

// pathOf 是改动视图里一行所在的文件。
func (dt *diffTab) pathOf(l diffLine) string {
	if dt.changes && l.sec >= 0 && l.sec < len(dt.secs) {
		return dt.secs[l.sec].path
	}
	return dt.path
}

// refreshDiffs 在文件改了、状态变了之后重算开着的改动视图（每帧调用，没变就不算）。
func (e *Editor) refreshDiffs() {
	for _, c := range e.cols {
		t := c.tab()
		if t == nil || t.diff == nil {
			continue
		}
		dt := t.diff
		if w := e.diffSplitWidthFor(c); w != dt.splitW || dt.reviewGen != e.review.gen { // 窗格变宽、变窄：重新排
			e.refreshDiff(t.doc, dt)
			continue
		}
		if dt.fixed {
			continue
		}
		if dt.changes {
			e.refreshChanges(t)
			e.followChangesScroll(t)
			continue
		}
		if e.diffStale(dt.path, dt.version) || dt.peer != "" && e.diffStale(dt.peer, dt.peerVersion) {
			e.refreshDiff(t.doc, dt)
		}
	}
}

// diffStale reports whether the file at path is no longer the version a diff read of it: its
// document was edited, opened or closed.
func (e *Editor) diffStale(path string, version uint64) bool {
	if d := e.docOf(path); d != nil {
		return d.buf.Snapshot().Version() != version
	}
	return version != 0
}

// ---- 并排（照着 Mayoi 的 side by side）：左边旧的、右边新的，改过的行左右对着 ----

// diffLayout 是改动视图的排法：自动（每边放得下 80 格时并排）、总是并排、总是一栏。
type diffLayout uint8

const (
	layoutAuto diffLayout = iota
	layoutSplit
	layoutInline
)

func (l diffLayout) label() string { return [...]string{"自动", "并排", "一栏"}[l] }

// splitMin 是自动时并排要每边至少几格。
const splitMin = 80

// columnOf 是显示文档 d 的那一列（几列都显示它时是第一列，都不显示时是当前这一列）。
func (e *Editor) columnOf(d *document) *column {
	for _, c := range e.cols {
		if t := c.tab(); t != nil && t.doc == d {
			return c
		}
	}
	return e.column()
}

// diffSplitWidthFor 是第 c 列里的改动视图一边几格宽。
func (e *Editor) diffSplitWidthFor(c *column) int {
	layout := e.layout()
	if layout == layoutInline {
		return 0
	}
	gutter := 6
	if t := c.tab(); t != nil {
		gutter = int(e.gutter(t.doc.buf.Snapshot()))
	}
	w := (int(c.size.W) - gutter - 3) / 2
	switch {
	case w < 20:
		return 0
	case layout == layoutSplit || w >= splitMin:
		return w
	}
	return 0
}

// toggleDiffLayout 换改动视图的排法：自动 → 并排 → 一栏 → 自动。
func (e *Editor) toggleDiffLayout() error {
	l := (e.layout() + 1) % 3
	e.cfg.DiffLayout = [...]string{"", "split", "inline"}[l]
	if e.state != nil {
		v := e.cfg.DiffLayout
		e.state.DiffLayout = &v
		e.saveState()
	}
	for _, c := range e.cols {
		if t := c.tab(); t != nil && t.diff != nil {
			e.refreshDiff(t.doc, t.diff)
		}
	}
	e.status = "改动视图：" + l.label()
	return nil
}

// layout 是设置里改动视图的排法。
func (e *Editor) layout() diffLayout {
	switch e.cfg.DiffLayout {
	case "split":
		return layoutSplit
	case "inline":
		return layoutInline
	}
	return layoutAuto
}

// fitCells 把 s（制表符换成空格）截到 w 格，补空格到正好 w 格；返回结果、留下的原文有几个字节，
// 和原文每个字节在结果里的位置（offs[i]，i 从 0 到 len(s)；截掉的都在结果的末尾）。
func fitCells(s string, w, tabW int) (string, int, []int) {
	var b strings.Builder
	offs := make([]int, len(s)+1)
	used, kept := 0, 0
	for i, r := range s {
		cw := width.String(string(r))
		piece := string(r)
		if r == '\t' {
			cw = max(tabW, 1) - used%max(tabW, 1)
			piece = strings.Repeat(" ", cw)
		}
		if used+cw > w {
			for j := i; j <= len(s); j++ {
				offs[j] = b.Len()
			}
			break
		}
		for j := i; j < i+len(string(r)); j++ {
			offs[j] = b.Len()
		}
		b.WriteString(piece)
		used += cw
		kept = i + len(string(r))
		offs[kept] = b.Len()
	}
	end := b.Len()
	b.WriteString(strings.Repeat(" ", w-used))
	return b.String(), min(kept, len(s)), clampOffs(offs, end)
}

// clampOffs 让截掉的部分都落在留下的文字末尾。
func clampOffs(offs []int, end int) []int {
	for i := range offs {
		offs[i] = min(offs[i], end)
	}
	return offs
}

// mapWords 把原文里的词区间换成排过的那一行里的（起点加上 at）。
func mapWords(ws []linediff.Span, offs []int, at int) []linediff.Span {
	out := make([]linediff.Span, 0, len(ws))
	for _, w := range ws {
		if w.Start < len(offs) && w.End < len(offs) {
			out = append(out, linediff.Span{Start: at + offs[w.Start], End: at + offs[w.End]})
		}
	}
	return out
}

// writeSplit 是并排的 writeHunks：每一行左边 splitW 格、「 │ 」、右边。
// splitSep 隔开并排的两边。
const splitSep = " │ "

func writeSplit(put func(string, diffLine), base, cur []string, hunks []linediff.Hunk, sec int, fold *int, expanded map[int]bool, o diffOpts) {
	splitW, tabW := o.splitW, o.tabW
	const sep = splitSep
	row := func(left, right *string, l diffLine) {
		l.split = true
		var lt, rt string
		if left != nil {
			l.ltext = *left
		}
		if right != nil {
			l.rtext = *right
		}
		if left != nil {
			var offs []int
			lt, _, offs = fitCells(*left, splitW, tabW)
			l.leftEnd, l.left = offs[len(offs)-1], true
			l.words = mapWords(l.words, offs, 0)
		} else {
			lt = strings.Repeat(" ", splitW)
		}
		l.rightAt = len(lt) + len(sep)
		if right != nil {
			var offs []int
			rt, _, offs = fitCells(*right, splitW, tabW)
			rt = strings.TrimRight(rt, " ")
			l.right = true
			l.rwords = mapWords(l.rwords, offs, l.rightAt)
		}
		put(lt+sep+rt, l)
	}
	at, shift := 0, 0 // shift: how many more lines the new side has than the old before here
	same := func(from, to int, lead, trail bool) {
		n := to - from
		keepHead, keepTail := o.context, o.context
		if lead {
			keepHead = 0
		}
		if trail {
			keepTail = 0
		}
		one := func(i int) {
			row(&cur[i], &cur[i], diffLine{kind: diffSame, line: i, oldLine: i - shift, sec: sec, hunk: -1})
		}
		if n <= keepHead+keepTail+1 || expanded[*fold] {
			for i := from; i < to; i++ {
				one(i)
			}
			*fold++
			return
		}
		for i := from; i < from+keepHead; i++ {
			one(i)
		}
		put(fmt.Sprintf("⋯ %d 行没改", n-keepHead-keepTail), diffLine{kind: diffFold, line: from + keepHead, fold: *fold, sec: sec, hunk: -1})
		*fold++
		for i := to - keepTail; i < to; i++ {
			one(i)
		}
	}
	for k, h := range hunks {
		same(at, h.B, k == 0, false)
		put(o.header(h), diffLine{kind: diffHeader, line: h.B, sec: sec, hunk: k})
		for i := 0; i < max(h.ALen, h.BLen); i++ {
			l := diffLine{sec: sec, hunk: k}
			var left, right *string
			if i < h.ALen {
				left, l.leftGone, l.kind, l.line = &base[h.A+i], true, diffRemoved, h.A+i
				l.oldLine = h.A + i
			}
			if i < h.BLen {
				right, l.rightAdded, l.kind, l.line = &cur[h.B+i], true, diffAdded, h.B+i
			}
			if left != nil && right != nil {
				l.words, l.rwords = o.words(*left, *right)
			}
			row(left, right, l)
		}
		at = h.B + h.BLen
		shift += h.BLen - h.ALen
	}
	if len(hunks) > 0 {
		same(at, len(cur), false, true)
	}
}

// splitRow 给并排的一行加上样式：删掉的左半边红底、加上的右半边绿底，改了的词更深；行号是右边的。
func (dt *diffTab) splitRow(row *view.RowFrame, l diffLine) {
	n := int32(len(row.Text))
	if !l.right {
		row.Num = -1 // 右边没有这一行：不写行号
	}
	if l.leftGone && l.left {
		row.Spans = append(row.Spans, style.Span{Start: 0, End: min(int32(l.rightAt-len(splitSep)), n), Style: style.DiffRemoved})
		for _, w := range l.words {
			if w.Start < w.End && w.End <= l.leftEnd {
				row.Spans = append(row.Spans, style.Span{Start: int32(w.Start), End: int32(w.End), Style: style.DiffRemovedWord})
			}
		}
	}
	if l.rightAdded && l.right && int32(l.rightAt) < n {
		row.Spans = append(row.Spans, style.Span{Start: int32(l.rightAt), End: n, Style: style.DiffAdded})
		for _, w := range l.rwords { // 已经换成这一行里的位置
			if w.Start < w.End && int32(w.End) <= n {
				row.Spans = append(row.Spans, style.Span{Start: int32(w.Start), End: int32(w.End), Style: style.DiffAddedWord})
			}
		}
	}
}
