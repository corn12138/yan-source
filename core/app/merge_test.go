package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/view"
)

// mergeConflict makes a repository where merging side into main stops for a conflict of each
// file of files (path → base, ours, theirs), and opens it.
func mergeConflict(t *testing.T, files map[string][3]string) (*Editor, string) {
	t.Helper()
	base := map[string]string{}
	for p, v := range files {
		base[p] = v[0]
	}
	root := gitRepo(t, base)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil && args[0] != "merge" {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(k int) {
		for p, v := range files {
			os.WriteFile(filepath.Join(root, p), []byte(v[k]), 0o644)
		}
	}
	git("checkout", "-q", "-b", "side")
	write(2)
	git("commit", "-qam", "theirs")
	git("checkout", "-q", "main")
	write(1)
	git("commit", "-qam", "ours")
	git("merge", "-q", "side")
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && e.git.status.Op != "" })
	return e, root
}

func resultText(mv *mergeView) string { return docText(mv.result.doc) }

// The merge view opens as three columns: ours, the result starting from the base with the
// changes no conflict holds already in, and theirs; the merge does not go on while a conflict
// stands.
func TestMergeViewOpens(t *testing.T) {
	e, root := mergeConflict(t, map[string][3]string{"shared.txt": {"top\na\nm\nz\n", "TOP\na\nours\nz\n", "top\na\nside\nz\n"}})
	path := filepath.Join(root, "shared.txt")
	if err := e.openMerge(path); err != nil {
		t.Fatal(err)
	}
	mv := e.cur().merge
	if mv == nil || len(e.cols) != 3 || e.col != 1 {
		t.Fatalf("columns %d, focus %d, merge %v", len(e.cols), e.col, mv)
	}
	if got := resultText(mv); got != "TOP\na\nm\nz\n" {
		t.Fatalf("result %q", got)
	}
	if mv.oursName != "main" || mv.theirsName != "side" {
		t.Fatalf("names %q %q", mv.oursName, mv.theirsName)
	}
	if n := mv.unresolved(); n != 1 || !strings.Contains(e.status, "还有 1 处没处理") {
		t.Fatalf("unresolved %d, status %q", n, e.status)
	}
	if got := filepath.Base(e.cols[0].tab().doc.buf.Path()); got != "shared.txt · 我们的" {
		t.Fatalf("left %q", got)
	}
	if err := e.continueOp(); err == nil {
		t.Fatal("the merge went on with a conflict")
	}
	// Opening again only shows it.
	e.FocusColumn(0)
	if err := e.openMerge(path); err != nil || len(e.cols) != 3 || e.cur() != mv.result {
		t.Fatalf("reopen: %v, columns %d", err, len(e.cols))
	}
}

// Taking a side, both, ignoring, resolving all, and undo and redo of each.
func TestMergeViewChoicesAndUndo(t *testing.T) {
	e, root := mergeConflict(t, map[string][3]string{"shared.txt": {"top\nm\nz\nend\n", "OURS\nm\nz\nOURS2\n", "SIDE\nm\nz\nSIDE2\n"}})
	if err := e.openMerge(filepath.Join(root, "shared.txt")); err != nil {
		t.Fatal(err)
	}
	mv := e.cur().merge
	if len(mv.conflicts) != 2 {
		t.Fatalf("conflicts %d", len(mv.conflicts))
	}
	e.mergeTake(mv, 0, false)
	if got := resultText(mv); got != "OURS\nm\nz\nend\n" || mv.state(0) != regionOurs || mv.unresolved() != 1 {
		t.Fatalf("take ours: %q %v", got, mv.state(0))
	}
	e.mergeTake(mv, 0, true)
	if got := resultText(mv); got != "OURS\nSIDE\nm\nz\nend\n" || mv.state(0) != regionBoth {
		t.Fatalf("both: %q %v", got, mv.state(0))
	}
	e.Run("undo")
	if got := resultText(mv); got != "OURS\nm\nz\nend\n" || mv.state(0) != regionOurs {
		t.Fatalf("undo both: %q", got)
	}
	e.mergeIgnore(mv, 0, false) // ours out again, ignored
	if got := resultText(mv); got != "top\nm\nz\nend\n" || mv.state(0) != regionIgnored || mv.unresolved() != 1 {
		t.Fatalf("ignore ours: %q %v", got, mv.state(0))
	}
	e.mergeIgnore(mv, 0, true) // no text, still ignored
	e.Run("undo")
	if mv.state(0) != regionIgnored {
		t.Fatalf("undo the empty ignore: %v", mv.state(0))
	}
	e.Run("undo")
	if mv.state(0) != regionOurs {
		t.Fatalf("undo the ignore of ours: %v %q", mv.state(0), resultText(mv))
	}
	e.Run("redo")
	if mv.state(0) != regionIgnored {
		t.Fatalf("redo: %v", mv.state(0))
	}
	// An empty ignore from the base handles a conflict, and undo takes it back.
	e.mergeIgnore(mv, 1, true)
	if mv.state(1) != regionIgnored || mv.unresolved() != 0 {
		t.Fatalf("ignore from the base: %v", mv.state(1))
	}
	e.Run("undo")
	if mv.state(1) != regionUndecided {
		t.Fatalf("undo ignore from the base: %v", mv.state(1))
	}
	e.mergeResolveUsing(mv, true)
	if got := resultText(mv); got != "SIDE\nm\nz\nSIDE2\n" || mv.unresolved() != 0 {
		t.Fatalf("resolve using theirs: %q", got)
	}
	e.Run("undo")
	if mv.state(0) != regionIgnored || mv.state(1) != regionUndecided {
		t.Fatalf("undo resolve: %v %v", mv.state(0), mv.state(1))
	}
	// Typing in the result undoes as anywhere, and the regions follow the text.
	d := mv.result.doc
	d.buf.SetSelections(edit.Single(edit.Cursor(0)))
	e.FocusColumn(1)
	typeText(e, "// note\n")
	e.mergeSync(mv)
	if !strings.HasPrefix(resultText(mv), "// note\n") || mv.ranges[mv.conflicts[1]][0] != 4 {
		t.Fatalf("typing: %q %v", resultText(mv), mv.ranges)
	}
	e.Run("undo")
	if strings.HasPrefix(resultText(mv), "// note") {
		t.Fatalf("undo typing: %q", resultText(mv))
	}
}

