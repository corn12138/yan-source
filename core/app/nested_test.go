package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/view"
)

func nestedFixture(t *testing.T, parent, name string) string {
	t.Helper()
	source := gitRepo(t, map[string]string{"file.txt": "first\n"})
	path := filepath.Join(parent, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(source, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNestedOpenAndCommitIsolation(t *testing.T) {
	root := t.TempDir()
	a := nestedFixture(t, root, "a")
	b := nestedFixture(t, root, "group/b")
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.ready && len(e.git.nested) == 2 })
	e.Run("workbench.view.scm")
	var frame view.Frame
	e.Frame(&frame)
	if frame.Sidebar.SCM.NoRepo || len(frame.Sidebar.SCM.Panels) != 2 || frame.Sidebar.SCM.Panels[1].Name != "group/b" {
		t.Fatalf("panels: %+v", frame.Sidebar.SCM)
	}
	e.WithRepository(e.repositoryForRoot(b).id, func(e *Editor) { e.FocusCommitMessage(true) })
	if e.root != root || e.repository().git.repo.Root != b {
		t.Fatal("same-screen selection changed workspace")
	}
	path := filepath.Join(b, "file.txt")
	os.WriteFile(path, []byte("changed\n"), 0o644)
	e.gitChanged()
	pump(t, e, func() bool { return e.changeCount() == 1 })
	e.SCMStage(path, false)
	pump(t, e, func() bool { return e.stagedCount() == 1 })
	e.SetCommitMessage("only b")
	if err := e.commit(false); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.repository().git.busy == "" && e.changeCount() == 0 })
	for _, check := range []struct{ root, want string }{{a, "first"}, {b, "only b"}} {
		out, err := exec.Command("git", "-C", check.root, "log", "-1", "--format=%s").Output()
		if err != nil || strings.TrimSpace(string(out)) != check.want {
			t.Fatalf("commit target %s: %s %v", check.root, out, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); !os.IsNotExist(err) {
		t.Fatal("initialized the workspace parent")
	}
}

func TestNestedRefreshAndStaleOpen(t *testing.T) {
	root := t.TempDir()
	b := nestedFixture(t, root, "b")
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return len(e.git.nested) == 1 })
	e.Run("workbench.view.scm")
	e.repositories.selected = b
	a := nestedFixture(t, root, "a")
	e.Run("yan.scm.refreshRepositories")
	pump(t, e, func() bool { return len(e.git.nested) == 2 })
	if e.repository().git.repo.Root != b {
		t.Fatal("selection moved to a newly inserted repository")
	}
	os.RemoveAll(a)
	e.SCMOpenRepository(a)
	pump(t, e, func() bool { return strings.Contains(e.status, "已不可用") })
	if e.root != root {
		t.Fatal("stale open switched workspace")
	}
	pump(t, e, func() bool { return len(e.git.nested) == 1 })
	// A failing scan must not present an empty list as a successful discovery.
	moved := root + "-moved"
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Rename(moved, root) })
	e.gitChanged()
	pump(t, e, func() bool { return e.git.nestedError != "" })
	if len(e.git.nested) != 1 {
		t.Fatal("failed scan dropped previous repositories")
	}
}

func TestNestedAutomaticDiscoveryAndRootInit(t *testing.T) {
	root := t.TempDir()
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.ready })
	nestedFixture(t, root, "new")
	deadline := time.NewTimer(8 * time.Second)
	defer deadline.Stop()
	for len(e.git.nested) == 0 {
		select {
		case <-e.Wake():
			e.RunPending()
		case <-deadline.C:
			t.Fatal("new repository was not discovered automatically")
		}
	}
	if err := e.initRepo(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.repo != nil && e.git.status != nil })
	if len(e.git.nested) != 0 || e.git.repo.Root != root {
		t.Fatal("root repository did not replace discovery list")
	}
}

func TestNestedProtectsUnsavedAndWorkspaceChanges(t *testing.T) {
	root := t.TempDir()
	child := nestedFixture(t, root, "child")
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return len(e.git.nested) == 1 })
	os.WriteFile(filepath.Join(root, "draft.txt"), nil, 0o644)
	e.OpenFile(filepath.Join(root, "draft.txt"))
	e.autoSave = false
	typeText(e, "keep me")
	e.SCMOpenRepository(child)
	pump(t, e, func() bool { return strings.Contains(e.status, "有没保存的修改") })
	if e.root != root || content(e) != "keep me" || !e.Dirty() {
		t.Fatal("opening a child lost the unsaved buffer")
	}
	// Queue an open result, then change workspace before its callback is delivered.
	e.SCMOpenRepository(child)
	other := gitRepo(t, map[string]string{"other.txt": "other"})
	if err := e.OpenFolder(other); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.status != nil })
	if e.root != other || e.git.repo.Root != other || len(e.git.nested) != 0 {
		t.Fatal("old discovery/open overwrote new workspace")
	}
}
