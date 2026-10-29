package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/syntax/highlight"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

// 文档、标签与列（模型照着 Mayoi）：
//   - document 是一个文件的文字和撤销历史；
//   - tab 是某一列里显示文档的一个标签，有自己的光标和滚动位置——同一个文件可以同时出现在几列里，
//     它们共享文字和撤销历史，各自保留光标；
//   - column 是编辑区的一列，有自己的标签条。

// document 是一个打开的文件。
type document struct {
	buf  *edit.Buffer
	tabs []*tab                 // 显示它的标签
	hl   *highlight.Highlighter // 后台的语法高亮（Chroma）；纯文本时为 nil
	// indent 是文档的缩进方式（第一次用到时从内容里看出来）。
	indent indentStyle
	// closers 是编辑器自动补上的右括号、引号的偏移：再打一遍就跨过去，退格时和左边的一起删。
	closers []int
	marks   gitMarks // 相对 HEAD 改了哪些行
	// overview 是窗格右边那一条里改了的行连成的段（overview.go）
	overview overviewCache
	// title 是还没存过的文档（没有路径）的名字，如「未命名-1」（untitled.go）。
	title string
	// askingPath 为真时另存为的对话框开着：不再开第二个。
	askingPath bool
}

// tab 是一列里的一个标签。
type tab struct {
	doc         *document
	sels        edit.Selections
	scrollY     int           // 顶部逻辑行
	scrollRow   vrow          // 自动换行时顶上是 scrollY 折出来的第几行（见 topRow）
	scrollX     view.Unit     // 水平滚动
	overview    view.Overview // 窗格右边那一条，每帧填一次（切片留着复用）
	wrap        int8          // ⌥Z 给这个标签单独定的自动换行：1 折行，-1 不折，0 按设置
	term        *termTab      // 终端标签（terminals.go）；文件的标签为 nil
	preview     bool          // 预览标签：下一个预览的文件顶替它，编辑或双击后转正
	diff        *diffTab      // 改动视图：文档是生成的只读文本
	merge       *mergeView    // 三方合并的结果：文件自己的文档（merge.go）
	mergeSide   *mergeSide    // 三方合并的一边：生成的只读文本
	image       bool          // 图片：文档只是占位
	svg         bool          // SVG 显示成图（⇧⌘V），文字还在
	previewLine int
	previewGen  uint64    // 预览第几次打开：前端据此滚到 previewLine
	markdown    bool      // Markdown 显示成排好的样子（⇧⌘V，只在窗口版），文字还在
	peek        *hunkPeek // 行号旁一处改动下面开着的框
	zoom        float64   // 图片的放大倍数；0 表示适应窗格
	vim         vimState  // 这个标签的 vim 模式
}

// column 是编辑区的一列。
type column struct {
	tabs   []*tab
	active int       // 正在显示的标签；-1 表示没有
	size   view.Size // 代码窗格的尺寸（前端给出）
	weight float64   // 这一列占编辑区宽度的份数（和别的列的份数比）；拖两列之间的边改它
	locked bool      // 隐式打开的文件、终端不进这一列；空了也留着（columnlock.go）
}

// closedTab 记下关掉的标签，Shift+Cmd+T 按原样重新打开。
type closedTab struct {
	term    *termTab // 重新打开只恢复目录和标题，创建新的 shell 和 ID
	path    string
	col     int
	at      int
	sels    edit.Selections
	scrollY int
}

func newColumn() *column { return &column{active: -1, size: view.Size{W: 80, H: 24}, weight: 1} }

// tab 返回列正在显示的标签。
func (c *column) tab() *tab {
	if c.active < 0 || c.active >= len(c.tabs) {
		return nil
	}
	return c.tabs[c.active]
}

// showing 返回列里显示 path 的标签下标；没有时返回 -1。
func (c *column) showing(path string) int {
	if path == "" {
		return -1
	}
	return slices.IndexFunc(c.tabs, func(t *tab) bool { return t.term == nil && t.doc.buf.Path() == path })
}

// column 返回有键盘焦点（或最后有）的列。
func (e *Editor) column() *column {
	e.col = max(min(e.col, len(e.cols)-1), 0)
	return e.cols[e.col]
}

