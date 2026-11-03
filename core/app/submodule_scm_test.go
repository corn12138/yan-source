package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

func submoduleGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s (%v)", args, out, err)
	}
	return strings.TrimSpace(string(out))
}

func submoduleEditor(t *testing.T) (*Editor, *repositoryContext, *repositoryContext) {
	t.Helper()
	root := gitRepo(t, map[string]string{"parent.txt": "parent\n"})
	source := gitRepo(t, map[string]string{"file.txt": "child\n"})
	submoduleGit(t, root, "-c", "protocol.file.allow=always", "submodule", "add", "-q", source, "modules/lib")
	submoduleGit(t, root, "commit", "-qm", "add module")
	child := filepath.Join(root, "modules", "lib")
	submoduleGit(t, child, "config", "user.name", "Test")
	submoduleGit(t, child, "config", "user.email", "test@example.invalid")
	e := newEditorIn(t, root)
	pump(t, e, func() bool {
		p, c := e.repositoryForRoot(root), e.repositoryForRoot(child)
		return p != nil && c != nil && p.git.status != nil && c.git.status != nil
	})
	return e, e.repositoryForRoot(root), e.repositoryForRoot(child)
}

func TestSubmoduleSCMCommitAndNavigation(t *testing.T) {
	e, parent, child := submoduleEditor(t)
	root, module := parent.git.repo.Root, child.git.repo.Root
	oldRef := submoduleGit(t, root, "rev-parse", "HEAD:modules/lib")
	writeRepoFile(t, module, "child change\n")
	e.gitChanged()
	pump(t, e, func() bool { return parent.git.entries[module].Dirty() && len(child.git.status.Entries) == 1 })
	e.WithRepository(parent.id, func(e *Editor) {
		e.SetCommitMessage("parent draft")
		rows := scmFlat(e.scmGroups())
		if len(rows) != 1 || !rows[0].Submodule || rows[0].Detail != "内部改动" || rows[0].Tooltip == "" {
			t.Fatalf("dirty module: %+v", rows)
		}
		e.SCMOpenSubmodule(module)
	})
	if e.repository() != child || child.changesCollapsed() || e.Root() != root {
		t.Fatal("opening the module did not select its panel in the same workspace")
	}
	if err := e.checkRepositoryPaths([]string{filepath.Join(root, "parent.txt")}); err == nil {
		t.Fatal("child accepted a parent file")
	}
	e.WithRepository(parent.id, func(e *Editor) {
		if err := e.checkRepositoryPaths([]string{filepath.Join(module, "file.txt")}); err == nil {
			t.Fatal("parent accepted a child's file instead of its gitlink")
		}
	})
	e.WithRepository(child.id, func(e *Editor) {
		e.SetCommitMessage("child commit")
		e.SCMStage(filepath.Join(module, "file.txt"), false)
	})
	pump(t, e, func() bool { return len(child.git.status.Entries) == 1 && child.git.status.Entries[0].Staged() })
	e.WithRepository(child.id, func(e *Editor) {
		if err := e.commit(false); err != nil {
			t.Fatal(err)
		}
	})
	pump(t, e, func() bool {
		return child.git.busy == "" && len(child.git.status.Entries) == 0 && parent.git.entries[module].Sub == "SC.."
	})
	if parent.scm.message != "parent draft" || submoduleGit(t, root, "rev-parse", "HEAD:modules/lib") != oldRef {
		t.Fatal("child commit changed the parent commit or draft")
	}
	writeRepoFile(t, module, "another child change\n")
	e.gitChanged()
	pump(t, e, func() bool { return parent.git.entries[module].Sub == "SCM." })
	e.WithRepository(parent.id, func(e *Editor) {
		row := scmFlat(e.scmGroups())[0]
		if row.Detail != "提交引用 · 内部改动" {
			t.Fatalf("combined state: %+v", row)
		}
		e.SCMStage(module, false)
	})
	pump(t, e, func() bool { return parent.git.entries[module].Staged() })
	e.WithRepository(parent.id, func(e *Editor) {
		rows := scmFlat(e.scmGroups())
		if len(rows) != 2 || !rows[0].Staged || rows[0].Detail != "提交引用" || rows[1].Detail != "内部改动" {
			t.Fatalf("index and worktree states: %+v", rows)
		}
		if err := e.commit(false); err != nil {
			t.Fatal(err)
		}
	})
	pump(t, e, func() bool { return parent.git.busy == "" && parent.scm.message == "" })
	if got := submoduleGit(t, root, "rev-parse", "HEAD:modules/lib"); got != submoduleGit(t, module, "rev-parse", "HEAD") {
		t.Fatal("parent commit did not record the child's new commit")
	}
	if got, _ := os.ReadFile(filepath.Join(module, "file.txt")); string(got) != "another child change\n" {
		t.Fatal("parent commit changed child content")
	}
}

