package gui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

func TestCloseDecisionRetainsRestartFlow(t *testing.T) {
	old := closeDialog
	t.Cleanup(func() { closeDialog = old })
	h := newHarness(t, map[string]string{"a.txt": "original"}, Options{})
	defer h.ed.Close()
	h.open("a.txt")
	h.tt.Type("edited ")
	h.settle()
	g := &group{projects: []*Frontend{h.f}}
	closeDialog = func(mygo.MessageOptions) (mygo.MessageResult, error) { return mygo.MessageResult{Button: 2}, nil }
	if g.confirmClose(nil) || !h.ed.Dirty() {
		t.Fatal("cancel lost dirty buffer")
	}
	closeDialog = func(mygo.MessageOptions) (mygo.MessageResult, error) {
		return mygo.MessageResult{}, errors.New("dialog unavailable")
	}
	if g.confirmClose(nil) {
		t.Fatal("dialog error allowed close")
	}
	closeDialog = func(mygo.MessageOptions) (mygo.MessageResult, error) {
		if g.confirmClose(nil) {
			t.Error("nested close accepted")
		}
		return mygo.MessageResult{Button: 0}, nil
	}
	if !g.confirmClose(nil) || h.ed.Dirty() {
		t.Fatal("save should return to pending close/relaunch")
	}
	b, err := os.ReadFile(filepath.Join(h.root, "a.txt"))
	if err != nil || string(b) != "edited original" {
		t.Fatalf("saved: %q, %v", b, err)
	}
	h.tt.Type("more ")
	h.settle()
	if err = os.WriteFile(filepath.Join(h.root, "a.txt"), []byte("external"), 0644); err != nil {
		t.Fatal(err)
	}
	if g.confirmClose(nil) || !h.ed.Dirty() {
		t.Fatal("save conflict allowed restart")
	}
	closeDialog = func(mygo.MessageOptions) (mygo.MessageResult, error) { return mygo.MessageResult{Button: 1}, nil }
	if !g.confirmClose(nil) {
		t.Fatal("discard should allow pending restart")
	}
}
