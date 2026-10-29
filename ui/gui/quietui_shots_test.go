package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/view"
)

// TestQuietUIShots renders the workbench at the Quiet UI's two target window sizes, light and
// dark: the workspace with a changed file and a split, Source Control, and the command palette.
func TestQuietUIShots(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("SHOTS=目录 时才截图")
	}
	for _, size := range []struct{ w, h int }{{1440, 900}, {1100, 720}} {
		for _, appearance := range []string{"light", "dark"} {
			name := fmt.Sprintf("quiet-%dx%d-%s", size.w, size.h, appearance)
			root := t.TempDir()
			for p, c := range sample {
				full := filepath.Join(root, filepath.FromSlash(p))
				os.MkdirAll(filepath.Dir(full), 0o755)
				os.WriteFile(full, []byte(c), 0o644)
			}
			for _, args := range [][]string{
				{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=砚", "-c", "user.email=t@t", "commit", "-q", "-m", "第一次提交"},
			} {
				cmd := exec.Command("git", args...)
				cmd.Dir = root
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}
			os.WriteFile(filepath.Join(root, "notes.txt"), []byte("todo\n"), 0o644)
			h := newHarnessAt(t, root, Options{Appearance: appearance})
			h.tt.SetSize(size.w, size.h)
			h.open("README.md")
			h.open("main.go")
			h.ed.GotoLine(8)
			h.tt.Key(0, ui.KeyEnd)
			h.tt.Type(" // 改")
			h.tt.Key(ui.Cmd, ui.KeyS)
			h.tt.Key(0, ui.KeyEnter)
			h.tt.Type("fmt.Println(\"未保存\")")
			for i := 0; i < 6; i++ {
				time.Sleep(100 * time.Millisecond)
				h.settle()
			}
			h.shot(name + "-workspace")
			h.ed.Run("workbench.action.splitEditor")
			h.settle()
			h.shot(name + "-split")
			h.ed.Run("workbench.view.scm")
			for i := 0; i < 4; i++ {
				time.Sleep(100 * time.Millisecond)
				h.settle()
			}
			h.shot(name + "-scm")
			h.ed.Run("workbench.action.showCommands")
			h.settle()
			h.shot(name + "-palette")
		}
	}
}

// mergeFixture is a repository whose greeting.go is in conflict after merging feature into main.
func mergeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput()
		if err != nil && args[0] != "merge" {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	p := filepath.Join(root, "greeting.go")
	git("init", "-q", "-b", "main")
	os.WriteFile(p, []byte("package demo\n\nfunc Greeting() string {\n\treturn \"hello\"\n}\n\nconst Version = 1\n"), 0o644)
	git("add", ".")
	git("commit", "-qm", "base")
	git("checkout", "-q", "-b", "feature")
	os.WriteFile(p, []byte("package demo\n\nfunc Greeting() string {\n\treturn \"bonjour\"\n}\n\nconst Version = 1\n"), 0o644)
	git("commit", "-qam", "theirs")
	git("checkout", "-q", "main")
	os.WriteFile(p, []byte("package demo\n\n// Greeting says hello.\nfunc Greeting() string {\n\treturn \"你好\"\n}\n\nconst Version = 2\n"), 0o644)
	git("commit", "-qam", "ours")
	git("merge", "-q", "feature")
	return root
}

// openMerge opens the fixture's conflicted file from Source Control.
func openMerge(t *testing.T, h *harness) {
	t.Helper()
	h.ed.Run("workbench.view.scm")
	for i := 0; i < 50 && !slices.ContainsFunc(h.f.frame.Columns, func(c view.Column) bool {
		return len(c.Tabs) > 0 && strings.Contains(c.Tabs[c.Active].Name, "greeting")
	}); i++ {
		if gs := h.f.frame.Sidebar.SCM.Groups; len(gs) > 0 && len(gs[0].Rows) > 0 && gs[0].Rows[0].Letter == 'C' {
			h.ed.SCMClick(0, false)
		}
		h.settle()
	}
}

// TestQuietUIMergeShots renders a merge beside a terminal column at the compact window size.
func TestQuietUIMergeShots(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("SHOTS=目录 时才截图")
	}
	for _, appearance := range []string{"light", "dark"} {
		h := newHarnessAt(t, mergeFixture(t), Options{Appearance: appearance})
		h.tt.SetSize(1100, 720)
		openMerge(t, h)
		h.ed.Run("workbench.action.createTerminalEditor")
		h.settle()
		h.shot("quiet-1100x720-" + appearance + "-merge-terminal")
	}
}

// TestQuietUIMarkdownShots renders Chinese Markdown in the preview at both target sizes: the body
// reads at 16/28 beside 14/22 code, within a column about 770 pixels wide.
func TestQuietUIMarkdownShots(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("SHOTS=目录 时才截图")
	}
	doc := "# 本周任务：中文编辑体验\n\n砚的目标是做一个轻量、可靠的本地编辑器。当前版本在插入中文时按字节计算列宽，导致光标在长段落里偏移；本周要把它改成按字形计算，并补上对中文标点的排版测试。\n\n## 待办\n\n- 统一换行符为 LF\n- Insert 按字符计算列宽\n- 状态栏显示字数与阅读时间\n\n```go\nb := NewBuffer(\"task.md\")\nb.Insert(0, 2, \"，\")\n```\n\n完成后请在终端运行 `go test ./...`。\n"
	for _, size := range []struct{ w, h int }{{1440, 900}, {1100, 720}} {
		for _, appearance := range []string{"light", "dark"} {
			h := newHarness(t, map[string]string{"task.md": doc}, Options{Appearance: appearance})
			h.tt.SetSize(size.w, size.h)
			h.open("task.md")
			h.ed.Run("yan.svg.togglePreview")
			for range 4 {
				h.settle()
			}
			h.shot(fmt.Sprintf("quiet-%dx%d-%s-markdown", size.w, size.h, appearance))
		}
	}
}
