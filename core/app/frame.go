package app

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/syntax/highlight"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/text/width"
	"github.com/corn12138/yan-source/core/view"
	"github.com/corn12138/yan-source/core/workspace"
)

func (e *Editor) paneHeight() int { return max(int(e.column().size.H), 1) }
func (e *Editor) pageLines() int  { return e.paneHeight() }

func (e *Editor) gutter(s text.Snapshot) view.Unit {
	if !e.cfg.LineNumbers {
		return 1
	}
	return view.Unit(max(len(strconv.Itoa(s.LineCount())), 3) + 3)
}

// reveal 滚动当前标签，让主光标可见。
func (e *Editor) reveal() {
	c := e.column()
	t := c.tab()
	if t == nil || c.size.H <= 0 {
		return
	}
	s := t.doc.buf.Snapshot()
	w := e.wrapWidth(c, t, s)
	at, x := e.rowAt(s, w, e.sels(t).Main().Head)
	h := int(c.size.H)
	if top := e.topRow(t, s, w); at.before(top) {
		t.setTop(at)
	} else if e.rowsBetween(s, w, top, at, h) >= h {
		v, _ := e.stepRows(s, w, at, -(h - 1))
		t.setTop(v)
	}
	if w > 0 {
		t.scrollX = 0 // 折行时没有水平滚动
		return
	}
	textW := c.size.W - e.gutter(s)
	margin := min(view.Unit(4), textW/4)
	if x < t.scrollX {
		t.scrollX = max(x-margin, 0)
	} else if x >= t.scrollX+textW-1 {
		t.scrollX = x - textW + 1 + margin
	}
}

// centerLine 让第 line 行可见：已经在视野里就不动，否则把它放到窗格中间。
func (e *Editor) centerLine(t *tab, line int) {
	c := e.colOf(t)
	s := t.doc.buf.Snapshot()
	w := e.wrapWidth(c, t, s)
	h := max(int(c.size.H), 1)
	at, top := vrow{line, 0}, e.topRow(t, s, w)
	if !at.before(top) && e.rowsBetween(s, w, top, at, h) < h {
		return
	}
	v, _ := e.stepRows(s, w, at, -(h / 2))
	t.setTop(v)
}

func (e *Editor) scroll(in ScrollInput) {
	if in.Sidebar {
		if e.ws != nil && e.sidebar {
			e.ws.top = max(e.ws.top+int(in.DY), 0)
		}
		return
	}
	if in.Column < 0 || in.Column >= len(e.cols) {
		return
	}
	if t := e.cols[in.Column].tab(); t != nil {
		e.scrollTab(t, int(in.DY))
		if !e.wraps(t) {
			t.scrollX = max(t.scrollX+in.DX, 0)
		}
	}
}

// scrollTab 把标签滚 n 个视觉行（最多滚到最后一行在顶上）。
func (e *Editor) scrollTab(t *tab, n int) {
	s := t.doc.buf.Snapshot()
	w := e.wrapWidth(e.colOf(t), t, s)
	v, _ := e.stepRows(s, w, e.topRow(t, s, w), n)
	t.setTop(v)
}

// posAt 把列 c 的窗格内的坐标换算成标签 t 的文档偏移。
func (e *Editor) posAt(c *column, t *tab, x, y view.Unit) int {
	s := t.doc.buf.Snapshot()
	w := e.wrapWidth(c, t, s)
	v, left := e.stepRows(s, w, e.topRow(t, s, w), int(y))
	if left > 0 {
		return s.Len()
	}
	tx := max(x-e.gutter(s), 0) + t.scrollX
	ls, _ := e.layoutWrapped(s, v.line, w)
	return ls + e.scratch.RowByteAt(v.sub, tx)
}

const doubleClick = 400 * time.Millisecond

