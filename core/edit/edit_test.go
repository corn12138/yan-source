package edit

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/text"
)

func load(t *testing.T, p string) *Buffer {
	t.Helper()
	s, err := text.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return NewBuffer(s)
}

func insert(b *Buffer, at int, s string) {
	b.Apply(Transaction{Changes: text.ChangeSet{{Start: at, End: at, Text: s}}, Origin: OriginCommand})
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// 外部程序改文件：时间戳往后挪，保证看得出来。
func writeLater(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	os.Chtimes(p, later, later)
}

func TestSaveRefusesExternalChange(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(p, []byte("base\n"), 0o644)
	b := load(t, p)
	insert(b, 0, "local ")
	writeLater(t, p, "external change\n")
	if b.DiskState() != DiskChanged {
		t.Fatal("external write not detected")
	}
	if err := b.Save(); !errors.Is(err, ErrConflict) {
		t.Fatalf("Save = %v, want ErrConflict", err)
	}
	if got := read(t, p); got != "external change\n" || !b.Dirty() {
		t.Fatalf("disk %q dirty %v: both versions must survive", got, b.Dirty())
	}
	if err := b.Overwrite(); err != nil {
		t.Fatal(err)
	}
	if got := read(t, p); got != "local base\n" || b.Dirty() || b.DiskState() != DiskSame {
		t.Fatalf("after overwrite disk %q dirty %v", got, b.Dirty())
	}
	// 保存之后再编辑、再保存：没人动过文件，照常保存
	insert(b, 0, "x")
	if err := b.Save(); err != nil || read(t, p) != "xlocal base\n" {
		t.Fatalf("plain save after overwrite: %v", err)
	}
}

func TestSaveSameSizeExternalChange(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(p, []byte("cat\n"), 0o644)
	b := load(t, p)
	insert(b, 0, "x")
	writeLater(t, p, "dog\n") // 大小不变，只有修改时间变
	if err := b.Save(); !errors.Is(err, ErrConflict) {
		t.Fatalf("Save = %v", err)
	}
}

func TestSaveDeletedAndRecreated(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(p, []byte("base\n"), 0o644)
	b := load(t, p)
	insert(b, 0, "local ")
	os.Remove(p)
	if b.DiskState() != DiskDeleted {
		t.Fatal("deletion not detected")
	}
	// 被删了：磁盘上没东西可丢，保存就是重新建出来
	if err := b.Save(); err != nil || read(t, p) != "local base\n" {
		t.Fatalf("save after delete: %v", err)
	}
	// 删了又被别人建出来：不是我们存的那个文件了
	insert(b, 0, "y")
	os.Remove(p)
	writeLater(t, p, "someone else\n")
	if err := b.Save(); !errors.Is(err, ErrConflict) || read(t, p) != "someone else\n" {
		t.Fatalf("recreated file: %v", err)
	}
}

func TestNewFileCreatedElsewhere(t *testing.T) {
	p := filepath.Join(t.TempDir(), "new.txt")
	b := load(t, p) // 还不存在
	insert(b, 0, "mine\n")
	writeLater(t, p, "theirs\n")
	if err := b.Save(); !errors.Is(err, ErrConflict) || read(t, p) != "theirs\n" {
		t.Fatalf("Save = %v", err)
	}
}

func TestReloadAdoptsDiskVersion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	os.WriteFile(p, []byte("base\n"), 0o644)
	b := load(t, p)
	writeLater(t, p, "new\n")
	s, _ := text.Load(p)
	b.Reload(s)
	if b.DiskState() != DiskSame || b.Dirty() {
		t.Fatal("reload should adopt the disk version")
	}
	insert(b, 0, "x")
	if err := b.Save(); err != nil || read(t, p) != "xnew\n" {
		t.Fatalf("save after reload: %v", err)
	}
}

