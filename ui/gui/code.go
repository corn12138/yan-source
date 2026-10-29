package gui

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/text/width"
	"github.com/corn12138/yan-source/core/view"
	"github.com/corn12138/yan-source/core/view/cell"
)

// 代码窗格四周的留白（DIP）。
const (
	codePadY = 6  // 第一行上面
	codePadR = 24 // 最长的行右边
)

// codeFont 是代码字体和它的格子：一格宽是 0 的宽度，一行高是字号乘行高（取整到像素）。
type codeFont struct {
	family     string
	size       float32
	lineHeight float32
	lineH      float32
	charW      float32
	baseline   float32 // 一行顶部到基线的距离
}

func newCodeFont(o Options) codeFont {
	f := codeFont{family: o.FontFamily, size: o.FontSize, lineHeight: o.LineHeight, lineH: float32(int(o.FontSize*o.LineHeight + 0.5))}
	w, _ := ui.ShapeText(strings.Repeat("0", 64), f.font(), 0).Size()
	f.charW = w / 64
	m := f.font().Metrics()
	f.baseline = float32(int((f.lineH-(m.Ascent+m.Descent))/2 + m.Ascent + 0.5))
	return f
}

func (f codeFont) is(o Options) bool {
	return f.family == o.FontFamily && f.size == o.FontSize && f.lineHeight == o.LineHeight
}

func (f codeFont) font() ui.Font { return ui.Font{Family: f.family, Size: f.size} }

// glyphCache 记住塑形过的文字：每帧画的大多是同样的几十行，塑形一次就够。
type glyphCache struct {
	m   map[string][]ui.Glyph
	tmp []ui.Glyph
}

func (g *glyphCache) reset() { g.m = nil }

func (g *glyphCache) shape(s string, f codeFont) []ui.Glyph {
	if g.m == nil || len(g.m) > 16384 {
		g.m = map[string][]ui.Glyph{}
	}
	gs, ok := g.m[s]
	if !ok {
		gs = ui.Shape(s, f.font())
		g.m[s] = gs
	}
	return gs
}

// codePane 画一列的代码窗格，并把指针事件换算成窗格内的格子坐标交给内核。
func (f *Frontend) codePane(c *ui.Context, pane ui.Element, ci int) {
	pf := &f.frame.Columns[ci].Pane
	focused := ci == f.frame.Active && f.frame.Focus == view.FocusEditor
	if pf.Split != nil { // 并排的改动视图：两个窗格，中间沟槽
		f.splitUpdate(ci, pf.Split, pf.GlowGen, time.Now())
		pane.Cursor(ui.CursorDefault).Background(f.pal.editor).
			HandleInput(func(ev ui.InputEvent) bool { return f.splitInput(ci, ev) }).
			Draw(func(p *ui.Painter, r ui.Rect) { f.cols[ci].size = r; f.paintSplit(p, r, pf, ci, focused) })
		pane.Children(func() { f.splitArrows(c, ci, pf) })
		return
	}
	f.glowUpdate(ci, pf.GlowGen, time.Now())
	cursor := ui.CursorText
	if f.updateLink(c, pane, ci) {
		cursor = ui.CursorPointer
	}
	pane.Cursor(cursor).Background(f.pal.editor).
		HandleInput(func(ev ui.InputEvent) bool { return f.paneInput(ci, ev) }).
		Draw(func(p *ui.Painter, r ui.Rect) {
			f.cols[ci].size = r
			f.glow = f.glowAlpha(ci, p.Now())
			if f.glow > 0 {
				p.AnimationFrame()
			}
			f.paintPane(p, r, pf, focused)
			f.paintLink(p, r, ci)
		})
	pane.Children(func() { f.reviewLineButtons(c, ci, pf); f.diffBlockButtons(c, ci, pf) })
	f.cols[ci].peek = ui.Rect{}
	if pk := pf.Peek; pk != nil && pk.After >= 0 {
		pane.Children(func() { f.cols[ci].peek = f.peekBox(c, pf, pk) })
	}
}

// peekMaxLines 是改动的框里最多写几行 HEAD 里的文字。
const peekMaxLines = 10