func (e *Editor) handleMouse(in MouseInput) {
	if in.Button != MouseLeft {
		return
	}
	if in.Kind == MouseRelease {
		e.mouse.dragging = false
		return
	}
	if in.Column < 0 || in.Column >= len(e.cols) {
		return
	}
	if in.Kind == MousePress {
		e.closeOverlay()
		e.area = areaEditor
		e.findFocus = fieldNone
		if t := e.cols[in.Column].tab(); t != nil && t.vim.mode.visual() {
			t.vim.mode = vimNormal // 点一下结束可视模式；拖动重新选
		}
		if in.Column != e.col {
			e.col = in.Column
			e.sync()
		}
	}
	t := e.cols[in.Column].tab()
	if t == nil || t != e.loaded {
		return
	}
	if in.Kind == MousePress && t.diff != nil && in.Y == 0 && e.cols[in.Column].size.H > 6 {
		if line := diffStickyLine(t); line >= 0 { // 点钉在顶上的文件标题：回到它
			t.scrollY = line
			e.diffGo(t, line)
			return
		}
	}
	if in.Kind == MousePress && t.scrollY > 0 { // 点钉在顶上的行：跳到那个块
		h := int(e.cols[in.Column].size.H)
		if st := e.stickyLines(t.doc.buf.Snapshot(), t.scrollY, min(maxSticky, h/3), map[int]int{}); isCode(t.doc) && h > 6 && int(in.Y) < len(st) {
			e.jumpToScope(st[in.Y])
			return
		}
	}
	if in.Kind == MousePress && e.gutterClick(e.cols[in.Column], t, in.X, in.Y) {
		return // 点在改动标记上：开、关它的框
	}
	buf := t.doc.buf
	pos := e.posAt(e.cols[in.Column], t, in.X, in.Y)
	s := buf.Snapshot()
	main := buf.Selections().Main()
	if in.Kind == MousePress && e.linkClick(in) && definable(t) {
		if _, _, ok := identifierAt(t.doc, s, pos); ok {
			buf.SetSelections(edit.Single(edit.Cursor(pos)))
			e.mouse.clicks, e.mouse.dragging = 0, false
			if e.definitionAt(t, pos) == nil {
				e.guideSaw("editor.action.revealDefinition") // a link click is the command by mouse
			}
			return
		}
	}
	switch in.Kind {
	case MousePress:
		now := time.Now()
		if now.Sub(e.mouse.lastClick) < doubleClick && pos == e.mouse.lastPos {
			e.mouse.clicks++
		} else {
			e.mouse.clicks = 1
		}
		e.mouse.lastClick, e.mouse.lastPos, e.mouse.dragging = now, pos, true
		r := edit.Cursor(pos)
		switch {
		case e.mouse.clicks == 2 && t.diff != nil:
			buf.SetSelections(edit.Single(edit.Cursor(pos)))
			e.keep(t)
			if err := e.diffOpen(t); err != nil {
				e.status = err.Error()
			}
			return
		case e.mouse.clicks == 2:
			a, z := edit.WordAt(s, pos)
			r = edit.Range{Anchor: a, Head: z, GoalX: -1}
			e.keep(t) // 双击预览标签里的文字：转正
		case e.mouse.clicks >= 3:
			line := s.LineAt(pos)
			r = edit.Range{Anchor: s.LineStart(line), Head: s.LineStart(line + 1), GoalX: -1}
			if line+1 >= s.LineCount() {
				r.Head = s.Len()
			}
		case in.Mods&1 != 0: // Shift+点击：扩展选区
			r = edit.Range{Anchor: main.Anchor, Head: pos, GoalX: -1}
		}
		buf.SetSelections(edit.Single(r))
		buf.BreakGroup()
	case MouseDrag:
		if e.mouse.dragging {
			buf.SetSelections(edit.Single(edit.Range{Anchor: main.Anchor, Head: pos, GoalX: -1}))
			e.reveal()
		}
	}
}

// Frame 把当前状态算成显示模型，填进可复用的 dst。
func (e *Editor) Frame(dst *view.Frame) {
	e.sync()
	e.checkMaximized()
	dst.Version++
	dst.Title = "yan"
	if e.root != "" {
		dst.Title = filepath.Base(e.root) + " — yan"
	}
	switch {
	case e.ov != nil:
		dst.Focus = view.FocusOverlay
	case e.area == areaTree && e.sidebar && e.ws != nil:
		dst.Focus = view.FocusSidebar
	case e.find.open && e.findFocus != fieldNone:
		dst.Focus = view.FocusFind
	default:
		dst.Focus = view.FocusEditor
	}
	e.followReview(false)
	e.scheduleBlame()
	e.refreshDiffs()
	e.refreshMerges()
	e.revealReview()
	e.sidebarFrame(&dst.Sidebar)
	e.guideOfferFrame(&dst.Guide)
	e.arena = e.arena[:0]
	for len(dst.Columns) < len(e.cols) {
		dst.Columns = append(dst.Columns, view.Column{})
	}
	dst.Columns = dst.Columns[:len(e.cols)]
	for i, c := range e.cols {
		e.columnFrame(c, i == e.col && dst.Focus == view.FocusEditor, &dst.Columns[i])
		dst.Columns[i].Hidden = e.hiddenColumn(c)
		dst.Columns[i].Merge = mergeOf(c.tab()) != nil
	}
	dst.Maximized = e.maximized != nil
	welcome := e.welcomeColumn()
	for i, c := range e.cols {
		dc := &dst.Columns[i]
		dc.Closable = len(c.tabs) == 0 && e.closableColumn(c)
		if i != welcome {
			dc.Pane.Hints = nil
		}
	}
	dst.Active = e.col
	e.statusFrame(&dst.Status)
	e.overlayFrame(&dst.Overlay)
	e.findFrame(&dst.Find)
	e.reviewFrame(dst)
}

