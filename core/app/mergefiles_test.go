package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/git"
)

// A merge that stops for conflicts lists its files by itself; a file applied in its merge view
// is staged and the next one in conflict opens; the resolved ones stand in the list after a
// restart, and the record goes when the merge is done.
func TestMergeFilesWorkedFileByFile(t *testing.T) {
	root := gitRepo(t, map[string]string{"one.txt": "a\nm\nz\n", "two.txt": "b\nn\ny\n"})
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("checkout", "-q", "-b", "side")
	os.WriteFile(filepath.Join(root, "one.txt"), []byte("a\nside\nz\n"), 0o644)
	os.WriteFile(filepath.Join(root, "two.txt"), []byte("b\nside\ny\n"), 0o644)
	run("commit", "-qam", "theirs")
	run("checkout", "-q", "main")
	os.WriteFile(filepath.Join(root, "one.txt"), []byte("a\nours\nz\n"), 0o644)
	os.WriteFile(filepath.Join(root, "two.txt"), []byte("b\nours\ny\n"), 0o644)
	run("commit", "-qam", "ours")
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil })
	if err := e.runOp(git.OpMerge, "合并中…", "done", func(r *git.Repo) error { return r.Merge("side") }); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.ov != nil && strings.HasPrefix(e.ov.title, "冲突文件") })
	if e.ov.title != "冲突文件：还有 2 个没解决" || len(e.ov.choices) != 2 || e.ov.choices[0].label != "● one.txt" {
		t.Fatalf("list: %q %+v", e.ov.title, e.ov.choices)
	}
	chooseItem(t, e, "● one.txt")
	mv := e.cur().merge
	if mv == nil {
		t.Fatalf("no merge view (%q)", e.status)
	}
	e.mergeTake(mv, 0, true)
	if err := e.mergeApplyAndNext(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool {
		return e.cur() != nil && e.cur().merge != nil && filepath.Base(e.cur().merge.path) == "two.txt"
	})
	if _, err := os.Stat(filepath.Join(e.git.repo.Dir, mergeRecordName)); err != nil {
		t.Fatalf("no record: %v", err)
	}
	pump(t, e, func() bool { en, ok := e.git.entries[filepath.Join(root, "one.txt")]; return !ok || !en.Conflicted() })

	// After a restart the list still has one.txt resolved.
	e2 := newEditorIn(t, root)
	pump(t, e2, func() bool { return e2.git.status != nil && e2.git.status.Op != "" })
	files := e2.mergeFiles()
	if len(files) != 2 || files[0].path != filepath.Join(root, "two.txt") || files[0].resolved || !files[1].resolved {
		t.Fatalf("after restart: %+v", files)
	}
	e2.showMergeFiles()
	if e2.ov.title != "冲突文件：还有 1 个没解决" || e2.ov.choices[1].label != "✓ one.txt" {
		t.Fatalf("list after restart: %q %+v", e2.ov.title, e2.ov.choices)
	}
	e2.closeOverlay()

	// Resolve two.txt, continue: the record goes.
	mv = e.cur().merge
	e.mergeTake(mv, 0, false)
	if err := e.mergeApplyAndNext(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { en, ok := e.git.entries[filepath.Join(root, "two.txt")]; return !ok || !en.Conflicted() })
	if err := e.continueOp(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.status != nil && e.git.status.Op == "" })
	if _, err := os.Stat(filepath.Join(e.git.repo.Dir, mergeRecordName)); !os.IsNotExist(err) {
		t.Fatalf("record left: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "one.txt")); string(b) != "a\nside\nz\n" {
		t.Fatalf("one.txt %q", b)
	}
}

// A resolved file goes back in conflict from the list, and out of the record.
func TestMergeFilesRestoreResolved(t *testing.T) {
	e, root := mergeConflict(t, map[string][3]string{"a.txt": {"x\n", "ours\n", "theirs\n"}, "b.txt": {"y\n", "OURS\n", "THEIRS\n"}})
	path := filepath.Join(root, "a.txt")
	e.openMerge(path)
	e.mergeTake(e.cur().merge, 0, false)
	if err := e.mergeMarkResolved(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { en, ok := e.git.entries[path]; return !ok || !en.Conflicted() })
	e.showMergeFiles()
	chooseItem(t, e, "✓ a.txt")
	chooseItem(t, e, "恢复成冲突")
	chooseItem(t, e, "恢复冲突")
	pump(t, e, func() bool { en, ok := e.git.entries[path]; return ok && en.Conflicted() })
	for _, f := range e.mergeFiles() {
		if f.resolved {
			t.Fatalf("still resolved: %+v", e.mergeFiles())
		}
	}
}