// peekBox 是行号旁一处改动下面的框（照着 Mayoi）：这几行在 HEAD 里是什么，还原、暂存、上一处、下一处、关掉。
// 它盖在下面几行上，返回它在窗格里占的地方：点在那里不算点在代码上（paneInput 让给框里的按钮）。
func (f *Frontend) peekBox(c *ui.Context, pf *view.PaneFrame, pk *view.Peek) ui.Rect {
	p := f.pal
	lh := f.font.lineH
	top := codePadY + float32(pk.After+1)*lh
	left := float32(pf.Gutter) * f.font.charW
	box := ui.Column(c.Key("hunk-peek")).Absolute().Top(top).Left(left).Right(12).Radius(6).Clip().
		Background(p.chrome).Border(1, p.border).Cursor(ui.CursorDefault).Label(i18n.F("Change %s", pk.Title))
	box.Children(func() {
		ui.Row(c).Height(26).Padding(0, 6, 0, 10).Gap(4).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(p.border).Children(func() {
			ui.Text(c, pk.Title).FontSize(11.5).TextColor(p.muted).SingleLine().Shrink(1)
			ui.Spacer(c)
			for _, b := range []struct{ label, cmd string }{{i18n.T("Revert"), "git.revertSelectedRanges"}, {i18n.T("Stage"), "git.stageSelectedRanges"}} {
				btn := ui.Box(c.Key("peek:"+b.cmd)).Height(20).Padding(0, 8).Radius(4).Justify(ui.Center).AlignItems(ui.Center).
					Label(b.label).Tooltip(i18n.F("%s This Change (%s)", b.label, f.ed.KeyHint(b.cmd)))
				if btn.Hovered() {
					btn.Background(p.hover)
				}
				btn.Children(func() { ui.Text(c, b.label).FontSize(11.5).FontWeight(600).TextColor(p.text) })
				if btn.Clicked() {
					f.ed.Run(b.cmd)
				}
			}
			for _, b := range []struct {
				icon      *ui.SVG
				label, id string
			}{{iconArrowUp, i18n.T("Previous Change"), "editor.action.dirtydiff.previous"}, {iconArrowDown, i18n.T("Next Change"), "editor.action.dirtydiff.next"}, {iconX, i18n.T("Close"), "yan.hunk.close"}} {
				if f.iconButton(c, b.icon, b.label, false).Clicked() {
					f.ed.Run(b.id)
				}
			}
		})
		ui.Column(c).Padding(4, 10).Children(func() {
			if len(pk.Removed) == 0 {
				ui.Text(c, i18n.T("These lines are not in HEAD: they are new")).FontSize(12).TextColor(p.muted).Height(lh)
				return
			}
			for i, l := range pk.Removed {
				if i == peekMaxLines {
					ui.Text(c, i18n.F("…%d more lines", len(pk.Removed)-i)).FontSize(12).TextColor(p.muted).Height(lh)
					break
				}
				ui.Text(c, strings.ReplaceAll(l, "\t", "    ")).Font(f.opts.FontFamily).FontSize(f.opts.FontSize).
					TextColor(p.text).Background(p.removed.Alpha(0.16)).Height(lh).SingleLine().WidthPercent(100)
			}
		})
	})
	h := box.Bounds().H // 上一帧排出来的高度；第一帧还没有时按一行算
	if h <= 0 {
		h = 26 + lh
	}
	return ui.Rect{X: left, Y: top, W: 1e6, H: h}
}

// cellAt 把窗格内的点换算成格子坐标：x 取最近的格线（点在字的右半边时算到下一个字前面）。
func (f *Frontend) cellAt(x, y float32) (view.Unit, view.Unit) {
	cx := int((x)/f.font.charW + 0.5)
	cy := int((y - codePadY) / f.font.lineH)
	if y < codePadY {
		cy = 0
	}
	return view.Unit(max(cx, 0)), view.Unit(max(cy, 0))
}

// cellUnder 是窗格内的点所在的格子（不取最近的格线）：按住链接修饰键时，指着、点着的是这个字。
func (f *Frontend) cellUnder(x, y float32) (view.Unit, view.Unit) {
	_, cy := f.cellAt(x, y)
	return view.Unit(max(int(x/f.font.charW), 0)), cy
}

