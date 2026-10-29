package gui

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/view"
)

// A side-by-side diff in two panes with a gutter between them, where each change is a band from
// its lines on the left to its lines on the right, curving where the sides stand apart, with an
// arrow in the band by its first line (after Mayoi v0.11.5's diff view, MIT). Going to a change
// glides there, and lines a block action wrote glow a moment.

const (
	splitGutterW  = 40                     // the gutter between the panes
	splitGlide    = 180 * time.Millisecond // how long going to a change glides
	splitGlowTime = 900 * time.Millisecond // how long written lines glow
)

// splitState is a column's split diff as the window last drew it.
type splitState struct {
	top        [2]int32 // the diff's line at the top of each side
	glideFrom  [2]float32
	glideAt    time.Time
	glowGen    uint64
	glowAt     time.Time
	wheel      [2]float32 // a trackpad's scroll short of a row, per side
	hover      int        // the band whose arrow the pointer is on; -1 none
	arrowHover bool
	review     int // the right side's row whose number the pointer is over: its note button shows; -1 none
}

// splitGeometry is where the panes stand in a pane's rect.
type splitGeometry struct {
	r                       ui.Rect
	leftX, rightX, sideW, g float32
}

func splitGeom(r ui.Rect) splitGeometry {
	w := max((r.W-splitGutterW)/2, 0)
	return splitGeometry{r: r, leftX: r.X, rightX: r.X + w + splitGutterW, sideW: w, g: r.X + w}
}

// splitShift is how far each side is drawn below its place while gliding, in DIPs.
func (f *Frontend) splitShift(st *splitState, now time.Time) [2]float32 {
	var out [2]float32
	if st.glideAt.IsZero() {
		return out
	}
	t := float32(now.Sub(st.glideAt)) / float32(splitGlide)
	if t >= 1 {
		st.glideAt = time.Time{}
		return out
	}
	ease := 1 - (1-t)*(1-t)*(1-t) // ease out
	for i := range out {
		out[i] = st.glideFrom[i] * (1 - ease)
	}
	return out
}

// splitUpdate takes a new frame: a glide from where the top was when the view moved to a change,
// and a glow for lines just written.
func (f *Frontend) splitUpdate(ci int, sd *view.SplitDiff, glowGen uint64, now time.Time) {
	st := &f.cols[ci].split
	for side, sp := range []*view.SplitSide{&sd.Left, &sd.Right} {
		if sp.Over >= len(sp.Lines) {
			continue
		}
		top := sp.Lines[sp.Over]
		if top != st.top[side] && sd.Glide && st.top[side] >= 0 {
			for i, l := range sp.Lines { // where the old top is among the rows: glide from there
				if l == st.top[side] {
					st.glideFrom[side] = float32(i-sp.Over) * f.font.lineH
					st.glideAt = now
				}
			}
		}
		st.top[side] = top
	}
	f.glowUpdate(ci, glowGen, now)
}

// glowUpdate starts the glow of lines a block action wrote, when the diff says new ones were.
func (f *Frontend) glowUpdate(ci int, glowGen uint64, now time.Time) {
	st := &f.cols[ci].split
	if glowGen != st.glowGen {
		st.glowGen = glowGen
		if glowGen > 0 {
			st.glowAt = now
		}
	}
}

// glowAlpha is how bright written lines glow now, 0 when they no longer do.
func (f *Frontend) glowAlpha(ci int, now time.Time) float32 {
	st := &f.cols[ci].split
	if st.glowAt.IsZero() {
		return 0
	}
	since := now.Sub(st.glowAt)
	if since >= splitGlowTime {
		return 0
	}
	return 1 - float32(since)/float32(splitGlowTime)
}

// rowY is the top of row i of a side, shifted while gliding.
func (f *Frontend) rowY(r ui.Rect, sp *view.SplitSide, i int, shift float32) float32 {
	return r.Y + codePadY + float32(i-sp.Over)*f.font.lineH + shift
}

