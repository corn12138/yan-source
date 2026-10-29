package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func TestExplorerFileOps(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"src/a.go": "package a\n"})
	e.Run("workbench.view.explorer")
	// 选中 src，新建 src/b/c.go（名字里带 /）
	e.SelectTree(0)
	e.Run("explorer.newFile")
	typeText(e, "b/c.go")
	e.Handle(key(keymap.KeyEnter, 0))
	if _, err := os.Stat(filepath.Join(root, "src/b/c.go")); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tabNames(e), " "); got != "c.go*" {
		t.Fatalf("tabs = %s", got)
	}
	// 开着的文件改名：以后存到新地方
	typeText(e, "package b\n")
	e.area = areaTree
	for i, r := range e.ws.visible {
		if r.Name == "c.go" {
			e.SelectTree(i)
		}
	}
	e.Run("renameFile")
	if e.ov == nil || e.ov.input != "c.go" {
		t.Fatalf("prompt = %+v", e.ov)
	}
	e.SetOverlayInput("d.go")
	e.Handle(key(keymap.KeyEnter, 0))
	e.FocusColumn(0)
	e.Run("workbench.action.files.save")
	if b, err := os.ReadFile(filepath.Join(root, "src/b/d.go")); err != nil || string(b) != "package b\n" {
		t.Fatalf("d.go = %q %v", b, err)
	}
	// 移到废纸篓要按两次；开着的文件随之关掉
	path := e.cur().doc.buf.Path()
	e.area = areaTree
	e.Handle(key(keymap.KeyDelete, 0))
	if _, err := os.Stat(path); err != nil {
		t.Fatal("first Delete should only ask")
	}
	e.Handle(key(keymap.KeyDelete, 0))
	if _, err := os.Stat(path); err == nil {
		t.Fatal("second Delete should trash the file")
	}
	if len(tabNames(e)) != 0 {
		t.Fatalf("tabs = %v", tabNames(e))
	}
}

// ⇧⌘V：窗口版把 Markdown 排好显示；终端版说一声，留在文字里。别的文件不能预览。
func TestTogglePreview(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "a.md")
	os.WriteFile(md, []byte("# hi\n"), 0o644)
	for _, gui := range []bool{true, false} {
		e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, keymap.Profile{Mods: []keymap.Mod{keymap.ModSuper}, GUI: gui, OS: "darwin"})
		if err := e.OpenFile(md); err != nil {
			t.Fatal(err)
		}
		e.Run("yan.svg.togglePreview")
		var f view.Frame
		e.Frame(&f)
		if p := f.Columns[0].Pane; gui && (p.Kind != view.PaneMarkdown || string(p.Markdown) != "# hi\n") || !gui && (p.Kind != view.PaneCode || !strings.Contains(e.status, "窗口版")) {
			t.Fatalf("gui %v: pane %v, status %q", gui, p.Kind, e.status)
		}
	}
}
