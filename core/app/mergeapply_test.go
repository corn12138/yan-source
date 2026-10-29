package app

import (
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

// With applying turned off, the result starts as the base; a side's change no conflict holds
// applies from its button, the rest from the command, each as a step undo takes back.
func TestMergeApplyChanges(t *testing.T) {
	e, root := mergeConflict(t, map[string][3]string{"f.txt": {
		"top\na\nm\nmid\nend\n",
		"TOP\na\nOURS\nmid\nend\n",
		"top\na\nSIDE\nmid\nEND\n",
	}})
	e.mergeManual = true
	if err := e.openMerge(filepath.Join(root, "f.txt")); err != nil {
		t.Fatal(err)
	}
	mv := e.cur().merge
	if got := resultText(mv); got != "top\na\nm\nmid\nend\n" {
		t.Fatalf("start %q", got)
	}
	var f view.Frame
	e.Frame(&f)
	applyRow := -1
	for _, r := range f.Columns[0].Pane.Rows { // ours: TOP is a change no conflict holds
		if r.Block == view.DiffBlockMergeApply {
			applyRow = int(r.Line)
		}
	}
	if applyRow < 0 {
		t.Fatal("no apply button on ours")
	}
	e.MergeBlockAt(0, applyRow, "apply")
	if got := resultText(mv); got != "TOP\na\nm\nmid\nend\n" {
		t.Fatalf("after applying ours' change: %q (%q)", got, e.status)
	}
	e.FocusColumn(1)
	e.Run("yan.merge.applyChanges")
	if got := resultText(mv); got != "TOP\na\nm\nmid\nEND\n" || mv.unresolved() != 1 {
		t.Fatalf("after applying the rest: %q", got)
	}
	e.Run("undo")
	if got := resultText(mv); got != "TOP\na\nm\nmid\nend\n" {
		t.Fatalf("undo: %q", got)
	}
	e.Run("yan.merge.toggleAutoApply")
	if e.mergeManual {
		t.Fatal("toggle")
	}
}