func (e *Editor) columnFrame(c *column, focused bool, dst *view.Column) {
	dst.Tabs = dst.Tabs[:0]
	for _, t := range c.tabs {
		b := t.doc.buf
		vt := view.Tab{
			Name: t.doc.name(), Path: b.Path(), Dirty: b.Dirty(), Preview: t.preview,
			ReadOnly: b.Snapshot().Meta().ReadOnly, Diff: t.diff != nil, History: t.diff != nil && t.diff.commit != nil,
		}
		if t.term != nil {
			vt.Name, vt.Terminal, vt.Path = t.term.title, t.term.id, t.term.dir
		}
		dst.Tabs = append(dst.Tabs, vt)
	}
	dst.Active = c.active
	dst.Weight = c.weight
	dst.Locked = c.locked
	p := &dst.Pane
	p.Size = c.size
	p.Rows = p.Rows[:0]
	p.Secondary = p.Secondary[:0]
	p.Caret = view.Caret{}
	p.Message, p.Hints = "", nil
	p.Terminal, p.Overview, p.Peek = 0, nil, nil
	p.CommitFiles = nil
	p.Sticky, dst.Crumbs = p.Sticky[:0], dst.Crumbs[:0]
	t := c.tab()
	if t == nil {
		p.Kind = view.PaneEmpty
		p.Hints = e.welcomeHints()
		return
	}
	p.Path, p.SVG, p.Zoom = "", nil, t.zoom
	p.TopLine, p.LineCount = 0, 0
	p.Markdown, p.Root = p.Markdown[:0], ""
	p.Terminal = 0
	if t.term != nil {
		p.Kind, p.Terminal, p.Path = view.PaneTerminal, t.term.id, t.term.dir
		return
	}
	if t.markdown {
		s := t.doc.buf.Snapshot()
		p.Kind, p.Path, p.Version = view.PaneMarkdown, t.doc.buf.Path(), s.Version()
		p.TopLine, p.LineCount, p.PreviewGen = t.previewLine, s.LineCount(), t.previewGen
		p.Markdown, p.Root = s.AppendRange(p.Markdown, 0, s.Len()), e.markdownRoot(p.Path)
		return
	}
	if t.image || t.svg {
		p.Kind, p.Path = view.PaneImage, t.doc.buf.Path()
		p.Message = filepath.Base(p.Path) + "：图片在窗口版里看（yan-app）"
		if t.svg {
			s := t.doc.buf.Snapshot()
			p.SVG = s.AppendRange(p.SVG[:0], 0, s.Len())
		}
		return
	}
	p.Kind = view.PaneCode
	e.codeFrame(t, c, focused, p)
	p.Split, p.GlowGen = nil, 0
	if t.diff != nil {
		e.splitFrame(t, c, p)
		p.GlowGen = t.diff.glowGen
		p.CommitFiles = e.commitFilesFrame(t)
	}
	e.stickyAndCrumbs(t, t.doc.buf.Snapshot(), int(c.size.H), e.sels(t).Main().Head, p, &dst.Crumbs)
}

