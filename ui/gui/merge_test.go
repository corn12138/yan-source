package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

// A conflicted file opens from Source Control as three columns; a side's buttons take its
// lines into the result, and the status bar's button marks the file resolved.
func TestMergeViewInTheWindow(t *testing.T) {
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
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("workbench.view.scm")
	// Click the conflicted file once Source Control has read the conflict.
	for i := 0; i < 50 && len(h.f.frame.Columns) < 3; i++ {
		if gs := h.f.frame.Sidebar.SCM.Groups; len(gs) > 0 && len(gs[0].Rows) > 0 && gs[0].Rows[0].Letter == 'C' {
			h.ed.SCMClick(0, false)
		}
		h.settle()
	}
	if n := len(h.f.frame.Columns); n != 3 {
		t.Fatalf("%d columns, status %q, texts %v", n, h.f.frame.Status.Message, h.tt.Texts())
	}
	text := func(ci int) string {
		var out []string
		for _, r := range h.f.frame.Columns[ci].Pane.Rows {
			out = append(out, string(r.Text))
		}
		return strings.Join(out, "\n")
	}
	if !strings.Contains(text(0), "我们的 · main") && !strings.Contains(text(0), "你好") || !strings.Contains(text(1), "\"hello\"") || !strings.Contains(text(1), "Version = 2") {
		t.Fatalf("columns:\n%s\n---\n%s", text(0), text(1))
	}
	h.shot("merge-view")
	// In a narrow window the conflict's buttons that do not fit go into "⋯" (Mayoi #63).
	// Narrow the window until a bar first runs out of room: "✕ 忽略" goes before "✓ 用这边".
	for w := 1200; w >= 480 && !h.tt.HasText("⋯"); w -= 20 {
		h.tt.SetSize(w, 780)
		h.settle()
	}
	if err := h.tt.Click("更多操作"); err != nil {
		t.Fatalf("no overflow menu in a narrow window: %v", h.tt.Texts())
	}
	if menu := h.tt.Menu(); !slices.Contains(menu, "✕ 忽略") || slices.Contains(menu, "✓ 用这边") {
		t.Fatalf("overflow menu %v", menu)
	}
	h.tt.ChooseMenuItem("✕ 忽略")
	h.settle()
	h.shot("merge-view-narrow")
	h.ed.Run("undo")
	h.tt.SetSize(1200, 780)
	h.settle()
	if err := h.tt.Click("✓ 用这边"); err != nil { // ours' button comes first
		t.Fatal(err)
	}
	h.settle()
	if !strings.Contains(text(1), "你好") {
		t.Fatalf("result after taking ours:\n%s", text(1))
	}
	if got := h.f.frame.Status.Action.Label; got != "标记为已解决" {
		t.Fatalf("status action %q (%q)", got, h.f.frame.Status.Message)
	}
	h.shot("merge-view-handled")
	h.ed.Run("yan.merge.files")
	h.settle()
	if !h.tt.HasText("冲突文件：还有 1 个没解决") || !h.tt.HasText("● greeting.go") {
		t.Fatalf("list: %v", h.tt.Texts())
	}
	h.shot("merge-files")
}

// A bar too narrow for its buttons keeps the most needed in their order and puts the rest,
// in their order too, in "⋯"; a bar wide enough has no "⋯".
func TestDiffBlockFit(t *testing.T) {
	buttons := diffBlockButtonsOf(view.DiffBlockRevert)
	width := func(s string) float32 { return float32(len([]rune(s))) * 7 }
	labels := func(bs [][3]string) (out []string) {
		for _, b := range bs {
			out = append(out, b[2])
		}
		return out
	}
	if shown, more := diffBlockFit(buttons, 1000, width); len(shown) != 4 || more != nil {
		t.Fatalf("wide: %v %v", labels(shown), labels(more))
	}
	prev := 4
	for room := float32(240); room >= 0; room -= 10 {
		shown, more := diffBlockFit(buttons, room, width)
		if len(shown)+len(more) != 4 || len(shown) > prev {
			t.Fatalf("room %v: %v %v", room, labels(shown), labels(more))
		}
		prev = len(shown)
		need := width("⋯") + 16
		for _, b := range shown {
			need += width(b[0]) + 16
		}
		if len(more) > 0 && need > room && len(shown) > 0 {
			t.Fatalf("room %v: %v do not fit", room, labels(shown))
		}
	}
	shown, more := diffBlockFit(buttons, 150, width)
	if got := strings.Join(labels(shown), ","); got != "stage,revert" || strings.Join(labels(more), ",") != "edit,append" {
		t.Fatalf("room 150: %v %v", labels(shown), labels(more))
	}
}