// cur 返回有键盘焦点的列正在显示的标签。
func (e *Editor) cur() *tab { return e.column().tab() }

// sync 把选区在 buffer 和标签之间对齐：buffer 里的选区属于 e.loaded 标签；
// 当前标签变了，就把旧标签的选区存回去、把新标签的选区装进它文档的 buffer。
// 所有对外入口（Handle、Run、RunPending……）在开头和结尾各调用一次。
func (e *Editor) sync() {
	if t := e.loaded; t != nil {
		t.sels = t.doc.buf.Selections()
	}
	t := e.cur()
	if t != nil && t != e.loaded {
		t.doc.buf.SetSelections(t.sels)
		t.doc.buf.BreakGroup() // 换了个标签打字是新的一步撤销
	}
	e.loaded = t
}

// sels 返回标签的选区（当前标签的以 buffer 为准）。
func (e *Editor) sels(t *tab) edit.Selections {
	if t == e.loaded {
		return t.doc.buf.Selections()
	}
	return t.sels
}

func (e *Editor) docOf(path string) *document {
	if path == "" {
		return nil // 未命名的文档没有路径，彼此不是同一个
	}
	for _, d := range e.docs {
		if d.buf.Path() == path {
			return d
		}
	}
	return nil
}

// loadDoc 读入文件，或返回已经打开的文档。
func (e *Editor) loadDoc(path string) (*document, error) {
	if d := e.docOf(path); d != nil {
		return d, nil
	}
	s, err := text.Load(path)
	if err != nil {
		return nil, err
	}
	d := &document{buf: edit.NewBuffer(s)}
	e.adopt(d)
	if e.ws != nil {
		e.ws.watcher.Add(filepath.Dir(path))
	}
	if m := s.Meta(); m.ReadOnly {
		e.status = "只读打开：文件不是 UTF-8 或超过 8 MB"
	}
	return d, nil
}

// highlightFor 按文档现在的路径（扩展名）换上语法高亮；未命名的文档没有。
func (e *Editor) highlightFor(d *document) {
	if lx := highlight.Lexer(d.buf.Path()); d.buf.Path() != "" && lx != nil {
		// 高亮在后台算好后，发一个空闭包把前端叫醒重画。
		d.hl = highlight.New(lx, func() { e.Post(func(*Editor) {}) })
	}
}

// adopt 让编辑器管起文档 d：高亮、编辑时的自动保存与搜索、别的标签的光标跟着文字走。
func (e *Editor) adopt(d *document) {
	e.highlightFor(d)
	d.buf.OnEdit(func(ed edit.Edit) {
		d.trackClosers(ed)
		if d.untitled() {
			e.checkpointSoon()
		}
		if !e.reloading {
			e.autoSaveSoon()
		}
		e.searchEditSoon(d)
		// 其他标签的光标跟着文字走；正在编辑的标签的选区由 buffer 自己维护。
		for _, t := range d.tabs {
			if t != e.loaded {
				t.sels = t.sels.Map(ed.Changes)
				for i, r := range t.sels.Ranges {
					t.sels.Ranges[i].Anchor = ed.After.CaretOffset(r.Anchor)
					t.sels.Ranges[i].Head = ed.After.CaretOffset(r.Head)
				}
			}
			if t.preview && ed.Origin != edit.OriginUndo && t == e.loaded && !e.reloading {
				e.keep(t) // 编辑过的预览标签转正
			}
		}
	})
	e.docs = append(e.docs, d)
}

// openIn 在列 c 的第 at 位打开 path（列里已有就切过去）。preview 为真时作为预览标签打开。
func (e *Editor) openIn(c *column, at int, path string, preview bool) error {
	if i := c.showing(path); i >= 0 {
		if !preview {
			e.keep(c.tabs[i])
		}
		e.activate(c, i)
		return nil
	}
	var d *document
	var err error
	if isImage(path) { // 图片：文档只是占位（不读进内存，前端自己解码）
		if _, err := os.Stat(path); err != nil {
			return err
		}
		d = &document{buf: edit.NewBuffer(text.FromString("", text.Meta{Path: path, ReadOnly: true}))}
	} else if d, err = e.loadDoc(path); err != nil {
		return err
	}
	t := &tab{doc: d, sels: edit.Single(edit.Cursor(0)), preview: preview, image: isImage(path)}
	// 同一个文件在别的列里开着：新标签从那里的光标和滚动位置开始。
	for _, o := range d.tabs {
		t.sels, t.scrollY = e.sels(o), o.scrollY
		break
	}
	d.tabs = append(d.tabs, t)
	at = max(min(at, len(c.tabs)), 0)
	c.tabs = slices.Insert(c.tabs, at, t)
	if c.active >= at {
		c.active++
	}
	e.activate(c, at)
	if !preview {
		e.touchRecent(path)
	}
	return nil
}

