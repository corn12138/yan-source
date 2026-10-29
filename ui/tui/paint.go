package tui

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/text/width"
	"github.com/corn12138/yan-source/core/view"
)

// palette 把主题换算成每个样式在终端里的前景色和背景色。终端画不出半透明，
// 半透明的颜色先叠到编辑区底色上。
type palette struct {
	p      *style.Palette
	fg, bg [style.Count]color.Color
	hasBg  [style.Count]bool
	theme  [style.Count]tcell.Style // 样式单独使用时的完整样式（边框、整行底色）
}

func tc(c style.Color) color.Color {
	r, g, b, _ := c.Parts()
	return color.NewRGBColor(int32(r), int32(g), int32(b))
}

func newPalette(p *style.Palette) *palette {
	t := &palette{p: p}
	for id := style.ID(0); id < style.Count; id++ {
		bg := p.Bg(id)
		base := p.Editor
		if bg != 0 {
			base = bg.Over(p.Editor)
			t.hasBg[id] = true
		}
		t.bg[id] = tc(base)
		t.fg[id] = tc(p.Fg(id).Over(base))
		t.theme[id] = tcell.StyleDefault.Background(t.bg[id]).Foreground(t.fg[id])
	}
	t.theme[style.Border] = t.theme[style.Border].Foreground(tc(p.Border.Over(p.Editor)))
	t.theme[style.StatusAccent] = t.theme[style.StatusAccent].Bold(true)
	t.theme[style.Match] = t.theme[style.Match].Bold(true)
	return t
}

// layout 是一帧的区域划分（VS Code 布局）：左侧资源管理器（含标题行），一列分隔线，
// 右侧若干列，每列上方一行标签栏、下方代码窗格；最底一行是横跨全宽的状态栏。
type layout struct {
	side    view.Rect // 资源管理器（W == 0 表示不显示）
	cols    []colRect
	status  view.Rect
	overlay view.Rect
}

type colRect struct {
	tabs, code view.Rect
}

// tabHit 是标签栏上一个标签占的列区间。
type tabHit struct {
	col, idx int
	x0, x1   int
	y        int
}

// layout places the sidebar and the columns; hidden columns (the editor area is maximized
// without them) get empty rectangles and the others share their width.
func (f *Frontend) layout(w, h int, hidden []bool, sidebar, right bool) layout {
	var l layout
	x0 := 0
	if sidebar && w >= 50 {
		sw := min(f.SidebarWidth, w/2)
		l.side = view.Rect{X: 0, Y: 0, W: view.Unit(sw), H: view.Unit(max(h-1, 1))}
		x0 = sw + 1
	}
	sideW := x0
	if right && sideW > 0 {
		l.side.X = view.Unit(w - sideW + 1)
		x0 = 0
	}
	ncols := 0
	for _, hid := range hidden {
		if !hid {
			ncols++
		}
	}
	ncols = max(ncols, 1)
	avail := w - sideW - (ncols - 1) // 列之间一格分隔线
	i := 0
	for _, hid := range hidden {
		if hid {
			l.cols = append(l.cols, colRect{})
			continue
		}
		cw := avail / ncols
		if i == ncols-1 {
			cw = avail - cw*(ncols-1)
		}
		cx := x0 + i*(avail/ncols+1)
		i++
		l.cols = append(l.cols, colRect{
			tabs: view.Rect{X: view.Unit(cx), Y: 0, W: view.Unit(cw), H: 1},
			code: view.Rect{X: view.Unit(cx), Y: 1, W: view.Unit(max(cw, 1)), H: view.Unit(max(h-2, 1))},
		})
	}
	l.status = view.Rect{X: 0, Y: view.Unit(h - 1), W: view.Unit(w), H: 1}
	return l
}

func overlayRect(w, h int, o *view.Overlay) view.Rect {
	ow := min(w-4, 90)
	ow = max(ow, min(w, 30))
	n := len(overlayRows(o))
	oh := 3 + max(n, 0)
	return view.Rect{X: view.Unit((w - ow) / 2), Y: 1, W: view.Unit(ow), H: view.Unit(min(oh, h-1))}
}

