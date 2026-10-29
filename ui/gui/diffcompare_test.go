package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The explorer's menu chooses a file for comparison and compares another with it; the Git menu's whitespace filter hides a change of line-end spaces from the diffs.
func TestCompareFilesAndWhitespaceInTheWindow(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(root, "old.go"), []byte("package demo\n\nfunc Name() string {\n\treturn \"old\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "new.go"), []byte("package demo\n\nfunc Name() string {\n\treturn \"new\"\n}\n\nfunc Extra() {}\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "base")
	os.WriteFile(filepath.Join(root, "old.go"), []byte("package demo   \n\nfunc Name() string {\n\treturn \"old\"\n}\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("workbench.view.explorer")
	h.settle()
	rows := func() string {
		var out []string
		for _, r := range h.f.frame.Columns[0].Pane.Rows {
			out = append(out, string(r.Text))
		}
		return strings.Join(out, "\n")
	}
	menu := func(subject, label string) {
		t.Helper()
		if err := h.tt.RightClick(subject); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(h.tt.Menu(), label) {
			t.Fatalf("no %s in %v", label, h.tt.Menu())
		}
		if err := h.tt.ChooseMenuItem(label); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	menu("old.go", "选为比较对象")
	menu("new.go", "与已选文件比较")
	if got := rows(); !strings.Contains(got, "@@ -4,1 +4,1 @@") || !strings.Contains(got, "func Extra() {}") {
		t.Fatalf("comparison rows:\n%s", got)
	}
	h.shot("compare-two-files")

	h.open("old.go")
	h.ed.Run("git.openChange")
	h.settle()
	if got := rows(); !strings.Contains(got, "@@ -1,1 +1,1 @@") {
		t.Fatalf("exact changes:\n%s", got)
	}
	h.ed.Run("yan.diff.whitespace")
	h.settle()
	if err := h.tt.Click("忽略行尾空白"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got := rows(); !strings.Contains(got, "（和 HEAD 一样，没有改动；忽略行尾空白）") {
		t.Fatalf("filtered changes:\n%s", got)
	}
	h.shot("diff-whitespace-eol")
}