func (e *Editor) codeFrame(t *tab, c *column, focused bool, pane *view.PaneFrame) {
	s := t.doc.buf.Snapshot()
	size := c.size
	sels := e.sels(t)
	main := sels.Main()
	gutter := e.gutter(s)
	w := e.wrapWidth(c, t, s)
	top := e.topRow(t, s, w)
	pane.TopLine, pane.LineCount = top.line, s.LineCount()
	pane.Gutter = gutter
	pane.ScrollX = t.scrollX
	if w > 0 {
		pane.ScrollX = 0
	}
	pane.Current = int32(s.LineAt(main.Head))
	h := int(size.H)
	hl := t.doc.hl
	src := source{s}
	hl.Request(src, top.line, top.line+h-1)
	e.refreshMarks(t.doc)
	lastLine := min(top.line+h, s.LineCount()) - 1
	dc := e.decorate(t, s, top.line, lastLine, sels)
	words := 0
	e.vis = e.vis[:0] // pane.Rows 里每一行是哪个视觉行
	for line := top.line; line < s.LineCount() && len(pane.Rows) < h; line++ {
		ls, le := s.LineRange(line)
		start := len(e.arena)
		e.arena = s.AppendRange(e.arena, ls, le)
		row := view.RowFrame{Line: int32(line), Text: e.arena[start:len(e.arena):len(e.arena)], Base: style.Text}
		if hl != nil {
			row.Spans = syntaxSpans(row.Spans, row.Text, hl.Line(line, string(row.Text)))
		}
		dc.row(&row, line, top.line, ls, &words)
		row.Mark = t.doc.marks.mark(line)
		if bl := e.blameFor(t.doc.buf.Path()); focused && bl.text != "" && bl.path == t.doc.buf.Path() && bl.line == line && main.Empty() {
			row.Ghost = bl.text
		}
		e.findSpans(&row, t.doc, ls)
		e.markReview(t, &row, line)
		if t.diff != nil {
			t.diff.row(&row, line)
		}
		if t.merge != nil || t.mergeSide != nil {
			e.mergeRow(t, &row, line)
		}
		if int32(line) == pane.Current && main.Empty() && t.diff == nil && (t.merge == nil && t.mergeSide == nil || row.Base == 0) {
			row.Base = style.CurrentLine
		}
		for _, r := range sels.Ranges {
			if r.Empty() || r.To() < ls || r.From() > le {
				continue
			}
			a, z := max(r.From(), ls)-ls, min(r.To(), le)-ls
			if r.To() > le { // 选区跨过行尾：多画一格，表示换行符也被选中
				z = le - ls + 1
			}
			if a < z {
				row.Spans = append(row.Spans, style.Span{Start: int32(a), End: int32(z), Style: style.Selection})
			}
		}
		if w <= 0 {
			pane.Rows = append(pane.Rows, row)
			e.vis = append(e.vis, vrow{line, 0})
			continue
		}
		e.layoutWrapped(s, line, w)
		e.wrapped = append(e.wrapped[:0], e.scratch.Rows...)
		sub := 0
		if line == top.line {
			sub = top.sub
		}
		for ; sub < len(e.wrapped) && len(pane.Rows) < h; sub++ {
			r := e.wrapped[sub]
			pane.Rows = append(pane.Rows, wrapRow(row, r.Start, r.End, sub == 0, sub == len(e.wrapped)-1))
			e.vis = append(e.vis, vrow{line, sub})
		}
	}
	if words == 1 { // 只有它自己：不标
		for i := range pane.Rows {
			r := &pane.Rows[i]
			r.Spans = slices.DeleteFunc(r.Spans, func(sp style.Span) bool { return sp.Style == style.Word })
		}
	}

	// 光标：有焦点的列里，主光标是真实光标（输入法候选框跟着它），其余光标由前端画成反色块或细线。
	for i, r := range sels.Ranges {
		at, rx := e.rowAt(s, w, r.Head)
		y := slices.Index(e.vis, at)
		if y < 0 {
			continue
		}
		x := gutter + rx - pane.ScrollX
		cr := view.Caret{X: x, Y: view.Unit(y), Height: 1, Visible: x >= gutter && x < size.W}
		if i == sels.Primary {
			pane.Caret = cr
			pane.Caret.Visible = cr.Visible && focused
			pane.Caret.Block = e.cfg.Vim && t.vim.mode != vimInsert
		} else if cr.Visible {
			pane.Secondary = append(pane.Secondary, cr)
		}
	}
	e.overviewFrame(t, s, top.line, pane)
	e.peekFrame(t, pane)
}

