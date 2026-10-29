package gui

import (
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/style"
	"github.com/egoist/mygo/ui"
)

func TestThemePickerPreviewAndCancel(t *testing.T) {
	h := newHarness(t, nil, Options{Appearance: "light"})
	h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if err := h.tt.Click("颜色主题…"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.tt.Key(0, ui.KeyDown)
	h.settle()
	if h.f.opts.Theme != "matrix" || !h.f.pal.dark {
		t.Fatal("keyboard selection did not preview Matrix")
	}
	h.tt.Key(0, ui.KeyDown)
	h.settle()
	h.tt.Key(0, ui.KeyRight)
	h.settle()
	if h.f.opts.Theme != "claude" || h.f.opts.Appearance != "dark" {
		t.Fatalf("theme/appearance preview not applied: %q %q", h.f.opts.Theme, h.f.opts.Appearance)
	}
	h.shot("theme-picker-dark")
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	if h.f.opts.Theme != "" || h.f.opts.Appearance != "light" {
		t.Fatal("Escape did not restore appearance")
	}
	h.ed.Run("workbench.action.selectTheme")
	h.settle()
	h.tt.Type("Claude")
	h.settle()
	if h.f.opts.Theme != "claude" {
		t.Fatal("filtered theme not previewed")
	}
	h.tt.Key(0, ui.KeyLeft)
	h.tt.Type("X")
	h.settle()
	if h.f.frame.Overlay.Input != "ClaudXe" || h.f.opts.Appearance != "light" {
		t.Fatalf("query navigation changed appearance or caret: %+v", h.f.frame.Overlay)
	}
	h.tt.Key(0, ui.KeyBackspace)
	h.settle()
	if err := h.tt.Click("主题外观：浅色"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.ed.ChooseOverlay(0)
	h.settle()
	if h.ed.ThemePreviewing() || h.f.opts.Theme != "claude" {
		t.Fatal("theme not confirmed")
	}
	h.tt.SetSize(480, 600)
	h.ed.Run("workbench.action.selectTheme")
	h.settle()
	h.shot("theme-picker-narrow")
	if h.f.paletteFade.from != nil {
		t.Fatal("reduced motion retained an animation")
	}
}

func TestThemePickerCompositionAndOtherWindow(t *testing.T) {
	h := newHarness(t, nil, Options{Appearance: "light"})
	other := newHarness(t, nil, Options{Theme: "claude", Appearance: "dark"})
	h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	h.ed.Run("workbench.action.selectTheme")
	h.settle()
	h.tt.Compose("ma", 2)
	h.settle()
	if h.f.frame.Overlay.Input != "" || h.f.opts.Appearance != "light" {
		t.Fatalf("composition changed filter=%q appearance=%q input=%q", h.f.frame.Overlay.Input, h.f.opts.Appearance, h.f.ovInput)
	}
	h.tt.Type("Matrix")
	h.settle()
	if h.f.frame.Overlay.Input != "Matrix" || h.f.opts.Theme != "matrix" {
		t.Fatal("composition commit did not filter themes")
	}
	other.settle()
	if other.f.opts.Theme != "claude" || !other.f.pal.dark {
		t.Fatal("preview changed another window")
	}
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	if h.f.opts.Theme != "" || h.f.pal.dark {
		t.Fatal("cancel did not restore the first window")
	}
}

func TestPaletteFadeInterruptAndReduceMotion(t *testing.T) {
	a := newPalette(style.ThemeFor("", false))
	b := newPalette(style.ThemeFor("claude", true))
	c := newPalette(style.ThemeFor("matrix", true))
	start := time.Unix(100, 0)
	var fade paletteFade
	if p, moving := fade.step(start, a, false); p != a || moving {
		t.Fatal("initial palette animated")
	}
	fade.step(start, b, false)
	mid, moving := fade.step(start.Add(paletteFadeDuration/2), b, false)
	if !moving || mid.editor == a.editor || mid.editor == b.editor {
		t.Fatal("no intermediate colors")
	}
	interrupted, _ := fade.step(start.Add(paletteFadeDuration/2), c, false)
	if interrupted.editor != mid.editor {
		t.Fatal("interrupted transition jumped")
	}
	if p, moving := fade.step(start.Add(paletteFadeDuration), c, true); p != c || moving || fade.from != nil {
		t.Fatal("reduce motion did not finish immediately")
	}
}

func TestThemePickerFollowsDesktopAppearance(t *testing.T) {
	h := newHarness(t, nil, Options{Appearance: "light"})
	other := newHarness(t, nil, Options{})
	g := &group{projects: []*Frontend{h.f, other.f}}
	h.tt = ui.NewTester(g.view, 1200, 780)
	h.tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	h.tt.SetDark(true)
	h.ed.Run("workbench.action.selectTheme")
	h.settle()
	if h.f.pal.dark {
		t.Fatal("explicit light preference ignored")
	}
	h.tt.Key(0, ui.KeyLeft)
	h.settle()
	if h.f.opts.Appearance != "" || !h.f.pal.dark {
		t.Fatal("system preview inherited the old explicit appearance")
	}
	h.tt.SetDark(false)
	h.settle()
	if h.f.pal.dark {
		t.Fatal("system preview did not follow desktop change")
	}
	h.tt.Key(0, ui.KeyDown)
	h.settle()
	if !h.f.pal.dark {
		t.Fatal("Matrix must remain dark")
	}
	h.tt.Key(0, ui.KeyDown)
	h.settle()
	if h.f.pal.dark {
		t.Fatal("leaving Matrix polluted system appearance")
	}
}
