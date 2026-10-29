package gui

import (
	"fmt"
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitMarkdown(t *testing.T, h *harness, path string) *markdownState {
	t.Helper()
	end := time.Now().Add(5 * time.Second)
	for time.Now().Before(end) {
		h.settle()
		if m := h.f.markdowns[path]; m != nil && m.loaded && !m.running {
			return m
		}
	}
	t.Fatal("Markdown parse did not finish")
	return nil
}

func TestMarkdownReadingWorkflow(t *testing.T) {
	source := "# Read me\n\n[Jump](#last)\n\n> [!TIP]\n> Keep the text.\n\n```go\nfmt.Println(\"你好\")\n```\n\n| Key | Description |\n|---|---|\n| a | A much longer description |\n\n" + strings.Repeat("A paragraph to scroll.\n\n", 45) + "## Last\n\nThe end.\n"
	h := newHarness(t, map[string]string{"readme.md": source}, Options{Appearance: "light"})
	h.open("readme.md")
	if err := h.tt.Click("在右侧打开预览"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(h.root, "readme.md")
	m := waitMarkdown(t, h, path)
	if len(h.f.frame.Columns) != 2 || h.f.frame.Active != 0 || h.f.frame.Columns[1].Pane.Kind != view.PaneMarkdown {
		t.Fatal("side preview/focus")
	}
	if !h.tt.HasText("提示") {
		t.Fatal("alert title absent")
	}
	if err := h.tt.Click("复制代码"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got := h.tt.Clipboard(); got != "fmt.Println(\"你好\")\n" {
		t.Fatalf("code clipboard %q", got)
	}
	// A real selectable paragraph must copy its rendered text, without editing the source.
	b, ok := h.tt.Find("Keep the text.")
	if !ok {
		t.Fatal("paragraph missing")
	}
	h.tt.Press(b.X+1, b.Y+b.H/2)
	h.tt.Move(b.X+b.W-1, b.Y+b.H/2)
	h.tt.Release(b.X+b.W-1, b.Y+b.H/2)
	h.tt.Key(ui.Cmd, ui.KeyC)
	h.settle()
	if !strings.Contains(h.tt.Clipboard(), "Keep") {
		t.Fatalf("selection copy %q", h.tt.Clipboard())
	}
	if err := h.tt.Click("打开链接 #last"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if m.viewport(1).scroll.Y <= 0 || !h.tt.HasText("Last") {
		t.Fatal("anchor did not scroll")
	}
	if string(h.f.frame.Columns[1].Pane.Markdown) != source {
		t.Fatal("preview mutated source")
	}
	h.shot("markdown-anchor")
	m.viewport(1).touched = time.Time{}
	h.ed.ScrollSourceTo(0, path, 0)
	h.settle()
	h.shot("markdown-reading")
}

func TestMarkdownScrollBothWays(t *testing.T) {
	var doc strings.Builder
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&doc, "## Section %d\n\nParagraph %d.\n\n", i, i)
	}
	h := newHarness(t, map[string]string{"readme.md": doc.String()}, Options{})
	h.open("readme.md")
	h.ed.Run("markdown.showPreviewToSide")
	path := filepath.Join(h.root, "readme.md")
	m := waitMarkdown(t, h, path)
	h.ed.ScrollSourceTo(0, path, 80)
	h.settle()
	v := m.viewport(1)
	if v.scroll.Y <= 0 || v.sourceLine != 80 {
		t.Fatalf("source did not lead: %v / %d", v.scroll.Y, v.sourceLine)
	}
	b, ok := h.tt.Find("预览 " + path)
	if !ok {
		t.Fatal("preview bounds")
	}
	h.tt.Scroll(b.X+b.W/2, b.Y+b.H/2, 0, 260)
	h.settle()
	top := h.f.frame.Columns[0].Pane.TopLine
	if top <= 80 {
		t.Fatalf("preview did not lead: %d", top)
	}
	// After momentum expires, an unchanged frame must not bounce back to the prior source location.
	v.touched = time.Now().Add(-time.Second)
	h.settle()
	if h.f.frame.Columns[0].Pane.TopLine != top {
		t.Fatal("scroll feedback loop")
	}
	if h.f.frame.Active != 0 || h.f.frame.Columns[0].Pane.Current != 0 {
		t.Fatal("scroll moved focus/caret")
	}
}

func TestMarkdownAnchorCollisionAndCodeText(t *testing.T) {
	blocks := parseMarkdown([]byte("# Foo\n# Foo-1\n# Foo\n```text\n x\t\n```\n"), "x.md", "", nil)
	seen := map[string]bool{}
	for _, b := range blocks {
		if b.slug != "" {
			if seen[b.slug] {
				t.Fatalf("duplicate anchor %s", b.slug)
			}
			seen[b.slug] = true
		}
		if b.kind == markdownCodeBlock && b.codeText != " x\t\n" {
			t.Fatalf("code whitespace %q", b.codeText)
		}
	}
}

// A footnote reference scrolls the preview to its note, and the note's back link scrolls back.
func TestMarkdownFootnoteLinks(t *testing.T) {
	var body strings.Builder
	body.WriteString("The start[^1].\n\n")
	for i := range 80 {
		body.WriteString(fmt.Sprintf("Paragraph %d.\n\n", i))
	}
	body.WriteString("[^1]: The note.\n")
	h := newHarness(t, map[string]string{"notes.md": body.String()}, Options{})
	h.open("notes.md")
	if err := h.tt.Click("在右侧打开预览"); err != nil {
		t.Fatal(err)
	}
	m := waitMarkdown(t, h, filepath.Join(h.root, "notes.md"))
	if h.tt.HasText("The note.") {
		t.Fatal("the note shows already")
	}
	if err := h.tt.Click("打开链接 #user-content-fn-1"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.tt.HasText("The note.") || m.viewport(1).scroll.Y <= 0 {
		t.Fatal("the reference did not scroll to its note")
	}
	h.shot("markdown-footnote")
	if err := h.tt.Click("打开链接 #user-content-fnref-1"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.tt.HasText("Paragraph 0.") {
		t.Fatal("the back link did not scroll to the reference")
	}
}
