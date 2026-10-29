package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The header row of a change in a diff has buttons that revert the change or append the other
// side's lines, through the file's document; undo in the diff takes it back.
func TestDiffBlockButtonsInTheWindow(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	p := filepath.Join(root, "notes.txt")
	os.WriteFile(p, []byte("first\nsecond\nthird\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "base")
	os.WriteFile(p, []byte("first\nSECOND\nthird\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.open("notes.txt")
	h.ed.Run("git.openChange")
	for i := 0; i < 50 && !h.tt.HasText("↶ 还原"); i++ {
		h.settle()
	}
	h.shot("diff-block-buttons")
	if err := h.tt.Click("↶ 还原"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	rows := func() string {
		var out []string
		for _, r := range h.f.frame.Columns[0].Pane.Rows {
			out = append(out, string(r.Text))
		}
		return strings.Join(out, "\n")
	}
	if got := rows(); !strings.Contains(got, "和 HEAD 一样") {
		t.Fatalf("after revert:\n%s\nstatus %q", got, h.f.frame.Status.Message)
	}
	h.shot("diff-block-reverted")
	h.ed.Run("undo")
	h.settle()
	if got := rows(); !strings.Contains(got, "SECOND") {
		t.Fatalf("after undo:\n%s", got)
	}
	if err := h.tt.Click("⤓ 追加"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got := rows(); !strings.Contains(got, "@@ -2,0 +2,1 @@\nSECOND\nsecond") {
		t.Fatalf("after append:\n%s", got)
	}
}
