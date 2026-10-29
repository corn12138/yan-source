package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// fakeLayouter：ASCII 占 1 格，其余字符占 2 格（足够模拟中文）。
type fakeLayouter struct{}

func (fakeLayouter) Epoch() uint64 { return 0 }
func (fakeLayouter) Layout(dst *view.LineLayout, line []byte, _ view.LayoutStyle) {
	dst.Rows, dst.Stops = dst.Rows[:0], dst.Stops[:0]
	x := view.Unit(0)
	for i, r := range string(line) {
		dst.Stops = append(dst.Stops, view.Stop{Byte: int32(i), X: x})
		if r < 0x80 {
			x++
		} else {
			x += 2
		}
	}
	dst.Stops = append(dst.Stops, view.Stop{Byte: int32(len(line)), X: x})
	dst.Rows = append(dst.Rows, view.Row{End: int32(len(line)), Height: 1})
}

type fakeHost struct {
	clip  string
	shell string
}

func (h *fakeHost) SetClipboard(s string) error { h.clip = s; return nil }
func (h *fakeHost) Clipboard() (string, error)  { return h.clip, nil }
func (h *fakeHost) Shell(dir string) error      { h.shell = dir; return nil }
func (h *fakeHost) Trash(path string) error     { return os.Remove(path) }

func newTestEditor(t *testing.T, content string) (*Editor, *fakeHost, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &fakeHost{}
	e, warns := New(config.Default(), fakeLayouter{}, h, keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl, keymap.ModSuper}})
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}
	e.SetPaneSize(0, view.Size{W: 40, H: 8})
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	var f view.Frame
	e.Frame(&f)
	return e, h, p
}

func ctrl(r rune) KeyInput {
	return KeyInput{keymap.Chord{Mods: keymap.ModCtrl, Key: keymap.KeyRune, Rune: r}}
}
func key(k keymap.Key, m keymap.Mod) KeyInput { return KeyInput{keymap.Chord{Mods: m, Key: k}} }

func content(e *Editor) string { s := e.cur().doc.buf.Snapshot(); return s.String(0, s.Len()) }

// tabNames 返回当前列的标签名，当前标签加 *。
func tabNames(e *Editor) []string {
	var out []string
	c := e.column()
	for i, t := range c.tabs {
		n := filepath.Base(t.doc.buf.Path())
		if t.preview {
			n = "~" + n
		}
		if i == c.active {
			n += "*"
		}
		out = append(out, n)
	}
	return out
}

func TestMultiCursorAddNextMatch(t *testing.T) {
	e, _, _ := newTestEditor(t, "foo bar foo baz foo")
	e.Handle(ctrl('d')) // 选中光标处的词 foo
	e.Handle(ctrl('d')) // 第二个 foo
	e.Handle(ctrl('d')) // 第三个 foo
	e.Handle(TextInput{Text: "x"})
	if got := content(e); got != "x bar x baz x" {
		t.Fatalf("got %q", got)
	}
	e.Handle(ctrl('z'))
	if got := content(e); got != "foo bar foo baz foo" {
		t.Fatalf("undo got %q", got)
	}
}

func TestVerticalMoveKeepsVisualColumn(t *testing.T) {
	// 第 1 行「中文ab」：b 前面是 4+1=5 格；第 2 行全 ASCII，↓ 后应落在第 5 列（x=5）。
	e, _, _ := newTestEditor(t, "中文ab\n0123456789\nx\n0123456789")
	e.Handle(key(keymap.KeyEnd, 0))
	e.Handle(key(keymap.KeyLeft, 0)) // 光标在 b 前
	e.Handle(key(keymap.KeyDown, 0))
	s := e.cur().doc.buf.Snapshot()
	if got := e.cur().doc.buf.Selections().Main().Head - s.LineStart(1); got != 5 {
		t.Fatalf("col after down = %d, want 5", got)
	}
	e.Handle(key(keymap.KeyDown, 0)) // 短行：落到行尾
	e.Handle(key(keymap.KeyDown, 0)) // 再往下：回到第 5 列（GoalX）
	if got := e.cur().doc.buf.Selections().Main().Head - s.LineStart(3); got != 5 {
		t.Fatalf("goal column lost: %d", got)
	}
}

func TestPasteNormalizesEOL(t *testing.T) {
	e, _, _ := newTestEditor(t, "a\r\nb\r\n")
	e.Handle(TextInput{Text: "1\n2\r\n3", Paste: true})
	if got := content(e); got != "1\r\n2\r\n3a\r\nb\r\n" {
		t.Fatalf("got %q", got)
	}
}

func TestChordAndTerminal(t *testing.T) {
	e, h, p := newTestEditor(t, "x")
	e.SetRoot(filepath.Dir(p))
	e.Handle(ctrl('k'))
	if !e.Pending() || e.status == "" {
		t.Fatal("Ctrl+K should be a pending prefix")
	}
	e.Handle(TextInput{Text: "t"}) // 连按第二键作为普通字符到达
	if h.shell != filepath.Dir(p) {
		t.Fatalf("shell dir = %q", h.shell)
	}
	if content(e) != "x" {
		t.Fatal("chord key must not be inserted as text")
	}
}

func TestCopyLineWhenNoSelection(t *testing.T) {
	e, h, _ := newTestEditor(t, "one\ntwo\n")
	e.Handle(ctrl('c'))
	if h.clip != "one\n" {
		t.Fatalf("clip = %q", h.clip)
	}
	e.Handle(key(keymap.KeyDown, 0))
	e.Handle(ctrl('v'))
	if got := content(e); got != "one\none\ntwo\n" {
		t.Fatalf("got %q", got)
	}
}

func TestPostRunsOnOwner(t *testing.T) {
	e, _, _ := newTestEditor(t, "")
	done := make(chan struct{})
	go func() { e.Post(func(e *Editor) { e.status = "from job" }); close(done) }()
	<-done
	<-e.Wake()
	e.RunPending()
	if e.status != "from job" {
		t.Fatal("posted closure did not run")
	}
}