// paintSplit draws a split diff's two panes and the bands between them.
func (f *Frontend) paintSplit(p *ui.Painter, r ui.Rect, pf *view.PaneFrame, ci int, focused bool) {
	sd := pf.Split
	st := &f.cols[ci].split
	now := p.Now()
	shift := f.splitShift(st, now)
	if !st.glideAt.IsZero() {
		p.AnimationFrame()
	}
	glow := f.glowAlpha(ci, now)
	if glow > 0 {
		p.AnimationFrame()
	}
	g := splitGeom(r)
	pl := f.pal
	sides := []*view.SplitSide{&sd.Left, &sd.Right}
	xs := []float32{g.leftX, g.rightX}
	for side, sp := range sides {
		x0 := xs[side]
		gutterW := float32(sp.Gutter) * f.font.charW
		p.Clip(ui.Rect{X: x0, Y: r.Y, W: g.sideW, H: r.H}, 0, func() {
			for i := range sp.Rows {
				row := &sp.Rows[i]
				y := f.rowY(r, sp, i, shift[side])
				if y+f.font.lineH < r.Y || y > r.Y+r.H {
					continue
				}
				switch {
				case row.Filler:
					p.Fill(ui.Rect{X: x0, Y: y, W: g.sideW, H: f.font.lineH}, pl.chrome.Alpha(0.55), 0)
					continue
				case row.Base != style.Text && pl.hasBg[row.Base]:
					p.Fill(ui.Rect{X: x0, Y: y, W: g.sideW, H: f.font.lineH}, pl.bg[row.Base], 0)
				}
				if row.Glow && glow > 0 {
					p.Fill(ui.Rect{X: x0, Y: y, W: g.sideW, H: f.font.lineH}, pl.accent.Alpha(0.35*glow), 0)
				}
				if row.Num > 0 {
					num := strconv.Itoa(int(row.Num))
					gc := pl.gutter
					if row.Line == sp.Current {
						gc = pl.gutterNow
					}
					gs := f.glyphs.shape(num, f.font)
					f.placeASCII(gs, len(num))
					p.Glyphs(f.glyphs.tmp, x0+gutterW-float32(len(num)+1)*f.font.charW, y+f.font.baseline, gc)
				}
				textX := x0 + gutterW
				if row.Num < 0 && !row.Filler { // a header spans its side from the left
					textX = x0 + f.font.charW
				}
				p.Clip(ui.Rect{X: textX, Y: y, W: max(x0+g.sideW-textX, 0), H: f.font.lineH}, 0, func() {
					f.paintRow(p, row, textX, y, focused)
				})
			}
		})
	}
	// The gutter: a band for each change, from its lines on the left to those on the right.
	p.Fill(ui.Rect{X: g.g, Y: r.Y, W: splitGutterW, H: r.H}, pl.editor, 0)
	p.Clip(ui.Rect{X: g.g, Y: r.Y, W: splitGutterW, H: r.H}, 0, func() {
		for bi, b := range sd.Bands {
			l0, l1 := f.rowY(r, &sd.Left, b.LeftFrom, shift[0]), f.rowY(r, &sd.Left, b.LeftTo, shift[0])
			r0, r1 := f.rowY(r, &sd.Right, b.RightFrom, shift[1]), f.rowY(r, &sd.Right, b.RightTo, shift[1])
			col := pl.changed
			switch {
			case b.LeftFrom == b.LeftTo:
				col = pl.added
			case b.RightFrom == b.RightTo:
				col = pl.removed
			}
			alpha := float32(0.22)
			if st.hover == bi {
				alpha = 0.42
			}
			x0, x1, mid := g.g, g.g+splitGutterW, g.g+splitGutterW/2
			path := (&ui.Path{}).MoveTo(x0, l0).CubeTo(mid, l0, mid, r0, x1, r0).LineTo(x1, max(r1, r0+1)).
				CubeTo(mid, max(r1, r0+1), mid, max(l1, l0+1), x0, max(l1, l0+1)).Close()
			p.FillPath(path, col.Alpha(alpha))
			edge := (&ui.Path{}).MoveTo(x0, l0).CubeTo(mid, l0, mid, r0, x1, r0)
			p.StrokePath(edge, 1, col.Alpha(0.6))
			edge = (&ui.Path{}).MoveTo(x0, max(l1, l0+1)).CubeTo(mid, max(l1, l0+1), mid, max(r1, r0+1), x1, max(r1, r0+1))
			p.StrokePath(edge, 1, col.Alpha(0.6))
		}
	})
	p.Fill(ui.Rect{X: g.g, Y: r.Y, W: 1, H: r.H}, pl.border, 0)
	p.Fill(ui.Rect{X: g.g + splitGutterW - 1, Y: r.Y, W: 1, H: r.H}, pl.border, 0)
	if st.hover >= 0 && st.hover < len(sd.Bands) { // pointing at an arrow lights the lines it takes
		b := sd.Bands[st.hover]
		for i := b.LeftFrom; i < b.LeftTo; i++ {
			p.Fill(ui.Rect{X: g.leftX, Y: f.rowY(r, &sd.Left, i, shift[0]), W: g.sideW, H: f.font.lineH}, pl.accent.Alpha(0.18), 0)
		}
	}
	f.paintSticky(p, r, pf)
}

