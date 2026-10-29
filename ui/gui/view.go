package gui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/view"
)

// 外壳的尺寸（DIP），与 Mayoi 一致。
const (
	titleBarH  = 30
	tabStripH  = 34
	crumbsH    = 26
	statusBarH = 24
)

// view 构建窗口：标题栏，资源管理器在编辑区旁边，最下面是状态栏，快速打开浮在上面。
//
// 整个工作台是一个可获得焦点的元素，按键全部经它交给内核，由内核按焦点区域（编辑器、
// 资源管理器、浮层）分派；标签、文件树的行这些元素都不可获得焦点，点它们焦点留在工作台上。
func (f *Frontend) view(c *ui.Context) {
	if !f.hasServices {
		f.services, f.hasServices = c.Services(), true
	}
	if theme, appearance, size, gen := f.ed.Settings(); gen != f.settingsGen { // 菜单里换了主题、外观、字号
		f.settingsGen = gen
		f.opts.Theme, f.opts.Appearance, f.opts.FontSize = theme, appearance, float32(size)
		// Saved or imported, the font list and line height apply without a restart.
		if family, lh := f.ed.Font(); family != "" && lh > 0 {
			f.opts.FontFamily, f.opts.LineHeight = family, float32(lh)
		}
		f.pal = nil
		for _, s := range f.allTerminals() { // 配置变更也应用到编辑区里的终端
			// Failed terminal tabs remain closable but have no native terminal to update.
			if s.t != nil {
				s.t.SetCopyOnSelect(f.ed.TerminalCopyOnSelect())
			}
		}
		if f.win != nil && mygo.FocusedWindow() == f.win {
			setAppMenu(f.menu())
			f.refreshDock()
		}
	}
	p := f.applyTheme(c)
	// A removed decoration has no paint callback to clear its last activity state.
	f.mascot, f.rain.moving = mascotState{}, false
	if f.font.charW == 0 || !f.font.is(f.opts) {
		f.font = newCodeFont(f.opts)
		f.glyphs.reset()
	}
	f.sizePanes(c)
	f.ed.Frame(&f.frame)
	f.reapTerminals()
	// Frame owns a stable active-column snapshot: tab presses during rendering
	// must not bind the single find input in two columns of the same frame.
	fr := &f.frame
	if f.ed.Quit() && !f.quitting { // 退出整个应用：每个窗口各自问要不要保存
		f.quitting = true
		mygo.App.Quit()
	}
	if f.win != nil && fr.Title != "" {
		f.win.SetTitle(fr.Title + "（" + f.badge + "）")
	}
	root := ui.Column(c.Key("workbench")).Fill().Background(p.chrome).Focusable().AutoFocus().Bind(&f.root).
		HandleInput(f.input).Label("yan")
	if !fr.Overlay.Visible {
		root.TextCaret(f.caretRect(c))
	}
	if fr.Focus == view.FocusEditor && fr.Columns[fr.Active].Pane.Kind != view.PaneTerminal && f.shownTerm != 0 {
		f.focusRoot = true // 终端仍在旁边可见时，列切换也必须把键盘交回文件。
	}
	if fr.Review == nil && !fr.Overlay.Visible && (f.focusRoot || !f.root.FocusWithin(c)) {
		f.root.Focus(c) // 浮层关掉后、关掉终端面板后，把键盘还给工作台
		f.focusRoot = false
		f.shownTerm = 0
	}
	if fr.Overlay.Visible || fr.Focus != view.FocusEditor || fr.Columns[fr.Active].Pane.Kind != view.PaneTerminal {
		f.shownTerm = 0
	}
	root.Children(func() {
		f.titleBar(c)
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			side := f.ed.PanelPosition()
			panel := !fr.Maximized // 最大化编辑区：面板不画，终端照样在跑
			if panel && side == "left" {
				f.panel(c, side) // outermost: beyond a sidebar on the same side
			}
			if fr.Sidebar.Visible && !fr.Sidebar.Right {
				f.sidebarPane(c)
			}
			main := ui.Column(c.Key("main")).Grow(1).MinWidth(0)
			if panel && side != "" && f.panelOpen && f.panelMax && len(f.terms) > 0 {
				main.Grow(0).Width(0).Invisible() // 面板最大化：编辑区还在，只是看不见
			}
			main.Children(func() {
				cols := ui.Row(c.Key("columns")).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Background(p.editor)
				if panel && side == "" && f.panelOpen && f.panelMax {
					cols.Grow(0).Height(0).Invisible() // 面板最大化：编辑区还在，只是看不见
				}
				width := cols.Bounds().W
				cols.Children(func() {
					first := true // 最大化时不画的列不算：第一列画出来的列左边不要线
					for i := range fr.Columns {
						grow := float32(max(fr.Columns[i].Weight, 0.01))
						if fr.Maximized {
							grow = 1 // maximized columns share the window evenly; their own widths come back on restore
						}
						box := ui.Column(c.Key(i)).Basis(0).Grow(grow).MinWidth(0)
						if fr.Columns[i].Hidden {
							// Still built, as the maximized panel's editor area is: its terminals and scroll stay.
							box.Grow(0).Width(0).Invisible()
						} else if !first {
							box.BorderWidth(0, 0, 0, 1).BorderColor(p.border)
						}
						edge := !first && !fr.Maximized
						first = first && fr.Columns[i].Hidden
						box.Children(func() {
							f.columnView(c, i)
							if edge && width > 0 { // 拖这一列左边的边：宽度在它和左边那一列之间挪
								f.splitter(c, edgeLeft, func(d float32) {
									f.ed.ResizeColumns(i, float64(d/width), float64(minColumnW/width))
								})
							}
						})
					}
				})
				if panel && side == "" {
					f.panel(c, side)
				}
			})
			if fr.Sidebar.Visible && fr.Sidebar.Right {
				f.sidebarPane(c)
			}
			if panel && side == "right" {
				f.panel(c, side)
			}
		})
		f.statusBar(c)
	})
	if fr.Guide.Visible && !fr.Overlay.Visible {
		f.guideOfferView(c)
	}
	if fr.Overlay.Visible {
		f.overlayView(c)
	}
	f.reviewView(c)
	if !fr.Find.Visible {
		f.findFocus = view.FindNone // 下次打开时重新把焦点交给输入框
	}
	if !fr.Sidebar.Visible || fr.Sidebar.View != view.SideSearch {
		f.searchFocus = view.SearchNone
	}
}

