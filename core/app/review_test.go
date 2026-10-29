package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func selectReviewLines(e *Editor, from, to int) {
	s := e.cur().doc.buf.Snapshot()
	e.cur().doc.buf.SetSelections(edit.Single(edit.Range{Anchor: s.LineStart(from), Head: s.LineStart(to), GoalX: -1}))
}
func keepReview(t *testing.T, e *Editor, value string) {
	t.Helper()
	if err := e.addReviewComment(); err != nil {
		t.Fatal(err)
	}
	e.SetReviewInput(value)
	if err := e.saveReview(); err != nil {
		t.Fatal(err)
	}
}

func TestReviewFileLifecycle(t *testing.T) {
	e, h, path := newTestEditor(t, "zero\n目标\nreturn 1\ntail\n")
	defer e.Close()
	e.OpenFolder(filepath.Dir(path))
	statePath := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(statePath))
	selectReviewLines(e, 1, 3)
	if err := e.addReviewComment(); err != nil {
		t.Fatal(err)
	}
	if e.review.draft.From != 1 || e.review.draft.To != 3 {
		t.Fatal("line-start selection included the next line")
	}
	if err := e.saveReview(); err == nil {
		t.Fatal("empty comment accepted")
	}
	e.Handle(TextInput{Text: "建议\n保留测试"})
	e.Handle(key(keymap.KeyEnter, keymap.ModCtrl))
	n := e.review.notes[0]
	if n.Text != "建议\n保留测试" || e.review.draft != nil {
		t.Fatalf("draft %+v", e.review)
	}
	selectReviewLines(e, 0, 0)
	typeText(e, "new\n")
	var frame view.Frame
	e.Frame(&frame)
	if n.From != 2 || n.Outdated {
		t.Fatalf("follow %+v", n)
	}
	if err := e.sendReview(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.clip, "f.txt:3-4") || len(e.review.notes) != 1 {
		t.Fatal("clipboard fallback lost comment")
	}
	e.SaveSession()
	saved := config.LoadState(statePath).Sessions[filepath.Dir(path)].Review
	if len(saved) != 1 || saved[0].Text != n.Text {
		t.Fatal("not persisted")
	}
	e.DeleteReview(n.id)
	selectReviewLines(e, 0, 0)
	keepReview(t, e, "later")
	e.undoReview()
	if len(e.review.notes) != 2 {
		t.Fatal("undo dropped a later comment")
	}
	e.saveAll()
	if err := e.SwitchFolder(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if len(e.review.notes) != 0 {
		t.Fatal("comments crossed projects")
	}
}

type reviewHost struct {
	fakeHost
	sent      string
	available bool
	err       error
}

func (h *reviewHost) PasteReview(s string) (bool, error) { h.sent = s; return h.available, h.err }
func TestReviewHandoffFailureAndUndo(t *testing.T) {
	e, _, _ := newTestEditor(t, "code\n")
	defer e.Close()
	h := &reviewHost{available: true, err: errors.New("closed")}
	e.host = h
	keepReview(t, e, "comment")
	if err := e.sendReview(); err == nil || len(e.review.notes) != 1 {
		t.Fatal("failed send discarded notes")
	}
	h.err = nil
	if err := e.sendReview(); err != nil || len(e.review.notes) != 0 {
		t.Fatal("successful handoff")
	}
	if strings.HasSuffix(h.sent, "\n") {
		t.Fatal("submitted review")
	}
	e.undoReview()
	if len(e.review.notes) != 1 {
		t.Fatal("send clear cannot be undone")
	}
}

func TestReviewSplitExcerptAndCards(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.go": "before\nold long text never clipped\nafter\n"})
	path := filepath.Join(root, "a.go")
	os.WriteFile(path, []byte("before\nnew long text never clipped\nafter\n"), 0644)
	e := newEditorIn(t, root)
	defer e.Close()
	pump(t, e, func() bool { return e.git.status != nil })
	e.cfg.DiffLayout = "split"
	e.SetPaneSize(0, view.Size{W: 50, H: 30})
	if err := e.openChange(path, false); err != nil {
		t.Fatal(err)
	}
	tb := e.cur()
	row := -1
	for i, l := range tb.diff.lines {
		if l.kind == diffAdded {
			row = i
			break
		}
	}
	if row < 0 {
		t.Fatal("no change")
	}
	e.diffGo(tb, row)
	ns := e.reviewExcerpts(tb)
	if len(ns) != 1 || strings.Join(ns[0].Snippet, "\n") != "-old long text never clipped\n+new long text never clipped" {
		t.Fatalf("excerpt %+v", ns)
	}
	keepReview(t, e, "Use the full text")
	var fr view.Frame
	e.Frame(&fr)
	card := -1
	for i, l := range tb.diff.lines {
		if l.kind == diffReview {
			card = i
			break
		}
	}
	if card < 0 {
		t.Fatal("no inline comment card")
	}
	e.diffGo(tb, card)
	e.Run("workbench.action.showCommands")
	e.Handle(key(keymap.KeyDelete, 0))
	if len(e.review.notes) != 1 {
		t.Fatal("palette Delete removed the underlying comment")
	}
	e.closeOverlay()
	e.Run("review.comment")
	if e.review.draft != e.review.notes[0] {
		t.Fatal("card did not edit original")
	}
	e.cancelReview()
	e.cfg.DiffLayout = "inline"
	e.refreshDiff(tb.doc, tb.diff)
	for i, l := range tb.diff.lines {
		if l.kind == diffRemoved {
			e.diffGo(tb, i)
			break
		}
	}
	keepReview(t, e, "Why removed?")
	if !e.review.notes[1].OldSide || e.review.notes[1].From != 1 {
		t.Fatal("removed-line coordinates")
	}
	// Staged and working-tree comments remain separate even at the same path and line.
	if out, err := exec.Command("git", "-C", root, "add", "a.go").CombinedOutput(); err != nil {
		t.Fatalf("%s %v", out, err)
	}
	e.gitChanged()
	pump(t, e, func() bool {
		return e.git.status != nil && len(e.git.status.Entries) > 0 && e.git.status.Entries[0].Staged()
	})
	if err := e.openAllChanges(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return !e.cur().diff.loading && len(e.cur().diff.secs) > 0 })
	for i, l := range e.cur().diff.lines {
		if l.kind == diffAdded {
			e.diffGo(e.cur(), i)
			break
		}
	}
	keepReview(t, e, "Staged only")
	if !e.review.notes[2].Staged {
		t.Fatal("staged scope lost")
	}
}

