package gui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	east "github.com/yuin/goldmark/extension/ast"
)

// Baseline parser fixtures originate from Mayoi v0.7.0 (MIT); newer cases name their source below.

const markdownFixture = `---
title: hidden front matter
---
# Markdown preview

[![CI](https://example.com/ci.svg)](https://example.com) [![npm](https://example.com/npm.svg)](https://example.org)
<!-- hidden block
comment -->
Inline <!-- hidden inline -->comment<br>hard break.<br/>Again<br />Done.
![SVG logo](logo.svg)

A paragraph with **bold**, *italic*, ***both***, ~~deleted~~, ` + "`inline code`" + ` and [a link](https://example.com). Visit https://example.com.
Soft
break and a hard  
break. <em>HTML stays text</em>

中文段落含 **中文粗体**、*中文斜体*、~~中文删除线~~、` + "`行内代码`" + `，以及 ~~[删除链接](https://example.com)~~。

## Lists and quotes

- Compact bullet
  - Nested compact bullet
    1. Nested compact number
- Next compact item

  Continuation paragraph.

  ~~~go
  println("continuation code")
  ~~~

1. One
2. Two
3. Three
4. Four
5. Five
6. Six
7. Seven
8. Eight
9. Nine
10. Ten
11. Eleven

1. Loose first item

2. Loose second item

- [x] Finished
- [ ] Pending
  - Nested bullet
    1. Nested number

> A quoted paragraph.
> > A nested quote.

` + "```go\npackage main\n\nfunc main() { println(\"Hello\") }\n```" + `

` + "```\nplain code\n```" + `

| Left | Center | Right |
| :--- | :---: | ---: |
| one | **two** | three |

---

![Local art](art.png)
![Remote alt](https://example.com/image.png)
![Missing alt](missing.png)

### Heading three
#### Heading four
##### Heading five
###### Heading six

<div>HTML block is displayed literally.</div>
`

// Bare URLs are links after punctuation too, as on GitHub: after a full-width colon or
// parenthesis, a digit or a CJK character, but not inside a word, code or another link.
func TestMarkdownLinkifyAfterPunctuation(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"- 反思：https://x.com/a/status/1", "反思：[https://x.com/a/status/1]"},
		{"a:https://x.com/b", "a:[https://x.com/b]"},
		{"见https://x.com/c 。", "见[https://x.com/c] 。"},
		{"1https://x.com/d", "1[https://x.com/d]"},
		{"a：https://x.com/a_b_c，后", "a：[https://x.com/a_b_c]，后"},
		{"a（https://x.com/e）and (https://x.com/(f))", "a（[https://x.com/e]）and ([https://x.com/(f)])"},
		{"a：https://x.com/g?x=1&amp; done.", "a：[https://x.com/g?x=1]& done."},
		{"a：https://x.com/h. b：https://x.com/i", "a：[https://x.com/h]. b：[https://x.com/i]"},
		{"foohttps://x.com/j", "foohttps://x.com/j"},
		{"a：www.example.com", "a：www.example.com"},
		{"`a：https://x.com/k`", "a：https://x.com/k"},
		{"[a：https://x.com/l](https://y.com)", "[a：https://x.com/l](https://y.com)"},
	} {
		var got strings.Builder
		for _, r := range parseMarkdown([]byte(c.source), "", "", nil)[0].runs {
			switch {
			case r.link != "" && r.link == r.text:
				got.WriteString("[" + r.text + "]")
			case r.link != "":
				got.WriteString("[" + r.text + "](" + r.link + ")")
			default:
				got.WriteString(r.text)
			}
		}
		if got.String() != c.want {
			t.Errorf("%q: got %q, want %q", c.source, got.String(), c.want)
		}
	}
}

