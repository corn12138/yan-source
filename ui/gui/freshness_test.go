package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A file another program rewrites shows its new text in the window, a file with unsaved edits
// keeps them and says so in the status bar, and a new file appears in the explorer.
func TestWindowFollowsFilesChangedOnDisk(t *testing.T) {
	h := newHarness(t, sample, Options{})
	h.open("main.go")
	texts := func() string { return strings.Join(h.tt.Texts(), "|") }
	waitFor := func(what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for !cond() {
			if time.Now().After(deadline) {
				t.Fatalf("%s: texts %q, status %q", what, texts(), h.f.frame.Status.Message)
			}
			h.settle()
		}
	}
	before := h.f.frame.Columns[0].Pane.LineCount
	os.WriteFile(filepath.Join(h.root, "main.go"), []byte("package main\n\n// 外部改过\n"), 0o644)
	waitFor("reload", func() bool {
		return h.f.frame.Columns[0].Pane.LineCount == 4 && !h.f.frame.Columns[0].Tabs[0].Dirty
	})
	if before == 4 {
		t.Fatal("sample must differ in length")
	}

	h.open("README.md")
	h.tt.Type("mine ")
	h.settle()
	os.WriteFile(filepath.Join(h.root, "README.md"), []byte("theirs\n"), 0o644)
	waitFor("conflict notice", func() bool { return strings.Contains(h.f.frame.Status.Message, "README.md") })
	if c := h.f.frame.Columns[0]; !c.Tabs[c.Active].Dirty {
		t.Fatal("unsaved edits lost")
	}

	os.WriteFile(filepath.Join(h.root, "added.txt"), []byte("x\n"), 0o644)
	waitFor("explorer", func() bool { return strings.Contains(texts(), "added.txt") })
	h.shot("freshness")
}
