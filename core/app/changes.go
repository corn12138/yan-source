package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/git/linediff"
	"github.com/corn12138/yan-source/core/text"
)

// 「所有改动」标签，照着 Mayoi（MIT，cmd/mayoi/diffview.go 的 Changes 标签）：源代码管理里每个改了的文件一段，
// 和源代码管理的分组一样——先是已暂存的（HEAD → 索引），再是更改（索引 → 工作区），再是未跟踪的。
// 每段一个标题行（路径、加了几行、删了几行），回车、z 折起或展开；改动前后各留三行。
// 在改动视图里（一个文件的和所有改动的都是）键像翻页器：j k 上下，] [ 下一处、上一处改动，. , 下一个、上一个文件，
// 空格 f b d u 翻页、翻半页，g G 到头、到尾，z 展开折叠或折起文件，e 打开文件，s 暂存这处改动或这个文件
// （已暂存的取消暂存）。

// changeGroup 是一段改动在源代码管理里的分组。
type changeGroup uint8

const (
	changeStaged changeGroup = iota
	changeUnstaged
	changeUntracked
	changeConflict
	changeCompare
	changeCommit // 提交的标签里：这个提交改的（相对它的第一个父提交）
)

func (g changeGroup) label() string {
	return [...]string{"已暂存", "更改", "未跟踪", "合并冲突", "比较", "提交里"}[g]
}

// changeSec 是所有改动里的一个文件：从 base 改成 cur。
type changeSec struct {
	orig      string
	status    byte
	binary    bool
	path      string
	group     changeGroup
	base, cur []string
	baseOK    bool
	hunks     []linediff.Hunk
	// lazy 是没读的大文件（超过 bigDiffBytes）：折着，展开时才读；size 是它在磁盘上的大小。
	lazy bool
	size int64
}

// 大文件：改动超过 bigDiffLines 行的文件默认折起来；超过 bigDiffBytes 的，展开之前连文字都不读。
const (
	bigDiffLines = 2000
	bigDiffBytes = 1 << 20
)

func (s *changeSec) key() string { return s.group.label() + ":" + s.path }

const changesName = "所有改动"

// openAllChanges 开「所有改动」标签（已经开着就切过去）。
func (e *Editor) openAllChanges() error {
	r := e.repository().git.repo
	if r == nil {
		return errors.New("不在 git 仓库里")
	}
	dt := &diffTab{path: filepath.Join(r.Root, changesName), changes: true, expanded: map[int]bool{}, collapsed: map[string]bool{}}
	e.showDiff(changesName, dt, false)
	if t := e.cur(); t != nil && t.diff == dt {
		e.loadChanges(t)
	}
	return nil
}

