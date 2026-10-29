package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
)

// A long commit message shows its first lines; Enter on the fold shows all of it, and again
// folds it back.
func TestCommitMessageFolds(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a\n"})
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("b\n"), 0o644)
	var msg strings.Builder
	msg.WriteString("Change a\n\n")
	for i := 1; i <= 12; i++ {
		msg.WriteString("Line " + string(rune('A'+i-1)) + " of the body\n")
	}
	cmd := exec.Command("git", "-C", root, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qam", msg.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil })
	tb := e.showHistoryTabAt(e.git.repo.Root, "", gitOut(t, root, "rev-parse", "HEAD"), false)
	pump(t, e, func() bool { return !tb.diff.loading && len(tb.diff.secs) == 1 })
	txt := tabText(tb)
	if !strings.Contains(txt, "Line E of the body\n⋯ 说明还有 7 行") || strings.Contains(txt, "Line L of the body") {
		t.Fatalf("folded:\n%s", txt)
	}
	at := strings.Index(txt, "⋯ 说明还有")
	tb.doc.buf.SetSelections(edit.Single(edit.Cursor(at)))
	if err := e.diffOpen(tb); err != nil {
		t.Fatal(err)
	}
	txt = tabText(tb)
	if !strings.Contains(txt, "Line L of the body\n⋯ 收起说明") {
		t.Fatalf("open:\n%s", txt)
	}
	tb.doc.buf.SetSelections(edit.Single(edit.Cursor(strings.Index(txt, "⋯ 收起说明"))))
	e.diffOpen(tb)
	if strings.Contains(tabText(tb), "Line L of the body") {
		t.Fatalf("folded again:\n%s", tabText(tb))
	}
}

func TestSessionPathsRelative(t *testing.T) {
	for _, c := range []struct{ root, path, want string }{
		{"/w/p", "/w/p/a.txt", "a.txt"},
		{"/w/p", "/w/p/src/b.go", "src/b.go"},
		{"/w/p", "/w/other/c.txt", "/w/other/c.txt"},
		{"/w/p", "/w/p", "."},
		{"/w/p", "", ""},
	} {
		if got := sessionRel(filepath.FromSlash(c.root), filepath.FromSlash(c.path)); got != filepath.ToSlash(c.want) && got != filepath.FromSlash(c.want) {
			t.Errorf("sessionRel(%q, %q) = %q, want %q", c.root, c.path, got, c.want)
		}
	}
	if got := sessionAbs(filepath.FromSlash("/x/p"), "src/b.go"); got != filepath.FromSlash("/x/p/src/b.go") {
		t.Errorf("sessionAbs = %q", got)
	}
}

// The tabs of a folder are kept relative to it, so they come back after the folder moves, and
// the record of the old place goes.
func TestSessionFollowsAMovedFolder(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n", "src/b.txt": "b\n"})
	statePath := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(statePath))
	e.OpenFile(filepath.Join(root, "a.txt"))
	e.OpenFile(filepath.Join(root, "src", "b.txt"))
	e.SaveSession()
	st := config.LoadState(statePath)
	old := e.ws.root
	tabs := st.Sessions[old].Columns[0].Tabs
	if len(tabs) != 2 || tabs[0].Path != "a.txt" || tabs[1].Path != "src/b.txt" {
		t.Fatalf("saved %+v", tabs)
	}
	moved := filepath.Join(t.TempDir(), filepath.Base(root))
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	e2, _ := newFolderEditorAt(t, moved)
	e2.SetState(config.LoadState(statePath))
	if !e2.RestoreSession() {
		t.Fatal("no session after the move")
	}
	if got := tabNames(e2); len(got) != 2 || !strings.HasPrefix(got[1], "b.txt") {
		t.Fatalf("tabs %v", got)
	}
	if p := e2.cur().doc.buf.Path(); p != filepath.Join(e2.ws.root, "src", "b.txt") {
		t.Fatalf("opened %q", p)
	}
	e2.SaveSession()
	st = config.LoadState(statePath)
	if _, ok := st.Sessions[old]; ok {
		t.Fatal("the old place is still kept")
	}
	if _, ok := st.Sessions[e2.ws.root]; !ok {
		t.Fatal("the new place is not kept")
	}
}
