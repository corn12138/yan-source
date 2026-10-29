package app

// The three-way merge view, after the behaviour of Mayoi PR #50 (Garfielk,
// https://github.com/sorrycc/Mayoi/pull/50, unmerged): a conflicted file opens as three
// columns, ours and theirs read only on either side of the result, which is the file's own
// document and starts from the common ancestor with the changes no conflict holds already in.
// Each conflict takes either side, both, or neither (ignored, which counts as handled); undo
// and redo bring the choice back with the text. Mark Resolved saves and stages the file;
// Restore Conflict puts it back in conflict (git checkout -m). Rewritten for yan's columns.

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/syntax/highlight"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

const (
	ignoredOurs   uint8 = 1
	ignoredTheirs uint8 = 2
)

// mergeView is a conflicted file being merged.
type mergeView struct {
	path, root               string
	base, ours, theirs       []string
	oursName, theirsName     string
	regions                  []mergeRegion
	conflicts                []int // the regions that are conflicts
	ignored                  map[int]uint8
	log, redo                []mergeStep
	result                   *tab
	oursTab, theirsTab       *tab
	ver                      uint64 // the result's version the ranges below were worked out for
	ranges                   [][2]int
	lines                    []string
	syncedTop, syncedResultV int
}

// mergeStep is a choice made in the merge view: the sides ignored before and after it, and
// the versions of the result's text before and after (the same when it changed no text).
type mergeStep struct {
	before, after map[int]uint8
	from, to      uint64
	text          bool
}

// mergeSide is a read-only side of a merge view, in its own column.
type mergeSide struct {
	mv     *mergeView
	theirs bool
}

// mergeHeader is how many lines the sides' documents have above the side's text.
const mergeHeader = 1

// openMerge opens the merge view of the conflicted file path.
func (e *Editor) openMerge(path string) error {
	defer e.bindRepository(e.repositoryForPath(path))()
	g := e.repository().git
	if g.repo == nil {
		return errors.New("不在 git 仓库里")
	}
	for _, c := range e.cols {
		for i, t := range c.tabs {
			if t.merge != nil && t.merge.path == path {
				e.activate(c, i)
				return nil
			}
		}
	}
	if en, ok := g.entries[path]; !ok || !en.Conflicted() {
		return errors.New(filepath.Base(path) + " 没有冲突")
	}
	r := g.repo
	base, _, err := r.ConflictStage(1, path)
	if err != nil {
		return err
	}
	ours, okOurs, err := r.ConflictStage(2, path)
	if err != nil {
		return err
	}
	theirs, okTheirs, err := r.ConflictStage(3, path)
	if err != nil {
		return err
	}
	if !okOurs || !okTheirs {
		return errors.New("一边删掉了 " + filepath.Base(path) + "：用暂存（留下）或删除来解决")
	}
	mv := &mergeView{path: path, root: r.Root, ignored: map[int]uint8{}, base: splitText(base), ours: splitText(ours), theirs: splitText(theirs)}
	if base == "" {
		mv.base = nil
	}
	mv.oursName, mv.theirsName = "HEAD", "合并进来的"
	if st := g.status; st != nil {
		if st.Branch != "" {
			mv.oursName = st.Branch
		}
		if name := r.OpHeadName(st.Op); name != "" {
			mv.theirsName = name
		}
	}
	mv.regions = mergeRegions(mv.base, mv.ours, mv.theirs)
	for i, rg := range mv.regions {
		if rg.conflict {
			mv.conflicts = append(mv.conflicts, i)
		}
	}
	c := e.fileColumn()
	if err := e.openIn(c, c.active+1, path, false); err != nil {
		return err
	}
	rt := e.cur()
	rt.merge, mv.result = mv, rt
	d := rt.doc
	if hasConflictMarkers(d.buf.Snapshot()) { // 还是 git 写的冲突标记：从共同祖先重新开始
		s := d.buf.Snapshot()
		start := strings.Join(mv.startLines(!e.mergeManual), eolOf(s))
		d.buf.BreakGroup()
		d.buf.Apply(edit.Transaction{Changes: text.ChangeSet{{Start: 0, End: s.Len(), Text: start}}, Origin: edit.OriginCommand})
		d.buf.BreakGroup()
		d.buf.SetSelections(edit.Single(edit.Cursor(0)))
	}
	ci, w := slices.Index(e.cols, c), c.weight
	oc := e.splitColumn(ci, true)
	mv.oursTab = e.mergeSideTab(oc, mv, false)
	tc := e.splitColumn(ci+1, false)
	mv.theirsTab = e.mergeSideTab(tc, mv, true)
	oc.weight, c.weight, tc.weight = w/3, w/3, w/3 // 三栏一样宽
	e.activate(c, slices.Index(c.tabs, rt))
	e.mergeSync(mv)
	if k := mv.nextUnresolved(-1, 1); k >= 0 {
		e.mergeGo(mv, k)
	}
	e.status = mv.summary()
	return nil
}