// loadChanges 在后台按 git 的状态读每个文件两边的文字。
func (e *Editor) loadChanges(t *tab) {
	defer e.bindDiff(t.diff)()
	dt := t.diff
	if dt.commit != nil {
		e.loadCommit(t)
		return
	}
	r, st := e.repository().git.repo, e.repository().git.status
	if r == nil || st == nil || dt.loading {
		return
	}
	dt.loading, dt.loadError = true, ""
	dt.statusOf = uintptr(unsafe.Pointer(st))
	entries := st.Entries
	post := e.postGit()
	dt.loadGen++
	gen := dt.loadGen
	opened := map[string]bool{} // 用户展开过的大文件：照常读
	for k, auto := range dt.autoFolded {
		if auto && !dt.collapsed[k] {
			opened[k] = true
		}
	}
	go func() {
		lazy := func(path string, g changeGroup) (int64, bool) {
			if opened[g.label()+":"+path] {
				return 0, false
			}
			fi, err := os.Stat(path)
			if err != nil || fi.IsDir() || fi.Size() <= bigDiffBytes {
				return 0, false
			}
			return fi.Size(), true
		}
		show := func(rev, path string) ([]string, bool) {
			txt, ok, err := r.Show(rev, path)
			if err != nil || !ok {
				return nil, false
			}
			return splitText(txt), true
		}
		disk := func(path string) []string {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			return splitText(string(b))
		}
		var staged, unstaged, untracked, conflicts []changeSec
		for _, en := range entries {
			switch {
			case en.Conflicted():
				conflicts = append(conflicts, changeSec{path: en.Path, group: changeConflict})
			case en.Untracked():
				if size, ok := lazy(en.Path, changeUntracked); ok {
					untracked = append(untracked, changeSec{path: en.Path, group: changeUntracked, lazy: true, size: size})
					continue
				}
				untracked = append(untracked, changeSec{path: en.Path, group: changeUntracked, cur: disk(en.Path)})
			default:
				if size, ok := lazy(en.Path, changeStaged); ok && en.Staged() {
					staged = append(staged, changeSec{path: en.Path, group: changeStaged, lazy: true, size: size})
				} else if en.Staged() {
					base, ok := show("HEAD", en.Path)
					cur, _ := show("", en.Path)
					staged = append(staged, changeSec{path: en.Path, group: changeStaged, base: base, baseOK: ok, cur: cur})
				}
				if size, ok := lazy(en.Path, changeUnstaged); ok && en.Unstaged() && !en.Submodule() {
					unstaged = append(unstaged, changeSec{path: en.Path, group: changeUnstaged, lazy: true, size: size})
				} else if en.Unstaged() {
					base, ok := show("", en.Path)
					cur := disk(en.Path)
					if en.Submodule() {
						if text, ok, err := r.WorkSubproject(en.Path, en.Dirty()); err == nil && ok {
							cur = splitText(text)
						}
					}
					unstaged = append(unstaged, changeSec{path: en.Path, group: changeUnstaged, base: base, baseOK: ok, cur: cur})
				}
			}
		}
		secs := append(append(append(conflicts, staged...), unstaged...), untracked...)
		post(func(e *Editor) {
			if gen != dt.loadGen {
				return // 读的时候又开始了一次：这次的已经过时
			}
			first := len(diffHeads(dt)) == 0
			dt.loading = false
			dt.secs = secs
			if dt.autoFolded == nil {
				dt.autoFolded = map[string]bool{}
			}
			reread := false
			for _, s := range secs {
				k := s.key()
				if big := s.lazy || max(len(s.base), len(s.cur)) > bigDiffLines; big && !dt.autoFolded[k] {
					dt.autoFolded[k], dt.collapsed[k] = true, true // 第一次见到的大文件：折起来
				}
				reread = reread || s.lazy && !dt.collapsed[k]
			}
			e.followReview(true)
			e.buildChanges(t.doc, dt)
			if n := firstChange(dt.lines); first && n > 0 && dt.restore == nil {
				// 第一次读到改动：光标停在第一处改动，像单个文件的改动视图
				if n >= t.scrollY+e.paneHeight() { // 第一个文件的标题看得见时不滚
					t.scrollY = max(n-diffContext-2, 0)
				}
				t.doc.buf.SetSelections(edit.Single(edit.Cursor(t.doc.buf.Snapshot().LineStart(n))))
			}
			if reread { // 读的时候展开了一个没读的大文件
				e.loadChanges(t)
			}
		})
	}()
}

