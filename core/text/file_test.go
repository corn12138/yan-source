package text

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchesFileBytes(t *testing.T) {
	for _, tc := range []struct{ name, original string }{
		{"empty", ""}, {"plain", "hello\n"}, {"bom-crlf", "\xef\xbb\xbf中文\r\n"}, {"large", strings.Repeat("hello world\r\n", 10000)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.original
			p := filepath.Join(t.TempDir(), "file.txt")
			if err := os.WriteFile(p, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			s, err := Load(p)
			if err != nil {
				t.Fatal(err)
			}
			if ok, err := MatchesFile(s, p); !ok || err != nil {
				t.Fatalf("original: %v %v", ok, err)
			}
			variants := []string{original + "x"}
			if len(original) > 0 {
				variants = append(variants, original[:len(original)-1], original[:len(original)-1]+"!")
			}
			for _, changed := range variants {
				if err := os.WriteFile(p, []byte(changed), 0o600); err != nil {
					t.Fatal(err)
				}
				if ok, err := MatchesFile(s, p); ok || err != nil {
					t.Fatalf("changed length %d: %v %v", len(changed), ok, err)
				}
			}
		})
	}
	s := FromString("", Meta{})
	if ok, err := MatchesFile(s, t.TempDir()); ok || err == nil {
		t.Fatalf("directory: %v %v", ok, err)
	}
	if ok, err := MatchesFile(s, filepath.Join(t.TempDir(), "missing")); ok || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing: %v %v", ok, err)
	}
}

func TestSaveCheckedRejectsPreparedReplacement(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(p, []byte("base"), 0o600); err != nil {
		t.Fatal(err)
	}
	refused := errors.New("external edit")
	called := false
	s := FromString("local\r\n", Meta{Encoding: EncUTF8BOM})
	err := SaveChecked(s, p, func() error {
		called = true
		temps, err := filepath.Glob(filepath.Join(dir, ".yan-save-*"))
		if err != nil || len(temps) != 1 {
			t.Fatalf("prepared temps: %v %v", temps, err)
		}
		data, err := os.ReadFile(temps[0])
		if err != nil || string(data) != "\xef\xbb\xbflocal\r\n" {
			t.Fatalf("incomplete prepared bytes: %q %v", data, err)
		}
		if err := os.WriteFile(p, []byte("external"), 0o600); err != nil {
			t.Fatal(err)
		}
		return refused
	})
	if !called || !errors.Is(err, refused) {
		t.Fatalf("verification: %v %v", called, err)
	}
	data, err := os.ReadFile(p)
	if err != nil || string(data) != "external" {
		t.Fatalf("destination: %q %v", data, err)
	}
	temps, _ := filepath.Glob(filepath.Join(dir, ".yan-save-*"))
	if len(temps) != 0 {
		t.Fatalf("leaked temps: %v", temps)
	}
	if err := SaveChecked(s, p, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(p)
	if err != nil || string(data) != "\xef\xbb\xbflocal\r\n" {
		t.Fatalf("accepted save: %q %v", data, err)
	}
}