// hasConflictMarkers reports whether s still has the markers git writes for a conflict.
func hasConflictMarkers(s text.Snapshot) bool {
	txt := s.String(0, s.Len())
	return strings.Contains(txt, "<<<<<<< ") && strings.Contains(txt, "\n=======") && strings.Contains(txt, ">>>>>>> ")
}

// mergeSideTab puts a read-only side of mv in column c.
func (e *Editor) mergeSideTab(c *column, mv *mergeView, theirs bool) *tab {
	lines, head := mv.ours, "我们的 · "+mv.oursName+"（当前分支，只读）"
	name := filepath.Base(mv.path) + " · 我们的"
	if theirs {
		lines, head = mv.theirs, "他们的 · "+mv.theirsName+"（合并进来的，只读）"
		name = filepath.Base(mv.path) + " · 他们的"
	}
	vpath := filepath.Join(filepath.Dir(mv.path), name)
	d := &document{buf: edit.NewBuffer(text.FromString(head+"\n"+strings.Join(lines, "\n"), text.Meta{Path: vpath, ReadOnly: true}))}
	if lx := highlight.Lexer(mv.path); lx != nil {
		d.hl = highlight.New(lx, func() { e.Post(func(*Editor) {}) })
	}
	t := &tab{doc: d, sels: edit.Single(edit.Cursor(0)), mergeSide: &mergeSide{mv: mv, theirs: theirs}}
	d.tabs = append(d.tabs, t)
	e.docs = append(e.docs, d)
	c.tabs = append(c.tabs, t)
	c.active = len(c.tabs) - 1
	return t
}

// mergeSync works out where the regions stand in the result, once per version of its text.
func (e *Editor) mergeSync(mv *mergeView) {
	s := mv.result.doc.buf.Snapshot()
	if mv.ranges != nil && s.Version() == mv.ver {
		return
	}
	mv.ver = s.Version()
	mv.lines = splitText(s.String(0, s.Len()))
	mv.ranges = resultRanges(mv.base, mv.lines, mv.regions)
}

// state is how conflict k (an index into conflicts) stands.
func (mv *mergeView) state(k int) regionState {
	i := mv.conflicts[k]
	rg := mv.ranges[i]
	return stateOf(mv.base, mv.ours, mv.theirs, mv.lines[rg[0]:rg[1]], mv.regions[i], mv.ignored[i])
}

func (mv *mergeView) unresolved() int {
	n := 0
	for k := range mv.conflicts {
		if mv.state(k) == regionUndecided {
			n++
		}
	}
	return n
}

// nextUnresolved is the next unresolved conflict from k the way by goes, -1 when none.
func (mv *mergeView) nextUnresolved(k, by int) int {
	for i := k + by; i >= 0 && i < len(mv.conflicts); i += by {
		if mv.state(i) == regionUndecided {
			return i
		}
	}
	return -1
}