func (f *Frontend) sidebarPane(c *ui.Context) {
	right := f.frame.Sidebar.Right
	edge, sign := edgeRight, float32(1)
	box := ui.Column(c.Key("sidebar")).Label("侧边栏").Width(f.opts.SidebarWidth).Shrink(0).BorderColor(f.pal.border)
	if right {
		edge, sign = edgeLeft, -1
		box.BorderWidth(0, 0, 0, 1)
	} else {
		box.BorderWidth(0, 1, 0, 0)
	}
	box.Children(func() {
		f.sidebar(c)
		f.splitter(c, edge, func(d float32) { f.opts.SidebarWidth = min(max(f.opts.SidebarWidth+sign*d, minSidebarW), maxSidebarW) })
	})
}

// sizePanes 按上一帧排好的位置算出每个代码窗格能放几列几行，告诉内核。
func (f *Frontend) sizePanes(c *ui.Context) {
	n := f.ed.Columns()
	for len(f.cols) < n {
		f.cols = append(f.cols, colState{reviewHover: -1, split: splitState{hover: -1, review: -1}})
	}
	f.cols = f.cols[:n]
	for i := range f.cols {
		b := f.cols[i].pane.Bounds(c)
		if b.W <= 0 || b.H <= 0 {
			continue // 还没排过版：内核用默认尺寸，下一帧再说
		}
		w := int((b.W - codePadR) / f.font.charW)
		h := int((b.H - codePadY) / f.font.lineH)
		f.ed.SetPaneSize(i, view.Size{W: view.Unit(max(w, 1)), H: view.Unit(max(h, 1))})
	}
	if b := f.tree.Handle.Bounds(c); b.H > 0 {
		f.ed.SetSidebarRows(int(b.H / treeRowH))
	}
}

func (f *Frontend) titleBar(c *ui.Context) {
	p := f.pal
	styleWindowControls(p.muted, p.text) // Linux 的窗口按钮跟着主题的颜色
	bar := c.TitleBar()
	f.nativeBar = bar
	ui.Row(c).Height(max(bar.Height, titleBarH)).Padding(0, bar.Right+8, 0, bar.Left+10).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(p.border).DragWindow().Children(func() {
		ui.Row(c).Width(120).Gap(2).Children(func() {
			if !nativeMenuBar() { // Windows、Linux：没有占一整行的原生菜单栏，菜单从这个按钮打开
				f.iconButton(c, iconMenu, "菜单", false).Menu(func(m *ui.Menu) { f.populateMenu(m, f.menu().Items()) })
			}
			if f.iconButton(c, iconSidebar, "切换侧边栏 ("+f.ed.KeyHint("workbench.action.toggleSidebarVisibility")+")", f.frame.Sidebar.Visible).Clicked() {
				f.ed.Run("workbench.action.toggleSidebarVisibility")
			}
		})
		ui.Row(c).Grow(1).MinWidth(0).Justify(ui.Center).AlignItems(ui.Center).Children(func() { f.commandCenter(c) })
		ui.Row(c).Width(120).Justify(ui.End).Gap(2).Children(func() {
			f.maximizeButton(c)
			if f.iconButton(c, iconColumns, "向右拆分 ("+f.ed.KeyHint("workbench.action.splitEditor")+")", false).Clicked() {
				f.ed.Run("workbench.action.splitEditor")
			}
		})
	})
}

// maximizeButton 最大化编辑区（合并视图是三栏一起）或者还原；最大化时它亮着。
func (f *Frontend) maximizeButton(c *ui.Context) {
	icon, label := iconMaximize, "最大化编辑区"
	if f.frame.Maximized {
		icon, label = iconMinimize, "还原布局"
	}
	if k := f.ed.KeyHint("workbench.action.toggleMaximizeEditorGroup"); k != "" {
		label += " (" + k + ")"
	}
	if f.iconButton(c, icon, label, f.frame.Maximized).Clicked() {
		f.ed.Run("workbench.action.toggleMaximizeEditorGroup")
	}
}