// activate 显示列 c 的第 i 个标签，并让这一列获得焦点。
func (e *Editor) activate(c *column, i int) {
	if i < 0 || i >= len(c.tabs) {
		return
	}
	if old := e.loaded; e.autoSave && old != nil && old != c.tabs[i] && old.doc != c.tabs[i].doc &&
		old.doc.buf.Dirty() && old.doc.buf.Exists() {
		if err := e.saveDoc(old.doc); err != nil { // 换到别的标签：自动保存
			e.status = err.Error()
		}
	}
	c.active = i
	if c.tabs[i].term != nil {
		e.closeFind()
	}
	if at := slices.Index(e.cols, c); at >= 0 {
		e.col = at
	}
	e.sync()
}

// keep 让预览标签转正。
func (e *Editor) keep(t *tab) {
	if t.preview {
		t.preview = false
		e.touchRecent(t.doc.buf.Path())
	}
}

// previewFile 在当前列的预览标签里显示 path：已有标签显示它就切过去；否则顶替预览标签；
// 没有预览标签就新开一个。资源管理器单击、搜索结果用它，翻看文件时只留下一个标签。
func (e *Editor) previewFile(path string) error {
	c := e.fileColumn()
	if i := c.showing(path); i >= 0 {
		e.activate(c, i)
		return nil
	}
	old := slices.IndexFunc(c.tabs, func(t *tab) bool { return t.preview })
	at := c.active + 1
	if old >= 0 {
		if c.tabs[old].doc.buf.Dirty() {
			e.keep(c.tabs[old])
			old = -1
		} else {
			at = old
		}
	}
	n := len(c.tabs)
	if err := e.openIn(c, at, path, true); err != nil {
		return err
	}
	if old >= 0 && len(c.tabs) > n {
		e.dropTab(c, old+1) // 新标签插在它前面
	}
	return nil
}

// PreviewFileIn previews path in column col, as a click in the breadcrumbs' folder list does:
// in that column even when it is locked, since the user chose it there.
func (e *Editor) PreviewFileIn(col int, path string) error {
	if col < 0 || col >= len(e.cols) {
		return nil
	}
	e.sync()
	defer e.sync()
	abs, err := e.checkOpenable(path)
	if err != nil {
		return err
	}
	c := e.cols[col]
	e.col = col
	if c.locked { // previewFile routes away from a locked column: open there directly
		err = e.openIn(c, c.active+1, abs, true)
	} else {
		err = e.previewFile(abs)
	}
	e.area = areaEditor
	return err
}

// closeTab 关闭列 c 的第 i 个标签。force 为假且文档有未保存修改、又没有别的标签显示它时，
// 第一次只提醒，再按一次才放弃修改并关闭。
func (e *Editor) closeTab(c *column, i int, force bool) error {
	if i < 0 || i >= len(c.tabs) {
		return nil
	}
	t := c.tabs[i]
	const id = "workbench.action.closeActiveEditor"
	var saveErr error
	if e.autoSave && t.doc.buf.Dirty() && t.doc.buf.Exists() && len(t.doc.tabs) == 1 {
		saveErr = e.saveDoc(t.doc) // 自动保存时关掉就是存好再关；存不了就和没开自动保存一样先问
	}
	if !force && t.doc.buf.Dirty() && len(t.doc.tabs) == 1 && e.armed != id {
		e.armed = id
		e.status = t.doc.name() + " 有未保存的修改：" + e.hintOr("workbench.action.files.save", "保存") +
			"，或再按一次 " + e.hintOr(id, "关闭") + " 放弃修改并关闭"
		if saveErr != nil {
			e.status = saveErr.Error() + "。再按一次 " + e.hintOr(id, "关闭") + " 放弃修改并关闭"
		}
		return nil
	}
	e.armed = ""
	e.sync()
	ct := closedTab{path: t.doc.buf.Path(), col: slices.Index(e.cols, c), at: i, sels: e.sels(t), scrollY: t.scrollY}
	if t.term != nil {
		saved := *t.term
		ct.term = &saved
	}
	if !t.doc.untitled() { // 未命名的文档关掉文字就没了，重新打开不了
		e.closed = append(e.closed, ct)
	}
	if len(e.closed) > 30 {
		e.closed = e.closed[1:]
	}
	e.dropTab(c, i)
	return nil
}