// codeLink 是按住链接修饰键时指着的、能转到定义的名字：窗格的第 row 行，从格子 from 到 to。
type codeLink struct {
	row, from, to view.Unit
	ok            bool
}

// linkHeld 报告 m 里有没有链接修饰键（同 Editor.LinkModifier：macOS 上 Cmd，Windows、Linux 上 Ctrl）。
func (f *Frontend) linkHeld(m ui.Modifiers) bool {
	lm := f.ed.LinkModifier()
	return lm != 0 && mods(m)&lm != 0
}

// updateLink 按住链接修饰键、指针在窗格里时，找指着的名字（像 VS Code 那样画成链接）；报告找到没有。
// 只在按住时问指针位置：问了，指针一动就要重画。
func (f *Frontend) updateLink(c *ui.Context, pane ui.Element, ci int) bool {
	st := &f.cols[ci]
	st.link = codeLink{}
	if f.pressCol >= 0 || !f.linkHeld(c.Modifiers()) {
		return false
	}
	px, py, over := pane.PointerPosition()
	if !over || f.onStrip(ci, px) {
		return false
	}
	x, y := f.cellUnder(px, py)
	from, to, ok := f.ed.LinkAt(ci, x, y)
	st.link = codeLink{row: y, from: from, to: to, ok: ok}
	return ok
}

// paintLink 给指着的名字画下划线。
func (f *Frontend) paintLink(p *ui.Painter, r ui.Rect, ci int) {
	l := f.cols[ci].link
	if !l.ok {
		return
	}
	cw, lh := f.font.charW, f.font.lineH
	y := r.Y + codePadY + float32(l.row)*lh + f.font.baseline + 2
	p.Fill(ui.Rect{X: r.X + float32(l.from)*cw, Y: min(y, r.Y+codePadY+float32(l.row+1)*lh-1), W: float32(l.to-l.from) * cw, H: 1}, f.pal.accent, 0)
}

func (f *Frontend) paneInput(ci int, ev ui.InputEvent) bool {
	if ev.Kind == ui.InputPointerMove {
		_, y := f.cellAt(ev.X, ev.Y)
		f.cols[ci].reviewHover = -1
		if ev.X >= 0 && ev.X < 18 && ev.Y >= codePadY {
			f.cols[ci].reviewHover = int(y)
		}
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.X >= 0 && ev.X < 18 && ev.Y >= codePadY {
			_, y := f.cellAt(ev.X, ev.Y)
			pf := &f.frame.Columns[ci].Pane
			if int(y) >= len(pf.Sticky) && int(y) < len(pf.Rows) && (pf.Rows[y].ReviewID != 0 || f.cols[ci].reviewHover == int(y)) {
				return false // The visible review button owns only its row; other gutter clicks still open Git hunks.
			}
		}
		if f.onDiffBlock(ci, ev.X, ev.Y) {
			return false // 改动标题行右边的块操作按钮
		}
		if ev.Button != 0 {
			return false
		}
		if r := f.cols[ci].peek; r.H > 0 && ev.X >= r.X && ev.Y >= r.Y && ev.Y < r.Y+r.H {
			return false // 点在改动的框里：让给框里的按钮
		}
		f.pressCol = ci
		if st := &f.cols[ci]; f.onStrip(ci, ev.X) { // 右边那一条：跳到文件的那个地方
			st.strip = true
			f.ed.OverviewJump(ci, float64(ev.Y/max(st.size.H, 1)))
			return true
		}
		x, y := f.cellAt(ev.X, ev.Y)
		if f.linkHeld(ev.Mods) {
			x, y = f.cellUnder(ev.X, ev.Y) // 同下划线：点的是指着的那个字
		}
		f.ed.Handle(app.MouseInput{Column: ci, X: x, Y: y, Mods: mods(ev.Mods), Kind: app.MousePress})
		return true
	case ui.InputPointerMove:
		if f.pressCol != ci || ev.Button != 0 {
			return false
		}
		if st := &f.cols[ci]; st.strip {
			f.ed.OverviewJump(ci, float64(ev.Y/max(st.size.H, 1)))
			return true
		}
		x, y := f.cellAt(ev.X, ev.Y)
		if ev.Y < 0 {
			y = -1 // 拖到窗格上面：往上滚
		}
		f.ed.Handle(app.MouseInput{Column: ci, X: x, Y: y, Mods: mods(ev.Mods), Kind: app.MouseDrag})
		return true
	case ui.InputPointerUp:
		if f.pressCol == ci {
			f.pressCol = -1
			if st := &f.cols[ci]; st.strip {
				st.strip = false
				return true
			}
			f.ed.Handle(app.MouseInput{Column: ci, Kind: app.MouseRelease})
			return true
		}
	case ui.InputScroll:
		// 触控板按像素滚，鼠标滚轮按行滚：攒够一行（横向攒够一格）才让内核滚。
		// 只沿主方向滚，同 VS Code 的 editor.scrollPredominantAxis（照着 Mayoi 8b7214a）：
		// 触控板横着划总带点上下，竖着划总带点左右，不然文字边滚边漂。
		st := &f.cols[ci]
		dx, dy := ev.DX, ev.DY
		if max(dx, -dx) > max(dy, -dy) {
			dy = 0
		} else {
			dx = 0
		}
		st.scroll += dy
		lines := int(st.scroll / f.font.lineH)
		st.scroll -= float32(lines) * f.font.lineH
		st.scrollX += dx
		cols := int(st.scrollX / f.font.charW)
		st.scrollX -= float32(cols) * f.font.charW
		if lines != 0 || cols != 0 {
			f.ed.Handle(app.ScrollInput{Column: ci, DY: view.Unit(lines), DX: view.Unit(cols)})
		}
		return true
	}
	return false
}

