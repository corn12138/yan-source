package gui

import (
	"github.com/dop251/goja"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/i18n"
)

func TestRenderMath(t *testing.T) {
	for _, tex := range []string{`E=ma^2`, `\frac{a}{b}+\sqrt{x^2+1}`, `\begin{pmatrix}1&2\\3&4\end{pmatrix}`, `\text{if } x>0`, `\boldsymbol{x}`} {
		for _, display := range []bool{false, true} {
			f := renderMath(tex, display)
			if f.err != "" || f.svg == nil || f.w <= 0 || f.ascent <= 0 || f.depth < 0 {
				t.Errorf("%s: %+v", tex, f)
			}
		}
	}
	if f := renderMath(`\frac{a}{b}`, false); f.depth < 0.3 {
		t.Errorf("a fraction does not reach below the baseline: %+v", f)
	}
	if f := renderMath(`\nosuchmacro`, false); !strings.Contains(f.err, `\nosuchmacro`) {
		t.Errorf("err = %q", f.err)
	}
	if f := renderMath(`\text{中文}`, false); f.err != i18n.T("The formula contains characters outside the bundled math font") {
		t.Errorf("err = %q", f.err)
	}
}

// TestMarkdownMath: GitHub's math, within text and in blocks of its own,
// parses as formulas, and the dollars that are not math stay text.
func TestMarkdownMath(t *testing.T) {
	parse := func(src string) []markdownBlock {
		return parseMarkdown([]byte(src), "", "", nil, &markdownCache{math: true})
	}
	for _, c := range []struct{ src, tex string }{
		{"$$E=ma^2$$\n", "E=ma^2"},
		{"$$\na_1 + b_1 = c_1\n$$\n", "a_1 + b_1 = c_1"},
		{"```math\n\\frac{1}{2}\n```\n", `\frac{1}{2}`},
	} {
		blocks := parse(c.src)
		if len(blocks) != 1 || blocks[0].kind != markdownMathBlock || blocks[0].formula == nil || blocks[0].formula.err != "" ||
			!blocks[0].formula.display || markdownRunsText(blocks[0].runs) != c.tex {
			t.Errorf("%q: %+v", c.src, blocks)
		}
	}
	for _, c := range []struct {
		src  string
		math []string
	}{
		{"Mass $E=mc^2$ here, and $a_1 + b_1$ *too*.\n", []string{"E=mc^2", "a_1 + b_1"}},
		{"Across $a +\nb$ lines\n", []string{"a +\nb"}},
		{"Inline $$\\sum_i x_i$$ display\n", []string{`\sum_i x_i`}},
		{"It costs $5 and $6, or $ y$.\n", nil},
		{"Escaped \\$x$ here\n", nil},
		{"`$x$` and $ and $$\n", nil},
		{"$x$5\n", nil},
		{"$$$$ $$ $\n", nil},
	} {
		blocks := parse(c.src)
		var math []string
		for _, b := range blocks {
			if b.kind != markdownParagraph {
				t.Errorf("%q: block %+v", c.src, b)
			}
			for _, r := range b.runs {
				if r.math != nil {
					if r.math.err != "" {
						t.Errorf("%q: %s", c.src, r.math.err)
					}
					math = append(math, strings.Trim(r.text, "$"))
				}
				if r.italic && r.text != "too" {
					t.Errorf("%q: %q is italic", c.src, r.text)
				}
			}
		}
		if strings.Join(math, "|") != strings.Join(c.math, "|") {
			t.Errorf("%q: math %q, want %q", c.src, math, c.math)
		}
	}
	if blocks := parse("It costs $5 and $6.\n"); markdownRunsText(blocks[0].runs) != "It costs $5 and $6." {
		t.Errorf("dollars: %+v", blocks)
	}
	if words, _ := markdownWords(parse("One $x + y$ two\n\n$$z$$\n")); words != 2 {
		t.Errorf("words = %d, counting math", words)
	}

	// Without math, the text is as it was.
	blocks := parseMarkdown([]byte("$$E=ma^2$$ and $a_1 b_1$\n"), "", "", nil, &markdownCache{})
	if len(blocks) != 1 || markdownRunsText(blocks[0].runs) != "$$E=ma^2$$ and $a_1 b_1$" {
		t.Errorf("math off: %+v", blocks)
	}
}

