package gui

import (
	"testing"

	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
)

func TestFindRequestedFocusWinsDuringLayout(t *testing.T) {
	h := newHarness(t, map[string]string{"a.txt": "token\n"}, Options{})
	want := view.FindField(0)
	h.tt = ui.NewTester(func(c *ui.Context) {
		h.f.view(c)
		if want != 0 {
			// Inspect before deferred native focus requests can mask a stale core focus.
			var frame view.Frame
			h.ed.Frame(&frame)
			if frame.Find.Focus != want {
				t.Fatalf("layout replaced focus request: got %v, want %v", frame.Find.Focus, want)
			}
		}
	}, 1200, 780)
	h.open("a.txt")
	h.ed.SetFindQuery("token")
	h.ed.Run("actions.find")
	h.settle()
	for _, step := range []struct {
		command string
		field   view.FindField
	}{
		{"editor.action.startFindReplaceAction", view.FindReplace},
		{"actions.find", view.FindQuery},
		{"editor.action.startFindReplaceAction", view.FindReplace},
	} {
		want = step.field
		h.ed.Run(step.command)
		h.settle()
		want = 0
	}
}

func TestFindSelectsOnEveryRequest(t *testing.T) {
	h := newHarness(t, map[string]string{"a.txt": "token 中文 words\n"}, Options{})
	h.open("a.txt")
	h.ed.SetFindQuery("token")
	for _, reopen := range []bool{false, true, false} {
		if reopen {
			h.ed.Run("yan.find.close")
			h.settle()
		}
		h.ed.Run("actions.find")
		h.settle()
		h.tt.Type("中文")
		h.settle()
		if got := h.f.frame.Find.Query; got != "中文" {
			t.Fatalf("reopen=%v: query=%q", reopen, got)
		}
		h.tt.Type("x")
		h.settle()
		if got := h.f.frame.Find.Query; got != "中文x" {
			t.Fatalf("selection repeated without request: %q", got)
		}
	}
	h.ed.SetFindReplace("原替换")
	for range 2 {
		h.ed.Run("editor.action.startFindReplaceAction")
		h.settle()
		if !h.tt.Focused("替换为") {
			t.Fatalf("replacement input not focused: %+v, selected generation %d", h.f.frame.Find, h.f.findSelectGen)
		}
		h.tt.Type("新值")
		h.settle()
		if got := h.f.frame.Find.With; got != "新值" {
			t.Fatalf("replacement=%q", got)
		}
	}
	h.ed.Run("actions.find")
	h.settle()
	h.tt.Compose("nihao", 5)
	h.settle()
	if got := h.f.frame.Find.Query; got != "中文" {
		t.Fatalf("composition changed query: %q", got)
	}
	h.tt.Type("你好")
	h.settle()
	if got := h.f.frame.Find.Query; got != "你好" {
		t.Fatalf("composition commit=%q", got)
	}
	if h.f.frame.Columns[0].Tabs[0].Dirty {
		t.Fatal("find input edited source document")
	}
}

func TestFindFollowsTabPressAcrossColumns(t *testing.T) {
	h := newHarness(t, map[string]string{"a.txt": "one\n", "b.txt": "two two\n"}, Options{})
	h.open("a.txt")
	h.ed.Run("workbench.action.splitEditor")
	h.open("b.txt")
	h.ed.FocusColumn(0)
	h.ed.Run("actions.find")
	h.ed.SetFindQuery("two")
	h.settle()
	for _, tc := range []struct {
		name  string
		col   int
		count string
	}{
		{"b.txt", 1, "1 / 2"}, {"a.txt", 0, "无结果"}, {"b.txt", 1, "1 / 2"},
	} {
		r, ok := h.tt.Find("关闭 " + tc.name)
		if !ok {
			t.Fatalf("missing tab %s", tc.name)
		}
		h.tt.Press(r.X-24, r.Y+r.H/2)
		h.settle()
		h.tt.Release(r.X-24, r.Y+r.H/2)
		h.settle()
		if h.f.frame.Active != tc.col || h.f.frame.Find.Count != tc.count {
			t.Fatalf("column %d find %+v", h.f.frame.Active, h.f.frame.Find)
		}
		h.ed.Run("actions.find")
		h.settle()
		h.tt.Type("two")
		h.settle()
		if h.f.frame.Find.Query != "two" || h.f.frame.Find.Focus != view.FindQuery {
			t.Fatal("find input lost after moving columns")
		}
	}
	for _, col := range h.f.frame.Columns {
		for _, tab := range col.Tabs {
			if tab.Dirty {
				t.Fatal("column change edited a document")
			}
		}
	}
}