// draw 排版、告诉内核每个代码窗格的大小、要一帧并画出来。tcell 自己会和上一帧比较，只把变化的单元格发给终端。
func (f *Frontend) draw(ed *app.Editor) {
	s := f.screen
	w, h := s.Size()
	fr := &f.frame
	if theme, appearance, _, gen := ed.Settings(); gen != f.settingsGen { // 菜单、命令面板里换了主题
		f.settingsGen = gen
		f.SetTheme(theme, appearance != "light")
	}
	// 先按上一帧的侧边栏和最大化状态排版；内核说变了，就按新状态再排一次。
	want, right := fr.Sidebar.Visible, ed.SidebarRight()
	hidden := make([]bool, ed.Columns())
	for i := range hidden {
		hidden[i] = i < len(fr.Columns) && fr.Columns[i].Hidden
	}
	for range 2 {
		f.lay = f.layout(w, h, hidden, want, right)
		for i, c := range f.lay.cols {
			if c.code.W > 0 { // a hidden column keeps the size it had: nothing in it re-wraps or scrolls
				ed.SetPaneSize(i, view.Size{W: c.code.W, H: c.code.H})
			}
		}
		ed.SetSidebarRows(max(int(f.lay.side.H)-1, 1))
		ed.Frame(fr)
		same := fr.Sidebar.Visible == want && len(fr.Columns) == len(hidden)
		for i := range hidden {
			same = same && i < len(fr.Columns) && fr.Columns[i].Hidden == hidden[i]
		}
		if same {
			break
		}
		want = fr.Sidebar.Visible
		hidden = make([]bool, len(fr.Columns))
		for i := range hidden {
			hidden[i] = fr.Columns[i].Hidden
		}
	}

	if fr.Title != f.title {
		f.title = fr.Title
		s.SetTitle(fr.Title)
	}
	s.Fill(' ', f.pal.theme[style.Text])
	var sideCaret [2]int
	sideOK := false
	if f.lay.side.W > 0 {
		sideCaret, sideOK = f.drawSidebar(&fr.Sidebar, f.lay.side, fr.Focus == view.FocusSidebar)
		for y := 0; y < int(f.lay.side.H); y++ { // 侧边栏右侧的分隔线
			x := int(f.lay.side.X + f.lay.side.W)
			if fr.Sidebar.Right {
				x = int(f.lay.side.X) - 1
			}
			s.Put(x, y, "│", f.pal.theme[style.Border])
		}
	}
	f.tabHits = f.tabHits[:0]
	first := true
	for i := range fr.Columns {
		if i >= len(f.lay.cols) {
			break
		}
		c := &fr.Columns[i]
		r := f.lay.cols[i]
		if c.Hidden || r.code.W == 0 {
			continue
		}
		f.drawTabs(i, c, r.tabs, i == fr.Active)
		f.drawPane(&c.Pane, r.code)
		if !first { // 列之间的分隔线
			for y := 0; y < int(r.code.Y+r.code.H); y++ {
				s.Put(int(r.tabs.X)-1, y, "│", f.pal.theme[style.Border])
			}
		}
		first = false
	}
	f.drawStatus(&fr.Status, f.lay.status)
	var findCaret [2]int
	if fr.Find.Visible && fr.Active < len(f.lay.cols) {
		findCaret = f.drawFind(&fr.Find, f.lay.cols[fr.Active].code)
	}

	caretX, caretY, caret := 0, 0, false
	if fr.Review != nil {
		d := fr.Review
		o := view.Overlay{Title: i18n.F("Review: %s", d.Ref), Input: "", Footer: i18n.T("Ctrl+S Save · Esc Cancel")}
		lines := strings.Split(d.Text, "\n")
		for _, line := range lines {
			o.Items = append(o.Items, view.OverlayItem{Label: line})
		}
		o.Selected = -1
		box := overlayRect(w, h, &o)
		f.drawOverlay(&o, box)
		caretX, caretY, caret = int(box.X)+2, int(box.Y)+2+min(len(lines)-1, view.MaxOverlayItems-1), true
		caretX += min(width.String(lines[len(lines)-1]), int(box.W)-4)
	} else if fr.Overlay.Visible {
		f.lay.overlay = overlayRect(w, h, &fr.Overlay)
		f.drawOverlay(&fr.Overlay, f.lay.overlay)
		// 真实光标放在输入框末尾：输入法的候选框就会出现在这里。
		b := f.lay.overlay
		caretX, caretY, caret = int(b.X)+2+width.String("› "+fr.Overlay.Input), int(b.Y)+1, true
	} else if fr.Focus == view.FocusSidebar && sideOK {
		caretX, caretY, caret = sideCaret[0], sideCaret[1], true
	} else if fr.Focus == view.FocusFind {
		caretX, caretY, caret = findCaret[0], findCaret[1], true
	} else if fr.Focus == view.FocusEditor && fr.Active < len(f.lay.cols) {
		p := &fr.Columns[fr.Active].Pane
		o := f.lay.cols[fr.Active].code
		for _, c := range p.Secondary { // 多光标的其余光标：反色块
			x, y := int(o.X+c.X), int(o.Y+c.Y)
			str, st, _ := s.Get(x, y)
			if str == "" {
				str = " "
			}
			s.Put(x, y, str, st.Reverse(true))
		}
		if p.Caret.Visible {
			caretX, caretY, caret = int(o.X+p.Caret.X), int(o.Y+p.Caret.Y), true
		}
		if p.Caret.Block != f.blockCaret { // vim 普通模式：块光标
			f.blockCaret = p.Caret.Block
			if p.Caret.Block {
				s.SetCursorStyle(tcell.CursorStyleSteadyBlock)
			} else {
				s.SetCursorStyle(tcell.CursorStyleBlinkingBar)
			}
		}
	}
	if caret {
		s.ShowCursor(caretX, caretY) // 真实光标：输入法候选框跟着它走
	} else {
		s.HideCursor()
	}
	s.Show()
}

// drawRow 在 (x0, y) 起、宽 w 的区域里画一行，超出部分截掉。
func (f *Frontend) drawRow(x0, y, w int, txt []byte, base style.ID, spans []style.Span) {
	f.screen.FillArea(x0, y, w, 1, ' ', f.pal.theme[base])
	forCells(txt, f.tabWidth, func(start, end, x, cw int) {
		if x+cw > w {
			return
		}
		f.screen.Put(x0+x, y, string(txt[start:end]), f.styleAt(base, spans, start))
	})
}

func (f *Frontend) drawText(x0, y, w int, s string, st style.ID) {
	f.drawRow(x0, y, w, []byte(s), st, nil)
}

// sideLine 记下侧边栏的一行是什么：点它时前端据此调用对应的语义动作。
type sideLine struct {
	repositoryID uint64

	kind  uint8 // sideTree、sideSearchRow、sideSCMRow
	index int
}

const (
	sideNone uint8 = iota
	sideTree
	sideSearchRow
	sideSCMRow
	sideRepository
	sideRepositoryMessage
	sideLesson
)

