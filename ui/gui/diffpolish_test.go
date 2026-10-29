package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/app"
)

// All Changes in the window: file headers with status and ± bar, scoped @@ headers, the
// buttons of each change and file, and a header pinned at the top once it scrolls away.
func TestAllChangesPolishInTheWindow(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return string(out)
	}
	var body strings.Builder
	body.WriteString("package demo\n\nfunc Sum(xs []int) int {\n\ttotal := 0\n")
	for i := 0; i < 60; i++ {
		body.WriteString("\ttotal += 0\n")
	}
	body.WriteString("\tfor _, x := range xs {\n\t\ttotal += x\n\t}\n\treturn total\n}\n")
	git("init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(root, "sum.go"), []byte(body.String()), 0o644)
	os.WriteFile(filepath.Join(root, "notes.md"), []byte("# Notes\n\nfirst\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "base")
	changed := strings.Replace(body.String(), "\ttotal := 0\n", "\ttotal := 1\n", 1)
	changed = strings.Replace(changed, "\t\ttotal += x\n", "\t\ttotal += x * 2\n", 1)
	os.WriteFile(filepath.Join(root, "sum.go"), []byte(changed), 0o644)
	os.WriteFile(filepath.Join(root, "notes.md"), []byte("# Notes\n\nfirst\nsecond\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	rows := func() string {
		var out []string
		for _, r := range h.f.frame.Columns[0].Pane.Rows {
			out = append(out, string(r.Text))
		}
		return strings.Join(out, "\n")
	}
	for end := time.Now().Add(15 * time.Second); !strings.Contains(rows(), "@@ -4,1 +4,1 @@ func Sum(xs []int) int") && time.Now().Before(end); {
		if len(h.f.frame.Columns[0].Pane.Rows) == 0 {
			h.ed.Run("git.openAllChanges") // once the repository is read
		}
		h.settle()
	}
	if got := rows(); !strings.Contains(got, "M sum.go  +2 −2  更改  ■■■■■") || !strings.Contains(got, "@@ -4,1 +4,1 @@ func Sum(xs []int) int") {
		t.Fatalf("rows:\n%s", got)
	}
	for _, want := range []string{"打开文件", "＋ 暂存文件", "＋ 暂存", "↶ 还原"} {
		if !h.tt.HasText(want) {
			t.Fatalf("no %q: %v", want, h.tt.Texts())
		}
	}
	h.shot("all-changes-headers")
	h.ed.Handle(app.ScrollInput{Column: 0, DY: 8}) // sum.go's header leaves the top
	h.settle()
	if st := h.f.frame.Columns[0].Pane.Sticky; len(st) != 1 || !strings.Contains(string(st[0].Text), "sum.go") {
		t.Fatalf("sticky %d rows", len(st))
	}
	h.shot("all-changes-sticky-header")
	if err := h.tt.Click("＋ 暂存文件"); err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(10 * time.Second); !strings.Contains(git("diff", "--cached", "--name-only"), "sum.go") && time.Now().Before(end); {
		h.settle()
	}
	if got := git("diff", "--cached", "--name-only"); !strings.Contains(got, "sum.go") {
		t.Fatalf("staged %q", got)
	}
}
