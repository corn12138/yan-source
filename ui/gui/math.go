package gui

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"errors"
	"html"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"

	"github.com/dop251/goja"
	"github.com/egoist/mygo/ui"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"

	"github.com/corn12138/yan-source/core/i18n"
)

// Adapted from Mayoi 1010308 (MIT, Copyright (c) 2026 chencheng).
// The runtime has no host I/O; document text is passed as data to the bundled renderer.

// mathJS is MathJax, which typesets TeX and draws it as SVG, its glyphs
// paths, bundled by scripts/mathjax.sh into one script that sets the
// global YanMath.
//
//go:embed mathjax.js.gz
var mathJS []byte

// mathFormula is a formula drawn, in ems of the text it is in: as wide
// as w, reaching ascent above the baseline and depth below it.
type mathFormula struct {
	svg              *ui.SVG
	w, ascent, depth float32
	display          bool
	// err is why the formula could not be drawn, "" when it was.
	err string
}

func (f *mathFormula) draw(pt *ui.Painter, r ui.Rect, color ui.Color) {
	if f.svg != nil && r.W > 0 {
		pt.Icon(f.svg, r, color)
	}
}

type mathKey struct {
	tex     string
	display bool
}

const mathTimeout = 5 * time.Second

// goja runtimes are not safe to share concurrently. Laziness keeps documents
// without formulas from paying the renderer startup cost.
var mathjax struct {
	sync.Mutex
	vm     *goja.Runtime
	render goja.Callable
	err    error
}

func mathStart() error {
	zr, err := gzip.NewReader(bytes.NewReader(mathJS))
	if err != nil {
		return err
	}
	defer zr.Close()
	script, err := io.ReadAll(zr)
	if err != nil {
		return err
	}
	vm := goja.New()
	if _, err := vm.RunScript("mathjax.js", string(script)); err != nil {
		return err
	}
	render, ok := goja.AssertFunction(vm.Get("YanMath").ToObject(vm).Get("render"))
	if !ok {
		return errors.New("mathjax: no render")
	}
	mathjax.vm, mathjax.render = vm, render
	return nil
}

var errMathTimeout = errors.New("timeout")

func mathSVG(tex string, display bool) (string, error) {
	return mathSVGWithin(tex, display, mathTimeout)
}

func mathSVGWithin(tex string, display bool, timeout time.Duration) (string, error) {
	if len(tex) > 32*1024 {
		return "", errors.New(i18n.T("The formula is too long (maximum 32 KiB)"))
	}
	mathjax.Lock()
	defer mathjax.Unlock()
	if mathjax.vm == nil && mathjax.err == nil {
		mathjax.err = mathStart()
	}
	if mathjax.err != nil {
		return "", mathjax.err
	}
	vm := mathjax.vm
	done := make(chan struct{})
	stop := time.AfterFunc(timeout, func() { vm.Interrupt(errMathTimeout); close(done) })
	v, err := mathjax.render(goja.Undefined(), vm.ToValue(tex), vm.ToValue(display))
	// Wait for a fired timer before clearing it, so it cannot interrupt the next formula.
	if !stop.Stop() {
		<-done
	}
	vm.ClearInterrupt()
	if err != nil {
		var interrupted *goja.InterruptedError
		if errors.As(err, &interrupted) {
			mathjax.vm, mathjax.render = nil, nil
			return "", errors.New(i18n.T("Formula rendering timed out"))
		}
		return "", err
	}
	return v.String(), nil
}

var (
	mathViewBox = regexp.MustCompile(`viewBox="([^"]*)"`)
	mathError   = regexp.MustCompile(`data-mjx-error="([^"]*)"`)
	mathSize    = regexp.MustCompile(` (width|height)="[^"]*"`)
)

func renderMath(tex string, display bool) *mathFormula {
	svg, err := mathSVG(tex, display)
	if err != nil {
		return &mathFormula{display: display, err: err.Error()}
	}
	if m := mathError.FindStringSubmatch(svg); m != nil {
		return &mathFormula{display: display, err: html.UnescapeString(m[1])}
	}
	// MyGo's SVGs have no text: MathJax writes in it what its fonts lack.
	if strings.Contains(svg, "<text") {
		return &mathFormula{display: display, err: i18n.T("The formula contains characters outside the bundled math font")}
	}
	m := mathViewBox.FindStringSubmatch(svg)
	if m == nil {
		return &mathFormula{display: display, err: i18n.T("The formula has no valid size")}
	}
	box := strings.Fields(m[1])
	var v [4]float64
	for i := range min(len(box), 4) {
		var err error
		v[i], err = strconv.ParseFloat(box[i], 64)
		if err != nil || math.IsNaN(v[i]) || math.IsInf(v[i], 0) {
			return &mathFormula{display: display, err: i18n.T("The formula has no valid size")}
		}
	}
	if len(box) != 4 || v[2] <= 0 || v[3] <= 0 {
		return &mathFormula{display: display, err: i18n.T("The formula has no valid size")}
	}
	// The root is as large as its view box, in thousandths of an em,
	// rather than in ex, which MyGo does not measure.
	end := strings.IndexByte(svg, '>')
	if end < 0 {
		return &mathFormula{display: display, err: i18n.T("The formula has no valid size")}
	}
	root := mathSize.ReplaceAllString(svg[:end], "")
	root += ` width="` + box[2] + `" height="` + box[3] + `"`
	s, err := ui.ParseSVG([]byte(root + svg[end:]))
	if err != nil {
		return &mathFormula{display: display, err: err.Error()}
	}
	// The view box starts its height above the baseline.
	return &mathFormula{svg: s, display: display, w: float32(v[2] / 1000), ascent: float32(-v[1] / 1000), depth: float32((v[1] + v[3]) / 1000)}
}

