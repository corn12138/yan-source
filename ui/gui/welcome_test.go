package gui

import "testing"

// With a folder open the welcome page lists the ways around it, and a click on a row does it.
func TestWelcomeHintsAreButtons(t *testing.T) {
	h := newHarness(t, sample, Options{})
	hints := h.f.frame.Columns[0].Pane.Hints
	titles := map[string]string{}
	for _, x := range hints {
		titles[x.Title] = x.Keys
	}
	if _, ok := titles["快速打开文件"]; !ok || titles["打开文件夹"] != "" || titles["在文件中搜索"] == "" {
		t.Fatalf("hints: %+v", hints)
	}
	h.shot("welcome-hints")
	if err := h.tt.Click("快速打开文件"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.f.frame.Overlay.Visible {
		t.Fatal("clicking the hint did not open quick open")
	}
}
