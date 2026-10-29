package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestThemeEffectsLayers(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if !Default().ThemeEffects {
		t.Fatal("effects disabled by default")
	}
	if err := os.MkdirAll(filepath.Dir(UserPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(UserPath(), []byte("[ui]\ntheme_effects = false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, warns := Load("")
	if len(warns) > 0 || cfg.ThemeEffects {
		t.Fatalf("user config: %v, effects %v", warns, cfg.ThemeEffects)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".yan.toml"), []byte("[ui]\ntheme_effects = true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, warns = Load(root)
	if len(warns) > 0 || !cfg.ThemeEffects {
		t.Fatalf("project config: %v, effects %v", warns, cfg.ThemeEffects)
	}
	off := false
	st := LoadState(filepath.Join(t.TempDir(), "state.json"))
	st.ThemeEffects = &off
	if err := st.Save(); err != nil {
		t.Fatal(err)
	}
	LoadState(st.path).Apply(cfg)
	if cfg.ThemeEffects {
		t.Fatal("saved menu choice lost to config")
	}
}
