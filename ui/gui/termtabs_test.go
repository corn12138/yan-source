package gui

import (
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

// These checks exercise ownership without starting a shell or requiring the native terminal library.
func TestTerminalOwnership(t *testing.T) {
	h := newHarness(t, sample, Options{})
	defer h.ed.Close()
	s := &termSession{title: "shell", dir: h.root}
	h.f.terms = []*termSession{s}
	h.f.panelToColumn(s, 0, 0)
	id := s.id
	if id == 0 || h.f.tabTerms[id] != s || len(h.f.terms) != 0 || s.closed {
		t.Fatal("panel move lost ownership")
	}
	h.ed.Frame(&h.f.frame)
	h.f.reapTerminals()
	if s.closed {
		t.Fatal("live tab reaped")
	}
	h.f.columnToPanel(id)
	if s.id != 0 || len(h.f.terms) != 1 || h.f.terms[0] != s || s.closed {
		t.Fatal("column move lost ownership")
	}
	h.f.panelToColumn(s, 0, 0)
	id = s.id
	h.ed.CloseTab(0, 0)
	h.ed.Frame(&h.f.frame)
	h.f.reapTerminals()
	if !s.closed || len(h.f.tabTerms) != 0 {
		t.Fatal("closed tab retained its shell")
	}
	// A queued exit from that shell must not close a new terminal created meanwhile.
	newer := h.ed.OpenTerminalTab(0, 0, h.root, "shell")
	h.f.terminalExited(s)
	var fr view.Frame
	h.ed.Frame(&fr)
	if fr.Columns[0].Tabs[0].Terminal != newer {
		t.Fatal("late exit removed new terminal")
	}
}

func TestFailedTerminalCanClose(t *testing.T) {
	h := newHarness(t, sample, Options{})
	defer h.ed.Close()
	id := h.ed.OpenTerminalTab(0, 0, h.root, "failed shell")
	s := &termSession{id: id, failed: true, title: "failed shell"}
	h.f.tabTerms = map[int]*termSession{id: s}
	h.settle()
	if _, ok := h.tt.Find("终端未能启动，请关闭此标签后重试"); !ok {
		t.Fatal("no failure message")
	}
	if err := h.tt.Click("关闭 failed shell"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !s.closed || len(h.f.tabTerms) != 0 {
		t.Fatal("failed terminal cannot be closed")
	}
}