func mods(m ui.Modifiers) keymap.Mod {
	var out keymap.Mod
	if m&ui.Shift != 0 {
		out |= keymap.ModShift
	}
	if m&ui.Ctrl != 0 {
		out |= keymap.ModCtrl
	}
	if m&ui.Alt != 0 {
		out |= keymap.ModAlt
	}
	if m&ui.Super != 0 {
		out |= keymap.ModSuper
	}
	return out
}

// caretRect 是输入法组字的位置（相对工作台，也就是窗口）：有焦点那一列的主光标。
func (f *Frontend) caretRect(c *ui.Context) ui.Rect {
	fr := &f.frame
	if fr.Active >= len(fr.Columns) || fr.Active >= len(f.cols) {
		return ui.Rect{}
	}
	pf := &fr.Columns[fr.Active].Pane
	b := f.cols[fr.Active].pane.Bounds(c)
	return ui.Rect{
		X: b.X + float32(pf.Caret.X)*f.font.charW,
		Y: b.Y + codePadY + float32(pf.Caret.Y)*f.font.lineH,
		W: 1, H: f.font.lineH,
	}
}

// paintPane 按格画代码：行号、当前行、选区和查找命中这些底色、文字、光标。
func (f *Frontend) paintPane(p *ui.Painter, r ui.Rect, pf *view.PaneFrame, focused bool) {
	pl := f.pal
	fo := f.font
	cw, lh := fo.charW, fo.lineH
	gutterW := float32(pf.Gutter) * cw
	textX := r.X + gutterW // 第 0 格的 x（还没减水平滚动）
	scroll := float32(pf.ScrollX) * cw
	for i := range pf.Rows {
		row := &pf.Rows[i]
		y := r.Y + codePadY + float32(i)*lh
		if row.Base != style.Text && pl.hasBg[row.Base] { // 当前行、改动视图里加上和删掉的行
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: lh}, pl.bg[row.Base], 0)
		}
		if row.Glow && f.glow > 0 { // 块操作刚写进去的行亮一下
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: lh}, pl.accent.Alpha(0.35*f.glow), 0)
		}
		// 行号：右对齐，离正文两格。
		num := strconv.Itoa(int(row.Line) + 1)
		switch {
		case row.Num < 0:
			num = ""
		case row.Num > 0:
			num = strconv.Itoa(int(row.Num))
		}
		gc := pl.gutter
		if row.Line == pf.Current {
			gc = pl.gutterNow
		}
		gs := f.glyphs.shape(num, fo)
		f.placeASCII(gs, len(num))
		p.Glyphs(f.glyphs.tmp, r.X+gutterW-float32(len(num)+2)*cw, y+fo.baseline, gc)
		// git 改动：行号右边一道竖条，删掉行的地方一个小缺口（照着 Mayoi）。
		barX := r.X + gutterW - cw - 3
		switch row.Mark {
		case view.MarkAdded:
			p.Fill(ui.Rect{X: barX, Y: y, W: 3, H: lh}, pl.added, 0)
		case view.MarkModified:
			p.Fill(ui.Rect{X: barX, Y: y, W: 3, H: lh}, pl.changed, 0)
		case view.MarkDeleted:
			p.Fill(ui.Rect{X: barX, Y: y - 2.5, W: 7, H: 5}, pl.removed, 2.5)
		case view.MarkDeletedBelow:
			p.Fill(ui.Rect{X: barX, Y: y + lh - 2.5, W: 7, H: 5}, pl.removed, 2.5)
		}

		p.Clip(ui.Rect{X: textX, Y: y, W: max(r.X+r.W-textX, 0), H: lh}, 0, func() {
			end := f.paintRow(p, row, textX-scroll, y, focused)
			if row.Ghost != "" { // 行尾的 blame：比注释还淡
				p.Text(textX-scroll+float32(end+4)*cw, y+(lh-fo.size)/2-1, row.Ghost, fo.size-1, pl.faint)
			}
		})
	}
	defer f.paintOverview(p, r, pf)
	defer f.paintSticky(p, r, pf)
	if !focused {
		return
	}
	// 光标：主光标是一条竖线；组字时先画输入法正在组的字，光标跟在组字光标后面。
	caret := func(c view.Caret, primary bool) {
		if int(c.Y) < len(pf.Sticky) {
			return // 被钉在顶上的行盖住了
		}
		x := r.X + float32(c.X)*cw
		y := r.Y + codePadY + float32(c.Y)*lh
		if primary && f.preedit != "" {
			x = f.paintPreedit(p, x, y)
		}
		if x < textX-1 {
			return
		}
		if c.Block { // vim 普通模式：盖住一个字的块
			p.Fill(ui.Rect{X: x, Y: y + 1, W: cw, H: lh - 2}, pl.caret.Alpha(0.5), 2)
			return
		}
		p.Fill(ui.Rect{X: x - 1, Y: y + 1, W: 2, H: lh - 2}, pl.caret, 1)
	}
	for _, c := range pf.Secondary {
		caret(c, false)
	}
	if pf.Caret.Visible {
		caret(pf.Caret, true)
	}
}