// closeTabsIn 关掉列 c 里除了 keep 以外的标签（标签的右键菜单「关闭其他」「关闭全部」）；
// 有未保存修改的留着（开着自动保存时先存好再关）。
func (e *Editor) closeTabsIn(c *column, keep *tab) error {
	left := 0
	for i := len(c.tabs) - 1; i >= 0 && i < len(c.tabs); i-- {
		t := c.tabs[i]
		if t == keep {
			continue
		}
		if d := t.doc; d.buf.Dirty() && len(d.tabs) == 1 && !(e.autoSave && d.buf.Exists() && e.saveDoc(d) == nil) {
			left++
			continue
		}
		if err := e.closeTab(c, i, true); err != nil {
			return err
		}
	}
	if left > 0 {
		return fmt.Errorf("%d 个标签有未保存的修改，没有关", left)
	}
	return nil
}

// dropTab 把标签从列里拿掉；文档随最后一个标签关闭，列随最后一个标签消失（只剩一列时保留）。
func (e *Editor) dropTab(c *column, i int) {
	t := c.tabs[i]
	if e.loaded == t {
		e.sync()
		e.loaded = nil
	}
	c.tabs = slices.Delete(c.tabs, i, i+1)
	switch {
	case len(c.tabs) == 0:
		c.active = -1
	case c.active > i || c.active == len(c.tabs):
		c.active--
	}
	d := t.doc
	if t.diff != nil && t.diff.sides != nil { // 并排两窗格的高亮
		for _, h := range t.diff.sides.hl {
			h.Close()
		}
		t.diff.sides = nil
	}
	d.tabs = slices.DeleteFunc(d.tabs, func(o *tab) bool { return o == t })
	if len(d.tabs) == 0 {
		e.docs = slices.DeleteFunc(e.docs, func(o *document) bool { return o == d }) // 再打开时读磁盘上的内容
		d.hl.Close()
		if d.untitled() {
			e.checkpointNow() // 放弃了它：备份里也不再有
		}
	}
	if len(c.tabs) == 0 {
		e.removeColumn(c)
	}
	if e.cur() == nil && e.ws != nil && e.sidebar {
		e.area = areaTree
	}
	e.sync()
}

// reopenClosed 重新打开最近关掉的标签，回到原来的列、位置、光标和滚动位置。
func (e *Editor) reopenClosed() error {
	for len(e.closed) > 0 {
		ct := e.closed[len(e.closed)-1]
		e.closed = e.closed[:len(e.closed)-1]
		c := e.column()
		if ct.col >= 0 && ct.col < len(e.cols) {
			c = e.cols[ct.col]
		}
		if ct.term != nil {
			e.newTerminalTab(c, ct.at, ct.term.dir, ct.term.title)
			return nil
		}
		if err := e.openIn(c, ct.at, ct.path, false); err != nil {
			continue // 文件可能已经删了：试下一个
		}
		if t := e.cur(); t != nil {
			s := t.doc.buf.Snapshot()
			t.doc.buf.SetSelections(clampSels(ct.sels, s.Len()))
			t.scrollY = min(ct.scrollY, s.LineCount()-1)
		}
		return nil
	}
	return errors.New("没有关闭过的标签")
}

func clampSels(s edit.Selections, n int) edit.Selections {
	out := edit.Selections{Ranges: slices.Clone(s.Ranges), Primary: s.Primary}
	for i := range out.Ranges {
		out.Ranges[i].Anchor = min(out.Ranges[i].Anchor, n)
		out.Ranges[i].Head = min(out.Ranges[i].Head, n)
	}
	return out
}

