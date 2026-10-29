package gui

import (
	"cmp"
	"html"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/egoist/mygo/ui"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"

	"github.com/corn12138/yan-source/core/syntax/highlight"
)

// Parsing and block metadata adapted from Mayoi 46990a1 and 88e00cf (MIT, Copyright (c) 2026 chencheng).
// Image reads stay local; rendering and scroll ownership belong to the frontend.

type markdownInline struct {
	text, link, image               string
	bold, italic, strike, code, raw bool
	bitmap                          ui.ImageSource
	w, h                            float32
	kind                            highlight.Kind
}

type markdownKind uint8

const (
	markdownParagraph markdownKind = iota
	markdownHeading
	markdownRule
	markdownCodeBlock
	markdownHTML
	markdownImageBlock
	markdownTable
	markdownTableHeader
	// markdownAlertTitle is the first row of a GitHub alert, its icon and
	// its kind, as "Note".
	markdownAlertTitle
)

// markdownAlert is the kind of a GitHub alert, a quote that opens with
// [!NOTE] or another of them, 0 for a plain quote.
type markdownAlert uint8

const (
	alertNone markdownAlert = iota
	alertNote
	alertTip
	alertImportant
	alertWarning
	alertCaution
)

var markdownAlerts = map[string]markdownAlert{
	"NOTE": alertNote, "TIP": alertTip, "IMPORTANT": alertImportant, "WARNING": alertWarning, "CAUTION": alertCaution,
}

type markdownImageKey struct {
	path  string
	mtime time.Time
	size  int64
}

// markdownCache keeps the images a parse decoded and the code it
// highlighted, by language and text, for the next parse of the document,
// which drops those it no longer has.
type markdownCache struct {
	images map[markdownImageKey]markdownInline
	code   map[[2]string][]markdownInline
	// mermaid is the style Mermaid blocks are drawn in, nil to show their source;
	// diagrams are those drawn in it, by source.
	mermaid  *mermaidStyle
	diagrams map[string]*mermaidDiagram
}

// markdownBlock is a paragraph-sized row, including its enclosing lists and quotes.
type markdownBlock struct {
	kind                 markdownKind
	offset, markerWidth  float32
	tight                bool
	level, indent, quote int
	marker, language     string
	codeText             string
	runs                 []markdownInline
	cells                [][]markdownInline
	aligns               []east.Alignment
	// table is the first row of the table a row is of, and row the place
	// of a row of its body, from 1; last marks the table's last row,
	// whose corners are round.
	table, row int
	last       bool
	// alert is the kind of the alert the block is in, and alertAt the
	// depth of its quote.
	alert   markdownAlert
	alertAt int
	// slug is a heading's anchor, as GitHub makes it, which a link to
	// #slug scrolls to.
	slug string
	// anchors are the other places a link to #anchor scrolls to: the
	// footnote the block starts and the references to footnotes in it.
	anchors []string
	// footnote is whether the block is of a footnote, smaller and muted.
	footnote bool
	// line is the line of the text the block starts on, from 0, which the
	// preview scrolls with as the text scrolls.
	line int
	// diagram is what a Mermaid block draws, or why it could not.
	diagram *mermaidDiagram
}

// isMermaidFile reports whether path is a Mermaid file, whose preview is
// its one diagram.
func isMermaidFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".mmd" || ext == ".mermaid"
}

func markdownPath(root, path, dest string) string {
	// A whole path of Windows, C:\ or C:/, which a URL would take for one
	// of scheme c. Not a share, as //host/x is a URL without its scheme.
	if len(dest) > 2 && filepath.VolumeName(dest) == dest[:2] && (dest[2] == '\\' || dest[2] == '/') {
		return filepath.Clean(dest)
	}
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Path == "" {
		return ""
	}
	// A URL's slash, which Windows does not take for a whole path.
	if strings.HasPrefix(u.Path, "/") {
		return filepath.Join(cmp.Or(root, "/"), filepath.FromSlash(u.Path))
	}
	return filepath.Join(filepath.Dir(path), filepath.FromSlash(u.Path))
}