// The foreign payload goes through the real drop targets in the destination's
// test renderer; native OS drag delivery is outside this headless regression.
func TestForeignWindowDropsStayAtSource(t *testing.T) {
	src := newHarness(t, map[string]string{"source.txt": "source"}, Options{})
	src.open("source.txt")
	panel := &termSession{title: "foreign", dir: src.root, failed: true}
	src.f.terms = []*termSession{panel}
	id := src.ed.OpenTerminalTab(0, 1, src.root, "source terminal")
	src.f.tabTerms = map[int]*termSession{id: {id: id, failed: true}}
	src.ed.ClickTab(0, 0, false)
	src.settle()
	for _, kind := range []string{"file", "terminal", "panel"} {
		for _, target := range []string{"tab", "body", "left", "right", "panel"} {
			t.Run(kind+"/"+target, func(t *testing.T) {
				dst := newHarness(t, map[string]string{"dest.txt": "destination"}, Options{})
				dst.open("dest.txt")
				d := tabDrag{f: src.f, col: 0, at: 0}
				if kind == "panel" {
					d.s = panel
				}
				if kind == "terminal" {
					// Same numeric terminal IDs in different editors must not imply ownership.
					id := dst.ed.OpenTerminalTab(0, 1, dst.root, "local")
					dst.f.tabTerms = map[int]*termSession{id: {id: id, failed: true}}
					dst.ed.ClickTab(0, 0, false)
					d.at = 1
				}

				dst.tt = ui.NewTester(func(c *ui.Context) {
					ui.Column(c).Fill().Children(func() {
						ui.Box(c).Height(30).Label("foreign drag").Drag(d)
						dst.f.dragTermFrames = 2
						dst.f.view(c)
					})
				}, 1200, 780)
				dst.settle()
				tabs := len(dst.f.frame.Columns[0].Tabs)
				r, _ := dst.tt.Find("foreign drag")
				pane := dst.f.cols[0].size
				x, y := r.X+20, r.Y+r.H/2
				tx, ty := pane.X+pane.W/2, pane.Y+pane.H/2
				switch target {
				case "tab":
					b, _ := dst.tt.Find("关闭 dest.txt")
					tx, ty = b.X-24, b.Y+b.H/2
				case "left":
					tx = pane.X + 8
				case "right":
					tx = pane.X + pane.W - 8
				case "panel":
					b, ok := dst.tt.Find("终端")
					if !ok {
						t.Fatal("missing panel")
					}
					tx, ty = b.X+b.W/2, b.Y+b.H/2
				}
				dst.tt.Press(x, y)
				for k := 1; k <= 8; k++ {
					dst.tt.Move(x+(tx-x)*float32(k)/8, y+(ty-y)*float32(k)/8)
					dst.tt.Frame()
				}
				dst.tt.Release(tx, ty)
				dst.settle()
				if len(dst.f.frame.Columns) != 1 || len(dst.f.frame.Columns[0].Tabs) != tabs || len(dst.f.terms) != 0 {
					t.Fatal("foreign drop changed destination ownership")
				}
				if len(src.f.terms) != 1 || src.f.terms[0] != panel || panel.closed || panel.id != 0 {
					t.Fatal("foreign panel moved or closed")
				}
				if dst.f.dragTerm(d) != 0 {
					t.Fatal("foreign terminal resolved against local IDs")
				}
			})
		}
	}
}