// commandCenter 是标题栏中间的框：写着当前文件和文件夹，点它打开快速打开。
func (f *Frontend) commandCenter(c *ui.Context) {
	p := f.pal
	b := ui.Row(c.Key("command-center")).Height(24).Width(420).MaxWidthPercent(100).Shrink(1).Padding(0, 10).Gap(8).
		Radius(6).Justify(ui.Center).AlignItems(ui.Center).Border(1, p.border).Background(p.hover).Label("快速打开")
	f.centerW = b.Bounds().W // 上一帧在这个窗口里排出来的宽度
	hovered := b.Hovered()
	if hovered {
		b.Background(p.hover.Alpha(2)).Border(1, p.border.Mix(p.muted, 0.3))
	}
	b.Children(func() {
		col := p.faint
		if hovered {
			col = p.muted
		}
		ui.Icon(c, iconSearch).FontSize(12).TextColor(col)
		name, folder := "", ""
		fr := &f.frame
		if a := &fr.Columns[fr.Active]; a.Active >= 0 && a.Active < len(a.Tabs) {
			name = a.Tabs[a.Active].Name
			if a.Tabs[a.Active].Dirty {
				name = "● " + name
			}
		}
		if root := f.ed.Root(); root != "" {
			folder = filepath.Base(root)
		}
		// 窄窗口里先不写文件夹，再截短文件名（照着 Mayoi v0.8.0）。
		if name != "" && folder != "" && f.centerW > 0 && textWidth(c, name, 12)+textWidth(c, folder, 12)+20+8+12+8 > f.centerW {
			folder = ""
		}
		switch {
		case name != "" && folder != "":
			ui.Text(c, name).FontSize(12).FontWeight(500).TextColor(p.text).SingleLine().Shrink(1)
			ui.Text(c, folder).FontSize(12).TextColor(p.faint).SingleLine().Shrink(1)
		case name != "" && folder == "" && f.ed.Root() != "":
			ui.Text(c, name).FontSize(12).FontWeight(500).TextColor(p.text).SingleLine().Shrink(1)
		case name != "" || folder != "":
			ui.Text(c, name+folder).FontSize(12).FontWeight(500).TextColor(p.muted).SingleLine().Shrink(1)
		default:
			ui.Text(c, "砚 yan").FontSize(12).FontWeight(500).TextColor(p.muted).SingleLine()
		}
	})
	if b.Clicked() {
		f.ed.Run("workbench.action.quickOpen")
	}
}

// statusFit 挑出窄窗口里放得下的状态栏右边的项：放不下时先去掉留得最不久的（编码最先，位置最后），
// 分支排在缩进和语言之间（照着 Mayoi v0.8.0）。左边的文件夹至少留 120。
func (f *Frontend) statusFit(c *ui.Context, st *view.Status) ([]view.StatusItem, bool) {
	items := slices.Clone(st.Right)
	branch := st.Branch != ""
	avail := f.statusW - 20 - 120 - textWidth(c, f.badge, 10.5) - 12 - 14
	if f.hasClawd() {
		avail -= 30 + 14
	}
	if st.Busy != "" {
		avail -= textWidth(c, st.Busy, 11.5) + 14
	}
	if updateShown != "" {
		avail -= textWidth(c, "新版本 "+updateShown, 11.5) + 16 + 14
	}
	if f.statusW <= 0 {
		return items, branch // 第一帧还不知道多宽
	}
	need := func() float32 {
		w := float32(0)
		for _, it := range items {
			w += textWidth(c, it.Text, 11.5) + 14
		}
		if branch {
			w += textWidth(c, st.Branch, 11.5) + 17 + 14
			if st.Sync {
				w += 18 + 5
			}
		}
		return w
	}
	const branchKeep = view.KeepIndent // 分支比缩进留得久，比语言早去掉
	for need() > avail {
		low, at := view.KeepPosition+1, -1
		for i, it := range items {
			if it.Keep < low && it.Keep < view.KeepPosition {
				low, at = it.Keep, i
			}
		}
		switch {
		case branch && (at < 0 || low > branchKeep):
			branch = false
		case at >= 0:
			items = slices.Delete(items, at, at+1)
		default:
			return items, branch // 只剩位置：不再去掉
		}
	}
	return items, branch
}

// textWidth 是 s 用界面字体、字号 size 排出来有多宽。
func textWidth(c *ui.Context, s string, size float32) float32 {
	w, _ := ui.ShapeText(s, ui.Font{Family: c.Theme().Font, Size: size}, 0).Size()
	return w
}

// iconButton 是显示一个图标的小方按钮，悬停时有提示。它不获得焦点：键盘留在工作台上。
func (f *Frontend) iconButton(c *ui.Context, icon *ui.SVG, label string, on bool) ui.Element {
	p := f.pal
	b := ui.Box(c).Size(26, 24).Radius(5).Justify(ui.Center).AlignItems(ui.Center).Label(label).Tooltip(label)
	col := p.muted
	switch {
	case b.Pressed(), b.Hovered():
		b.Background(p.hover)
		col = p.text
	}
	if on {
		col = p.text
	}
	b.Children(func() { ui.Icon(c, icon).FontSize(15).TextColor(col) })
	return b
}

// 拖动改大小的地方：列与列之间、侧边栏和面板的边。
const (
	minColumnW  = 160 // 一列最窄拖到多宽
	minSidebarW = 170
	maxSidebarW = 640
)

type edge uint8

const (
	edgeLeft edge = iota
	edgeRight
	edgeTop
)

