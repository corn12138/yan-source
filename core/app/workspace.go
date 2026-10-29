package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
	"github.com/corn12138/yan-source/core/workspace"
)

// area 是键盘焦点所在的区域（浮层打开时浮层优先）。
type area uint8

const (
	areaEditor area = iota
	areaTree
)

// ws 是打开的项目文件夹。
type ws struct {
	root     string
	ig       *workspace.Ignore
	tree     *workspace.Tree
	index    []string // 相对路径；只由所有者追加。后台模糊匹配拿的是 index[:n:n]，互不干扰
	indexing bool
	cancel   context.CancelFunc
	// indexCancel stops a refreshIndex walk; indexAgain asks for another once the running one ends.
	indexCancel context.CancelFunc
	indexAgain  bool
	// searchAfterIndex runs Find in Files again once the index reflects changed ignore rules.
	searchAfterIndex bool
	watcher          *workspace.Watcher // nil where the platform gives no file events

	visible []*workspace.Node // 本帧可见的树节点（每次 Frame 重算）
	sel     int               // 选中的可见节点下标
	top     int               // 第一行显示的节点下标
}

// OpenFolder 把 root 设为项目根：建 gitignore 规则、目录树，并在后台开始建立文件索引。
func (e *Editor) OpenFolder(root string) error {
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	if e.root != abs {
		e.restoreReview(nil)
	}
	e.root = abs
	e.wsEpoch++
	if e.ws != nil {
		e.ws.close()
	}
	ig := workspace.NewIgnore(abs)
	e.ws = &ws{root: abs, ig: ig, tree: workspace.NewTree(abs, ig)}
	e.sidebar = true
	e.reindex()
	e.watchFolder(e.ws)
	e.startGit(abs)
	if e.state != nil {
		e.state.AddRecent(abs)
		e.saveState()
	}
	// 上次留下的未命名文档：等调用的人把会话里的标签开好（同一轮里），再加在它们后面。
	epoch := e.wsEpoch
	e.Post(func(e *Editor) {
		if e.wsEpoch == epoch {
			e.restoreFolderDrafts()
		}
	})
	return nil
}

// reindex 取消上一次遍历并重新建立索引。
func (e *Editor) reindex() {
	w := e.ws
	if w == nil {
		return
	}
	if w.cancel != nil {
		w.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel, w.index, w.indexing = cancel, w.index[:0:0], true
	root, ig := w.root, w.ig
	go func() {
		// 后台 goroutine：只读文件系统，结果一律经 Post 交回所有者。
		err := workspace.Walk(ctx, root, ig, func(batch []string) {
			e.Post(func(e *Editor) {
				if e.ws != w || ctx.Err() != nil {
					return // 已被新的遍历取代
				}
				w.index = append(w.index, batch...)
				e.overlayIndexGrew()
			})
		})
		e.Post(func(e *Editor) {
			if e.ws == w && ctx.Err() == nil {
				w.indexing = false
				if err != nil {
					e.status = "索引出错：" + err.Error()
				}
				if e.fs.query != "" { // 索引建完了：搜索结果补全
					e.fs.key = ""
					e.searchSoon(0)
				}
				e.overlayIndexGrew()
				if w.indexAgain {
					w.indexAgain = false
					e.refreshIndex()
				}
			}
		})
	}()
}

// rel 返回 path 相对项目根的路径；不在项目里时返回绝对路径。
func (e *Editor) rel(path string) string {
	if e.root != "" {
		if r, err := filepath.Rel(e.root, path); err == nil && !strings.HasPrefix(r, "..") {
			return filepath.ToSlash(r)
		}
	}
	return path
}

// openPath 打开（或切换到）一个文件，作为正式标签，并记入最近打开。
func (e *Editor) openPath(path string) error {
	abs, err := e.checkOpenable(path)
	if err != nil {
		return err
	}
	c := e.fileColumn()
	return e.openIn(c, c.active+1, abs, false)
}

// checkOpenable 返回绝对路径；目录、二进制文件不在编辑器里打开。
func (e *Editor) checkOpenable(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if fi, err := os.Stat(abs); err == nil && !fi.Mode().IsRegular() {
		return "", errNotFile(abs)
	}
	if e.docOf(abs) == nil && !isImage(abs) && binary(abs) {
		return "", errBinary(abs)
	}
	return abs, nil
}

func (e *Editor) touchRecent(abs string) {
	for i, p := range e.recent {
		if p == abs {
			e.recent = append(e.recent[:i], e.recent[i+1:]...)
			break
		}
	}
	e.recent = append([]string{abs}, e.recent...)
	if len(e.recent) > 50 {
		e.recent = e.recent[:50]
	}
}

// binary 报告文件前 8 KB 是否含 NUL：这样的文件不在编辑器里打开。
func binary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8192)
	n, _ := f.Read(buf)
	return strings.IndexByte(string(buf[:n]), 0) >= 0
}