// markdownFrontMatter reports whether lines, those between front matter's
// delimiters, look like YAML, so a document opening with a rule keeps the
// text up to its next rule.
func markdownFrontMatter(lines []string) bool {
	keys := false
	for _, line := range lines {
		line = strings.TrimRight(line, "\r\n")
		switch {
		case strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") || line == "-" || strings.HasPrefix(line, "- "):
			if !keys {
				return false
			}
		default:
			key, rest, ok := strings.Cut(line, ":")
			if !ok || strings.TrimSpace(key) == "" || rest != "" && rest[0] != ' ' && rest[0] != '\t' {
				return false
			}
			keys = true
		}
	}
	return keys
}

// markdownParser parses GitHub's Markdown: goldmark's parser with the
// parts of its GFM and footnote extensions that parse, without their HTML
// renderers, which the preview has no use for and would add unused
// rendering code.
func markdownParser() parser.Parser {
	return parser.NewParser(
		parser.WithBlockParsers(append(parser.DefaultBlockParsers(),
			util.Prioritized(extension.NewFootnoteBlockParser(), 999))...),
		parser.WithInlineParsers(append(parser.DefaultInlineParsers(),
			util.Prioritized(extension.NewStrikethroughParser(), 500),
			util.Prioritized(extension.NewTaskCheckBoxParser(), 0),
			util.Prioritized(extension.NewLinkifyParser(), 999),
			util.Prioritized(extension.NewFootnoteParser(), 101))...),
		parser.WithParagraphTransformers(append(parser.DefaultParagraphTransformers(),
			util.Prioritized(extension.NewTableParagraphTransformer(), 200))...),
		parser.WithASTTransformers(util.Prioritized(extension.NewTableASTTransformer(), 0),
			util.Prioritized(urlLinkifier{}, 100), util.Prioritized(extension.NewFootnoteASTTransformer(), 999)),
	)
}

