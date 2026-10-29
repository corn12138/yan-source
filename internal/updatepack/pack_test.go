package updatepack

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (string, string, ed25519.PrivateKey) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "yan.app")
	if err = os.MkdirAll(filepath.Join(bundle, "Contents", "MacOS"), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(bundle, "Contents", "MacOS", "yan"), []byte("new executable"), 0755); err != nil {
		t.Fatal(err)
	}
	return bundle, base64.StdEncoding.EncodeToString(pub), key
}

func TestPackAndVerify(t *testing.T) {
	bundle, pub, key := fixture(t)
	out := t.TempDir()
	tag := "v0.1.0-alpha.11"
	m, err := Pack(bundle, out, tag, "darwin-arm64", "Release notes", key)
	if err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(out, "yan-"+tag+"-darwin-arm64.update.tar.gz")
	if m.Version != "0.1.0-alpha.11" || m.Notes != "Release notes" || !strings.Contains(m.URL, Repository+"/releases/download/"+tag+"/") {
		t.Fatalf("manifest: %+v", m)
	}
	if err = Verify(archive, m, pub); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	executable := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(h.Name, "yan.app/") || h.Uid != 0 || h.Gid != 0 || h.Uname != "" {
			t.Fatalf("archive metadata: %+v", h)
		}
		if h.Name == "yan.app/Contents/MacOS/yan" {
			executable = h.Mode&0111 != 0
		}
	}
	if !executable {
		t.Fatal("executable mode lost")
	}
	m.Size++
	if Verify(archive, m, pub) == nil {
		t.Fatal("wrong length accepted")
	}
	m.Size--
	b, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	b[len(b)/2] ^= 1
	if err = os.WriteFile(archive, b, 0644); err != nil {
		t.Fatal(err)
	}
	if Verify(archive, m, pub) == nil {
		t.Fatal("tampered archive accepted")
	}
}

func TestWrongKeyAndEscapingLink(t *testing.T) {
	bundle, pub, key := fixture(t)
	if _, err := Key(base64.StdEncoding.EncodeToString(key), pub); err != nil {
		t.Fatal(err)
	}
	_, wrong, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := Key(base64.StdEncoding.EncodeToString(wrong), pub); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := Key("invalid", pub); err == nil {
		t.Fatal("malformed key accepted")
	}
	if err := os.Symlink("../../../outside", filepath.Join(bundle, "escape")); err != nil {
		t.Skip(err)
	}
	if _, err := Pack(bundle, t.TempDir(), "v0.1.0-alpha.11", "darwin-arm64", "", key); err == nil {
		t.Fatal("escaping link packaged")
	}
}

func TestPromotion(t *testing.T) {
	for _, tc := range []struct {
		old, tag string
		yes      bool
	}{
		{"0.1.0-alpha.9", "v0.1.0-alpha.10", true},
		{"0.1.0-alpha.11", "v0.1.0-alpha.11", true},
		{"0.1.0-alpha.11", "v0.1.0-alpha.10", false},
		{"0.1.0-alpha.11", "v0.1.0", true},
		{"0.1.0", "v0.1.0-alpha.12", false},
		{"0.2.0-alpha.1", "v0.1.1", false},
	} {
		v, err := CanPromote(tc.old, tc.tag)
		if err != nil || v != tc.yes {
			t.Errorf("%s -> %s: %v, %v", tc.old, tc.tag, v, err)
		}
	}
	if _, err := CanPromote("broken", "v0.1.0"); err == nil {
		t.Fatal("invalid previous manifest accepted")
	}
	for _, v := range []string{"v0.1.0-alpha.11", "v0.1.0"} {
		ch, err := Channel(v)
		if err != nil || ch == "" {
			t.Fatal(v, err)
		}
	}
	if _, err := Channel("v../../other"); err == nil {
		t.Fatal("invalid tag accepted")
	}
}
