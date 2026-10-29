package symbols

import (
	"fmt"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/text"
)

func defs(src string) []string {
	var out []string
	for _, s := range Definitions(text.FromString(src, text.Meta{})) {
		out = append(out, fmt.Sprintf("%d:%d %s %s", s.Line, s.Col, s.Name, s.Kind))
	}
	return out
}

func TestDefinitionsGo(t *testing.T) {
	src := strings.Join([]string{
		"package app",                   // 0
		"",                              // 1
		"const (",                       // 2
		"\tmaxItems = 10 // comment",    // 3
		"\tminA, minB = 1, 2",           // 4
		")",                             // 5
		"type editor struct {",          // 6
		"\troot string",                 // 7
		"\tcols, rows []int",            // 8
		"}",                             // 9
		"var Global = 1",                // 10
		"func (e *editor) run(x int) {", // 11
		"\tn, err := e.do(x)",           // 12
		"\tfor i, v := range e.cols {",  // 13
		"\t\tfunc() {",                  // 14
		"\t}",                           // 15
		"}",                             // 16
	}, "\n")
	got := strings.Join(defs(src), "\n")
	for _, want := range []string{
		"3:1 maxItems variable", "4:1 minA variable", "4:7 minB variable",
		"6:5 editor class", "7:1 root field", "8:1 cols field", "8:7 rows field",
		"10:4 Global variable", "11:17 run method",
		"12:1 n variable", "12:4 err variable", "13:5 i variable", "13:8 v variable",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, " string ") || strings.Contains(got, "14:") {
		t.Errorf("a type or a func literal is not a definition:\n%s", got)
	}
}

func TestDefinitionsOtherLanguages(t *testing.T) {
	ts := strings.Join([]string{
		"export const { a, b: renamed, ...rest } = obj",
		"let [first, second = 2] = list",
		"export interface Shape {",
		"  readonly area?: number;",
		"  draw(ctx: Ctx): void",
		"}",
		"class Box {",
		"  private size = 1",
		"  async open() {",
		"    return this.size",
		"  }",
		"}",
		"enum Color {",
		"  Red,",
		"  Green = 2,",
		"}",
	}, "\n")
	got := strings.Join(defs(ts), "\n")
	for _, want := range []string{"a variable", "renamed variable", "rest variable", "first variable", "second variable",
		"Shape interface", "area field", "draw field", "Box class", "size field", "open field", "Red field", "Green field"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, " b ") || strings.Contains(got, "return") {
		t.Errorf("unexpected:\n%s", got)
	}
	py := "MAX = 3\n# NOT = 1\nclass A:\n    def f(self, depth):\n        y = 1\n"
	got = strings.Join(defs(py), "\n")
	for _, want := range []string{"0:0 MAX variable", "2:6 A class", "3:8 f method"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if strings.Contains(got, "NOT") {
		t.Errorf("a comment defines nothing:\n%s", got)
	}
	c := "#include <x.h>\n#define LIMIT 4\n"
	if got := defs(c); len(got) != 1 || got[0] != "1:8 LIMIT macro" {
		t.Errorf("c = %q", got)
	}
}

func TestParam(t *testing.T) {
	line := "func (e *editor) run(x int, depth int) (n int, err error) {"
	sym, _ := declare(line)
	sym.Col = strings.Index(line, "run")
	for name, want := range map[string]int{"e": 6, "x": 21, "depth": 28, "err": 47, "run": -1, "editor": 9} {
		if got := Param(line, sym, name); got != want {
			t.Errorf("%s: %d, want %d", name, got, want)
		}
	}
}

// A func literal alone on its line is not a method with an empty receiver (Mayoi PR #83).
func TestFuncLiteralLine(t *testing.T) {
	if sym, ok := declare("\tfunc() {"); ok {
		t.Fatalf("declares %+v", sym)
	}
}