// splitter 是父元素一条边上能拖的窄条（照着 Mayoi）：指针移上去、拖着时亮一道强调色，拖动时把挪了多少交给 drag。
func (f *Frontend) splitter(c *ui.Context, at edge, drag func(d float32)) {
	h := ui.Box(c).Absolute()
	switch at {
	case edgeTop:
		h.Top(0).Left(0).Right(0).Height(5).Cursor(ui.CursorResizeNS)
	case edgeLeft:
		h.Top(0).Bottom(0).Left(0).Width(5).Cursor(ui.CursorResizeEW)
	default:
		h.Top(0).Bottom(0).Right(0).Width(5).Cursor(ui.CursorResizeEW)
	}
	if h.Hovered() || h.Dragging() {
		h.Children(func() {
			line := ui.Box(c).Absolute().Background(f.pal.accentDim)
			switch at {
			case edgeTop:
				line.Top(0).Left(0).Right(0).Height(2)
			case edgeLeft:
				line.Top(0).Bottom(0).Left(0).Width(2)
			default:
				line.Top(0).Bottom(0).Right(0).Width(2)
			}
		})
	}
	if dx, dy, held := h.Dragged(); held && (dx != 0 || dy != 0) {
		if at == edgeTop {
			drag(dy)
		} else {
			drag(dx)
		}
	}
}

// columnView 是一列：标签条、面包屑、正在显示的代码窗格或欢迎页。
func (f *Frontend) columnView(c *ui.Context, i int) {
	col := &f.frame.Columns[i]
	if len(col.Tabs) > 0 {
		f.tabStrip(c, i)
		if col.Pane.Kind != view.PaneTerminal {
			f.breadcrumbs(c, i)
		}
	} else if len(f.frame.Columns) > 1 {
		f.emptyColumnHeader(c, i)
	}
	pane := ui.Box(c.Key("pane")).Grow(1).MinHeight(0).Clip()
	if col.Pane.Kind != view.PaneCode {
		pane.Bind(&f.cols[i].pane)
	}
	defer f.paneDrops(c, pane, i)
	if col.Pane.Kind == view.PaneTerminal {
		f.termPane(c, pane, i)
		return
	}

	switch col.Pane.Kind {
	case view.PaneEmpty:
		pane.Children(func() {
			if len(col.Pane.Hints) == 0 && len(f.frame.Columns) > 1 {
				f.emptyColumn(c, i)
			} else {
				f.welcome(c, &col.Pane)
			}
		})
	case view.PaneImage:
		f.imagePane(c, pane, i)
	case view.PaneMarkdown:
		f.markdownPane(c, pane, i)
	case view.PaneMessage:
		pane.Children(func() {
			ui.Text(c, col.Pane.Message).FontSize(13).TextColor(f.pal.muted).Padding(24)
		})
	default:
		f.commitDiffView(c, pane, i)
	}
}

// emptyColumnHeader is an empty column's header, where its tab strip would be: the lock button,
// and a button that closes the column (an empty column has no tab to close).
func (f *Frontend) emptyColumnHeader(c *ui.Context, ci int) {
	p := f.pal
	col := &f.frame.Columns[ci]
	ui.Row(c.Key("empty-header")).Height(tabStripH).Background(p.chrome).Padding(0, 6, 0, 10).AlignItems(ui.Center).Gap(2).
		BorderWidth(0, 0, 1, 0).BorderColor(p.border).Children(func() {
		label := "空列"
		if col.Locked {
			label = "已锁定的空列"
		}
		ui.Text(c, label).FontSize(12).TextColor(p.faint).SingleLine()
		ui.Spacer(c)
		f.columnLockButton(c, ci)
		if col.Closable && f.iconButton(c, iconX, "关闭这一列", false).Clicked() {
			f.ed.CloseColumn(ci)
		}
	})
}

// columnLockButton locks or unlocks column ci.
func (f *Frontend) columnLockButton(c *ui.Context, ci int) {
	tip := "锁定这一列：打开的文件和新终端去别的列"
	if f.frame.Columns[ci].Locked {
		tip = "解锁这一列"
	}
	if f.iconButton(c, iconLock, tip, f.frame.Columns[ci].Locked).Clicked() {
		f.ed.FocusColumn(ci)
		f.ed.Run("view.lockColumn")
	}
}

// emptyColumn is an empty column beside the one showing the welcome page: what the column is
// for, and how to fill or close it, instead of a second copy of the page.
func (f *Frontend) emptyColumn(c *ui.Context, ci int) {
	p := f.pal
	col := &f.frame.Columns[ci]
	text := "把标签拖到这里，或在这一列里打开文件"
	if col.Locked {
		text = "锁定的列：打开的文件和新终端去别的列；把标签拖到这里放进来"
	}
	ui.Column(c).Fill().Justify(ui.Center).AlignItems(ui.Center).Padding(16).Children(func() {
		ui.Text(c, text).FontSize(12.5).TextColor(p.faint).TextAlign(ui.Center).MaxWidth(260)
		if col.Closable {
			b := ui.Box(c.Key("empty-close")).Margin(12, 0, 0, 0).Padding(4, 10).Radius(6).Label("关闭这一列")
			if b.Hovered() {
				b.Background(p.hover)
			}
			b.Children(func() { ui.Text(c, "关闭这一列").FontSize(12.5).TextColor(p.accent) })
			if b.Clicked() {
				f.ed.CloseColumn(ci)
			}
		}
	})
}

// tabDrag 是拖着的一个标签（照着 Mayoi）：拖到标签条上别的位置、别的列的标签条上，放在那里。
// 只认这个窗口里拖的（别的窗口是另一个编辑器）。
type tabDrag struct {
	f       *Frontend
	col, at int
	s       *termSession // 非 nil 时来自终端面板；共用一种负载，避免覆盖同一落点的类型注册
	match   *searchRef   // 非 nil 时是搜索结果：放下的地方打开文件并选中这一处
}

