package gui

import (
	"github.com/egoist/mygo"
	"testing"
)

func TestTerminalLocationMenu(t *testing.T) {
	h := newHarness(t, sample, Options{})
	panel := &termSession{title: "existing", dir: h.root, failed: true}
	h.f.terms = []*termSession{panel}
	var find func([]*mygo.MenuItem) *mygo.MenuItem
	find = func(items []*mygo.MenuItem) *mygo.MenuItem {
		for _, it := range items {
			if it.Label == "默认在编辑区新建终端" {
				return it
			}
			if match := find(it.Submenu); match != nil {
				return match
			}
		}
		return nil
	}
	item := find(h.f.menu().Items())
	if item == nil || item.Checked || item.Type != mygo.MenuItemCheckbox {
		t.Fatal("missing unchecked location option")
	}
	item.Click(item, nil)
	h.settle()
	if !find(h.f.menu().Items()).Checked || h.ed.TerminalLocation() != "editor" {
		t.Fatal("menu did not enable editor location")
	}
	h.ed.Run("workbench.action.terminal.new")
	h.ed.Frame(&h.f.frame)
	id := h.f.frame.Columns[h.f.frame.Active].Pane.Terminal
	if id == 0 {
		t.Fatal("menu preference did not route new terminal to editor")
	}
	// Ownership is checked without loading the native terminal library in headless tests.
	tab := &termSession{id: id, title: "new", dir: h.root, failed: true}
	h.f.tabTerms = map[int]*termSession{id: tab}
	h.settle()
	item = find(h.f.menu().Items())
	item.Click(item, nil)
	h.settle()
	if find(h.f.menu().Items()).Checked || h.ed.TerminalLocation() != "" {
		t.Fatal("menu did not restore panel default")
	}
	if len(h.f.terms) != 1 || h.f.terms[0] != panel || panel.closed || h.f.tabTerms[id] != tab || tab.closed || h.f.panelOpen {
		t.Fatal("changing default moved, closed or showed an existing session")
	}
	h.f.termTheme = "refresh-after-failed-start"
	h.f.restyleTerminals()
	h.shot("terminal-default-location")
}