func (mv *mergeView) summary() string {
	n := mv.unresolved()
	switch {
	case len(mv.conflicts) == 0:
		return filepath.Base(mv.path) + "：两边的改动没有冲突，已经合进结果；保存后标记为已解决"
	case n == 0:
		return fmt.Sprintf("%s：%d 处冲突都处理了，可以标记为已解决", filepath.Base(mv.path), len(mv.conflicts))
	}
	return fmt.Sprintf("%s：%d 处冲突，还有 %d 处没处理", filepath.Base(mv.path), len(mv.conflicts), n)
}

// mergeOf is the merge view of the tab t, of its result or a side.
func mergeOf(t *tab) *mergeView {
	switch {
	case t == nil:
		return nil
	case t.merge != nil:
		return t.merge
	case t.mergeSide != nil:
		return t.mergeSide.mv
	}
	return nil
}

// mergeCurrent is the merge view of the tab being edited and its conflict at the caret (or
// the first unresolved one, or the first).
func (e *Editor) mergeCurrent() (*mergeView, int, error) {
	t := e.cur()
	mv := mergeOf(t)
	if mv == nil {
		return nil, -1, errors.New("先打开一个冲突文件的三方合并")
	}
	e.mergeSync(mv)
	if len(mv.conflicts) == 0 {
		return mv, -1, errors.New("这个文件没有冲突要处理")
	}
	line := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	for k, i := range mv.conflicts {
		lo, hi := mv.ranges[i][0], mv.ranges[i][1]
		if t.mergeSide != nil {
			lo, hi = mv.sideRange(i, t.mergeSide.theirs)
			lo, hi = lo+mergeHeader, hi+mergeHeader
		}
		if line >= lo && line < max(hi, lo+1) {
			return mv, k, nil
		}
	}
	if k := mv.nextUnresolved(-1, 1); k >= 0 {
		return mv, k, nil
	}
	return mv, 0, nil
}

func (mv *mergeView) sideRange(i int, theirs bool) (int, int) {
	r := mv.regions[i]
	if theirs {
		return r.theirsLo, r.theirsHi
	}
	return r.oursLo, r.oursHi
}

// mergeAct makes a choice on mv: set changes the text of conflict regions (index → lines),
// flags the sides ignored; both are one step that undo and redo take back.
func (e *Editor) mergeAct(mv *mergeView, set map[int][]string, flags func(map[int]uint8)) {
	e.mergeSync(mv)
	d := mv.result.doc
	step := mergeStep{before: maps.Clone(mv.ignored), from: d.buf.Snapshot().Version()}
	if len(set) > 0 {
		s := d.buf.Snapshot()
		var cs text.ChangeSet
		for _, i := range slices.Sorted(maps.Keys(set)) {
			rg := mv.ranges[i]
			cs = append(cs, replaceLines(s, rg[0], rg[1]-rg[0], set[i]))
		}
		d.buf.BreakGroup()
		d.buf.Apply(edit.Transaction{Changes: cs, Origin: edit.OriginCommand})
		d.buf.BreakGroup()
		step.text = true
	}
	flags(mv.ignored)
	step.after, step.to = maps.Clone(mv.ignored), d.buf.Snapshot().Version()
	mv.log, mv.redo = append(mv.log, step), nil
	e.mergeSync(mv)
	e.mergeTell(mv)
}

// mergeTell says how the merge stands, with a Mark Resolved button once every conflict is
// handled.
func (e *Editor) mergeTell(mv *mergeView) {
	if len(mv.conflicts) > 0 && mv.unresolved() == 0 {
		e.tell(mv.summary(), "标记为已解决", "yan.merge.markResolved")
		return
	}
	e.status = mv.summary()
}

// mergeTake puts a side's lines of conflict k in the result: in place of the base's (or the
// other side's ignored), or after the other side's when that is in already.
func (e *Editor) mergeTake(mv *mergeView, k int, theirs bool) {
	i := mv.conflicts[k]
	r := mv.regions[i]
	o, t := mv.ours[r.oursLo:r.oursHi], mv.theirs[r.theirsLo:r.theirsHi]
	side, bit := o, ignoredOurs
	if theirs {
		side, bit = t, ignoredTheirs
	}
	lines := slices.Clone(side)
	switch st := mv.state(k); {
	case st == regionOurs && theirs, st == regionTheirs && !theirs:
		rg := mv.ranges[i]
		lines = append(slices.Clone(mv.lines[rg[0]:rg[1]]), side...)
	case st == regionBoth, st == regionOurs && !theirs, st == regionTheirs && theirs:
		return
	}
	e.mergeAct(mv, map[int][]string{i: lines}, func(m map[int]uint8) { m[i] &^= bit })
}

