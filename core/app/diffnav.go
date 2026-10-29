package app

// Change and file navigation in the diffs with boundaries, after the behaviour of Mayoi PR #50
// (Garfielk, https://github.com/sorrycc/Mayoi/pull/50, unmerged): at the last (or first)
// change of a file the diff says so, and one more press goes on to the next file with changes
// ([diff] cross_files may cross at once, or never); at the ends of the diff it says so and
// stays. It never wraps round without a word. Rewritten for yan's diff tabs.

import (
	"fmt"
	"path/filepath"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/view"
)

// diffBoundary is an end a diff's change navigation said it reached: at line, going by.
// One more step the same way from the same line crosses it.
type diffBoundary struct{ line, by int }

// diffHeads are the lines of a diff tab's changes (their @@ headers).
func diffHeads(dt *diffTab) []int {
	var out []int
	for i, l := range dt.lines {
		if l.kind == diffHeader && l.hunk >= 0 {
			out = append(out, i)
		}
	}
	return out
}

// diffName is the name of the file a line of a diff tab is in.
func (dt *diffTab) nameAt(i int) string {
	if i >= 0 && i < len(dt.lines) {
		return filepath.Base(dt.pathOf(dt.lines[i]))
	}
	return filepath.Base(dt.path)
}

// diffStepChange goes to the next (by 1) or previous (by -1) change of the diff tab t.
func (e *Editor) diffStepChange(t *tab, by int) {
	dt := t.diff
	cur := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	armed := dt.boundary != nil && *dt.boundary == diffBoundary{cur, by}
	dt.boundary = nil
	heads := diffHeads(dt)
	to := -1
	if by > 0 {
		for _, h := range heads {
			if h > cur {
				to = h
				break
			}
		}
	} else {
		for i := len(heads) - 1; i >= 0; i-- {
			if heads[i] < cur {
				to = heads[i]
				break
			}
		}
	}
	sec := -1
	if cur < len(dt.lines) {
		sec = dt.lines[cur].sec
	}
	switch {
	case to >= 0 && (!dt.changes || sec < 0 || dt.lines[to].sec == sec):
		e.diffGoChange(t, to)
	case to >= 0: // the next change is another file's
		if e.diffCross(dt, cur, by, armed, dt.nameAt(cur), dt.nameAt(to)) {
			e.diffGoChange(t, to)
		}
	default:
		next := e.diffNextFile(dt, by)
		if next == "" {
			e.status = diffEndMessage(by, len(heads) == 0)
			return
		}
		if e.diffCross(dt, cur, by, armed, dt.nameAt(cur), filepath.Base(next)) {
			e.diffCrossTo(t, next, by)
		}
	}
}

// diffCross reports whether a step across into another file goes on now; otherwise it says
// where the navigation is, arming the next press when the settings ask for one.
func (e *Editor) diffCross(dt *diffTab, cur, by int, armed bool, from, to string) bool {
	mode := e.cfg.DiffCrossFiles
	if mode == "always" || mode == "" && armed {
		return true
	}
	where := "最后一"
	if by < 0 {
		where = "第一"
	}
	e.status = fmt.Sprintf("已是 %s 的%s处改动", from, where)
	if mode == "" {
		dt.boundary = &diffBoundary{cur, by}
		e.status += "，再按一次到 " + to
	}
	return false
}

func diffEndMessage(by int, none bool) string {
	switch {
	case none:
		return "没有改动"
	case by > 0:
		return "已是改动视图的最后一处改动"
	}
	return "已是改动视图的第一处改动"
}

// diffGoChange puts the caret on the change at line i and says which of how many it is.
func (e *Editor) diffGoChange(t *tab, i int) {
	t.diff.glide = true // 两窗格：滑过去
	e.diffGo(t, i)
	heads := diffHeads(t.diff)
	for k, h := range heads {
		if h == i {
			e.status = fmt.Sprintf("第 %d/%d 处改动", k+1, len(heads))
			if t.diff.changes {
				e.status += " · " + t.diff.nameAt(i)
			}
		}
	}
}

