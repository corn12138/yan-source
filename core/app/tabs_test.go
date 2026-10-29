package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func TestPreviewTabsReplaceEachOther(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a", "b.txt": "b", "c.txt": "c"})
	// 资源管理器：a.txt、b.txt、c.txt。单击是预览，下一个预览顶替它。
	e.ClickTree(0, false)
	e.ClickTree(1, false)
	if got := strings.Join(tabNames(e), " "); got != "~b.txt*" {
		t.Fatalf("tabs = %s", got)
	}
	if e.area != areaTree {
		t.Fatal("previewing keeps the keyboard in the explorer")
	}
	// 双击转正，焦点到编辑器；再单击别的文件就新开一个预览标签。
	e.ClickTree(1, true)
	e.ClickTree(2, false)
	if got := strings.Join(tabNames(e), " "); got != "b.txt ~c.txt*" {
		t.Fatalf("tabs = %s", got)
	}
	// 在预览标签里打字：转正。
	e.FocusColumn(0)
	typeText(e, "x")
	if got := strings.Join(tabNames(e), " "); got != "b.txt c.txt*" {
		t.Fatalf("editing should keep the preview tab: %s", got)
	}
	_ = root
}

func TestReopenClosedTabRestoresCaret(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "one\ntwo\nthree\n", "b.txt": "b"})
	e.OpenFile(filepath.Join(root, "a.txt"))
	e.OpenFile(filepath.Join(root, "b.txt"))
	e.ClickTab(0, 0, false)
	e.Handle(key(keymap.KeyDown, 0))
	e.Handle(key(keymap.KeyDown, 0))
	e.Handle(ctrl('w'))
	if got := strings.Join(tabNames(e), " "); got != "b.txt*" {
		t.Fatalf("tabs = %s", got)
	}
	e.Handle(KeyInput{keymap.Chord{Mods: keymap.ModCtrl | keymap.ModShift, Key: keymap.KeyRune, Rune: 't'}})
	if got := strings.Join(tabNames(e), " "); got != "a.txt* b.txt" {
		t.Fatalf("reopen: tabs = %s", got)
	}
	s := e.cur().doc.buf.Snapshot()
	if head := e.cur().doc.buf.Selections().Main().Head; s.LineAt(head) != 2 {
		t.Fatalf("caret line = %d, want 2", s.LineAt(head))
	}
}

func TestSplitRightKeepsCaretsApart(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "one\ntwo\nthree\n"})
	e.OpenFile(filepath.Join(root, "a.txt"))
	e.Run("workbench.action.splitEditor")
	if e.Columns() != 2 || e.col != 1 {
		t.Fatalf("columns = %d, focused %d", e.Columns(), e.col)
	}
	// 右列：光标移到第 3 行并打字；左列的光标不动，但跟着文字走。
	e.Handle(key(keymap.KeyDown, 0))
	e.Handle(key(keymap.KeyDown, 0))
	typeText(e, ">")
	e.FocusColumn(0)
	typeText(e, "<")
	if got := content(e); got != "<one\ntwo\n>three\n" {
		t.Fatalf("content = %q", got)
	}
	var f view.Frame
	e.Frame(&f)
	if len(f.Columns) != 2 || f.Columns[0].Pane.Caret.Y != 0 || f.Columns[1].Pane.Caret.Y != 2 {
		t.Fatalf("carets: left %+v right %+v", f.Columns[0].Pane.Caret, f.Columns[1].Pane.Caret)
	}
	if !f.Columns[0].Pane.Caret.Visible || f.Columns[1].Pane.Caret.Visible {
		t.Fatal("only the focused column shows the real caret")
	}
	// 撤销作用于共享的文档。
	e.Handle(ctrl('z'))
	if got := content(e); got != "one\ntwo\n>three\n" {
		t.Fatalf("undo = %q", got)
	}
	// 关掉右列的标签，右列随之消失。
	e.FocusColumn(1)
	e.Handle(ctrl('w'))
	if e.Columns() != 1 {
		t.Fatalf("columns = %d", e.Columns())
	}
}

func TestMoveTabToNewColumn(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a", "b.txt": "b"})
	e.OpenFile(filepath.Join(root, "a.txt"))
	e.OpenFile(filepath.Join(root, "b.txt"))
	e.Run("workbench.action.moveEditorToNextGroup")
	if e.Columns() != 2 || strings.Join(tabNames(e), " ") != "b.txt*" {
		t.Fatalf("columns %d tabs %v", e.Columns(), tabNames(e))
	}
	e.Run("workbench.action.focusPreviousGroup")
	if strings.Join(tabNames(e), " ") != "a.txt*" {
		t.Fatalf("left tabs %v", tabNames(e))
	}
}

