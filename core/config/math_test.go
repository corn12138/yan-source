package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewMathLayers(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if !Default().PreviewMath {
		t.Fatal("math disabled by default")
	}
	if err := os.MkdirAll(filepath.Dir(UserPath()), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(UserPath(), []byte("[ui]\npreview_math = false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, warns := Load("")
	if len(warns) > 0 || cfg.PreviewMath {
		t.Fatalf("user config: %v, math %v", warns, cfg.PreviewMath)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".yan.toml"), []byte("[ui]\npreview_math = true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, warns = Load(root)
	if len(warns) > 0 || !cfg.PreviewMath {
		t.Fatalf("project config: %v, math %v", warns, cfg.PreviewMath)
	}
}
