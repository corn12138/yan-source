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

// 用例移植自 Mayoi 的 cmd/mayoi/vim_test.go：偏移是字节（这些文字都是 ASCII，和 rune 一样）。

func newVimEditor(t *testing.T, name, content string) (*Editor, *fakeHost) {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Vim = true
	h := &fakeHost{}
	e, warns := New(cfg, fakeLayouter{}, h, keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl, keymap.ModSuper}})
	if len(warns) > 0 {
		t.Fatal(warns)
	}
	e.SetPaneSize(0, view.Size{W: 80, H: 20})
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	return e, h
}

// vim 按 vim 的写法按键：普通字符像终端那样作为打的字送进来，<Esc> <CR> <BS> <C-x> 是按键。
func vim(e *Editor, keys string) {
	for len(keys) > 0 {
		if end := strings.IndexByte(keys, '>'); keys[0] == '<' && end > 2 {
			switch name := keys[1:end]; {
			case name == "Esc":
				e.Handle(key(keymap.KeyEsc, 0))
			case name == "CR":
				e.Handle(key(keymap.KeyEnter, 0))
			case name == "BS":
				e.Handle(key(keymap.KeyBackspace, 0))
			case strings.HasPrefix(name, "C-"):
				e.Handle(KeyInput{keymap.Chord{Mods: keymap.ModCtrl, Key: keymap.KeyRune, Rune: rune(name[2])}})
			}
			keys = keys[end+1:]
			continue
		}
		e.Handle(TextInput{Text: keys[:1]})
		keys = keys[1:]
	}
}

func cursorAt(t *testing.T, e *Editor, at int) {
	t.Helper()
	if m := e.cur().doc.buf.Selections().Main(); m.Anchor != at || m.Head != at {
		t.Fatalf("cursor %d..%d, want %d", m.Anchor, m.Head, at)
	}
}

func textIs(t *testing.T, e *Editor, want string) {
	t.Helper()
	if got := content(e); got != want {
		t.Fatalf("text %q, want %q", got, want)
	}
}

func TestVimNormalModeTypesNothing(t *testing.T) {
	e, _ := newVimEditor(t, "a.txt", "hello\n")
	var f view.Frame
	e.Frame(&f)
	if f.Status.Right[0].Text != "-- 普通 --" || !f.Columns[0].Pane.Caret.Block {
		t.Fatalf("status %v block %v", f.Status.Right, f.Columns[0].Pane.Caret.Block)
	}
	vim(e, "lj")
	textIs(t, e, "hello\n")
	cursorAt(t, e, 1) // j 不会走到最后那个空行
	vim(e, "0i")
	e.Frame(&f)
	if f.Status.Right[0].Text != "-- 插入 --" || f.Columns[0].Pane.Caret.Block {
		t.Fatal("i should enter insert mode with a bar caret")
	}
	typeText(e, "oh ")
	textIs(t, e, "oh hello\n")
	vim(e, "<Esc>")
	cursorAt(t, e, 2)
}

func TestVimMotions(t *testing.T) {
	e, _ := newVimEditor(t, "a.txt", "one two.three  four\n  five (six [x]) end\n\nlast line\n")
	for _, s := range []struct {
		keys string
		at   int
	}{
		{"w", 4}, {"w", 7}, {"w", 8}, {"W", 15}, {"b", 8}, {"B", 4}, {"e", 6}, {"E", 12}, {"$", 18}, {"0", 0},
		{"j", 20}, {"^", 22}, {"fx", 30}, {"Fs", 28}, {"tx", 29}, {";", 32}, {",", 31}, {"0", 20}, {"f(", 27},
		{"%", 35}, {"%", 27}, {"}", 41}, {"{", 0}, {"G", 42}, {"gg", 0}, {"2G", 22}, {"k", 2}, {"3l", 5}, {"h", 4},
		{"ge", 2}, {"2w", 7},
	} {
		vim(e, s.keys)
		if m := e.cur().doc.buf.Selections().Main(); m.Head != s.at || !m.Empty() {
			t.Fatalf("after %q: cursor %d..%d, want %d", s.keys, m.Anchor, m.Head, s.at)
		}
	}
	// j k 穿过短行后回到原来的列
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(18)))
	vim(e, "jjj")
	cursorAt(t, e, 50)
	vim(e, "kk")
	cursorAt(t, e, 38)
}