func (e *Editor) sidebarFrame(dst *view.Sidebar) {
	w := e.ws
	dst.Right = e.SidebarRight()
	dst.Visible = e.sidebar && (w != nil || e.side == sideGuide) && e.maximized == nil
	dst.Rows = dst.Rows[:0]
	if !dst.Visible {
		return
	}
	if e.side == sideGuide {
		dst.View = view.SideLearn
		e.guideFrame(&dst.Learn)
		if w == nil {
			return
		}
	}
	dst.View, dst.Changes = view.SideView(e.side), e.changeCount()
	switch e.side {
	case sideSCM:
		e.scmFrame(&dst.SCM)
	case sideSearch:
		e.searchFrame(&dst.Search)
	}
	e.refreshVisible()
	dst.Title = filepath.Base(w.root)
	dst.Busy = w.indexing
	rows := e.sideRows
	if w.sel < w.top {
		w.top = w.sel
	} else if w.sel >= w.top+rows {
		w.top = w.sel - rows + 1
	}
	w.top = max(min(w.top, len(w.visible)-rows), 0)
	for _, n := range w.visible {
		dst.Rows = append(dst.Rows, e.treeRow(n))
	}
	dst.Selected, dst.Top = w.sel, w.top
}

func (e *Editor) treeRow(n *workspace.Node) view.TreeRow {
	r := view.TreeRow{Name: n.Name, Depth: n.Depth, Dir: n.Dir, Expanded: n.Expanded, Ignored: n.Ignored}
	if e.repository().git.status != nil || len(e.docs) > 0 {
		abs := filepath.Join(e.root, filepath.FromSlash(n.Rel))
		r.Git = e.gitLetter(abs, n.Dir)
		if !n.Dir {
			if d := e.docOf(abs); d != nil {
				r.Modified = d.buf.Dirty()
			}
		}
	}
	return r
}

// welcomeHints are the welcome page's actions with their keys, each a command the window runs
// on a click. With a folder open they are the ways around it: files, symbols, search, Source
// Control; without one, how to open a folder.
func (e *Editor) welcomeHints() []view.Hint {
	type hint struct{ id, title string }
	hints := []hint{
		{"workbench.action.files.openFolder", "打开文件夹"},
		{"workbench.action.openRecent", "打开最近的文件夹"},
		{"workbench.action.showCommands", "命令面板"},
		{"workbench.action.terminal.toggleTerminal", "终端"},
		{"workbench.action.quit", "退出"},
	}
	if e.ws != nil {
		hints = []hint{
			{"workbench.action.quickOpen", "快速打开文件"},
			{"workbench.action.gotoSymbol", "转到文件里的符号"},
			{"workbench.action.findInFiles", "在文件中搜索"},
			{"workbench.view.scm", "源代码管理"},
			{"workbench.action.showCommands", "命令面板"},
			{"workbench.action.toggleSidebarVisibility", "显示/隐藏资源管理器"},
			{"workbench.action.terminal.toggleTerminal", "终端"},
			{"workbench.action.openRecent", "打开最近的文件夹"},
		}
		if e.repository().git.repo == nil {
			hints = slices.DeleteFunc(hints, func(h hint) bool { return h.id == "workbench.view.scm" })
		}
	}
	var out []view.Hint
	for _, h := range hints {
		if k := e.keyHint[h.id]; k != "" {
			out = append(out, view.Hint{Keys: k, Title: h.title, Command: h.id})
		}
	}
	return out
}

