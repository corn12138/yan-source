package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 资源管理器里的文件操作，照着 Mayoi：新建文件、新建文件夹、改名、移到废纸篓、在访达中显示、复制路径。

// treeTarget 返回资源管理器选中的那一项（绝对路径），和新建东西该放进的目录。
func (e *Editor) treeTarget() (path, dir string, ok bool) {
	if e.ws == nil {
		return "", "", false
	}
	e.refreshVisible()
	n := e.treeNode()
	if n == nil {
		return e.root, e.root, true
	}
	path = filepath.Join(e.root, filepath.FromSlash(n.Rel))
	dir = path
	if !n.Dir {
		dir = filepath.Dir(path)
	}
	return path, dir, true
}

// validName 检查文件名：不能空、不能含路径分隔以外的怪字符、不能已经存在。
func validName(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, 0) {
		return "", errors.New("名字不对")
	}
	p := filepath.Join(dir, filepath.FromSlash(name))
	if _, err := os.Lstat(p); err == nil {
		return "", fmt.Errorf("%s 已经有了", name)
	}
	return p, nil
}

func (e *Editor) newEntry(folder bool) error {
	_, dir, ok := e.treeTarget()
	if !ok {
		return errors.New("没有打开文件夹")
	}
	title, verb := "新建文件", "新建文件"
	if folder {
		title, verb = "新建文件夹", "新建文件夹"
	}
	rel := e.rel(dir)
	if rel == e.root || rel == "." {
		rel = ""
	} else {
		rel += "/"
	}
	e.prompt(title+"（在 "+filepath.Base(dir)+" 里，名字里可以带 /）", "", func(in string) string { return verb + " " + rel + in },
		func(e *Editor, in string) error {
			p, err := validName(dir, in)
			if err != nil {
				return err
			}
			if folder {
				if err := os.MkdirAll(p, 0o755); err != nil {
					return err
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(p, nil, 0o644); err != nil {
					return err
				}
			}
			e.afterFileOp(p)
			if !folder {
				if err := e.openPath(p); err != nil {
					return err
				}
				e.area = areaEditor // 新文件：接着就写
			}
			return nil
		})
	return nil
}

// afterFileOp 在文件操作之后刷新资源管理器、索引和 git，并选中 p。
func (e *Editor) afterFileOp(p string) {
	if e.ws == nil {
		return
	}
	e.ws.tree.Refresh()
	e.reindex()
	e.gitChanged()
	if p != "" {
		if rel := e.rel(p); !filepath.IsAbs(rel) {
			target := e.ws.tree.Reveal(rel)
			e.refreshVisible()
			for i, n := range e.ws.visible {
				if n == target {
					e.ws.sel = i
				}
			}
		}
	}
}

func (e *Editor) renameEntry() error {
	path, _, ok := e.treeTarget()
	if !ok || path == e.root {
		return errors.New("选中一个文件或文件夹再改名")
	}
	old := filepath.Base(path)
	e.prompt("改名："+old, old, func(in string) string { return old + " → " + in }, func(e *Editor, in string) error {
		if strings.TrimSpace(in) == old {
			return nil
		}
		np, err := validName(filepath.Dir(path), in)
		if err != nil {
			return err
		}
		if err := os.Rename(path, np); err != nil {
			return err
		}
		// 开着的文件跟着改路径：以后存到新地方。
		for _, d := range e.docs {
			if p := d.buf.Path(); p == path || strings.HasPrefix(p, path+string(filepath.Separator)) {
				d.buf.Rename(np + p[len(path):])
			}
		}
		e.afterFileOp(np)
		return nil
	})
	return nil
}

// trashEntry 把选中的文件或文件夹移到废纸篓（要按两次：开着的文件会关掉）。
func (e *Editor) trashEntry() error {
	path, _, ok := e.treeTarget()
	if !ok || path == e.root {
		return errors.New("选中一个文件或文件夹再删除")
	}
	id := e.running
	if e.armed != id {
		e.armed = id
		return fmt.Errorf("再按一次 %s 把 %s 移到%s", e.hintOr(id, "Delete"), filepath.Base(path), e.trashName())
	}
	e.armed = ""
	return e.TrashPath(path)
}

// TrashPath 把 path 移到废纸篓，关掉开着的、在它里面的文件（GUI 先问过用户）。
func (e *Editor) TrashPath(path string) error {
	e.sync()
	defer e.sync()
	if e.host == nil {
		return errors.New("当前前端不能移到" + e.trashName())
	}
	if err := e.host.Trash(path); err != nil {
		return fmt.Errorf("移到%s失败：%w", e.trashName(), err)
	}
	for _, c := range e.cols {
		for i := len(c.tabs) - 1; i >= 0; i-- {
			if p := c.tabs[i].doc.buf.Path(); p == path || strings.HasPrefix(p, path+string(filepath.Separator)) {
				e.dropTab(c, i)
			}
		}
	}
	e.afterFileOp("")
	e.status = filepath.Base(path) + " 已移到" + e.trashName()
	return nil
}

func (e *Editor) copyPath(relative bool) error {
	path, _, ok := e.treeTarget()
	if e.area != areaTree || !ok {
		t := e.cur()
		if t == nil {
			return errors.New("没有打开的文件")
		}
		if t.doc.untitled() {
			return userError(t.doc.title + " 还没存过，没有路径")
		}
		path = t.doc.buf.Path()
	}
	if relative {
		path = e.rel(path)
	}
	if e.host == nil {
		return errors.New("剪贴板不可用")
	}
	e.status = "已复制 " + path
	return e.host.SetClipboard(path)
}

// revealInOS 在访达里显示选中的文件（只有窗口版能做）。
func (e *Editor) revealInOS() error {
	path, _, ok := e.treeTarget()
	if e.area != areaTree || !ok {
		if t := e.cur(); t != nil {
			if t.doc.untitled() {
				return userError(t.doc.title + " 还没存过，没有文件可以显示")
			}
			path = t.doc.buf.Path()
		}
	}
	h, can := e.host.(interface{ Reveal(path string) })
	if !can {
		return errors.New("终端版不能在" + FileManager(e.profile.OS) + "中显示：路径已复制到剪贴板")
	}
	h.Reveal(path)
	return nil
}

// SelectTree 只选中资源管理器的第 i 行，不打开（右键菜单之前）。
func (e *Editor) SelectTree(i int) {
	if e.ws == nil {
		return
	}
	e.refreshVisible()
	if i >= 0 && i < len(e.ws.visible) {
		e.ws.sel = i
		e.side, e.area = sideExplorer, areaTree
	}
}

// TreePath 返回资源管理器第 i 个可见行的绝对路径。
func (e *Editor) TreePath(i int) string {
	if e.ws == nil {
		return ""
	}
	e.refreshVisible()
	if i < 0 || i >= len(e.ws.visible) {
		return ""
	}
	return filepath.Join(e.root, filepath.FromSlash(e.ws.visible[i].Rel))
}

// isImage 报告文件是不是看图打开的图片。
func isImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp":
		return true
	}
	return false
}

