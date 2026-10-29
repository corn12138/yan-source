package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogoFiles(t *testing.T) {
	root := t.TempDir()
	write := func(rel, s string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(s), 0o644)
	}
	if got := LogoFiles(root); len(got) != 0 {
		t.Fatalf("empty project: %v", got)
	}
	// A declared icon only: an index.html naming a file in public/ from the site's root.
	write("index.html", `<html><head><link rel="shortcut icon" href="/brand/mark.png?v=2"></head></html>`)
	write("public/brand/mark.png", "png")
	if got := LogoFiles(root); len(got) != 1 || got[0] != filepath.Join(root, "public", "brand", "mark.png") {
		t.Fatalf("declared: %v", got)
	}
	// A conventional candidate comes before a declared one.
	write("public/favicon.svg", "<svg/>")
	if got := LogoFiles(root); got[0] != filepath.Join(root, "public", "favicon.svg") {
		t.Fatalf("candidate first: %v", got)
	}
}

func TestExtractIconHref(t *testing.T) {
	for src, want := range map[string]string{
		`<link href="favicon.ico" rel="icon">`:                                 "favicon.ico",
		`<link rel="apple-touch-icon" href="/apple.png">`:                      "/apple.png",
		`<link rel="icon" href="data:image/png;base64,AA==">`:                  "",
		`<link rel="icon" href="https://cdn.example.com/x.png">`:               "",
		`<link rel="icon" href="../outside.png">`:                              "",
		`export const metadata = { icons: { icon: "/icon.svg" } }`:             "/icon.svg",
		`icons: [{ rel: "icon", url: "/brand.png" }]`:                          "/brand.png",
		`<link rel="stylesheet" href="a.css"><link rel="icon" href="./f.png">`: "f.png",
	} {
		if got := extractIconHref(src); got != want {
			t.Errorf("%s: %q, want %q", src, got, want)
		}
	}
}
