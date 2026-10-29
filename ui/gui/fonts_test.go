package gui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The View menu's font picker shows each family in itself, and a typed family list applies to
// the code at once.
func TestChooseFontInTheWindow(t *testing.T) {
	h := newHarness(t, sample, Options{})
	h.open("main.go")
	h.ed.Run("yan.font.family")
	h.settle()
	if !h.f.frame.Overlay.Visible || !h.tt.HasText("系统等宽字体") || !h.tt.HasText(fontSample) {
		t.Fatal("the font picker did not open with samples")
	}
	h.shot("font-picker")
	if err := h.tt.Click("输入字体名…"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.ed.SetOverlayInput("Courier New, monospace")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if h.f.opts.FontFamily != "Courier New, monospace" || h.f.frame.Overlay.Visible {
		t.Fatalf("font %q", h.f.opts.FontFamily)
	}
	h.shot("font-chosen")
}
