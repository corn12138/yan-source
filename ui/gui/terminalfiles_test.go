package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

func TestTerminalFileTarget(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	file := filepath.Join(root, "a b.go")
	homeFile := filepath.Join(home, "中文.go")
	for _, path := range []string{file, homeFile} {
		if err := os.WriteFile(path, []byte("text"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		link, path string
		line, col  int
	}{
		{file, file, 0, 0}, {file + ":12:3", file, 12, 3},
		{"a b.go:12", file, 12, 0}, {"./a b.go:2:4", file, 2, 4},
		{"~/中文.go:7:2", homeFile, 7, 2}, {`~\中文.go`, homeFile, 0, 0},
	} {
		path, line, col, ok := terminalFileTarget(tc.link, root, home)
		if !ok || path != tc.path || line != tc.line || col != tc.col {
			t.Errorf("%q: got %q %d:%d %v", tc.link, path, line, col, ok)
		}
	}
	for _, link := range []string{"", root, root + ":2", "missing.go:2", "a b.go:0", "a b.go:-1", "a b.go:2:0", "a b.go:99999999999999999999999999", "https://example.com/a.go:2", "file://" + file, "mailto:a.go", "a b.go:2\x00"} {
		if _, _, _, ok := terminalFileTarget(link, root, home); ok {
			t.Errorf("accepted %q", link)
		}
	}
	if _, _, _, ok := terminalFileTarget("a b.go:2", "relative", home); ok {
		t.Fatal("resolved against process cwd instead of terminal cwd")
	}
	if _, _, _, ok := terminalFileTarget("~/中文.go", root, ""); ok {
		t.Fatal("resolved an unknown home")
	}
	if runtime.GOOS != "windows" {
		literal := filepath.Join(root, "a b.go:12:3")
		if err := os.WriteFile(literal, []byte("literal"), 0o600); err != nil {
			t.Fatal(err)
		}
		if path, line, col, ok := terminalFileTarget(literal, root, home); !ok || path != literal || line != 0 || col != 0 {
			t.Fatal("numeric suffix overrode an existing literal filename")
		}
	}
}

func TestTerminalFileOpenPreservesDocumentsAndColumns(t *testing.T) {
	h := newHarness(t, map[string]string{"main.go": "one\n中😀x\n", ".gitignore": "ignored/\n", "ignored/out.go": "one\ntwo\n"}, Options{})
	h.open("main.go")
	h.tt.Type("unsaved ")
	h.settle()
	h.ed.Run("workbench.action.splitEditor")
	h.ed.Run("view.lockColumn")
	h.settle()
	s := &termSession{dir: h.root}
	if !h.f.openTerminalFile(s, "ignored/out.go:2:2") {
		t.Fatal("ignored file did not open")
	}
	h.settle()
	assertTerminalLocation(t, h, "ignored/out.go", "行 2，列 2")
	if h.f.frame.Active != 0 || !h.f.frame.Columns[1].Locked {
		t.Fatal("file open replaced the locked column")
	}
	if !h.f.openTerminalFile(s, "main.go:2:3") {
		t.Fatal("open dirty document")
	}
	h.settle()
	assertTerminalLocation(t, h, "main.go", "行 2，列 3")
	c := h.f.frame.Columns[h.f.frame.Active]
	if !c.Tabs[c.Active].Dirty || len(c.Tabs) != 2 {
		t.Fatal("reopening discarded edits or duplicated the file")
	}
	if data, err := os.ReadFile(filepath.Join(h.root, "main.go")); err != nil || strings.Contains(string(data), "unsaved") {
		t.Fatal("link navigation saved the dirty document")
	}
	for _, link := range []string{"missing.go:2", "https://example.com", "."} {
		if h.f.openTerminalFile(s, link) {
			t.Fatalf("accepted %q", link)
		}
	}
	s.closed = true
	if h.f.openTerminalFile(s, "main.go:1") {
		t.Fatal("closed terminal changed the editor")
	}
	h.settle()
	assertTerminalLocation(t, h, "main.go", "行 2，列 3")
}

func assertTerminalLocation(t *testing.T, h *harness, rel, position string) {
	t.Helper()
	c := h.f.frame.Columns[h.f.frame.Active]
	if c.Tabs[c.Active].Path != filepath.Join(h.root, filepath.FromSlash(rel)) {
		t.Fatalf("opened %s, want %s", c.Tabs[c.Active].Path, rel)
	}
	for _, item := range h.f.frame.Status.Right {
		if item.Keep == view.KeepPosition {
			if item.Text != position {
				t.Fatalf("position %q, want %q", item.Text, position)
			}
			return
		}
	}
	t.Fatal("missing cursor position")
}
