package app

import (
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func TestThemePreviewIsTransactional(t *testing.T) {
	e, _ := newFolderEditor(t, nil)
	other, _ := newFolderEditor(t, nil)
	path := filepath.Join(t.TempDir(), "state.json")
	state := config.LoadState(path)
	e.SetState(state)
	other.SetState(state)
	e.Run("yan.theme.claude")
	e.Run("yan.appearance.light")
	e.Run("workbench.action.selectTheme")
	if e.ov.sel != 2 || e.ov.kind != view.OverlayTheme {
		t.Fatal("picker did not select the current theme")
	}
	e.Handle(key(keymap.KeyUp, 0))
	e.Handle(key(keymap.KeyRight, 0))
	if theme, appearance, _, _ := e.Settings(); theme != "matrix" || appearance != "light" {
		t.Fatalf("preview %s %s", theme, appearance)
	}
	if e.cfg.Theme != "claude" || e.cfg.Appearance != "light" {
		t.Fatal("preview changed config")
	}
	if other.ThemePreviewing() {
		t.Fatal("preview leaked to another window")
	}
	e.Run("workbench.action.zoomIn")
	saved := config.LoadState(path)
	if *saved.Theme != "claude" || *saved.Appearance != "light" {
		t.Fatal("saving an unrelated setting committed the preview")
	}
	e.Handle(key(keymap.KeyEsc, 0))
	if theme, appearance, size, _ := e.Settings(); theme != "claude" || appearance != "light" || size != config.Default().FontSize+1 {
		t.Fatalf("rollback %s %s %v", theme, appearance, size)
	}
	e.Run("workbench.action.selectTheme")
	e.PreviewThemeAppearance("dark")
	e.SetOverlayInput("Matrix")
	e.Handle(key(keymap.KeyEnter, 0))
	saved = config.LoadState(path)
	if e.ThemePreviewing() || *saved.Theme != "matrix" || *saved.Appearance != "dark" {
		t.Fatal("confirmation not persisted")
	}
}

func TestThemePickerKeepsTextNavigationAndConfigurationReloads(t *testing.T) {
	e, _ := newFolderEditor(t, nil)
	e.Run("workbench.action.selectTheme")
	e.SetOverlayInput("cld")
	if len(e.ov.items) != 1 || e.ov.items[0].theme != "claude" {
		t.Fatal("fuzzy theme search did not match Claude")
	}
	if e.handleOverlayKey(keymap.Chord{Key: keymap.KeyRight}) {
		t.Fatal("theme picker consumed query caret movement")
	}
	e.SetOverlayInput("")
	if e.handleOverlayKey(keymap.Chord{Key: keymap.KeyRight, Mods: keymap.ModShift}) {
		t.Fatal("theme picker consumed text selection")
	}
	// A canceled local preview must expose the latest config, not restore a stale copy.
	e.cfg.Theme, e.cfg.Appearance = "matrix", "dark"
	e.settingsGen++
	e.CloseOverlay()
	if theme, appearance, _, _ := e.Settings(); theme != "matrix" || appearance != "dark" {
		t.Fatal("preview cancellation lost externally changed settings")
	}
}

func TestThemePreviewCancellationAndEmptyResults(t *testing.T) {
	e, _ := newFolderEditor(t, nil)
	e.Run("workbench.action.selectTheme")
	e.SelectOverlay(2)
	e.SetOverlayInput("no-such-theme")
	e.Handle(key(keymap.KeyEnter, 0))
	if !e.ThemePreviewing() || len(e.ov.items) != 0 {
		t.Fatal("empty search confirmed a theme")
	}
	e.Run("workbench.action.showCommands")
	if theme, _, _, _ := e.Settings(); theme != "" {
		t.Fatal("replacement overlay retained preview")
	}
	e.Run("workbench.action.selectTheme")
	e.SelectOverlay(1)
	e.CloseOverlay()
	if theme, _, _, _ := e.Settings(); theme != "" {
		t.Fatal("outside dismissal retained preview")
	}
	e.Run("workbench.action.selectTheme")
	e.SelectOverlay(2)
	e.Run("yan.appearance.dark")
	if theme, appearance, _, _ := e.Settings(); e.ThemePreviewing() || theme != "" || appearance != "dark" {
		t.Fatal("explicit appearance command left preview active")
	}
}