// mathSpan is the span of a formula within text of size: a box it is
// drawn in, as wide as it and as high as it reaches above the baseline.
func mathSpan(f *mathFormula, size float32) ui.Span {
	return ui.Span{Text: "￼", ObjectWidth: max(f.w*size, 1), ObjectHeight: max(f.ascent*size, 0)}
}

// mathPlaces finds where the formulas of a text drawn as a RichText of
// spans in font, width wide, its lines lineHeight high, stand in it: the
// boxes of the formulas, in their order, below their tops by their
// depths.
//
// The text's own layout is out of reach, so spans are laid out again,
// in the font's own line spacing, and their lines spaced as the text's:
// as high as lineHeight or what is on them, centered in that height.
func mathPlaces(spans []ui.Span, font ui.Font, width, lineHeight float32) []ui.Rect {
	return mathPlacesAligned(spans, font, width, lineHeight, ui.Start)
}

func mathPlacesAligned(spans []ui.Span, font ui.Font, width, lineHeight float32, align ui.Align) []ui.Rect {
	tl := ui.ShapeRichTextWith(spans, font, ui.TextShape{Width: width, Align: align})
	objects := tl.Objects()
	if len(objects) == 0 {
		return nil
	}
	n := 0
	for _, s := range spans {
		for _, r := range s.Text {
			n += utf16.RuneLen(r)
		}
	}
	type line struct{ y, h, top float32 }
	var lines []line
	top := float32(0)
	for i := 0; i <= n; i++ {
		c := tl.Caret(i)
		if len(lines) > 0 && lines[len(lines)-1].y == c.Y {
			continue
		}
		lines = append(lines, line{c.Y, c.H, top})
		top += max(lineHeight, c.H)
	}
	out := make([]ui.Rect, len(objects))
	for i, o := range objects {
		base := o.Rect.Y + o.Rect.H
		l := lines[0]
		for _, k := range lines {
			if base >= k.y {
				l = k
			}
		}
		out[i] = ui.Rect{X: o.Rect.X, Y: l.top + o.Rect.Y - l.y + (max(lineHeight, l.h)-l.h)/2, W: o.Rect.W, H: o.Rect.H}
	}
	return out
}

type mathNode struct {
	ast.BaseInline
	tex     string
	display bool
}

var kindMath = ast.NewNodeKind("Math")

func (n *mathNode) Kind() ast.NodeKind { return kindMath }

func (n *mathNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"TeX": n.tex}, nil)
}

// mathParser parses the formulas of text as GitHub does: $$ to $$, or $
// to a $ that is not after a space nor before a digit, the first not
// before a space, the TeX between them left as it is.
type mathParser struct{}

func (mathParser) Trigger() []byte { return []byte{'$'} }

func (mathParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	l, pos := block.Position()
	line, _ := block.PeekLine()
	fence := 1
	if len(line) > 1 && line[1] == '$' {
		fence = 2
	}
	if len(line) <= fence || fence == 1 && (line[1] == ' ' || line[1] == '\t' || line[1] == '\n') {
		return nil
	}
	block.Advance(fence)
	// The formula may go on over the lines of its paragraph.
	var tex []byte
	for {
		line, _ := block.PeekLine()
		if line == nil {
			// No end: the $ is a dollar.
			block.SetPosition(l, pos)
			return nil
		}
		for i := 0; i < len(line); i++ {
			switch {
			case line[i] == '\\':
				i++
				continue
			case line[i] != '$' || fence == 2 && (i+1 >= len(line) || line[i+1] != '$'):
				continue
			}
			tex = append(tex, line[:i]...)
			ok := len(bytes.TrimSpace(tex)) > 0
			if fence == 1 && ok {
				last := tex[len(tex)-1]
				ok = ok && last != ' ' && last != '\t' && last != '\n' && (i+1 >= len(line) || line[i+1] < '0' || line[i+1] > '9')
			}
			if !ok {
				block.SetPosition(l, pos)
				return nil
			}
			block.Advance(i + fence)
			return &mathNode{tex: string(bytes.TrimSpace(tex)), display: fence == 2}
		}
		tex = append(tex, line...)
		block.AdvanceLine()
	}
}
