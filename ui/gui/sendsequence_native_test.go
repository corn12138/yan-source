//go:build selftest

package gui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
)

// keybindings.json's sendSequence reaches the real terminal widget that last had the keyboard.
func TestNativeTerminalSendSequence(t *testing.T) {
	h := newHarness(t, sample, Options{})
	defer h.ed.Close()
	defer h.f.closeTerminals()
	conn := &termTestConn{closed: make(chan struct{})}
	term, err := terminal.New(terminal.Options{Conn: conn, NoBlink: true})
	if err != nil {
		t.Fatal(err)
	}
	s := &termSession{t: term, dir: h.root, reviewRoot: h.root, title: "keys"}
	if (host{h.f}).SendTerminal("x") {
		t.Fatal("sent without a terminal")
	}
	h.f.terms, h.f.termActive, h.f.panelOpen = []*termSession{s}, 0, true
	if !(host{h.f}).SendTerminal("\x03echo hi\r") {
		t.Fatal("not sent")
	}
	deadline := time.Now().Add(time.Second)
	for conn.text() != "\x03echo hi\r" && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := conn.text(); got != "\x03echo hi\r" {
		t.Fatalf("terminal got %q", got)
	}
}
