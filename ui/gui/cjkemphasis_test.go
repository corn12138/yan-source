package gui

import (
	"strings"
	"testing"
)

// Parser fixtures adapted from Mayoi ff4eb6a6c433 (MIT).
func TestMarkdownCJKEmphasis(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{`**京东/淘宝上的港版"准新机"**标着"仅海外 eSIM"`, `[b:京东/淘宝上的港版"准新机"]标着"仅海外 eSIM"`},
		{"**「准新机」**标着", "[b:「准新机」]标着"},
		{`中文**"引号"**中文`, `中文[b:"引号"]中文`},
		{"*（斜体）*后", "[i:（斜体）]后"},
		{"价格**很高**，除非", "价格[b:很高]，除非"},
		{"**同步调用。**tool", "[b:同步调用。]tool"},
		{"**一句话：没做 Tasks。**editor", "[b:一句话：没做 Tasks。]editor"},
		{"abc**「准新机」**", "abc[b:「准新机」]"},
		{`~~"旧价"~~现价`, `[s:"旧价"]现价`},
		{`a**"b"**c`, `a**"b"**c`},
		{"snake_case_word", "snake_case_word"},
		{"これは**「強調」**です", "これは[b:「強調」]です"},
		{"한국어**「강조」**입니다", "한국어[b:「강조」]입니다"},
		{"`**中文。**tool`", "**中文。**tool"},
		{`\*\*中文。\*\*tool`, "**中文。**tool"},
		{"**bold** and ~~gone~~", "[b:bold] and [s:gone]"},
		{"a ~~~b~~~ c", "a ~~~b~~~ c"},
	} {
		var got strings.Builder
		for _, r := range parseMarkdown([]byte(c.source), "", "", nil)[0].runs {
			switch {
			case r.bold:
				got.WriteString("[b:" + r.text + "]")
			case r.italic:
				got.WriteString("[i:" + r.text + "]")
			case r.strike:
				got.WriteString("[s:" + r.text + "]")
			default:
				got.WriteString(r.text)
			}
		}
		if got.String() != c.want {
			t.Errorf("%q: got %q, want %q", c.source, got.String(), c.want)
		}
	}
}