// mergeIgnore leaves a side of conflict k out: its lines come out of the result if they are in,
// and the conflict counts as handled.
func (e *Editor) mergeIgnore(mv *mergeView, k int, theirs bool) {
	i := mv.conflicts[k]
	r := mv.regions[i]
	bit := ignoredOurs
	if theirs {
		bit = ignoredTheirs
	}
	set := map[int][]string{}
	switch st := mv.state(k); {
	case st == regionBoth && theirs:
		set[i] = slices.Clone(mv.ours[r.oursLo:r.oursHi])
	case st == regionBoth:
		set[i] = slices.Clone(mv.theirs[r.theirsLo:r.theirsHi])
	case st == regionOurs && !theirs, st == regionTheirs && theirs:
		set[i] = slices.Clone(mv.base[r.baseLo:r.baseHi])
	}
	e.mergeAct(mv, set, func(m map[int]uint8) { m[i] |= bit })
}

// mergeResolveUsing takes one side for every conflict, as one step.
func (e *Editor) mergeResolveUsing(mv *mergeView, theirs bool) {
	set := map[int][]string{}
	for _, i := range mv.conflicts {
		r := mv.regions[i]
		if theirs {
			set[i] = slices.Clone(mv.theirs[r.theirsLo:r.theirsHi])
		} else {
			set[i] = slices.Clone(mv.ours[r.oursLo:r.oursHi])
		}
	}
	e.mergeAct(mv, set, func(m map[int]uint8) { clear(m) })
}

// mergeHistory is undo (or redo) in a merge view: a choice made there is taken back with the
// sides it ignored; typing in the result undoes as anywhere.
func (e *Editor) mergeHistory(mv *mergeView, undo bool) {
	d := mv.result.doc
	v := d.buf.Snapshot().Version()
	var ok bool
	switch {
	case undo && len(mv.log) > 0 && mv.log[len(mv.log)-1].to == v:
		st := mv.log[len(mv.log)-1]
		mv.log, mv.redo = mv.log[:len(mv.log)-1], append(mv.redo, st)
		if st.text {
			d.buf.Undo()
		}
		mv.ignored, ok = maps.Clone(st.before), true
	case !undo && len(mv.redo) > 0 && mv.redo[len(mv.redo)-1].from == v:
		st := mv.redo[len(mv.redo)-1]
		mv.redo, mv.log = mv.redo[:len(mv.redo)-1], append(mv.log, st)
		if st.text {
			d.buf.Redo()
		}
		mv.ignored, ok = maps.Clone(st.after), true
	case undo:
		_, ok = d.buf.Undo()
	default:
		_, ok = d.buf.Redo()
	}
	if !ok {
		e.status = map[bool]string{true: "没有可撤销的操作", false: "没有可重做的操作"}[undo]
		return
	}
	e.mergeSync(mv)
	e.mergeTell(mv)
}

// mergeGo shows conflict k in the three columns, the caret of the result at its start.
func (e *Editor) mergeGo(mv *mergeView, k int) {
	e.mergeSync(mv)
	if k < 0 || k >= len(mv.conflicts) {
		return
	}
	i := mv.conflicts[k]
	s := mv.result.doc.buf.Snapshot()
	line := min(mv.ranges[i][0], s.LineCount()-1)
	mv.result.doc.buf.SetSelections(edit.Single(edit.Cursor(s.LineStart(line))))
	mv.result.sels = mv.result.doc.buf.Selections()
	mv.result.scrollY = max(line-3, 0)
	for _, st := range []*tab{mv.oursTab, mv.theirsTab} {
		if st == nil {
			continue
		}
		lo, _ := mv.sideRange(i, st == mv.theirsTab)
		ss := st.doc.buf.Snapshot()
		at := min(lo+mergeHeader, ss.LineCount()-1)
		st.sels = edit.Single(edit.Cursor(ss.LineStart(at)))
		st.doc.buf.SetSelections(st.sels)
		st.scrollY = max(at-3, 0)
	}
	e.status = fmt.Sprintf("冲突 %d/%d：%s · %s", k+1, len(mv.conflicts), mv.state(k).label(), mv.summary())
}