func writePreservingTime(t *testing.T, p, value string) {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(value)) != info.Size() {
		t.Fatal("fixture must preserve byte size")
	}
	if err := os.WriteFile(p, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

func TestSaveConflictWithPreservedMetadata(t *testing.T) {
	for _, prefix := range []string{"", "\xef\xbb\xbf"} {
		t.Run(prefix, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "a.txt")
			if err := os.WriteFile(p, []byte(prefix+"before\r\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			b := load(t, p)
			insert(b, 0, "local ")
			writePreservingTime(t, p, prefix+"AFTER!\r\n")
			if st, err := StatStamp(p); err != nil || st != b.DiskStamp() {
				t.Fatalf("metadata differs: %v %v", st, err)
			}
			if b.DiskState() != DiskChanged {
				t.Fatal("missed content-only external change")
			}
			if err := b.Save(); !errors.Is(err, ErrConflict) {
				t.Fatalf("save: %v", err)
			}
			if read(t, p) != prefix+"AFTER!\r\n" || !b.Dirty() {
				t.Fatal("both versions must survive")
			}
			if err := b.Overwrite(); err != nil {
				t.Fatal(err)
			}
			if read(t, p) != prefix+"local before\r\n" || b.Dirty() || b.DiskState() != DiskSame {
				t.Fatal("explicit overwrite did not establish baseline")
			}
			insert(b, 0, "again ")
			if err := b.Save(); err != nil {
				t.Fatal(err)
			}
			if read(t, p) != prefix+"again local before\r\n" {
				t.Fatal("ordinary save lost encoding or content")
			}
		})
	}
}

func TestReloadUndoKeepsDiskBaseline(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(p, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := load(t, p)
	writePreservingTime(t, p, "AFTER!\n")
	s, err := text.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	b.Reload(s)
	if b.Dirty() || b.DiskState() != DiskSame {
		t.Fatal("reload must adopt external version")
	}
	if _, ok := b.Undo(); !ok {
		t.Fatal("reload must be undoable")
	}
	if !b.Dirty() || b.DiskState() != DiskSame || b.Snapshot().String(0, b.Snapshot().Len()) != "before\n" {
		t.Fatal("undo must restore local text without changing disk baseline")
	}
	if _, ok := b.Redo(); !ok || b.Dirty() {
		t.Fatal("redo must return to saved state")
	}
	b.Undo()
	if err := b.Save(); err != nil {
		t.Fatal(err)
	}
	if read(t, p) != "before\n" || b.Dirty() {
		t.Fatal("explicitly saving an undone reload must work")
	}
}

func TestSaveDoesNotIgnoreUnreadableBaseline(t *testing.T) {
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("POSIX permissions required")
	}
	p := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(p, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := load(t, p)
	insert(b, 0, "local ")
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(p, 0o600) })
	if err := b.Save(); err == nil {
		t.Fatal("unreadable baseline must not authorize overwrite")
	}
	if err := os.Chmod(p, 0o600); err != nil {
		t.Fatal(err)
	}
	if read(t, p) != "before\n" || !b.Dirty() {
		t.Fatal("failed verification must keep both versions")
	}
}

// Save As writes first and only then takes the new path: a failed write leaves an
// untitled buffer untitled and dirty, and its undo history survives a success.
func TestSaveAs(t *testing.T) {
	dir := t.TempDir()
	b := NewBuffer(text.FromString("", text.Meta{}))
	insert(b, 0, "draft")
	if err := b.SaveAs(filepath.Join(dir, "missing", "a.txt"), Stamp{}); err == nil || b.Path() != "" || !b.Dirty() {
		t.Fatalf("failed write: err=%v path=%q dirty=%v", err, b.Path(), b.Dirty())
	}
	p := filepath.Join(dir, "a.txt")
	if err := b.SaveAs(p, Stamp{}); err != nil {
		t.Fatal(err)
	}
	if b.Path() != p || b.Dirty() || read(t, p) != "draft" || !b.Exists() {
		t.Fatalf("path=%q dirty=%v disk=%q", b.Path(), b.Dirty(), read(t, p))
	}
	b.Undo()
	if b.Snapshot().String(0, b.Snapshot().Len()) != "" || !b.Dirty() {
		t.Fatal("undo history lost by Save As")
	}
	b.Redo()
	insert(b, 5, "!")
	if err := b.Save(); err != nil || read(t, p) != "draft!" {
		t.Fatalf("save after Save As: %v %q", err, read(t, p))
	}
}

// The target is written only if it is still what it was when it was chosen.
func TestSaveAsRefusesChangedTarget(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	b := NewBuffer(text.FromString("", text.Meta{}))
	insert(b, 0, "mine")
	writeLater(t, p, "appeared")
	if err := b.SaveAs(p, Stamp{}); !errors.Is(err, ErrConflict) || read(t, p) != "appeared" || b.Path() != "" {
		t.Fatalf("created over a new file: %v %q", err, read(t, p))
	}
	chosen, err := StatStamp(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SaveAs(p, chosen); err != nil || read(t, p) != "mine" {
		t.Fatalf("confirmed replace: %v %q", err, read(t, p))
	}
	o := NewBuffer(text.FromString("", text.Meta{}))
	insert(o, 0, "other")
	writeLater(t, p, "changed again")
	if err := o.SaveAs(p, chosen); !errors.Is(err, ErrConflict) || read(t, p) != "changed again" {
		t.Fatalf("replaced a file changed after confirmation: %v", err)
	}
}

// Recovered text is unsaved from the start and stays unsaved through undo.
func TestNewUnsaved(t *testing.T) {
	b := NewUnsaved(text.FromString("kept", text.Meta{}))
	if !b.Dirty() {
		t.Fatal("recovered text reads as saved")
	}
	insert(b, 4, "!")
	b.Undo()
	if !b.Dirty() || b.Snapshot().String(0, b.Snapshot().Len()) != "kept" {
		t.Fatal("undo made recovered text saved or changed it")
	}
}
