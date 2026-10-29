package symbols

import (
	"testing"

	"github.com/corn12138/yan-source/core/text"
)

func TestFind(t *testing.T) {
	src := "package main\n\ntype app struct {\n}\n\nfunc (a *app) run(x int) error {\n}\n\nfunc main() {}\n"
	syms := Find(text.FromString(src, text.Meta{}), false)
	if len(syms) != 3 {
		t.Fatalf("syms = %+v", syms)
	}
	if s := syms[1]; s.Name != "run" || s.Kind != "method" || s.Line != 5 || s.Detail != "(x int) error · app" || s.Col != len("func (a *app) ") {
		t.Fatalf("method = %+v", s)
	}
	if syms[0].Kind != "class" || syms[2].Name != "main" {
		t.Fatalf("syms = %+v", syms)
	}
	md := Find(text.FromString("# 标题\n\n```\n# 不是标题\n```\n## 小节\n", text.Meta{}), true)
	if len(md) != 2 || md[0].Name != "标题" || md[1].Depth != 2 {
		t.Fatalf("md = %+v", md)
	}
}

// A line opening a function literal has empty parentheses where a method's
// receiver would be; it declares nothing and must not crash Go to Symbol.
func TestFindAnonymousFunc(t *testing.T) {
	src := "package main\n\nfunc main() {\n\trun(\n\t\tfunc() {\n\t\t},\n\t)\n}\nfunc() {\n"
	syms := Find(text.FromString(src, text.Meta{}), false)
	if len(syms) != 1 || syms[0].Name != "main" {
		t.Fatalf("syms = %+v", syms)
	}
}