// drawSidebar 画侧边栏的当前视图，返回键盘所在输入框的光标位置（没有时 ok 为假）。
func (f *Frontend) drawSidebar(sb *view.Sidebar, r view.Rect, focused bool) (caret [2]int, ok bool) {
	x0, w := int(r.X), int(r.W)
	f.sideLines = f.sideLines[:0]
	for range int(r.H) {
		f.sideLines = append(f.sideLines, sideLine{})
	}
	switch sb.View {
	case view.SideLearn:
		f.drawLearn(&sb.Learn, r, focused)
		return [2]int{}, false
	case view.SideSearch:
		return f.drawSearch(&sb.Search, r, focused)
	case view.SideSCM:
		return f.drawSCM(&sb.SCM, sb.Changes, r, focused)
	}
	header := i18n.F(" Explorer: %s", strings.ToUpper(sb.Title))
	if sb.Busy {
		header += " …"
	}
	f.drawText(x0, 0, w, header, style.Tab)
	if len(sb.Rows) == 0 && !sb.Busy {
		f.drawText(x0+1, 1, w-2, i18n.T("This folder is empty: \"Explorer: New File…\" in the Command Palette"), style.Dim)
		return caret, false
	}
	rows := int(r.H) - 1
	for i := sb.Top; i < len(sb.Rows) && i < sb.Top+rows; i++ {
		n := &sb.Rows[i]
		var b strings.Builder
		b.WriteString(strings.Repeat("  ", n.Depth+1))
		switch {
		case n.Dir && n.Expanded:
			b.WriteString("▾ ")
		case n.Dir:
			b.WriteString("▸ ")
		default:
			b.WriteString("  ")
		}
		b.WriteString(n.Name)
		st := style.Text
		if n.Ignored {
			st = style.Dim
		}
		if i == sb.Selected {
			st = style.TreeCursor
			if focused {
				st = style.TreeFocused
			}
		}
		y := 1 + i - sb.Top
		f.sideLines[y] = sideLine{kind: sideTree, index: i}
		f.drawText(x0, y, w, b.String(), st)
		if n.Git != 0 || n.Modified { // git 的字母、未保存的圆点，靠右
			c, mark := f.pal.p.Modified, string(n.Git)
			switch n.Git {
			case 'A', 'U':
				c = f.pal.p.Added
			case 'D':
				c = f.pal.p.Removed
			case 'C':
				c = f.pal.p.Danger
			}
			if n.Dir {
				mark = "•"
			}
			if n.Modified {
				c, mark = f.pal.p.Modified, "●"
			}
			f.screen.Put(x0+w-2, y, mark, f.pal.theme[st].Foreground(tc(c.Over(f.pal.p.Editor))))
		}
	}
	return caret, false
}

// gitTC 是 git 字母的终端颜色。
func (f *Frontend) gitTC(letter byte) color.Color {
	c := f.pal.p.Modified
	switch letter {
	case 'A', 'U':
		c = f.pal.p.Added
	case 'D':
		c = f.pal.p.Removed
	case 'C':
		c = f.pal.p.Danger
	}
	return tc(c.Over(f.pal.p.Editor))
}

// field 画一个输入框：标签、文字（放不下时只显示末尾）；有焦点时返回光标位置。
func (f *Frontend) field(x0, y, w int, label, value, placeholder string, focused bool) (int, bool) {
	f.drawText(x0, y, w, label, style.Dim)
	lw := width.String(label)
	fw := w - lw
	v := value
	for width.String(v) > fw-1 && v != "" {
		_, n := utf8.DecodeRuneInString(v)
		v = v[n:]
	}
	st := style.Overlay
	if focused {
		st = style.OverlaySelected
	}
	f.screen.FillArea(x0+lw, y, fw, 1, ' ', f.pal.theme[st])
	if v == "" && !focused {
		f.drawText(x0+lw, y, fw, placeholder, style.Dim)
	} else {
		f.drawText(x0+lw, y, fw, v, st)
	}
	return x0 + lw + width.String(v), focused
}

func (f *Frontend) drawSearch(s *view.Search, r view.Rect, focused bool) (caret [2]int, ok bool) {
	x0, w := int(r.X), int(r.W)
	opts := ""
	for _, o := range []struct {
		on    bool
		label string
	}{{s.MatchCase, "Aa"}, {s.Word, "W"}, {s.Regex, ".*"}} {
		if o.on {
			opts += "[" + o.label + "]"
		} else {
			opts += " " + o.label + " "
		}
	}
	f.drawText(x0, 0, w, i18n.T(" Search"), style.Tab)
	f.drawText(x0+w-width.String(opts)-1, 0, width.String(opts), opts, style.Tab)
	y := 1
	put := func(label, value, ph string, fieldID view.SearchField) {
		x, on := f.field(x0+1, y, w-2, label, value, ph, focused && s.Focus == fieldID)
		if on {
			caret, ok = [2]int{x, y}, true
		}
		y++
	}
	put(i18n.T("Find    "), s.Query, "", view.SearchQuery)
	if s.Replacing {
		put(i18n.T("Replace "), s.With, "", view.SearchReplace)
	}
	if s.Details {
		put(i18n.T("Include "), s.Include, "*.go, src/**", view.SearchInclude)
		put(i18n.T("Exclude "), s.Exclude, "testdata", view.SearchExclude)
	}
	sum := s.Summary
	if s.Running && s.Query != "" {
		sum = i18n.F("Searching… %s", sum)
	}
	f.drawText(x0+1, y, w-2, sum, style.Dim)
	y++
	for i := s.Top; i < len(s.Rows) && y < int(r.H); i++ {
		row := &s.Rows[i]
		st := style.Text
		if i == s.Selected && focused && s.Focus == view.SearchResults {
			st = style.TreeFocused
		}
		f.sideLines[y] = sideLine{kind: sideSearchRow, index: i}
		if !row.Match {
			mark := "▾ "
			if row.Collapsed {
				mark = "▸ "
			}
			f.drawText(x0, y, w, " "+mark+row.Name+"  ", st)
			nameEnd := x0 + 1 + width.String(mark+row.Name) + 2
			if row.Dir != "" && nameEnd < x0+w-6 {
				f.drawText(nameEnd, y, x0+w-6-nameEnd, row.Dir, style.Dim)
			}
			n := strconv.Itoa(row.Count)
			f.drawText(x0+w-len(n)-1, y, len(n), n, style.Dim)
		} else {
			pre := strings.TrimLeft(row.Pre, " \t")
			txt := []byte("    " + pre + row.Hit + row.Post)
			a := 4 + len(pre)
			spans := []style.Span{{Start: int32(a), End: int32(a + len(row.Hit)), Style: style.SearchCurrent}}
			if row.Replacing {
				txt = []byte("    " + pre + row.Hit + row.With + row.Post)
				spans = []style.Span{{Start: int32(a), End: int32(a + len(row.Hit)), Style: style.Deleted},
					{Start: int32(a + len(row.Hit)), End: int32(a + len(row.Hit) + len(row.With)), Style: style.Inserted}}
			}
			f.drawRow(x0, y, w, txt, st, spans)
		}
		y++
	}
	return caret, ok
}

