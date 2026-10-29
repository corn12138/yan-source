package gui

// Adapted from Mayoi 3d03b96 cmd/mayoi/markdown.go (MIT, Copyright (c) 2026 chencheng).

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// linkURL is goldmark's linkify pattern of a URL with its protocol.
var linkURL = regexp.MustCompile(`^(?:http|https|ftp)://[-a-zA-Z0-9@:%._\+~#=]{1,256}\.[a-z]+(?::\d+)?(?:[/#?][-a-zA-Z0-9@:%_+.~#$!?&/=\(\);,'">\^{}\[\]` + "`" + `]*)?`)

// urlLinkifier links the URLs goldmark's linkify leaves as text because
// they follow neither a space nor a delimiter, as after the ： of
// "链接：https://…", which GitHub links after anything but a letter.
type urlLinkifier struct{}

func (urlLinkifier) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	source := reader.Source()
	var texts []*ast.Text
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch n := n.(type) {
		case *ast.Link, *ast.AutoLink, *ast.Image, *ast.CodeSpan, *ast.RawHTML:
			return ast.WalkSkipChildren, nil
		case *ast.Text:
			if entering && !n.IsRaw() {
				texts = append(texts, n)
			}
		}
		return ast.WalkContinue, nil
	})
	for _, t := range texts {
		linkifyText(t, source)
	}
}

// linkifyText splits t into its text and the links of its URLs, first
// joining the text after it that a delimiter such as the _ of a URL split
// from it.
func linkifyText(t *ast.Text, source []byte) {
	parent := t.Parent()
	if parent == nil {
		return
	}
	for !t.SoftLineBreak() && !t.HardLineBreak() {
		next, ok := t.NextSibling().(*ast.Text)
		if !ok || next.IsRaw() || next.Segment.Start != t.Segment.Stop {
			break
		}
		t.Segment = t.Segment.WithStop(next.Segment.Stop)
		t.SetSoftLineBreak(next.SoftLineBreak())
		t.SetHardLineBreak(next.HardLineBreak())
		parent.RemoveChild(parent, next)
	}
	for {
		seg := t.Segment
		b := seg.Value(source)
		at, end := -1, 0
		for i := range b {
			if b[i] != 'h' && b[i] != 'f' {
				continue
			}
			if p := seg.Start + i; p > 0 && (source[p-1] >= 'a' && source[p-1] <= 'z' || source[p-1] >= 'A' && source[p-1] <= 'Z') {
				continue
			}
			if m := linkURL.Find(b[i:]); m != nil {
				at, end = i, i+linkEnd(m)
				break
			}
		}
		if at < 0 {
			if seg.Len() == 0 && !t.SoftLineBreak() && !t.HardLineBreak() {
				parent.RemoveChild(parent, t)
			}
			return
		}
		if at > 0 {
			parent.InsertBefore(parent, t, ast.NewTextSegment(seg.WithStop(seg.Start+at)))
		}
		url := ast.NewTextSegment(text.NewSegment(seg.Start+at, seg.Start+end))
		parent.InsertBefore(parent, t, ast.NewAutoLink(ast.AutoLinkURL, url))
		t.Segment = seg.WithStart(seg.Start + end)
	}
}

// linkEnd is where the URL of m ends without the punctuation that ends
// its sentence, a ) it did not open, or an entity such as &amp;, as
// goldmark's linkify ends it.
func linkEnd(m []byte) int {
	end := len(m)
	for end > 0 {
		switch c := m[end-1]; {
		case strings.IndexByte("?!.,:*_~", c) >= 0:
			end--
			continue
		case c == ')' && bytes.Count(m[:end], []byte(")")) > bytes.Count(m[:end], []byte("(")):
			end--
			continue
		case c == ';':
			if i := bytes.LastIndexByte(m[:end-1], '&'); i >= 0 && isAlnum(m[i+1:end-1]) {
				end = i
				continue
			}
		}
		return end
	}
	return end
}

func isAlnum(b []byte) bool {
	for _, c := range b {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return len(b) > 0
}