func TestReviewRestoreAndCommitReference(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.go": "original\n"})
	path := filepath.Join(root, "a.go")
	os.WriteFile(path, []byte("replacement\n"), 0644)
	for _, args := range [][]string{{"add", "a.go"}, {"commit", "-qm", "replacement"}} {
		c := exec.Command("git", args...)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%s %v", out, err)
		}
	}
	statePath := filepath.Join(t.TempDir(), "state.json")
	e := newEditorIn(t, root)
	e.SetState(config.LoadState(statePath))
	pump(t, e, func() bool { return e.git.repo != nil })
	commit, err := e.git.repo.CommitInfo("HEAD")
	if err != nil {
		t.Fatal(err)
	}
	files, err := e.git.repo.CommitFiles(commit)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.openCommitChange(commit, files[0]); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.cur() != nil && e.cur().diff != nil })
	for i, l := range e.cur().diff.lines {
		if l.kind == diffAdded {
			e.diffGo(e.cur(), i)
			break
		}
	}
	keepReview(t, e, "commit feedback")
	if e.review.notes[0].Commit != commit.Hash {
		t.Fatal("commit identity lost")
	}
	e.SaveSession()
	e.Close()
	restored := newEditorIn(t, root)
	defer restored.Close()
	restored.SetState(config.LoadState(statePath))
	restored.RestoreSession()
	if len(restored.review.notes) != 1 {
		t.Fatal("review-only session did not restore")
	}
	pump(t, restored, func() bool { return restored.git.repo != nil })
	if err := restored.goToReview(restored.review.notes[0]); err != nil {
		t.Fatal(err)
	}
	pump(t, restored, func() bool {
		var fr view.Frame
		restored.Frame(&fr)
		return fr.Review != nil
	})
	if restored.cur().diff.commit == nil || restored.cur().diff.commit.Hash != commit.Hash {
		t.Fatal("navigation did not reopen commit")
	}
	if !strings.Contains(restored.reviewText(), commit.Hash) {
		t.Fatal("copied review omitted revision")
	}
	restored.cancelReview()
	for i, l := range restored.cur().diff.lines {
		if l.kind == diffReview && restored.diffStage(restored.cur(), i) == nil {
			t.Fatal("comment card allowed staging")
		}
	}
}

func TestReviewPlatformShortcut(t *testing.T) {
	for _, osName := range []string{"darwin", "windows", "linux"} {
		t.Run(osName, func(t *testing.T) {
			mod, seq := keymap.ModCtrl, "ctrl+shift+m"
			if osName == "darwin" {
				mod, seq = keymap.ModSuper, "cmd+shift+m"
			}
			e, warns := New(config.Default(), fakeLayouter{}, &fakeHost{}, keymap.Profile{OS: osName, GUI: true, Mods: []keymap.Mod{mod}})
			defer e.Close()
			if len(warns) > 0 {
				t.Fatal(warns)
			}
			if got := resolve(t, e, keymap.CtxGlobal, seq); got != "review.comment" {
				t.Fatalf("binding = %q", got)
			}
		})
	}
}

func TestReviewMissingSourceRemainsEditable(t *testing.T) {
	e, _, path := newTestEditor(t, "quoted source\n")
	keepReview(t, e, "feedback")
	notes := e.reviewNotes()
	e.Close()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	restored := newEditorIn(t, filepath.Dir(path))
	defer restored.Close()
	restored.restoreReview(notes)
	restored.followReview(true)
	n := restored.review.notes[0]
	if !n.Outdated {
		t.Fatal("missing source was not marked outdated")
	}
	if err := restored.goToReview(n); err != nil {
		t.Fatal(err)
	}
	if restored.review.draft != n || n.Snippet[0] != "quoted source" {
		t.Fatal("original review inaccessible")
	}
}