func (f *Frontend) drawSCM(s *view.SCM, changes int, r view.Rect, focused bool) (caret [2]int, ok bool) {
	if len(s.Panels) > 1 {
		return f.drawRepositories(s, changes, r, focused)
	}
	x0, w := int(r.X), int(r.W)
	f.drawText(x0, 0, w, i18n.F(" Source Control (%d)", changes), style.Tab)
	if s.NoRepo {
		if s.RepositoryLoading || s.RepositoryError != "" || len(s.Repositories) > 0 {
			f.drawText(x0+1, 1, w-2, i18n.T("Repositories in folder · Enter to open"), style.Dim)
			message := i18n.T("Command Palette: Git: Refresh Repository List")
			if s.RepositoryLoading {
				message = i18n.T("Looking for repositories…")
			}
			if s.RepositoryError != "" {
				message = s.RepositoryError
			}
			f.drawText(x0+1, 2, w-2, message, style.Dim)
			count := max(0, int(r.H)-3)
			top := max(0, s.Selected-count+1)
			for i := top; i < len(s.Repositories) && i-top < count; i++ {
				y := 3 + i - top
				st := style.Text
				if focused && i == s.Selected {
					st = style.TreeFocused
				}
				f.sideLines[y] = sideLine{kind: sideSCMRow, index: i}
				f.drawText(x0+1, y, w-2, s.Repositories[i].Name, st)
			}
			return caret, false
		}
		f.drawText(x0+1, 2, w-2, i18n.T("This folder is not in a Git repository"), style.Dim)
		f.drawText(x0+1, 3, w-2, i18n.T("Command Palette \"Git: Initialize Repository\""), style.Dim)
		return caret, false
	}
	branch := "⎇ " + s.Branch
	if s.Ahead > 0 {
		branch += fmt.Sprintf(" ↑%d", s.Ahead)
	}
	if s.Behind > 0 {
		branch += fmt.Sprintf(" ↓%d", s.Behind)
	}
	if s.Busy != "" {
		branch += "  " + s.Busy
	}
	f.drawText(x0+1, 1, w-2, branch, style.Text)
	y := 2
	if s.Op != "" { // 命令面板里有「继续」「中止」
		f.drawText(x0+1, y, w-2, i18n.F("%s hit conflicts: resolve and stage them, then \"Git: Continue…\" or \"Git: Abort…\" in the Command Palette", s.Op), style.StatusAccent)
		y++
	}
	label := i18n.T("Message ")
	if s.Amend {
		label = i18n.T("Amend   ")
	}
	lines := strings.Split(s.Message, "\n")
	for i, l := range lines {
		if y >= int(r.H)-1 || i >= 4 {
			break
		}
		x, on := f.field(x0+1, y, w-2, label, l, i18n.T("Write a commit message (Tab to enter)"), focused && s.MessageFocused && i == len(lines)-1)
		if on {
			caret, ok = [2]int{x, y}, true
		}
		label = "     "
		y++
	}
	f.drawText(x0+1, y, w-2, i18n.F("⌃K C %s · Space Stage · Del Discard", s.CommitLabel), style.Dim)
	y++
	i := 0
	for _, g := range s.Groups {
		if y >= int(r.H) {
			break
		}
		mark := "▾"
		if g.Folded {
			mark = "▸"
		}
		f.drawText(x0, y, w, fmt.Sprintf(" %s %s (%d)", mark, g.Title, g.Count), style.Dim)
		y++
		for _, row := range g.Rows {
			if y >= int(r.H) {
				break
			}
			st := style.Text
			if i == s.Selected && focused && !s.MessageFocused {
				st = style.TreeFocused
			}
			f.sideLines[y] = sideLine{kind: sideSCMRow, index: i}
			indent := strings.Repeat("  ", row.Depth)
			if row.Folder { // 以树的形式查看：文件夹
				mark := "▾ "
				if row.Collapsed {
					mark = "▸ "
				}
				f.drawText(x0, y, w, "   "+indent+mark+row.Name+"/", st)
				y++
				i++
				continue
			}
			f.drawText(x0, y, w, "   "+indent+row.Name+"  ", st)
			end := x0 + 3 + width.String(indent+row.Name) + 2
			if row.Dir != "" && end < x0+w-3 {
				f.drawText(end, y, x0+w-3-end, row.Dir, style.Dim)
			}
			f.screen.Put(x0+w-2, y, string(row.Letter), f.pal.theme[st].Foreground(f.gitTC(row.Letter)))
			y++
			i++
		}
	}
	if y < int(r.H) && s.CommitsFolded {
		f.drawText(x0, y, w, i18n.T(" ▸ Commits"), style.Dim)
		y++
	} else if y < int(r.H) && (len(s.Commits) > 0 || s.HistoryLoading) {
		count := 0
		for _, cm := range s.Commits {
			if cm.File == nil {
				count++
			}
		}
		title := i18n.F(" ▾ Commits (%d)", count)
		if s.AllBranches {
			title += i18n.T(" · all branches")
		}
		if s.HistoryLoading {
			title += i18n.T(" · reading…")
		}
		f.drawText(x0, y, w, title, style.Dim)
		y++
	}
	lanes := min(max(s.Lanes, 1), tuiMaxLanes)
	for k := range s.Commits {
		if y >= int(r.H) {
			break
		}
		cm := &s.Commits[k]
		st := style.Text
		if i == s.Selected && focused && !s.MessageFocused {
			st = style.TreeFocused
		}
		f.sideLines[y] = sideLine{kind: sideSCMRow, index: i}
		f.drawText(x0, y, w, "", st)
		x := x0 + 1
		for l := range lanes {
			ch, c := graphCell(cm, l)
			if ch != ' ' {
				f.screen.Put(x, y, string(ch), f.pal.theme[st].Foreground(tc(f.pal.p.Lane(c).Over(f.pal.p.Editor))))
			}
			x += 2
		}
		label := cm.Subject
		if cm.File != nil {
			r := cm.File
			prefix := "  "
			if r.Folder {
				prefix = "▾ "
				if r.Collapsed {
					prefix = "▸ "
				}
			}
			label = strings.Repeat("  ", r.Depth+1) + prefix + r.Name
		} else if cm.Expanded {
			label = "▾ " + label
		} else {
			label = "▸ " + label
		}
		for _, ref := range cm.Refs {
			label = "(" + ref.Name + ") " + label
			break
		}
		when := " " + cm.When
		if cm.Unpushed {
			when = " ↑" + when
		}
		end := x0 + w - width.String(when)
		f.drawText(x, y, max(end-x, 0), label, st)
		if end > x {
			f.drawText(end, y, x0+w-end, when, style.Dim)
		}
		y++
		i++
	}
	if s.MoreCommits && y < int(r.H) {
		f.drawText(x0+3, y, w-3, i18n.T("… Show more commits: Command Palette \"Git: Show More Commits\""), style.Dim)
	}
	return caret, ok
}

