package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
)

func TestCommitFilesPanelAndHistory(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "a.txt"), []byte(strings.Repeat("a\n", 80)), 0o644)
	os.WriteFile(filepath.Join(root, "z.txt"), []byte("z\n"), 0o644)
	git("init", "-q", "-b", "main")
	git("add", ".")
	git("commit", "-qm", "tree files")
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	wait := func(ready func() bool) {
		t.Helper()
		end := time.Now().Add(8 * time.Second)
		for !ready() && time.Now().Before(end) {
			h.settle()
		}
		if !ready() {
			t.Fatalf("UI not ready: %s", h.f.frame.Status.Message)
		}
	}
	click := func(label string) {
		t.Helper()
		if err := h.tt.Click(label); err != nil {
			t.Fatal(err)
		}
		h.settle()
	}
	files := func() *view.CommitFiles { return h.f.frame.Columns[h.f.frame.Active].Pane.CommitFiles }
	h.ed.Run("workbench.view.scm")
	h.settle()
	wait(func() bool { return len(h.f.frame.Sidebar.SCM.Commits) == 1 })
	cm := h.f.frame.Sidebar.SCM.Commits[0]
	click("展开/折叠提交文件 " + cm.Short)
	wait(func() bool { return len(h.f.frame.Sidebar.SCM.Commits) == 3 })
	h.tt.Key(0, ui.KeyLeft)
	h.settle()
	if len(h.f.frame.Sidebar.SCM.Commits) != 1 {
		t.Fatal("history left did not collapse")
	}
	h.tt.Key(0, ui.KeyRight)
	h.settle()
	if len(h.f.frame.Sidebar.SCM.Commits) != 3 {
		t.Fatal("history right did not expand")
	}
	click("提交文件 src/a.txt")
	wait(func() bool { return files() != nil && !files().Loading && files().Count == 2 })
	if files().Visible {
		t.Fatal("panel should start hidden")
	}
	h.ed.Run("toggle.diff.renderSideBySide")
	h.settle()
	click("显示/隐藏提交文件列表")
	box, ok := h.tt.Find("提交文件列表")
	if !ok || box.W < 140 {
		t.Fatal("panel did not open")
	}
	code := h.f.cols[h.f.frame.Active].size
	if code.X+code.W > box.X+1 {
		t.Fatal("diff hit area overlaps file list")
	}
	click("提交文件树 z.txt")
	if files().Selected != filepath.Join(root, "z.txt") {
		t.Fatalf("selected %s", files().Selected)
	}
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	col := h.f.frame.Columns[h.f.frame.Active]
	if col.Tabs[col.Active].Preview {
		t.Fatal("Enter did not pin commit tab")
	}
	click("切换提交文件树/列表")
	click("提交文件树 src")
	if len(files().Rows) != 2 || !files().Rows[0].Collapsed {
		t.Fatalf("folder click: %+v", files().Rows)
	}
	h.tt.Key(0, ui.KeyRight)
	h.settle()
	if len(files().Rows) != 3 {
		t.Fatal("right did not expand folder")
	}
	box, _ = h.tt.Find("提交文件列表")
	h.tt.Press(box.X+2, box.Y+80)
	h.tt.Move(box.X-38, box.Y+80)
	h.tt.Release(box.X-38, box.Y+80)
	h.settle()
	if h.ed.CommitFilesWidth() < 250 {
		t.Fatalf("resize=%f", h.ed.CommitFilesWidth())
	}
	h.shot("commit-file-tree")
	h.ed.Run("toggle.diff.renderSideBySide")
	h.settle()
	if h.f.frame.Columns[h.f.frame.Active].Pane.Split != nil {
		t.Fatal("inline layout still split")
	}
	click("提交文件树 z.txt")
	if files().Selected != filepath.Join(root, "z.txt") {
		t.Fatal("inline file navigation failed")
	}
	h.tt.SetSize(600, 780)
	h.settle()
	if _, ok := h.tt.Find("提交文件列表"); ok {
		t.Fatal("narrow diff kept file panel")
	}
	h.tt.SetSize(1200, 780)
	h.settle()
	if _, ok := h.tt.Find("提交文件列表"); !ok {
		t.Fatal("wide diff did not restore panel")
	}
	click("隐藏提交文件列表")
	if files().Visible {
		t.Fatal("hide did not update preference")
	}
}