// parseMarkdown converts the AST once, away from the UI thread.
// A parse that current reports superseded keeps the images it skipped cached.
func parseMarkdown(source []byte, path, root string, current func() bool, caches ...*markdownCache) []markdownBlock {
	c := &markdownCache{}
	if len(caches) > 0 && caches[0] != nil {
		c = caches[0]
	}
	if c.images == nil {
		c.images = map[markdownImageKey]markdownInline{}
	}
	if c.code == nil {
		c.code = map[[2]string][]markdownInline{}
	}
	cache := c.images
	used := map[markdownImageKey]bool{}
	usedCode := map[[2]string]bool{}
	usedDiagrams := map[string]bool{}
	defer func() {
		if current != nil && !current() {
			return
		}
		for k := range cache {
			if !used[k] {
				delete(cache, k)
			}
		}
		for k := range c.code {
			if !usedCode[k] {
				delete(c.code, k)
			}
		}
		for k := range c.diagrams {
			if !usedDiagrams[k] {
				delete(c.diagrams, k)
			}
		}
	}()
	// diagram draws a Mermaid block, as the last parse did if it had it.
	diagram := func(text string) *mermaidDiagram {
		if c.mermaid == nil || current != nil && !current() {
			return nil
		}
		usedDiagrams[text] = true
		d, ok := c.diagrams[text]
		if !ok {
			d = renderMermaid(text, *c.mermaid)
			if c.diagrams == nil {
				c.diagrams = map[string]*mermaidDiagram{}
			}
			c.diagrams[text] = d
		}
		return d
	}
	// code highlights a block of code, as the last parse did if it had it.
	code := func(text, language string) []markdownInline {
		key := [2]string{language, text}
		usedCode[key] = true
		runs, ok := c.code[key]
		if !ok {
			runs = markdownCode(text, language)
			runs = runs[:len(runs):len(runs)]
			c.code[key] = runs
		}
		return runs
	}
	if isMermaidFile(path) {
		text := string(source)
		return []markdownBlock{{kind: markdownCodeBlock, language: "mermaid", codeText: text, runs: code(text, "mermaid"), diagram: diagram(text)}}
	}
	// Front matter is recognized only at the start, with exact delimiter lines
	// around what looks like YAML. skip is the lines it took.
	skip := 0
	if lines := strings.SplitAfter(string(source), "\n"); strings.TrimSuffix(strings.TrimSuffix(lines[0], "\n"), "\r") == "---" {
		for i := 1; i < len(lines); i++ {
			line := strings.TrimSuffix(strings.TrimSuffix(lines[i], "\n"), "\r")
			if line == "---" || line == "..." {
				if markdownFrontMatter(lines[1:i]) {
					source = []byte(strings.Join(lines[i+1:], ""))
					skip = i + 1
				}
				break
			}
		}
	}

	tree := markdownParser().Parse(text.NewReader(source))
	// starts are where the lines of source start, for the line of a block.
	starts := []int{0}
	for i, ch := range source {
		if ch == '\n' {
			starts = append(starts, i+1)
		}
	}
	lineOf := func(n ast.Node) int {
		at := markdownStart(n)
		if at < 0 {
			return -1
		}
		i, _ := slices.BinarySearch(starts, at+1)
		return skip + i - 1
	}
	var blocks []markdownBlock
	// anchors are those of the block being walked, which it takes as it is added.
	var anchors []string
	var inline func(ast.Node, markdownInline) []markdownInline
	inline = func(n ast.Node, style markdownInline) []markdownInline {
		var out []markdownInline
		for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
			s := style
			switch x := ch.(type) {
			case *ast.Text:
				s.text = string(x.Segment.Value(source))
				if !x.IsRaw() && strings.ContainsAny(s.text, "\\&") {
					s.text = html.UnescapeString(string(util.UnescapePunctuations([]byte(s.text))))
				}
				if x.HardLineBreak() {
					s.text += "\n"
				} else if x.SoftLineBreak() {
					s.text += " "
				}
			case *ast.String:
				s.text = string(x.Value)
			case *ast.Emphasis:
				if x.Level == 2 {
					s.bold = true
				} else {
					s.italic = true
				}
				out = append(out, inline(ch, s)...)
				continue
			case *east.Strikethrough:
				s.strike = true
				out = append(out, inline(ch, s)...)
				continue
			case *ast.CodeSpan:
				s.code = true
				out = append(out, inline(ch, s)...)
				continue
			case *ast.Link:
				s.link = string(x.Destination)
				out = append(out, inline(ch, s)...)
				continue
			case *ast.AutoLink:
				s.text, s.link = string(x.Label(source)), string(x.URL(source))
			case *ast.Image:
				s.text = string(x.Text(source))
				s.image = string(x.Destination)
				if local := markdownPath(root, path, s.image); local != "" && (current == nil || current()) {
					if info, err := os.Stat(local); err == nil && info.Mode().IsRegular() {
						key := markdownImageKey{local, info.ModTime(), info.Size()}
						used[key] = true
						image, ok := cache[key]
						if !ok {
							if strings.EqualFold(filepath.Ext(local), ".svg") {
								if data, err := os.ReadFile(local); err == nil {
									if svg, err := ui.ParseSVG(data); err == nil {
										image.bitmap = svg
										image.w, image.h = svg.Size()
										if image.w <= 0 || image.h <= 0 {
											image.bitmap = nil
										}
									}
								}
							} else {
								if data, err := os.ReadFile(local); err == nil {
									if bm, err := ui.DecodeBitmap(data); err == nil {
										w, h := bm.Size()
										image.bitmap, image.w, image.h = bm, float32(w), float32(h)
									}
								}
							}
							cache[key] = image
						}
						s.bitmap, s.w, s.h = image.bitmap, image.w, image.h
					}
				}
				if s.text == "" {
					s.text = filepath.Base(s.image)
					if s.text == "." || s.text == "" {
						s.text = "image"
					}
				}
				s.raw = s.bitmap == nil
			case *ast.RawHTML:
				s.raw = true
				s.text = markdownHTMLText(string(x.Segments.Value(source)))
				if tag := strings.ToLower(strings.TrimSpace(s.text)); tag == "<br>" || tag == "<br/>" || tag == "<br />" {
					s.text, s.raw = "\n", false
				}
			case *east.FootnoteLink:
				// A reference to a footnote is its number raised, as
				// GitHub's <sup>, linking to it and linked back to.
				n := strconv.Itoa(x.Index)
				s.text, s.link = markdownSuperscript(n), "#user-content-fn-"+n
				anchors = append(anchors, markdownFootnoteRef(x.Index, x.RefIndex))
			case *east.FootnoteBacklink:
				s.text, s.link = " ↩", "#"+markdownFootnoteRef(x.Index, x.RefIndex)
				if x.RefIndex > 0 {
					s.text += markdownSuperscript(strconv.Itoa(x.RefIndex + 1))
				}
			case *east.TaskCheckBox:
				s.text = "☐ "
				if x.IsChecked {
					s.text = "☑ "
				}
			default:
				out = append(out, inline(ch, s)...)
				continue
			}
			out = append(out, s)
		}
		return out
	}
	// alert and alertAt are those of the alert being walked; strip is the
	// length of its marker, which its first paragraph drops.
	alert, alertAt, strip := alertNone, 0, 0
	slugs := map[string]int{}
	// footnote is whether the blocks being walked are of a footnote.
	footnote := false
	var walk func(ast.Node, int, int, string, bool, float32, float32)
	walk = func(n ast.Node, indent, quote int, marker string, tight bool, offset, markerWidth float32) {
		b := markdownBlock{indent: indent, quote: quote, marker: marker, tight: tight, offset: offset, markerWidth: markerWidth, alert: alert, alertAt: alertAt, footnote: footnote}
		if _, doc := n.(*ast.Document); !doc {
			b.line = lineOf(n)
		}
		switch x := n.(type) {
		case *ast.Heading:
			b.kind, b.level = markdownHeading, x.Level
			b.runs = inline(n, markdownInline{bold: true})
			b.slug = markdownSlug(markdownRunsText(b.runs))
			base := b.slug
			for k := 1; slugs[b.slug] > 0; k++ {
				b.slug = base + "-" + strconv.Itoa(k)
			}
			slugs[b.slug] = 1

		case *ast.Paragraph, *ast.TextBlock:
			b.kind = markdownParagraph
			b.runs = inline(n, markdownInline{})
			if strip > 0 {
				b.runs = markdownTrimRuns(b.runs, strip)
				strip = 0
			}
			if _, task := n.FirstChild().(*east.TaskCheckBox); task && marker != "" {
				b.marker = strings.TrimSpace(b.runs[0].text)
				b.runs = b.runs[1:]
			}

		case *ast.ThematicBreak:
			b.kind = markdownRule
		case *ast.FencedCodeBlock:
			b.kind, b.language = markdownCodeBlock, string(x.Language(source))
			b.codeText = string(x.Lines().Value(source))
			b.runs = code(b.codeText, b.language)
			if strings.EqualFold(b.language, "mermaid") {
				b.diagram = diagram(b.codeText)
			}
		case *ast.CodeBlock:
			b.kind = markdownCodeBlock
			b.codeText = string(x.Lines().Value(source))
			b.runs = code(b.codeText, "")
		case *ast.HTMLBlock:
			raw := string(x.Lines().Value(source))
			if x.HasClosure() {
				raw += string(x.ClosureLine.Value(source))
			}
			b.kind = markdownHTML
			raw = markdownHTMLText(raw)
			if strings.TrimSpace(raw) == "" {
				return
			}
			b.runs = []markdownInline{{text: raw, raw: true}}
		case *ast.Blockquote:
			outer, outerAt := alert, alertAt
			if k, n := markdownAlertOf(x, source); k != alertNone {
				alert, alertAt, strip = k, quote+1, n
				blocks = append(blocks, markdownBlock{kind: markdownAlertTitle, indent: indent, quote: quote + 1, marker: marker, tight: tight,
					offset: offset, markerWidth: markerWidth, alert: k, alertAt: quote + 1, line: b.line})
				if marker != "" {
					offset += markerWidth + 0.5
					markerWidth = 0
				}
				marker = ""
			}
			for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
				walk(ch, indent, quote+1, marker, tight, offset, markerWidth)
				if marker != "" {
					offset += markerWidth + 0.5
					markerWidth = 0
				}
				marker = ""
			}
			alert, alertAt, strip = outer, outerAt, 0
			return
		case *ast.List:
			i := x.Start
			width := float32(1.5)
			if x.IsOrdered() {
				width = max(width, float32(len(strconv.Itoa(x.Start+x.ChildCount()-1))+1)*0.65)
			}
			if marker != "" {
				offset += markerWidth + 0.5
			}
			for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
				m := "•"
				if x.IsOrdered() {
					m = strconv.Itoa(i) + "."
					i++
				}
				walk(ch, indent+1, quote, m, x.IsTight, offset, width)
			}
			return
		case *east.FootnoteList:
			footnote = true
			defer func() { footnote = false }()
			// The footnotes are a numbered list at the end, below a rule,
			// as on GitHub.
			blocks = append(blocks, markdownBlock{kind: markdownRule, indent: indent, quote: quote, line: b.line})
			width := max(1.5, float32(len(strconv.Itoa(x.Count))+1)*0.65)
			for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
				fn := ch.(*east.Footnote)
				anchors = append(anchors, "user-content-fn-"+strconv.Itoa(fn.Index))
				walk(ch, indent+1, quote, strconv.Itoa(fn.Index)+".", true, offset, width)
			}
			return
		case *east.Table:
			first, body := len(blocks), 0
			for row := n.FirstChild(); row != nil; row = row.NextSibling() {
				rb := b
				rb.kind = markdownTable
				rb.aligns = x.Alignments
				rb.line = lineOf(row)
				_, head := row.(*east.TableHeader)
				if head {
					rb.kind = markdownTableHeader
				} else {
					body++
					rb.row = body
				}
				for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
					rb.cells = append(rb.cells, markdownMerge(inline(cell, markdownInline{bold: head})))
				}
				rb.anchors, anchors = anchors, nil
				blocks = append(blocks, rb)
			}
			for i := first; i < len(blocks); i++ {
				blocks[i].table = first
			}
			blocks[len(blocks)-1].last = true
			return
		default:
			for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
				walk(ch, indent, quote, marker, tight, offset, markerWidth)
				if marker != "" {
					offset += markerWidth + 0.5
					markerWidth = 0
				}
				marker = ""
			}
			return
		}
		b.anchors, anchors = anchors, nil
		// Decoded images get their own rows so that paragraphs retain native text wrapping.
		if b.kind == markdownParagraph || b.kind == markdownHeading {
			b.runs = markdownMerge(b.runs)
			if !markdownHasText(b.runs) && !slices.ContainsFunc(b.runs, func(r markdownInline) bool { return r.bitmap != nil }) {
				return
			}
		}
		if (b.kind == markdownParagraph || b.kind == markdownHeading) && slices.ContainsFunc(b.runs, func(r markdownInline) bool { return r.bitmap != nil }) {
			runs := b.runs
			b.runs = nil
			for _, r := range runs {
				if r.bitmap != nil {
					if markdownHasText(b.runs) {
						blocks = append(blocks, b)
						b.runs = nil
						if b.marker != "" {
							b.offset += b.markerWidth + 0.5
						}
						b.markerWidth = 0
						b.marker = ""
					}
					ib := b
					ib.kind = markdownImageBlock
					ib.runs = []markdownInline{r}
					blocks = append(blocks, ib)
					if b.marker != "" {
						b.offset += b.markerWidth + 0.5
					}
					b.markerWidth = 0
					b.marker = ""
				} else {
					b.runs = append(b.runs, r)
				}
			}
			if !markdownHasText(b.runs) {
				return
			}
		}
		blocks = append(blocks, b)
	}
	walk(tree, 0, 0, "", false, 0, 0)
	// A block without a place in the text, as a rule, is where the one
	// before it is, so that the lines only go down.
	line := skip
	for i := range blocks {
		line = max(line, blocks[i].line)
		blocks[i].line = line
	}
	return blocks
}

