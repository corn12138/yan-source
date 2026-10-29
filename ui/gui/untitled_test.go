package gui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/keymap"
)

// The platform's new-file key opens an untitled tab named, not "."; saving it asks the system
// save dialog where, and the tab becomes that file.
func TestUntitledInTheWindow(t *testing.T) {
	h := newHarness(t, map[string]string{"a.txt": "a\n"}, Options{})
	mod := ui.Ctrl
	if slices.Contains(h.f.Profile().Mods, keymap.ModSuper) {
		mod = ui.Super
	}
	h.tt.Key(mod, ui.KeyN)
	h.settle()
	c := h.f.frame.Columns[h.f.frame.Active]
	if len(c.Tabs) == 0 || c.Tabs[c.Active].Name != "未命名-1" {
		t.Fatalf("tabs %+v", c.Tabs)
	}
	if texts := h.tt.Texts(); slices.Contains(texts, ".") || !slices.Contains(texts, "未命名-1") {
		t.Fatalf("texts %q", texts)
	}
	h.tt.Type("hello")
	h.settle()

	target := filepath.Join(h.root, "hello.txt")
	var asked string
	old := saveAsDialog
	t.Cleanup(func() { saveAsDialog = old })
	saveAsDialog = func(opts mygo.SaveDialogOptions) (string, error) { asked = opts.DefaultPath; return target, nil }
	h.ed.Run("workbench.action.files.save")
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); {
		if b, err := os.ReadFile(target); err == nil && string(b) == "hello" {
			break
		}
		h.settle()
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "hello" {
		t.Fatalf("saved %q %v (status %q)", b, err, h.f.frame.Status.Message)
	}
	if asked != filepath.Join(h.root, "未命名-1.txt") {
		t.Fatalf("dialog started at %q", asked)
	}
	h.settle()
	c = h.f.frame.Columns[h.f.frame.Active]
	if c.Tabs[c.Active].Name != "hello.txt" || c.Tabs[c.Active].Dirty {
		t.Fatalf("tab %+v", c.Tabs[c.Active])
	}
}

// Closing a window with only untitled drafts keeps them for next time without asking; when they
// cannot be kept, the question says so and Save All cannot close the window.
func TestWindowCloseKeepsDrafts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := newHarness(t, nil, Options{})
	h.ed.Run("workbench.action.files.newUntitledFile")
	h.tt.Type("draft")
	h.settle()
	g := &group{projects: []*Frontend{h.f}}
	old := closeDialog
	t.Cleanup(func() { closeDialog = old })
	asked := ""
	closeDialog = func(o mygo.MessageOptions) (mygo.MessageResult, error) {
		asked = o.Detail
		return mygo.MessageResult{Button: 0}, nil
	}
	if !g.mayClose() || asked != "" {
		t.Fatalf("kept drafts asked %q", asked)
	}
	blocked := filepath.Join(t.TempDir(), "not-a-dir")
	os.WriteFile(blocked, nil, 0o600)
	t.Setenv("XDG_CONFIG_HOME", blocked)
	h.tt.Type(" more")
	h.settle()
	if g.mayClose() || !strings.Contains(asked, "没能留到下次") {
		t.Fatalf("closed though the draft could not be kept: %q", asked)
	}
}
