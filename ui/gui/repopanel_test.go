package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// The repository panel under Source Control opens from its header, lists the branches, and
// switches to the remotes from its tab.
func TestRepositoryPanelInTheWindow(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(t.TempDir(), "origin.git")
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("a\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "first")
	git("init", "-q", "--bare", bare)
	git("remote", "add", "origin", bare)
	git("push", "-q", "-u", "origin", "main")
	git("branch", "feature/login")
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("workbench.view.scm")
	wait := func(text string) {
		t.Helper()
		for end := time.Now().Add(15 * time.Second); !h.tt.HasText(text) && time.Now().Before(end); {
			h.settle()
		}
		if !h.tt.HasText(text) {
			t.Fatalf("no %q: %v", text, h.tt.Texts())
		}
	}
	wait("仓库")
	if err := h.tt.Click("仓库面板"); err != nil {
		t.Fatal(err)
	}
	wait("feature/login")
	h.shot("repository-panel-branches")
	if err := h.tt.Click("远程"); err != nil {
		t.Fatal(err)
	}
	wait("origin")
	h.shot("repository-panel-remotes")
}