// tuiMaxLanes 是终端里提交图最多画几道（每道两格）。
const tuiMaxLanes = 4

// graphCell 是提交图第 l 道在这一行画的字和颜色：点，直通的竖线，并进点、从点分出去的拐角。
// 终端里只画得出近似的样子；道多于 tuiMaxLanes 时右边的不画。
func graphCell(cm *view.SCMCommit, l int) (rune, int) {
	if cm.File != nil {
		return ' ', 0
	}
	above := l < len(cm.Above) && cm.Above[l] >= 0
	below := l < len(cm.Below) && cm.Below[l] >= 0
	switch {
	case l == cm.Lane && cm.Head:
		return '◉', cm.Color
	case l == cm.Lane && cm.Merge:
		return '○', cm.Color
	case l == cm.Lane:
		return '●', cm.Color
	case above && !cm.Joins[l] || below && cm.Through[l]:
		if above {
			return '│', cm.Above[l]
		}
		return '│', cm.Below[l]
	case above && cm.Joins[l]:
		if l > cm.Lane {
			return '╯', cm.Above[l]
		}
		return '╰', cm.Above[l]
	case below && cm.From[l]:
		if l > cm.Lane {
			return '╮', cm.Below[l]
		}
		return '╭', cm.Below[l]
	case below:
		return '│', cm.Below[l]
	}
	return ' ', 0
}

// drawTabs 画一列的标签栏：当前标签高亮，有修改的文件名后跟 ●，预览标签加 ~；
// 放不下时从左边裁掉，保证当前标签可见。
func (f *Frontend) drawTabs(col int, c *view.Column, r view.Rect, focused bool) {
	type seg struct {
		text string
		w    int
	}
	segs := make([]seg, len(c.Tabs))
	for i, t := range c.Tabs {
		s := " " + t.Name
		if t.Preview {
			s = " ~" + t.Name
		}
		switch {
		case t.Dirty:
			s += " ●"
		case t.ReadOnly:
			s += i18n.T(" [Read-only]")
		}
		s += " "
		segs[i] = seg{s, width.String(s)}
	}
	first := 0
	for {
		total := 0
		for i := first; i <= c.Active && i < len(segs); i++ {
			total += segs[i].w + 1
		}
		if total <= int(r.W) || first >= c.Active {
			break
		}
		first++
	}
	x0, y, w := int(r.X), int(r.Y), int(r.W)
	f.screen.FillArea(x0, y, w, 1, ' ', f.pal.theme[style.Tab])
	x := 0
	for i := first; i < len(segs) && x < w; i++ {
		st := style.Tab
		if i == c.Active {
			st = style.TabActive
			if !focused {
				st = style.Text
			}
		}
		f.drawText(x0+x, y, min(segs[i].w, w-x), segs[i].text, st)
		f.tabHits = append(f.tabHits, tabHit{col: col, idx: i, x0: x0 + x, x1: x0 + x + segs[i].w, y: y})
		x += segs[i].w
		if x < w {
			f.screen.Put(x0+x, y, "│", f.pal.theme[style.Border])
		}
		x++
	}
}