// splitArrows are the arrows of the bands, by the first line of each change's left side, and the
// buttons of the headers, on the right side.
func (f *Frontend) splitArrows(c *ui.Context, ci int, pf *view.PaneFrame) {
	sd := pf.Split
	st := &f.cols[ci].split
	r := f.cols[ci].size
	if r.W <= 0 {
		return
	}
	shift := f.splitShift(st, time.Now())
	g := splitGeom(ui.Rect{W: r.W, H: r.H})
	st.hover = -1
	for bi, b := range sd.Bands {
		if !b.Arrow || b.Line < 0 {
			continue
		}
		y := f.rowY(ui.Rect{}, &sd.Left, b.LeftFrom, shift[0])
		if y < codePadY-2 || y > r.H {
			continue
		}
		label, tip := "»", "还原这处：左边的行换进右边（可以撤销）；按住 Ctrl 或 Alt 点击是追加"
		if b.Compare {
			tip = "把左边这处写进右边的文件（可以撤销）"
		}
		a := ui.Box(c.Key(fmt.Sprintf("split-arrow-%d", b.Line))).Absolute().Left(g.g+splitGutterW/2-10).Top(y+(f.font.lineH-18)/2).
			Size(20, 18).Radius(9).Justify(ui.Center).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label(label).Tooltip(tip)
		col := f.pal.accent
		if a.Hovered() {
			st.hover = bi
			a.Background(f.pal.accent)
			col = f.pal.editor
		} else {
			a.Background(f.pal.editor).Border(1, f.pal.accent.Alpha(0.6))
		}
		a.Children(func() { ui.Text(c, label).FontSize(12).FontWeight(700).TextColor(col) })
		if a.Clicked() {
			m := a.ClickModifiers()
			appendLines := !b.Compare && m&(ui.Ctrl|ui.Alt) != 0
			f.ed.SplitArrow(ci, int(b.Line), appendLines)
		}
	}
	// The review's buttons, by the right side's line numbers: a note's, and one to add a note
	// where the pointer is.
	for i, row := range sd.Right.Rows {
		if row.Filler || row.Num <= 0 || row.ReviewID == 0 && i != st.review {
			continue
		}
		y := f.rowY(ui.Rect{}, &sd.Right, i, shift[1])
		if y < codePadY+float32(len(pf.Sticky))*f.font.lineH || y > r.H {
			continue
		}
		label, word := "添加评审意见", "+"
		if row.ReviewID != 0 {
			label, word = fmt.Sprintf("编辑意见 %d", row.ReviewID), "●"
		}
		b := ui.Box(c.Key(fmt.Sprintf("split-review-%d", row.Line))).Absolute().Left(g.rightX).Top(y).Width(18).Height(f.font.lineH).
			AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Label(label).Tooltip(label)
		b.Children(func() { ui.Text(c, word).FontSize(12).TextColor(f.pal.accent) })
		if b.Clicked() {
			f.ed.ReviewLine(ci, int(row.Line))
		}
	}
	// The headers' buttons stand on the right side.
	for i, row := range sd.Right.Rows {
		if row.Block == view.DiffBlockNone {
			continue
		}
		y := f.rowY(ui.Rect{}, &sd.Right, i, shift[1])
		if y < codePadY+float32(len(pf.Sticky))*f.font.lineH-2 || y > r.H {
			continue
		}
		f.diffBlockBarAt(c, ci, fmt.Sprintf("split-block-%d", row.Line), y, row)
	}
	for i, row := range pf.Sticky {
		f.diffBlockBar(c, ci, fmt.Sprintf("diff-sticky-%d", i), i, row)
	}
}

// splitInput takes the pointer on a split diff: a click puts the caret on a row (two open it),
// and the wheel scrolls the side under it, or both.
func (f *Frontend) splitInput(ci int, ev ui.InputEvent) bool {
	pf := &f.frame.Columns[ci].Pane
	sd := pf.Split
	if sd == nil {
		return false
	}
	st := &f.cols[ci].split
	r := f.cols[ci].size
	g := splitGeom(ui.Rect{W: r.W, H: r.H})
	right := ev.X >= g.rightX
	switch ev.Kind {
	case ui.InputPointerMove:
		st.review = -1
		if right && ev.X < g.rightX+18 && ev.Y >= codePadY {
			shift := f.splitShift(st, time.Now())
			st.review = int((ev.Y-codePadY-shift[1])/f.font.lineH) + sd.Right.Over
		}
		return false
	case ui.InputPointerDown:
		if ev.Button != 0 || ev.X >= g.g && ev.X < g.rightX || right && ev.X < g.rightX+18 {
			return false // the gutter's arrows, the review's buttons
		}
		if ev.Y < codePadY+float32(len(pf.Sticky))*f.font.lineH && len(pf.Sticky) > 0 {
			f.ed.Handle(app.MouseInput{Column: ci, X: 10, Y: 0, Kind: app.MousePress}) // the pinned header: back to it
			f.ed.Handle(app.MouseInput{Column: ci, Kind: app.MouseRelease})
			return true
		}
		sp := &sd.Left
		side := 0
		if right {
			sp, side = &sd.Right, 1
		}
		shift := f.splitShift(st, time.Now())
		i := int((ev.Y-codePadY-shift[side])/f.font.lineH) + sp.Over
		if i < 0 || i >= len(sp.Lines) {
			return true
		}
		f.ed.SplitClick(ci, int(sp.Lines[i]), right, ev.Clicks >= 2)
		return true
	case ui.InputScroll:
		side := 0
		if right {
			side = 1
		}
		st.wheel[side] += ev.DY
		lines := int(st.wheel[side] / f.font.lineH)
		st.wheel[side] -= float32(lines) * f.font.lineH
		if lines != 0 {
			f.ed.SplitScroll(ci, right, lines)
		}
		return true
	}
	return false
}
