package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// 行号旁的改动：点标记开框，框里是 HEAD 里的样子；还原是撤销的一步；暂存只把那一处放进索引。
func TestHunkPeekRevertStage(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "one\ntwo\nthree\nfour\nfive\nsix\nseven\n"})
	path := filepath.Join(root, "a.txt")
	os.WriteFile(path, []byte("one\nTWO\nthree\nfour\nfive\nsix\nseven\nEIGHT\n"), 0o644)
	e := newEditorIn(t, root)
	if err := e.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	tb := e.cur()
	var f view.Frame
	pump(t, e, func() bool { e.Frame(&f); return len(tb.doc.marks.hunks) == 2 })

	// 点第 2 行（TWO）行号旁：开框
	if !e.gutterClick(e.column(), tb, 0, 1) {
		t.Fatal("gutter click on a mark did not open the box")
	}
	e.Frame(&f)
	p := f.Columns[0].Pane.Peek
	if p == nil || p.After != 1 || strings.Join(p.Removed, ",") != "two" || p.Count != 2 || p.Index != 0 {
		t.Fatalf("peek %+v", p)
	}
	if !e.gutterClick(e.column(), tb, 0, 1) { // 再点一下：关上
		t.Fatal("second click")
	}
	if e.Frame(&f); f.Columns[0].Pane.Peek != nil {
		t.Fatal("box still open")
	}

	// ⌥F3：开下一处（第 8 行加的 EIGHT），Esc 关上
	e.GotoLine(1)
	e.Handle(KeyInput{keymap.Chord{Mods: keymap.ModAlt, Key: keymap.KeyF3}})
	if e.Frame(&f); f.Columns[0].Pane.Peek == nil {
		t.Fatalf("alt+f3: %+v", f.Columns[0].Pane.Peek)
	}
	e.Handle(key(keymap.KeyEsc, 0))
	if e.Frame(&f); f.Columns[0].Pane.Peek != nil {
		t.Fatal("esc did not close the box")
	}

	// 还原 TWO：回到 two，撤销又是 TWO
	e.GotoLine(2)
	if err := e.revertHunk(); err != nil {
		t.Fatal(err)
	}
	text := func() string { s := tb.doc.buf.Snapshot(); return s.String(0, s.Len()) }
	if !strings.HasPrefix(text(), "one\ntwo\nthree") {
		t.Fatalf("after revert %q", text())
	}
	e.Run("undo")
	if !strings.HasPrefix(text(), "one\nTWO\nthree") {
		t.Fatalf("after undo %q", text())
	}
	pump(t, e, func() bool { e.Frame(&f); return len(tb.doc.marks.hunks) == 2 })

	// 暂存最后加的 EIGHT：索引里有它，没有 TWO
	e.GotoLine(8)
	if err := e.stageHunk(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.busy == "" })
	out, _ := exec.Command("git", "-C", root, "show", ":a.txt").Output()
	if got := string(out); !strings.Contains(got, "EIGHT") || strings.Contains(got, "TWO") {
		t.Fatalf("index %q (status %q)", got, e.status)
	}
}

// 删掉的行也能还原：插回原来的地方。
func TestRevertDeletedLines(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "one\ntwo\nthree\n"})
	path := filepath.Join(root, "a.txt")
	os.WriteFile(path, []byte("one\nthree\n"), 0o644)
	e := newEditorIn(t, root)
	e.OpenFile(path)
	tb := e.cur()
	var f view.Frame
	pump(t, e, func() bool { e.Frame(&f); return len(tb.doc.marks.hunks) == 1 })
	e.GotoLine(1) // 标记在删掉的行上面那一行
	if err := e.revertHunk(); err != nil {
		t.Fatal(err)
	}
	if s := tb.doc.buf.Snapshot(); s.String(0, s.Len()) != "one\ntwo\nthree\n" {
		t.Fatalf("after revert %q", s.String(0, s.Len()))
	}
}
