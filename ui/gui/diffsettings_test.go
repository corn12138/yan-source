package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// In the window, the diff's highlight and context are chosen from the palette's choices and
// redraw the open diff.
func TestDiffViewSettingsInTheWindow(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	var lines []string
	for _, w := range strings.Fields("alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu") {
		lines = append(lines, "const "+w+" = \""+w+"\"")
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(root, "greek.go"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "base")
	lines[6] = "const eta = \"ETA\""
	os.WriteFile(filepath.Join(root, "greek.go"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.open("greek.go")
	h.ed.Run("git.openChange")
	for i := 0; i < 50 && !h.tt.HasText("@@ -7,1 +7,1 @@"); i++ {
		h.settle()
	}
	rows := func() string {
		var out []string
		for _, r := range h.f.frame.Columns[0].Pane.Rows {
			out = append(out, string(r.Text))
		}
		return strings.Join(out, "\n")
	}
	choose := func(cmd, label string) {
		t.Helper()
		h.ed.Run(cmd)
		h.settle()
		if err := h.tt.Click(label); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	choose("yan.diff.highlight", "按字标出改了的部分")
	choose("yan.diff.context", "改动前后留 1 行")
	if got := rows(); !strings.Contains(got, "⋯ 5 行没改") || strings.Contains(got, "zeta\"\n") && strings.Contains(got, "epsilon") {
		t.Fatalf("context 1:\n%s", got)
	}
	h.shot("diff-char-highlight-context-1")
}