func (f *Frontend) drawStatus(st *view.Status, r view.Rect) {
	left := " " + st.Left
	base := style.Status
	if st.Message != "" {
		left = " " + st.Message
		if a := st.Action; a.Command != "" { // 终端里没有按钮：说去命令面板
			left += i18n.F(" (%s: Command Palette)", a.Label)
		}
	}
	var right strings.Builder
	if st.Busy != "" {
		right.WriteString(st.Busy + "   ")
	}
	if st.Branch != "" {
		right.WriteString("⎇ " + st.Branch + "   ")
	}
	for _, it := range st.Right {
		right.WriteString(it.Text)
		right.WriteString("   ")
	}
	rs := strings.TrimRight(right.String(), " ") + " "
	pad := max(int(r.W)-width.String(left)-width.String(rs), 1)
	txt := left + strings.Repeat(" ", pad) + rs
	var spans []style.Span
	if st.Message != "" {
		spans = []style.Span{{Start: 0, End: int32(len(left)), Style: style.StatusAccent}}
	}
	f.drawRow(int(r.X), int(r.Y), int(r.W), []byte(txt), base, spans)
}

// drawOverlay 画带圆角边框的浮层：标题嵌在上边框，输入行，候选列表，提示嵌在下边框。
func (f *Frontend) drawOverlay(o *view.Overlay, b view.Rect) {
	s := f.screen
	x0, y0, w, h := int(b.X), int(b.Y), int(b.W), int(b.H)
	bs := f.pal.theme[style.Border].Background(f.pal.theme[style.Overlay].GetBackground())
	s.FillArea(x0, y0, w, h, ' ', f.pal.theme[style.Overlay])
	for x := x0 + 1; x < x0+w-1; x++ {
		s.Put(x, y0, "─", bs)
		s.Put(x, y0+h-1, "─", bs)
	}
	for y := y0 + 1; y < y0+h-1; y++ {
		s.Put(x0, y, "│", bs)
		s.Put(x0+w-1, y, "│", bs)
	}
	s.Put(x0, y0, "╭", bs)
	s.Put(x0+w-1, y0, "╮", bs)
	s.Put(x0, y0+h-1, "╰", bs)
	s.Put(x0+w-1, y0+h-1, "╯", bs)
	title := " " + o.Title + " "
	if o.Section != "" {
		title = " " + o.Title + " › " + o.Section + " "
	}
	f.drawText(x0+2, y0, min(width.String(title), w-4), title, style.Overlay)
	f.drawText(x0+2, y0+1, w-4, "› "+o.Input, style.Overlay)
	inner := w - 4
	for r, row := range overlayRows(o) {
		y := y0 + 2 + r
		if y >= y0+h-1 {
			break
		}
		if row.item < 0 {
			f.drawText(x0+2, y, inner, row.text, row.style)
			continue
		}
		i := row.item
		base := style.Overlay
		if i == o.Selected {
			base = style.OverlaySelected
			s.FillArea(x0+1, y, w-2, 1, ' ', f.pal.theme[style.OverlaySelected])
		}
		it := o.Items[i]
		if o.Kind == view.OverlaySettings {
			mark := "  "
			if it.Checked {
				mark = "✓ "
			}
			it.Label = mark + it.Label
		}
		txt, spans := itemRow(&it, inner)
		f.drawRow(x0+2, y, inner, txt, base, spans)
	}
	if o.Footer != "" {
		ft := " " + o.Footer + " "
		fw := width.String(ft)
		if fw < w-4 {
			f.drawText(x0+w-2-fw, y0+h-1, fw, ft, style.Dim)
		}
	}
}

// overlayRow is a row of an overlay's list: item is the index of the item it shows, -1 for a
// line of text (a heading or what Settings says is in effect).
type overlayRow struct {
	item  int
	text  string
	style style.ID
}

// overlayRows are the rows under an overlay's input line: the screen of items from Top, and in
// Settings what is in effect, the restart notice and a heading over each group of items.
func overlayRows(o *view.Overlay) []overlayRow {
	var rows []overlayRow
	if o.Kind == view.OverlaySettings {
		rows = append(rows, overlayRow{-1, o.Status, style.Dim})
		if o.Notice != "" {
			rows = append(rows, overlayRow{-1, o.Notice, style.Overlay})
		}
	}
	for i := o.Top; i < len(o.Items) && i < o.Top+view.MaxOverlayItems; i++ {
		if g := o.Items[i].Group; g != "" && (i == o.Top || o.Items[i-1].Group != g) {
			rows = append(rows, overlayRow{-1, g, style.Dim})
		}
		rows = append(rows, overlayRow{item: i})
	}
	return rows
}

// itemRow 把候选项排成一行：名字、灰色的目录或命令 ID、右对齐的快捷键。
func itemRow(it *view.OverlayItem, inner int) ([]byte, []style.Span) {
	txt := []byte(it.Label)
	var spans []style.Span
	for _, h := range it.LabelHits {
		spans = append(spans, style.Span{Start: int32(h), End: int32(h + 1), Style: style.Match})
	}
	if it.Detail != "" {
		txt = append(txt, "  "...)
		off := len(txt)
		txt = append(txt, it.Detail...)
		spans = append(spans, style.Span{Start: int32(off), End: int32(len(txt)), Style: style.Dim})
		for _, h := range it.DetailHits {
			spans = append(spans, style.Span{Start: int32(off + h), End: int32(off + h + 1), Style: style.Match})
		}
	}
	if it.Hint != "" {
		pad := inner - width.String(string(txt)) - width.String(it.Hint)
		if pad >= 2 {
			txt = append(txt, strings.Repeat(" ", pad)...)
			off := len(txt)
			txt = append(txt, it.Hint...)
			spans = append(spans, style.Span{Start: int32(off), End: int32(len(txt)), Style: style.Dim})
		}
	}
	return txt, spans
}

