package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/i18n"
)

// 资源管理器里复制、粘贴文件，照着 Mayoi v0.6.0（MIT，cmd/mayoi/filecopy.go）：⌘C 复制选中的文件或文件夹，
// ⌘V 粘贴进选中的文件夹（选中的是文件就粘进它所在的文件夹，什么都没选就粘进打开的文件夹）；
// 窗口版走系统剪贴板，所以也能和访达、别的窗口之间复制。名字被占了就加「copy」，同访达。
// 点文件下面的空白、按 Esc 取消选中，新建的、粘贴的东西就落在打开的文件夹里。

// FileClipboard 是能放文件的剪贴板（窗口版用系统的；没有它时砚自己记着复制了什么）。
type FileClipboard interface {
	CopyFiles(paths []string) error
	ClipboardFiles() []string
}

// copyFiles 复制资源管理器里选中的文件或文件夹。
func (e *Editor) copyFiles() error {
	n := e.treeNode()
	if n == nil {
		return errors.New(i18n.T("Select the files to copy in the Explorer first"))
	}
	path := filepath.Join(e.root, filepath.FromSlash(n.Rel))
	if fc, ok := e.host.(FileClipboard); ok {
		if err := fc.CopyFiles([]string{path}); err != nil {
			return fmt.Errorf(i18n.T("Couldn't copy: %w"), err)
		}
	} else {
		e.copied = []string{path}
	}
	e.status = i18n.F("Copied %s", n.Name)
	return nil
}

// clipboardFiles 是可以粘贴的文件。
func (e *Editor) clipboardFiles() []string {
	if fc, ok := e.host.(FileClipboard); ok {
		return fc.ClipboardFiles()
	}
	return e.copied
}

// CanPaste 报告剪贴板里有没有能粘贴进资源管理器的文件（右键菜单用）。
func (e *Editor) CanPaste() bool { return len(e.clipboardFiles()) > 0 }

// pasteFiles 把剪贴板里的文件复制进资源管理器选中的文件夹，选中第一个。
func (e *Editor) pasteFiles() error {
	_, dir, ok := e.treeTarget()
	if !ok {
		return errors.New(i18n.T("No folder open"))
	}
	files := e.clipboardFiles()
	if len(files) == 0 {
		return errors.New(i18n.T("No files on the clipboard"))
	}
	var first string
	for _, p := range files {
		to, err := copyInto(p, dir)
		if err != nil {
			e.afterFileOp(first)
			return fmt.Errorf(i18n.T("Couldn't paste %s: %w"), filepath.Base(p), err)
		}
		if first == "" {
			first = to
		}
	}
	e.afterFileOp(first)
	e.status = i18n.F("Pasted %d items", len(files))
	return nil
}

// ClearTreeSelection 让资源管理器什么都不选（点文件下面的空白处时）：新建、粘贴的东西落在打开的文件夹里。
func (e *Editor) ClearTreeSelection() {
	e.sync()
	defer e.sync()
	if e.ws != nil {
		e.ws.sel = -1
		e.area = areaTree
		e.closeOverlay()
	}
}

// clearOrLeaveTree 是资源管理器里的 Esc：选着东西就取消选中，否则把焦点交回编辑器。
func (e *Editor) clearOrLeaveTree() error {
	if e.ws != nil && e.ws.sel >= 0 && e.side == sideExplorer {
		e.ws.sel = -1
		return nil
	}
	e.area = areaEditor
	return nil
}

// DropFiles 打开拖到第 col 列上的文件（从资源管理器、源代码管理、搜索结果或访达拖来的）；
// 拖来的是文件夹就打开那个文件夹。
func (e *Editor) DropFiles(col int, paths []string) {
	if col < 0 || col >= len(e.cols) {
		col = e.col
	}
	e.DropFilesAt(col, e.cols[col].active+1, paths)
}

// DropFilesAt inserts at a tab-strip position. Existing tabs move there instead of being duplicated.
func (e *Editor) DropFilesAt(col, at int, paths []string) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	e.sync()
	defer e.sync()
	e.dropFilesAt(e.cols[col], at, paths)
}

func (e *Editor) dropFilesAt(c *column, at int, paths []string) {
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			e.status = err.Error()
			continue
		}
		if fi.IsDir() {
			if err := e.SwitchFolder(p); err != nil {
				e.status = err.Error()
			}
			return
		}
		if err := e.openIn(c, at, p, false); err != nil {
			e.status = err.Error()
			continue
		}
		e.moveTab(c, c.active, c, at)
		at = c.active + 1
	}
	e.area = areaEditor
}

// DropFilesInSplit creates no empty column if none of the dropped paths can be opened.
func (e *Editor) DropFilesInSplit(col int, before bool, paths []string) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	e.sync()
	defer e.sync()
	c := e.splitColumn(col, before)
	e.dropFilesAt(c, 0, paths)
	if len(c.tabs) == 0 {
		// A directory drop may already have replaced the workspace and its columns.
		for i, o := range e.cols {
			if o == c {
				if i > 0 {
					e.cols[i-1].weight += c.weight
				} else if len(e.cols) > 1 {
					e.cols[1].weight += c.weight
				}
				e.cols = append(e.cols[:i], e.cols[i+1:]...)
				if e.col >= i {
					e.col = max(e.col-1, 0)
				}
				break
			}
		}
	}
}

// copyInto 把 src 复制进 dir，用一个那里没人用的名字，返回复制到了哪里。
func copyInto(src, dir string) (string, error) {
	info, err := os.Lstat(src)
	if err != nil {
		return "", err
	}
	if info.IsDir() && (dir == src || strings.HasPrefix(dir, src+string(filepath.Separator))) {
		return "", errors.New(i18n.T("Can't paste a folder into itself"))
	}
	to := freeName(dir, filepath.Base(src), info.IsDir())
	return to, copyEntry(src, to)
}

// freeName 是 dir 里的 name；被占了就是「name copy」「name copy 2」……，加在文件的扩展名前面，同访达。
func freeName(dir, name string, isDir bool) string {
	path := filepath.Join(dir, name)
	if _, err := os.Lstat(path); err != nil {
		return path
	}
	stem, ext := name, ""
	if e := filepath.Ext(name); !isDir && e != name {
		stem, ext = strings.TrimSuffix(name, e), e
	}
	for n := 1; ; n++ {
		suffix := " copy"
		if n > 1 {
			suffix = fmt.Sprintf(" copy %d", n)
		}
		path = filepath.Join(dir, stem+suffix+ext)
		if _, err := os.Lstat(path); err != nil {
			return path
		}
	}
}

// copyEntry 复制一个文件、链接或者整个文件夹，保留权限。
func copyEntry(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case info.IsDir():
		if err := os.Mkdir(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, en := range entries {
			if err := copyEntry(filepath.Join(src, en.Name()), filepath.Join(dst, en.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