// searchRef is a search result being dragged: its row, and its line, which the core checks
// against the results at the drop.
type searchRef struct{ row, line int }

// tabStrip 是一列的标签条：点一下显示，× 关闭，圆点表示有未保存的修改，斜体是预览标签。
// 标签能拖：沿着标签条挪位置，或者拖到别的列的标签条上；放下的地方画一道强调色的竖线。
func (f *Frontend) tabStrip(c *ui.Context, ci int) {
	p := f.pal
	col := &f.frame.Columns[ci]
	focused := ci == f.frame.Active && f.frame.Focus == view.FocusEditor
	// drop 把拖来的标签放在第 at 个标签前面（at 是标签数时放在最后）。
	drop := func(e ui.Element, at int) bool {
		d, over := ui.DragOver[tabDrag](e)
		if over && d.f != f {
			over = false
		}
		if d, ok := ui.Drop[tabDrag](e); ok && d.f == f {
			f.do(func() {
				if m := d.match; m != nil {
					f.ed.DropSearchMatch(ci, m.row, m.line, 0)
				} else if d.s != nil {
					f.panelToColumn(d.s, ci, at)
				} else {
					f.ed.MoveTab(d.col, d.at, ci, at)
				}
			})
		}

		opts := transfer.DropOptions{Formats: []transfer.Format{transfer.FileList}}
		if _, hovering := ui.DataDragOver(e, opts); hovering {
			over = true
		}
		if data, ok := ui.DropData(e, opts); ok {
			if paths, err := data.Data.Files(); err == nil {
				f.do(func() { f.ed.DropFilesAt(ci, at, paths) })
			}
		}
		return over
	}
	ui.Row(c).Height(tabStripH).Background(p.chrome).AlignItems(ui.Center).Children(func() {
		// 标签条底边那条线；正在显示的标签断开它，和下面的正文连在一起。
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(1).Background(p.border)
		strip := ui.ScrollHorizontal(c.Key("tabs")).Grow(1).Height(tabStripH).ScrollbarInsets(0, 0, 0, 0)
		atEnd := drop(strip, len(col.Tabs)) // 最后一个标签后面的空处：放在最后
		strip.Children(func() {
			ui.Row(c).Height(tabStripH).AlignItems(ui.Stretch).Children(func() {
				for i, t := range col.Tabs {
					active := i == col.Active
					// 终端的目录、标题可以重复；身份不能跟着标题改变。
					key := any(t.Path)
					if t.Terminal != 0 {
						key = t.Terminal
					}
					tip := f.rel(t.Path)
					if t.History {
						tip = t.Name
					}
					tab := ui.Row(c.Key(key)).Padding(0, 4, 0, 10).Gap(6).AlignItems(ui.Center).
						BorderWidth(0, 1, 0, 0).BorderColor(p.border).Label(t.Name).Tooltip(tip).
						Drag(tabDrag{f: f, col: ci, at: i})
					if t.Terminal != 0 && tab.Dragging() {
						f.dragTermFrames = 2
					}
					// The middle button closes the tab without showing it, as a browser's tabs do.
					box := tab.LayoutBounds()
					tab = tab.HandleInput(func(ev ui.InputEvent) bool { return f.tabMiddle(key, ev, box) })
					over := drop(tab, i)
					hovered := tab.Hovered()
					switch {
					case active:
						tab.Background(p.editor)
					case hovered:
						tab.Background(p.chrome.Mix(p.editor, 0.5))
					}
					if tab.Pressed() && f.middle != key {
						f.focusRoot = true
						f.ed.ClickTab(ci, i, false)
					}
					if tab.DoubleClicked() {
						f.ed.ClickTab(ci, i, true)
					}
					tab.ContextMenu(func(m *ui.Menu) { f.tabMenu(m, ci, i, t) })
					if active {
						tab.ScrollIntoView()
					}
					tab.Children(func() {
						if !active {
							ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(1).Background(p.border)
						}
						switch {
						case over: // 拖着的标签放在这一个前面
							ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).Width(2).Background(p.accent)
						case active && focused:
							ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(2).Background(p.accent)
						case active:
							ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(2).Background(p.faint)
						}
						icon, tint := fileIcon(p, t.Name)
						if t.Terminal != 0 {
							icon, tint = iconTerminal, p.muted
						}
						ui.Icon(c, icon).FontSize(14).TextColor(tint)
						name := ui.Text(c, t.Name).FontSize(13).SingleLine().MaxWidth(220)
						if t.Preview {
							name.Italic() // 预览：下一个预览的文件会顶替它
						}
						if active {
							name.TextColor(p.text)
						} else {
							name.TextColor(p.muted)
						}
						// 未保存的圆点，指针移上去变成 ×。
						x := ui.Box(c).Size(20, 20).Radius(4).Justify(ui.Center).AlignItems(ui.Center).Label("关闭 " + t.Name)
						xh := x.Hovered()
						if xh {
							x.Background(p.hover)
						}
						x.Children(func() {
							switch {
							case t.Dirty && !xh:
								ui.Box(c).Size(8, 8).Radius(4).Background(p.text.Alpha(0.7))
							case active || hovered || xh:
								ui.Icon(c, iconX).FontSize(13).TextColor(p.muted)
							}
						})
						x.OnClick(func() { f.ed.CloseTab(ci, i) })
					})
				}
				if atEnd {
					ui.Box(c).Width(2).Background(p.accent)
				}
			})
		})
		ui.Row(c).Padding(0, 6, 0, 4).Children(func() {
			// A merge beside other columns is cramped: its focused column offers the window to it.
			if col.Merge && !f.frame.Maximized && ci == f.frame.Active {
				f.maximizeButton(c)
			}
			f.columnLockButton(c, ci)
			if f.iconButton(c, iconColumns, "向右拆分 ("+f.ed.KeyHint("workbench.action.splitEditor")+")", false).Clicked() {
				f.ed.FocusColumn(ci)
				f.ed.Run("workbench.action.splitEditor")
			}
		})
	})
}

