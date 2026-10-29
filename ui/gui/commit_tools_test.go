package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestCommitToolsMenuAndExport(t *testing.T) {
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
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("before\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "first")
	first := git("rev-parse", "HEAD")
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("after\n"), 0o644)
	git("commit", "-qam", "second")
	second := git("rev-parse", "HEAD")
	git("remote", "add", "origin", "git@github.com:example/repo.git")
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	defer h.ed.Close()
	wait := func(ready func() bool) {
		t.Helper()
		until := time.Now().Add(8 * time.Second)
		for !ready() && time.Now().Before(until) {
			h.settle()
			time.Sleep(20 * time.Millisecond)
		}
		if !ready() {
			t.Fatalf("UI did not settle: %+v", h.f.frame.Status)
		}
	}
	h.ed.Run("workbench.view.scm")
	wait(func() bool { return len(h.f.frame.Sidebar.SCM.Commits) == 2 })
	menu := func(subject, label string) {
		t.Helper()
		if err := h.tt.RightClick(subject); err != nil {
			t.Fatal(err)
		}
		if err := h.tt.ChooseMenuItem(label); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	if err := h.tt.RightClick("second"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"导出补丁…", "复制改动补丁", "选为比较起点", "在远端打开提交"} {
		if !slices.Contains(h.tt.Menu(), label) {
			t.Fatalf("missing menu %s: %v", label, h.tt.Menu())
		}
	}
	h.tt.ChooseMenuItem("复制改动补丁")
	wait(func() bool { return strings.Contains(h.tt.Clipboard(), "-before\n+after") })
	menu("first", "选为比较起点")
	if h.ed.ComparisonStart() != first {
		t.Fatal("wrong comparison origin")
	}
	menu("second", "与选定起点比较")
	rows := func() string {
		var out []string
		for _, r := range h.f.frame.Columns[0].Pane.Rows {
			out = append(out, string(r.Text))
		}
		return strings.Join(out, "\n")
	}
	wait(func() bool { return strings.Contains(rows(), "a.txt  +1 −1  比较") })
	if !strings.Contains(rows(), "起点 "+first) || !strings.Contains(rows(), "终点 "+second) {
		t.Fatalf("comparison header: %s", rows())
	}
	if h.tt.HasText(".yan-history") {
		t.Fatal("virtual identity leaked into breadcrumbs")
	}
	h.shot("commit-comparison")
	menu("second", "在远端打开提交")
	wait(func() bool { return len(h.tt.OpenedURLs()) == 1 })
	if h.tt.OpenedURLs()[0] != "https://github.com/example/repo/commit/"+second {
		t.Fatal(h.tt.OpenedURLs())
	}
	path := filepath.Join(t.TempDir(), "export.patch")
	old := patchSaveDialog
	t.Cleanup(func() { patchSaveDialog = old })
	patchSaveDialog = func(opts mygo.SaveDialogOptions) (string, error) { return path, nil }
	menu("second", "导出补丁…")
	wait(func() bool { b, _ := os.ReadFile(path); return strings.Contains(string(b), "+after") })
	git("apply", "--reverse", "--check", path)
}
