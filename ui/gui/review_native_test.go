//go:build selftest

package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
)

func TestNativeTerminalReviewHandoff(t *testing.T) {
	h := newHarness(t, sample, Options{})
	defer h.ed.Close()
	defer h.f.closeTerminals()
	h.open("main.go")
	makeTerm := func() (*termSession, *termTestConn) {
		t.Helper()
		conn := &termTestConn{closed: make(chan struct{})}
		term, err := terminal.New(terminal.Options{Conn: conn, NoBlink: true})
		if err != nil {
			t.Fatal(err)
		}
		return &termSession{t: term, dir: h.root, reviewRoot: h.root, title: "review-shell"}, conn
	}
	first, firstConn := makeTerm()
	last, lastConn := makeTerm()
	h.f.terms = []*termSession{first, last}
	h.f.termActive = 0
	h.f.lastReviewTerm = last
	value := "review line\nsecond\r\n\x1b[201~end\t\x03"
	if sent, err := (host{h.f}).PasteReview(value); err != nil || sent {
		t.Fatal("unbracketed terminal accepted multiline review")
	}
	if lastConn.text() != "" {
		t.Fatal("fallback wrote to terminal")
	}
	last.t.Feed([]byte("\x1b[?2004h"))
	if sent, err := (host{h.f}).PasteReview(value); err != nil || !sent {
		t.Fatalf("paste: %v %v", sent, err)
	}
	want := "\x1b[200~review line\nsecond\n[201~end\t\x1b[201~"
	deadline := time.Now().Add(time.Second)
	for lastConn.text() != want && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := lastConn.text(); got != want {
		t.Fatalf("paste bytes: %q", got)
	}
	if firstConn.text() != "" || !h.f.panelOpen || h.f.termActive != 1 {
		t.Fatal("paste did not target last focused project terminal")
	}
	h.f.panelToColumn(last, 0, 0)
	h.settle()
	h.open("main.go")
	h.ed.Run("review.comment")
	h.ed.SetReviewInput("terminal tab review")
	h.ed.Run("review.save")
	h.settle()
	h.ed.Run("review.send")
	h.settle()
	col := h.f.frame.Columns[0]
	if col.Tabs[col.Active].Terminal != last.id || h.f.frame.ReviewCount != 0 {
		t.Fatal("terminal tab not focused or successful handoff not cleared")
	}
	deadline = time.Now().Add(time.Second)
	for !strings.Contains(lastConn.text(), "terminal tab review") && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !strings.Contains(lastConn.text(), "terminal tab review") {
		t.Fatal("review did not reach tab")
	}
	last.close()
	if h.f.reviewTerminal() != first {
		t.Fatal("closed terminal remained eligible")
	}
	if err := h.ed.SwitchFolder(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if h.f.reviewTerminal() != nil {
		t.Fatal("folder switch offered the old project's panel")
	}
	foreign, _ := makeTerm()
	defer foreign.close()
	h.f.lastReviewTerm = foreign
	if h.f.reviewTerminal() == foreign {
		t.Fatal("another project's terminal accepted review")
	}
}