// middleButton is the middle pointer button's number in MyGo's input events.
const middleButton = 2

// tabMiddle closes a tab on a middle click (ported from Mayoi, cmd/mayoi/tabs.go at 46990a1). The
// press is taken so the release comes back here; released off the tab, it closes nothing.
func (f *Frontend) tabMiddle(key any, ev ui.InputEvent, box ui.Rect) bool {
	if ev.Button != middleButton {
		if ev.Kind == ui.InputPointerDown {
			f.middle = nil // a release that never came must not keep the tab from showing
		}
		return false
	}
	switch ev.Kind {
	case ui.InputPointerDown:
		f.middle = key
		return true
	case ui.InputPointerUp:
		on := box.W == 0 || ev.X >= 0 && ev.Y >= 0 && ev.X < box.W && ev.Y < box.H
		if f.middle == key && on {
			for ci, col := range f.frame.Columns {
				for i, t := range col.Tabs {
					k := any(t.Path)
					if t.Terminal != 0 {
						k = t.Terminal
					}
					if k == key {
						f.ed.CloseTab(ci, i)
						f.middle = nil
						return true
					}
				}
			}
		}
		f.middle = nil
		return true
	}
	return false
}

// tabMenu 是一个标签的右键菜单（照着 Mayoi）：先显示这个标签，再跑命令。
func (f *Frontend) tabMenu(m *ui.Menu, ci, i int, t view.Tab) {
	run := func(cmd string) {
		f.ed.ClickTab(ci, i, false)
		f.ed.Run(cmd)
	}
	n := len(f.frame.Columns[ci].Tabs)
	if m.Item("关闭").Chosen() {
		f.ed.CloseTab(ci, i)
	}
	if m.Item("关闭其他").Disabled(n < 2).Chosen() {
		run("workbench.action.closeOtherEditors")
	}
	if m.Item("关闭全部").Chosen() {
		run("workbench.action.closeEditorsInGroup")
	}
	m.Separator()
	if m.Item("向右拆分").Chosen() {
		run("workbench.action.splitEditor")
	}
	last := ci == len(f.frame.Columns)-1
	if m.Item("移到下一列").Disabled(n < 2 && last).Chosen() {
		run("workbench.action.moveEditorToNextGroup")
	}
	if m.Item("移到上一列").Disabled(n < 2 && ci == 0).Chosen() {
		run("workbench.action.moveEditorToPreviousGroup")
	}
	if m.Item("锁定列").Checked(f.frame.Columns[ci].Locked).Chosen() {
		run("view.lockColumn")
	}
	if t.Terminal != 0 {
		m.Separator()
		if m.Item("移到右侧锁定列").Chosen() {
			run("view.terminalLockRight")
		}
		if m.Item("移到面板").Chosen() {
			f.columnToPanel(t.Terminal)
		}
		return
	}
	if t.Diff {
		return
	}
	m.Separator()
	if m.Item("复制路径").Chosen() {
		run("copyFilePath")
	}
	if m.Item("复制相对路径").Chosen() {
		run("copyRelativeFilePath")
	}
	if m.Item("在" + app.FileManager(hostOS) + "中显示").Chosen() {
		run("revealFileInOS")
	}
}

