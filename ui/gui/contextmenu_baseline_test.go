package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/view"
)

// The context menus of a09d29b, where the Quiet UI redesign started: an entry may move within its
// menu but stays there. An intentional, documented change regenerates the file with
// UPDATE_CONTEXT_MENU_BASELINE=1.
const contextMenuBaseline = "testdata/context-menu-baseline.tsv"

// contextMenus right-clicks each kind of thing that has a context menu and records its entries.
func contextMenus(t *testing.T) map[string][]string {
	t.Helper()
	// The baseline was recorded on macOS; other systems name the file manager and trash their own way.
	old := hostOS
	hostOS = "darwin"
	t.Cleanup(func() { hostOS = old })
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
	os.WriteFile(filepath.Join(root, "main.go"), []byte(sample["main.go"]+"// changed\n"), 0o644)
	h := newHarnessAt(t, root, Options{})
	out := map[string][]string{}
	menu := func(name, target string) {
		t.Helper()
		h.settle()
		if err := h.tt.RightClick(target); err != nil {
			t.Fatalf("%s: %v (texts %q)", name, err, h.tt.Texts())
		}
		h.settle()
		items := slices.DeleteFunc(h.tt.Menu(), func(s string) bool { return s == "-" })
		if len(items) == 0 {
			t.Fatalf("%s: no context menu", name)
		}
		out[name] = items
		h.tt.CloseMenu()
		h.settle()
	}
	h.ed.Run("workbench.view.explorer")
	menu("explorer file", "README.md")
	menu("explorer folder", "core")
	h.ed.Run("workbench.view.scm")
	for i := 0; i < 50 && !slices.ContainsFunc(h.f.frame.Sidebar.SCM.Groups, func(g view.SCMGroup) bool { return len(g.Rows) > 0 }); i++ {
		time.Sleep(50 * time.Millisecond)
		h.settle()
	}
	menu("source control commit", "第一次提交")
	h.ed.Run("workbench.action.toggleSidebarVisibility")
	h.open("README.md")
	menu("editor tab", "关闭 README.md")
	h.ed.Run("workbench.action.createTerminalEditor")
	menu("terminal tab", "关闭 终端")
	return out
}

func TestContextMenuBaselineKept(t *testing.T) {
	now := contextMenus(t)
	if os.Getenv("UPDATE_CONTEXT_MENU_BASELINE") != "" {
		var rows []string
		for name, items := range now {
			for _, it := range items {
				rows = append(rows, name+"\t"+it)
			}
		}
		slices.Sort(rows)
		os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(contextMenuBaseline, []byte("# context\tentry\n"+strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(contextMenuBaseline)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		name, entry, _ := strings.Cut(line, "\t")
		if !slices.Contains(now[name], entry) {
			t.Errorf("%s context menu no longer offers %q (now %q)", name, entry, now[name])
		}
	}
}
