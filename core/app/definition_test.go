package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

const defMain = `package main

func helper(n int) int {
	total := n
	for i := 0; i < n; i++ {
		total += i
	}
	return total
}

func other() {
	total := 0
	_ = total
}

func main() {
	x := helper(2)
	_ = x
}
`

// caretAt puts the caret on the k-th (from 0) whole-word occurrence of word.
func caretAt(t *testing.T, e *Editor, word string, k int) int {
	t.Helper()
	all := regexp.MustCompile(`\b`+regexp.QuoteMeta(word)+`\b`).FindAllStringIndex(content(e), -1)
	if k >= len(all) {
		t.Fatalf("no %q #%d", word, k)
	}
	at := all[k][0]
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(at + 1)))
	return at
}

// selected is the selected text and the line it starts on (from 0).
func selected(e *Editor) (string, int, string) {
	t := e.cur()
	r := t.doc.buf.Selections().Main()
	s := t.doc.buf.Snapshot()
	return s.String(r.From(), r.To()), s.LineAt(r.From()), filepath.Base(t.doc.buf.Path())
}

func TestDefinitionInTheFile(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"main.go": defMain})
	if err := e.OpenFile(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		word string
		k    int
		line int
		why  string
	}{
		{"helper", 1, 2, "a function declared above"},
		{"total", 2, 3, "the nearest local in the same block"},
		{"total", 4, 11, "a local of another function is not this one"},
		{"n", 2, 2, "a parameter of the enclosing function"},
		{"i", 2, 4, "a variable of a for clause, on its own line"},
		{"i", 3, 4, "a variable of a for clause, in its body"},
	} {
		start := caretAt(t, e, tc.word, tc.k)
		e.Run("editor.action.revealDefinition")
		got, line, _ := selected(e)
		if got != tc.word || line != tc.line {
			t.Errorf("%s (%s #%d at %d): selected %q on line %d, want line %d (%s)", tc.why, tc.word, tc.k, start, got, line, tc.line, e.status)
		}
	}
	caretAt(t, e, "helper", 0)
	e.Run("editor.action.revealDefinition")
	if !strings.Contains(e.status, "就定义在这里") {
		t.Errorf("on the definition itself: %q", e.status)
	}
}

func TestNavigateBackAndForward(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"main.go": defMain})
	e.OpenFile(filepath.Join(root, "main.go"))
	from := caretAt(t, e, "helper", 1) + 1
	e.Run("editor.action.revealDefinition")
	if _, line, _ := selected(e); line != 2 {
		t.Fatalf("jumped to line %d", line)
	}
	e.Run("workbench.action.navigateBack")
	if got := e.cur().doc.buf.Selections().Main(); !got.Empty() || got.Head != from {
		t.Fatalf("back to %+v, want %d", got, from)
	}
	e.Run("workbench.action.navigateForward")
	if _, line, _ := selected(e); line != 2 {
		t.Fatalf("forward to line %d", line)
	}
	e.Run("workbench.action.navigateForward")
	if !strings.Contains(e.status, "没有可以前进") {
		t.Errorf("status %q", e.status)
	}
}

func TestDefinitionInTheFolder(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{
		"cmd/main.go":      "package main\n\nfunc main() {\n\tutil.Format(parse(1))\n\tutil.Missing()\n\tTwice()\n}\n",
		"cmd/parse.go":     "package main\n\nfunc parse(n int) int { return n }\n",
		"other/parse.go":   "package other\n\nfunc parse(n int) int { return n }\n",
		"util/format.go":   "package util\n\n// Format formats.\nfunc Format(n int) string { return \"\" }\n",
		"other/format.go":  "package other\n\nfunc Format(n int) string { return \"\" }\n",
		"other/missing.go": "package other\n\nfunc Missing() {}\n",
		"a/twice.go":       "package a\n\nfunc Twice() {}\n",
		"b/twice.go":       "package b\n\nfunc Twice() {}\n",
		"notes.txt":        "func Format() {}\n",
	})
	main := filepath.Join(root, "cmd/main.go")
	open := func(word string) {
		t.Helper()
		if err := e.OpenFile(main); err != nil {
			t.Fatal(err)
		}
		caretAt(t, e, word, 0)
		e.Run("editor.action.revealDefinition")
	}

	open("Format")
	pump(t, e, func() bool { return e.cur().doc.buf.Path() != main })
	if got, line, file := selected(e); got != "Format" || line != 3 || e.cur().doc.buf.Path() != filepath.Join(root, "util/format.go") {
		t.Fatalf("util.Format went to %s line %d %q", file, line, got)
	}

	open("parse")
	pump(t, e, func() bool { return e.cur().doc.buf.Path() != main })
	if got := e.cur().doc.buf.Path(); got != filepath.Join(root, "cmd/parse.go") {
		t.Fatalf("parse in the same directory went to %s", got)
	}

	// util.Missing is only in another package: a function, not a member, so not it.
	open("Missing")
	pump(t, e, func() bool { return strings.Contains(e.status, "找不到") })

	open("Twice")
	pump(t, e, func() bool { return e.ov != nil })
	if e.ov.kind != view.OverlayChoice || len(e.ov.items) != 2 || !strings.HasPrefix(e.ov.items[0].detail, "a/twice.go:3") {
		t.Fatalf("overlay %v %+v", e.ov.kind, e.ov.items)
	}
	e.Handle(key(keymap.KeyDown, 0))
	e.Handle(key(keymap.KeyEnter, 0))
	if got := e.cur().doc.buf.Path(); got != filepath.Join(root, "b/twice.go") {
		t.Fatalf("chose %s", got)
	}
}