// mergeStepConflict goes to the next (or previous) conflict, unresolved ones first.
func (e *Editor) mergeStepConflict(by int) error {
	mv, k, err := e.mergeCurrent()
	if err != nil {
		return err
	}
	t := e.cur()
	line := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	at := func(k int) int {
		i := mv.conflicts[k]
		if t.mergeSide != nil {
			lo, _ := mv.sideRange(i, t.mergeSide.theirs)
			return lo + mergeHeader
		}
		return mv.ranges[i][0]
	}
	for n := 0; n < len(mv.conflicts); n++ {
		j := k + by*(n+1)
		if by > 0 && k >= 0 && at(k) > line && n == 0 { // the caret is above conflict k
			j = k
		}
		if j < 0 || j >= len(mv.conflicts) {
			break
		}
		if by > 0 && at(j) > line || by < 0 && at(j) < line {
			e.mergeGo(mv, j)
			return nil
		}
	}
	if by > 0 {
		e.status = "已是最后一处冲突 · " + mv.summary()
	} else {
		e.status = "已是第一处冲突 · " + mv.summary()
	}
	return nil
}

// mergeCommand runs a choice on the conflict at the caret.
func (e *Editor) mergeCommand(do func(e *Editor, mv *mergeView, k int)) error {
	mv, k, err := e.mergeCurrent()
	if err != nil {
		return err
	}
	do(e, mv, k)
	return nil
}

// mergeMarkResolved saves the result and stages it, which ends the file's conflict; with
// conflicts left it asks first.
func (e *Editor) mergeMarkResolved() error {
	mv := mergeOf(e.cur())
	if mv == nil {
		return errors.New("先打开一个冲突文件的三方合并")
	}
	e.mergeSync(mv)
	if hasConflictMarkers(mv.result.doc.buf.Snapshot()) {
		return errors.New("结果里还有冲突标记（<<<<<<<）：先处理它们")
	}
	do := func(e *Editor) error { return e.mergeStage(mv) }
	if n := mv.unresolved(); n > 0 {
		e.choose(fmt.Sprintf("还有 %d 处冲突没处理（留着共同祖先的样子），仍标记为已解决？", n),
			ovItem{label: "标记为已解决", run: do},
			ovItem{label: "取消", run: func(*Editor) error { return nil }})
		return nil
	}
	return do(e)
}

func (e *Editor) mergeStage(mv *mergeView) error {
	defer e.bindRepository(e.repositoryForPath(mv.path))()
	if err := e.saveDoc(mv.result.doc); err != nil {
		return err
	}
	path := mv.path
	return e.gitDo("暂存中…", func(r *git.Repo) error { return r.Stage(path) }, func(e *Editor) {
		e.mergeResolvedDone(path)
		e.status = filepath.Base(path) + " 已标记为已解决（已暂存）"
	})
}

// mergeRestore puts the file back in conflict, after asking: the markers and stages come back
// and the result starts again from the common ancestor.
func (e *Editor) mergeRestore() error {
	mv := mergeOf(e.cur())
	if mv == nil {
		return errors.New("先打开一个冲突文件的三方合并")
	}
	path := mv.path
	e.choose("把 "+filepath.Base(path)+" 恢复成冲突？结果里的改动会丢掉",
		ovItem{label: "恢复冲突", run: func(e *Editor) error {
			defer e.bindRepository(e.repositoryForPath(path))()
			return e.gitDo("恢复冲突中…", func(r *git.Repo) error { return r.RestoreConflict(path) }, func(e *Editor) {
				e.mergeRestored(mv)
			})
		}},
		ovItem{label: "取消", run: func(*Editor) error { return nil }})
	return nil
}