// The sides' keys and buttons take and ignore, ] and [ go between conflicts, and the rows
// show how each conflict stands.
func TestMergeViewSides(t *testing.T) {
	e, root := mergeConflict(t, map[string][3]string{"shared.txt": {"top\nm\nz\nend\n", "OURS\nm\nz\nOURS2\n", "SIDE\nm\nz\nSIDE2\n"}})
	if err := e.openMerge(filepath.Join(root, "shared.txt")); err != nil {
		t.Fatal(err)
	}
	mv := e.cur().merge
	e.FocusColumn(2) // theirs
	press(e, ']')
	if !strings.HasPrefix(e.status, "冲突 2/2") {
		t.Fatalf("next: %q", e.status)
	}
	press(e, 'a')
	if got := resultText(mv); got != "top\nm\nz\nSIDE2\n" {
		t.Fatalf("take theirs by key: %q (%q)", got, e.status)
	}
	var f view.Frame
	e.Frame(&f)
	blocks := map[int32]view.DiffBlock{}
	for _, r := range f.Columns[0].Pane.Rows {
		blocks[r.Line] = r.Block
	}
	if blocks[1] != view.DiffBlockMerge || blocks[4] != view.DiffBlockMerge || blocks[2] != view.DiffBlockNone {
		t.Fatalf("ours rows: %v", blocks)
	}
	e.MergeBlockAt(0, 1, "take")
	if got := resultText(mv); got != "OURS\nm\nz\nSIDE2\n" || mv.unresolved() != 0 {
		t.Fatalf("ours button: %q", got)
	}
	e.MergeBlockAt(0, 4, "ignore")
	if got := resultText(mv); got != "OURS\nm\nz\nSIDE2\n" || mv.state(1) != regionTheirs {
		t.Fatalf("ignore ours of a conflict that took theirs: %q %v", got, mv.state(1))
	}
}

// Mark Resolved saves and stages the file, and the merge goes on; Save and Close keeps the
// conflict and the result, which a merge view opened again goes on from.
func TestMergeViewResolveAndContinue(t *testing.T) {
	e, root := mergeConflict(t, map[string][3]string{"shared.txt": {"a\nm\nz\n", "a\nours\nz\n", "a\nside\nz\n"}})
	path := filepath.Join(root, "shared.txt")
	e.openMerge(path)
	mv := e.cur().merge
	e.mergeTake(mv, 0, false)
	if err := e.mergeClose(); err != nil {
		t.Fatal(err)
	}
	if len(e.cols) != 1 || e.cur().merge != nil {
		t.Fatalf("after close: %d columns", len(e.cols))
	}
	if b, _ := os.ReadFile(path); string(b) != "a\nours\nz\n" {
		t.Fatalf("saved %q", b)
	}
	e.gitChanged()
	pump(t, e, func() bool { return e.git.entries[path].Conflicted() })
	if err := e.openMerge(path); err != nil {
		t.Fatal(err)
	}
	mv = e.cur().merge
	if got := resultText(mv); got != "a\nours\nz\n" || mv.state(0) != regionOurs {
		t.Fatalf("reopened: %q %v", got, mv.state(0))
	}
	e.mergeTake(mv, 0, true)
	if err := e.mergeMarkResolved(); err != nil {
		t.Fatal(err)
	}
	// The watcher can observe the staged file before gitDo posts completion.
	pump(t, e, func() bool { en, ok := e.git.entries[path]; return e.git.busy == "" && ok && !en.Conflicted() })
	if out := gitOut(t, root, "diff", "--cached", "--name-only"); out != "shared.txt" {
		t.Fatalf("staged %q", out)
	}
	if err := e.continueOp(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.status != nil && e.git.status.Op == "" })
	if b, _ := os.ReadFile(path); string(b) != "a\nours\nside\nz\n" {
		t.Fatalf("merged %q", b)
	}
}

