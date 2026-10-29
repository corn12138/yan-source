package text

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsSourceMapLongLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.js.map")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 26<<20)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); !errors.Is(err, ErrLineTooLong) {
		t.Fatal("26 MiB single line was accepted for layout")
	}
}

func TestLoadLineLimitBoundaries(t *testing.T) {
	line := strings.Repeat("x", MaxLineBytes)
	for _, tc := range []struct {
		name, data string
		reject     bool
	}{
		{"empty", "", false}, {"limit", line, false}, {"lf", line + "\n" + line, false},
		{"crlf-bom", "\xef\xbb\xbf" + line + "\r\n" + line + "\r\n", false},
		{"excess", line + "x", true}, {"middle", "ok\n" + line + "x\nend", true},
		{"last", "ok\n" + line + "x", true}, {"crlf-excess", line + "x\r\n", true},
		{"bare-cr", line + "\r", true}, {"unicode-bytes", strings.Repeat("中", MaxLineBytes/3) + "文", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sample.txt")
			if err := os.WriteFile(path, []byte(tc.data), 0o600); err != nil {
				t.Fatal(err)
			}
			s, err := Load(path)
			if errors.Is(err, ErrLineTooLong) != tc.reject || err != nil && !tc.reject {
				t.Fatalf("load: %v", err)
			}
			if !tc.reject {
				if err := Save(s, path); err != nil {
					t.Fatal(err)
				}
				b, _ := os.ReadFile(path)
				if string(b) != tc.data {
					t.Fatal("round trip changed BOM or line endings")
				}
			}
		})
	}
	// Total size still controls read-only mode independently of individual line length.
	path := filepath.Join(t.TempDir(), "large.txt")
	data := strings.Repeat("short line\n", NormalMax/11+1)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil || s.Meta().Tier != TierLarge || !s.Meta().ReadOnly {
		t.Fatalf("large multiline: %v", err)
	}
}