// styleAt 叠出第 b 个字节的样式：前景取最上层管前景的样式，背景取最上层管背景的样式
// （半透明的背景叠在下面的背景上）——选中的关键字仍是关键字的颜色。
func (f *Frontend) styleAt(base style.ID, spans []style.Span, b int) tcell.Style {
	pl := f.pal
	fg := pl.fg[base]
	bg := pl.p.Bg(base)
	if bg == 0 {
		bg = pl.p.Editor
	} else {
		bg = bg.Over(pl.p.Editor)
	}
	bold := false
	for _, sp := range spans {
		if int(sp.Start) > b || b >= int(sp.End) {
			continue
		}
		l := style.LayerOf(sp.Style)
		if l&style.LayerBg != 0 && pl.hasBg[sp.Style] {
			bg = pl.p.Bg(sp.Style).Over(bg)
		}
		if l&style.LayerFg != 0 {
			fg = pl.fg[sp.Style]
			bold = sp.Style == style.Match
		}
	}
	return tcell.StyleDefault.Background(tc(bg)).Foreground(fg).Bold(bold)
}

func (f *Frontend) drawPane(p *view.PaneFrame, r view.Rect) {
	switch p.Kind {
	case view.PaneEmpty:
		f.drawWelcome(p, r)
	case view.PaneMessage, view.PaneImage, view.PaneMarkdown, view.PaneTerminal:
		f.drawText(int(r.X)+2, int(r.Y)+1, int(r.W)-2, p.Message, style.Dim)
	default:
		f.drawCode(p, r)
	}
}

func (f *Frontend) drawWelcome(p *view.PaneFrame, r view.Rect) {
	lines := []string{i18n.T("Yan"), ""}
	for _, h := range p.Hints {
		k := h.Keys
		if pad := 12 - width.String(k); pad > 0 {
			k += strings.Repeat(" ", pad)
		}
		lines = append(lines, k+h.Title)
	}
	top := int(r.Y) + max(int(r.H)/2-len(lines)/2, 0)
	for i, l := range lines {
		pad := max((int(r.W)-30)/2, 0)
		if top+i < int(r.Y+r.H) {
			f.drawText(int(r.X)+pad, top+i, int(r.W)-pad, l, style.Dim)
		}
	}
}

func (f *Frontend) drawCode(p *view.PaneFrame, r view.Rect) {
	defer func() { // 钉在顶上的行：画在最上面几行上，用当前行的底色和下面的正文分开
		if len(p.Sticky) == 0 {
			return
		}
		sp := *p
		sp.Rows, sp.Sticky, sp.Current, sp.Peek = make([]view.RowFrame, len(p.Sticky)), nil, -1, nil
		for i, row := range p.Sticky {
			row.Base, row.Mark, row.Ghost = style.CurrentLine, view.MarkNone, ""
			sp.Rows[i] = row
		}
		f.drawCode(&sp, view.Rect{X: r.X, Y: r.Y, W: r.W, H: view.Unit(len(p.Sticky))})
	}()
	s := f.screen
	x0, y0, w := int(r.X), int(r.Y), int(r.W)
	gutter := int(p.Gutter)
	textW := w - gutter
	defer f.drawPeek(p, r) // 改动的框盖在代码上面
	for i, row := range p.Rows {
		if i >= int(r.H) {
			break
		}
		y := y0 + i
		base := f.pal.theme[row.Base]
		// 行号栏
		gs := f.pal.theme[style.Gutter]
		if row.Line == p.Current {
			gs = f.pal.theme[style.GutterCurrent]
		}
		s.FillArea(x0, y, gutter, 1, ' ', gs)
		num := strconv.Itoa(int(row.Line) + 1)
		switch {
		case row.Num < 0:
			num = ""
		case row.Num > 0:
			num = strconv.Itoa(int(row.Num))
		}
		if gutter > len(num)+1 {
			s.PutStrStyled(x0+gutter-2-len(num), y, num, gs)
		}
		if m := row.Mark; m != view.MarkNone && gutter >= 2 { // git 改动：行号右边一道竖条
			c, ch := f.pal.p.Changed, "▎"
			switch m {
			case view.MarkAdded:
				c = f.pal.p.Added
			case view.MarkDeleted:
				c, ch = f.pal.p.Removed, "▔"
			case view.MarkDeletedBelow:
				c, ch = f.pal.p.Removed, "▁"
			}
			s.Put(x0+gutter-1, y, ch, gs.Foreground(tc(c)))
		}
		if row.ReviewID != 0 && gutter >= 2 {
			s.Put(x0, y, "●", gs.Foreground(tc(f.pal.p.Accent)))
		}
		// 正文
		s.FillArea(x0+gutter, y, textW, 1, ' ', base)
		scroll := int(p.ScrollX)
		endX := 0
		forCells(row.Text, f.tabWidth, func(start, end, cx, cw int) {
			endX = cx + cw
			vx := cx - scroll
			if vx+cw <= 0 || vx >= textW {
				return
			}
			st := f.styleAt(row.Base, row.Spans, start)
			g := string(row.Text[start:end])
			switch {
			case row.Text[start] == '\t' || vx < 0 || vx+cw > textW: // 制表符、被左右边缘截断的宽字符：画空格
				for k := max(vx, 0); k < min(vx+cw, textW); k++ {
					s.Put(x0+gutter+k, y, " ", st)
				}
			case cw == 1 && (row.Text[start] < 0x20 || row.Text[start] == 0x7f):
				s.Put(x0+gutter+vx, y, "␣", st.Foreground(color.NewHexColor(0xf14c4c)))
			default:
				s.Put(x0+gutter+vx, y, g, st)
			}
		})
		// 行尾的 blame：暗淡的灰字
		if row.Ghost != "" {
			if vx := endX + 3 - scroll; vx >= 0 && vx < textW {
				f.drawText(x0+gutter+vx, y, textW-vx, row.Ghost, style.Dim)
			}
		}
		// 缩进参考线：落在空白格上的画成暗淡的 │
		for _, g := range row.Guides {
			vx := int(g.X) - scroll
			if vx < 0 || vx >= textW {
				continue
			}
			x := x0 + gutter + vx
			if str, st, _ := s.Get(x, y); str == " " || str == "" {
				c := f.pal.p.Guide
				if g.Active {
					c = f.pal.p.GuideNow
				}
				s.Put(x, y, "│", st.Foreground(tc(c.Over(f.pal.p.Editor))))
			}
		}
		// 选区跨过行尾时，多画一格表示换行符被选中
		for _, sp := range row.Spans {
			if int(sp.End) > len(row.Text) && sp.Style == style.Selection {
				if vx := endX - scroll; vx >= 0 && vx < textW {
					s.Put(x0+gutter+vx, y, " ", f.pal.theme[style.Selection])
				}
			}
		}
	}
}

