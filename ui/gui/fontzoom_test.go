package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/view"
)

func TestFontZoomPlatformModifiers(t *testing.T) {
	for _, goos := range []string{"darwin", "windows", "linux"} {
		for _, mod := range []ui.Modifiers{0, ui.Super, ui.Ctrl, ui.Ctrl | ui.Alt, ui.Super | ui.Shift} {
			var z fontZoomGesture
			by, handled := z.step(ui.InputEvent{Mods: mod, DY: -120}, guiProfile(goos), time.Now())
			want := goos == "darwin" && mod == ui.Super || goos != "darwin" && mod == ui.Ctrl
			if handled != want || want && by != 1 || !want && by != 0 {
				t.Errorf("%s modifiers %v: step %d, handled %v", goos, mod, by, handled)
			}
		}
	}
}

func TestFontZoomTrackpadGestures(t *testing.T) {
	var z fontZoomGesture
	now := time.Now()
	for i, s := range []struct {
		dx, dy  float32
		mods    ui.Modifiers
		pause   time.Duration
		want    int
		handled bool
	}{
		{dy: -15, mods: ui.Super, handled: true},
		{dy: -15, mods: ui.Super, handled: true},
		{dy: -15, mods: ui.Super, want: 1, handled: true},
		{dy: 35, mods: ui.Super, handled: true},
		{dy: 5, mods: ui.Super, want: -1, handled: true},
		{dy: -30, mods: ui.Super, handled: true},
		{dy: -10, mods: ui.Super, pause: time.Second, handled: true},
		{dy: -10},
		{dy: -30, mods: ui.Super, handled: true},
		{dx: 80, dy: -10, mods: ui.Super, handled: true},
		{dy: -10, mods: ui.Super, handled: true},
		{dy: 80, mods: ui.Super, want: -2, handled: true},
		{mods: ui.Super, handled: true},
	} {
		now = now.Add(10*time.Millisecond + s.pause)
		by, handled := z.step(ui.InputEvent{DX: s.dx, DY: s.dy, Mods: s.mods, Precise: true}, guiProfile("darwin"), now)
		if by != s.want || handled != s.handled {
			t.Fatalf("event %d: step %d, handled %v; want %d, %v", i, by, handled, s.want, s.handled)
		}
	}
}

func TestFontZoomOverInactiveCodePane(t *testing.T) {
	h := newHarness(t, map[string]string{
		"long.txt": strings.Repeat("A long line of editable text.\n", 200),
		"pic.svg":  `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><rect width="100" height="100" fill="red"/></svg>`,
	}, Options{})
	h.open("long.txt")
	h.ed.Run("workbench.action.splitEditor")
	h.open("pic.svg")
	h.ed.Run("yan.svg.togglePreview")
	h.ed.Handle(app.ScrollInput{Column: 0, DY: 20})
	h.settle()
	before := h.f.frame.Columns[0].Pane
	_, _, base, _ := h.ed.Settings()
	lineH := h.f.font.lineH
	top, caret, scrollX := before.Rows[0].Line, before.Caret, before.ScrollX
	imageZoom := h.f.frame.Columns[1].Pane.Zoom
	if h.f.frame.Active != 1 || h.f.frame.Columns[1].Pane.Kind != view.PaneImage {
		t.Fatal("expected the image pane to have keyboard focus")
	}
	modifier := ui.Ctrl
	if h.f.Profile().MacKeys() {
		modifier = ui.Super
	}
	// 滚轮在左边的文字上，键盘仍属于右边的图片；不能把字号命令路由给图片。
	for _, ev := range []ui.InputEvent{
		{Mods: modifier, DY: -120},
		{Mods: modifier, DY: -20, Precise: true},
		{Mods: modifier, DY: -20, Precise: true},
	} {
		ev.Kind = ui.InputScroll
		if !h.f.paneInput(0, ev) {
			t.Fatal("font zoom was not consumed")
		}
	}
	h.settle()
	_, _, size, _ := h.ed.Settings()
	if size != base+2 || h.f.font.size != float32(base+2) || h.f.font.lineH <= lineH {
		t.Fatalf("font did not resize: settings %v, rendered %+v", size, h.f.font)
	}
	after := h.f.frame.Columns[0].Pane
	if after.Rows[0].Line != top || after.Caret != caret || after.ScrollX != scrollX {
		t.Fatal("zoom scrolled the text or moved the caret")
	}
	if h.f.frame.Active != 1 || h.f.frame.Columns[1].Pane.Zoom != imageZoom {
		t.Fatal("zoom changed focus or resized the image")
	}
	h.shot("font-zoom")
	h.f.paneInput(0, ui.InputEvent{Kind: ui.InputScroll, DY: h.f.font.lineH * 3})
	h.settle()
	_, _, size, _ = h.ed.Settings()
	if h.f.frame.Columns[0].Pane.Rows[0].Line != top+3 || size != base+2 {
		t.Fatal("releasing the modifier did not restore ordinary scrolling")
	}
}

func TestFontZoomKeepsReadingPositionAwayFromCaret(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		h := newHarness(t, map[string]string{"long.txt": strings.Repeat(strings.Repeat("long line ", 40)+"\n", 100)}, Options{})
		h.open("long.txt")
		if wrap {
			h.ed.Run("editor.action.toggleWordWrap")
			h.settle()
		}
		h.ed.Handle(app.ScrollInput{Column: 0, DY: 40, DX: 10})
		h.settle()
		before := h.f.frame.Columns[0].Pane
		top, x := before.Rows[0].Line, before.ScrollX
		if top == 0 || before.Caret.Visible {
			t.Fatal("expected to be reading away from the caret")
		}
		modifier := ui.Ctrl
		if h.f.Profile().MacKeys() {
			modifier = ui.Super
		}
		h.f.paneInput(0, ui.InputEvent{Kind: ui.InputScroll, Mods: modifier, DY: -120})
		h.settle()
		after := h.f.frame.Columns[0].Pane
		if after.Rows[0].Line != top || after.ScrollX != x || after.Caret.Visible {
			t.Fatalf("wrap %v: zoom pulled the reading position back to the caret", wrap)
		}
	}
}
