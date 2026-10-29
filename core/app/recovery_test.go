package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/recovery"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
)

func rootlessEditor(t *testing.T) *Editor {
	t.Helper()
	e, warns := New(config.Default(), fakeLayouter{}, &fakeHost{}, keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl, keymap.ModSuper}})
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}
	t.Cleanup(e.Close)
	e.SetPaneSize(0, view.Size{W: 90, H: 28})
	return e
}

func untitledDocs(e *Editor) []*document {
	var out []*document
	for _, d := range e.docs {
		if d.untitled() {
			out = append(out, d)
		}
	}
	return out
}

// waitRestored pumps until the drafts posted by opening a folder are back.
func waitRestored(t *testing.T, e *Editor, n int) {
	t.Helper()
	pump(t, e, func() bool { return len(untitledDocs(e)) >= n })
}

// Kept drafts come back when the folder opens again: titles, text, line endings, column, cursor,
// still unsaved. Closing the window does not ask about them once they are kept.
func TestKeepAndRestoreDrafts(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "first draft")
	if err := e.setEOL(text.EOLCRLF); err != nil {
		t.Fatal(err)
	}
	typeText(e, "!")
	if err := e.splitRight(); err != nil {
		t.Fatal(err)
	}
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "二号")
	if dirty, kept := e.Unsaved(); dirty || kept != nil {
		t.Fatalf("kept drafts still count as unsaved: %v %v", dirty, kept)
	}
	e.Close()

	again, _ := newFolderEditorAt(t, root)
	waitRestored(t, again, 2)
	docs := untitledDocs(again)
	byTitle := map[string]*document{}
	for _, d := range docs {
		byTitle[d.title] = d
	}
	one, two := byTitle["未命名-1"], byTitle["未命名-2"]
	if one == nil || two == nil || docText(two) != "二号" || !one.buf.Dirty() || !two.buf.Dirty() {
		t.Fatalf("restored %+v", docs)
	}
	if one.buf.Snapshot().Meta().EOL != text.EOLCRLF {
		t.Fatal("line endings not kept")
	}
	if len(again.cols) < 2 || !slices.ContainsFunc(again.cols[1].tabs, func(tb *tab) bool { return tb.doc == two }) {
		t.Fatal("second draft not back in its column")
	}
	if !strings.Contains(again.status, "找回了 2 个") {
		t.Fatalf("status %q", again.status)
	}
}

// Discarding a draft (closing its last tab twice) removes it from the backup; the editor closing
// afterwards writes nothing older back.
func TestDiscardedDraftStaysGone(t *testing.T) {
	e, root := newFolderEditor(t, nil)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "throw away")
	if err := e.KeepForRestart(); err != nil {
		t.Fatal(err)
	}
	e.Run("workbench.action.closeActiveEditor")
	e.Run("workbench.action.closeActiveEditor")
	e.Close()
	if recovery.Exists(recoveryDir(), recovery.FolderID(root)) {
		t.Fatal("discarded draft still backed up")
	}
}

// A late write of an older checkpoint never replaces a newer one.
func TestOlderCheckpointIsDropped(t *testing.T) {
	dir := t.TempDir()
	id := recovery.NewRootlessID()
	w := newRecoveryWriter(nil)
	defer w.close()
	write := func(gen uint64, s string) error {
		done := make(chan error, 1)
		w.submit(&recoveryJob{dir: dir, cp: recovery.Checkpoint{ID: id, Gen: gen, Drafts: []recovery.Draft{{Title: "未命名-1"}}},
			snaps: []text.Snapshot{text.FromString(s, text.Meta{})}, waiters: []chan error{done}})
		return <-done
	}
	if err := write(5, "newer"); err != nil {
		t.Fatal(err)
	}
	if err := write(3, "older"); err != nil {
		t.Fatal(err)
	}
	cp, err := recovery.Read(dir, id)
	if err != nil || cp.Gen != 5 || cp.Drafts[0].Text != "newer" {
		t.Fatalf("checkpoint %+v %v", cp, err)
	}
}