// mergeRestored starts the result over after the file went back in conflict.
func (e *Editor) mergeRestored(mv *mergeView) {
	e.mergeResolvedUndone(mv.path)
	d := mv.result.doc
	if s, err := text.Load(mv.path); err == nil {
		d.buf.Reload(s)
	}
	s := d.buf.Snapshot()
	start := strings.Join(mv.startLines(!e.mergeManual), eolOf(s))
	d.buf.BreakGroup()
	d.buf.Apply(edit.Transaction{Changes: text.ChangeSet{{Start: 0, End: s.Len(), Text: start}}, Origin: edit.OriginCommand})
	d.buf.BreakGroup()
	clear(mv.ignored)
	mv.log, mv.redo, mv.ranges = nil, nil, nil
	e.mergeSync(mv)
	e.status = filepath.Base(mv.path) + " 又是冲突了：结果从共同祖先重新开始 · " + mv.summary()
}

// mergeClose saves the result and closes the sides, leaving the file's tab.
func (e *Editor) mergeClose() error {
	mv := mergeOf(e.cur())
	if mv == nil {
		return errors.New("先打开一个冲突文件的三方合并")
	}
	if err := e.saveDoc(mv.result.doc); err != nil {
		return err
	}
	e.mergeDetach(mv)
	e.status = filepath.Base(mv.path) + " 已保存；冲突还没标记为已解决"
	return nil
}

// mergeDetach closes the sides of mv and makes its result a file's tab again.
func (e *Editor) mergeDetach(mv *mergeView) {
	for _, st := range []*tab{mv.oursTab, mv.theirsTab} {
		for _, c := range e.cols {
			if i := slices.Index(c.tabs, st); i >= 0 {
				e.dropTab(c, i)
				break
			}
		}
	}
	mv.oursTab, mv.theirsTab = nil, nil
	if mv.result != nil {
		mv.result.merge = nil
		for _, c := range e.cols {
			if i := slices.Index(c.tabs, mv.result); i >= 0 {
				e.activate(c, i)
			}
		}
	}
}

// refreshMerges keeps the merge views in step each frame: a closed result closes its sides,
// and the sides scroll with the result.
func (e *Editor) refreshMerges() {
	for _, c := range e.cols {
		t := c.tab()
		if t == nil || t.merge == nil {
			continue
		}
		mv := t.merge
		e.mergeSync(mv)
		if t.scrollY == mv.syncedTop && int(mv.ver) == mv.syncedResultV {
			continue
		}
		mv.syncedTop, mv.syncedResultV = t.scrollY, int(mv.ver)
		for _, st := range []*tab{mv.oursTab, mv.theirsTab} {
			if st != nil {
				st.scrollY = mv.sideLine(t.scrollY, st == mv.theirsTab) + mergeHeader
			}
		}
	}
	for _, d := range e.docs {
		for _, t := range d.tabs {
			if ms := t.mergeSide; ms != nil && !e.tabOpen(ms.mv.result) {
				defer e.mergeDetach(ms.mv)
			}
		}
	}
}

func (e *Editor) tabOpen(t *tab) bool {
	for _, c := range e.cols {
		if slices.Contains(c.tabs, t) {
			return true
		}
	}
	return false
}

// sideLine is the line of a side that line of the result shows: the same place in the region
// it is in, or as far past the last region before it.
func (mv *mergeView) sideLine(line int, theirs bool) int {
	at, side := 0, 0
	for i, rg := range mv.ranges {
		lo, hi := mv.sideRange(i, theirs)
		if line < rg[0] {
			break
		}
		if line < rg[1] {
			return lo + min(line-rg[0], max(hi-lo-1, 0))
		}
		at, side = rg[1], hi
	}
	return side + line - at
}