// Footnotes, as on GitHub: references are raised numbers in order of first use, linking to
// their notes, which are listed after a rule, smaller and muted, each linking back to every
// reference; a note nothing refers to is left out.
func TestMarkdownFootnotes(t *testing.T) {
	source := "## 数据[^11]\n\nA[^1] b[^1] c[^note].\n\n## 参考链接\n\n" +
		"[^1]: 第 4 次尝试 https://x.com/a/status/1\n[^note]: 锁定：https://x.com/a/status/2\n[^11]: Eleven.\n[^unused]: Never referenced.\n\n" +
		"After.\n"
	blocks := parseMarkdown([]byte(source), "", "", nil)
	text := func(b markdownBlock) string {
		var s strings.Builder
		for _, r := range b.runs {
			if r.link != "" {
				s.WriteString("[" + r.text + "](" + r.link + ")")
			} else {
				s.WriteString(r.text)
			}
		}
		return s.String()
	}
	var got []string
	for _, b := range blocks {
		got = append(got, fmt.Sprintf("%d %s %v %v %s", b.kind, b.marker, b.footnote, b.anchors, text(b)))
	}
	want := []string{
		fmt.Sprintf("%d  false [user-content-fnref-1] 数据[¹](#user-content-fn-1)", markdownHeading),
		fmt.Sprintf("%d  false [user-content-fnref-2 user-content-fnref-2-2 user-content-fnref-3] A[²](#user-content-fn-2) b[²](#user-content-fn-2) c[³](#user-content-fn-3).", markdownParagraph),
		fmt.Sprintf("%d  false [] 参考链接", markdownHeading),
		fmt.Sprintf("%d  false [] After.", markdownParagraph),
		fmt.Sprintf("%d  false [] ", markdownRule),
		fmt.Sprintf("%d 1. true [user-content-fn-1] Eleven.[ ↩](#user-content-fnref-1)", markdownParagraph),
		fmt.Sprintf("%d 2. true [user-content-fn-2] 第 4 次尝试 [https://x.com/a/status/1](https://x.com/a/status/1)[ ↩](#user-content-fnref-2)[ ↩²](#user-content-fnref-2-2)", markdownParagraph),
		fmt.Sprintf("%d 3. true [user-content-fn-3] 锁定：[https://x.com/a/status/2](https://x.com/a/status/2)[ ↩](#user-content-fnref-3)", markdownParagraph),
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("blocks\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestMarkdownBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "docs", "readme.md")
	blocks := parseMarkdown([]byte(markdownFixture), path, "", nil)
	kinds := map[markdownKind]int{}
	levels := map[int]bool{}
	var bold, italic, both, strike, code, link, nested, task, quote, language, table, image, raw bool
	for _, b := range blocks {
		kinds[b.kind]++
		if b.kind == markdownHeading {
			levels[b.level] = true
		}
		nested = nested || b.indent >= 3
		quote = quote || b.quote == 2
		language = language || b.language == "go"
		if b.kind == markdownTableHeader {
			table = len(b.aligns) == 3 && b.aligns[0] == east.AlignLeft && b.aligns[1] == east.AlignCenter && b.aligns[2] == east.AlignRight && b.cells[0][0].bold
		}
		for _, r := range b.runs {
			bold = bold || r.bold
			italic = italic || r.italic
			both = both || (r.bold && r.italic)
			strike = strike || r.strike
			code = code || r.code
			link = link || r.link == "https://example.com"
			task = task || b.marker == "☑" || strings.Contains(r.text, "☑")
			raw = raw || r.raw
			image = image || (r.image == "art.png" && markdownPath("", path, r.image) == filepath.Join(filepath.Dir(path), "art.png"))
		}
	}
	if len(levels) != 6 || !bold || !italic || !both || !strike || !code || !link || !nested || !task || !quote || !language || !table || !image || !raw || kinds[markdownRule] != 1 {
		t.Fatalf("conversion: levels %v, styles %v %v %v %v %v, link %v nested %v task %v quote %v language %v table %v image %v raw %v kinds %v", levels, bold, italic, both, strike, code, link, nested, task, quote, language, table, image, raw, kinds)
	}
	for _, ending := range []string{"---", "..."} {
		bs := parseMarkdown([]byte("---\ntitle: hidden\n"+ending+"\nVisible <!-- hidden\ncomment -->text<br>next<br/>last<br />end"), path, "", nil)
		if len(bs) != 1 || markdownRunsText(bs[0].runs) != "Visible text\nnext\nlast\nend" {
			t.Fatalf("front matter/comments/br: %+v", bs)
		}
	}
	if bs := parseMarkdown([]byte("---\nIntro text.\n\n---\nAfter"), path, "", nil); len(bs) != 4 {
		t.Fatalf("rules taken for front matter: %+v", bs)
	}
	badges := parseMarkdown([]byte("[![CI](https://example.com/ci)](https://example.com) [![npm](missing)](https://example.org)"), path, "", nil)
	if len(badges) != 1 || badges[0].kind != markdownParagraph || badges[0].runs[0].link == "" || !badges[0].runs[0].raw {
		t.Fatal("badges not inline muted links")
	}
	bs := parseMarkdown([]byte("99. first\n\n    continuation\n\n    ~~~\n    code\n    ~~~\n\n    > quote\n\n    - nested\n\n100. second\n\n- literal ☑ text\n- [x] task"), path, "", nil)
	if bs[0].markerWidth < 2.5 || bs[0].marker != "99." {
		t.Fatalf("ordered width: %+v", bs)
	}
	for _, b := range bs[1:5] {
		if b.offset != bs[0].markerWidth+0.5 {
			t.Fatalf("continuation indent: %+v", bs)
		}
	}
	if bs[len(bs)-2].marker != "•" || bs[len(bs)-1].marker != "☑" {
		t.Fatal("task checkbox detection")
	}
	if markdownPath("", path, "https://example.com/a.png") != "" || markdownPath("", path, "//example.com/a.png") != "" || markdownPath("", path, "/tmp/a%20b.png") != "/tmp/a b.png" {
		t.Fatal("destination resolution")
	}
}

func TestMarkdownImageSpacing(t *testing.T) {
	blocks := parseMarkdown([]byte("![remote](https://example.com/a.png)\n![missing](missing.png)\n![missing too](missing2.png)\n"), "test.md", "", nil)
	if len(blocks) != 1 {
		t.Fatalf("images gained empty paragraphs: %+v", blocks)
	}
	for _, b := range blocks {
		if b.kind != markdownParagraph {
			t.Fatalf("unexpected image separator: %+v", b)
		}
	}
	for _, source := range []string{"- ![one](a.png) ![two](b.png) text", "1. text ![one](a.png) ![two](b.png) tail"} {
		blocks = parseMarkdown([]byte(source), "test.md", "", nil)
		for i, b := range blocks {
			if (b.marker != "") != (i == 0) {
				t.Fatalf("repeated or missing marker: %+v", blocks)
			}
		}
	}
}

// Parser regressions adapted from Mayoi 46990a1 (MIT).
func TestMarkdownAlerts(t *testing.T) {
	src := "> [!NOTE]\n> Read this.\n\n> [!warning]\n> Careful **now**.\n> > Nested quote.\n\n> [!FOO]\n> Not an alert.\n\n> [!TIP]\n\n> Plain.\n"
	blocks := parseMarkdown([]byte(src), "test.md", "", nil)
	var got []string
	for _, b := range blocks {
		switch b.kind {
		case markdownAlertTitle:
			got = append(got, "title:"+strconv.Itoa(int(b.alert)))
		default:
			got = append(got, strconv.Itoa(int(b.alert))+"@"+strconv.Itoa(b.alertAt)+"/"+strconv.Itoa(b.quote)+":"+markdownRunsText(b.runs))
		}
	}
	want := []string{
		"title:1", "1@1/1:Read this.",
		"title:4", "4@1/1:Careful now.", "4@1/2:Nested quote.",
		"0@0/1:[!FOO] Not an alert.",
		"title:2",
		"0@0/1:Plain.",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("alerts:\n got %q\nwant %q", got, want)
	}
}

func TestMarkdownSlugsAndWidths(t *testing.T) {
	blocks := parseMarkdown([]byte("# Hello, World!\n## Hello, World!\n### `Code` and 中文 _x_\n\n| a | a much longer column |\n|---|---|\n| 1 | 2 |\n"), "test.md", "", nil)
	var slugs []string
	var rows []markdownBlock
	for i, b := range blocks {
		if b.slug != "" {
			slugs = append(slugs, b.slug)
		}
		if b.kind == markdownTable || b.kind == markdownTableHeader {
			rows = append(rows, b)
			if b.table != 3 || b.kind == markdownTable && (b.row != 1 || !b.last) {
				t.Fatalf("row %d: table %d, row %d, last %v", i, b.table, b.row, b.last)
			}
		}
	}
	// A character 10 wide, a cell's padding 1.5 times the size and 2.
	widths := markdownColumns(rows, 10, func(r markdownInline, head bool) float32 { return float32(len(r.text)) * 10 })
	if strings.Join(slugs, " ") != "hello-world hello-world-1 code-and-中文-x" {
		t.Fatalf("slugs %q", slugs)
	}
	if len(widths) != 2 || widths[0] != 20+17 || widths[1] != 200+17 {
		t.Fatalf("widths %v", widths)
	}
}

func TestMarkdownBlockLines(t *testing.T) {
	src := "---\ntitle: x\n---\n# Title\n\npara one\nstill one\n\n- a\n- b\n\n> [!NOTE]\n> noted\n\n```go\nx := 1\n```\n\n| h | i |\n|---|---|\n| 1 | 2 |\n\n---\n\ntail\n"
	blocks := parseMarkdown([]byte(src), "/x.md", "", nil)
	var got []string
	for _, b := range blocks {
		got = append(got, strconv.Itoa(b.line))
	}
	// Title, para, a, b, alert title, noted, code, header, row, rule (where
	// the row is, as goldmark keeps no place for it), tail.
	if want := "3 5 8 9 11 11 14 18 20 20 24"; strings.Join(got, " ") != want {
		t.Fatalf("lines %s, want %s", strings.Join(got, " "), want)
	}
}