// breadcrumbs 写出当前标签的文件在项目里的路径。
func (f *Frontend) breadcrumbs(c *ui.Context, ci int) {
	p := f.pal
	col := &f.frame.Columns[ci]
	if col.Active < 0 || col.Active >= len(col.Tabs) {
		return
	}
	tab := col.Tabs[col.Active]
	parts := strings.Split(f.rel(tab.Path), "/")
	untitled := tab.Path == "" && tab.Terminal == 0
	if tab.History || untitled {
		parts = []string{tab.Name}
	}
	var dirs []string
	if !tab.History && !untitled && tab.Terminal == 0 {
		dirs = crumbDirs(f.ed.Root(), tab.Path, parts)
	}
	ui.Row(c).Height(crumbsH).Padding(0, 14).Gap(4).AlignItems(ui.Center).Background(p.editor).Clip().Children(func() {
		for i, part := range parts {
			last := i == len(parts)-1
			if last {
				icon, tint := fileIcon(p, part)
				ui.Icon(c, icon).FontSize(13).TextColor(tint)
			}
			if i < len(dirs) && dirs[i] != "" { // 文件夹：点开一个列着它的文件的框
				dir := dirs[i]
				b := ui.Row(c.Key("crumb:"+dir)).Height(crumbsH-6).Padding(0, 4).Radius(4).AlignItems(ui.Center).Shrink(0).
					Label("文件夹 " + part).Tooltip(dir)
				open := f.crumbs.open && f.crumbs.col == ci && f.crumbs.dir == dir
				if open || b.Hovered() {
					b.Background(p.hover)
				}
				b.Children(func() { ui.Text(c, part).FontSize(12).SingleLine().TextColor(p.muted) })
				if b.Clicked() {
					f.openCrumbs(ci, tab.Path, dir)
				}
				if open {
					f.crumbsBox(c, b)
				}
				ui.Icon(c, iconChevronRight).FontSize(11).TextColor(p.faint)
				continue
			}
			t := ui.Text(c, part).FontSize(12).SingleLine().TextColor(p.muted).Shrink(0)
			if last {
				t.TextColor(p.text)
			}
			if !last || len(col.Crumbs) > 0 {
				ui.Icon(c, iconChevronRight).FontSize(11).TextColor(p.faint)
			}
		}
		// 光标所在的块：点一下跳到开始它的那一行。
		for i, cr := range col.Crumbs {
			b := ui.Row(c.Key(fmt.Sprint("scope:", i))).Height(crumbsH-6).Padding(0, 5).Radius(4).
				AlignItems(ui.Center).Shrink(1).MinWidth(0).Label("转到 " + cr.Label)
			if b.Hovered() {
				b.Background(p.hover)
			}
			b.Children(func() {
				ui.Text(c, cr.Label).FontSize(12).Font(f.opts.FontFamily).SingleLine().TextColor(p.muted)
			})
			if b.Clicked() {
				f.ed.JumpToLine(ci, cr.Line)
			}
			if i < len(col.Crumbs)-1 {
				ui.Icon(c, iconChevronRight).FontSize(11).TextColor(p.faint)
			}
		}
		if ci == f.frame.Active {
			f.reviewButton(c, "+ 意见", "review.comment")
			if f.frame.ReviewCount > 0 {
				f.reviewButton(c, "发送意见", "review.send")
			}
		}
		if ext := strings.ToLower(filepath.Ext(tab.Path)); !tab.Diff && (ext == ".md" || ext == ".markdown" || ext == ".mmd" || ext == ".mermaid" || ext == ".svg") {
			ui.Spacer(c)
			if f.iconButton(c, iconFileText, "切换预览", false).Clicked() {
				f.ed.ClickTab(ci, col.Active, true)
				f.ed.Run("yan.svg.togglePreview")
			}
			if f.iconButton(c, iconColumns, "在右侧打开预览", false).Clicked() {
				f.ed.ClickTab(ci, col.Active, true)
				f.ed.Run("markdown.showPreviewToSide")
			}
		}
		if col.Tabs[col.Active].History {
			if f.iconButton(c, iconListTree, "显示/隐藏提交文件列表", col.Pane.CommitFiles != nil && col.Pane.CommitFiles.Visible).Clicked() {
				f.ed.Run("git.toggleCommitFiles")
			}
		}
		if col.Tabs[col.Active].Diff { // 改动视图：换排法（自动、并排、一栏），照着 Mayoi
			ui.Spacer(c)
			if f.iconButton(c, iconColumns, "换排法：自动、并排、一栏", false).Clicked() {
				f.ed.Run("toggle.diff.renderSideBySide")
			}
		}
	})
}

// rel 返回 path 相对项目根的路径。
func (f *Frontend) rel(path string) string {
	if root := f.ed.Root(); root != "" && strings.HasPrefix(path, root+string(filepath.Separator)) {
		return filepath.ToSlash(path[len(root)+1:])
	}
	return path
}

// welcome 是没有标签的列：名字和几个快捷键。
func (f *Frontend) welcome(c *ui.Context, pane *view.PaneFrame) {
	p := f.pal
	rain := f.opts.Theme == "matrix" && f.ed.ThemeEffects()
	page := ui.Column(c).Fill().Clip().Justify(ui.Center).AlignItems(ui.Center)
	if rain {
		reduce := c.Preferences().ReduceMotion
		page.Draw(func(pt *ui.Painter, r ui.Rect) { f.drawRain(pt, r, reduce || f.blurred) })
	}
	page.Children(func() {
		content := ui.Column(c).Padding(24).Gap(18).AlignItems(ui.Center)
		if rain {
			content.Radius(12).Background(p.editor.Alpha(0.9)).Border(1, p.border)
		}
		content.Children(func() {
			if f.hasClawd() {
				f.clawdView(c, "welcome-clawd", 4, p.editor)
			} else {
				ui.Text(c, "砚").FontSize(64).FontWeight(300).TextColor(p.faint)
			}
			ui.Column(c).Gap(8).Children(func() {
				for _, h := range pane.Hints { // each a button: a click does it, the keys teach it
					row := ui.Row(c.Key("hint:"+h.Command)).Gap(24).Padding(3, 8).Radius(6).Label(h.Title).Cursor(ui.CursorPointer)
					title := p.muted
					if row.Hovered() {
						row.Background(p.hover)
						title = p.text
					}
					row.Children(func() {
						ui.Text(c, h.Title).FontSize(13).TextColor(title).Width(180).TextAlign(ui.End)
						ui.Text(c, h.Keys).FontSize(13).TextColor(p.text).Width(120)
					})
					if row.Clicked() && h.Command != "" {
						f.ed.Run(h.Command)
					}
				}
			})
			// New here: the feature guide teaches the editor by doing, as an IDE's Learn panel does.
			learn := ui.Row(c.Key("welcome-learn")).Gap(6).Padding(4, 10).Radius(6).AlignItems(ui.Center).Label("功能导览")
			if learn.Hovered() {
				learn.Background(p.hover)
			}
			learn.Children(func() {
				ui.Icon(c, iconLearn).FontSize(14).TextColor(p.accent)
				ui.Text(c, "第一次用？打开功能导览，边做边学").FontSize(13).TextColor(p.accent)
			})
			if learn.Clicked() {
				f.ed.Run("yan.guide.open")
			}
			if ui.Button(c.Key("welcome-theme"), "颜色主题…").Label("颜色主题…").Clicked() {
				f.ed.Run("workbench.action.selectTheme")
			}
			if updateShown != "" {
				up := ui.Row(c.Key("welcome-update")).Gap(6).Padding(4, 10).Radius(6).AlignItems(ui.Center).Label("更新到 " + updateShown)
				if up.Hovered() {
					up.Background(p.hover)
				}
				up.Children(func() {
					ui.Icon(c, iconPull).FontSize(14).TextColor(p.accent)
					ui.Text(c, "有新版本 "+updateShown+"：点击更新").FontSize(13).TextColor(p.accent)
				})
				if up.Clicked() {
					installUpdate()
				}
			}
		})
	})
}

