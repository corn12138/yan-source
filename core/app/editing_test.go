package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// newGoEditor 打开一个 Go 文件；| 标出光标，[ ] 标出选区。
func newGoEditor(t *testing.T, src string) *Editor {
	t.Helper()
	p := filepath.Join(t.TempDir(), "main.go")
	clean := strings.NewReplacer("|", "", "[", "", "]", "").Replace(src)
	if err := os.WriteFile(p, []byte(clean), 0o644); err != nil {
		t.Fatal(err)
	}
	e, warns := New(config.Default(), fakeLayouter{}, &fakeHost{}, keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl, keymap.ModSuper}})
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}
	e.SetPaneSize(0, view.Size{W: 80, H: 20})
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	var r edit.Range
	switch {
	case strings.Contains(src, "|"):
		at := strings.Index(src, "|")
		r = edit.Cursor(at)
	case strings.Contains(src, "["):
		a := strings.Index(src, "[")
		z := strings.Index(src, "]") - 1
		r = edit.Range{Anchor: a, Head: z, GoalX: -1}
	}
	e.cur().doc.buf.SetSelections(edit.Single(r))
	return e
}

// shown 返回文字，光标处插入 |（有选区时用 [ ] 标出）。
func shown(e *Editor) string {
	b := e.cur().doc.buf
	s := b.Snapshot().String(0, b.Snapshot().Len())
	r := b.Selections().Main()
	if r.Empty() {
		return s[:r.Head] + "|" + s[r.Head:]
	}
	return s[:r.From()] + "[" + s[r.From():r.To()] + "]" + s[r.To():]
}

func cmdKey(r rune, m keymap.Mod) KeyInput {
	return KeyInput{keymap.Chord{Mods: m, Key: keymap.KeyRune, Rune: r}}
}

