package app

import (
	"testing"

	"github.com/corn12138/yan-source/core/keymap"
)

func TestEOLKeepsSplitCaretsBeforeNewline(t *testing.T) {
	e, _, _ := newTestEditor(t, "abc\ndef")
	defer e.Close()
	e.Handle(key(keymap.KeyEnd, 0))
	e.Run("workbench.action.splitEditor")
	e.Run("yan.eol.crlf")
	for _, c := range e.cols {
		if c.tab().sels.Main().Head != 3 {
			t.Fatalf("caret split CRLF: %+v", c.tab().sels)
		}
	}
	e.Handle(TextInput{Text: "!"})
	if content(e) != "abc!\r\ndef" {
		t.Fatalf("insert damaged newline: %q", content(e))
	}
}