// diffNextFile is the changed file a one-file diff of the work tree goes on to: the next (or
// previous) one in Source Control's list. Comparisons and commits have none.
func (e *Editor) diffNextFile(dt *diffTab, by int) string {
	if dt.changes || dt.fixed || dt.commit != nil || dt.peer != "" {
		return ""
	}
	defer e.bindDiff(dt)()
	var paths []string
	seen := map[string]bool{}
	for _, g := range e.scmAllGroups() {
		for _, r := range g.Rows {
			if !r.Folder && !seen[r.Path] {
				if en, ok := e.repository().git.entries[r.Path]; ok && en.Submodule() {
					continue
				}
				seen[r.Path] = true
				paths = append(paths, r.Path)
			}
		}
	}
	for i, p := range paths {
		if p == dt.path {
			if j := i + by; j >= 0 && j < len(paths) {
				return paths[j]
			}
			return ""
		}
	}
	return ""
}

// diffCrossTo opens the diff of the file path, at its first change (by 1) or last (by -1).
func (e *Editor) diffCrossTo(t *tab, path string, by int) {
	defer e.bindDiff(t.diff)()
	if err := e.openChange(path, t.preview); err != nil {
		e.status = err.Error()
		return
	}
	nt := e.cur()
	if nt == nil || nt.diff == nil {
		return
	}
	if heads := diffHeads(nt.diff); len(heads) > 0 {
		at := heads[0]
		if by < 0 {
			at = heads[len(heads)-1]
		}
		e.diffGoChange(nt, at)
	}
}

// diffStepFile goes to the next (or previous) file of the diff: in the tab of all the changes
// its next file's header, in a one-file diff the next changed file's diff. At the first and
// last file it says so.
func (e *Editor) diffStepFile(t *tab, by int) {
	dt := t.diff
	cur := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	dt.boundary = nil
	if dt.changes {
		for i := cur + by; i >= 0 && i < len(dt.lines); i += by {
			if dt.lines[i].kind == diffFile {
				dt.glide = true
				e.diffGo(t, i)
				return
			}
		}
	} else if next := e.diffNextFile(dt, by); next != "" {
		e.diffCrossTo(t, next, 1)
		return
	}
	if by > 0 {
		e.status = "已是最后一个文件"
	} else {
		e.status = "已是第一个文件"
	}
}

// diffTabNav runs a diff's navigation on the diff tab being edited.
func (e *Editor) diffTabNav(step func(*Editor, *tab, int), by int) error {
	t := e.cur()
	if t == nil || t.diff == nil {
		return fmt.Errorf("先打开一个改动视图")
	}
	step(e, t, by)
	return nil
}

// diffStickyLine is the header of the file at the top of the tab of all the changes, when it
// has scrolled out of sight: it stays pinned there with its buttons. -1 when none.
func diffStickyLine(t *tab) int {
	dt := t.diff
	if !dt.changes || t.scrollY <= 0 || t.scrollY >= len(dt.lines) {
		return -1
	}
	for i := t.scrollY; i >= 0; i-- {
		if dt.lines[i].kind == diffFile {
			if i == t.scrollY {
				return -1 // the header is the top line itself
			}
			return i
		}
	}
	return -1
}

// toggleFullFiles shows the whole files in the diff being edited, or only the lines near the
// changes again.
func (e *Editor) toggleFullFiles() error {
	t := e.cur()
	if t == nil || t.diff == nil {
		return fmt.Errorf("先打开一个改动视图")
	}
	t.diff.full = !t.diff.full
	clear(t.diff.expanded)
	e.refreshDiff(t.doc, t.diff)
	if t.diff.full {
		e.status = "改动视图：显示整个文件"
	} else {
		e.status = "改动视图：只显示改动附近"
	}
	return nil
}

// shownChanges is the tab of all the changes of the repository bound, when a column shows it.
func (e *Editor) shownChanges() (*column, *tab) {
	r := e.repository().git.repo
	if r == nil {
		return nil, nil
	}
	for _, c := range e.cols {
		if t := c.tab(); t != nil && t.diff != nil && t.diff.changes && t.diff.commit == nil && t.diff.repoRoot == r.Root {
			return c, t
		}
	}
	return nil, nil
}

// changesLine is the header line of Source Control's row r in the tab of all the changes.
func changesLine(dt *diffTab, r view.SCMRow) int {
	for i, l := range dt.lines {
		if l.kind == diffFile && l.sec >= 0 && l.sec < len(dt.secs) {
			s := dt.secs[l.sec]
			if s.path == r.Path && (s.group == changeStaged) == r.Staged {
				return i
			}
		}
	}
	return -1
}

