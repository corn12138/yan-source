package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestReviewEditorDialog(t *testing.T) {
	h := newHarness(t, sample, Options{Appearance: "dark"})
	defer h.ed.Close()
	h.open("main.go")
	click := func(label string) {
		t.Helper()
		if err := h.tt.Click(label); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	click("+ 意见")
	if h.f.frame.Review == nil {
		t.Fatal("review did not open")
	}
	click("保存意见")
	if h.f.frame.Review == nil || h.f.frame.ReviewCount != 0 {
		t.Fatal("empty comment was saved")
	}
	h.tt.Type("请检查错误处理\n并保留测试")
	h.settle()
	if !strings.Contains(h.f.frame.Review.Text, "请检查") {
		t.Fatalf("input not captured: %+v", h.f.frame.Review)
	}
	h.shot("review-editor")
	click("保存意见")
	if h.f.frame.Review != nil || h.f.frame.ReviewCount != 1 {
		t.Fatal("save did not finish")
	}
	col := h.f.frame.Columns[0]
	if col.Tabs[col.Active].Dirty {
		t.Fatal("review typing changed source")
	}
	click("编辑意见 1")
	h.tt.Type("取消的内容")
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	if h.f.frame.Review != nil {
		t.Fatal("escape did not cancel")
	}
	click("编辑意见 1")
	if strings.Contains(h.f.frame.Review.Text, "取消的内容") {
		t.Fatal("cancel changed saved comment")
	}
	click("删除意见")
	if h.f.frame.ReviewCount != 0 {
		t.Fatal("delete failed")
	}
	h.ed.Run("review.undo")
	h.settle()
	if h.f.frame.ReviewCount != 1 {
		t.Fatal("undo failed")
	}
	h.tt.Type("// back in source\n")
	h.settle()
	col = h.f.frame.Columns[0]
	if !col.Tabs[col.Active].Dirty {
		t.Fatal("source did not regain focus")
	}
}

func TestReviewDiffCard(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "sample.go")
	os.WriteFile(path, []byte("first\nold value\nlast\n"), 0644)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=Yan", "-c", "user.email=yan@example.invalid", "commit", "-qm", "initial"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, out)
		}
	}
	os.WriteFile(path, []byte("first\nnew value\nlast\n"), 0644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	defer h.ed.Close()
	h.open("sample.go")
	// An idle UI does not imply the Git subprocess or diff loader has finished.
	wait := func(ready func() bool) {
		t.Helper()
		until := time.Now().Add(8 * time.Second)
		for !ready() && time.Now().Before(until) {
			h.settle()
		}
		if !ready() {
			t.Fatal("Git/diff data did not arrive")
		}
	}
	wait(func() bool { return h.f.frame.Status.Branch == "main" })
	h.ed.Run("git.openAllChanges")
	line := -1
	wait(func() bool {
		col := h.f.frame.Columns[0]
		if len(col.Tabs) == 0 || !col.Tabs[col.Active].Diff {
			return false
		}
		for _, r := range col.Pane.Rows {
			if strings.Contains(string(r.Text), "new value") {
				line = int(r.Line)
				return true
			}
		}
		return false
	})
	if line < 0 {
		t.Fatal("no changed row")
	}
	h.ed.ReviewLine(0, line)
	h.settle()
	h.tt.Type("请检查这里的返回值\n同时覆盖错误分支")
	if err := h.tt.Click("保存意见"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	pf := h.f.frame.Columns[0].Pane
	count := 0
	for _, r := range pf.Rows {
		if r.ReviewID != 0 {
			count++
		}
	}
	if count < 3 || pf.Overview == nil || len(pf.Overview.Reviews) != 1 {
		t.Fatal("comment card or overview marker missing")
	}
	h.shot("review-diff")
	if err := h.tt.Click("编辑意见 1"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.f.frame.Review == nil || !strings.Contains(h.f.frame.Review.Text, "错误分支") {
		t.Fatal("card did not reopen editor")
	}
}
