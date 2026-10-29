package gui

import (
	"strings"
	"testing"
)

// On Windows and Linux, shared controls name keys the way those systems do: no macOS symbols.
func TestWindowsLabelsUseWindowsKeys(t *testing.T) {
	old := hostOS
	hostOS = "windows"
	t.Cleanup(func() { hostOS = old })
	h := newHarness(t, sample, Options{})
	h.open("main.go")
	h.ed.Run("actions.find")
	h.ed.Run("workbench.action.terminal.toggleTerminal")
	for range 3 {
		h.settle()
	}
	for _, s := range h.tt.Texts() {
		if strings.ContainsAny(s, "⌘⌃⌥⇧↩") {
			t.Errorf("macOS key symbol on Windows: %q", s)
		}
	}
	for _, want := range []string{"收起面板 (Ctrl+`)", "上一个匹配 (Shift+Enter)", "下一个匹配 (Enter)"} {
		if _, ok := h.tt.Find(want); !ok {
			t.Errorf("no control labeled %q", want)
		}
	}
}