// imageZoom 改图片的放大倍数：by 为 0 时回到适应窗格；返回 false 表示当前标签不是图片（该缩放字号）。
func (e *Editor) imageZoom(by float64) bool {
	t := e.cur()
	if t == nil || !t.image && !t.svg {
		return false
	}
	switch {
	case by == 0:
		t.zoom = 0
	case t.zoom == 0:
		t.zoom = 1
		fallthrough
	default:
		if by > 0 {
			t.zoom = min(t.zoom*1.25, 32)
		} else {
			t.zoom = max(t.zoom/1.25, 0.05)
		}
	}
	return true
}

// ToggleImageFit 在适应窗格和实际大小之间切换（双击图片）。
func (e *Editor) ToggleImageFit(col int) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	if t := e.cols[col].tab(); t != nil && (t.image || t.svg) {
		if t.zoom == 0 {
			t.zoom = 1
		} else {
			t.zoom = 0
		}
	}
}

// SetImageZoom 设置图片的放大倍数（⌘ 加滚轮缩放时，前端算好的）。
func (e *Editor) SetImageZoom(col int, z float64) {
	if col >= 0 && col < len(e.cols) {
		if t := e.cols[col].tab(); t != nil && (t.image || t.svg) {
			t.zoom = max(min(z, 32), 0.05)
		}
	}
}

// togglePreview 把 SVG 显示成图、把 Markdown 排好显示，或显示回文字（⇧⌘V）。
// 同一个文件在另一列里开着时，那一列还是文字：边改边看。
func (e *Editor) togglePreview() error {
	t := e.cur()
	if t == nil || t.diff != nil {
		return errors.New("只有 SVG、Markdown 和 Mermaid 文件能预览")
	}
	switch strings.ToLower(filepath.Ext(t.doc.buf.Path())) {
	case ".svg":
		t.svg = !t.svg
	case ".md", ".markdown", ".mmd", ".mermaid":
		if !e.profile.GUI {
			return errors.New("Markdown 和 Mermaid 预览在窗口版里看（yan-app）")
		}
		// 原位切换时阅读位置跟着走：文字顶上的那一行成了预览的顶，预览的顶成了文字的顶（照着 Mayoi 0580cb5）。
		if t.markdown {
			t.setTop(vrow{line: max(0, min(t.previewLine, t.doc.buf.Snapshot().LineCount()-1))})
		} else {
			t.previewLine = t.scrollY
			t.previewGen++
		}
		t.markdown = !t.markdown
	default:
		return errors.New("只有 SVG、Markdown 和 Mermaid 文件能预览")
	}
	return nil
}

// markdownRoot 是 Markdown 里开头为 / 的链接相对的目录：文件所在的 git 仓库，或者打开的文件夹，同 GitHub。
func (e *Editor) markdownRoot(path string) string {
	if rc := e.repositoryForPath(path); rc != nil {
		return rc.git.repo.Root
	}
	if e.root != "" {
		if rel, err := filepath.Rel(e.root, path); err == nil && filepath.IsLocal(rel) {
			return e.root
		}
	}
	return ""
}
