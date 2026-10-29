package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func TestSearchInFilesAndReplace(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{
		"a.go":       "package a\n// hello world\nvar Hello = 1\n",
		"sub/b.txt":  "hello again\r\nbye\r\n",
		"skip/c.txt": "hello skipped\n",
	})
	e.Run("workbench.action.findInFiles")
	typeText(e, "hello")
	var f view.Frame
	pump(t, e, func() bool {
		e.Frame(&f)
		return !f.Sidebar.Search.Running && f.Sidebar.Search.Summary != ""
	})
	sr := f.Sidebar.Search
	if f.Sidebar.View != view.SideSearch || sr.Summary != "3 个文件中有 4 个结果" {
		t.Fatalf("search = %+v", sr)
	}
	// 区分大小写，并排除 skip/
	e.ToggleSearchOption(0)
	e.ToggleSearchOption(4)
	e.SetSearchField(3, "skip")
	pump(t, e, func() bool {
		e.Frame(&f)
		return !f.Sidebar.Search.Running && f.Sidebar.Search.Summary == "2 个文件中有 2 个结果"
	})
	// ↓ 进结果：在预览标签里显示第一处
	e.Handle(key(keymap.KeyDown, 0))
	e.Handle(key(keymap.KeyDown, 0))
	if tabs := tabNames(e); len(tabs) != 1 || tabs[0] != "~a.go*" {
		t.Fatalf("tabs = %v", tabs)
	}
	// 全部替换：开着的 a.go 改在编辑器里（可撤销），没开的 b.txt 直接写回（保留 CRLF）
	e.SetSearchField(1, "bye")
	e.ReplaceSearch(-1)
	if got := content(e); got != "package a\n// bye world\nvar Hello = 1\n" {
		t.Fatalf("a.go = %q", got)
	}
	b, _ := os.ReadFile(filepath.Join(root, "sub/b.txt"))
	if string(b) != "bye again\r\nbye\r\n" {
		t.Fatalf("b.txt = %q", b)
	}
	e.FocusColumn(0) // 键盘还在搜索视图里：回到编辑器再撤销
	e.Handle(ctrl('z'))
	if got := content(e); got != "package a\n// hello world\nvar Hello = 1\n" {
		t.Fatalf("undo = %q", got)
	}
}