// The caret moving on drops a definition the folder search finds afterwards.
func TestLateDefinitionIsDropped(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{
		"main.go":  "package main\n\nfunc main() { Far() }\n",
		"far.go":   "package main\n",
		"x/far.go": "package x\n\nfunc Far() {}\n",
	})
	main := filepath.Join(root, "main.go")
	e.OpenFile(main)
	caretAt(t, e, "Far", 0)
	e.Run("editor.action.revealDefinition")
	e.cur().doc.buf.SetSelections(edit.Single(edit.Cursor(0)))
	pump(t, e, func() bool { return !strings.Contains(e.status, "正在找") })
	if got := e.cur().doc.buf.Path(); got != main {
		t.Fatalf("jumped to %s after the caret moved", got)
	}
}

// A second folder search stops the first: its scan would only be thrown away.
func TestNewDefinitionSearchStopsTheLast(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{
		"main.go":  "package main\n\nfunc main() { Far(); Near() }\n",
		"x/far.go": "package x\n\nfunc Far() {}\n\nfunc Near() {}\n",
	})
	e.OpenFile(filepath.Join(root, "main.go"))
	caretAt(t, e, "Far", 0)
	e.Run("editor.action.revealDefinition")
	first := e.def.stop
	caretAt(t, e, "Near", 0)
	e.Run("editor.action.revealDefinition")
	select {
	case <-first:
	default:
		t.Fatal("the first search kept reading the folder")
	}
	pump(t, e, func() bool { return !strings.Contains(e.status, "正在找") })
	if got := e.cur().doc.buf.Path(); got != filepath.Join(root, "x", "far.go") {
		t.Fatalf("jumped to %s", got)
	}
	if s := e.cur().doc.buf.Snapshot(); s.LineAt(e.sels(e.cur()).Main().Head) != 4 {
		t.Fatal("did not land on Near")
	}
}

func newProfileEditor(t *testing.T, p keymap.Profile, files map[string]string) (*Editor, string) {
	t.Helper()
	root := t.TempDir()
	for n, c := range files {
		os.WriteFile(filepath.Join(root, n), []byte(c), 0o644)
	}
	e, warns := New(config.Default(), fakeLayouter{}, &fakeHost{}, p)
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}
	t.Cleanup(e.Close)
	if err := e.OpenFolder(root); err != nil {
		t.Fatal(err)
	}
	e.SetPaneSize(0, view.Size{W: 90, H: 28})
	var f view.Frame
	e.Frame(&f)
	pump(t, e, func() bool { return !e.ws.indexing })
	return e, root
}

// The modifier of a link click is the profile's: Cmd in the macOS window, Ctrl in the Windows
// and Linux windows, and what the terminal reports in the terminal version.
func TestDefinitionLinkClick(t *testing.T) {
	for _, tc := range []struct {
		name      string
		p         keymap.Profile
		link, not keymap.Mod
	}{
		{"mac window", keymap.Profile{Mods: []keymap.Mod{keymap.ModSuper}, GUI: true}, keymap.ModSuper, keymap.ModCtrl},
		{"windows window", keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}, GUI: true, OS: "windows"}, keymap.ModCtrl, keymap.ModSuper},
		{"linux window", keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}, GUI: true, OS: "linux"}, keymap.ModCtrl, keymap.ModSuper},
		{"terminal", keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}}, keymap.ModCtrl, keymap.ModAlt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, root := newProfileEditor(t, tc.p, map[string]string{"main.go": defMain})
			if got := e.LinkModifier(); got != tc.link {
				t.Fatalf("link modifier %v, want %v", got, tc.link)
			}
			e.OpenFile(filepath.Join(root, "main.go"))
			var f view.Frame
			e.Frame(&f)
			s := e.cur().doc.buf.Snapshot()
			g := e.gutter(s)
			at := strings.Index(content(e), "helper(2)")
			line := s.LineAt(at)
			x, y := g+view.Unit(at-s.LineStart(line)+2), view.Unit(line)
			col := view.Unit(at - s.LineStart(line))
			from, to, ok := e.LinkAt(0, x, y)
			if !ok || from != g+col || to != g+col+6 {
				t.Fatalf("link at %d,%d: %d..%d %v (gutter %d)", x, y, from, to, ok, g)
			}
			if _, _, ok := e.LinkAt(0, g+40, y); ok {
				t.Error("a link past the end of the line")
			}
			e.Handle(MouseInput{Column: 0, X: x, Y: y, Mods: tc.not, Kind: MousePress})
			e.Handle(MouseInput{Column: 0, Kind: MouseRelease})
			if _, l, _ := selected(e); l != line {
				t.Fatalf("a %v click jumped to line %d", tc.not, l)
			}
			e.Handle(MouseInput{Column: 0, X: x, Y: y, Mods: tc.link, Kind: MousePress})
			e.Handle(MouseInput{Column: 0, Kind: MouseRelease})
			if got, l, _ := selected(e); got != "helper" || l != 2 {
				t.Fatalf("link click selected %q on line %d (%s)", got, l, e.status)
			}
		})
	}
}

func TestPlatformNavigateKeys(t *testing.T) {
	e, _ := newProfileEditor(t, keymap.Profile{Mods: []keymap.Mod{keymap.ModSuper}, GUI: true}, nil)
	for seq, want := range map[string]string{
		"ctrl+-": "workbench.action.navigateBack", "ctrl+shift+-": "workbench.action.navigateForward",
		"f12": "editor.action.revealDefinition", "cmd+-": "workbench.action.zoomOut", "alt+left": "cursorWordLeft",
	} {
		cs, _ := keymap.Parse(seq, keymap.ModSuper)
		if got, m := e.keys.Resolve(keymap.CtxEditor, cs); m != keymap.MatchExact || got != want {
			t.Errorf("%s = %q, want %q", seq, got, want)
		}
	}
}
