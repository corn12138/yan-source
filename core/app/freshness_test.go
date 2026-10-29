package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/view"
)

func writeFile(t *testing.T, path, s string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

// settle pumps background work for d and fails if cond turns true (used for "never happens").
func settle(t *testing.T, e *Editor, d time.Duration) {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case <-e.Wake():
			e.RunPending()
		case <-deadline:
			return
		}
	}
}

func TestWatcherReloadsCleanDocumentsAndKeepsDirtyOnes(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "one\n", "b.txt": "bee\n"})
	if e.ws.watcher == nil {
		t.Skip("no file events on this platform")
	}
	a, b := filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")
	if err := e.OpenFile(b); err != nil {
		t.Fatal(err)
	}
	typeText(e, "mine ") // b is dirty now
	if err := e.OpenFile(a); err != nil {
		t.Fatal(err)
	}
	writeFile(t, a, "two\n")
	pump(t, e, func() bool { return content(e) == "two\n" })
	if e.cur().doc.buf.Dirty() {
		t.Fatal("a reloaded document must be clean")
	}
	e.Run("undo") // the reload itself is one undoable step
	if content(e) != "one\n" {
		t.Fatalf("undo reload = %q", content(e))
	}
	e.Run("redo")

	writeFile(t, b, "theirs\n")
	pump(t, e, func() bool { return strings.Contains(e.status, "b.txt") })
	d := e.docOf(b)
	if s := d.buf.Snapshot(); s.String(0, s.Len()) != "mine bee\n" || !d.buf.Dirty() {
		t.Fatalf("dirty document overwritten: %q", s.String(0, s.Len()))
	}
	if got, _ := os.ReadFile(b); string(got) != "theirs\n" {
		t.Fatalf("external file lost: %q", got)
	}
}

func TestWatcherSeesChangesThatKeepSizeAndTime(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "abc\n"})
	if e.ws.watcher == nil {
		t.Skip("no file events on this platform")
	}
	a := filepath.Join(root, "a.txt")
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	os.Chtimes(a, old, old)
	if err := e.OpenFile(a); err != nil {
		t.Fatal(err)
	}
	writeFile(t, a, "xyz\n")
	os.Chtimes(a, old, old) // same size, same modification time
	pump(t, e, func() bool { return content(e) == "xyz\n" })
}

func TestWatcherHandlesDeletionAndRecreation(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "keep\n"})
	if e.ws.watcher == nil {
		t.Skip("no file events on this platform")
	}
	a := filepath.Join(root, "a.txt")
	if err := e.OpenFile(a); err != nil {
		t.Fatal(err)
	}
	os.Remove(a)
	pump(t, e, func() bool { return strings.Contains(e.status, "已从磁盘上删除") })
	if content(e) != "keep\n" {
		t.Fatalf("deleted file's text lost: %q", content(e))
	}
	pump(t, e, func() bool { return !slices.Contains(e.ws.index, "a.txt") })
	writeFile(t, a, "back\n")
	pump(t, e, func() bool { return content(e) == "back\n" })
	pump(t, e, func() bool { return slices.Contains(e.ws.index, "a.txt") })
}

func TestWatcherRefreshesTreeAndIndex(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"sub/x.txt": "x\n"})
	if e.ws.watcher == nil {
		t.Skip("no file events on this platform")
	}
	names := func() []string {
		e.refreshVisible()
		var out []string
		for _, n := range e.ws.visible {
			out = append(out, n.Rel)
		}
		return out
	}
	writeFile(t, filepath.Join(root, "new.txt"), "n\n")
	pump(t, e, func() bool { return slices.Contains(names(), "new.txt") })
	pump(t, e, func() bool { return slices.Contains(e.ws.index, "new.txt") })

	// An expanded subdirectory is watched too; a collapsed one is not read.
	e.ws.tree.Expand(e.ws.tree.Root.Children[0])
	e.watchTree()
	writeFile(t, filepath.Join(root, "sub", "y.txt"), "y\n")
	pump(t, e, func() bool { return slices.Contains(names(), "sub/y.txt") })
	os.Remove(filepath.Join(root, "new.txt"))
	pump(t, e, func() bool { return !slices.Contains(names(), "new.txt") })
}

func TestSearchResultsFollowDiskAndUnsavedEdits(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "hello\n", "b.txt": "nothing\n"})
	if e.ws.watcher == nil {
		t.Skip("no file events on this platform")
	}
	e.Run("workbench.action.findInFiles")
	typeText(e, "hello")
	var f view.Frame
	summary := func() string { e.Frame(&f); return f.Sidebar.Search.Summary }
	pump(t, e, func() bool { return summary() == "1 个文件中有 1 个结果" })

	writeFile(t, filepath.Join(root, "b.txt"), "hello hello\n")
	pump(t, e, func() bool { return summary() == "2 个文件中有 3 个结果" })
	os.Remove(filepath.Join(root, "a.txt"))
	pump(t, e, func() bool { return summary() == "1 个文件中有 2 个结果" })

	// An unsaved edit counts with the editor's text, not the file's.
	if err := e.OpenFile(filepath.Join(root, "b.txt")); err != nil {
		t.Fatal(err)
	}
	e.FocusColumn(0)
	typeText(e, "hello ")
	pump(t, e, func() bool { return summary() == "1 个文件中有 3 个结果" })
	if got, _ := os.ReadFile(filepath.Join(root, "b.txt")); string(got) != "hello hello\n" {
		t.Fatalf("search refresh wrote the file: %q", got)
	}
}

func TestLateDiskReadsAreDropped(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "one\n"})
	a := filepath.Join(root, "a.txt")
	if err := e.OpenFile(a); err != nil {
		t.Fatal(err)
	}
	d := e.cur().doc
	read := diskRead{doc: d, version: d.buf.Snapshot().Version(), stamp: d.buf.DiskStamp(), state: edit.DiskDeleted}
	typeText(e, "x") // the buffer moved on after the read began
	e.status = ""
	e.takeDiskReads([]diskRead{read})
	if e.status != "" || content(e) != "xone\n" {
		t.Fatalf("stale read applied: status %q, text %q", e.status, content(e))
	}
}

func TestDiskEventsOfAPreviousFolderAreDropped(t *testing.T) {
	e, rootA := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	rootB := t.TempDir()
	writeFile(t, filepath.Join(rootB, "a.txt"), "b\n")
	old := e.ws
	if err := e.SwitchFolder(rootB); err != nil {
		t.Fatal(err)
	}
	if err := e.OpenFile(filepath.Join(rootB, "a.txt")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(rootA, "a.txt"), "changed\n")
	settle(t, e, 400*time.Millisecond)
	if e.ws == old || content(e) != "b\n" {
		t.Fatalf("previous folder leaked into the new one: %q", content(e))
	}
	if old.watcher != nil && old.watcher.Add(rootA) {
		t.Fatal("the previous folder's watcher must be closed")
	}
}