// Once a draft is saved to a file, the backup no longer holds it.
func TestSaveAsDropsTheDraft(t *testing.T) {
	e, root := newFolderEditor(t, nil)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "keep me")
	if err := e.KeepForRestart(); err != nil {
		t.Fatal(err)
	}
	e.Run("workbench.action.files.saveAs")
	answer(e, "kept.txt")
	e.Close()
	if recovery.Exists(recoveryDir(), recovery.FolderID(root)) {
		t.Fatal("saved draft still backed up")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "kept.txt")); string(b) != "keep me" {
		t.Fatalf("file %q", b)
	}
}

// When the backup cannot be written, untitled drafts count as unsaved again: closing must ask.
func TestKeepFailureCountsAsUnsaved(t *testing.T) {
	e, _ := newFolderEditor(t, nil)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "fragile")
	blocked := filepath.Join(t.TempDir(), "file-not-dir")
	os.WriteFile(blocked, nil, 0o600)
	saved := recoveryDir
	recoveryDir = func() string { return blocked }
	t.Cleanup(func() { recoveryDir = saved })
	if dirty, kept := e.Unsaved(); !dirty || kept == nil {
		t.Fatalf("a failed backup let the draft go: %v %v", dirty, kept)
	}
}

// A window without a folder keeps its drafts under its own identity, which a new rootless editor
// adopts; while one editor holds it, no other one takes it.
func TestRootlessDraftsAreAdopted(t *testing.T) {
	e := rootlessEditor(t)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "no folder")
	if err := e.KeepForRestart(); err != nil {
		t.Fatal(err)
	}
	id := e.rec.owned
	other := rootlessEditor(t)
	other.AdoptDrafts(id)
	if len(untitledDocs(other)) != 0 {
		t.Fatal("adopted drafts another editor still owns")
	}
	e.Close()
	ids, problems := RootlessDrafts()
	if !slices.Contains(ids, id) || len(problems) != 0 {
		t.Fatalf("rootless drafts %v %v", ids, problems)
	}
	other.AdoptDrafts(id)
	if docs := untitledDocs(other); len(docs) != 1 || docText(docs[0]) != "no folder" {
		t.Fatalf("adopted %+v", docs)
	}
	if other.rec.owned != id {
		t.Fatal("did not take over the identity")
	}
}

// Switching folders keeps the drafts of the folder left behind; they return with it.
func TestSwitchFolderKeepsDrafts(t *testing.T) {
	e, a := newFolderEditor(t, nil)
	b := t.TempDir()
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "for a")
	if err := e.SwitchFolder(b); err != nil {
		t.Fatal(err)
	}
	if len(untitledDocs(e)) != 0 {
		t.Fatal("drafts followed into the other folder")
	}
	if err := e.SwitchFolder(a); err != nil {
		t.Fatal(err)
	}
	waitRestored(t, e, 1)
	if docText(untitledDocs(e)[0]) != "for a" {
		t.Fatal("wrong draft back")
	}
}

// Another yan holding a folder's drafts is left alone: this editor keeps its own drafts under an
// identity of its own instead of overwriting the other's.
func TestFolderInUseKeepsDraftsApart(t *testing.T) {
	root := t.TempDir()
	l, err := recovery.Acquire(recoveryDir(), recovery.FolderID(root))
	if err != nil {
		t.Fatal(err)
	}
	defer l.Release()
	e, _ := newFolderEditorAt(t, root)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "mine")
	if err := e.KeepForRestart(); err != nil {
		t.Fatal(err)
	}
	if !recovery.Rootless(e.rec.owned) || recovery.Exists(recoveryDir(), recovery.FolderID(root)) {
		t.Fatalf("wrote into the other's backup: %s", e.rec.owned)
	}
}
