package gui

import (
	"bytes"
	"image"
	"image/draw"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestCodeFontCacheRecovery(t *testing.T) {
	h := newHarness(t, map[string]string{"text.txt": "English and 中文 日本語 한국어 remain visible after fonts are recycled.\n"}, Options{})
	h.open("text.txt")
	r := h.f.cols[0].size
	// Exclude the blinking caret at the start of the line and the scrollbar.
	area := image.Rect(int(r.X)+120, int(r.Y), int(r.X+r.W)-30, int(r.Y)+60)
	pixels := func() []byte {
		im := image.NewRGBA(image.Rect(0, 0, area.Dx(), area.Dy()))
		draw.Draw(im, im.Bounds(), h.tt.Image(), area.Min, draw.Src)
		return im.Pix
	}
	want := pixels()
	for cycle := range 3 {
		for i := range 640 {
			ui.ShapeText("cache 中文", ui.Font{Size: 10 + float32(i)/8}, 0)
		}
		h.tt.Frame()
		h.tt.Frame()
		if !bytes.Equal(want, pixels()) {
			t.Fatalf("cycle %d: cached editor glyphs disappeared after font recycling", cycle)
		}
	}
}

func TestMermaidFontCacheRecovery(t *testing.T) {
	d := renderMermaid("flowchart LR\nA[English 中文] --> B[日本語 한국어]", testMermaidStyle())
	if d.err != "" || len(d.labels) == 0 {
		t.Fatalf("diagram fixture: %s", d.err)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) { d.draw(p, r) })
	}, 600, 250)
	tt.Frame()
	want := bytes.Clone(tt.Image().Pix)
	for cycle := range 3 {
		for i := range 640 {
			ui.ShapeText("cache 中文", ui.Font{Size: 10 + float32(i)/8}, 0)
		}
		tt.Frame()
		tt.Frame()
		if !bytes.Equal(want, tt.Image().Pix) {
			t.Fatalf("cycle %d: Mermaid labels disappeared after font recycling", cycle)
		}
	}
}