func (e *Editor) statusFrame(dst *view.Status) {
	dst.Message = e.status
	if dst.Message == "" && e.configProblem != "" {
		dst.Message = "配置有错：" + e.configProblem
	}
	dst.Action = view.StatusAction{}
	if a := e.action; a.text != "" && a.text == e.status {
		dst.Action = view.StatusAction{Label: a.label, Command: a.cmd, RepositoryID: a.repositoryID}
	}
	dst.Branch, dst.Busy = e.statusBranch(), e.repository().git.busy
	dst.Sync, dst.Publish = false, false
	if st := e.repository().git.status; st != nil && e.repository().git.repo != nil && st.Branch != "" && st.Head != "" {
		dst.Sync, dst.Publish = true, st.Upstream == ""
	}
	dst.Right = dst.Right[:0]
	t := e.cur()
	switch {
	case t != nil && t.term != nil:
		dst.Left = t.term.dir
	case t != nil && t.doc.untitled():
		dst.Left = t.doc.title
	case t != nil:
		dst.Left = e.rel(t.doc.buf.Path())
	case e.root != "":
		dst.Left = e.root
	default:
		dst.Left = "砚 yan"
	}
	if t == nil || t.image || t.term != nil {
		return // 图片：只显示路径
	}
	s := t.doc.buf.Snapshot()
	sels := e.sels(t)
	main := sels.Main()
	line := s.LineAt(main.Head)
	ls, _ := s.LineRange(line)
	col := 1
	width.Graphemes(s.AppendRange(nil, ls, main.Head), func(_, _, _ int) bool { col++; return true })
	m := s.Meta()
	if e.cfg.Vim {
		dst.Right = append(dst.Right, view.StatusItem{Text: t.vim.label(), Keep: view.KeepVim})
	}
	pos := fmt.Sprintf("行 %d，列 %d", line+1, col)
	if !main.Empty() {
		pos += fmt.Sprintf("（已选择 %d）", main.To()-main.From())
	}
	dst.Right = append(dst.Right, view.StatusItem{Text: pos, Keep: view.KeepPosition})
	if n := len(sels.Ranges); n > 1 {
		dst.Right = append(dst.Right, view.StatusItem{Text: fmt.Sprintf("%d 个光标", n), Keep: view.KeepCursors})
	}
	lang := "纯文本"
	if t.doc.hl != nil {
		lang = highlight.DisplayName(t.doc.hl.Lexer())
	}
	dst.Right = append(dst.Right,
		view.StatusItem{Text: e.indentOf(t).String(), Keep: view.KeepIndent, Command: "yan.indent.choose"},
		view.StatusItem{Text: m.Encoding.String(), Keep: view.KeepEncoding},
		view.StatusItem{Text: m.EOL.String(), Keep: view.KeepEOL, Command: "yan.eol.choose"},
		view.StatusItem{Text: lang, Keep: view.KeepLanguage})
}

func (e *Editor) overlayFrame(dst *view.Overlay) {
	ov := e.ov
	dst.Visible = ov != nil
	dst.Items = dst.Items[:0]
	if ov == nil {
		return
	}
	dst.Kind, dst.Input, dst.Footer = ov.kind, ov.input, ov.footer
	dst.Appearance = ""
	if ov.preview != nil {
		dst.Appearance = ov.preview.appearance
	}
	dst.Selected, dst.Top = ov.sel, ov.top
	dst.Title = "快速打开"
	switch ov.kind {
	case view.OverlayPalette:
		dst.Title = "命令面板"
	case view.OverlayBranch:
		dst.Title = "切换分支"
	case view.OverlayRecent:
		dst.Title = "最近打开"
	case view.OverlayPrompt, view.OverlayChoice, view.OverlayTheme:
		dst.Title = ov.title
	}
	for _, it := range ov.items {
		dst.Items = append(dst.Items, view.OverlayItem{Label: it.label, Detail: it.detail, Hint: it.hint, Font: it.font, Theme: it.theme, LabelHits: it.labelHit, DetailHits: it.detailHit, Unreleased: it.unreleased})
	}
}

// source 让高亮器读快照：偏移是字节。CRLF 文件的行之间是 "\r\n"，词法器只认 "\n"。
type source struct{ s text.Snapshot }

func (x source) Version() uint64               { return x.s.Version() }
func (x source) LineCount() int                { return x.s.LineCount() }
func (x source) LineRange(line int) (int, int) { return x.s.LineRange(line) }
func (x source) Slice(start, end int) string {
	str := x.s.String(start, end)
	if x.s.Meta().EOL == text.EOLCRLF {
		str = strings.ReplaceAll(str, "\r\n", "\n")
	}
	return str
}

// syntaxSpans 把高亮器的记号（行内第几个 rune）换成字节区间的样式，追加到 dst。
func syntaxSpans(dst []style.Span, line []byte, toks []highlight.Token) []style.Span {
	if len(toks) == 0 {
		return dst
	}
	r, b := 0, 0 // 第 r 个 rune 从第 b 个字节开始
	at := func(rn int) int {
		for r < rn && b < len(line) {
			_, n := utf8.DecodeRune(line[b:])
			b += n
			r++
		}
		return b
	}
	for _, t := range toks {
		a := at(t.Start)
		z := at(t.End)
		if z > a {
			dst = append(dst, style.Span{Start: int32(a), End: int32(z), Style: style.Keyword + style.ID(t.Kind-highlight.Keyword)})
		}
	}
	return dst
}
