package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func multiEditor(t *testing.T) (*Editor, string, string) {
	t.Helper()
	root := t.TempDir()
	a, b := nestedFixture(t, root, "a"), nestedFixture(t, root, "group/b")
	e := newEditorIn(t, root)
	pump(t, e, func() bool {
		if len(e.repositories.contexts) != 2 {
			return false
		}
		for _, rc := range e.repositories.contexts {
			if rc.git.status == nil || len(rc.scm.history.commits) == 0 {
				return false
			}
		}
		return true
	})
	return e, a, b
}
func writeRepoFile(t *testing.T, root, value string) string {
	t.Helper()
	p := filepath.Join(root, "file.txt")
	if err := os.WriteFile(p, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestMultiRepositoryDraftsAndCommit(t *testing.T) {
	e, a, b := multiEditor(t)
	ac, bc := e.repositoryForRoot(a), e.repositoryForRoot(b)
	writeRepoFile(t, a, "a change\n")
	bp := writeRepoFile(t, b, "b change\n")
	e.gitChanged()
	pump(t, e, func() bool { return e.changeCount() == 2 })
	e.WithRepository(ac.id, func(e *Editor) { e.SetCommitMessage("A draft") })
	e.WithRepository(bc.id, func(e *Editor) { e.SetCommitMessage("only B"); e.SCMStage(bp, false) })
	e.WithRepository(ac.id, func(e *Editor) { e.FocusCommitMessage(true) })
	pump(t, e, func() bool { return len(bc.git.status.Entries) > 0 && bc.git.status.Entries[0].Staged() })
	if ac.git.status.Entries[0].Staged() {
		t.Fatal("B stage changed A index")
	}
	e.WithRepository(bc.id, func(e *Editor) {
		if err := e.commit(false); err != nil {
			t.Fatal(err)
		}
	})
	e.WithRepository(ac.id, func(e *Editor) { e.SetCommitMessage("A still typing") })
	pump(t, e, func() bool { return bc.git.busy == "" && len(bc.git.status.Entries) == 0 })
	if ac.scm.message != "A still typing" || bc.scm.message != "" {
		t.Fatalf("drafts: A=%q B=%q", ac.scm.message, bc.scm.message)
	}
	for _, check := range []struct{ root, want string }{{a, "first"}, {b, "only B"}} {
		out, err := exec.Command("git", "-C", check.root, "log", "-1", "--format=%s").Output()
		if err != nil || strings.TrimSpace(string(out)) != check.want {
			t.Fatalf("commit %s: %s %v", check.root, out, err)
		}
	}
	if e.action.repositoryID != bc.id {
		t.Fatal("undo toast lost repository")
	}
	e.WithRepository(e.action.repositoryID, func(e *Editor) { e.Run("git.undoCommit") })
	e.repositories.selected = a
	pump(t, e, func() bool { return bc.git.busy == "" && bc.scm.message == "only B" })
	if ac.scm.message != "A still typing" {
		t.Fatal("undo replaced another repository draft")
	}
}
func TestMultiRepositoryPinnedDiffSessionAndReview(t *testing.T) {
	e, a, b := multiEditor(t)
	ac, bc := e.repositoryForRoot(a), e.repositoryForRoot(b)
	ap := writeRepoFile(t, a, "A working\n")
	writeRepoFile(t, b, "B working\n")
	e.gitChanged()
	pump(t, e, func() bool { return e.changeCount() == 2 })
	e.repositories.selected = a
	if err := e.openAllChanges(); err != nil {
		t.Fatal(err)
	}
	changes := e.cur()
	pump(t, e, func() bool { return !changes.diff.loading && len(changes.diff.secs) == 1 })
	e.repositories.selected = b
	e.refreshChanges(changes)
	if changes.diff.repoRoot != a || changes.diff.secs[0].path != ap {
		t.Fatal("diff followed picker")
	}
	line := -1
	for i, l := range changes.diff.lines {
		if l.kind == diffFile {
			line = i
			break
		}
	}
	if err := e.diffStage(changes, line); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return ac.git.status.Entries[0].Staged() })
	if bc.git.status.Entries[0].Staged() {
		t.Fatal("diff staging followed picker")
	}
	// Worktrees can share hashes; the root must be part of the virtual tab identity.
	statePath := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(statePath))
	hash := ac.scm.history.commits[0].Hash
	e.repositories.selected = a
	at := e.showHistoryTab("", hash, false)
	e.repositories.selected = b
	bt := e.showHistoryTab("", bc.scm.history.commits[0].Hash, false)
	pump(t, e, func() bool { return !at.diff.loading && !bt.diff.loading })
	if at == bt || at.diff.repoRoot != a || bt.diff.repoRoot != b || at.doc.buf.Path() == bt.doc.buf.Path() {
		t.Fatal("history identities collided")
	}
	if !strings.Contains(tabText(at), "first") || !strings.Contains(tabText(bt), "first") {
		t.Fatal("history data missing")
	}
	e.SaveSession()
	restored := newEditorIn(t, e.root)
	restored.SetState(config.LoadState(statePath))
	restored.RestoreSession()
	pump(t, restored, func() bool {
		count := 0
		for _, c := range restored.cols {
			for _, tab := range c.tabs {
				if tab.diff != nil && tab.diff.commit != nil {
					if tab.diff.loading {
						return false
					}
					count++
				}
			}
		}
		return count == 2
	})
	roots := map[string]bool{}
	for _, c := range restored.cols {
		for _, tab := range c.tabs {
			if tab.diff != nil {
				roots[tab.diff.repoRoot] = true
				if tab.diff.loadError != "" {
					t.Fatal(tab.diff.loadError)
				}
			}
		}
	}
	if !roots[a] || !roots[b] {
		t.Fatalf("restored roots: %v", roots)
	}
}
func TestMultiRepositoryStaleCallbacksAndDialogs(t *testing.T) {
	e, a, b := multiEditor(t)
	ac, bc := e.repositoryForRoot(a), e.repositoryForRoot(b)
	e.repositories.selected = a
	post := e.postGit()
	e.gitPrompt("new branch", "topic", func(s string) string { return s }, func(e *Editor, s string) error { e.repository().scm.message = s; return nil })
	e.repositories.selected = b
	e.Handle(key(keymap.KeyEnter, 0))
	if ac.scm.message != "topic" || bc.scm.message != "" {
		t.Fatal("prompt followed picker")
	}
	post(func(e *Editor) { e.repository().scm.message = "completed A" })
	e.RunPending()
	if ac.scm.message != "completed A" || bc.scm.message != "" {
		t.Fatal("async callback followed picker")
	}
	e.repositories.selected = a
	post = e.postGit()
	e.gitChoose("delete", ovItem{label: "confirm", run: func(e *Editor) error { e.repository().scm.message = "unsafe"; return nil }})
	if err := os.RemoveAll(a); err != nil {
		t.Fatal(err)
	}
	e.gitChanged()
	pump(t, e, func() bool { return ac.dead })
	nestedFixture(t, e.root, "a")
	e.gitChanged()
	pump(t, e, func() bool { return e.repositoryForRoot(a) != nil })
	replacement := e.repositoryForRoot(a)
	if replacement.id == ac.id {
		t.Fatal("retired ID reused")
	}
	e.Handle(key(keymap.KeyEnter, 0))
	e.WithRepository(ac.id, func(e *Editor) { e.SetCommitMessage("stale click") })
	post(func(e *Editor) { e.SetCommitMessage("stale completion") })
	e.RunPending()
	if replacement.scm.message != "" || bc.scm.message != "" {
		t.Fatal("stale action reached a live repository")
	}
}
func TestMultiRepositoryAutoAndFrame(t *testing.T) {
	e, a, b := multiEditor(t)
	if err := e.OpenFile(filepath.Join(b, "file.txt")); err != nil {
		t.Fatal(err)
	}
	if e.repository().git.repo.Root != b {
		t.Fatal("auto did not follow file")
	}
	e.WithRepository(e.repositoryForRoot(a).id, func(*Editor) {})
	if e.repository().git.repo.Root != a {
		t.Fatal("explicit selection not retained")
	}
	if e.markdownRoot(filepath.Join(b, "README.md")) != b {
		t.Fatal("Markdown links followed SCM selection")
	}
	e.Run("yan.scm.chooseRepository")
	e.Handle(key(keymap.KeyEnter, 0))
	if e.repository().git.repo.Root != b {
		t.Fatal("auto choice did not restore file target")
	}
	e.Run("workbench.view.scm")
	var frame view.Frame
	e.Frame(&frame)
	if len(frame.Sidebar.SCM.Panels) != 2 || !frame.Sidebar.SCM.AutoRepository {
		t.Fatal("multi frame missing")
	}
	for _, p := range frame.Sidebar.SCM.Panels {
		if p.State.Root == a && len(p.State.Commits) > 0 {
			t.Fatal("inactive history duplicated")
		}
	}
}

