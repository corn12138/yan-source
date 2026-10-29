package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/view"
)

// A click on a Source Control section's title folds it and shows how many files it has; the
// commits fold the same way, and a second click unfolds.
func TestSCMSectionsFoldInTheWindow(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	for _, n := range []string{"alpha", "beta"} {
		os.WriteFile(filepath.Join(root, n+".txt"), []byte(n+"\n"), 0o644)
	}
	git("add", ".")
	git("commit", "-qm", "base commit")
	for _, n := range []string{"alpha", "beta"} {
		os.WriteFile(filepath.Join(root, n+".txt"), []byte(strings.ToUpper(n)+"\n"), 0o644)
	}
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("workbench.view.scm")
	for end := time.Now().Add(15 * time.Second); !(h.tt.HasText("beta.txt") && h.tt.HasText("base commit")) && time.Now().Before(end); {
		h.settle()
	}
	changes := func() view.SCMGroup {
		for _, g := range h.f.frame.Sidebar.SCM.Groups {
			if g.Kind == view.GroupChanges {
				return g
			}
		}
		t.Fatal("no changes group")
		return view.SCMGroup{}
	}
	for _, label := range []string{"折起更改", "折起提交"} {
		if err := h.tt.Click(label); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	if g := changes(); !g.Folded || g.Count != 2 || len(g.Rows) != 0 || h.tt.HasText("beta.txt") {
		t.Fatalf("changes not folded: %+v", g)
	}
	if s := h.f.frame.Sidebar.SCM; !s.CommitsFolded || len(s.Commits) != 0 || h.tt.HasText("base commit") {
		t.Fatal("commits not folded")
	}
	h.shot("scm-folded")
	for _, label := range []string{"展开更改", "展开提交"} {
		if err := h.tt.Click(label); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	if g := changes(); g.Folded || len(g.Rows) != 2 || !h.tt.HasText("base commit") {
		t.Fatalf("not unfolded: %+v", g)
	}
}
