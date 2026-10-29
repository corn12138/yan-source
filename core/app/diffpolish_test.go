package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/view"
)

func TestStatBar(t *testing.T) {
	for _, c := range []struct{ add, del, a, d int }{
		{0, 0, 0, 0}, {1, 0, 1, 0}, {0, 3, 0, 3}, {1, 1, 1, 1}, {10, 0, 5, 0}, {100, 1, 4, 1}, {1, 100, 1, 4}, {3, 2, 3, 2}, {6, 4, 3, 2},
	} {
		if a, d := statBar(c.add, c.del); a != c.a || d != c.d {
			t.Errorf("statBar(%d, %d) = %d, %d; want %d, %d", c.add, c.del, a, d, c.a, c.d)
		}
	}
}

func goChanges(t *testing.T) (*Editor, string) {
	t.Helper()
	var body strings.Builder
	body.WriteString("package main\n\nfunc main() {\n")
	for i := 0; i < 40; i++ {
		body.WriteString("\tprintln(\"line\")\n")
	}
	body.WriteString("}\n")
	root := gitRepo(t, map[string]string{"main.go": body.String(), "notes.txt": "a\nb\n"})
	changed := strings.Replace(body.String(), "\tprintln(\"line\")\n", "\tprintln(\"first\")\n", 1)
	changed = strings.Replace(changed, "}\n", "\tprintln(\"end\")\n}\n", 1)
	os.WriteFile(filepath.Join(root, "main.go"), []byte(changed), 0o644)
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("a\nB\n"), 0o644)
	gitOut(t, root, "add", "notes.txt")
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 2 })
	return e, root
}

// The headers of All Changes say each file's status and show a bar of what it added and
// removed; the @@ headers of code name the block the change is in.
func TestDiffFileHeadersAndScopes(t *testing.T) {
	e, _ := goChanges(t)
	e.openAllChanges()
	pump(t, e, func() bool { return len(diffHeads(e.cur().diff)) == 3 })
	txt := content(e)
	for _, want := range []string{"▾ M notes.txt  +1 −1  已暂存  ■■■■■", "▾ M main.go  +2 −1  更改  ■■■■■", "@@ -4,0 +4,1 @@ func main()", "@@ -43,1 +44,1 @@ func main()"} {
		if !strings.Contains(txt, want) {
			t.Fatalf("missing %q in\n%s", want, txt)
		}
	}
	if !strings.Contains(txt, "已暂存  ■■■■■\na\n@@ -2,1 +2,1 @@\n") {
		t.Fatalf("a text file got a scope:\n%s", txt)
	}
	var f view.Frame
	e.SetPaneSize(0, view.Size{W: 120, H: 30})
	e.Frame(&f)
	var bar []style.ID
	for _, r := range f.Columns[0].Pane.Rows {
		if strings.Contains(string(r.Text), "main.go  +2 −1") {
			for _, sp := range r.Spans {
				bar = append(bar, sp.Style)
			}
			if r.Block != view.DiffBlockFile {
				t.Fatalf("main.go header block %v", r.Block)
			}
		}
		if strings.Contains(string(r.Text), "notes.txt  +1 −1") && r.Block != view.DiffBlockFileStaged {
			t.Fatalf("notes.txt header block %v", r.Block)
		}
	}
	// +2 −1 over 3 lines: two cells added, one removed, two empty.
	if len(bar) != 5 || bar[0] != style.Inserted || bar[1] != style.Inserted || bar[2] != style.Deleted || bar[3] != style.Dim || bar[4] != style.Dim {
		t.Fatalf("bar spans %v", bar)
	}
}

// Show Full Files folds nothing, and folds again when turned off.
func TestDiffFullFiles(t *testing.T) {
	e, root := goChanges(t)
	e.openChange(filepath.Join(root, "main.go"), false)
	if !strings.Contains(content(e), "行没改") {
		t.Fatalf("folded:\n%s", content(e))
	}
	e.Run("yan.diff.toggleFullFiles")
	if txt := content(e); strings.Contains(txt, "行没改") || strings.Count(txt, "println(\"line\")") < 39 {
		t.Fatalf("full:\n%s", txt)
	}
	e.Run("yan.diff.toggleFullFiles")
	if !strings.Contains(content(e), "行没改") {
		t.Fatalf("folded again:\n%s", content(e))
	}
}

// A file's header scrolled away stays pinned at the top with its buttons, and a click on it
// goes back to it; the @@ rows offer staging a change, and unstaging a staged one.
func TestDiffStickyHeaderAndHunkButtons(t *testing.T) {
	e, root := goChanges(t)
	e.SetPaneSize(0, view.Size{W: 120, H: 10})
	e.openAllChanges()
	pump(t, e, func() bool { return len(diffHeads(e.cur().diff)) == 3 })
	tb := e.cur()
	head := -1
	for i, l := range tb.diff.lines {
		if l.kind == diffFile && tb.diff.secs[l.sec].path == filepath.Join(root, "main.go") {
			head = i
		}
	}
	tb.scrollY = head + 3
	var f view.Frame
	e.Frame(&f)
	st := f.Columns[0].Pane.Sticky
	if len(st) != 1 || !strings.Contains(string(st[0].Text), "main.go") || st[0].Block != view.DiffBlockFile || st[0].Num != -1 {
		t.Fatalf("sticky %+v", st)
	}
	e.Handle(MouseInput{Column: 0, X: 10, Y: 0, Kind: MousePress})
	if tb.scrollY != head {
		t.Fatalf("click on the pinned header: scroll %d, want %d", tb.scrollY, head)
	}
	// Stage the first change of main.go from its @@ row, then unstage notes.txt's.
	hunkOf := func(name string, group changeGroup) int {
		for i, l := range tb.diff.lines {
			if l.kind == diffHeader && l.hunk == 0 && l.sec >= 0 && tb.diff.secs[l.sec].path == filepath.Join(root, name) && tb.diff.secs[l.sec].group == group {
				return i
			}
		}
		t.Fatalf("no first change of %s", name)
		return -1
	}
	e.DiffBlockAt(0, hunkOf("main.go", changeUnstaged), "stage")
	pump(t, e, func() bool { return strings.Contains(gitOut(t, root, "diff", "--cached", "--", "main.go"), "first") })
	if cached := gitOut(t, root, "diff", "--cached", "--", "main.go"); strings.Contains(cached, "end") {
		t.Fatalf("staged both changes:\n%s", cached)
	}
	pump(t, e, func() bool { // the tab reads main.go's staged change too
		e.Frame(&f)
		for _, sec := range tb.diff.secs {
			if sec.path == filepath.Join(root, "main.go") && sec.group == changeStaged {
				return true
			}
		}
		return false
	})
	e.DiffBlockAt(0, hunkOf("notes.txt", changeStaged), "unstage")
	pump(t, e, func() bool { return gitOut(t, root, "diff", "--cached", "--", "notes.txt") == "" })
}