// markdownSuperscript is the digits of n raised, as ¹².
func markdownSuperscript(n string) string {
	return strings.Map(func(r rune) rune { return []rune("⁰¹²³⁴⁵⁶⁷⁸⁹")[r-'0'] }, n)
}

// markdownFootnoteRef is the anchor of a reference to footnote index,
// the ref-th to it from 0, as GitHub names it.
func markdownFootnoteRef(index, ref int) string {
	a := "user-content-fnref-" + strconv.Itoa(index)
	if ref > 0 {
		a += "-" + strconv.Itoa(ref+1)
	}
	return a
}

// markdownStart is the offset in the source where a node starts, the
// line of a fence for a block of code, or -1 for a node without text.
func markdownStart(n ast.Node) int {
	if f, ok := n.(*ast.FencedCodeBlock); ok && f.Info != nil {
		return f.Info.Segment.Start
	}
	if n.Type() == ast.TypeBlock {
		if l := n.Lines(); l != nil && l.Len() > 0 {
			at := l.At(0).Start
			if _, ok := n.(*ast.FencedCodeBlock); ok {
				// The fence is the line above, as the block has no info.
				at = max(0, at-1)
			}
			return at
		}
	}
	if t, ok := n.(*ast.Text); ok {
		return t.Segment.Start
	}
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if at := markdownStart(ch); at >= 0 {
			return at
		}
	}
	return -1
}

