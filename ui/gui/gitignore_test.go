package gui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/config"
)

// The comment key (Cmd+/ on macOS, Ctrl+/ elsewhere) comments a line of .gitignore in the window.
func TestCommentKeyInGitignore(t *testing.T) {
	h := newHarness(t, map[string]string{".gitignore": "H5Build\n"}, Options{})
	h.open(".gitignore")
	mod := ui.Ctrl
	if h.f.Profile().MacKeys() {
		mod = ui.Cmd
	}
	h.tt.Key(mod, ui.KeySlash)
	h.settle()
	if !h.f.frame.Columns[0].Tabs[0].Dirty || h.f.frame.Columns[0].Pane.LineCount != 2 {
		t.Fatalf("the line was not commented; status %q", h.f.frame.Status.Message)
	}
	if got := string(h.f.frame.Columns[0].Pane.Rows[0].Text); got != "# H5Build" {
		t.Fatalf("first row %q", got)
	}
}

// A font list and line height saved in config.toml, by an import or by hand, apply to the open
// window without a restart.
func TestFontSettingsApplyLive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	os.MkdirAll(filepath.Dir(config.UserPath()), 0o755)
	h := newHarness(t, sample, Options{})
	h.open("main.go")
	os.WriteFile(config.UserPath(), []byte("[ui]\nfont_family = \"Menlo, monospace\"\nline_height = 2\n"), 0o644)
	end := time.Now().Add(5 * time.Second)
	for (h.f.opts.FontFamily != "Menlo, monospace" || h.f.opts.LineHeight != 2) && time.Now().Before(end) {
		h.settle()
	}
	if h.f.opts.FontFamily != "Menlo, monospace" || h.f.opts.LineHeight != 2 || h.f.font.family != "Menlo, monospace" {
		t.Fatalf("font %q / %v, painted with %q", h.f.opts.FontFamily, h.f.opts.LineHeight, h.f.font.family)
	}
}
