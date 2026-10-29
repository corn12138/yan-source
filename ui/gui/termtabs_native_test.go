//go:build selftest

package gui

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
)

type termTestConn struct {
	mu     sync.Mutex
	input  strings.Builder
	closed chan struct{}
	once   sync.Once
}

func (p *termTestConn) Read([]byte) (int, error) { <-p.closed; return 0, io.EOF }
func (p *termTestConn) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.input.Write(b)
}
func (p *termTestConn) Close() error { p.once.Do(func() { close(p.closed) }); return nil }
func (p *termTestConn) text() string { p.mu.Lock(); defer p.mu.Unlock(); return p.input.String() }

func TestNativeTerminalTabDragAndFocus(t *testing.T) {
	h := newHarness(t, sample, Options{Appearance: "dark"})
	defer h.ed.Close()
	defer h.f.closeTerminals()
	h.open("main.go")
	conn := &termTestConn{closed: make(chan struct{})}
	term, err := terminal.New(terminal.Options{Conn: conn, NoBlink: true})
	if err != nil {
		t.Fatal(err)
	}
	s := &termSession{t: term, dir: h.root, title: "test-shell"}
	h.f.terms = []*termSession{s}
	h.f.panelOpen, h.f.focusTerm = true, true
	h.settle()
	from, ok := h.tt.Find("test-shell")
	to, ok2 := h.tt.Find("关闭 main.go")
	if !ok || !ok2 {
		t.Fatal("missing drag targets")
	}
	drag := func(x, y, tx, ty float32) {
		h.tt.Press(x, y)
		for k := 1; k <= 8; k++ {
			h.tt.Move(x+(tx-x)*float32(k)/8, y+(ty-y)*float32(k)/8)
			h.settle()
		}
		h.tt.Release(tx, ty)
		h.settle()
	}
	drag(from.X+20, from.Y+from.H/2, to.X-24, to.Y+to.H/2)
	if s.id == 0 || h.f.tabTerms[s.id] != s || len(h.f.terms) != 0 {
		t.Fatal("drag into editor did not move existing session")
	}
	term.Feed([]byte("preserved-output\r\n"))
	h.tt.Type("from-tab")
	h.settle()
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(conn.text(), "from-tab") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(conn.text(), "from-tab") {
		t.Fatalf("tab did not receive typing: %q", conn.text())
	}
	// Return to file by clicking its tab, then return to the terminal; focus must follow.
	r, _ := h.tt.Find("关闭 main.go")
	h.tt.ClickAt(r.X-24, r.Y+r.H/2)
	h.settle()
	h.tt.Type("// focus check\n")
	h.settle()
	c := h.f.frame.Columns[0]
	if !c.Tabs[c.Active].Dirty {
		t.Fatal("file did not regain keyboard")
	}
	r, _ = h.tt.Find("关闭 test-shell")
	h.tt.ClickAt(r.X-24, r.Y+r.H/2)
	h.settle()
	h.tt.Type("-again")
	h.settle()
	deadline = time.Now().Add(2 * time.Second)
	for !strings.Contains(conn.text(), "-again") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(conn.text(), "-again") {
		t.Fatal("terminal did not regain keyboard")
	}
	h.shot("terminal-tab")
	// Moving back via context menu must retain emulator output and the same connection.
	h.tt.RightClickAt(r.X-24, r.Y+r.H/2)
	if err := h.tt.ChooseMenuItem("移到面板"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if s.id != 0 || len(h.f.terms) != 1 || h.f.terms[0] != s || !strings.Contains(term.Text(), "preserved-output") {
		t.Fatal("move back restarted terminal")
	}
	h.f.panelToColumn(s, 0, 0)
	h.settle()
	r, _ = h.tt.Find("关闭 test-shell")
	h.tt.Press(r.X-24, r.Y+r.H/2)
	h.tt.Move(r.X-24, r.Y+70)
	h.settle()
	target, ok := h.tt.Find("终端")
	if !ok {
		t.Fatal("collapsed panel did not offer a drop target")
	}
	h.tt.Move(target.X+20, target.Y+target.H/2)
	h.settle()
	h.tt.Release(target.X+20, target.Y+target.H/2)
	h.settle()
	if s.id != 0 || len(h.f.terms) != 1 {
		t.Fatal("drag back to collapsed panel failed")
	}
	h.f.panelToColumn(s, 0, 0)
	h.settle()
	if err := h.tt.Click("关闭 test-shell"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	select {
	case <-conn.closed:
	default:
		t.Fatal("close did not release terminal connection")
	}
}

func TestNativeTerminalColumnFocus(t *testing.T) {
	h := newHarness(t, sample, Options{})
	defer h.ed.Close()
	defer h.f.closeTerminals()
	h.open("main.go")
	conn := &termTestConn{closed: make(chan struct{})}
	term, err := terminal.New(terminal.Options{Conn: conn, NoBlink: true})
	if err != nil {
		t.Fatal(err)
	}
	id := h.ed.OpenTerminalInSplit(0, false, h.root, "shell")
	h.f.tabTerms = map[int]*termSession{id: {t: term, id: id, dir: h.root, title: "shell"}}
	h.settle()
	h.ed.FocusColumn(0)
	h.settle()
	h.tt.Type("// from other column\n")
	h.settle()
	if !h.f.frame.Columns[0].Tabs[0].Dirty {
		t.Fatal("column switch left keyboard in terminal")
	}
	h.ed.FocusColumn(1)
	h.settle()
	h.tt.Type("terminal-column")
	h.settle()
	deadline := time.Now().Add(time.Second)
	for !strings.Contains(conn.text(), "terminal-column") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(conn.text(), "terminal-column") {
		t.Fatal("terminal column did not regain focus")
	}
}
