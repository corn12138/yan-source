package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/keymap"
)

// Shift-click and the platform's modifier-click choose several files in Source Control, and a
// row's stage button stages all of them.
func TestSCMMultiSelectInTheWindow(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	for _, n := range []string{"alpha", "beta", "gamma", "delta"} {
		os.WriteFile(filepath.Join(root, n+".txt"), []byte(n+"\n"), 0o644)
	}
	git("add", ".")
	git("commit", "-qm", "base")
	for _, n := range []string{"alpha", "beta", "gamma", "delta"} {
		os.WriteFile(filepath.Join(root, n+".txt"), []byte(strings.ToUpper(n)+"\n"), 0o644)
	}
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("workbench.view.scm")
	// The Explorer also lists these names until Source Control's first frame;
	// wait for its own rows, not just the text.
	changes := func() (n int) {
		for _, g := range h.f.frame.Sidebar.SCM.Groups {
			n += len(g.Rows)
		}
		return n
	}
	for end := time.Now().Add(15 * time.Second); (changes() < 4 || !h.tt.HasText("delta.txt")) && time.Now().Before(end); {
		h.settle()
	}
	mod := ui.Ctrl
	if slices.Contains(h.f.Profile().Mods, keymap.ModSuper) {
		mod = ui.Super
	}
	if err := h.tt.Click("alpha.txt"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.ClickWith(ui.Shift, "beta.txt"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.ClickWith(mod, "delta.txt"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got := len(h.ed.SCMPicked()); got != 3 {
		t.Fatalf("picked %d: %q", got, h.f.frame.Status.Message)
	}
	h.shot("scm-multi-select")
	h.ed.Run("list.toggleExpand")
	for end := time.Now().Add(10 * time.Second); git("diff", "--cached", "--name-only") != "alpha.txt\nbeta.txt\ndelta.txt" && time.Now().Before(end); {
		h.settle()
	}
	if got := git("diff", "--cached", "--name-only"); got != "alpha.txt\nbeta.txt\ndelta.txt" {
		t.Fatalf("staged %q", got)
	}
}
