package gui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/keymap"
)

// Holding the platform's link modifier (Cmd on macOS, Ctrl on Windows and Linux) underlines the
// name under the pointer; clicking it goes to its definition, here or in another file, and the
// back key returns.
func TestGoToDefinitionInTheWindow(t *testing.T) {
	h := newHarness(t, map[string]string{
		"main.go":   "package main\n\nfunc helper() {}\n\nfunc main() {\n\thelper()\n\tShared()\n}\n",
		"shared.go": "package main\n\n// Shared is in another file.\nfunc Shared() {}\n",
	}, Options{Appearance: "dark"})
	h.open("main.go")
	mod, other := ui.Ctrl, ui.Super
	if slices.Contains(h.f.Profile().Mods, keymap.ModSuper) {
		mod, other = ui.Super, ui.Ctrl
	}
	pane := h.f.cols[0].size
	pf := &h.f.frame.Columns[0].Pane
	// at is the middle of the cell of column col (from 0; a tab takes TabWidth) on line row.
	at := func(row, col int) (float32, float32) {
		cw, lh := h.f.font.charW, h.f.font.lineH
		return pane.X + float32(int(pf.Gutter)+col)*cw + cw/2, pane.Y + codePadY + float32(row)*lh + lh/2
	}
	// line is the caret's line (from 0) and the file of the focused column.
	line := func() (int, string) {
		c := h.f.frame.Columns[h.f.frame.Active]
		return int(c.Pane.Current), c.Tabs[c.Active].Name
	}

	tab := h.f.opts.TabWidth
	x, y := at(5, tab+2) // the "l" of helper() on line 6
	h.tt.Move(x, y)
	h.settle()
	if h.f.cols[0].link.ok {
		t.Fatal("underlined without the modifier")
	}
	// The tester's moves carry no modifiers, so the modifier goes down after the pointer is there.
	h.tt.Move(x+1, y)
	h.tt.HoldModifiers(mod)
	h.settle()
	if l := h.f.cols[0].link; !l.ok || l.row != 5 || int(l.from) != int(pf.Gutter)+tab || int(l.to) != int(pf.Gutter)+tab+6 {
		t.Fatalf("link %+v (gutter %d)", l, pf.Gutter)
	}
	if got := h.tt.Cursor(); got != ui.CursorPointer {
		t.Errorf("cursor %v over a link", got)
	}
	h.shot("go-to-definition-link")
	h.tt.HoldModifiers(0)
	h.settle()
	if h.f.cols[0].link.ok {
		t.Fatal("still underlined after letting go of the modifier")
	}

	h.tt.ClickAtWith(other, x, y)
	h.settle()
	if l, _ := line(); l != 5 {
		t.Fatalf("a click with the other modifier went to line %d", l+1)
	}
	h.tt.ClickAtWith(mod, x, y)
	h.settle()
	if l, name := line(); l != 2 || name != "main.go" {
		t.Fatalf("link click went to %s line %d", name, l+1)
	}

	x, y = at(6, tab+1) // Shared()
	h.tt.ClickAtWith(mod, x, y)
	for end := time.Now().Add(10 * time.Second); time.Now().Before(end); h.settle() {
		if _, name := line(); name == "shared.go" {
			break
		}
	}
	if l, name := line(); l != 3 || name != "shared.go" {
		t.Fatalf("link click went to %s line %d", name, l+1)
	}

	// Back: Ctrl+- on macOS, Alt+Left on Windows and Linux, as VS Code.
	if mod == ui.Super {
		h.tt.Key(ui.Ctrl, ui.KeyMinus)
	} else {
		h.tt.Key(ui.Alt, ui.KeyLeft)
	}
	h.settle()
	if l, name := line(); l != 6 || name != "main.go" {
		t.Fatalf("back went to %s line %d", name, l+1)
	}
	if got := h.ed.KeyHint("editor.action.revealDefinition"); !strings.Contains(got, "F12") {
		t.Errorf("hint %q", got)
	}
}