// TestMathPlaces: a formula stands where the text puts its box, on the
// baseline of its line, in lines spaced as the text spaces them.
func TestMathPlaces(t *testing.T) {
	f := renderMath(`\frac{a}{b}`, false)
	font := ui.Font{Size: 20}
	object := mathSpan(f, font.Size)
	spans := []ui.Span{{Text: strings.Repeat("word ", 30)}, object, {Text: " end"}}
	tl := ui.ShapeRichText(spans, font, 300)
	objects := tl.Objects()
	if len(objects) != 1 || objects[0].Rect.Y <= 0 {
		t.Fatalf("objects %+v", objects)
	}
	// In the font's own spacing, the boxes are the layout's.
	if got := mathPlaces(spans, font, 300, 0); len(got) != 1 || got[0] != objects[0].Rect {
		t.Errorf("places %+v, want %+v", got, objects[0].Rect)
	}
	// Taller lines move it down by what the lines above grew by, and half
	// what its own did.
	natural := tl.Caret(0).H
	line := tl.Caret(len(strings.Repeat("word ", 30)) + 1)
	lines := line.Y / natural
	got := mathPlaces(spans, font, 300, 40)
	want := objects[0].Rect.Y + lines*(40-natural) + (max(40, line.H)-line.H)/2
	if len(got) != 1 || got[0].X != objects[0].Rect.X || abs32(got[0].Y-want) > 0.01 {
		t.Errorf("places %+v, want y %v", got, want)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

const mathMarkdown = "# Math\n\nThe energy $E=mc^2$ of a mass, and a fraction $\\frac{a}{b}$ within text that wraps " +
	"over more than one line of the preview, $\\sqrt{x^2+y^2}$ at its end.\n\n" +
	"$$\n\\int_0^\\infty e^{-x^2}\\,dx = \\frac{\\sqrt{\\pi}}{2}\n$$\n\n" +
	"```math\n\\begin{aligned}a &= b + c \\\\ d &= e\\end{aligned}\n```\n\n" +
	"$$\\nosuchmacro$$\n\nIt costs $5 and $6.\n"

func waitMath(t *testing.T, h *harness, path string, enabled bool) *markdownState {
	t.Helper()
	for end := time.Now().Add(30 * time.Second); time.Now().Before(end); {
		h.settle()
		if m := h.f.markdowns[path]; m != nil && m.loaded && !m.running && m.wait == nil && m.cache.math == enabled {
			return m
		}
	}
	t.Fatal("formula preview did not settle")
	return nil
}

func TestMathPreview(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, appearance := range []string{"light", "dark"} {
		h := newHarness(t, map[string]string{"math.md": mathMarkdown}, Options{Appearance: appearance})
		h.tt.SetSize(1000, 1100)
		h.open("math.md")
		h.ed.Run("yan.svg.togglePreview")
		path := filepath.Join(h.root, "math.md")
		m := waitMath(t, h, path, true)
		blocks, inline := 0, 0
		for _, b := range m.blocks {
			if b.kind == markdownMathBlock {
				blocks++
			}
			for _, r := range b.runs {
				if r.math != nil && r.math.err == "" {
					inline++
				}
			}
		}
		if blocks != 3 || inline != 3 {
			t.Fatalf("blocks %d inline %d", blocks, inline)
		}
		if !h.tt.HasText("无法绘制此公式：") || !h.tt.HasText("It costs $5 and $6.") {
			t.Fatal("missing error or currency")
		}
		h.shot("math-" + appearance)
		var display markdownBlock
		for _, b := range m.blocks {
			if b.formula != nil && b.formula.err == "" {
				display = b
				break
			}
		}
		box, ok := h.tt.Find(i18n.F("Formula: %s", display.codeText))
		if !ok || box.W <= 0 || box.H <= 0 {
			t.Fatalf("formula not visible: %+v", box)
		}
		h.tt.Move(box.X+box.W/2, box.Y+box.H/2)
		h.settle()
		if err := h.tt.Click("复制代码"); err != nil {
			t.Fatal(err)
		}
		h.settle()
		if h.tt.Clipboard() != display.codeText {
			t.Fatal("formula copy changed source")
		}
		h.tt.Move(box.X+box.W/2, box.Y+box.H/2)
		h.settle()
		if err := h.tt.Click("显示源码"); err != nil {
			t.Fatal(err)
		}
		h.settle()
		if !h.tt.HasText(display.codeText) {
			t.Fatal("show source failed")
		}
		if err := h.tt.Click("显示公式"); err != nil {
			t.Fatal(err)
		}
		h.settle()
		h.tt.SetSize(620, 1100)
		h.settle()
		h.shot("math-narrow-" + appearance)
		if err := os.WriteFile(filepath.Join(h.root, ".yan.toml"), []byte("[ui]\npreview_math = false\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		m = waitMath(t, h, path, false)
		for _, b := range m.blocks {
			if b.formula != nil || b.kind == markdownMathBlock {
				t.Fatal("math remained after disabling")
			}
			for _, r := range b.runs {
				if r.math != nil {
					t.Fatal("inline math remained")
				}
			}
		}
		if !h.tt.HasText("$E=mc^2$") {
			t.Fatal("raw source not restored")
		}
		if err := os.WriteFile(filepath.Join(h.root, ".yan.toml"), []byte("[ui]\npreview_math = true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		waitMath(t, h, path, true)
	}
}

func TestMathIsolationAndRecovery(t *testing.T) {
	if f := renderMath(`\def\privateMacro{x}\privateMacro`, false); f.err != "" {
		t.Fatal(f.err)
	}
	if f := renderMath(`\privateMacro`, false); f.err == "" {
		t.Fatal("macro leaked between formulas")
	}
	for _, tex := range []string{`\require{html}`, `\href{https://example.invalid}{x}`, `\includegraphics{file:///etc/passwd}`} {
		if f := renderMath(tex, false); f.err == "" {
			t.Errorf("unsupported package accepted: %s", tex)
		}
	}
	if _, err := mathSVG(strings.Repeat("x", 32769), false); err == nil {
		t.Fatal("unbounded input")
	}
	// An interrupted VM must not poison the next document's render.
	mathjax.Lock()
	vm := goja.New()
	v, err := vm.RunString(`(function(){while(true){}})`)
	if err != nil {
		t.Fatal(err)
	}
	mathjax.vm = vm
	mathjax.render, _ = goja.AssertFunction(v)
	mathjax.err = nil
	mathjax.Unlock()
	if _, err := mathSVGWithin("x", false, time.Millisecond); err == nil {
		t.Fatal("infinite renderer did not time out")
	}
	if f := renderMath("x+1", false); f.err != "" {
		t.Fatal("timeout broke later formula:", f.err)
	}
	mathjax.Lock()
	defer mathjax.Unlock()
	for _, name := range []string{"fetch", "XMLHttpRequest", "require", "process"} {
		if v := mathjax.vm.Get(name); v != nil && !goja.IsUndefined(v) {
			t.Errorf("host API %s exposed", name)
		}
	}
}

func TestMathCacheAndCancellation(t *testing.T) {
	c := &markdownCache{math: true}
	parse := func(src string) []markdownBlock { return parseMarkdown([]byte(src), "a.md", "", nil, c) }
	a := parse("$x$ $x$")
	if a[0].runs[0].math != a[0].runs[2].math {
		t.Fatal("duplicate formula not shared")
	}
	before := a[0].runs[0].math
	if b := parse("$x$"); b[0].runs[0].math != before {
		t.Fatal("unchanged formula rerendered")
	}
	parse("$y$")
	if _, ok := c.formulas[mathKey{"x", false}]; ok {
		t.Fatal("removed formula retained")
	}
	parseMarkdown([]byte("$never$"), "a.md", "", func() bool { return false }, c)
	if _, ok := c.formulas[mathKey{"never", false}]; ok {
		t.Fatal("cancelled parse rendered formula")
	}
	c.math = false
	parse("$y$")
	if len(c.formulas) != 0 {
		t.Fatal("disabled formulas retained")
	}
}

func TestMathTableAlignmentAndWidth(t *testing.T) {
	f := renderMath(`\frac{x}{y}`, false)
	spans := []ui.Span{{Text: "cell "}, mathSpan(f, 20), {Text: " end"}}
	font := ui.Font{Size: 20}
	for _, align := range []ui.Align{ui.Start, ui.Center, ui.End} {
		want := ui.ShapeRichTextWith(spans, font, ui.TextShape{Width: 300, Align: align}).Objects()
		got := mathPlacesAligned(spans, font, 300, 0, align)
		if len(got) != 1 || got[0] != want[0].Rect {
			t.Fatalf("alignment %v: %v vs %v", align, got, want)
		}
	}
	span := fitMathSpan(ui.Span{ObjectWidth: 600, ObjectHeight: 60}, 200)
	if span.ObjectWidth != 200 || span.ObjectHeight != 20 {
		t.Fatal(span)
	}
}

func TestMathPreviewEditUndo(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, src := range []string{"$$x+1$$\n", "```math\nx+1\n```\n"} {
		h := newHarness(t, map[string]string{"math.md": src}, Options{})
		h.open("math.md")
		h.ed.Run("markdown.showPreviewToSide")
		path := filepath.Join(h.root, "math.md")
		m := waitMath(t, h, path, true)
		b := m.blocks[0]
		box, ok := h.tt.Find(i18n.F("Formula: %s", b.codeText))
		if !ok {
			t.Fatal("formula missing")
		}
		v := m.viewport(1)
		doubleClick(t, h, v, path, box.X+box.W/2, box.Y+box.H/2)
		if v.edit == nil || v.edit.text != strings.TrimSuffix(src, "\n") {
			t.Fatalf("lost formula delimiters: %+v", v.edit)
		}
		h.tt.Key(ui.Cmd, ui.KeyA)
		h.tt.Type(strings.ReplaceAll(strings.TrimSuffix(src, "\n"), "x+1", "x+2"))
		h.settle()
		h.tt.Key(0, ui.KeyEscape)
		waitMath(t, h, path, true)
		if got := string(h.f.frame.Columns[1].Pane.Markdown); got != strings.ReplaceAll(src, "x+1", "x+2") {
			t.Fatalf("edit changed source: %q", got)
		}
		h.ed.FocusColumn(0)
		h.ed.Run("undo")
		h.settle()
		waitMath(t, h, path, true)
		if got := string(h.f.frame.Columns[1].Pane.Markdown); got != src {
			t.Fatalf("undo lost source: %q", got)
		}
	}
}

func TestMathLongInlineResizes(t *testing.T) {
	// A wide rule isolates layout scaling from the renderer time budget under -race.
	h := newHarness(t, map[string]string{"long.md": "$\\rule{200em}{1em}$\n"}, Options{})
	h.open("long.md")
	h.ed.Run("yan.svg.togglePreview")
	path := filepath.Join(h.root, "long.md")
	m := waitMath(t, h, path, true)
	formula := m.blocks[0].runs[0].math
	if formula == nil || formula.err != "" || formula.w < 100 {
		t.Fatalf("oversized fixture: %+v", formula)
	}
	for _, width := range []int{1200, 620, 1000} {
		h.tt.SetSize(width, 780)
		h.settle()
		pl, ok := m.places[&m.blocks[0].runs[0]]
		if !ok || len(pl.places) != 1 || pl.places[0].W > pl.width+1 {
			t.Fatalf("width %d: %+v", width, pl)
		}
	}
}
