package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// In All Changes a file of more than 2000 changed lines starts folded, and one of more than a
// megabyte is not even read until it is unfolded.
func TestBigFilesFoldAndReadLazily(t *testing.T) {
	lines := func(n int, word string) string { return strings.Repeat(word+" line of some text, long enough\n", n) }
	root := gitRepo(t, map[string]string{"small.txt": "a\n", "big.txt": lines(2500, "old"), "huge.txt": lines(40000, "same")})
	os.WriteFile(filepath.Join(root, "small.txt"), []byte("b\n"), 0o644)
	os.WriteFile(filepath.Join(root, "big.txt"), []byte(lines(2500, "new")), 0o644)
	os.WriteFile(filepath.Join(root, "huge.txt"), []byte("changed\n"+lines(39999, "same")), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 3 })
	e.openAllChanges()
	tb := e.cur()
	pump(t, e, func() bool { return !tb.diff.loading && len(tb.diff.secs) == 3 })
	sec := func(name string) (int, *changeSec) {
		for i := range tb.diff.secs {
			if filepath.Base(tb.diff.secs[i].path) == name {
				return i, &tb.diff.secs[i]
			}
		}
		t.Fatalf("no %s", name)
		return -1, nil
	}
	txt := content(e)
	if !strings.Contains(txt, "▸ M big.txt  +2500 −2500") || !strings.Contains(txt, "▾ M small.txt") {
		t.Fatalf("headers:\n%s", txt)
	}
	hi, huge := sec("huge.txt")
	if !huge.lazy || huge.cur != nil || !strings.Contains(txt, "▸ M huge.txt  大文件（") || !strings.Contains(txt, " MB）：展开时读取") {
		t.Fatalf("huge read eagerly (lazy %v):\n%s", huge.lazy, txt)
	}
	e.toggleChangesFile(tb, hi)
	pump(t, e, func() bool { _, s := sec("huge.txt"); return !tb.diff.loading && !s.lazy && len(s.hunks) > 0 })
	if txt := content(e); !strings.Contains(txt, "▾ M huge.txt  +1 −1") {
		t.Fatalf("after unfolding:\n%s", txt[:min(len(txt), 400)])
	}
	// Unfolded, big.txt stays as the user left it when the changes are read again.
	bi, _ := sec("big.txt")
	e.toggleChangesFile(tb, bi)
	tb.diff.statusOf = 0 // as when Git's status changes
	e.loadChanges(tb)
	pump(t, e, func() bool { return !tb.diff.loading })
	if txt := content(e); !strings.Contains(txt, "▾ M big.txt") || !strings.Contains(txt, "▾ M huge.txt") {
		t.Fatalf("read again:\n%s", txt[:min(len(txt), 400)])
	}
}

// A repository made in the folder from outside is found by the folder's watcher, without
// waiting for a timer.
func TestRepositoryFoundWhenItAppears(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("没有 git")
	}
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	pump(t, e, func() bool { return e.git.ready })
	if e.git.repo != nil {
		t.Fatal("a repository before git init")
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	pump(t, e, func() bool { return e.git.repo != nil })
}
