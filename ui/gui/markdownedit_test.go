package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// A double click on a rendered block edits its Markdown in place; typing writes the document, which
// the source column shows at once, and Escape shows the block rendered again.
func TestMarkdownBlockEditsInPlace(t *testing.T) {
	source := "# Title\n\nFirst paragraph.\n\n- one\n- two\n"
	h := newHarness(t, map[string]string{"doc.md": source}, Options{Appearance: "light"})
	h.open("doc.md")
	if err := h.tt.Click("在右侧打开预览"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(h.root, "doc.md")
	m := waitMarkdown(t, h, path)
	b, ok := h.tt.Find("First paragraph.")
	if !ok {
		t.Fatal("paragraph missing")
	}
	v := m.viewport(1)
	doubleClick(t, h, v, path, b.X+b.W/2, b.Y+b.H/2)
	if v.edit == nil || v.edit.text != "First paragraph." || !h.tt.Focused("编辑 Markdown 块") {
		t.Fatalf("no in-place edit: %+v", v.edit)
	}
	h.tt.Key(ui.Cmd, ui.KeyRight) // to the end of the line, inside the box
	h.tt.Type(" More **bold** 中文")
	h.settle()
	want := "# Title\n\nFirst paragraph. More **bold** 中文\n\n- one\n- two\n"
	if got := string(h.f.frame.Columns[1].Pane.Markdown); got != want {
		t.Fatalf("document %q", got)
	}
	if !h.f.frame.Columns[0].Tabs[0].Dirty {
		t.Fatal("the source tab must show the edit as unsaved")
	}
	h.shot("markdown-edit")
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	h.settle()
	if v.edit != nil || !h.tt.HasText("bold") || h.tt.HasText("**bold**") {
		t.Fatal("Escape must show the block rendered")
	}
	// A single click or a drag still selects text rather than editing.
	b, _ = h.tt.Find("two")
	h.tt.ClickAt(b.X+b.W/2, b.Y+b.H/2)
	h.settle()
	if v.edit != nil {
		t.Fatal("a single click started an edit")
	}
	b, _ = h.tt.Find("one")
	h.tt.Press(b.X+1, b.Y+b.H/2)
	h.tt.Move(b.X+b.W-1, b.Y+b.H/2)
	h.tt.Release(b.X+b.W-1, b.Y+b.H/2)
	h.settle()
	if v.edit != nil {
		t.Fatal("a drag started an edit")
	}
	// Undo in the source column walks the preview's typing back.
	h.ed.FocusColumn(0)
	h.ed.Run("undo")
	h.settle()
	if got := string(h.f.frame.Columns[1].Pane.Markdown); strings.Contains(got, "中文") && got == want {
		t.Fatalf("undo did not reach the preview's edit: %q", got)
	}
}

// doubleClick gives the preview's list the press of a double click at (x, y) in the window: the
// headless tester has no click count, which the system supplies in a real window.
func doubleClick(t *testing.T, h *harness, v *markdownViewport, path string, x, y float32) {
	t.Helper()
	list, ok := h.tt.Find("预览 " + path)
	if !ok {
		t.Fatal("no preview list")
	}
	v.trackClick(ui.InputEvent{Kind: ui.InputPointerDown, Clicks: 2, X: x - list.X, Y: y - list.Y})
	h.settle()
}

// The edit box's document keys: Cmd/Ctrl+B bolds the selection (and leaves the sidebar alone),
// Enter continues a list item, Tab nests it.
func TestMarkdownEditBoxFormattingKeys(t *testing.T) {
	source := "# Title\n\nFirst paragraph.\n\n- one\n- two\n"
	h := newHarness(t, map[string]string{"doc.md": source}, Options{Appearance: "light"})
	h.open("doc.md")
	if err := h.tt.Click("在右侧打开预览"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(h.root, "doc.md")
	m := waitMarkdown(t, h, path)
	v := m.viewport(1)
	mod := ui.Ctrl
	if h.f.Profile().MacKeys() {
		mod = ui.Cmd
	}
	doc := func() string { return string(h.f.frame.Columns[1].Pane.Markdown) }

	b, _ := h.tt.Find("First paragraph.")
	doubleClick(t, h, v, path, b.X+b.W/2, b.Y+b.H/2)
	sidebar := h.f.frame.Sidebar.Visible
	h.tt.Key(mod, ui.KeyA)
	h.tt.Key(mod, ui.KeyB)
	h.settle()
	if got := doc(); got != "# Title\n\n**First paragraph.**\n\n- one\n- two\n" || h.f.frame.Sidebar.Visible != sidebar {
		t.Fatalf("bold: %q (sidebar toggled: %v)", got, h.f.frame.Sidebar.Visible != sidebar)
	}
	h.tt.Key(mod, ui.KeyB) // again: unbold, the selection kept on the words
	h.settle()
	if got := doc(); got != source {
		t.Fatalf("unbold: %q", got)
	}
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	h.settle()

	b, _ = h.tt.Find("one")
	doubleClick(t, h, v, path, b.X+b.W/2, b.Y+b.H/2)
	if v.edit == nil || v.edit.text != "- one" {
		t.Fatalf("list item edit: %+v", v.edit)
	}
	h.tt.Key(mod, ui.KeyRight)
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Type("一点五")
	h.tt.Key(0, ui.KeyTab)
	h.settle()
	if got := doc(); got != "# Title\n\nFirst paragraph.\n\n- one\n  - 一点五\n- two\n" {
		t.Fatalf("list: %q", got)
	}
	h.shot("markdown-format")
}

// Typing "/" at the start of a line in the edit box offers blocks; Enter inserts the chosen one.
func TestMarkdownSlashMenuInsertsBlocks(t *testing.T) {
	h := newHarness(t, map[string]string{"doc.md": "# Title\n\nFirst.\n"}, Options{Appearance: "light"})
	h.open("doc.md")
	if err := h.tt.Click("在右侧打开预览"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(h.root, "doc.md")
	m := waitMarkdown(t, h, path)
	v := m.viewport(1)
	mod := ui.Ctrl
	if h.f.Profile().MacKeys() {
		mod = ui.Cmd
	}
	b, _ := h.tt.Find("First.")
	doubleClick(t, h, v, path, b.X+b.W/2, b.Y+b.H/2)
	h.tt.Key(mod, ui.KeyRight)
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Type("/dai")
	h.settle()
	if v.edit.slash == nil || !h.tt.HasText("代码块") || !h.tt.HasText("待办") {
		t.Fatalf("no block menu: %+v", v.edit.slash)
	}
	h.shot("markdown-slash")
	h.tt.Key(0, ui.KeyDown) // 待办 → 代码块
	h.tt.Key(0, ui.KeyDown) // and round again to 待办
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Type("买菜")
	h.settle()
	if got := string(h.f.frame.Columns[1].Pane.Markdown); got != "# Title\n\nFirst.\n\n- [ ] 买菜\n" {
		t.Fatalf("document %q", got)
	}
	// Esc closes only the menu; the edit goes on. (Enter twice: the empty task ends the list.)
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Type("/")
	h.settle()
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	if v.edit == nil || v.edit.slash != nil {
		t.Fatal("Esc must close the menu and keep editing")
	}
}

// Cmd/Ctrl+V with an image (and no text) on the clipboard saves it beside the document and
// links it where the caret is.
func TestMarkdownPasteImage(t *testing.T) {
	h := newHarness(t, map[string]string{"doc.md": "# Title\n\nSee:\n"}, Options{Appearance: "light"})
	h.open("doc.md")
	if err := h.tt.Click("在右侧打开预览"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(h.root, "doc.md")
	m := waitMarkdown(t, h, path)
	v := m.viewport(1)
	b, _ := h.tt.Find("See:")
	doubleClick(t, h, v, path, b.X+b.W/2, b.Y+b.H/2)
	h.tt.Key(h.f.uiMod(), ui.KeyRight)
	h.settle()
	h.f.clipImage = func() []byte { return []byte("\x89PNG fake") }
	h.tt.Key(h.f.uiMod(), ui.KeyV)
	h.settle()
	got := string(h.f.frame.Columns[1].Pane.Markdown)
	if !strings.HasPrefix(got, "# Title\n\nSee:![](assets/doc-") || !strings.HasSuffix(got, ".png)\n") {
		t.Fatalf("document %q", got)
	}
	link := got[strings.Index(got, "](")+2 : strings.LastIndex(got, ")")]
	if data, err := os.ReadFile(filepath.Join(h.root, filepath.FromSlash(link))); err != nil || string(data) != "\x89PNG fake" {
		t.Fatalf("saved image: %q %v", data, err)
	}
	// Text on the clipboard pastes as text: no image is saved.
	h.f.clipImage = func() []byte { return nil }
	h.tt.Key(h.f.uiMod(), ui.KeyV)
	h.settle()
	if n, _ := os.ReadDir(filepath.Join(h.root, "assets")); len(n) != 1 {
		t.Fatalf("assets %d", len(n))
	}
}