// paintSticky 把钉在顶上的行画在最上面几行上，下面一道阴影（照着 Mayoi）。
func (f *Frontend) paintSticky(p *ui.Painter, r ui.Rect, pf *view.PaneFrame) {
	n := len(pf.Sticky)
	if n == 0 {
		return
	}
	pl, fo := f.pal, f.font
	cw, lh := fo.charW, fo.lineH
	gutterW := float32(pf.Gutter) * cw
	textX := r.X + gutterW
	h := codePadY + float32(n)*lh
	p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: h}, pl.editor, 0)
	for i := range pf.Sticky {
		row := &pf.Sticky[i]
		y := r.Y + codePadY + float32(i)*lh
		if row.Base != style.Text && pl.hasBg[row.Base] { // 改动视图钉住的文件标题
			p.Fill(ui.Rect{X: r.X, Y: y, W: r.W, H: lh}, pl.bg[row.Base], 0)
		}
		if row.Num != -1 {
			num := strconv.Itoa(int(row.Line) + 1)
			gs := f.glyphs.shape(num, fo)
			f.placeASCII(gs, len(num))
			p.Glyphs(f.glyphs.tmp, r.X+gutterW-float32(len(num)+2)*cw, y+fo.baseline, pl.muted)
		}
		p.Clip(ui.Rect{X: textX, Y: y, W: max(r.X+r.W-textX, 0), H: lh}, 0, func() {
			f.paintRow(p, row, textX-float32(pf.ScrollX)*cw, y, false)
		})
	}
	p.FillGradient(ui.Rect{X: r.X, Y: r.Y + h, W: r.W, H: 6}, ui.LinearGradient{From: pl.shadow, To: pl.shadow.Alpha(0), Angle: 180}, 0)
}

