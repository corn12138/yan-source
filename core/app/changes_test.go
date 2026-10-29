package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 所有改动：已暂存、更改、未跟踪各一个文件，按源代码管理的顺序；翻页器的键在文件、改动之间走，z 折起文件，
// s 在标题上暂存整个文件，在改动里只暂存那一处。
func TestAllChanges(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a1\na2\n", "b.txt": "b1\nb2\nb3\nb4\nb5\nb6\nb7\nb8\nb9\nb10\n"})
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("a1\nA2\n"), 0o644)
	run("add", "a.txt")
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("B1\nb2\nb3\nb4\nb5\nb6\nb7\nb8\nb9\nB10\n"), 0o644)
	os.WriteFile(filepath.Join(root, "c.txt"), []byte("new\n"), 0o644)
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil && len(e.git.status.Entries) == 3 })
	if err := e.openAllChanges(); err != nil {
		t.Fatal(err)
	}
	tb := e.cur()
	headers := func() []string {
		var hs []string
		s := tb.doc.buf.Snapshot()
		for i, l := range tb.diff.lines {
			if l.kind == diffFile {
				ls, le := s.LineRange(i)
				hs = append(hs, s.String(ls, le))
			}
		}
		return hs
	}
	pump(t, e, func() bool { return len(headers()) == 3 })
	hs := headers()
	if !strings.Contains(hs[0], "a.txt  +1 −1  已暂存") || !strings.Contains(hs[1], "b.txt  +2 −2  更改") || !strings.Contains(hs[2], "c.txt  +1 −0  未跟踪") {
		t.Fatalf("headers %q", hs)
	}
	line := func() int { return tb.doc.buf.Snapshot().LineAt(tb.doc.buf.Selections().Main().Head) }
	kind := func() diffKind { return tb.diff.lines[line()].kind }

	e.diffGo(tb, 0)
	typeText(e, ".") // 下一个文件：b.txt
	if kind() != diffFile || tb.diff.lines[line()].sec != 1 {
		t.Fatalf("after . on line %d (%v)", line(), kind())
	}
	typeText(e, "]") // b.txt 的第一处改动
	if kind() != diffHeader || tb.diff.lines[line()].hunk != 0 {
		t.Fatalf("after ] on line %d", line())
	}
	typeText(e, ",") // 回到 b.txt 的标题
	typeText(e, "z") // 折起 b.txt
	if n := len(tb.diff.lines); strings.Contains(strings.Join(headers(), "|"), "▾ M b.txt") || n > 12 {
		t.Fatalf("b.txt not folded: %q, %d lines", headers(), n)
	}
	typeText(e, "z") // 再展开

	// 在 b.txt 第一处改动里按 s：索引里只有 B1，没有 B10
	e.diffGo(tb, 0)
	typeText(e, ".")
	typeText(e, "]")
	typeText(e, "s")
	pump(t, e, func() bool { return e.git.busy == "" })
	if idx := run("show", ":b.txt"); !strings.HasPrefix(idx, "B1\n") || strings.Contains(idx, "B10") {
		t.Fatalf("index after staging one change: %q (status %q)", idx, e.status)
	}

	// 在 c.txt 的标题上按 s：整个文件暂存
	for i, l := range tb.diff.lines {
		if l.kind == diffFile && tb.diff.secs[l.sec].group == changeUntracked {
			e.diffGo(tb, i)
		}
	}
	typeText(e, "s")
	// 整个文件暂存时 git 在后台跑，不占「忙」：等它真的暂存了
	pump(t, e, func() bool {
		for _, en := range e.git.status.Entries {
			if filepath.Base(en.Path) == "c.txt" && en.Staged() {
				return true
			}
		}
		return false
	})
	if out := run("status", "--porcelain", "c.txt"); !strings.HasPrefix(out, "A ") {
		t.Fatalf("c.txt after s on its header: %q", out)
	}
}

// 提交的标签：先是说明、作者、提交号，再是这个提交改的每个文件（相对第一个父提交）；里面暂存不了。
func TestCommitTab(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a1\na2\n"})
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("a1\nA2\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("new\n"), 0o644)
	run("add", ".")
	run("commit", "-q", "-m", "change a, add b\n\nthe body")
	e := newEditorIn(t, root)
	e.Run("workbench.view.scm")
	pump(t, e, func() bool { return len(e.scm.history.commits) == 2 })
	if err := e.openCommitTab(0, true); err != nil {
		t.Fatal(err)
	}
	tb := e.cur()
	pump(t, e, func() bool { return !tb.diff.loading && len(tb.diff.secs) == 2 })
	text := tb.doc.buf.Snapshot().String(0, tb.doc.buf.Snapshot().Len())
	c := e.scm.history.commits[0]
	for _, want := range []string{"change a, add b", c.Hash, "the body", "a.txt  +1 −1  提交里", "b.txt  +1 −0  提交里", "\nA2\n", "\na2\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("commit tab lacks %q:\n%s", want, text)
		}
	}
	if !tb.preview {
		t.Fatal("a click should open the commit in the preview tab")
	}
	for i, l := range tb.diff.lines {
		if l.kind == diffFile {
			e.diffGo(tb, i)
			break
		}
	}
	e.area = areaEditor
	if typeText(e, "s"); !strings.Contains(e.status, "暂存不了") {
		t.Fatalf("staging in a commit tab: %q", e.status)
	}
}