// markdownAlertOf returns the kind of the alert a quote is, by the marker
// alone on its first line, as [!NOTE], and the marker's length in runes.
func markdownAlertOf(q *ast.Blockquote, source []byte) (markdownAlert, int) {
	p, ok := q.FirstChild().(*ast.Paragraph)
	if !ok || p.Lines().Len() == 0 {
		return alertNone, 0
	}
	seg := p.Lines().At(0)
	line := strings.TrimSpace(string(seg.Value(source)))
	if len(line) < 4 || !strings.HasPrefix(line, "[!") || !strings.HasSuffix(line, "]") {
		return alertNone, 0
	}
	return markdownAlerts[strings.ToUpper(line[2:len(line)-1])], utf8.RuneCountInString(line)
}

// markdownTrimRuns drops the first n runes of the text of runs, and the
// break after them.
func markdownTrimRuns(runs []markdownInline, n int) []markdownInline {
	for len(runs) > 0 && n > 0 {
		rs := []rune(runs[0].text)
		if len(rs) > n {
			runs[0].text = string(rs[n:])
			break
		}
		n -= len(rs)
		runs = runs[1:]
	}
	for len(runs) > 0 {
		runs[0].text = strings.TrimLeft(runs[0].text, " \t\n")
		if runs[0].text != "" || runs[0].bitmap != nil {
			break
		}
		runs = runs[1:]
	}
	return runs
}