// Restore Conflict puts the file back in conflict and the result back at the start.
func TestMergeViewRestoreConflict(t *testing.T) {
	e, root := mergeConflict(t, map[string][3]string{"shared.txt": {"a\nm\nz\n", "a\nours\nz\n", "a\nside\nz\n"}})
	path := filepath.Join(root, "shared.txt")
	e.openMerge(path)
	mv := e.cur().merge
	e.mergeTake(mv, 0, true)
	e.mergeMarkResolved()
	// Watcher state can precede the completion callback that resets the merge view.
	pump(t, e, func() bool { en, ok := e.git.entries[path]; return e.git.busy == "" && ok && !en.Conflicted() })
	e.FocusColumn(1)
	e.Run("yan.merge.restoreConflict")
	chooseItem(t, e, "恢复冲突")
	pump(t, e, func() bool { en, ok := e.git.entries[path]; return e.git.busy == "" && ok && en.Conflicted() })
	if got := resultText(mv); got != "a\nm\nz\n" || mv.unresolved() != 1 {
		t.Fatalf("restored %q", got)
	}
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "<<<<<<<") {
		t.Fatalf("markers %q", b)
	}
}

// Opening a conflicted file from Source Control opens its merge view; closing the result
// closes the sides.
func TestMergeViewFromSourceControl(t *testing.T) {
	e, root := mergeConflict(t, map[string][3]string{"shared.txt": {"a\nm\nz\n", "a\nours\nz\n", "a\nside\nz\n"}})
	e.Run("workbench.view.scm")
	e.SCMClick(0, true)
	if mv := e.cur().merge; mv == nil || mv.path != filepath.Join(root, "shared.txt") {
		t.Fatalf("SCM opened %v (%q)", tabNames(e), e.status)
	}
	e.closeTab(e.column(), e.column().active, true)
	var f view.Frame
	e.Frame(&f)
	if len(e.cols) != 1 || len(e.cols[0].tabs) != 0 {
		t.Fatalf("sides left: %d columns", len(e.cols))
	}
}

// The sides scroll with the result (as Mayoi #62 has its panes do): each lands on the line the
// changes between the texts put beside the result's top, ours past the lines it added and
// theirs past the lines it took out, and none runs past its own end.
func TestMergeSidesScrollWithTheResult(t *testing.T) {
	lines := func(f func(i int) []string) string {
		var out []string
		for i := range 60 {
			out = append(out, f(i)...)
		}
		return strings.Join(out, "\n") + "\n"
	}
	base := lines(func(i int) []string { return []string{"L" + strconv.Itoa(i)} })
	ours := lines(func(i int) []string {
		switch {
		case i == 5:
			out := []string{"L5"}
			for k := range 20 {
				out = append(out, "O"+strconv.Itoa(k))
			}
			return out
		case i == 59:
			return []string{"END-ours"}
		}
		return []string{"L" + strconv.Itoa(i)}
	})
	theirs := lines(func(i int) []string {
		switch {
		case i >= 30 && i < 40:
			return nil
		case i == 59:
			return []string{"END-theirs"}
		}
		return []string{"L" + strconv.Itoa(i)}
	})
	e, root := mergeConflict(t, map[string][3]string{"long.txt": {base, ours, theirs}})
	if err := e.openMerge(filepath.Join(root, "long.txt")); err != nil {
		t.Fatal(err)
	}
	mv := e.cur().merge
	result := strings.Split(resultText(mv), "\n")
	top := func(st *tab, theirs bool) string {
		side := mv.sideLines(theirs)
		return side[min(st.scrollY-mergeHeader, len(side)-1)]
	}
	var f view.Frame
	for _, want := range []string{"L3", "L20", "L45", "L58"} {
		mv.result.scrollY = slices.Index(result, want)
		e.Frame(&f)
		if got := top(mv.oursTab, false); got != want {
			t.Errorf("result at %s: ours at %s", want, got)
		}
		if got := top(mv.theirsTab, true); got != want {
			t.Errorf("result at %s: theirs at %s", want, got)
		}
	}
	mv.result.scrollY = len(result) - 1
	e.Frame(&f)
	for _, st := range []*tab{mv.oursTab, mv.theirsTab} {
		if n := len(mv.sideLines(st == mv.theirsTab)); st.scrollY-mergeHeader > n {
			t.Errorf("a side scrolled to %d, past its %d lines", st.scrollY-mergeHeader, n)
		}
	}
}