// switchTo 切换到当前列的第 i 个标签。
func (e *Editor) switchTo(i int) {
	c := e.column()
	if i < 0 || i >= len(c.tabs) {
		return
	}
	e.activate(c, i)
	e.area = areaEditor
}

func (e *Editor) cycleTab(n int) error {
	c := e.column()
	if len(c.tabs) > 1 {
		e.switchTo((c.active + n + len(c.tabs)) % len(c.tabs))
	}
	return nil
}

// splitRight 在右边开一列，显示当前标签的文件（与 VS Code、Mayoi 的「向右拆分」一样）。
func (e *Editor) splitRight() error {
	t := e.cur()
	nc := newColumn()
	nc.size = e.column().size
	e.column().weight /= 2 // 新的一列分走这一列的一半宽度（同 Mayoi）
	nc.weight = e.column().weight
	e.cols = slices.Insert(e.cols, e.col+1, nc)
	e.col++
	e.area = areaEditor
	if t == nil {
		return nil
	}
	if t.term != nil {
		// 一个 PTY 只能有一个所有者；拆分创建新 shell，移动才保留会话。
		e.newTerminalTab(nc, 0, t.term.dir, "")
		return nil
	}
	if t.doc.untitled() {
		e.addTab(nc, 0, t.doc, t)
		return nil
	}
	return e.openIn(nc, 0, t.doc.buf.Path(), false)
}

// focusColumnBy 把焦点移到左边或右边的列（绕回）。
func (e *Editor) focusColumnBy(n int) error {
	e.FocusColumn((e.col + n + len(e.cols)) % len(e.cols))
	return nil
}

// moveTabBy 把当前标签移到左边或右边的列；没有那一列就新开一列。
func (e *Editor) moveTabBy(n int) error {
	c := e.column()
	if c.tab() == nil {
		return nil
	}
	to := e.col + n
	if to < 0 || to >= len(e.cols) {
		if len(c.tabs) == 1 {
			return nil // 只有一个标签的列挪到边上还是它自己
		}
		nc := newColumn()
		nc.size = c.size
		c.weight /= 2
		nc.weight = c.weight
		if to < 0 {
			e.cols = slices.Insert(e.cols, 0, nc)
			e.col++
			to = 0
		} else {
			e.cols = append(e.cols, nc)
		}
	}
	e.moveTab(c, c.active, e.cols[to], e.cols[to].active+1)
	return nil
}

// moveTab 把列 src 的第 i 个标签挪到列 dst 的第 at 个位置（照着 Mayoi 拖标签）：挪的是标签本身，
// 改动视图、提交的标签、光标、滚动位置都跟着走。dst 里已经有这个文件的标签时，显示那一个，这个不要了。
// src 空了就关掉这一列（只剩它一列时除外）。
func (e *Editor) moveTab(src *column, i int, dst *column, at int) {
	if i < 0 || i >= len(src.tabs) {
		return
	}
	e.sync()
	t := src.tabs[i]
	at = min(max(at, 0), len(dst.tabs))
	if src == dst {
		if at > i {
			at--
		}
		src.tabs = slices.Insert(slices.Delete(src.tabs, i, i+1), at, t)
		e.activate(src, at)
		e.area = areaEditor
		return
	}
	if e.loaded == t {
		e.loaded = nil
	}
	src.tabs = slices.Delete(src.tabs, i, i+1)
	switch {
	case len(src.tabs) == 0:
		src.active = -1
	case src.active > i || src.active == len(src.tabs):
		src.active--
	}
	same := slices.IndexFunc(dst.tabs, func(o *tab) bool { return o.doc == t.doc && o.diff == t.diff })
	if same >= 0 {
		t.doc.tabs = slices.DeleteFunc(t.doc.tabs, func(o *tab) bool { return o == t })
	} else {
		e.keep(t) // 挪到别的列：预览标签转正
		dst.tabs = slices.Insert(dst.tabs, at, t)
		same = at
	}
	if len(src.tabs) == 0 {
		e.removeColumn(src)
	}
	e.activate(dst, same)
	e.area = areaEditor
}

