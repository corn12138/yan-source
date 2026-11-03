//go:build selftest

package gui

import (
	"bytes"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestNativeTerminalFontCacheRecovery(t *testing.T) {
	h := newHarness(t, sample, Options{})
	s, conn, tt := newFileTestTerminal(t, h)
	s.t.Feed([]byte("\x1b[?25lEnglish 中文 日本語 한국어\r\n\x1b[1mBold\x1b[0m and normal"))
	tt.Frame()
	tt.Frame()
	want := bytes.Clone(tt.Image().Pix)
	for cycle := range 3 {
		// MyGo releases its native fonts after a frame exceeds 512 cached fonts.
		// Keep terminal output unchanged so dirty-row invalidation cannot hide stale glyphs.
		for i := range 640 {
			ui.ShapeText("cache 中文", ui.Font{Size: 10 + float32(i)/8}, 0)
		}
		tt.Frame()
		tt.Frame()
		if !bytes.Equal(want, tt.Image().Pix) {
			t.Fatalf("cycle %d: font recycling changed or erased terminal text", cycle)
		}
	}
	tt.Type("still responsive")
	waitTerminalInput(t, conn, "still responsive")
}
