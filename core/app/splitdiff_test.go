package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// splitEditor opens, in a window's editor, the split diff of a file that had three lines
// inserted after line 10 and line 30 changed, of 40.
func splitEditor(t *testing.T) (*Editor, *tab, string) {
	t.Helper()
	var lines []string
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	root := gitRepo(t, map[string]string{"a.txt": strings.Join(lines, "\n") + "\n"})
	next := append(append(append([]string{}, lines[:10]...), "new a", "new b", "new c"), lines[10:]...)
	next[32] = "line thirty"
	p := filepath.Join(root, "a.txt")
	os.WriteFile(p, []byte(strings.Join(next, "\n")+"\n"), 0o644)
	e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, keymap.Profile{GUI: true, Mods: []keymap.Mod{keymap.ModSuper}})
	if err := e.OpenFolder(root); err != nil {
		t.Fatal(err)
	}
	e.SetPaneSize(0, view.Size{W: 220, H: 60})
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	e.cfg.DiffLayout, e.cfg.DiffContext = "split", 40
	if err := e.openChange(p, false); err != nil {
		t.Fatal(err)
	}
	return e, e.cur(), p
}

func splitOf(e *Editor) *view.SplitDiff {
	var f view.Frame
	e.Frame(&f)
	return f.Columns[0].Pane.Split
}

func TestSplitDiffPanes(t *testing.T) {
	e, tb, _ := splitEditor(t)
	sd := splitOf(e)
	if sd == nil || !sd.Aligned || !sd.Synced {
		t.Fatalf("split %+v", sd)
	}
	if len(sd.Left.Rows) != len(sd.Right.Rows) {
		t.Fatalf("aligned sides differ: %d, %d", len(sd.Left.Rows), len(sd.Right.Rows))
	}
	fillers := 0
	for _, r := range sd.Left.Rows {
		if r.Filler {
			fillers++
		}
	}
	if fillers != 3 || len(sd.Bands) != 2 {
		t.Fatalf("fillers %d, bands %+v", fillers, sd.Bands)
	}
	if b := sd.Bands[0]; b.LeftFrom != b.LeftTo || b.RightTo-b.RightFrom != 3 || !b.Arrow {
		t.Fatalf("insertion band %+v", b)
	}
	// Unaligned: each side's lines close up.
	e.Run("yan.diff.toggleAlignedRows")
	sd = splitOf(e)
	for _, r := range sd.Left.Rows {
		if r.Filler {
			t.Fatal("a filler in an unaligned side")
		}
	}
	if len(sd.Right.Rows)-len(sd.Left.Rows) != 3 {
		t.Fatalf("streams: left %d, right %d", len(sd.Left.Rows), len(sd.Right.Rows))
	}
	e.Run("yan.diff.toggleAlignedRows")
	// Going to a change asks the window to glide there; the next frame does not.
	e.Run("workbench.action.compareEditor.nextChange")
	if !splitOf(e).Glide || splitOf(e).Glide {
		t.Fatal("glide is for the frame after going to a change only")
	}
	// Each side on its own: the right scrolls alone; together again, the left lines up with it.
	e.Run("yan.diff.toggleSyncScroll")
	left := tb.scrollY
	e.SplitScroll(0, true, 5)
	sd = splitOf(e)
	if sd.Right.Lines[sd.Right.Over] != int32(left+5) || sd.Left.Lines[sd.Left.Over] != int32(left) || tb.scrollY != left {
		t.Fatalf("right %d, left %d (was %d)", sd.Right.Lines[sd.Right.Over], sd.Left.Lines[sd.Left.Over], left)
	}
	e.Run("yan.diff.toggleSyncScroll")
	if tb.scrollY != left+5 {
		t.Fatalf("together again at %d, want %d", tb.scrollY, left+5)
	}
}

// The arrow of a change reverts it, and the lines written glow.
func TestSplitArrowRevertsAndGlows(t *testing.T) {
	e, tb, p := splitEditor(t)
	sd := splitOf(e)
	b := sd.Bands[1] // line 30 changed
	e.SplitArrow(0, int(b.Line), false)
	if d := e.docOf(p); d == nil || !strings.Contains(docText(d), "line 30\n") {
		t.Fatalf("not reverted (%q)", e.status)
	}
	var f view.Frame
	e.FocusColumn(0)
	e.Frame(&f)
	pane := f.Columns[0].Pane
	if pane.GlowGen == 0 {
		t.Fatal("no glow")
	}
	glowing := 0
	for _, r := range pane.Split.Right.Rows {
		if r.Glow {
			glowing++
		}
	}
	if glowing != 1 {
		t.Fatalf("%d rows glow", glowing)
	}
	_ = tb
}

// Copying with nothing selected in a split diff copies the caret's line of the side last
// clicked, not the two sides' row.
func TestSplitCopyOneSide(t *testing.T) {
	e, _, _ := splitEditor(t)
	sd := splitOf(e)
	b := sd.Bands[1] // line 30 changed: "line 30" ↔ "line thirty"
	row := -1
	for i, l := range sd.Right.Lines {
		if i >= b.RightFrom && i < b.RightTo {
			row = int(l)
			break
		}
	}
	e.SplitClick(0, row, true, false)
	e.Run("editor.action.clipboardCopyAction")
	if got := e.host.(*fakeHost).clip; got != "line thirty\n" {
		t.Fatalf("right: %q (%q)", got, e.status)
	}
	e.SplitClick(0, row, false, false)
	e.Run("editor.action.clipboardCopyAction")
	if got := e.host.(*fakeHost).clip; got != "line 30\n" {
		t.Fatalf("left: %q", got)
	}
}