// mergeRow tints a row of a merge view's result or side.
func (e *Editor) mergeRow(t *tab, row *view.RowFrame, line int) {
	if t.merge != nil {
		mv := t.merge
		e.mergeSync(mv)
		for _, i := range mv.conflicts {
			if rg := mv.ranges[i]; line >= rg[0] && line < rg[1] {
				k := slices.Index(mv.conflicts, i)
				if mv.state(k) == regionUndecided {
					row.Base = style.MergeConflict
				} else {
					row.Base = style.DiffAdded
				}
			}
		}
		return
	}
	ms := t.mergeSide
	if ms == nil {
		return
	}
	if line < mergeHeader {
		row.Base, row.Num = style.DiffHeader, -1
		row.Spans = row.Spans[:0]
		return
	}
	mv := ms.mv
	e.mergeSync(mv)
	row.Num = int32(line - mergeHeader + 1)
	l := line - mergeHeader
	for i, rg := range mv.regions {
		lo, hi := mv.sideRange(i, ms.theirs)
		if !rg.conflict {
			if !mv.changedBy(i, ms.theirs) {
				continue
			}
			applied := mv.applied(i)
			if !applied && l == min(lo, max(len(mv.sideLines(ms.theirs))-1, 0)) {
				row.Block = view.DiffBlockMergeApply
			}
			if l >= lo && l < hi {
				row.Base = style.DiffHeader // 还没应用到结果里
				if applied {
					row.Base = style.DiffAdded
				}
			}
			continue
		}
		k := slices.Index(mv.conflicts, i)
		if l == min(lo, max(len(mv.sideLines(ms.theirs))-1, 0)) {
			row.Block = view.DiffBlockMerge
		}
		if l >= lo && l < hi {
			switch st := mv.state(k); {
			case st == regionUndecided:
				row.Base = style.MergeConflict
			case st == regionBoth, st == regionOurs && !ms.theirs, st == regionTheirs && ms.theirs:
				row.Base = style.DiffAdded
			default:
				row.Base = style.DiffHeader
			}
		}
	}
}

func (mv *mergeView) sideLines(theirs bool) []string {
	if theirs {
		return mv.theirs
	}
	return mv.ours
}

// mergeKey is a side's keys: Enter or a takes this side of the conflict at the caret, x ignores
// it, ] and [ go to the next and previous conflict.
func (e *Editor) mergeKey(r rune) bool {
	t := e.cur()
	if t == nil || t.mergeSide == nil || e.area != areaEditor || e.ov != nil || e.find.open && e.findFocus != fieldNone {
		return false
	}
	theirs := t.mergeSide.theirs
	var err error
	switch r {
	case 'a':
		err = e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeTake(mv, k, theirs) })
	case 'x':
		err = e.mergeCommand(func(e *Editor, mv *mergeView, k int) { e.mergeIgnore(mv, k, theirs) })
	case ']':
		err = e.mergeStepConflict(1)
	case '[':
		err = e.mergeStepConflict(-1)
	default:
		return false
	}
	if err != nil {
		e.status = err.Error()
	}
	return true
}

// MergeBlockAt runs a side's button on the conflict whose first line of the side is line of
// column col: "take" or "ignore".
func (e *Editor) MergeBlockAt(col, line int, action string) {
	e.FocusColumn(col)
	defer e.sync()
	t := e.cur()
	if t == nil || t.mergeSide == nil {
		return
	}
	s := t.doc.buf.Snapshot()
	if line >= 0 && line < s.LineCount() {
		t.doc.buf.SetSelections(edit.Single(edit.Cursor(s.LineStart(line))))
	}
	theirs := t.mergeSide.theirs
	if action == "apply" {
		if err := e.mergeApplyAt(t.mergeSide.mv, theirs, line); err != nil {
			e.status = err.Error()
		}
		return
	}
	err := e.mergeCommand(func(e *Editor, mv *mergeView, k int) {
		if action == "ignore" {
			e.mergeIgnore(mv, k, theirs)
		} else {
			e.mergeTake(mv, k, theirs)
		}
	})
	if err != nil {
		e.status = err.Error()
	}
}

