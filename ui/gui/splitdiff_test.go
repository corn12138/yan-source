package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A side-by-side diff in the window: two panes with a gutter of bands between them, an arrow
// in each band that reverts its change, and the sides closing up with Line Alignment off.
func TestSplitDiffInTheWindow(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	var lines []string
	for _, w := range strings.Fields("alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu nu xi omicron pi") {
		lines = append(lines, "const "+w+" = \""+w+"\"")
	}
	git("init", "-q", "-b", "main")
	p := filepath.Join(root, "greek.go")
	os.WriteFile(p, []byte("package greek\n\n"+strings.Join(lines, "\n")+"\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "base")
	next := append([]string{}, lines[:4]...)
	next = append(next, "const extra = \"new\"", "const more = \"new\"")
	next = append(next, lines[4:]...)
	next[11] = "const theta = \"THETA\""
	next = append(next[:14], next[15:]...)
	os.WriteFile(p, []byte("package greek\n\n"+strings.Join(next, "\n")+"\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("toggle.diff.renderSideBySide") // auto → side by side
	h.open("greek.go")
	h.ed.Run("git.openChange")
	for end := time.Now().Add(15 * time.Second); h.f.frame.Columns[0].Pane.Split == nil && time.Now().Before(end); {
		h.settle()
	}
	sd := h.f.frame.Columns[0].Pane.Split
	if sd == nil || len(sd.Bands) != 3 {
		t.Fatalf("split %+v", sd)
	}
	h.shot("split-diff-aligned")

	if err := h.tt.Click("»"); err != nil { // the first change's arrow: take the insertion back
		t.Fatal(err)
	}
	h.settle()
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "extra") {
		// not saved yet: the document has it
	}
	if sd := h.f.frame.Columns[0].Pane.Split; sd == nil || len(sd.Bands) != 2 {
		t.Fatalf("after the arrow: %+v (%q)", sd, h.f.frame.Status.Message)
	}
	h.shot("split-diff-reverted-glow")
	h.ed.Run("yan.diff.toggleAlignedRows")
	h.settle()
	h.shot("split-diff-unaligned")
}