// ResizeColumns 拖第 i 列左边那条边（照着 Mayoi）：by 是挪给第 i-1 列的宽度占编辑区的比例（负的是从它那里拿），
// 两列都至少留 least（也是比例）。
func (e *Editor) ResizeColumns(i int, by, least float64) {
	if i <= 0 || i >= len(e.cols) {
		return
	}
	var total float64
	for _, c := range e.cols {
		total += c.weight
	}
	prev, cur := e.cols[i-1], e.cols[i]
	lo, hi := least*total-prev.weight, cur.weight-least*total
	if lo > hi {
		return
	}
	d := min(max(by*total, lo), hi)
	prev.weight, cur.weight = prev.weight+d, cur.weight-d
}

// MoveTab 把第 from 列的第 i 个标签拖到第 to 列的第 at 个位置（at 超出时放在最后）；to 等于列数时在最右边新开一列。
func (e *Editor) MoveTab(from, i, to, at int) {
	if from < 0 || from >= len(e.cols) || to < 0 || to > len(e.cols) {
		return
	}
	src := e.cols[from]
	if i < 0 || i >= len(src.tabs) {
		return
	}
	if to == len(e.cols) {
		if len(src.tabs) == 1 {
			return
		}
		nc := newColumn()
		nc.size = src.size
		src.weight /= 2
		nc.weight = src.weight
		e.cols = append(e.cols, nc)
	}
	e.moveTab(src, i, e.cols[to], at)
}

// ---- 前端用的语义动作：前端自己做命中测试，把「点了什么」交给内核 ----

// SetPaneSize 告诉内核第 col 列代码窗格的尺寸（Unit）；前端每次排版后调用。
func (e *Editor) SetPaneSize(col int, size view.Size) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	c := e.cols[col]
	if c.size != size {
		c.size = size
		e.sync()
		if col == e.col {
			e.reveal()
		}
		e.sync()
	}
}

// SetSidebarRows 告诉内核资源管理器能显示几行（键盘导航时保持选中行可见）。
func (e *Editor) SetSidebarRows(n int) { e.sideRows = max(n, 1) }

// ClickTab 显示第 col 列的第 i 个标签。keep 为真（双击）时预览标签转正。
func (e *Editor) ClickTab(col, i int, keep bool) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	e.sync()
	defer e.sync()
	c := e.cols[col]
	if i < 0 || i >= len(c.tabs) {
		return
	}
	if keep {
		e.keep(c.tabs[i])
	}
	e.activate(c, i)
	e.area = areaEditor
	e.closeOverlay()
}

// CloseTab 关闭第 col 列的第 i 个标签（标签上的 ×、中键）。
func (e *Editor) CloseTab(col, i int) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	e.sync()
	defer e.sync()
	if err := e.closeTab(e.cols[col], i, false); err != nil {
		e.status = err.Error()
	}
}

// FocusColumn 把键盘焦点交给第 col 列（点击代码窗格前前端会先调用它）。
func (e *Editor) FocusColumn(col int) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	e.sync()
	defer e.sync()
	e.col = col
	if t := e.cur(); t != nil && t.term != nil {
		e.closeFind()
	}
	e.area = areaEditor
}

// Columns 返回列数（前端排版用）。
func (e *Editor) Columns() int { return len(e.cols) }

func (e *Editor) splitColumn(col int, before bool) *column {
	src := e.cols[col]
	nc := newColumn()
	nc.size = src.size
	src.weight /= 2
	nc.weight = src.weight
	at := col + 1
	if before {
		at = col
	}
	e.cols = slices.Insert(e.cols, at, nc)
	if e.col >= at {
		e.col++
	}
	return nc
}

// MoveTabToSplit keeps the tab and shell identity when dropped on a column edge.
func (e *Editor) MoveTabToSplit(from, i, to int, before bool) {
	if from < 0 || from >= len(e.cols) || to < 0 || to >= len(e.cols) {
		return
	}
	src := e.cols[from]
	if i < 0 || i >= len(src.tabs) || from == to && len(src.tabs) == 1 {
		return
	}
	dst := e.splitColumn(to, before)
	e.moveTab(src, i, dst, 0)
}