// markdownSlug is the anchor GitHub gives a heading: its text in lower
// case, without punctuation, its spaces hyphens.
func markdownSlug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

// markdownColumns are the widths of the columns of the table rows are
// of, in DIPs, as wide as the widest text of each, which measure gives,
// within bounds that keep a short column readable and a long one from
// taking all the room, and with the cells' padding.
func markdownColumns(rows []markdownBlock, size float32, measure func(r markdownInline, head bool) float32) []float32 {
	var cols []float32
	for _, row := range rows {
		for i, cell := range row.cells {
			w := float32(0)
			for _, r := range cell {
				if r.bitmap != nil {
					w += min(r.w, size*12)
					continue
				}
				w += measure(r, row.kind == markdownTableHeader)
			}
			if i >= len(cols) {
				cols = append(cols, 0)
			}
			cols[i] = max(cols[i], w)
		}
	}
	for i, w := range cols {
		cols[i] = min(max(w, size*2), size*24) + size*1.5 + 2
	}
	return cols
}

// markdownMerge joins the runs of a style that follow each other, as
// goldmark ends a run at each line and at much punctuation, into fewer
// runs for the preview to lay out.
func markdownMerge(runs []markdownInline) []markdownInline {
	out := runs[:0]
	for _, r := range runs {
		if n := len(out); n > 0 {
			l := &out[n-1]
			if r.bitmap == nil && l.bitmap == nil && r.image == "" && l.image == "" && r.link == l.link && r.kind == l.kind &&
				r.bold == l.bold && r.italic == l.italic && r.strike == l.strike && r.code == l.code && r.raw == l.raw {
				l.text += r.text
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// markdownWords counts the words of the text outside its code, a Chinese
// or Japanese character a word, and the minutes it takes to read them, at
// 230 words a minute, or 500 of those characters.
func markdownWords(blocks []markdownBlock) (words, minutes int) {
	latin, wide := 0, 0
	count := func(runs []markdownInline) {
		for _, r := range runs {
			if r.code || r.raw {
				continue
			}
			in := false
			for _, ch := range r.text {
				switch {
				case unicode.In(ch, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul):
					wide++
					in = false
				case unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '\'' || ch == '’':
					if !in {
						latin++
					}
					in = true
				default:
					in = false
				}
			}
		}
	}
	for _, b := range blocks {
		if b.kind == markdownCodeBlock || b.kind == markdownHTML {
			continue
		}
		count(b.runs)
		for _, cell := range b.cells {
			count(cell)
		}
	}
	words = latin + wide
	if words > 0 {
		minutes = max(1, int(math.Round(float64(latin)/230+float64(wide)/500)))
	}
	return words, minutes
}

// markdownRunsText is the text of runs.
func markdownRunsText(runs []markdownInline) string {
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(r.text)
	}
	return b.String()
}

// HTML comments may span lines; other HTML remains literal.
func markdownHTMLText(s string) string {
	for {
		start := strings.Index(s, "<!--")
		if start < 0 {
			return s
		}
		end := strings.Index(s[start+4:], "-->")
		if end < 0 {
			return s[:start]
		}
		s = s[:start] + s[start+4+end+3:]
	}
}

// Soft breaks between standalone images must not become empty paragraph rows.
func markdownHasText(runs []markdownInline) bool {
	for _, r := range runs {
		if strings.TrimSpace(r.text) != "" {
			return true
		}
	}
	return false
}

func markdownCode(code, language string) []markdownInline {
	var out []markdownInline
	var lines []highlight.Line
	if language != "" {
		lexer := lexers.Get(language)
		if lexer != nil {
			lexer = chroma.Coalesce(lexer)
		}
		lines = highlight.Lex(lexer, strings.TrimSuffix(code, "\n"))
	} else {
		return []markdownInline{{text: strings.TrimSuffix(code, "\n"), code: true}}
	}
	for i, line := range lines {
		rs := []rune(line.Text)
		at := 0
		for _, tok := range line.Tokens {
			if tok.Start > at {
				out = append(out, markdownInline{text: string(rs[at:tok.Start]), code: true})
			}
			out = append(out, markdownInline{text: string(rs[tok.Start:tok.End]), code: true, kind: tok.Kind})
			at = tok.End
		}
		tail := string(rs[at:])
		if i < len(lines)-1 {
			tail += "\n"
		}
		out = append(out, markdownInline{text: tail, code: true})
	}
	return out
}
