package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/keymap"
)

func twoChangedFiles(t *testing.T) (*Editor, string) {
	t.Helper()
	root := gitRepo(t, map[string]string{"a.txt": "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n", "b.txt": "four\nfive\nsix\n"})
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("ONE\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nTEN\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("four\n6\nsix\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 2 })
	return e, root
}

func diffLineText(e *Editor) string {
	s := e.cur().doc.buf.Snapshot()
	i := s.LineAt(e.cur().doc.buf.Selections().Main().Head)
	return strings.TrimSpace(snapshotLine(s, i))
}

func press(e *Editor, r rune) { e.Handle(TextInput{Text: string(r)}) }

// In the tab of all the changes, ] goes from change to change; at the last change of a file
// it says so and crosses on one more press; at the end of the diff it says so and stays. [ does
// the same backwards, and any other move disarms the crossing.
func TestDiffChangeNavigationBoundaries(t *testing.T) {
	e, _ := twoChangedFiles(t)
	if err := e.openAllChanges(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return len(diffHeads(e.cur().diff)) == 3 })
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(0)))
	steps := []struct {
		key          rune
		line, status string
	}{
		{']', "@@ -1,1 +1,1 @@", "第 1/3 处改动 · a.txt"},
		{']', "@@ -10,1 +10,1 @@", "第 2/3 处改动 · a.txt"},
		{']', "@@ -10,1 +10,1 @@", "已是 a.txt 的最后一处改动，再按一次到 b.txt"},
		{']', "@@ -2,1 +2,1 @@", "第 3/3 处改动 · b.txt"},
		{']', "@@ -2,1 +2,1 @@", "已是改动视图的最后一处改动"},
		{']', "@@ -2,1 +2,1 @@", "已是改动视图的最后一处改动"},
		{'[', "@@ -2,1 +2,1 @@", "已是 b.txt 的第一处改动，再按一次到 a.txt"},
		{'[', "@@ -10,1 +10,1 @@", "第 2/3 处改动 · a.txt"},
		{'[', "@@ -1,1 +1,1 @@", "第 1/3 处改动 · a.txt"},
		{'[', "@@ -1,1 +1,1 @@", "已是改动视图的第一处改动"},
	}
	for i, s := range steps {
		press(e, s.key)
		if got := diffLineText(e); got != s.line || e.status != s.status {
			t.Fatalf("step %d %c: on %q, status %q; want %q, %q", i, s.key, got, e.status, s.line, s.status)
		}
	}
	// Disarmed by another move: ] to a's end, j, k, ] says it again rather than crossing.
	press(e, ']')
	press(e, ']')
	press(e, 'j')
	press(e, 'k')
	press(e, ']')
	if got := diffLineText(e); got != "@@ -10,1 +10,1 @@" || !strings.HasPrefix(e.status, "已是 a.txt") {
		t.Fatalf("after a move: on %q, status %q", got, e.status)
	}
	// File navigation says where it is at the ends.
	press(e, '.')
	press(e, '.')
	if e.status != "已是最后一个文件" {
		t.Fatalf("file end: %q", e.status)
	}
}

// [diff] cross_files: always crosses at once, never stays in the file.
func TestDiffChangeNavigationCrossSetting(t *testing.T) {
	e, _ := twoChangedFiles(t)
	e.openAllChanges()
	pump(t, e, func() bool { return len(diffHeads(e.cur().diff)) == 3 })
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(0)))
	e.cfg.DiffCrossFiles = "always"
	press(e, ']')
	press(e, ']')
	press(e, ']')
	if got := diffLineText(e); got != "@@ -2,1 +2,1 @@" {
		t.Fatalf("always: on %q (%q)", got, e.status)
	}
	e.cfg.DiffCrossFiles = "never"
	press(e, '[')
	press(e, '[')
	if got := diffLineText(e); got != "@@ -2,1 +2,1 @@" || e.status != "已是 b.txt 的第一处改动" {
		t.Fatalf("never: on %q (%q)", got, e.status)
	}
}

// A one-file diff goes on to the next changed file of Source Control, after saying so, and
// the commands do what ] and [ do; Enter on a removed line opens the file at its change.
func TestOneFileDiffCrossesToTheNextFile(t *testing.T) {
	e, root := twoChangedFiles(t)
	if err := e.openChange(filepath.Join(root, "a.txt"), false); err != nil {
		t.Fatal(err)
	}
	e.Run("workbench.action.compareEditor.nextChange") // the tab opened on the first change
	if got := diffLineText(e); got != "@@ -10,1 +10,1 @@" {
		t.Fatalf("on %q (%q)", got, e.status)
	}
	e.Run("workbench.action.editor.nextChange") // Alt+F5 in a diff: the diff's changes
	if e.status != "已是 a.txt 的最后一处改动，再按一次到 b.txt" {
		t.Fatalf("status %q", e.status)
	}
	e.Run("workbench.action.compareEditor.nextChange")
	if dt := e.cur().diff; dt == nil || dt.path != filepath.Join(root, "b.txt") || diffLineText(e) != "@@ -2,1 +2,1 @@" {
		t.Fatalf("crossed to %v on %q", tabNames(e), diffLineText(e))
	}
	press(e, ']')
	if e.status != "已是改动视图的最后一处改动" {
		t.Fatalf("end: %q", e.status)
	}
	e.Run("yan.diff.previousFile")
	if dt := e.cur().diff; dt == nil || dt.path != filepath.Join(root, "a.txt") {
		t.Fatalf("previous file: %v", tabNames(e))
	}
	e.Run("yan.diff.previousFile")
	if e.status != "已是第一个文件" {
		t.Fatalf("first file: %q", e.status)
	}
	// Enter on the removed "one" lands on line 1 of a.txt.
	txt := content(e)
	at := strings.Index(txt, "\none\n") + 1
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(at)))
	e.Handle(key(keymap.KeyEnter, 0))
	if p := e.cur().doc.buf.Path(); p != filepath.Join(root, "a.txt") || e.cur().diff != nil {
		t.Fatalf("opened %q (%q)", p, e.status)
	}
	if s := e.cur().doc.buf.Snapshot(); s.LineAt(e.cur().doc.buf.Selections().Main().Head) != 0 {
		t.Fatal("should land on the change's line")
	}
}
