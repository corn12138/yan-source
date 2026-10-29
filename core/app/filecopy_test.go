package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/keymap"
)

// 资源管理器里 ⌘C、⌘V：粘进选中的文件夹，名字占了加 copy；文件夹不能粘进它自己。
// Esc 先取消选中（再粘贴就落在打开的文件夹里），再按一次回到编辑器。
func TestExplorerCopyPaste(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n", "src/b.go": "package b\n"})
	e.Run("workbench.view.explorer")
	selectRel := func(rel string) {
		t.Helper()
		e.refreshVisible()
		for i, n := range e.ws.visible {
			if n.Rel == rel {
				e.SelectTree(i)
				return
			}
		}
		t.Fatalf("no %s in the tree", rel)
	}
	cmdKey := func(r rune) KeyInput {
		return KeyInput{keymap.Chord{Mods: keymap.ModCtrl, Key: keymap.KeyRune, Rune: r}}
	}

	selectRel("a.txt")
	e.Handle(cmdKey('c'))
	if !e.CanPaste() {
		t.Fatal("nothing to paste after copying")
	}
	e.Handle(cmdKey('v')) // 选中的是文件：粘进它所在的文件夹
	if b, err := os.ReadFile(filepath.Join(root, "a copy.txt")); err != nil || string(b) != "a\n" {
		t.Fatalf("a copy.txt: %q, %v (status %q)", b, err, e.status)
	}
	selectRel("src")
	e.Handle(cmdKey('v'))
	if _, err := os.Stat(filepath.Join(root, "src", "a.txt")); err != nil {
		t.Fatalf("pasted into src: %v", err)
	}

	// 文件夹粘进它自己
	selectRel("src")
	e.Handle(cmdKey('c'))
	e.Handle(cmdKey('v'))
	if !strings.Contains(e.status, "它自己") {
		t.Fatalf("status %q", e.status)
	}

	// Esc：取消选中，粘贴落在打开的文件夹里；再按一次回到编辑器
	selectRel("a.txt")
	e.Handle(cmdKey('c'))
	e.Handle(key(keymap.KeyEsc, 0))
	if e.ws.sel != -1 || e.area != areaTree {
		t.Fatalf("after esc: sel %d, area %v", e.ws.sel, e.area)
	}
	e.Handle(cmdKey('v'))
	if _, err := os.Stat(filepath.Join(root, "a copy 2.txt")); err != nil {
		var all []string
		filepath.Walk(root, func(p string, _ os.FileInfo, _ error) error {
			all = append(all, strings.TrimPrefix(p, root))
			return nil
		})
		t.Fatalf("pasted into the workspace: %v (status %q), files %q, sel %d", err, e.status, all, e.ws.sel)
	}
	e.ClearTreeSelection()
	e.Handle(key(keymap.KeyEsc, 0))
	if e.area != areaEditor {
		t.Fatalf("second esc: area %v", e.area)
	}
}

// 拖到第二列上的文件在第二列打开。
func TestDropFilesOpensInTheColumn(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	e.OpenFile(filepath.Join(root, "a.txt"))
	e.Run("workbench.action.splitEditor")
	e.DropFiles(1, []string{filepath.Join(root, "b.txt")})
	if e.col != 1 || e.cur() == nil || filepath.Base(e.cur().doc.buf.Path()) != "b.txt" {
		t.Fatalf("column %d, tab %v", e.col, e.cur())
	}
	if t0 := e.cols[0].tab(); t0 == nil || filepath.Base(t0.doc.buf.Path()) != "a.txt" {
		t.Fatal("first column changed")
	}
}

func TestDropFilesAtAndSplit(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a", "b.txt": "b", "c.txt": "c"})
	defer e.Close()
	p := func(n string) string { return filepath.Join(root, n+".txt") }
	e.DropFiles(0, []string{p("a"), p("b")})
	e.DropFilesAt(0, 0, []string{p("c"), p("b")})
	if got := fmt.Sprint(tabNames(e)); got != "[c.txt b.txt* a.txt]" {
		t.Fatalf("drop order: %s", got)
	}
	e.DropFilesInSplit(0, true, []string{p("a")})
	if e.Columns() != 2 || e.col != 0 || e.cur().doc.buf.Path() != p("a") {
		t.Fatal("left-edge file split failed")
	}
	e.DropFilesInSplit(0, false, []string{p("missing")})
	if e.Columns() != 2 {
		t.Fatal("failed drop left an empty column")
	}
}