func TestMultiRepositoryWorktreeAndSubmoduleOwnership(t *testing.T) {
	root := t.TempDir()
	a := nestedFixture(t, root, "main")
	run := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	linked := filepath.Join(root, "linked")
	run(a, "worktree", "add", "-b", "linked", linked)
	module := gitRepo(t, map[string]string{"file.txt": "module\n"})
	run(a, "-c", "protocol.file.allow=always", "submodule", "add", "-q", module, "module")
	run(a, "commit", "-qm", "add module")
	e := newEditorIn(t, root)
	pump(t, e, func() bool {
		return len(e.repositories.contexts) == 3 && e.repositoryForRoot(a).git.status != nil && e.repositoryForRoot(filepath.Join(a, "module")).git.status != nil
	})
	ac, lc, mc := e.repositoryForRoot(a), e.repositoryForRoot(linked), e.repositoryForRoot(filepath.Join(a, "module"))
	ai, ae := os.Stat(ac.git.repo.Common)
	li, le := os.Stat(lc.git.repo.Common)
	if ac == lc || ae != nil || le != nil || !os.SameFile(ai, li) {
		t.Fatal("linked worktrees were merged")
	}
	if e.repositoryForPath(filepath.Join(mc.git.repo.Root, "file.txt")) != mc {
		t.Fatal("parent owns child file")
	}
	e.autoSave = false
	e.OpenFile(filepath.Join(mc.git.repo.Root, "file.txt"))
	typeText(e, "unsaved module ")
	e.WithRepository(ac.id, func(e *Editor) {
		if err := e.saveDirtyInRepo(); err != nil {
			t.Fatal(err)
		}
	})
	data, err := os.ReadFile(filepath.Join(mc.git.repo.Root, "file.txt"))
	if err != nil || string(data) != "module\n" {
		t.Fatal("parent saved child buffer")
	}
	writeRepoFile(t, mc.git.repo.Root, "module change\n")
	e.gitChanged()
	pump(t, e, func() bool { return len(mc.git.status.Entries) > 0 && len(ac.git.status.Entries) > 0 })
	e.repositories.selected = a
	if err := e.openAllChanges(); err != nil {
		t.Fatal(err)
	}
	tab := e.cur()
	pump(t, e, func() bool { return !tab.diff.loading && len(tab.diff.secs) > 0 })
	if !strings.Contains(tabText(tab), "Subproject commit ") {
		t.Fatalf("gitlink diff missing: %s", tabText(tab))
	}
	e.WithRepository(lc.id, func(e *Editor) { e.SetCommitMessage("linked draft") })
	if ac.scm.message != "" || mc.scm.message != "" {
		t.Fatal("worktree draft leaked")
	}
}

