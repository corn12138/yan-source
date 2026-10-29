package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// Toggling a Markdown file between its text and its preview in place keeps the place read:
// the text's top line becomes the preview's, and the preview's the text's.
func TestMarkdownToggleKeepsThePlace(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&b, "Paragraph %d.\n\n", i)
	}
	p := filepath.Join(t.TempDir(), "doc.md")
	os.WriteFile(p, []byte(b.String()), 0o644)
	e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, keymap.Profile{GUI: true, Mods: []keymap.Mod{keymap.ModSuper}})
	e.SetPaneSize(0, view.Size{W: 80, H: 20})
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	tb := e.cur()
	tb.scrollY = 120
	if err := e.togglePreview(); err != nil {
		t.Fatal(err)
	}
	if !tb.markdown || tb.previewLine != 120 || tb.previewGen != 1 {
		t.Fatalf("preview at %d", tb.previewLine)
	}
	e.SetPreviewTop(0, p, 250) // read further in the preview
	e.togglePreview()
	if tb.markdown || tb.scrollY != 250 {
		t.Fatalf("text at %d", tb.scrollY)
	}
}