func TestSubmoduleSCMDiscardProtectsChild(t *testing.T) {
	e, parent, child := submoduleEditor(t)
	root, module := parent.git.repo.Root, child.git.repo.Root
	// With recursion enabled, restoring a parent gitlink can overwrite child
	// files. Verify mixed selections still restore ordinary parent files only.
	submoduleGit(t, root, "config", "submodule.recurse", "true")
	writeRepoFile(t, module, "keep this child content\n")
	parentFile := filepath.Join(root, "parent.txt")
	if err := os.WriteFile(parentFile, []byte("parent change\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.gitChanged()
	pump(t, e, func() bool { return len(parent.git.status.Entries) == 2 && parent.git.entries[module].Dirty() })
	e.WithRepository(parent.id, func(e *Editor) {
		rows := scmFlat(e.scmGroups())
		for i := range rows {
			rows[i].Submodule = false // A delayed caller need not have the latest frame metadata.
		}
		if err := e.discard(rows); err != nil {
			t.Fatal(err)
		}
	})
	pump(t, e, func() bool { return e.gitPending == 0 && len(parent.git.status.Entries) == 1 })
	if got, _ := os.ReadFile(filepath.Join(module, "file.txt")); string(got) != "keep this child content\n" {
		t.Fatalf("parent discard touched the child: %q", got)
	}
	if got, _ := os.ReadFile(parentFile); string(got) != "parent\n" {
		t.Fatal("parent file was not restored")
	}
	// Uninitialized modules must not select the parent as a fallback child.
	submoduleGit(t, root, "submodule", "deinit", "-f", "--", "modules/lib")
	e.gitChanged()
	pump(t, e, func() bool { return e.repositoryForRoot(module) == nil })
	e.WithRepository(parent.id, func(e *Editor) { e.SCMOpenSubmodule(module) })
	if e.repository() != parent || e.status == "" {
		t.Fatal("unavailable child did not leave the target intact and report why")
	}
}

func TestRepositoryExpansionAndSelection(t *testing.T) {
	e, a, b := multiEditor(t)
	ac, bc := e.repositoryForRoot(a), e.repositoryForRoot(b)
	var frame view.SCM
	e.scmFrame(&frame)
	for _, panel := range frame.Panels {
		if !panel.State.Collapsed {
			t.Fatal("clean repository did not start collapsed")
		}
	}
	e.SelectRepository(ac.id)
	e.ToggleRepository(bc.id)
	if e.repository() != ac || bc.changesCollapsed() {
		t.Fatal("expanding B changed the selected repository")
	}
	e.WithRepository(bc.id, func(e *Editor) { e.SetCommitMessage("retained draft") })
	e.ToggleRepository(bc.id)
	e.SelectRepository(ac.id)
	writeRepoFile(t, b, "B change\n")
	e.gitChanged()
	pump(t, e, func() bool { return len(bc.git.status.Entries) == 1 })
	if !bc.changesCollapsed() || bc.scm.message != "retained draft" {
		t.Fatal("refresh forgot explicit collapse or draft")
	}
	writeRepoFile(t, a, "A change\n")
	e.gitChanged()
	pump(t, e, func() bool { return len(ac.git.status.Entries) == 1 })
	if ac.changesCollapsed() {
		t.Fatal("repository with new changes did not expand by default")
	}
	e.SelectRepository(bc.id)
	if !bc.changesCollapsed() || e.repository() != bc {
		t.Fatal("choosing history changed expansion")
	}
}

func TestSCMSelectionSurvivesStatusRefresh(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n", "c.txt": "c\n"})
	b := filepath.Join(root, "b.txt")
	if err := os.WriteFile(b, []byte("B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 1 })
	e.scm.sel, e.scm.anchor = 0, 0
	a := filepath.Join(root, "a.txt")
	if err := os.WriteFile(a, []byte("A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.gitChanged()
	pump(t, e, func() bool { return len(e.git.status.Entries) == 2 })
	if row, ok := e.scmSelected(); !ok || row.Path != b || e.scm.anchor != e.scm.sel {
		t.Fatalf("refresh changed the selected file/range anchor: %+v", row)
	}
	if err := e.scmToggleStage(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.entries[b].Staged() })
	if e.git.entries[a].Staged() {
		t.Fatal("Space staged the inserted neighbor")
	}
}

func TestRepositoryMessageFocusRevealsDraft(t *testing.T) {
	e, a, _ := multiEditor(t)
	rc := e.repositoryForRoot(a)
	e.SelectRepository(rc.id)
	e.SetCommitMessage("retained draft")
	for _, explicit := range []bool{false, true} {
		if explicit {
			e.ToggleRepository(rc.id)
		}
		if !rc.changesCollapsed() {
			t.Fatal("expected a collapsed repository before keyboard focus")
		}
		e.Run("list.focusMessage")
		var frame view.SCM
		e.scmFrame(&frame)
		if frame.Panels[0].State.Collapsed || !frame.Panels[0].State.MessageFocused || frame.Panels[0].State.Message != "retained draft" {
			t.Fatalf("keyboard focus hid or lost the draft: %+v", frame.Panels[0].State)
		}
		if !frame.Panels[1].State.Collapsed {
			t.Fatal("focusing one draft expanded another repository")
		}
	}
}
