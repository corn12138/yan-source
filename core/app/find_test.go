package app

import (
	"testing"

	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/style"
	"github.com/corn12138/yan-source/core/view"
)

func TestFindStepsAndReplaces(t *testing.T) {
	e := newGoEditor(t, "|foo bar Foo foo\n")
	e.Handle(cmdKey('f', keymap.ModCtrl))
	typeText(e, "foo")
	var f view.Frame
	e.Frame(&f)
	if !f.Find.Visible || f.Find.Count != "1 / 3" || f.Focus != view.FocusFind {
		t.Fatalf("find = %+v focus %v", f.Find, f.Focus)
	}
	if got := shown(e); got != "[foo] bar Foo foo\n" {
		t.Fatalf("first match not selected: %q", got)
	}
	// 区分大小写后只剩两个
	e.Handle(cmdKey('c', keymap.ModCtrl|keymap.ModAlt))
	e.Frame(&f)
	if f.Find.Count != "1 / 2" {
		t.Fatalf("count = %q", f.Find.Count)
	}
	e.Handle(key(keymap.KeyEnter, 0))
	if got := shown(e); got != "foo bar Foo [foo]\n" {
		t.Fatalf("enter should go to the next match: %q", got)
	}
	// 匹配在行里标出来，当前匹配更亮
	var hits, cur int
	for _, sp := range f.Columns[0].Pane.Rows[0].Spans {
		switch sp.Style {
		case style.SearchHit:
			hits++
		case style.SearchCurrent:
			cur++
		}
	}
	if hits != 1 || cur != 1 {
		t.Fatalf("hits %d current %d", hits, cur)
	}
	// 替换全部（作为一步撤销）
	e.Handle(cmdKey('f', keymap.ModCtrl|keymap.ModAlt)) // 打开替换
	typeText(e, "baz")
	e.Handle(KeyInput{keymap.Chord{Mods: keymap.ModCtrl | keymap.ModAlt, Key: keymap.KeyEnter}})
	if got := content(e); got != "baz bar Foo baz\n" {
		t.Fatalf("replace all = %q", got)
	}
	e.Handle(key(keymap.KeyEsc, 0))
	e.Frame(&f)
	if f.Find.Visible {
		t.Fatal("Esc should close find")
	}
	e.Handle(ctrl('z'))
	if got := content(e); got != "foo bar Foo foo\n" {
		t.Fatalf("undo = %q", got)
	}
}

func TestFindRegexGroups(t *testing.T) {
	e := newGoEditor(t, "|a1 b22\n")
	e.Run("toggleFindRegex")
	e.Run("editor.action.startFindReplaceAction")
	e.SetFindQuery(`([a-z])(\d+)`)
	e.SetFindReplace("$2$1")
	e.Run("editor.action.replaceAll")
	if got := content(e); got != "1a 22b\n" {
		t.Fatalf("got %q", got)
	}
}

func TestFindSelectionRequestPreservesDocument(t *testing.T) {
	for _, platform := range []string{"darwin", "windows", "linux"} {
		t.Run(platform, func(t *testing.T) {
			e := newGoEditor(t, "a[中文]z\n")
			mod := keymap.ModCtrl
			if platform == "darwin" {
				mod = keymap.ModSuper
			}
			e.profile = keymap.Profile{OS: platform, GUI: true, Mods: []keymap.Mod{mod}}
			e.compileKeys(e.cfg)
			before := shown(e)
			var frame view.Frame
			e.Handle(cmdKey('f', mod))
			e.Frame(&frame)
			gen := frame.Find.SelectGen
			if gen == 0 || frame.Find.Query != "中文" {
				t.Fatalf("find=%+v", frame.Find)
			}
			e.Handle(cmdKey('f', mod))
			e.Frame(&frame)
			if frame.Find.SelectGen <= gen || shown(e) != before {
				t.Fatal("repeated find must request input selection without moving the document selection")
			}
			gen = frame.Find.SelectGen
			e.FocusFind(int(view.FindQuery))
			e.Frame(&frame)
			if frame.Find.SelectGen != gen {
				t.Fatal("mouse focus must not request select-all")
			}
		})
	}
}