// placeASCII 把塑形好的 ASCII 字形逐格摆好，放进 f.glyphs.tmp。
func (f *Frontend) placeASCII(gs []ui.Glyph, n int) {
	t := f.glyphs.tmp[:0]
	for _, g := range gs {
		g.X = float32(min(g.Cluster, n)) * f.font.charW
		t = append(t, g)
	}
	f.glyphs.tmp = t
}

// paintRow 画一行正文：先画各层底色，再按前景色分段塑形，每个字形摆到它的格子里（宽字符居中在两格里）。
func (f *Frontend) paintRow(p *ui.Painter, row *view.RowFrame, x0, y float32, focused bool) int {
	pl := f.pal
	fo := f.font
	cw, lh := fo.charW, fo.lineH
	tw := f.opts.TabWidth
	// 每个字节所在格子的起点，最后一项是行尾。
	type seg struct{ start, end, x, w int }
	var segs [256]seg
	cells := segs[:0]
	cell.ForEach(row.Text, tw, func(start, end, x, w int) {
		cells = append(cells, seg{start, end, x, w})
	})
	endX := 0
	if n := len(cells); n > 0 {
		endX = cells[n-1].x + cells[n-1].w
	}
	xOf := func(b int) int {
		for _, s := range cells {
			if s.start >= b {
				return s.x
			}
		}
		return endX
	}
	// 底色：选区、查找命中、括号、同词，按层叠上去；选区跨过行尾时多画一格。
	for _, sp := range row.Spans {
		if style.LayerOf(sp.Style)&style.LayerBg == 0 || !pl.hasBg[sp.Style] {
			continue
		}
		a, z := xOf(int(sp.Start)), xOf(int(sp.End))
		if int(sp.End) > len(row.Text) {
			z = endX + 1
		}
		bg := pl.bg[sp.Style]
		if sp.Style == style.Selection && !focused {
			bg = pl.selectedDim
		}
		if z > a {
			p.Fill(ui.Rect{X: x0 + float32(a)*cw, Y: y, W: float32(z-a) * cw, H: lh}, bg, 3)
		}
	}
	// 缩进参考线：一像素宽的竖线，光标所在的块那条亮一些。
	for _, g := range row.Guides {
		c := pl.guide
		if g.Active {
			c = pl.guideNow
		}
		p.Fill(ui.Rect{X: x0 + float32(g.X)*cw, Y: y, W: 1, H: lh}, c, 0)
	}
	// 前景：同色的一段一起塑形。
	fgAt := func(b int) ui.Color {
		c := pl.fg[row.Base]
		if row.Base == style.CurrentLine || row.Base == style.Text {
			c = pl.text
		}
		for _, sp := range row.Spans {
			if int(sp.Start) <= b && b < int(sp.End) && style.LayerOf(sp.Style)&style.LayerFg != 0 {
				c = pl.fg[sp.Style]
			}
		}
		return c
	}
	i := 0
	for i < len(cells) {
		s := cells[i]
		ch := row.Text[s.start]
		if ch == '\t' || ch == ' ' {
			i++
			continue
		}
		if ch < 0x20 || ch == 0x7f { // 看不见的控制字符画成红色的 ␣
			gs := f.glyphs.shape("␣", fo)
			f.placeASCII(gs, 1)
			p.Glyphs(f.glyphs.tmp, x0+float32(s.x)*cw, y+fo.baseline, pl.danger)
			i++
			continue
		}
		color := fgAt(s.start)
		j := i + 1
		for j < len(cells) {
			n := cells[j]
			c := row.Text[n.start]
			if c == '\t' || c < 0x20 || c == 0x7f || fgAt(n.start) != color {
				break
			}
			j++
		}
		run := row.Text[cells[i].start:cells[j-1].end]
		gs := f.glyphs.shape(string(run), fo)
		// 字形的 Cluster 是段内第几个字（rune）：换算成格子。
		t := f.glyphs.tmp[:0]
		k, ri := i, 0
		for _, g := range gs {
			for k < j-1 && ri+utf8.RuneCount(row.Text[cells[k].start:cells[k].end]) <= g.Cluster {
				ri += utf8.RuneCount(row.Text[cells[k].start:cells[k].end])
				k++
			}
			sc := cells[k]
			g.X = float32(sc.x) * cw
			if sc.w > 1 && g.Advance < float32(sc.w)*cw { // 宽字符居中在它的格子里
				g.X += (float32(sc.w)*cw - g.Advance) / 2
			}
			t = append(t, g)
		}
		f.glyphs.tmp = t
		p.Glyphs(t, x0, y+fo.baseline, color)
		i = j
	}
	return endX
}