func TestVimOperators(t *testing.T) {
	e, _ := newVimEditor(t, "a.txt", "alpha beta gamma\nsecond line\nthird\n")
	vim(e, "dw")
	textIs(t, e, "beta gamma\nsecond line\nthird\n")
	vim(e, "cw")
	typeText(e, "BETA")
	vim(e, "<Esc>")
	textIs(t, e, "BETA gamma\nsecond line\nthird\n")
	cursorAt(t, e, 3)
	vim(e, "u")
	textIs(t, e, "beta gamma\nsecond line\nthird\n")
	vim(e, "0x")
	textIs(t, e, "eta gamma\nsecond line\nthird\n")
	vim(e, "3x")
	textIs(t, e, " gamma\nsecond line\nthird\n")
	vim(e, "dd")
	textIs(t, e, "second line\nthird\n")
	vim(e, "p")
	textIs(t, e, "second line\n gamma\nthird\n")
	cursorAt(t, e, 13)
	vim(e, "ggd$")
	textIs(t, e, "\n gamma\nthird\n")
	vim(e, "jJ")
	textIs(t, e, "\n gamma third\n")
	vim(e, "u")
	textIs(t, e, "\n gamma\nthird\n")
	vim(e, "<C-r>")
	textIs(t, e, "\n gamma third\n")
	if e.ov != nil {
		t.Fatal("⌃R opened Open Recent rather than redoing")
	}
	vim(e, "gg2dd")
	textIs(t, e, "")
}

func TestVimChangeUndoesAtOnce(t *testing.T) {
	e, _ := newVimEditor(t, "a.go", "f(alpha, beta)\n")
	vim(e, "fbciw")
	typeText(e, "g")
	typeText(e, "amma")
	vim(e, "<Esc>")
	textIs(t, e, "f(alpha, gamma)\n")
	cursorAt(t, e, 13)
	vim(e, "u")
	textIs(t, e, "f(alpha, beta)\n")
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(0)))
	vim(e, "w")
	vim(e, "ci(")
	textIs(t, e, "f()\n")
	typeText(e, "a, b")
	vim(e, "<Esc>u")
	textIs(t, e, "f(alpha, beta)\n")
}

func TestVimDotRepeats(t *testing.T) {
	e, _ := newVimEditor(t, "a.txt", "one two three four\n")
	vim(e, "ciw")
	typeText(e, "1")
	vim(e, "<Esc>w.w.")
	textIs(t, e, "1 1 1 four\n")
	vim(e, "0x..")
	textIs(t, e, " 1 four\n")
	vim(e, "u")
	textIs(t, e, "1 1 four\n")
	vim(e, "A")
	typeText(e, "!")
	vim(e, "<Esc>0.")
	textIs(t, e, "1 1 four!!\n")
}

func TestVimTextObjects(t *testing.T) {
	e, _ := newVimEditor(t, "a.go", "x := f(\"a b\", [1, 2])\nif ok {\n\tbody()\n}\n")
	vim(e, "f\"di\"")
	textIs(t, e, "x := f(\"\", [1, 2])\nif ok {\n\tbody()\n}\n")
	vim(e, "f1da[")
	textIs(t, e, "x := f(\"\", )\nif ok {\n\tbody()\n}\n")
	vim(e, "jjdi{")
	textIs(t, e, "x := f(\"\", )\nif ok {\n}\n")
	vim(e, "ggdaw")
	textIs(t, e, ":= f(\"\", )\nif ok {\n}\n")
}

