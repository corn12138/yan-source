package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/view"
)

// The diffs' highlight shows changed words, characters or only lines; the context keeps 0 to
// 10 unchanged lines around a change; the layout chosen is kept in the settings.
func TestDiffViewSettings(t *testing.T) {
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	root := gitRepo(t, map[string]string{"a.txt": strings.Join(lines, "\n") + "\n"})
	lines[14] = "line fifteen"
	lines[19] = "line 2O"
	os.WriteFile(filepath.Join(root, "a.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	e.cfg.DiffLayout = "inline"
	if err := e.openChange(filepath.Join(root, "a.txt"), false); err != nil {
		t.Fatal(err)
	}
	words := func() []string { // the changed parts the rows mark
		var f view.Frame
		e.Frame(&f)
		var out []string
		for _, r := range f.Columns[0].Pane.Rows {
			for _, sp := range r.Spans {
				if sp.Style == style.DiffAddedWord || sp.Style == style.DiffRemovedWord {
					out = append(out, string(r.Text[sp.Start:sp.End]))
				}
			}
		}
		return out
	}
	if got := strings.Join(words(), "|"); got != "15|fifteen|20|2O" {
		t.Fatalf("by word: %q", got)
	}
	e.Run("yan.diff.highlight")
	chooseItem(t, e, "按字")
	if got := strings.Join(words(), "|"); got != "15|fifteen|0|O" {
		t.Fatalf("by char: %q", got)
	}
	if e.cfg.DiffHighlight != "char" {
		t.Fatalf("highlight %q", e.cfg.DiffHighlight)
	}
	e.Run("yan.diff.highlight")
	chooseItem(t, e, "只标出")
	if got := words(); len(got) != 0 {
		t.Fatalf("by line: %q", got)
	}
	if txt := content(e); !strings.Contains(txt, "⋯ 11 行没改") || !strings.Contains(txt, "line 12\nline 13\nline 14\n@@") || !strings.HasSuffix(txt, "line 23\n⋯ 8 行没改") {
		t.Fatalf("context 3:\n%s", txt)
	}
	e.Run("yan.diff.context")
	chooseItem(t, e, "改动前后留 0 行")
	if txt := content(e); !strings.HasPrefix(txt, "⋯ 14 行没改\n@@ -15,1 +15,1 @@\nline 15\nline fifteen\n⋯ 4 行没改\n@@ -20,1 +20,1 @@\nline 20\nline 2O\n⋯ 11 行没改") {
		t.Fatalf("context 0:\n%s", txt)
	}
	e.Run("yan.diff.context")
	chooseItem(t, e, "改动前后留 10 行")
	if txt := content(e); !strings.Contains(txt, "⋯ 4 行没改\nline 5\n") {
		t.Fatalf("context 10:\n%s", txt)
	}
	e.Run("toggle.diff.renderSideBySide") // inline → auto
	if e.cfg.DiffLayout != "" {
		t.Fatalf("layout %q", e.cfg.DiffLayout)
	}
}
