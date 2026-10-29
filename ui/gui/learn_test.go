package gui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/view"
)

// The guide in the window: a click starts a lesson, its practice file opens, and pressing the
// shortcut the step names ticks the step off.
func TestLearnPanelLesson(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := newHarness(t, sample, Options{})
	if err := h.tt.Click("学习：功能导览"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.f.frame.Sidebar.View != view.SideLearn || !h.tt.HasText("编辑基础") {
		t.Fatalf("guide not shown: view %v", h.f.frame.Sidebar.View)
	}
	if err := h.tt.Click("课程 编辑基础"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	col := h.f.frame.Columns[0]
	if col.Tabs[col.Active].Name != "编辑基础.go" {
		t.Fatalf("practice file not open: %+v", col.Tabs)
	}
	lg := h.f.frame.Sidebar.Learn
	if lg.Active != 0 || !lg.Steps[0].Current || lg.Steps[0].Keys == "" {
		t.Fatalf("first step %+v", lg.Steps)
	}
	mod := ui.Ctrl
	if h.f.Profile().MacKeys() {
		mod = ui.Cmd
	}
	h.tt.Key(mod, ui.KeyL) // 选中当前行
	h.settle()
	if lg := h.f.frame.Sidebar.Learn; !lg.Steps[0].Done || !lg.Steps[1].Current {
		t.Fatalf("the key did not complete the step: %+v", lg.Steps[:2])
	}
	if !strings.Contains(strings.Join(h.tt.Texts(), "|"), "把这一行向下复制一份") {
		t.Fatal("next step not shown")
	}
	h.shot("learn-panel")
}

// The welcome page offers the guide.
func TestWelcomeOffersTheGuide(t *testing.T) {
	h := newHarness(t, map[string]string{}, Options{})
	if err := h.tt.Click("功能导览"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.f.frame.Sidebar.View != view.SideLearn {
		t.Fatal("the welcome link did not open the guide")
	}
}