func TestVimVisual(t *testing.T) {
	e, h := newVimEditor(t, "a.txt", "one two three\nfour\nfive\n")
	vim(e, "wv")
	var f view.Frame
	e.Frame(&f)
	if f.Status.Right[0].Text != "-- 可视 --" {
		t.Fatalf("status %v", f.Status.Right)
	}
	vim(e, "e")
	if m := e.cur().doc.buf.Selections().Main(); m.From() != 4 || m.To() != 7 {
		t.Fatalf("visual selection %d..%d", m.From(), m.To())
	}
	vim(e, "y")
	if h.clip != "two" {
		t.Fatalf("visual y copied %q", h.clip)
	}
	cursorAt(t, e, 4)
	vim(e, "veU")
	textIs(t, e, "one TWO three\nfour\nfive\n")
	vim(e, "jVjd")
	textIs(t, e, "one TWO three\n")
	if h.clip != "four\nfive\n" {
		t.Fatalf("V d cut %q", h.clip)
	}
	vim(e, "P")
	textIs(t, e, "four\nfive\none TWO three\n")
	vim(e, "vlohc")
	typeText(e, "X")
	vim(e, "<Esc>")
	textIs(t, e, "Xur\nfive\none TWO three\n")
	vim(e, "Vj>")
	textIs(t, e, "    Xur\n    five\none TWO three\n")
}

func TestVimClipboard(t *testing.T) {
	e, h := newVimEditor(t, "a.txt", "ab\ncd\n")
	vim(e, "yy")
	if h.clip != "ab\n" {
		t.Fatalf("yy copied %q", h.clip)
	}
	h.clip = "XY" // 别处复制的文字：贴在光标后面，不是整行
	vim(e, "p")
	textIs(t, e, "aXYb\ncd\n")
	cursorAt(t, e, 2)
	h.clip = "line\n"
	vim(e, "jP")
	textIs(t, e, "aXYb\nline\ncd\n")
}

func TestVimIndentAndComment(t *testing.T) {
	e, _ := newVimEditor(t, "a.go", "a()\nb()\nc()\n")
	vim(e, "2>>")
	textIs(t, e, "\ta()\n\tb()\nc()\n")
	vim(e, "<j")
	textIs(t, e, "a()\nb()\nc()\n")
	vim(e, "Ggcc")
	textIs(t, e, "a()\nb()\n// c()\n")
	vim(e, "ggo")
	typeText(e, "x")
	vim(e, "<Esc>")
	textIs(t, e, "a()\nx\nb()\n// c()\n")
	vim(e, "u")
	textIs(t, e, "a()\nb()\n// c()\n")
}

func TestVimScrolls(t *testing.T) {
	e, _ := newVimEditor(t, "a.txt", strings.Repeat("line\n", 300))
	vim(e, "<C-d>")
	s := e.cur().doc.buf.Snapshot()
	if l := s.LineAt(e.cur().doc.buf.Selections().Main().Head); l < 5 || e.cur().scrollY <= 0 {
		t.Fatalf("⌃D went to line %d, scrolled to %d", l, e.cur().scrollY)
	}
	vim(e, "<C-u>")
	if l := s.LineAt(e.cur().doc.buf.Selections().Main().Head); l != 0 {
		t.Fatalf("⌃U went to line %d", l)
	}
	vim(e, "G")
	if l := s.LineAt(e.cur().doc.buf.Selections().Main().Head); l != 299 {
		t.Fatalf("G went to line %d, want 299", l)
	}
}

func TestVimOff(t *testing.T) {
	e, _ := newVimEditor(t, "a.txt", "hello\n")
	e.Run("yan.toggleVim")
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(0)))
	typeText(e, "j")
	textIs(t, e, "jhello\n")
	var f view.Frame
	e.Frame(&f)
	for _, it := range f.Status.Right {
		if strings.Contains(it.Text, "普通") {
			t.Fatal("the status bar shows a mode with vim mode off")
		}
	}
}

func TestVimCRLF(t *testing.T) {
	e, _ := newVimEditor(t, "a.txt", "one two\r\nthree\r\n")
	vim(e, "jdd")
	textIs(t, e, "one two\r\n")
	vim(e, "wD")
	textIs(t, e, "one \r\n")
	vim(e, "yyp")
	textIs(t, e, "one \r\none \r\n")
}
