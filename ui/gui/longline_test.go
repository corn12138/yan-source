package gui

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/text"
)

func TestLongLineDoesNotReachGUI(t *testing.T) {
	h := newHarness(t, map[string]string{"keep.txt": "original\n", "bundle.map": strings.Repeat("x", 26<<20)}, Options{})
	h.open("keep.txt")
	h.ed.Handle(app.TextInput{Text: "mine "})
	err := h.ed.OpenFile(filepath.Join(h.root, "bundle.map"))
	if !errors.Is(err, text.ErrLineTooLong) {
		t.Fatalf("open: %v", err)
	}
	h.ed.Notify(err.Error())
	h.settle()
	col := h.f.frame.Columns[0]
	if len(col.Tabs) != 1 || !col.Tabs[0].Dirty || len(col.Pane.Rows) == 0 || string(col.Pane.Rows[0].Text) != "mine original" {
		t.Fatal("rejected input reached layout or replaced dirty tab")
	}
	if !h.tt.HasText(err.Error()) {
		t.Fatal("refusal is not visible")
	}
	h.shot("long-line-refused")
	h.ed.Run("undo")
	h.settle()
	if h.f.frame.Columns[0].Tabs[0].Dirty {
		t.Fatal("undo no longer restores saved document")
	}
}
