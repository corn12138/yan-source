//go:build selftest

package gui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
)

// The terminal panel moves between the bottom and the window's sides with its running session:
// at a side it is the outermost pane, beyond the sidebar, and takes its own width.
func TestNativeTerminalPanelSides(t *testing.T) {
	h := newHarness(t, sample, Options{})
	defer h.ed.Close()
	defer h.f.closeTerminals()
	h.open("main.go")
	conn := &termTestConn{closed: make(chan struct{})}
	term, err := terminal.New(terminal.Options{Conn: conn, NoBlink: true})
	if err != nil {
		t.Fatal(err)
	}
	s := &termSession{t: term, dir: h.root, reviewRoot: h.root, title: "shell"}
	h.f.terms, h.f.termActive, h.f.panelOpen = []*termSession{s}, 0, true
	h.settle()
	panel := func() (x, w, y float32) {
		b, ok := h.tt.Find("终端")
		if !ok {
			t.Fatal("no panel")
		}
		return b.X, b.W, b.Y
	}
	sidebar, ok := h.tt.Find("侧边栏")
	if !ok {
		t.Fatal("no sidebar")
	}
	_, _, bottomY := panel()

	h.ed.Run("workbench.action.positionPanelLeft")
	h.settle()
	sidebar, ok = h.tt.Find("侧边栏")
	if !ok {
		t.Fatal("no sidebar beside left panel")
	}
	x, _, y := panel()
	if x >= sidebar.X || y >= bottomY {
		t.Fatalf("left panel at x %v (sidebar %v), y %v", x, sidebar.X, y)
	}
	h.ed.Run("workbench.action.positionPanelRight")
	h.settle()
	x, _, _ = panel()
	editor, _ := h.tt.Find("main.go")
	if x <= editor.X {
		t.Fatalf("right panel at x %v, editor tab at %v", x, editor.X)
	}
	// The same session, still running: what it is sent arrives.
	if len(h.f.terms) != 1 || h.f.terms[0] != s || s.closed {
		t.Fatal("moving the panel replaced its terminal")
	}
	s.t.Send([]byte("still here\r"))
	for end := time.Now().Add(time.Second); conn.text() != "still here\r" && time.Now().Before(end); {
		time.Sleep(time.Millisecond)
	}
	if conn.text() != "still here\r" {
		t.Fatalf("terminal input %q", conn.text())
	}
	h.shot("panel-right")
	h.ed.Run("workbench.action.positionPanelBottom")
	h.settle()
	if _, _, y = panel(); y != bottomY {
		t.Fatalf("back at the bottom: y %v, want %v", y, bottomY)
	}
}
