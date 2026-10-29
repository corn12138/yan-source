package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// frameTabNames are the tab names the front ends show.
func frameTabNames(e *Editor) []string {
	var f view.Frame
	e.Frame(&f)
	var out []string
	for _, c := range f.Columns {
		for _, t := range c.Tabs {
			out = append(out, t.Name)
		}
	}
	return out
}

// answer types into the open prompt and presses Enter.
func answer(e *Editor, s string) {
	e.ov.input = s
	e.setOverlayInput(s)
	e.Handle(key(keymap.KeyEnter, 0))
}

// Each new untitled document is its own: an empty path never makes two of them one.
func TestUntitledDocumentsAreIndependent(t *testing.T) {
	e, _ := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	e.Run("workbench.action.files.newUntitledFile")
	first := e.cur().doc
	typeText(e, "one")
	e.Run("workbench.action.files.newUntitledFile")
	second := e.cur().doc
	typeText(e, "two")
	if first == second || docText(first) != "one" || docText(second) != "two" {
		t.Fatalf("drafts share text: %q %q", docText(first), docText(second))
	}
	if names := strings.Join(frameTabNames(e), ","); names != "未命名-1,未命名-2" {
		t.Fatalf("tabs %s", names)
	}
	if e.docOf("") != nil {
		t.Fatal("an empty path names a document")
	}
}

// A split of an untitled document shows the same document: text and undo are shared.
func TestUntitledSplitSharesTextAndUndo(t *testing.T) {
	e, _ := newFolderEditor(t, nil)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "shared")
	d := e.cur().doc
	if err := e.splitRight(); err != nil {
		t.Fatal(err)
	}
	if e.cur().doc != d || len(d.tabs) != 2 {
		t.Fatalf("split opened another document")
	}
	typeText(e, "!")
	e.Run("undo")
	if got := docText(d); got != "shared" {
		t.Fatalf("after undo in the split: %q", got)
	}
}

// Saving an untitled document asks where; the file is written first, then the document is
// that file, with highlighting from its name and no unsaved changes.
func TestSaveUntitledThroughPrompt(t *testing.T) {
	e, root := newFolderEditor(t, nil)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "package main\n")
	d := e.cur().doc
	e.Run("workbench.action.files.save")
	if e.ov == nil {
		t.Fatal("save did not ask where")
	}
	answer(e, "main.go")
	p := filepath.Join(root, "main.go")
	if b, err := os.ReadFile(p); err != nil || string(b) != "package main\n" {
		t.Fatalf("file %q %v (status %q)", b, err, e.status)
	}
	if d.buf.Path() != p || d.buf.Dirty() || d.untitled() || d.hl == nil {
		t.Fatalf("not bound: path=%q dirty=%v hl=%v", d.buf.Path(), d.buf.Dirty(), d.hl != nil)
	}
	if names := frameTabNames(e); len(names) != 1 || names[0] != "main.go" {
		t.Fatalf("tabs %v", names)
	}
	e.Run("undo")
	if docText(d) != "" {
		t.Fatal("undo history lost by saving")
	}
}

// An existing file is replaced only after the user says so.
func TestSaveAsPromptAsksBeforeReplacing(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "old\n"})
	p := filepath.Join(root, "a.txt")
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "new\n")
	d := e.cur().doc
	e.Run("workbench.action.files.saveAs")
	answer(e, "a.txt")
	if b, _ := os.ReadFile(p); string(b) != "old\n" || !d.untitled() {
		t.Fatalf("replaced without asking: %q", b)
	}
	if e.ov == nil || !strings.Contains(e.ov.title, "已经存在") {
		t.Fatalf("no replace question: %+v", e.ov)
	}
	e.Handle(key(keymap.KeyEnter, 0)) // 替换
	if b, _ := os.ReadFile(p); string(b) != "new\n" || d.buf.Path() != p {
		t.Fatalf("after replace: %q path=%q", b, d.buf.Path())
	}
}

// A file already open in another tab is not saved over: two editable documents for one
// file would overwrite each other's changes.
func TestSaveAsRefusesAnOpenTarget(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "open\n"})
	if err := e.openPath(filepath.Join(root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "draft")
	d := e.cur().doc
	e.Run("workbench.action.files.saveAs")
	answer(e, "a.txt")
	if !d.untitled() || !d.buf.Dirty() || !strings.Contains(e.status, "已经在编辑器里打开") {
		t.Fatalf("status %q, untitled=%v", e.status, d.untitled())
	}
	if b, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(b) != "open\n" {
		t.Fatalf("open file overwritten: %q", b)
	}
}