// openMergeCurrent opens the merge view of the file being edited, or of Source Control's
// selected file.
func (e *Editor) openMergeCurrent() error {
	if t := e.cur(); t != nil && e.area == areaEditor {
		switch {
		case mergeOf(t) != nil:
			return nil
		case t.diff != nil && t.diff.changes:
			line := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
			if line < len(t.diff.lines) {
				if l := t.diff.lines[line]; l.sec >= 0 && l.sec < len(t.diff.secs) {
					return e.openMerge(t.diff.secs[l.sec].path)
				}
			}
		case t.diff == nil && t.term == nil:
			return e.openMerge(t.doc.buf.Path())
		}
	}
	if r, ok := e.scmSelected(); ok {
		return e.openMerge(r.Path)
	}
	return errors.New("先打开或选中一个有冲突的文件")
}

// startLines is the result a merge view starts from: the base, with every change no conflict
// holds applied when auto is set.
func (mv *mergeView) startLines(auto bool) []string {
	if auto {
		return autoMerged(mv.base, mv.ours, mv.theirs, mv.regions)
	}
	return slices.Clone(mv.base)
}

// changedBy reports whether region i is a change of a side (theirs, or ours) that no conflict
// holds.
func (mv *mergeView) changedBy(i int, theirs bool) bool {
	rg := mv.regions[i]
	return !rg.conflict && (theirs && rg.theirs || !theirs && rg.ours)
}

// sideOf is the lines a region no conflict holds takes from the side that changed it.
func (mv *mergeView) sideOf(i int) []string {
	rg := mv.regions[i]
	if rg.ours {
		return mv.ours[rg.oursLo:rg.oursHi]
	}
	return mv.theirs[rg.theirsLo:rg.theirsHi]
}

// applied reports whether the result has the change of region i (no conflict's) in it.
func (mv *mergeView) applied(i int) bool {
	rg := mv.ranges[i]
	return slices.Equal(mv.lines[rg[0]:rg[1]], mv.sideOf(i))
}

// mergeApplySide applies the changes no conflict holds that the result lacks, as one step:
// of both sides (0), ours (1) or theirs (2).
func (e *Editor) mergeApplySide(which int) error {
	mv := mergeOf(e.cur())
	if mv == nil {
		return errors.New("先打开一个冲突文件的三方合并")
	}
	e.mergeSync(mv)
	set := map[int][]string{}
	for i, rg := range mv.regions {
		if rg.conflict || mv.applied(i) || which == 1 && !rg.ours || which == 2 && !rg.theirs {
			continue
		}
		set[i] = slices.Clone(mv.sideOf(i))
	}
	if len(set) == 0 {
		e.status = "没有要应用的不冲突改动"
		return nil
	}
	e.mergeAct(mv, set, func(map[int]uint8) {})
	e.status = fmt.Sprintf("已应用 %d 处不冲突的改动 · %s", len(set), mv.summary())
	return nil
}

// mergeApplyAt applies the one change no conflict holds whose lines of a side start at line
// (of the side's document).
func (e *Editor) mergeApplyAt(mv *mergeView, theirs bool, line int) error {
	e.mergeSync(mv)
	l := line - mergeHeader
	for i := range mv.regions {
		if !mv.changedBy(i, theirs) || mv.applied(i) {
			continue
		}
		if lo, _ := mv.sideRange(i, theirs); lo == l || min(lo, max(len(mv.sideLines(theirs))-1, 0)) == l {
			e.mergeAct(mv, map[int][]string{i: slices.Clone(mv.sideOf(i))}, func(map[int]uint8) {})
			return nil
		}
	}
	return errors.New("这里没有要应用的改动")
}

// toggleMergeAutoApply turns applying the changes no conflict holds on or off for the merge
// views opened from now on.
func (e *Editor) toggleMergeAutoApply() error {
	e.mergeManual = !e.mergeManual
	if e.state != nil {
		e.state.MergeManual = e.mergeManual
		e.saveState()
	}
	if e.mergeManual {
		e.status = "三方合并打开时不再自动应用不冲突的改动（用「应用两边不冲突的改动」）"
	} else {
		e.status = "三方合并打开时自动应用不冲突的改动"
	}
	return nil
}
