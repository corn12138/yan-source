package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/view"
)

// The font picker offers the code fonts installed here, marks Chinese monospaced ones, and the
// choice, the size and the line height apply at once and are kept for the next start.
func TestChooseFontFromTheWindow(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"JetBrainsMono-Regular.ttf", "Sarasa-Mono-SC-Regular.ttf", "Arial.ttf"} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o644)
	}
	old := fontDirs
	fontDirs = func() []string { return []string{dir} }
	t.Cleanup(func() { fontDirs = old })

	e, _ := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	path := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(path))
	e.Run("yan.font.family")
	if e.ov != nil {
		t.Fatal("the terminal version opened the font picker")
	}
	e.profile.GUI = true
	e.Run("yan.font.family")
	var f view.Frame
	e.Frame(&f)
	labels := map[string]view.OverlayItem{}
	for _, it := range f.Overlay.Items {
		labels[it.Label] = it
	}
	if len(f.Overlay.Items) != 4 || labels["JetBrains Mono"].Font != "JetBrains Mono" || labels["Sarasa Mono SC"].Detail == "" || labels["系统等宽字体"].Hint != "当前" {
		t.Fatalf("items: %+v", f.Overlay.Items)
	}
	gen := e.settingsGen
	for i, it := range f.Overlay.Items {
		if it.Label == "Sarasa Mono SC" {
			e.ChooseOverlay(i)
		}
	}
	if family, _ := e.Font(); family != "Sarasa Mono SC" || e.settingsGen == gen {
		t.Fatalf("family %q", family)
	}
	e.Run("yan.font.size")
	e.Frame(&f)
	for i, it := range f.Overlay.Items {
		if it.Label == "16 号" {
			e.ChooseOverlay(i)
		}
	}
	e.Run("yan.font.lineHeight")
	e.Frame(&f)
	for i, it := range f.Overlay.Items {
		if it.Label == "1.8 倍" {
			e.ChooseOverlay(i)
		}
	}
	c := config.Default()
	config.LoadState(path).Apply(c)
	if c.FontFamily != "Sarasa Mono SC" || c.FontSize != 16 || c.LineHeight != 1.8 {
		t.Fatalf("kept %q %g %g", c.FontFamily, c.FontSize, c.LineHeight)
	}
}