// drawFind 在代码窗格右上角画查找框：一行查找、（替换时）一行替换，返回有焦点的输入框的光标位置。
func (f *Frontend) drawFind(fd *view.Find, code view.Rect) [2]int {
	w := min(int(code.W)-2, 56)
	if w < 20 {
		return [2]int{}
	}
	x0, y := int(code.X+code.W)-w-1, int(code.Y)
	opts := ""
	for _, o := range []struct {
		on    bool
		label string
	}{{fd.MatchCase, "Aa"}, {fd.Word, "W"}, {fd.Regex, ".*"}} {
		if o.on {
			opts += "[" + o.label + "]"
		} else {
			opts += " " + o.label + " "
		}
	}
	var caret [2]int
	row := func(y int, label, value string, focused bool, right string) {
		f.screen.FillArea(x0, y, w, 1, ' ', f.pal.theme[style.Overlay])
		f.drawText(x0+1, y, w-2, label, style.Dim)
		lw := width.String(label) + 2
		fieldW := w - lw - width.String(right) - 2
		v := value
		for width.String(v) > fieldW-1 && v != "" { // 放不下：只显示末尾
			_, n := utf8.DecodeRuneInString(v)
			v = v[n:]
		}
		st := style.Overlay
		if focused {
			st = style.OverlaySelected
		}
		f.screen.FillArea(x0+lw, y, fieldW, 1, ' ', f.pal.theme[st])
		f.drawText(x0+lw, y, fieldW, v, st)
		if focused {
			caret = [2]int{x0 + lw + width.String(v), y}
		}
		if right != "" {
			f.drawText(x0+w-1-width.String(right), y, width.String(right), right, style.Dim)
		}
	}
	right := fd.Count + " " + opts
	row(y, i18n.T("Find"), fd.Query, fd.Focus == view.FindQuery, right)
	if fd.Replacing {
		row(y+1, i18n.T("Replace"), fd.With, fd.Focus == view.FindReplace, "")
	}
	return caret
}

// drawPeek 画行号旁一处改动下面的框：标题和键，HEAD 里的那几行（照着 Mayoi 的 peek）。终端里没有按钮，用键。
func (f *Frontend) drawPeek(p *view.PaneFrame, r view.Rect) {
	pk := p.Peek
	if pk == nil || pk.After < 0 {
		return
	}
	x := int(r.X) + int(p.Gutter)
	w := int(r.W) - int(p.Gutter)
	y := int(r.Y) + pk.After + 1
	bottom := int(r.Y + r.H)
	if y >= bottom || w < 10 {
		return
	}
	f.drawText(x, y, w, i18n.F("┤ %s · ⌃K R Revert · ⌃K S Stage · ⌥F3 Next · Esc Close", pk.Title), style.Overlay)
	y++
	lines := pk.Removed
	if len(lines) == 0 {
		lines = []string{i18n.T("(These lines are not in HEAD: they are new)")}
	}
	for i, l := range lines {
		if y >= bottom || i == 10 {
			break
		}
		f.drawText(x, y, w, " "+l, style.Deleted)
		y++
	}
}

// drawLearn draws the feature guide: the steps of the lesson running with their keys, the
// lessons (Enter or a click starts one) and a tip.
func (f *Frontend) drawLearn(lg *view.Learn, r view.Rect, focused bool) {
	x0, w, h := int(r.X), int(r.W), int(r.H)
	y := 0
	line := func(s string, st style.ID) {
		if y < h {
			f.drawText(x0, y, w, s, st)
		}
		y++
	}
	line(i18n.F(" Learn: Feature Tour  %d/%d", lg.Done, len(lg.Lessons)), style.Tab)
	if lg.Active >= 0 && lg.Active < len(lg.Lessons) {
		line(" "+lg.Lessons[lg.Active].Title, style.Text)
		for _, s := range lg.Steps {
			mark, st := " ○ ", style.Text
			switch {
			case s.Done:
				mark, st = " ✓ ", style.Dim
			case s.WindowOnly:
				mark, st = " - ", style.Dim
			case s.Current:
				mark = " ▸ "
			}
			text := mark + s.Text
			if s.Current && s.Keys != "" {
				text += "  " + s.Keys
			}
			if s.Current && s.Times > 1 {
				text += fmt.Sprintf("  %d/%d", s.Count, s.Times)
			}
			line(text, st)
		}
		line("", style.Text)
	}
	module := ""
	for i, l := range lg.Lessons {
		if l.Module != module {
			module = l.Module
			line(" "+module, style.Dim)
		}
		mark := " ○ "
		if l.Done {
			mark = " ✓ "
		}
		st := style.Text
		if focused && i == lg.Selected {
			st = style.TreeFocused
		}
		if y < h {
			f.sideLines[y] = sideLine{kind: sideLesson, index: i}
		}
		line(mark+l.Title, st)
	}
	line("", style.Text)
	line(i18n.F(" Tip: %s", lg.Tip), style.Dim)
}