func TestMultiRepositoryRemovalClearsPendingDiff(t *testing.T) {
	e, a, b := multiEditor(t)
	e.repositories.selected = a
	writeRepoFile(t, a, "change\n")
	e.gitChanged()
	pump(t, e, func() bool { return e.changeCount() == 1 })
	e.openAllChanges()
	tab := e.cur()
	pump(t, e, func() bool { return !tab.diff.loading })
	// Model an in-flight read whose completion will be discarded when its context retires.
	tab.diff.loading = true
	os.RemoveAll(a)
	e.gitChanged()
	pump(t, e, func() bool { return e.repositoryForRoot(a) == nil })
	if tab.diff.loading || tab.diff.loadError == "" {
		t.Fatal("removed repository left a permanent loading state")
	}
	if err := e.diffStage(tab, 0); err == nil {
		t.Fatal("removed repository diff allowed staging")
	}
	nestedFixture(t, e.root, "a")
	writeRepoFile(t, a, "replacement\n")
	e.gitChanged()
	pump(t, e, func() bool { rc := e.repositoryForRoot(a); return rc != nil && rc.git.status != nil })
	e.repositories.selected = b
	e.refreshChanges(tab)
	pump(t, e, func() bool { return !tab.diff.loading })
	if tab.diff.loadError != "" || !strings.Contains(tabText(tab), "replacement") {
		t.Fatal("rediscovered repository did not refresh its changes")
	}
}