// paintPreedit 在光标处画输入法正在组的字（带下划线，盖住后面的字），返回组字光标的 x。
func (f *Frontend) paintPreedit(p *ui.Painter, x, y float32) float32 {
	pl, fo := f.pal, f.font
	var cells []float32 // 每个字的起点
	w := 0
	for _, r := range f.preedit {
		cells = append(cells, float32(w)*fo.charW)
		w += max(width.String(string(r)), 1)
	}
	width := float32(w) * fo.charW
	p.Fill(ui.Rect{X: x, Y: y, W: width, H: fo.lineH}, pl.editor, 0)
	gs := ui.Shape(f.preedit, fo.font())
	t := f.glyphs.tmp[:0]
	for _, g := range gs {
		if g.Cluster < len(cells) {
			g.X = cells[g.Cluster]
		}
		t = append(t, g)
	}
	f.glyphs.tmp = t
	p.Glyphs(t, x, y+fo.baseline, pl.text)
	p.Fill(ui.Rect{X: x, Y: y + fo.lineH - 3, W: width, H: 1}, pl.text.Alpha(0.6), 0)
	if f.preeditAt < len(cells) {
		return x + cells[f.preeditAt]
	}
	return x + width
}

// overviewW 是窗格右边那一条有多宽（同 Mayoi，在滚动条的地方）。
const overviewW = 12

// onStrip 报告窗格里的 x 是不是在右边那一条上。
func (f *Frontend) onStrip(ci int, x float32) bool {
	pf := &f.frame.Columns[ci].Pane
	st := &f.cols[ci]
	return pf.Overview != nil && st.size.W > 0 && x >= st.size.W-overviewW
}

// paintOverview 画窗格右边那一条（照着 Mayoi）：眼前这一屏一块淡底，改了的行一道（新加的绿、改过的蓝、删掉的一个红点），
// 查找的匹配一道短横，光标所在的行一道长横。
func (f *Frontend) paintOverview(p *ui.Painter, r ui.Rect, pf *view.PaneFrame) {
	ov := pf.Overview
	if ov == nil || ov.Lines <= 1 {
		return
	}
	pl := f.pal
	n := float32(ov.Lines)
	at := func(line int) float32 { return r.Y + float32(line)/n*r.H }
	x := r.X + r.W - overviewW + 1
	if ov.Visible > 0 && ov.Visible < ov.Lines {
		y := at(ov.Top)
		p.Fill(ui.Rect{X: x - 1, Y: y, W: overviewW, H: max(at(ov.Top+ov.Visible)-y, 6)}, pl.scrollbar.Alpha(0.35), 2)
	}
	for _, c := range ov.Changes {
		y, h := at(c.From), max(at(c.To)-at(c.From), 2)
		switch c.Mark {
		case view.MarkAdded:
			p.Fill(ui.Rect{X: x, Y: y, W: 3, H: h}, pl.added, 0)
		case view.MarkModified:
			p.Fill(ui.Rect{X: x, Y: y, W: 3, H: h}, pl.changed, 0)
		default:
			p.Fill(ui.Rect{X: x, Y: y - 1, W: 3, H: max(h, 3)}, pl.removed, 0)
		}
	}
	for _, line := range ov.Reviews {
		p.Fill(ui.Rect{X: x + 4, Y: at(line), W: 5, H: 3}, pl.accent, 1)
	}
	for _, m := range ov.Matches {
		p.Fill(ui.Rect{X: x + 4, Y: at(m), W: 5, H: 2}, pl.matchRing, 1)
	}
	p.Fill(ui.Rect{X: x, Y: at(ov.Caret) - 1, W: 9, H: 2}, pl.text.Alpha(0.6), 1)
}
