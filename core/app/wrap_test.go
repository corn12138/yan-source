package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
	"github.com/corn12138/yan-source/core/view/cell"
)

// newWrapEditor 打开 content，窗格 20 格宽（行号栏 6 格，一行折到 13 格），自动换行开着。
func newWrapEditor(t *testing.T, content string) *Editor {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.WordWrap = true
	e, _ := New(cfg, cell.Layouter{}, &fakeHost{}, keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl, keymap.ModSuper}})
	e.SetPaneSize(0, view.Size{W: 20, H: 4})
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	return e
}

func frameRows(e *Editor) (*view.PaneFrame, []string) {
	var f view.Frame
	e.Frame(&f)
	p := &f.Columns[0].Pane
	var out []string
	for _, r := range p.Rows {
		out = append(out, string(r.Text))
	}
	return p, out
}

func TestWordWrapFrame(t *testing.T) {
	e := newWrapEditor(t, "one two three four five six\nshort")
	p, rows := frameRows(e)
	want := "one two three |four five six|short"
	if got := strings.Join(rows, "|"); got != want {
		t.Fatalf("rows %q, want %q", got, want)
	}
	if p.Rows[1].Num != -1 || p.Rows[0].Num != 0 || p.Rows[2].Line != 1 {
		t.Fatalf("line numbers: %+v", p.Rows)
	}
	// End 落在第二个视觉行的末尾
	e.Handle(key(keymap.KeyEnd, 0))
	p, _ = frameRows(e)
	if p.Caret.Y != 1 || p.Caret.X != p.Gutter+13 {
		t.Fatalf("caret at %d,%d", p.Caret.X, p.Caret.Y)
	}
}

func TestWordWrapMovesAndScrollsByVisualRow(t *testing.T) {
	e := newWrapEditor(t, "one two three four five six\nshort\na\nb\nc")
	b := e.cur().doc.buf
	e.Handle(key(keymap.KeyRight, 0))
	e.Handle(key(keymap.KeyRight, 0))
	e.Handle(key(keymap.KeyDown, 0)) // 还在第一行：第二个视觉行的第 2 格
	if h := b.Selections().Main().Head; h != 16 {
		t.Fatalf("down went to %d, want 16", h)
	}
	e.Handle(key(keymap.KeyDown, 0)) // 第二行 short
	if h := b.Selections().Main().Head; h != 30 {
		t.Fatalf("down x2 at %d, want 30", h)
	}
	e.Handle(key(keymap.KeyUp, 0))
	if h := b.Selections().Main().Head; h != 16 {
		t.Fatalf("up at %d, want 16", h)
	}
	for range 3 {
		e.Handle(key(keymap.KeyDown, 0)) // 第四行 b：滚一个视觉行（窗格 4 行高）
	}
	if _, rows := frameRows(e); strings.Join(rows, "|") != "four five six|short|a|b" {
		t.Fatalf("scrolled rows %q", rows)
	}
	e.Handle(ScrollInput{DY: 1})
	if _, rows := frameRows(e); strings.Join(rows, "|") != "short|a|b|c" {
		t.Fatalf("scrolled by wheel %q", rows)
	}
	// 点第二个视觉行的第 2 格（第一个逻辑行折下来的部分）
	e.Handle(ScrollInput{DY: -10})
	if pos := e.posAt(e.cols[0], e.cur(), 6+2, 1); pos != 16 {
		t.Fatalf("click at row 1 col 2 = %d, want 16", pos)
	}
	// 点第一个视觉行末尾之外：停在挂着的空格前面，不跳到下一行
	if pos := e.posAt(e.cols[0], e.cur(), 19, 0); pos != 13 {
		t.Fatalf("click past row end = %d, want 13", pos)
	}
}

func TestToggleWordWrap(t *testing.T) {
	e := newWrapEditor(t, "one two three four five six")
	e.Run("editor.action.toggleWordWrap")
	if _, rows := frameRows(e); len(rows) != 1 {
		t.Fatalf("wrap off: %q", rows)
	}
	e.Run("editor.action.toggleWordWrap")
	if _, rows := frameRows(e); len(rows) != 2 {
		t.Fatalf("wrap on: %q", rows)
	}
}

// ⌥Z 只切换这个标签：别的标签照设置；设置再变，各标签单独定的不算了。
func TestToggleTabWordWrap(t *testing.T) {
	e := newWrapEditor(t, "one two three four five six")
	rows := func(c int) int {
		var f view.Frame
		e.Frame(&f)
		return len(f.Columns[c].Pane.Rows)
	}
	e.Run("workbench.action.splitEditor") // 右边一列，同一个文件
	e.Handle(KeyInput{keymap.Chord{Mods: keymap.ModAlt, Key: keymap.KeyRune, Rune: 'z'}})
	if rows(1) != 1 || rows(0) != 2 {
		t.Fatalf("after ⌥Z in the right column: %d rows there, %d on the left", rows(1), rows(0))
	}
	e.Run("editor.action.toggleWordWrap") // 设置关了：两个都不折
	e.Run("editor.action.toggleWordWrap") // 再开：两个都折，包括右边按过 ⌥Z 的
	if rows(1) != 2 || rows(0) != 2 {
		t.Fatalf("after the setting changed: %d, %d", rows(1), rows(0))
	}
}