func TestOpenEditorAtIndex(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a", "b.txt": "b", "c.txt": "c"})
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		e.OpenFile(filepath.Join(root, n))
	}
	e.Run("workbench.action.openEditorAtIndex1")
	if strings.Join(tabNames(e), " ") != "a.txt* b.txt c.txt" {
		t.Fatalf("tabs %v", tabNames(e))
	}
	e.Run("workbench.action.openEditorAtIndex9") // 9 是最后一个
	if strings.Join(tabNames(e), " ") != "a.txt b.txt c.txt*" {
		t.Fatalf("tabs %v", tabNames(e))
	}
}

// 拖标签：沿着标签条挪位置；挪到别的列，挪的是标签本身（改动视图也行），原来的列空了就关掉；
// 那一列已经有这个文件的标签时显示那一个。「关闭其他」「关闭全部」留下有未保存修改的标签。
func TestMoveAndCloseTabs(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		os.WriteFile(filepath.Join(root, n), []byte(n+"\n"), 0o644)
	}
	e := newEditorIn(t, root)
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := e.OpenFile(filepath.Join(root, n)); err != nil {
			t.Fatal(err)
		}
		e.keep(e.cur())
	}
	names := func(c int) string {
		var ns []string
		for _, tb := range e.cols[c].tabs {
			ns = append(ns, filepath.Base(tb.doc.buf.Path()))
		}
		return strings.Join(ns, ",")
	}
	e.MoveTab(0, 2, 0, 0) // c 拖到最前面
	if names(0) != "c.txt,a.txt,b.txt" || e.cur().doc.buf.Path() != filepath.Join(root, "c.txt") {
		t.Fatalf("after reorder: %s", names(0))
	}
	e.MoveTab(0, 0, 0, 3) // 再拖到最后
	if names(0) != "a.txt,b.txt,c.txt" {
		t.Fatalf("after moving to the end: %s", names(0))
	}
	e.MoveTab(0, 1, 1, 0) // b 拖到右边（新开一列）
	if len(e.cols) != 2 || names(0) != "a.txt,c.txt" || names(1) != "b.txt" || e.col != 1 {
		t.Fatalf("after moving to a new column: %s | %s", names(0), names(1))
	}
	e.MoveTab(1, 0, 0, 1) // 拖回来：右边那一列空了，关掉
	if len(e.cols) != 1 || names(0) != "a.txt,b.txt,c.txt" || e.cur().doc.buf.Path() != filepath.Join(root, "b.txt") {
		t.Fatalf("after moving back: %d columns, %s", len(e.cols), names(0))
	}
	// 同一个文件在两列里：拖过去显示那边的标签
	e.Run("workbench.action.splitEditor")
	e.MoveTab(0, 1, 1, 0)
	if names(0) != "a.txt,c.txt" || names(1) != "b.txt" || len(e.cur().doc.tabs) != 1 {
		t.Fatalf("after moving onto the same file: %s | %s", names(0), names(1))
	}
	e.MoveTab(1, 0, 0, 2)

	// 关闭其他：改过没存的 a 留着
	e.ClickTab(0, 0, false)
	e.Handle(TextInput{Text: "x"})
	e.ClickTab(0, 1, false)
	e.Run("workbench.action.closeOtherEditors")
	if names(0) != "a.txt,c.txt" || !strings.Contains(e.status, "1 个标签有未保存的修改") {
		t.Fatalf("after close others: %s (%q)", names(0), e.status)
	}
	e.Run("workbench.action.closeEditorsInGroup")
	if names(0) != "a.txt" {
		t.Fatalf("after close all: %s", names(0))
	}
}

// 列的宽度：向右拆分各占一半；拖两列之间的边挪宽度，每列至少留 least；拖标签开的新列也分走一半。
func TestColumnWeights(t *testing.T) {
	e, _, _ := newTestEditor(t, "a\n")
	e.Run("workbench.action.splitEditor")
	if w0, w1 := e.cols[0].weight, e.cols[1].weight; w0 != 0.5 || w1 != 0.5 {
		t.Fatalf("after split: %v %v", w0, w1)
	}
	e.ResizeColumns(1, 0.25, 0.1) // 左边那一列宽到 3/4
	if w0, w1 := e.cols[0].weight, e.cols[1].weight; w0 != 0.75 || w1 != 0.25 {
		t.Fatalf("after resize: %v %v", w0, w1)
	}
	e.ResizeColumns(1, 0.5, 0.1) // 右边那一列最窄留 0.1
	if w0, w1 := e.cols[0].weight, e.cols[1].weight; w0 != 0.9 || w1 < 0.0999 || w1 > 0.1001 {
		t.Fatalf("after resize past the least: %v %v", w0, w1)
	}
	var f view.Frame
	if e.Frame(&f); f.Columns[0].Weight != 0.9 {
		t.Fatalf("frame weight %v", f.Columns[0].Weight)
	}
}