func (f *Frontend) statusBar(c *ui.Context) {
	p := f.pal
	st := &f.frame.Status
	row := ui.Row(c.Key("status")).Height(statusBarH).Padding(0, 10).Gap(14).AlignItems(ui.Center).Background(p.status).
		BorderWidth(1, 0, 0, 0).BorderColor(p.border)
	f.statusW = row.Bounds().W // 上一帧在这个窗口里排出来的宽度
	row.Children(func() {
		if f.hasClawd() {
			f.clawdView(c, "status-clawd", 1, p.status)
		}
		text := func(s string) ui.Element {
			return ui.Text(c, s).FontSize(11.5).TextColor(p.statusFg).SingleLine()
		}
		if st.Message != "" {
			text(st.Message).TextColor(p.text).Shrink(1)
			if a := st.Action; a.Command != "" { // 跟着消息的按钮，如提交之后的「撤销」
				b := ui.Box(c.Key("status-action")).Height(18).Padding(0, 8).Radius(4).Justify(ui.Center).AlignItems(ui.Center).
					Background(p.accent.Alpha(0.18)).Label(a.Label)
				if b.Hovered() {
					b.Background(p.accent.Alpha(0.3))
				}
				b.Children(func() { ui.Text(c, a.Label).FontSize(11.5).FontWeight(600).TextColor(p.accent) })
				if b.Clicked() {
					repositoryEditor{Editor: f.ed, id: a.RepositoryID}.Run(a.Command)
				}
			}
		} else {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
				ui.Icon(c, iconFolder).FontSize(12).TextColor(p.statusFg)
				text(st.Left).Shrink(1)
			})
		}
		ui.Spacer(c)
		if st.Busy != "" {
			text(st.Busy).TextColor(p.text)
		}
		if f.frame.ReviewCount > 0 {
			f.reviewButton(c, fmt.Sprintf("评审 %d", f.frame.ReviewCount), "review.list")
		}
		if reading := f.markdownReadingTime(); reading != "" && f.statusW > 700 {
			text(reading)
		}
		right, branch := f.statusFit(c, st)
		if branch {
			b := ui.Row(c.Key("status-branch")).Gap(5).AlignItems(ui.Center).Label("切换分支").Tooltip("切换或新建分支")
			col := p.statusFg
			if b.Hovered() {
				col = p.text
			}
			b.Children(func() {
				ui.Icon(c, iconBranch).FontSize(12).TextColor(col)
				text(st.Branch).TextColor(col)
			})
			if b.Clicked() {
				f.ed.Run("git.checkout") // 和源代码管理同一个能搜索的分支浮层
			}
			if st.Sync {
				label, tip := "同步", "同步：先拉取（只快进），再推送"
				if st.Publish {
					label, tip = "发布分支", "发布分支：推送并设置上游"
				}
				s := ui.Box(c.Key("status-sync")).Size(18, 18).Radius(4).Justify(ui.Center).AlignItems(ui.Center).Label(label).Tooltip(tip)
				col := p.statusFg
				if s.Hovered() {
					s.Background(p.hover)
					col = p.text
				}
				icon := iconPull
				if st.Publish {
					icon = iconPush
				}
				s.Children(func() { ui.Icon(c, icon).FontSize(12).TextColor(col) })
				if s.Clicked() {
					f.ed.Run("git.sync")
				}
			}
		}
		for _, it := range right {
			item := text(it.Text)
			if it.Command != "" {
				item.Label("设置 " + it.Text).Tooltip("点击设置 " + it.Text)
				if item.Hovered() {
					item.TextColor(p.text)
				}
				if item.Clicked() {
					f.ed.Run(it.Command)
				}
			}
		}
		// 版本标签：一眼看出用的是预览版、哪一版
		if updateShown != "" { // 检查到新版本：点一下打开更新窗口
			u := ui.Row(c.Key("status-update")).Gap(4).AlignItems(ui.Center).Label("更新到 " + updateShown).Tooltip("有新版本 " + updateShown + "：点击更新")
			col := p.accent
			if u.Hovered() {
				col = p.text
			}
			u.Children(func() {
				ui.Icon(c, iconPull).FontSize(12).TextColor(col)
				ui.Text(c, "新版本 "+updateShown).FontSize(11.5).TextColor(col).SingleLine()
			})
			if u.Clicked() {
				installUpdate()
			}
		}
		ui.Row(c.Key("version-badge")).Padding(1, 6).Radius(4).Background(p.accentDim).Children(func() {
			ui.Text(c, f.badge).FontSize(10.5).TextColor(p.accent).SingleLine()
		})
	})
}
