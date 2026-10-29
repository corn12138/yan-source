package recovery

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteReplacesAndRemoves(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recovery")
	id := FolderID("/work/project")
	cp := Checkpoint{ID: id, Root: "/work/project", Gen: 1, Drafts: []Draft{{Title: "未命名-1", Text: "一\r\n二", EOL: 1, Column: 1, Anchor: 2, Head: 3}}}
	if err := Write(dir, cp); err != nil {
		t.Fatal(err)
	}
	cp.Gen, cp.Drafts[0].Text = 2, "newer"
	if err := Write(dir, cp); err != nil {
		t.Fatal(err)
	}
	got, err := Read(dir, id)
	if err != nil || got.Gen != 2 || len(got.Drafts) != 1 || got.Drafts[0].Text != "newer" || got.Drafts[0].Head != 3 || got.Root != "/work/project" {
		t.Fatalf("read %+v %v", got, err)
	}
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(filepath.Join(dir, id+".json"))
		di, _ := os.Stat(dir)
		if fi.Mode().Perm() != 0o600 || di.Mode().Perm() != 0o700 {
			t.Fatalf("modes %v %v", fi.Mode(), di.Mode())
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("left files behind: %v", entries)
	}
	cp.Drafts = nil
	if err := Write(dir, cp); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, id+".json")); !os.IsNotExist(err) {
		t.Fatal("a checkpoint without drafts was kept")
	}
	if got, err := Read(dir, id); err != nil || len(got.Drafts) != 0 {
		t.Fatalf("missing checkpoint: %+v %v", got, err)
	}
}

// An unreadable or foreign file is moved aside and reported; the others still load, and a
// temp file left by an interrupted write is ignored.
func TestLoadIsolatesBadFiles(t *testing.T) {
	dir := t.TempDir()
	good := NewRootlessID()
	if err := Write(dir, Checkpoint{ID: good, Drafts: []Draft{{Title: "未命名-1", Text: "kept"}}}); err != nil {
		t.Fatal(err)
	}
	bad := NewRootlessID()
	os.WriteFile(filepath.Join(dir, bad+".json"), []byte("{not json"), 0o600)
	other := NewRootlessID()
	os.WriteFile(filepath.Join(dir, other+".json"), []byte(`{"schema":1,"id":"`+good+`","drafts":[]}`), 0o600)
	os.WriteFile(filepath.Join(dir, "."+good+"-123.tmp"), []byte("partial"), 0o600)
	cps, problems := Load(dir)
	if len(cps) != 1 || cps[0].ID != good || cps[0].Drafts[0].Text != "kept" {
		t.Fatalf("loaded %+v", cps)
	}
	if len(problems) != 2 {
		t.Fatalf("problems %v", problems)
	}
	entries, _ := os.ReadDir(dir)
	corrupt := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), ".corrupt-") {
			corrupt++
		}
	}
	if corrupt != 2 {
		t.Fatalf("not moved aside: %v", entries)
	}
	if cps, problems := Load(dir); len(cps) != 1 || len(problems) != 0 {
		t.Fatalf("second load %v %v", cps, problems)
	}
}

func TestLimitsAndNames(t *testing.T) {
	dir := t.TempDir()
	if err := Write(dir, Checkpoint{ID: "../x", Drafts: []Draft{{Text: "a"}}}); err == nil {
		t.Fatal("wrote outside the store")
	}
	if err := Write(dir, Checkpoint{ID: NewRootlessID(), Drafts: []Draft{{Text: strings.Repeat("x", maxDraft+1)}}}); err == nil {
		t.Fatal("kept an oversized draft")
	}
	id := NewRootlessID()
	os.WriteFile(filepath.Join(dir, id+".json"), []byte(`{"schema":9,"id":"`+id+`","drafts":[]}`), 0o600)
	if _, err := Read(dir, id); err == nil {
		t.Fatal("read an unknown schema")
	}
	if !Rootless(id) || Rootless(FolderID("/a")) || FolderID("/a/") != FolderID("/a") {
		t.Fatal("identities")
	}
}

// One owner per session at a time; releasing hands it on.
func TestAcquire(t *testing.T) {
	dir := t.TempDir()
	id := FolderID("/work/shared")
	first, err := Acquire(dir, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Acquire(dir, id); err != ErrInUse {
		t.Fatalf("second owner: %v", err)
	}
	first.Release()
	second, err := Acquire(dir, id)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	second.Release()
	second.Release()
	if _, err := Acquire(dir, "../x"); err == nil {
		t.Fatal("locked outside the store")
	}
}
