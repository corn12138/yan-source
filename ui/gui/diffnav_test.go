package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// In the window, ] at a file's last change says so in the status bar, and one more ] crosses
// into the next file of the tab of all the changes.
func TestDiffNavigationBoundaryInTheWindow(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\nthree\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("four\nfive\nsix\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "base")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\n2\nthree\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("four\n6\nsix\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	// Repository discovery is asynchronous; the diff command requires a ready repository.
	h.ed.Run("workbench.view.scm")
	for end := time.Now().Add(15 * time.Second); h.f.frame.Sidebar.SCM.Branch != "main" && time.Now().Before(end); {
		h.settle()
	}
	if h.f.frame.Sidebar.SCM.Branch != "main" {
		t.Fatal("repository did not become ready")
	}
	h.ed.Run("workbench.view.explorer")
	h.settle()
	h.ed.Run("git.openAllChanges")
	h.ed.Run("workbench.action.focusActiveEditorGroup")
	loaded := func() bool {
		for _, row := range h.f.frame.Columns[h.f.frame.Active].Pane.Rows {
			if strings.Contains(string(row.Text), "@@") {
				return true
			}
		}
		return false
	}
	for end := time.Now().Add(15 * time.Second); !loaded() && time.Now().Before(end); {
		h.settle()
	}
	if !loaded() {
		t.Fatal("diff did not finish loading")
	}
	for i := 0; i < 3 && h.f.frame.Status.Message != "已是 a.txt 的最后一处改动，再按一次到 b.txt"; i++ {
		h.ed.Run("workbench.action.compareEditor.nextChange")
		h.settle()
	}
	if got := h.f.frame.Status.Message; got != "已是 a.txt 的最后一处改动，再按一次到 b.txt" {
		t.Fatalf("status %q", got)
	}
	h.shot("diff-navigation-boundary")
	h.tt.Type("]")
	h.settle()
	if got := h.f.frame.Status.Message; got != "第 2/2 处改动 · b.txt" {
		t.Fatalf("status after crossing %q", got)
	}
	h.shot("diff-navigation-crossed")
}
