package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/corn12138/yan-source/core/review"
)

func TestStateSaveProtectsReviewContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "yan", "state.json")
	s := LoadState(path)
	s.Sessions = map[string]Session{"project": {Review: []review.Note{{
		Path: "source.go", Text: "private review", Snippet: []string{"private source excerpt"},
	}}}}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	got := LoadState(path).Sessions["project"].Review
	if len(got) != 1 || got[0].Text != "private review" || len(got[0].Snippet) != 1 || got[0].Snippet[0] != "private source excerpt" {
		t.Fatalf("review did not survive saving: %#v", got)
	}
	if runtime.GOOS != "windows" {
		assertStateMode(t, filepath.Dir(path), 0o700)
		assertStateMode(t, path, 0o600)
	}
}

func TestStateSaveProtectsExistingPublicState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	dir := filepath.Join(t.TempDir(), "shared-config")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte(`{"recent":["project"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	s := LoadState(path)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	assertStateMode(t, path, 0o600)
	assertStateMode(t, dir, 0o755)
	if got := LoadState(path).Recent; len(got) != 1 || got[0] != "project" {
		t.Fatalf("existing state lost: %v", got)
	}
}

func TestStateSaveDoesNotFollowPredictableTemporarySymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require elevated privileges")
	}
	dir := t.TempDir()
	path, victim := filepath.Join(dir, "state.json"), filepath.Join(dir, "unrelated.txt")
	if err := os.WriteFile(victim, []byte("keep this content"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, path+".tmp"); err != nil {
		t.Fatal(err)
	}
	s := LoadState(path)
	s.Recent = []string{"project"}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(victim); err != nil || string(got) != "keep this content" {
		t.Fatalf("temporary symlink target changed: %q, %v", got, err)
	}
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("state is not a regular file: %v, %v", info, err)
	}
	assertStateMode(t, path, 0o600)
}

func TestStateSaveRemovesTemporaryContentOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	s := LoadState(path)
	s.Recent = []string{"project"}
	if err := s.Save(); err == nil {
		t.Fatal("saving over a directory succeeded")
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Name() != "state.json" || !files[0].IsDir() {
		t.Fatalf("failed save left temporary content: %v", files)
	}
}

func assertStateMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s permissions = %04o, want %04o", filepath.Base(path), got, want)
	}
}
