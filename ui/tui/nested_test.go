package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/view"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

func TestNestedRepositoryViewport(t *testing.T) {
	term := vt.NewMockTerm(vt.MockOptSize{X: 40, Y: 8})
	screen, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		t.Fatal(err)
	}
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	f := New(4)
	f.screen, f.sideLines = screen, make([]sideLine, 8)
	scm := view.SCM{NoRepo: true, Selected: 11}
	for i := range 12 {
		scm.Repositories = append(scm.Repositories, view.SCMRepository{Name: fmt.Sprintf("repo-%02d", i)})
	}
	f.drawSCM(&scm, 0, view.Rect{W: 40, H: 6}, true)
	screen.Show()
	if f.sideLines[5].index != 11 || f.sideLines[5].kind != sideSCMRow {
		t.Fatal("selected repository is not visible/clickable")
	}
	var row strings.Builder
	for x := range 40 {
		row.WriteString(term.GetCell(vt.Coord{X: vt.Col(x), Y: 5}).C)
	}
	if !strings.Contains(row.String(), "repo-11") {
		t.Fatalf("selected row: %q", row.String())
	}
}

func TestMultiRepositoryViewportTargets(t *testing.T) {
	term := vt.NewMockTerm(vt.MockOptSize{X: 50, Y: 12})
	screen, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		t.Fatal(err)
	}
	if err = screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	f := New(4)
	f.screen, f.sideLines = screen, make([]sideLine, 12)
	s := view.SCM{RepositoryID: 20, Selected: 0}
	for _, id := range []uint64{10, 20} {
		p := view.SCM{RepositoryID: id, Branch: "main", Groups: []view.SCMGroup{{Title: "更改", Rows: []view.SCMRow{{Name: fmt.Sprint(id, ".txt"), Letter: 'M'}}}}}
		s.Panels = append(s.Panels, view.SCMPanel{Name: fmt.Sprint("repo-", id), State: p})
	}
	f.drawSCM(&s, 2, view.Rect{W: 50, H: 12}, true)
	found := false
	for _, hit := range f.sideLines {
		if hit.kind == sideSCMRow && hit.repositoryID == 20 && hit.index == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("selected repository file not visible/clickable")
	}
	s.Panels[1].State.MessageFocused = true
	s.Panels[1].State.Message = "B draft"
	_, ok := f.drawSCM(&s, 2, view.Rect{W: 50, H: 12}, true)
	if !ok {
		t.Fatal("selected repository message lost keyboard caret")
	}
}
