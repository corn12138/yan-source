package gui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/view"
	"github.com/corn12138/yan-source/internal/launch"
)

func TestOpenExistingProjectPreservesDirtyEditor(t *testing.T) {
	h := newHarness(t, map[string]string{"sub/a.txt": "one\ntwo\nthree"}, Options{})
	defer h.ed.Close()
	h.ed.OpenFile(filepath.Join(h.root, "sub/a.txt"))
	h.ed.Handle(app.TextInput{Text: "dirty"})
	g := &group{projects: []*Frontend{h.f}}
	h.f.g = g
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(h.root, alias); err != nil {
		t.Fatal(err)
	}
	if err := h.f.openFolder(alias, true); err != nil {
		t.Fatal(err)
	}
	if !h.ed.Dirty() || len(g.projects) != 1 {
		t.Fatal("duplicate project or lost buffer")
	}
	r := launch.Parse([]string{filepath.Join(alias, "sub/a.txt:3")}, h.root)
	if err := h.f.openTarget(r.Targets[0], false); err != nil {
		t.Fatal(err)
	}
	var frame view.Frame
	h.ed.Frame(&frame)
	if len(frame.Columns[0].Tabs) != 1 || !frame.Columns[0].Tabs[0].Dirty {
		t.Fatal("file reopen discarded edits")
	}
}

func TestStatusSettingsAndRightSidebar(t *testing.T) {
	h := newHarness(t, map[string]string{"a.txt": "hello\nworld"}, Options{})
	defer h.ed.Close()
	h.ed.OpenFile(filepath.Join(h.root, "a.txt"))
	h.settle()
	if err := h.tt.Click("设置 LF"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.Click("CRLF"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if _, ok := h.tt.Find("设置 CRLF"); !ok {
		t.Fatal("EOL did not change")
	}
	h.ed.Run("workbench.action.toggleSidebarPosition")
	h.settle()
	if !h.f.frame.Sidebar.Right {
		t.Fatal("right flag missing")
	}
	item, ok := h.tt.Find("侧边栏")
	if !ok {
		t.Fatal("file tree missing")
	}
	if item.X < 900 {
		t.Fatalf("tree was not moved to right: %+v", item)
	}
	sw := h.f.opts.SidebarWidth
	x, y := item.X+2, item.Y+150
	h.tt.Press(x, y)
	for step := 1; step <= 5; step++ {
		h.tt.Move(x-float32(step)*15, y)
		h.tt.Frame()
	}
	h.tt.Release(x-75, y)
	h.settle()
	if h.f.opts.SidebarWidth < sw+40 {
		t.Fatal("right sidebar drag did not widen it")
	}
	h.shot("right-sidebar")
	h.ed.Run("workbench.action.toggleSidebarPosition")
	h.settle()
	if h.f.frame.Sidebar.Right {
		t.Fatal("left flag missing")
	}
}
