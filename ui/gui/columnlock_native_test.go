//go:build selftest

package gui

import (
	"testing"

	"github.com/egoist/mygo/plugins/terminal"
)

// The panel's terminal moves into a locked right column as the same session: no new shell.
func TestNativeTerminalMovesToLockedRight(t *testing.T) {
	h := newHarness(t, sample, Options{})
	defer h.ed.Close()
	defer h.f.closeTerminals()
	h.open("main.go")
	conn := &termTestConn{closed: make(chan struct{})}
	term, err := terminal.New(terminal.Options{Conn: conn, NoBlink: true})
	if err != nil {
		t.Fatal(err)
	}
	s := &termSession{t: term, dir: h.root, reviewRoot: h.root, title: "panel"}
	h.f.terms, h.f.termActive, h.f.panelOpen = []*termSession{s}, 0, true
	h.settle()
	h.ed.Run("view.terminalLockRight")
	h.settle()
	cols := h.f.frame.Columns
	last := cols[len(cols)-1]
	if len(cols) != 2 || !last.Locked || len(last.Tabs) != 1 || last.Tabs[0].Terminal == 0 {
		t.Fatalf("columns %d, locked %v", len(cols), last.Locked)
	}
	if h.f.tabTerms[last.Tabs[0].Terminal] != s || len(h.f.terms) != 0 || s.closed {
		t.Fatal("the terminal was not moved as the same session")
	}
	h.ed.FocusColumn(1)
	h.open("README.md")
	if c := h.f.frame.Columns; len(c[1].Tabs) != 1 || len(c[0].Tabs) != 2 {
		t.Fatal("a file opened in the locked terminal column")
	}
}
