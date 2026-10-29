package gui

import (
	"path/filepath"
	"testing"
	"time"
)

// A folder in the breadcrumbs lists its files as a tree of its own: folders open in it without
// touching the explorer, and a file chosen there previews in the same column.
func TestBreadcrumbFolderList(t *testing.T) {
	h := newHarness(t, sample, Options{})
	h.open("core/edit/edit.go")
	if err := h.tt.Click("文件夹 core"); err != nil {
		t.Fatal(err)
	}
	waitText := func(s string) {
		t.Helper()
		for end := time.Now().Add(3 * time.Second); !h.tt.HasText(s) && time.Now().Before(end); {
			h.settle()
		}
		if !h.tt.HasText(s) {
			t.Fatalf("%q not shown", s)
		}
	}
	waitText("app")
	if !h.f.crumbs.open || h.f.crumbs.dir != filepath.Join(h.root, "core") {
		t.Fatalf("box %+v", h.f.crumbs.dir)
	}
	if err := h.tt.Click("app"); err != nil { // a folder: opens in the box
		t.Fatal(err)
	}
	waitText("app.go")
	h.shot("breadcrumb-folder")
	if err := h.tt.Click("app.go"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	col := h.f.frame.Columns[0]
	if h.f.crumbs.open || col.Tabs[col.Active].Name != "app.go" || !col.Tabs[col.Active].Preview {
		t.Fatalf("after choosing: open %v, tabs %+v", h.f.crumbs.open, col.Tabs)
	}
	for _, r := range h.f.frame.Sidebar.Rows {
		if r.Name == "app" && r.Expanded {
			t.Fatal("the box expanded the explorer's folder")
		}
	}
}

func TestCrumbDirs(t *testing.T) {
	root := filepath.FromSlash("/w/proj")
	got := crumbDirs(root, filepath.Join(root, "a", "b", "c.go"), []string{"a", "b", "c.go"})
	if got[0] != filepath.Join(root, "a") || got[1] != filepath.Join(root, "a", "b") || got[2] != "" {
		t.Fatalf("inside: %v", got)
	}
	out := crumbDirs(root, filepath.FromSlash("/etc/hosts"), []string{"", "etc", "hosts"})
	if out[1] != filepath.FromSlash("/etc") || out[2] != "" {
		t.Fatalf("outside: %v", out)
	}
}