// splitText 把文字拆成行（CRLF 当 LF）。
func splitText(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

// refreshChanges 每帧看一眼：git 的状态变了就重读，编辑器里改了文件的文字就重新比较。
func (e *Editor) refreshChanges(t *tab) {
	defer e.bindDiff(t.diff)()
	dt := t.diff
	if dt.commit != nil {
		return // 提交里的改动不会变
	}
	if st := e.repository().git.status; st != nil && uintptr(unsafe.Pointer(st)) != dt.statusOf {
		e.loadChanges(t)
		return
	}
	changed := false
	for i := range dt.secs {
		s := &dt.secs[i]
		if s.group != changeUnstaged && s.group != changeUntracked || s.lazy {
			continue
		}
		if d := e.docOf(s.path); d != nil {
			cur, _ := e.currentLines(s.path)
			if !equalLines(cur, s.cur) {
				s.cur, changed = cur, true
			}
		}
	}
	if changed {
		e.buildChanges(t.doc, dt)
	}
}

// trimLastEmpty 去掉末尾换行符拆出来的那个空行。
func trimLastEmpty(lines []string) []string {
	if n := len(lines); n > 0 && lines[n-1] == "" {
		return lines[:n-1]
	}
	return lines
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// buildChanges 把每个文件的改动写进只读文档：标题行，再是改动（折起来的文件只有标题）。
func (e *Editor) buildChanges(d *document, dt *diffTab) {
	defer e.keepDiffPlace(d, dt)()
	defer e.bindDiff(dt)()
	var b strings.Builder
	dt.lines = dt.lines[:0]
	put := func(s string, l diffLine) {
		b.WriteString(s)
		b.WriteByte('\n')
		dt.lines = append(dt.lines, l)
	}
	r := e.repository().git.repo
	dt.splitW = e.diffSplitWidthFor(e.columnOf(d))
	if c := dt.commit; c != nil { // 提交的标签：先是说明、作者、日期、提交号、分支
		if from := dt.compareFrom; from != nil {
			put("比较：起点 → 终点（回车或双击提交行复制提交号）", diffLine{kind: diffHeader, line: -1, hunk: -1, sec: -1})
			put("起点 "+from.Hash+" · "+from.Subject, diffLine{kind: diffHeader, line: -1, hunk: -1, sec: -1, copyHash: from.Hash})
			put("终点 "+c.Hash+" · "+c.Subject, diffLine{kind: diffHeader, line: -1, hunk: -1, sec: -1, copyHash: c.Hash})
		} else {
			put(c.Subject, diffLine{kind: diffHeader, line: -1, hunk: -1})
			meta := fmt.Sprintf("%s · %s · %s", c.Author, c.Time.Format("2006-01-02 15:04"), c.Hash)
			var refs []string
			for _, ref := range c.Refs {
				refs = append(refs, ref.Name)
			}
			if len(refs) > 0 {
				meta += " · " + strings.Join(refs, ", ")
			}
			put(meta+"（回车或双击复制提交号）", diffLine{kind: diffHeader, line: -1, hunk: -1, sec: -1, copyHash: c.Hash})
			body := strings.Split(strings.TrimRight(c.Body, "\n"), "\n")
			if c.Body == "" {
				body = nil
			}
			folded := len(body) > commitBodyFold && !dt.bodyOpen
			for i, l := range body {
				if folded && i == commitBodyShown {
					put(fmt.Sprintf("⋯ 说明还有 %d 行（回车展开）", len(body)-i), diffLine{kind: diffFold, line: -1, hunk: -1, sec: -1, fold: -1, bodyToggle: true})
					break
				}
				put(l, diffLine{kind: diffHeader, line: -1, hunk: -1})
			}
			if len(body) > commitBodyFold && dt.bodyOpen {
				put("⋯ 收起说明", diffLine{kind: diffFold, line: -1, hunk: -1, sec: -1, fold: -1, bodyToggle: true})
			}
		}
	}
	if dt.loadError != "" {
		put("读取失败："+dt.loadError, diffLine{kind: diffHeader, line: -1, hunk: -1, sec: -1})
	} else if dt.loading && len(dt.secs) == 0 {
		put("（读取改动中…）", diffLine{kind: diffHeader, line: -1, hunk: -1})
	} else if len(dt.secs) == 0 {
		put("（没有改动）", diffLine{kind: diffHeader, line: -1, hunk: -1})
	}
	fold := 0
	for i := range dt.secs {
		s := &dt.secs[i]
		var base []string
		if s.baseOK {
			base = s.base
		}
		// 新加的、删掉的文件只有一边：末尾换行符后面那个空行不算一行（两边都有时它们抵消）。
		// 只在显示时去掉：s.cur 留着原样，好和编辑器里的文字比较、暂存时不丢换行符。
		cur := s.cur
		if len(base) == 0 {
			cur = trimLastEmpty(cur)
		}
		if len(cur) == 0 {
			base = trimLastEmpty(base)
		}
		s.hunks = e.diffHunks(base, cur)
		if s.group == changeConflict {
			s.hunks = nil
		}
		add, del := 0, 0
		for _, h := range s.hunks {
			add, del = add+h.BLen, del+h.ALen
		}
		rel := s.path
		if r != nil {
			rel = r.Rel(s.path)
		}
		mark := "▾"
		if dt.collapsed[s.key()] {
			mark = "▸"
		}
		if s.orig != "" && r != nil {
			rel = r.Rel(s.orig) + " → " + rel
		}
		letter := e.secLetter(s)
		if s.status != 0 {
			letter = rune(s.status)
		}
		head := fmt.Sprintf("%s %c %s  +%d −%d  %s  ", mark, letter, rel, add, del, s.group.label())
		fl := diffLine{kind: diffFile, line: -1, sec: i, hunk: -1}
		fl.barAt = len(head)
		fl.addB, fl.delB = statBar(add, del)
		head += strings.Repeat("■", statBarW)
		if s.binary {
			head, fl.barAt = fmt.Sprintf("%s %c %s  二进制文件  %s", mark, letter, rel, s.group.label()), -1
		}
		if s.lazy {
			head, fl.barAt = fmt.Sprintf("%s %c %s  大文件（%.1f MB）：展开时读取  %s", mark, letter, rel, float64(s.size)/(1<<20), s.group.label()), -1
		}
		if s.group == changeConflict {
			head, fl.barAt = fmt.Sprintf("%s %c %s  %s：回车打开三方合并", mark, letter, rel, s.group.label()), -1
		}
		put(head, fl)
		if dt.collapsed[s.key()] || s.group == changeConflict || s.binary || s.lazy {
			continue
		}
		if len(s.hunks) == 0 {
			put("（文字没有改动）", diffLine{kind: diffHeader, line: -1, sec: i, hunk: -1})
			continue
		}
		writeHunks(put, base, cur, s.hunks, i, &fold, dt.expanded, e.diffOptsFor(dt, s.path, cur))
	}
	txt := e.reviewDiffText(dt, strings.TrimSuffix(b.String(), "\n"))
	d.buf.Reload(text.FromString(txt, d.buf.Snapshot().Meta()))
}

// toggleChangesFile 折起或展开所有改动里的第 sec 个文件，光标留在它的标题上。
func (e *Editor) toggleChangesFile(t *tab, sec int) {
	dt := t.diff
	if !dt.changes || sec < 0 || sec >= len(dt.secs) {
		return
	}
	k := dt.secs[sec].key()
	dt.collapsed[k] = !dt.collapsed[k]
	if dt.secs[sec].lazy && !dt.collapsed[k] { // 没读的大文件：现在读
		e.loadChanges(t)
	}
	e.buildChanges(t.doc, dt)
	for i, l := range dt.lines {
		if l.kind == diffFile && l.sec == sec {
			e.diffGo(t, i)
			return
		}
	}
}

// diffGo 把改动视图的光标放到第 i 行，让它看得见。
func (e *Editor) diffGo(t *tab, i int) {
	t.diff.alignRight = true // 两窗格各自滚动时，键盘走到哪，右边也跟到哪
	s := t.doc.buf.Snapshot()
	i = max(min(i, s.LineCount()-1), 0)
	t.doc.buf.SetSelections(edit.Single(edit.Cursor(s.LineStart(i))))
	e.reveal()
}

// diffKey 在改动视图里把一个字当翻页器的键；不是这样的键时返回 false。
func (e *Editor) diffKey(r rune) bool {
	t := e.cur()
	if t == nil || t.diff == nil || e.area != areaEditor || e.ov != nil || e.find.open && e.findFocus != fieldNone {
		return false
	}
	dt := t.diff
	s := t.doc.buf.Snapshot()
	cur := s.LineAt(t.doc.buf.Selections().Main().Head)
	half := max(e.paneHeight()/2, 1)
	if r != ']' && r != '[' {
		dt.boundary = nil // 别的移动：再按 ] 先重新提示，不直接跨文件
	}
	switch r {
	case 'c':
		e.Run("review.comment")
	case 'y':
		e.Run("review.copyExcerpt")
	case 'n':
		e.Run("review.next")
	case 'N':
		e.Run("review.previous")
	case 'J':
		e.Run("cursorDownSelect")
	case 'K':
		e.Run("cursorUpSelect")
	case 'j':
		e.diffGo(t, cur+1)
	case 'k':
		e.diffGo(t, cur-1)
	case ']':
		e.diffStepChange(t, 1)
	case '[':
		e.diffStepChange(t, -1)
	case '.':
		e.diffStepFile(t, 1)
	case ',':
		e.diffStepFile(t, -1)
	case ' ', 'f':
		e.diffGo(t, cur+e.paneHeight())
	case 'b':
		e.diffGo(t, cur-e.paneHeight())
	case 'd':
		e.diffGo(t, cur+half)
	case 'u':
		e.diffGo(t, cur-half)
	case 'g':
		e.diffGo(t, 0)
	case 'G':
		e.diffGo(t, len(dt.lines)-1)
	case 'z':
		if cur < len(dt.lines) {
			switch l := dt.lines[cur]; {
			case l.kind == diffFold:
				dt.expanded[l.fold] = true
				e.refreshDiff(t.doc, dt)
			case dt.changes:
				e.toggleChangesFile(t, l.sec)
			}
		}
	case 'e':
		if err := e.diffOpen(t); err != nil {
			e.status = err.Error()
		}
	case 's':
		if err := e.diffStage(t, cur); err != nil {
			e.status = err.Error()
		}
	case 'i':
		if err := e.diffEditBeside(t); err != nil {
			e.status = err.Error()
		}
	case 'r', 'a', 'R':
		act := map[rune]blockAction{'r': blockRevert, 'a': blockAppend, 'R': blockAcceptRight}[r]
		if err := e.diffBlockAction(act); err != nil {
			e.status = err.Error()
		}
	default:
		return false
	}
	return true
}

// diffStage 是改动视图里的 s：在文件标题上暂存（已暂存的取消暂存）整个文件，在一处改动里只暂存（取消暂存）那一处。
func (e *Editor) diffStage(t *tab, line int) error {
	if t.diff.peer != "" {
		return errors.New("这是两个文件的比较：暂存不了")
	}
	defer e.bindDiff(t.diff)()
	if e.repository().git.repo == nil {
		return errors.New("原仓库已不可用")
	}
	dt := t.diff
	if line < 0 || line >= len(dt.lines) {
		return nil
	}
	l := dt.lines[line]
	if l.kind == diffReview {
		return errors.New("请把光标移到要暂存的代码行")
	}
	if dt.commit != nil {
		return errors.New("这是一个提交里的改动：暂存不了")
	}

	if !dt.changes {
		if l.hunk < 0 {
			return errors.New("光标放在一处改动里，再按 s")
		}
		return e.stageHunkOf(dt, l.hunk)
	}
	if l.sec < 0 || l.sec >= len(dt.secs) {
		return nil
	}
	sec := dt.secs[l.sec]
	if en, ok := e.repository().git.entries[sec.path]; ok && en.Submodule() {
		return e.stage([]string{sec.path}, sec.group == changeStaged)
	}
	// 标题行、改动外面的行、只有一边的文件（新加的、删掉的）：整个文件。
	if l.kind == diffFile || l.hunk < 0 || sec.group == changeUntracked || !sec.baseOK || len(sec.cur) == 0 {
		return e.stage([]string{sec.path}, sec.group == changeStaged)
	}
	h := sec.hunks[l.hunk]
	var out []string
	if sec.group == changeStaged { // 取消暂存这一处：索引里这几行换回 HEAD 里的
		out = append(append(append([]string{}, sec.cur[:h.B]...), sec.base[h.A:h.A+h.ALen]...), sec.cur[h.B+h.BLen:]...)
	} else { // 暂存这一处：索引里换成工作区的这几行
		out = append(append(append([]string{}, sec.base[:h.A]...), sec.cur[h.B:h.B+h.BLen]...), sec.base[h.A+h.ALen:]...)
	}
	path, unstage := sec.path, sec.group == changeStaged
	return e.gitDo("暂存中…", func(r *git.Repo) error { return r.StageText(path, strings.Join(out, "\n")) }, func(e *Editor) {
		if unstage {
			e.status = "已取消暂存这处改动"
		} else {
			e.status = "已暂存这处改动"
		}
	})
}

// stageHunkOf 在一个文件的改动视图里暂存第 k 处改动（相对 HEAD 算的）：和行号旁的暂存一样，按索引重新比较，
// 找和它重叠的那一处。
func (e *Editor) stageHunkOf(dt *diffTab, k int) error {
	defer e.bindDiff(dt)()
	cur, _ := e.currentLines(dt.path)
	var base []string
	if dt.hasBase {
		base = dt.base
	}
	hs := e.diffHunks(base, cur)
	if k < 0 || k >= len(hs) {
		return nil
	}
	first, last := hs[k].B, hs[k].B+max(hs[k].BLen, 1)-1
	path := dt.path
	return e.gitDo("暂存中…", func(r *git.Repo) error {
		idx, ok, err := r.Show("", path)
		if err != nil {
			return err
		}
		var lines []string
		if ok {
			lines = splitText(idx)
		}
		for _, ih := range linediff.Hunks(lines, cur) {
			a, z := ih.B, ih.B+max(ih.BLen, 1)-1
			if z < first || a > last {
				continue
			}
			out := append(append(append([]string{}, lines[:ih.A]...), cur[ih.B:ih.B+ih.BLen]...), lines[ih.A+ih.ALen:]...)
			return r.StageText(path, strings.Join(out, "\n"))
		}
		return errors.New("这处改动已经暂存了")
	}, func(e *Editor) { e.status = "已暂存这处改动" })
}

// ---- 提交的标签（照着 Mayoi 的 commit tab）：说明、作者、日期、提交号、分支，再是它改的每个文件 ----

// openCommitTab 在标签里显示历史里的第 i 个提交；preview 为真时用预览标签，键盘留在列表里。
func (e *Editor) openCommitTab(i int, preview bool) error {
	r := e.repository().git.repo
	if r == nil || i < 0 || i >= len(e.repository().scm.history.commits) {
		return nil
	}
	e.showHistoryTab("", e.repository().scm.history.commits[i].Hash, preview)
	return nil
}

func (e *Editor) showHistoryTab(from, to string, preview bool) *tab {
	root := e.root
	if r := e.repository().git.repo; r != nil {
		root = r.Root
	}
	return e.showHistoryTabAt(root, from, to, preview)
}
func (e *Editor) showHistoryTabAt(root, from, to string, preview bool) *tab {
	name, identity := "提交 "+git.ShortHash(to), to
	dt := &diffTab{repoRoot: root, changes: true, commit: &git.Commit{Hash: to}, expanded: map[int]bool{}, collapsed: map[string]bool{}}
	if from != "" {
		dt.compareFrom = &git.Commit{Hash: from}
		name, identity = "比较 "+git.ShortHash(from)+" → "+git.ShortHash(to), from+".."+to
	}
	// Full hashes identify virtual tabs; short display names can collide.
	dt.path = filepath.Join(root, ".yan-history", identity, "changes")
	e.showDiff(name, dt, preview)
	t := e.cur()
	if t.diff == dt {
		e.loadCommit(t)
	}
	return t
}

// Both revisions are resolved before opening; HEAD changes never retarget a comparison.
func (e *Editor) loadCommit(t *tab) {
	// Legacy sessions used the opened folder, which may be inside a worktree.
	if t.diff.repoRoot == e.root && e.git.repo != nil && e.git.repo.Contains(e.root) {
		t.diff.repoRoot = e.git.repo.Root
	}
	defer e.bindDiff(t.diff)()
	dt, r := t.diff, e.repository().git.repo
	if dt.loading {
		return
	}
	if r == nil {
		// Session tabs are restored before background repository discovery finishes.
		dt.awaitRepo, dt.loading = !e.repository().git.ready, !e.repository().git.ready
		if e.repository().git.ready {
			dt.loadError = "此文件夹已不在 Git 仓库中"
		}
		e.buildChanges(t.doc, dt)
		return
	}
	dt.loading, dt.loadError = true, ""
	to, from := dt.commit.Hash, ""
	if dt.compareFrom != nil {
		from = dt.compareFrom.Hash
	}
	e.buildChanges(t.doc, dt)
	post := e.postGit()
	go func() {
		c, err := r.CommitInfo(to)
		var baseCommit *git.Commit
		base := ""
		if from != "" && err == nil {
			var b git.Commit
			b, err = r.CommitInfo(from)
			baseCommit, base = &b, b.Hash
		} else if len(c.Parents) > 0 {
			base = c.Parents[0]
		}
		var files []git.CommitFile
		if err == nil {
			pair := c
			if from != "" {
				pair.Parents = []string{base}
			}
			files, err = r.CommitFiles(pair)
		}
		var secs []changeSec
		for _, f := range files {
			if err != nil {
				break
			}
			s := changeSec{path: f.Path, orig: f.Orig, status: f.Status, group: changeCommit}
			if from != "" {
				s.group = changeCompare
			}
			read := func(rev, path string) ([]string, bool) {
				if err != nil {
					return nil, false
				}
				var txt string
				var ok bool
				txt, ok, err = r.Show(rev, path)
				if err == nil && !ok {
					err = fmt.Errorf("无法读取 %s 中的 %s", git.ShortHash(rev), r.Rel(path))
				}
				if strings.ContainsRune(txt, 0) {
					s.binary = true
					return nil, ok
				}
				return splitText(txt), ok
			}
			if base != "" && f.Status != 'A' {
				s.base, s.baseOK = read(base, cmpOr(f.Orig, f.Path))
			}
			if f.Status != 'D' {
				s.cur, _ = read(c.Hash, f.Path)
			}
			if s.binary {
				s.base, s.cur = nil, nil
			}
			secs = append(secs, s)
		}
		post(func(e *Editor) {
			if e.repository().git.repo != r {
				return
			}
			dt.loading = false
			if err != nil {
				dt.loadError = gitError(err)
				e.status = dt.loadError
			} else {
				dt.commit, dt.compareFrom, dt.secs = &c, baseCommit, secs
			}
			e.buildChanges(t.doc, dt)
			if st := dt.restore; st != nil {
				n := t.doc.buf.Snapshot().Len()
				t.sels = edit.Single(edit.Range{Anchor: max(0, min(st.Anchor, n)), Head: max(0, min(st.Head, n)), GoalX: -1})
				t.doc.buf.SetSelections(t.sels)
				t.scrollY = max(0, min(st.ScrollY, t.doc.buf.Snapshot().LineCount()-1))
				t.wrap = min(max(st.Wrap, -1), 1)
				dt.restore = nil
			}
			if path := dt.revealFile; path != "" {
				dt.revealFile = ""
				e.revealCommitFile(t, path)
			}
		})
	}()
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// SCMCommitActions 打开历史里第 i 行那个提交能做的事（右键：检出、合并、变基、重置、还原、拣选、建分支、打标签）。
func (e *Editor) SCMCommitActions(i int) {
	e.sync()
	defer e.sync()
	e.side, e.area, e.repository().scm.sel, e.repository().scm.focusMsg = sideSCM, areaTree, i, false
	if k, ok := e.scmSelectedCommit(); ok {
		e.openCommit(k)
	}
}

// commitBodyFold 是提交说明超过几行时折起来，只显示前 commitBodyShown 行（长说明不占满改动视图）。
const (
	commitBodyFold  = 8
	commitBodyShown = 5
)

// statBarW 是所有改动里文件标题后面的增减条有几格（照着 GitHub）。
const statBarW = 5

// statBar 是加了 add 行、删了 del 行的增减条里加的和删的各几格：按比例，有改动的那边至少一格。
func statBar(add, del int) (int, int) {
	n := add + del
	if n == 0 {
		return 0, 0
	}
	w := min(statBarW, n)
	a := (add*w + n/2) / n
	if add > 0 && a == 0 {
		a = 1
	}
	if del > 0 && a == w {
		a = w - 1
	}
	return a, w - a
}

// secLetter 是所有改动里一个文件的状态字母：同源代码管理（已暂存的看索引，其余看工作区）。
func (e *Editor) secLetter(s *changeSec) rune {
	switch s.group {
	case changeConflict:
		return 'C'
	case changeUntracked:
		return 'U'
	}
	en, ok := e.repository().git.entries[s.path]
	switch {
	case !ok:
		return 'M'
	case s.group == changeStaged && en.X != '.':
		return rune(en.X)
	case en.Y != '.':
		return rune(en.Y)
	}
	return rune(en.Letter())
}
