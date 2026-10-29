package app

// A side-by-side diff in two panes, after Mayoi v0.11.5's diff view (MIT): the window draws each
// side in a pane of its own, with a gutter between them where each change is a band from its
// lines on the left to its lines on the right. With Line Alignment on (the default) a line one
// side lacks holds a space on the other and the rows line up; off, each side's lines close up
// and read as streams of their own. With Synchronized Scrolling on, the sides scroll together;
// off, a wheel over a side scrolls that side alone, and turned on again the sides line up from
// the line the right side was at. The diff tab's own text (a row per pair, "left │ right") stays
// what the keys move through, and what the terminal shows.

import (
	"errors"
	"strings"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/git/linediff"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/syntax/highlight"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// splitSides is what a split diff's two panes are made of, worked out once for each text of
// the diff: the lines of each side in order, the diff's line of each, and a highlighter each.
type splitSides struct {
	version uint64
	streams [2][]int // the diff's lines each side shows (headers in both), in order
	at      [2][]int // for each line of the diff, its place in each side's text (-1 none)
	snap    [2]text.Snapshot
	hl      [2]*highlight.Highlighter
}

// splitGlow is lines a block action wrote: the file, from line to line (exclusive).
type splitGlow struct {
	path     string
	from, to int
}

// splitPanes reports whether the diff tab t shows in two panes.
func (e *Editor) splitPanes(t *tab) bool {
	return e.profile.GUI && t.diff != nil && t.diff.splitW > 0
}

// sides works out (or reuses) the two sides of the split diff t shows.
func (e *Editor) sides(t *tab) *splitSides {
	dt := t.diff
	v := t.doc.buf.Snapshot().Version()
	if dt.sides != nil && dt.sides.version == v && len(dt.sides.at[0]) == len(dt.lines) {
		return dt.sides
	}
	old := dt.sides
	ss := &splitSides{version: v}
	var texts [2][]string
	for side := range 2 {
		ss.at[side] = make([]int, len(dt.lines))
	}
	for i, l := range dt.lines {
		for side := range 2 {
			ss.at[side][i] = -1
			has := !l.split || side == 0 && l.left || side == 1 && l.right
			if !has {
				continue
			}
			ss.streams[side] = append(ss.streams[side], i)
			if l.split {
				ss.at[side][i] = len(texts[side])
				if side == 0 {
					texts[side] = append(texts[side], l.ltext)
				} else {
					texts[side] = append(texts[side], l.rtext)
				}
			}
		}
	}
	lx := highlight.Lexer(dt.path)
	for side := range 2 {
		ss.snap[side] = text.FromString(strings.Join(texts[side], "\n"), text.Meta{Path: dt.path, ReadOnly: true})
		if lx != nil {
			ss.hl[side] = highlight.New(lx, func() { e.Post(func(*Editor) {}) })
		}
	}
	if old != nil {
		for _, h := range old.hl {
			h.Close()
		}
	}
	dt.sides = ss
	return ss
}

// splitFrame fills pane.Split for a split diff the window shows in two panes.
func (e *Editor) splitFrame(t *tab, c *column, pane *view.PaneFrame) {
	pane.Split = nil
	if !e.splitPanes(t) {
		return
	}
	dt := t.diff
	ss := e.sides(t)
	h := max(int(c.size.H), 1)
	over := h
	aligned, synced := e.cfg.DiffAlignedRows, e.cfg.DiffSyncScroll
	n := len(dt.lines)
	if dt.alignRight { // going to a change brings both sides to it
		dt.rightTop, dt.alignRight = t.scrollY, false
	}
	dt.rightTop = max(min(dt.rightTop, n-1), 0)
	sd := &view.SplitDiff{Aligned: aligned, Synced: synced, Glide: dt.glide}
	dt.glide = false
	caret := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	// The first row of each side in view, as a place in its list.
	var lists [2][]int
	if aligned {
		all := make([]int, n)
		for i := range all {
			all[i] = i
		}
		lists = [2][]int{all, all}
	} else {
		lists = ss.streams
	}
	first := func(list []int, doc int) int { // the first place whose line is doc or after
		lo, hi := 0, len(list)
		for lo < hi {
			m := (lo + hi) / 2
			if list[m] < doc {
				lo = m + 1
			} else {
				hi = m
			}
		}
		return min(lo, max(len(list)-1, 0))
	}
	tops := [2]int{first(lists[0], t.scrollY), 0}
	if synced {
		if aligned {
			tops[1] = tops[0]
		} else if len(lists[0]) > 0 {
			tops[1] = first(lists[1], lists[0][tops[0]])
		}
	} else {
		tops[1] = first(lists[1], dt.rightTop)
	}
	s := t.doc.buf.Snapshot()
	words := map[int][2][]style.Span{}
	o := e.diffOpts(0)
	for side := range 2 {
		sp := &sd.Left
		if side == 1 {
			sp = &sd.Right
		}
		list := lists[side]
		from := max(tops[side]-over, 0)
		to := min(tops[side]+h+over, len(list))
		sp.Over, sp.Current = tops[side]-from, int32(caret)
		maxNum := 1
		if hl := ss.hl[side]; hl != nil && from < to {
			a, z := -1, -1
			for _, i := range list[from:to] {
				if at := ss.at[side][i]; at >= 0 {
					if a < 0 {
						a = at
					}
					z = at
				}
			}
			if a >= 0 {
				hl.Request(source{ss.snap[side]}, a, z)
			}
		}
		for _, i := range list[from:to] {
			row := e.splitRow(t, ss, s, side, i, words, o)
			if row.Num > int32(maxNum) {
				maxNum = int(row.Num)
			}
			if i == caret && row.Base == style.Text {
				row.Base = style.CurrentLine
			}
			sp.Rows = append(sp.Rows, row)
			sp.Lines = append(sp.Lines, int32(i))
		}
		sp.Gutter = view.Unit(len(itoaInt(maxNum)) + 3)
	}
	sd.Bands = splitBands(dt, sd)
	pane.Split = sd
}

func itoaInt(n int) string {
	if n <= 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// splitRow is line i of the diff as one side shows it.
func (e *Editor) splitRow(t *tab, ss *splitSides, s text.Snapshot, side, i int, words map[int][2][]style.Span, o diffOpts) view.RowFrame {
	dt := t.diff
	l := dt.lines[i]
	row := view.RowFrame{Line: int32(i), Base: style.Text}
	if !l.split { // a header, a fold, a file, a note: the diff's own row
		ls, le := s.LineRange(i)
		start := len(e.arena)
		e.arena = s.AppendRange(e.arena, ls, le)
		row.Text = e.arena[start:len(e.arena):len(e.arena)]
		dt.row(&row, i)
		if side == 0 {
			row.Block = view.DiffBlockNone // a header's buttons stand once, on the right
		}
		return row
	}
	present, txt := l.left, l.ltext
	if side == 1 {
		present, txt = l.right, l.rtext
	}
	if !present {
		row.Filler, row.Num = true, -1
		return row
	}
	if side == 1 {
		e.markReview(t, &row, i) // the review's notes and buttons stand on the right
	}
	start := len(e.arena)
	e.arena = append(e.arena, txt...)
	row.Text = e.arena[start:len(e.arena):len(e.arena)]
	if hl := ss.hl[side]; hl != nil {
		row.Spans = syntaxSpans(row.Spans, row.Text, hl.Line(ss.at[side][i], txt))
	}
	if side == 0 {
		row.Num = int32(l.oldLine + 1)
		if l.leftGone {
			row.Base = style.DiffRemoved
		}
	} else {
		row.Num = int32(l.line + 1)
		if l.rightAdded {
			row.Base = style.DiffAdded
		}
	}
	if l.leftGone && l.rightAdded { // a changed line: what changed against the line it pairs with
		ws, ok := words[i]
		if !ok {
			a, b := o.words(l.ltext, l.rtext)
			ws = [2][]style.Span{wordSpans(a, style.DiffRemovedWord), wordSpans(b, style.DiffAddedWord)}
			words[i] = ws
		}
		row.Spans = append(row.Spans, ws[side]...)
	}
	if g := dt.glow; side == 1 && g.path != "" && g.path == dt.pathOf(l) && l.line >= g.from && l.line < g.to {
		row.Glow = true
	}
	return row
}

func wordSpans(ws []linediff.Span, st style.ID) []style.Span {
	out := make([]style.Span, 0, len(ws))
	for _, w := range ws {
		if w.Start < w.End {
			out = append(out, style.Span{Start: int32(w.Start), End: int32(w.End), Style: st})
		}
	}
	return out
}

// splitBands are the changes with lines in view on either side, as rows of each side's list.
func splitBands(dt *diffTab, sd *view.SplitDiff) []view.SplitBand {
	type ends struct{ lf, lt, rf, rt, first int }
	at := map[[2]int]*ends{} // by file and change
	var order [][2]int
	mark := func(lines []int32, left bool) {
		for p, li := range lines {
			l := dt.lines[li]
			if !l.split || l.hunk < 0 {
				continue
			}
			k := [2]int{l.sec, l.hunk}
			e := at[k]
			if e == nil {
				e = &ends{lf: -1, rf: -1, first: int(li)}
				at[k] = e
				order = append(order, k)
			}
			e.first = min(e.first, int(li))
			switch {
			case left && l.left && l.leftGone:
				if e.lf < 0 {
					e.lf = p
				}
				e.lt = p + 1
			case !left && l.right && l.rightAdded:
				if e.rf < 0 {
					e.rf = p
				}
				e.rt = p + 1
			}
		}
	}
	mark(sd.Left.Lines, true)
	mark(sd.Right.Lines, false)
	place := func(lines []int32, doc int) int { // where a side with no lines of the change has it
		for p, li := range lines {
			if int(li) >= doc {
				return p
			}
		}
		return len(lines)
	}
	var out []view.SplitBand
	for _, k := range order {
		e := at[k]
		b := view.SplitBand{LeftFrom: e.lf, LeftTo: e.lt, RightFrom: e.rf, RightTo: e.rt, Line: -1}
		if b.LeftFrom < 0 {
			b.LeftFrom = place(sd.Left.Lines, e.first)
			b.LeftTo = b.LeftFrom
		}
		if b.RightFrom < 0 {
			b.RightFrom = place(sd.Right.Lines, e.first)
			b.RightTo = b.RightFrom
		}
	header:
		for j := e.first; j >= 0; j-- { // the change's header: its actions
			if l := dt.lines[j]; l.kind == diffHeader && l.sec == k[0] && l.hunk == k[1] {
				b.Line = int32(j)
				switch dt.blockOf(l) {
				case view.DiffBlockRevert:
					b.Arrow = true
				case view.DiffBlockCompare:
					b.Arrow, b.Compare = true, true
				}
				break header
			}
		}
		out = append(out, b)
	}
	return out
}

// SplitScroll scrolls column col's split diff by dy rows: both sides when they scroll together,
// the side under the wheel (right or not) when they do not.
func (e *Editor) SplitScroll(col int, right bool, dy int) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	t := e.cols[col].tab()
	if t == nil || !e.splitPanes(t) || dy == 0 {
		return
	}
	dt := t.diff
	n := max(len(dt.lines)-1, 0)
	if right && !e.cfg.DiffSyncScroll {
		dt.rightTop = max(min(dt.rightTop+dy, n), 0)
		return
	}
	t.scrollY = max(min(t.scrollY+dy, n), 0)
	if e.cfg.DiffSyncScroll {
		dt.rightTop = t.scrollY
	}
}

// SplitClick puts the caret on line of column col's split diff, on the side clicked (right or
// not, for copying); open (a double click) opens it.
func (e *Editor) SplitClick(col, line int, right, open bool) {
	e.FocusColumn(col)
	defer e.sync()
	t := e.cur()
	if t == nil || t.diff == nil {
		return
	}
	s := t.doc.buf.Snapshot()
	if line < 0 || line >= s.LineCount() {
		return
	}
	t.doc.buf.SetSelections(edit.Single(edit.Cursor(s.LineStart(line))))
	t.diff.copyLeft = !right
	if open {
		if err := e.diffOpen(t); err != nil {
			e.status = err.Error()
		}
	}
}

// SplitArrow is a click on a change's arrow in the gutter: it reverts the change (or, comparing
// two files, copies the left into the right); with append set, it appends instead.
func (e *Editor) SplitArrow(col, header int, appendLines bool) {
	action := "revert"
	if appendLines {
		action = "append"
	}
	e.DiffBlockAt(col, header, action)
}

// toggleSyncScroll lets the sides of a split diff scroll together, or each on its own; together
// again, they line up from the line the right side was at.
func (e *Editor) toggleSyncScroll() error {
	e.cfg.DiffSyncScroll = !e.cfg.DiffSyncScroll
	if e.state != nil {
		v := e.cfg.DiffSyncScroll
		e.state.DiffSyncScroll = &v
		e.saveState()
	}
	for _, c := range e.cols {
		if t := c.tab(); t != nil && t.diff != nil {
			if e.cfg.DiffSyncScroll {
				t.scrollY = t.diff.rightTop
			} else {
				t.diff.rightTop = t.scrollY
			}
		}
	}
	if e.cfg.DiffSyncScroll {
		e.status = i18n.T("Side-by-side diff: both sides scroll together")
	} else {
		e.status = i18n.T("Side-by-side diff: each side scrolls separately")
	}
	return nil
}

// toggleAlignedRows lines the rows of a split diff's sides up, or lets each side's lines close up.
func (e *Editor) toggleAlignedRows() error {
	e.cfg.DiffAlignedRows = !e.cfg.DiffAlignedRows
	if e.state != nil {
		v := e.cfg.DiffAlignedRows
		e.state.DiffAlignedRows = &v
		e.saveState()
	}
	if e.cfg.DiffAlignedRows {
		e.status = i18n.T("Side-by-side diff: rows of both sides aligned")
	} else {
		e.status = i18n.T("Side-by-side diff: rows laid out separately (not aligned)")
	}
	return nil
}

// glowLines marks the lines a block action wrote in path, from line from, n of them, to glow.
func (dt *diffTab) glowLines(path string, from, n int) {
	dt.glow = splitGlow{path: path, from: from, to: from + n}
	dt.glowGen++
}

// copySplitLine copies the caret's line of a split diff as one side has it: the side last
// clicked, or the right (the left when the right has no line there).
func (e *Editor) copySplitLine(t *tab) error {
	if e.host == nil {
		return errors.New(i18n.T("Clipboard unavailable"))
	}
	dt := t.diff
	i := t.doc.buf.Snapshot().LineAt(t.doc.buf.Selections().Main().Head)
	if i < 0 || i >= len(dt.lines) {
		return nil
	}
	l := dt.lines[i]
	txt, side := l.rtext, i18n.T("right")
	if !l.split {
		s := t.doc.buf.Snapshot()
		ls, le := s.LineRange(i)
		txt, side = s.String(ls, le), ""
	} else if dt.copyLeft && l.left || !l.right {
		txt, side = l.ltext, i18n.T("left")
	}
	if err := e.host.SetClipboard(txt + "\n"); err != nil {
		return err
	}
	e.status = i18n.F("Copied this line on the %s", side)
	return nil
}
