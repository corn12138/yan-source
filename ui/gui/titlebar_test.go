package gui

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAcceleratorKey(t *testing.T) {
	for acc, want := range map[string]struct {
		mods ui.Modifiers
		key  ui.Key
		ok   bool
	}{
		"Ctrl+Shift+P": {ui.Ctrl | ui.Shift, ui.KeyP, true},
		"Cmd+S":        {ui.Super, ui.KeyS, true},
		"Alt+1":        {ui.Alt, ui.Key1, true},
		"F5":           {0, ui.KeyF5, true},
		"Ctrl+`":       {0, 0, false},
		"":             {0, 0, false},
	} {
		mods, key, ok := acceleratorKey(acc)
		if mods != want.mods || key != want.key || ok != want.ok {
			t.Errorf("%q: %v %v %v", acc, mods, key, ok)
		}
	}
}

// On Windows and Linux the title bar has a menu button that opens the app's menu in the window:
// its submenus, and items that run Yan's commands.
func TestTitleBarMenuButton(t *testing.T) {
	old := nativeMenuBar
	nativeMenuBar = func() bool { return false }
	defer func() { nativeMenuBar = old }()
	h := newHarness(t, map[string]string{"a.txt": "a\n"}, Options{Appearance: "dark"})
	h.settle()
	h.shot("titlebar-menu-button")
	if err := h.tt.Click("菜单"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	menu := h.tt.Menu()
	for _, want := range []string{"文件", "编辑", "Git", "窗口", "帮助"} {
		if !slices.Contains(menu, want) {
			t.Fatalf("no %s in %v", want, menu)
		}
	}
	if err := h.tt.ChooseMenuItem("编辑", "全选"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	h.open("a.txt")
	if err := h.tt.Click("菜单"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if err := h.tt.ChooseMenuItem("文件", "关闭标签"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	for _, c := range h.f.frame.Columns {
		if len(c.Tabs) != 0 {
			t.Fatalf("the tab is still open: %+v", c.Tabs)
		}
	}
}