func TestEditingCommands(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		do        func(e *Editor)
		want      string
	}{
		{"enter keeps indent", "func f() {\n\tx := 1|\n}\n",
			func(e *Editor) { e.Handle(key(keymap.KeyEnter, 0)) }, "func f() {\n\tx := 1\n\t|\n}\n"},
		{"enter after brace indents and splits the pair", "func f() {|}\n",
			func(e *Editor) { e.Handle(key(keymap.KeyEnter, 0)) }, "func f() {\n\t|\n}\n"},
		{"enter drops trailing spaces", "a  |\n",
			func(e *Editor) { e.Handle(key(keymap.KeyEnter, 0)) }, "a\n|\n"},
		{"tab inserts a tab in Go", "|x\n",
			func(e *Editor) { e.Handle(key(keymap.KeyTab, 0)) }, "\t|x\n"},
		{"shift+tab outdents", "\t\t|x\n",
			func(e *Editor) { e.Handle(key(keymap.KeyTab, keymap.ModShift)) }, "\t|x\n"},
		{"tab indents selected lines", "[a\nb\n]c\n",
			func(e *Editor) { e.Handle(key(keymap.KeyTab, 0)) }, "[\ta\n\tb\n]c\n"},
		{"comment toggles", "\t|x := 1\n\ty := 2\n",
			func(e *Editor) { e.Handle(cmdKey('/', keymap.ModCtrl)) }, "\t// |x := 1\n\ty := 2\n"},
		{"uncomment", "\t// |x := 1\n",
			func(e *Editor) { e.Handle(cmdKey('/', keymap.ModCtrl)) }, "\t|x := 1\n"},
		{"move line down", "o|ne\ntwo\nthree\n",
			func(e *Editor) { e.Handle(key(keymap.KeyDown, keymap.ModAlt)) }, "two\no|ne\nthree\n"},
		{"move line up", "one\ntwo\nth|ree\n",
			func(e *Editor) { e.Handle(key(keymap.KeyUp, keymap.ModAlt)) }, "one\nth|ree\ntwo\n"},
		{"copy line down selects the copy", "o|ne\ntwo\n",
			func(e *Editor) { e.Handle(key(keymap.KeyDown, keymap.ModAlt|keymap.ModShift)) }, "one\no|ne\ntwo\n"},
		{"delete line keeps the column", "one\ntw|o\nthree\n",
			func(e *Editor) { e.Handle(cmdKey('k', keymap.ModCtrl|keymap.ModShift)) }, "one\nth|ree\n"},
		{"insert line below a brace", "func f() {|\n}\n",
			func(e *Editor) { e.Handle(key(keymap.KeyEnter, keymap.ModCtrl)) }, "func f() {\n\t|\n}\n"},
		{"select line", "one\nt|wo\nthree\n",
			func(e *Editor) { e.Handle(cmdKey('l', keymap.ModCtrl)) }, "one\n[two\n]three\n"},
		{"jump to bracket", "f(a, |(b))\n",
			func(e *Editor) { e.Handle(cmdKey('\\', keymap.ModCtrl|keymap.ModShift)) }, "f(a, (b|))\n"}, // 光标在括号前面，就停在配对括号前面（Mayoi 的行为）
		{"auto-close a paren", "f|\n",
			func(e *Editor) { typeText(e, "(") }, "f(|)\n"},
		{"step over the closer", "f|\n",
			func(e *Editor) { typeText(e, "("); typeText(e, "x"); typeText(e, ")") }, "f(x)|\n"},
		{"backspace deletes the pair", "f|\n",
			func(e *Editor) { typeText(e, "("); e.Handle(key(keymap.KeyBackspace, 0)) }, "f|\n"},
		{"wrap a selection in quotes", "x := [hi]\n",
			func(e *Editor) { typeText(e, `"`) }, "x := \"[hi]\"\n"},
		{"no close before a word", "|x\n",
			func(e *Editor) { typeText(e, "(") }, "(|x\n"},
		{"no apostrophe pair in Go", "|\n",
			func(e *Editor) { typeText(e, "'") }, "'|\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newGoEditor(t, tc.src)
			tc.do(e)
			if got := shown(e); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEditingUndoesAsOneStep(t *testing.T) {
	e := newGoEditor(t, "a\n[b\nc\n]d\n")
	e.Handle(cmdKey('/', keymap.ModCtrl))
	if !strings.Contains(content(e), "// b") {
		t.Fatalf("not commented: %q", content(e))
	}
	e.Handle(ctrl('z'))
	if content(e) != "a\nb\nc\nd\n" {
		t.Fatalf("undo = %q", content(e))
	}
}

func TestSortCamelAndIndentCommands(t *testing.T) {
	for _, tc := range []struct {
		name, src, cmd, want string
	}{
		{"sort the whole file, its last newline staying last", "c\na|\nb\n", "editor.action.sortLinesAscending", "a\nb|\nc\n"},
		{"sort descending", "a\n|b\nc\n", "editor.action.sortLinesDescending", "c\n|b\na\n"},
		{"sort the selected lines only", "z\n[c\nb\n]a\n", "editor.action.sortLinesAscending", "z\n[b\nc\n]a\n"},
		{"camel case the word of the caret", "x := foo_b|ar_baz\n", "editor.action.transformToCamelcase", "x := [fooBarBaz]\n"},
		{"camel case a selection", "[hello big-world]\n", "editor.action.transformToCamelcase", "[helloBigWorld]\n"},
		{"camel case keeps the first word", "Ünï_çode|\n", "editor.action.transformToCamelcase", "[ÜnïÇode]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newGoEditor(t, tc.src)
			e.Run(tc.cmd)
			if got := shown(e); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("each cursor camel cases its own word, as one undo", func(t *testing.T) {
		e := newGoEditor(t, "a_b c_d\n")
		b := e.cur().doc.buf
		b.SetSelections(edit.Selections{Ranges: []edit.Range{edit.Cursor(1), edit.Cursor(5)}})
		e.Run("editor.action.transformToCamelcase")
		if got := b.Snapshot().String(0, b.Snapshot().Len()); got != "aB cD\n" {
			t.Fatalf("got %q", got)
		}
		if rs := b.Selections().Ranges; len(rs) != 2 || rs[1].From() != 3 || rs[1].To() != 5 {
			t.Fatalf("selections %+v", rs)
		}
		e.Run("undo")
		if got := b.Snapshot().String(0, b.Snapshot().Len()); got != "a_b c_d\n" {
			t.Fatalf("after undo %q", got)
		}
	})

	t.Run("indent using spaces, then tabs", func(t *testing.T) {
		e := newGoEditor(t, "func f() {|}\n")
		e.Run("yan.indentUsingSpaces4")
		e.Handle(key(keymap.KeyEnter, 0))
		if got := shown(e); got != "func f() {\n    |\n}\n" {
			t.Fatalf("spaces: %q", got)
		}
		e.Run("editor.action.indentUsingTabs")
		e.Handle(key(keymap.KeyTab, 0))
		if got := shown(e); got != "func f() {\n    \t|\n}\n" {
			t.Fatalf("tabs: %q", got)
		}
	})
}
