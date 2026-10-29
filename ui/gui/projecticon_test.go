package gui

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func pngBytes(t *testing.T, w int) []byte {
	t.Helper()
	var b bytes.Buffer
	png.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, w)))
	return b.Bytes()
}

// icoOf builds an .ico holding the given PNG images.
func icoOf(imgs ...[]byte) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.LittleEndian, [3]uint16{0, 1, uint16(len(imgs))})
	off := 6 + 16*len(imgs)
	for _, img := range imgs {
		w, _ := png.DecodeConfig(bytes.NewReader(img))
		binary.Write(&b, binary.LittleEndian, [4]uint8{uint8(w.Width), uint8(w.Width), 0, 0})
		binary.Write(&b, binary.LittleEndian, [2]uint16{1, 32})
		binary.Write(&b, binary.LittleEndian, [2]uint32{uint32(len(img)), uint32(off)})
		off += len(img)
	}
	for _, img := range imgs {
		b.Write(img)
	}
	return b.Bytes()
}

func TestIcoPNGTakesTheLargest(t *testing.T) {
	small, large := pngBytes(t, 16), pngBytes(t, 48)
	if got := icoPNG(icoOf(small, large)); !bytes.Equal(got, large) {
		t.Fatal("not the largest PNG")
	}
	if icoPNG([]byte("not an icon")) != nil {
		t.Fatal("garbage decoded")
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "favicon.ico"), icoOf(small), 0o644)
	if decodeProjectImage(filepath.Join(dir, "favicon.ico")) == nil {
		t.Fatal("ico did not decode")
	}
}

// A project's logo is found in the background and shown in its square.
func TestProjectLogoFound(t *testing.T) {
	h := newHarness(t, map[string]string{"public/favicon.svg": `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16"/></svg>`}, Options{})
	g := &group{}
	if g.logoFor(h.f, h.root) != nil {
		t.Fatal("found before looking")
	}
	for end := time.Now().Add(3 * time.Second); h.f.logo.looking && time.Now().Before(end); {
		h.settle()
	}
	if g.logoFor(h.f, h.root) == nil {
		t.Fatal("no logo")
	}
}