type userError string

func (u userError) Error() string { return string(u) }

func errNotFile(p string) error { return userError(filepath.Base(p) + " 不是普通文件") }
func errBinary(p string) error {
	return userError(filepath.Base(p) + " 是二进制文件，不打开")
}

// ---- 资源管理器命令 ----

func (e *Editor) treeNode() *workspace.Node {
	w := e.ws
	if w == nil || w.sel < 0 || w.sel >= len(w.visible) {
		return nil
	}
	return w.visible[w.sel]
}

func (e *Editor) refreshVisible() {
	if w := e.ws; w != nil {
		w.visible = w.tree.Visible(w.visible[:0])
		w.sel = max(min(w.sel, len(w.visible)-1), -1) // -1：什么都没选（点了文件下面的空白、按了 Esc）
	}
}

func (e *Editor) treeMove(n int) error {
	if w := e.ws; w != nil {
		e.refreshVisible()
		w.sel = max(min(w.sel+n, len(w.visible)-1), 0)
	}
	return nil
}

// treeLeft：展开的目录先收起，否则跳到父目录。
func (e *Editor) treeLeft() error {
	n := e.treeNode()
	if n == nil {
		return nil
	}
	if n.Dir && n.Expanded {
		n.Expanded = false
		e.refreshVisible()
		return nil
	}
	for i := e.ws.sel - 1; i >= 0; i-- {
		if e.ws.visible[i].Depth < n.Depth {
			e.ws.sel = i
			break
		}
	}
	return nil
}

// treeRight：收起的目录展开，已展开则进入第一个子项。
func (e *Editor) treeRight() error {
	n := e.treeNode()
	if n == nil || !n.Dir {
		return nil
	}
	if !n.Expanded {
		e.ws.tree.Expand(n)
		e.refreshVisible()
		e.watchTree()
		return nil
	}
	return e.treeMove(1)
}

// treeOpen：目录切换展开；文件在编辑器中打开。keep 为假时用预览标签打开、焦点留在资源管理器
// （空格、单击），为真时作为正式标签打开并把焦点交给编辑器（回车、双击）。
func (e *Editor) treeOpen(keep bool) error {
	n := e.treeNode()
	if n == nil {
		return nil
	}
	if n.Dir {
		if n.Expanded {
			n.Expanded = false
		} else {
			e.ws.tree.Expand(n)
			e.watchTree()
		}
		e.refreshVisible()
		return nil
	}
	abs, err := e.checkOpenable(filepath.Join(e.root, filepath.FromSlash(n.Rel)))
	if err != nil {
		return err
	}
	if !keep {
		return e.previewFile(abs)
	}
	c := e.fileColumn()
	if err := e.openIn(c, c.active+1, abs, false); err != nil {
		return err
	}
	e.area = areaEditor
	return nil
}

// ClickTree 选中资源管理器的第 i 个可见行并打开它：单击用预览标签，双击（open）转正并把焦点交给编辑器。
func (e *Editor) ClickTree(i int, open bool) {
	if e.ws == nil {
		return
	}
	e.sync()
	defer e.sync()
	e.closeOverlay()
	e.area = areaTree
	e.refreshVisible()
	if i < 0 || i >= len(e.ws.visible) {
		return
	}
	e.ws.sel = i
	n := e.ws.visible[i]
	if n.Dir && open {
		return // 双击目录：第一下已经展开或收起了
	}
	if err := e.treeOpen(open); err != nil {
		e.status = err.Error()
	}
}

// FocusSidebar 把键盘焦点交给资源管理器（点击它的空白处时）。
func (e *Editor) FocusSidebar() {
	if e.ws != nil && e.sidebar {
		e.closeOverlay()
		e.area = areaTree
	}
}

// revealInTree 展开到当前文件并选中它。
func (e *Editor) revealInTree() {
	t := e.cur()
	if e.ws == nil || t == nil || t.doc.untitled() {
		return
	}
	rel := e.rel(t.doc.buf.Path())
	if filepath.IsAbs(rel) {
		return
	}
	target := e.ws.tree.Reveal(rel)
	e.refreshVisible()
	e.watchTree()
	for i, n := range e.ws.visible {
		if n == target {
			e.ws.sel = i
		}
	}
}

// ---- 外部修改 ----

// checkDisk 在重新获得焦点、从终端返回时调用：刷新文件树；
// 磁盘上被改过的文件，没有未保存修改就静默重载，有则提示。
func (e *Editor) checkDisk() {
	if e.ws != nil {
		e.ws.ig.ReloadAll() // a .gitignore may have changed while no file events came
		e.ws.tree.Refresh()
		e.refreshVisible()
	}
	for _, d := range e.docs {
		if d.buf.DiskState() != edit.DiskChanged {
			continue
		}
		if d.buf.Dirty() {
			e.status = e.conflictError(d).Error() // 不自动保存它：Save 也会拒绝
			continue
		}
		s, err := text.Load(d.buf.Path())
		if err != nil {
			continue
		}
		e.reloading = true
		d.buf.Reload(s) // 以差异的形式应用：选区随之挪动，重载本身也能撤销
		e.reloading = false
	}
}

