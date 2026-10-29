//go:build selftest

package gui

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func newFileTestTerminal(t *testing.T, h *harness) (*termSession, *termTestConn, *ui.Tester) {
	t.Helper()
	s := &termSession{dir: h.root, title: "file-test"}
	conn := &termTestConn{closed: make(chan struct{})}
	opts := h.f.terminalOptions(s)
	opts.Conn, opts.NoBlink = conn, true
	term, err := terminal.New(opts)
	if err != nil {
		t.Fatal(err)
	}
	s.t = term
	t.Cleanup(s.close)
	tt := ui.NewTester(func(c *ui.Context) { terminal.View(c, term).Fill().AutoFocus() }, 800, 300)
	tt.Frame()
	return s, conn, tt
}

func TestNativeTerminalFileLinks(t *testing.T) {
	h := newHarness(t, map[string]string{"main.go": "start\n", "sub/main.go": "one\n中😀x\n"}, Options{})
	s, _, tt := newFileTestTerminal(t, h)
	click := func(output string) {
		t.Helper()
		s.t.Feed([]byte("\x1b[2J\x1b[H" + output))
		tt.Frame()
		// Inside the first cell after the terminal's 6 x 4 DIP padding.
		tt.ClickAtWith(ui.Cmd, 12, 12)
		h.settle()
	}
	click("./main.go:1")
	assertTerminalLocation(t, h, "main.go", "行 1，列 1")
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(h.root, "sub"))}
	s.t.Feed([]byte("\x1b]7;" + u.String() + "\x1b\\"))
	click("./main.go:2:3")
	assertTerminalLocation(t, h, "sub/main.go", "行 2，列 3")
	click("https://example.com/path")
	if got := tt.OpenedURLs(); len(got) != 1 || got[0] != "https://example.com/path" {
		t.Fatalf("URL lost its system handler: %v", got)
	}
	click("./missing.go:2")
	assertTerminalLocation(t, h, "sub/main.go", "行 2，列 3")
	if len(tt.OpenedURLs()) != 1 {
		t.Fatal("missing file was handed to a browser")
	}
	s.t.Feed([]byte("\x1b]7;relative\x1b\\"))
	click("./main.go:1")
	assertTerminalLocation(t, h, "main.go", "行 1，列 1")
}

func TestNativeTerminalFilePaste(t *testing.T) {
	h := newHarness(t, sample, Options{})
	s, conn, tt := newFileTestTerminal(t, h)
	s.t.Feed([]byte("\x1b[?2004h"))
	paths := []string{filepath.Join(h.root, "a b.txt"), filepath.Join(h.root, "it's $(code).txt")}
	if err := (host{h.f}).CopyFiles(paths); err != nil {
		t.Fatal(err)
	}
	tt.SetClipboard("a conflicting text representation")
	tt.Command("paste")
	want := "\x1b[200~" + quotePaths(paths) + "\x1b[201~"
	waitTerminalInput(t, conn, want)
	h.f.copiedFiles = nil
	tt.SetClipboard("ordinary text")
	tt.Command("paste")
	want += "\x1b[200~ordinary text\x1b[201~"
	waitTerminalInput(t, conn, want)
	tt.SetClipboard("")
	tt.Command("paste")
	want += "\x1b[200~\x1b[201~"
	waitTerminalInput(t, conn, want)
	if strings.Contains(conn.text(), "\r") || strings.Contains(conn.text(), "\n") {
		t.Fatal("paste submitted a command")
	}
}

func waitTerminalInput(t *testing.T, conn *termTestConn, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for conn.text() != want && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := conn.text(); got != want {
		t.Fatalf("terminal input %q, want %q", got, want)
	}
}