// revealInChanges scrolls the tab of all the changes shown to the file of Source Control's row r.
func (e *Editor) revealInChanges(r view.SCMRow) bool {
	_, t := e.shownChanges()
	if t == nil {
		return false
	}
	line := changesLine(t.diff, r)
	if line < 0 {
		return false
	}
	if t.diff.collapsed[t.diff.secs[t.diff.lines[line].sec].key()] {
		e.toggleChangesFile(t, t.diff.lines[line].sec)
		line = changesLine(t.diff, r)
	}
	t.scrollY, t.diff.syncedTop = line, line
	s := t.doc.buf.Snapshot()
	t.doc.buf.SetSelections(edit.Single(edit.Cursor(s.LineStart(min(line, s.LineCount()-1)))))
	t.sels = t.doc.buf.Selections()
	return true
}

// followChangesScroll selects in Source Control the file at the top of the tab of all the
// changes once it has scrolled.
func (e *Editor) followChangesScroll(t *tab) {
	dt := t.diff
	if dt.commit != nil || t.scrollY == dt.syncedTop || t.scrollY >= len(dt.lines) {
		return
	}
	dt.syncedTop = t.scrollY
	defer e.bindDiff(dt)()
	sec := -1
	for i := t.scrollY; i >= 0 && sec < 0; i-- {
		if dt.lines[i].sec >= 0 && dt.lines[i].kind != diffReview {
			sec = dt.lines[i].sec
		}
	}
	if sec < 0 || sec >= len(dt.secs) {
		return
	}
	s := dt.secs[sec]
	for i, r := range scmFlat(e.scmGroups()) {
		if !r.Folder && r.Path == s.path && r.Staged == (s.group == changeStaged) {
			if sc := e.repository().scm; sc.sel != i && !(e.area == areaTree && e.side == sideSCM) {
				sc.sel = i
				sc.picked = nil
			}
			return
		}
	}
}

// diffPlace is a row of a diff as it stands for a file: which file, what kind of row, and its
// line on the new side (the old side's for a removed line).
type diffPlace struct {
	sec  int
	kind diffKind
	line int
	ok   bool
}

func placeOf(dt *diffTab, i int) diffPlace {
	if i < 0 || i >= len(dt.lines) {
		return diffPlace{}
	}
	l := dt.lines[i]
	return diffPlace{sec: l.sec, kind: l.kind, line: l.line, ok: true}
}

// findPlace is the row of p in the diff read again: the same row, or the first of the same file
// at or past its line, or its file's header.
func findPlace(dt *diffTab, p diffPlace) int {
	if !p.ok {
		return -1
	}
	near := -1
	for i, l := range dt.lines {
		if l.sec != p.sec {
			continue
		}
		if l.kind == p.kind && l.line == p.line {
			return i
		}
		if near < 0 && (l.kind == diffFile || l.line >= p.line && p.line >= 0 && l.kind != diffRemoved) {
			near = i
		}
	}
	return near
}

// keepDiffPlace keeps the line at the top of each tab showing the diff d, and the caret's,
// while it is read again: folding, unfolding, wrapping or a new width move no line away from
// the view (after Mayoi v0.11.5).
func (e *Editor) keepDiffPlace(d *document, dt *diffTab) func() {
	type place struct {
		t          *tab
		top, caret diffPlace
	}
	var ps []place
	for _, t := range d.tabs {
		if t.diff != dt || len(dt.lines) == 0 {
			continue
		}
		s := t.doc.buf.Snapshot()
		caret := s.LineAt(t.doc.buf.Selections().Main().Head)
		ps = append(ps, place{t: t, top: placeOf(dt, t.scrollY), caret: placeOf(dt, caret)})
	}
	return func() {
		for _, p := range ps {
			if p.t.scrollY > 0 {
				if i := findPlace(dt, p.top); i >= 0 {
					p.t.scrollY = i
				}
			}
			if i := findPlace(dt, p.caret); i >= 0 && i != p.t.doc.buf.Snapshot().LineAt(p.t.doc.buf.Selections().Main().Head) {
				s := p.t.doc.buf.Snapshot()
				p.t.doc.buf.SetSelections(edit.Single(edit.Cursor(s.LineStart(min(i, s.LineCount()-1)))))
				if p.t != e.loaded {
					p.t.sels = p.t.doc.buf.Selections()
				}
			}
		}
	}
}