// SwitchFolder 在这个窗口里换一个文件夹（⌘O、最近打开）：先记下现在的标签，关掉所有文件
// （有没存的修改、又没开自动保存时不换），再打开新文件夹，恢复它上次的标签，没有就打开 README。
func (e *Editor) SwitchFolder(dir string) error {
	e.sync()
	defer e.sync()
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if fi, err := os.Stat(abs); err != nil {
		return err
	} else if !fi.IsDir() {
		return userError("不是文件夹：" + abs)
	}
	if e.ws != nil && e.ws.root == abs {
		return nil
	}
	e.saveDirty()
	if dirty, kept := e.Unsaved(); dirty {
		msg := "有没保存的修改：先保存或关掉那些文件，再换文件夹"
		if kept != nil {
			msg += "（未命名的文件没能留到下次：" + kept.Error() + "）"
		}
		return userError(msg)
	}
	e.SaveSession()
	if e.ws != nil {
		e.ws.close()
	}
	for _, d := range e.docs {
		d.hl.Close()
	}
	e.docs, e.loaded, e.closed = nil, nil, nil
	e.cols, e.col = []*column{newColumn()}, 0
	e.maximized = nil
	e.closeOverlay()
	e.closeFind()
	e.fs.stop()
	e.fs = fileSearch{gen: e.fs.gen} // 代号接着往上数：旧文件夹的结果对不上号
	e.scm = scmState{}
	e.chain = nil // runCommands belonged to the previous folder
	if err := e.OpenFolder(abs); err != nil {
		return err
	}
	if e.state != nil {
		e.state.AddRecent(abs)
		e.saveState()
	}
	e.reloadConfig()
	if !e.RestoreSession() {
		if r := readmeIn(abs); r != "" {
			e.openPath(r)
		}
	}
	e.area = areaEditor
	e.settingsGen++ // 窗口版重建菜单（最近打开）
	return nil
}

// readmeIn 返回文件夹里的 README（没有时返回 ""）。
func readmeIn(root string) string {
	for _, n := range []string{"README.md", "readme.md", "README.markdown", "README"} {
		p := filepath.Join(root, n)
		if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
			return p
		}
	}
	return ""
}

// RecentFolders 返回最近打开的文件夹（不含当前的、已经不在的）。
func (e *Editor) RecentFolders() []string { return e.recentFolders(false) }

// AllRecentFolders includes the active project so the Dock can bring it forward.
func (e *Editor) AllRecentFolders() []string { return e.recentFolders(true) }

func (e *Editor) recentFolders(includeCurrent bool) []string {
	if e.state == nil {
		return nil
	}
	var out []string
	for _, d := range e.state.Recent {
		if !includeCurrent && e.ws != nil && d == e.ws.root {
			continue
		}
		if fi, err := os.Stat(d); err == nil && fi.IsDir() {
			out = append(out, d)
		}
	}
	return out
}

// openRecentPicker 打开「最近打开的文件夹」浮层（⌃R）。
func (e *Editor) openRecentPicker() error {
	e.closeOverlay()
	e.ov = &overlay{kind: view.OverlayRecent}
	e.setOverlayInput("")
	return nil
}

// shortHome 把家目录下的路径写成 ~/…。
func shortHome(p string) string {
	if home, _ := os.UserHomeDir(); home != "" && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

// recentItems 列出符合输入的最近文件夹。
func (e *Editor) recentFolderItems(q string) {
	ov := e.ov
	ov.items, ov.footer = ov.items[:0], ""
	for _, d := range e.RecentFolders() {
		short := shortHome(d)
		if q != "" && !strings.Contains(strings.ToLower(short), strings.ToLower(q)) {
			continue
		}
		dir := d
		ov.items = append(ov.items, ovItem{label: filepath.Base(d), detail: short, run: func(e *Editor) error {
			if h, ok := e.host.(interface{ OpenFolder(string) error }); ok {
				return h.OpenFolder(dir)
			}
			return e.SwitchFolder(dir)
		}})
	}
	if len(ov.items) == 0 {
		ov.footer = "没有最近打开的文件夹"
	}
}

// NewFile 在 path 新建一个空文件并打开它（GUI 的 ⌘N 先用保存对话框选好位置）。
func (e *Editor) NewFile(path string) error {
	e.sync()
	defer e.sync()
	if _, err := os.Stat(path); err != nil {
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			return err
		}
	}
	if e.ws != nil {
		e.ws.tree.Refresh()
		e.reindex()
	}
	return e.openPath(path)
}
