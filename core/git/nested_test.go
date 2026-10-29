package git

// Depth and exclusion cases adapted from Mayoi d28bc2500f47 (MIT).

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNested(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	dir := t.TempDir()
	for _, rel := range []string{"a", "group/b", "a/inner", "group/x/c", ".hidden/d", "node_modules/e", "plain/not-a-repo"} {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if rel == "plain/not-a-repo" {
			continue
		}
		if out, err := exec.Command("git", "-C", p, "init", "-q").CombinedOutput(); err != nil {
			t.Fatalf("%s: %v %s", rel, err, out)
		}
	}
	os.MkdirAll(filepath.Join(dir, "broken", ".git"), 0o755)
	repos, err := Nested(context.Background(), dir, NestedDepth)
	if err != nil {
		t.Fatal(err)
	}
	var roots []string
	for _, repo := range repos {
		rel, _ := filepath.Rel(dir, repo.Root)
		roots = append(roots, filepath.ToSlash(rel))
	}
	if !reflect.DeepEqual(roots, []string{"a", "group/b"}) {
		t.Fatalf("roots %q", roots)
	}
	repos, err = Nested(context.Background(), dir, 1)
	if err != nil || len(repos) != 1 || repos[0].Root != filepath.Join(dir, "a") {
		t.Fatalf("depth one: %v %v", repos, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Nested(ctx, dir, NestedDepth); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := Nested(context.Background(), filepath.Join(dir, "gone"), NestedDepth); err == nil {
		t.Fatal("missing root was accepted")
	}
}

func TestNestedLinkedWorktreeAndSymlinks(t *testing.T) {
	_, git := repo(t)
	dir := t.TempDir()
	work := filepath.Join(dir, "linked")
	git("worktree", "add", "-b", "nested", work)
	repos, err := Nested(context.Background(), dir, NestedDepth)
	if err != nil || len(repos) != 1 || repos[0].Root != work {
		t.Fatalf("linked .git file: %v %v", repos, err)
	}
	if err := os.Symlink(work, filepath.Join(dir, "alias")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	repos, err = Nested(context.Background(), dir, NestedDepth)
	if err != nil || len(repos) != 1 {
		t.Fatalf("followed child symlink: %v %v", repos, err)
	}
	alias := filepath.Join(t.TempDir(), "workspace")
	if err := os.Symlink(dir, alias); err != nil {
		t.Fatal(err)
	}
	repos, err = Nested(context.Background(), alias, NestedDepth)
	if err != nil || len(repos) != 1 || repos[0].Root != filepath.Join(alias, "linked") {
		t.Fatalf("workspace spelling lost: %v %v", repos, err)
	}
}