// Save As of a named file writes the new file and leaves the old one as it was.
func TestSaveAsMovesANamedDocument(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	a := filepath.Join(root, "a.txt")
	if err := e.openPath(a); err != nil {
		t.Fatal(err)
	}
	typeText(e, "edited ")
	d := e.cur().doc
	e.Run("workbench.action.files.saveAs")
	answer(e, "b.txt")
	if b, _ := os.ReadFile(a); string(b) != "a\n" {
		t.Fatalf("old file changed: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "b.txt")); string(b) != "edited a\n" || d.buf.Path() != filepath.Join(root, "b.txt") {
		t.Fatalf("new file %q, path %q", b, d.buf.Path())
	}
}

// Saves that cannot ask where (Save All, closing with autosave) leave untitled documents
// alone and say so; their text stays.
func TestNonInteractiveSavesSkipUntitled(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	if err := e.openPath(filepath.Join(root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	typeText(e, "x")
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "draft")
	d := e.cur().doc
	e.Run("workbench.action.files.saveAll")
	if b, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(b) != "xa\n" {
		t.Fatalf("named file not saved: %q", b)
	}
	if !d.buf.Dirty() || !strings.Contains(e.status, "未命名") {
		t.Fatalf("status %q", e.status)
	}
	if err := e.saveDoc(d); err != errUntitled {
		t.Fatalf("saveDoc: %v", err)
	}
	if !e.Dirty() {
		t.Fatal("a dirty untitled document does not keep the editor dirty")
	}
}

// The last tab of a dirty untitled document closes only on the second request, and it is
// not offered for reopening: its text would be gone.
func TestCloseUntitled(t *testing.T) {
	e, _ := newFolderEditor(t, nil)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "draft")
	e.Run("workbench.action.closeActiveEditor")
	if e.cur() == nil || !strings.Contains(e.status, "未命名-1 有未保存的修改") {
		t.Fatalf("closed without warning: %q", e.status)
	}
	e.Run("workbench.action.closeActiveEditor")
	if e.cur() != nil || len(e.docs) != 0 {
		t.Fatal("second close kept the draft")
	}
	if len(e.closed) != 0 {
		t.Fatalf("untitled draft offered for reopening: %+v", e.closed)
	}
}

type saveAsHost struct {
	fakeHost
	asked []string
	done  []func(string, error)
}

func (h *saveAsHost) SaveAsDialog(path string, done func(string, error)) {
	h.asked = append(h.asked, path)
	h.done = append(h.done, done)
}

func newDialogEditor(t *testing.T) (*Editor, *saveAsHost, string) {
	t.Helper()
	root := t.TempDir()
	h := &saveAsHost{}
	e, warns := New(config.Default(), fakeLayouter{}, h, keymap.Profile{GUI: true, Mods: []keymap.Mod{keymap.ModSuper}})
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if err := e.OpenFolder(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	e.SetPaneSize(0, view.Size{W: 90, H: 28})
	pump(t, e, func() bool { return !e.ws.indexing })
	return e, h, root
}

// The window's save dialog opens once per document at a time, and its answer is applied to
// the document that asked, back on the editor's goroutine.
func TestSaveAsDialog(t *testing.T) {
	e, h, root := newDialogEditor(t)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "dialog")
	d := e.cur().doc
	e.Run("workbench.action.files.save")
	e.Run("workbench.action.files.save")
	if len(h.asked) != 1 || h.asked[0] != filepath.Join(root, "未命名-1.txt") {
		t.Fatalf("dialogs %v", h.asked)
	}
	p := filepath.Join(root, "notes.txt")
	go h.done[0](p, nil)
	pump(t, e, func() bool { return !d.untitled() || e.status != "" })
	if b, err := os.ReadFile(p); err != nil || string(b) != "dialog" || d.buf.Path() != p {
		t.Fatalf("file %q %v, status %q", b, err, e.status)
	}
}

// A document closed while its dialog is open is not written when the answer arrives.
func TestSaveAsDialogAfterClose(t *testing.T) {
	e, h, root := newDialogEditor(t)
	e.Run("workbench.action.files.newUntitledFile")
	typeText(e, "gone")
	d := e.cur().doc
	e.Run("workbench.action.files.saveAs")
	e.Run("workbench.action.closeActiveEditor")
	e.Run("workbench.action.closeActiveEditor")
	p := filepath.Join(root, "late.txt")
	h.done[0](p, nil)
	pump(t, e, func() bool { return !d.askingPath })
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("closed draft written: %v", err)
	}
}
