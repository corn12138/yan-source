package launch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRequests(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a b.go", "literal:42", "-file"} {
		os.WriteFile(filepath.Join(dir, name), nil, 0644)
	}
	r := Parse([]string{"--new-window", "a b.go:12", "literal:42", "--", "-file"}, dir)
	if !r.NewWindow || len(r.Targets) != 3 {
		t.Fatalf("request %+v", r)
	}
	if r.Targets[0].File != filepath.Join(dir, "a b.go") || r.Targets[0].Line != 12 {
		t.Fatalf("line target %+v", r.Targets[0])
	}
	if r.Targets[1].File != filepath.Join(dir, "literal:42") || r.Targets[1].Line != 0 {
		t.Fatal("literal colon misread")
	}
	if r.Targets[2].File != filepath.Join(dir, "-file") {
		t.Fatal("dash filename lost")
	}
	r = Parse([]string{"."}, dir)
	if !r.Targets[0].Folder || r.Targets[0].Root != dir {
		t.Fatal("relative folder wrong")
	}
}
