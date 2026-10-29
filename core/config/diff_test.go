package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// [diff] whitespace takes the four filters, the menus' choice overrides it, and a wrong value
// warns and keeps every character compared.
func TestDiffWhitespaceSetting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(UserPath()), 0755); err != nil {
		t.Fatal(err)
	}
	for v, want := range map[string]string{"exact": "", "eol": "eol", "all": "all", "blank": "blank"} {
		os.WriteFile(UserPath(), []byte("[diff]\nwhitespace = \""+v+"\"\n"), 0644)
		cfg, warns := Load("")
		if len(warns) > 0 || cfg.DiffWhitespace != want {
			t.Fatalf("%s: %q, %v", v, cfg.DiffWhitespace, warns)
		}
	}
	os.WriteFile(UserPath(), []byte("[diff]\nwhitespace = \"tabs\"\n"), 0644)
	cfg, warns := Load("")
	if len(warns) != 1 || !strings.Contains(warns[0], "diff.whitespace") || cfg.DiffWhitespace != "" {
		t.Fatalf("bad value: %q, %v", cfg.DiffWhitespace, warns)
	}
	chosen := "all"
	(&State{DiffWhitespace: &chosen}).Apply(cfg)
	if cfg.DiffWhitespace != "all" {
		t.Fatalf("state: %q", cfg.DiffWhitespace)
	}
}

// [diff] highlight, context and layout, and the menus' choices over them.
func TestDiffViewKeys(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	os.MkdirAll(filepath.Dir(UserPath()), 0755)
	if c := Default(); c.DiffContext != 3 || c.DiffHighlight != "" || c.DiffLayout != "" {
		t.Fatalf("defaults %+v", c)
	}
	os.WriteFile(UserPath(), []byte("[diff]\nhighlight = \"char\"\ncontext = 0\nlayout = \"inline\"\n"), 0644)
	cfg, warns := Load("")
	if len(warns) > 0 || cfg.DiffHighlight != "char" || cfg.DiffContext != 0 || cfg.DiffLayout != "inline" {
		t.Fatalf("%+v %v", cfg, warns)
	}
	os.WriteFile(UserPath(), []byte("[diff]\nhighlight = \"words\"\ncontext = 11\nlayout = \"wide\"\n"), 0644)
	cfg, warns = Load("")
	if len(warns) != 3 || cfg.DiffHighlight != "" || cfg.DiffContext != 3 || cfg.DiffLayout != "" {
		t.Fatalf("bad values: %+v %v", cfg, warns)
	}
	h, n, l := "line", 7, "split"
	(&State{DiffHighlight: &h, DiffContext: &n, DiffLayout: &l}).Apply(cfg)
	if cfg.DiffHighlight != "line" || cfg.DiffContext != 7 || cfg.DiffLayout != "split" {
		t.Fatalf("state: %+v", cfg)
	}
}
