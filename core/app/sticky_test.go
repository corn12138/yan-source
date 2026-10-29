package app

import (
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/view"
)

func TestStickyCrumbsAndSymbols(t *testing.T) {
	var b strings.Builder
	b.WriteString("package main\n\nfunc main() {\n\tfor i := 0; i < 3; i++ {\n")
	for i := 0; i < 30; i++ {
		b.WriteString("\t\tprintln(i)\n")
	}
	b.WriteString("\t}\n}\n\nfunc other() {}\n")
	e := newGoEditor(t, "|"+b.String())
	e.GotoLine(20)
	e.cur().scrollY = 15
	var f view.Frame
	e.Frame(&f)
	pane := f.Columns[0].Pane
	if len(pane.Sticky) != 2 || pane.Sticky[0].Line != 2 || pane.Sticky[1].Line != 3 {
		t.Fatalf("sticky = %+v", pane.Sticky)
	}
	if cr := f.Columns[0].Crumbs; len(cr) != 2 || cr[0].Label != "func main()" || cr[1].Label != "for i := 0; i < 3; i++" {
		t.Fatalf("crumbs = %+v", cr)
	}
	// 点钉住的第一行：跳到 func main
	e.Handle(MouseInput{Column: 0, X: 10, Y: 0, Kind: MousePress})
	if s := e.cur().doc.buf.Snapshot(); s.LineAt(e.cur().doc.buf.Selections().Main().Head) != 2 {
		t.Fatal("click on a sticky line should jump to it")
	}
	// ⇧⌘O 列出符号，打字过滤，回车跳过去并选中名字
	e.Run("workbench.action.gotoSymbol")
	if len(e.ov.items) != 2 {
		t.Fatalf("symbols = %+v", e.ov.items)
	}
	typeText(e, "oth")
	e.ChooseOverlay(0)
	if got := shown(e); !strings.Contains(got, "func [other]()") {
		t.Fatalf("got %q", got[len(got)-40:])
	}
}
