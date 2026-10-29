package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/git/linediff"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// 窗格右边那一条：改了的行、查找的匹配、光标所在的行、眼前这一屏；点它跳到文件的那个地方。
func TestOverview(t *testing.T) {
	var b strings.Builder
	for i := range 200 {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	root := gitRepo(t, map[string]string{"a.txt": b.String()})
	path := filepath.Join(root, "a.txt")
	changed := strings.Replace(b.String(), "line 100\n", "LINE 100\nadded\n", 1)
	os.WriteFile(path, []byte(changed), 0o644)
	e := newEditorIn(t, root)
	if err := e.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	tb := e.cur()
	var f view.Frame
	pump(t, e, func() bool { e.Frame(&f); return len(tb.doc.marks.hunks) == 1 })
	e.Frame(&f)
	ov := f.Columns[0].Pane.Overview
	if ov == nil || ov.Lines != 202 || ov.Top != 0 || ov.Visible != 28 || ov.Caret != 0 {
		t.Fatalf("overview %+v", ov)
	}
	if fmt.Sprint(ov.Changes) != fmt.Sprint([]view.OverviewRun{{From: 100, To: 101, Mark: view.MarkModified}, {From: 101, To: 102, Mark: view.MarkAdded}}) {
		t.Fatalf("changes %+v", ov.Changes)
	}

	e.Handle(cmdKey('f', keymap.ModCtrl))
	typeText(e, "line 19")
	e.Frame(&f)
	if ms := f.Columns[0].Pane.Overview.Matches; fmt.Sprint(ms) != "[19 191 192 193 194 195 196 197 198 199 200]" {
		t.Fatalf("matches %v", ms) // line 19，line 190…199（第 100 行后多了一行）
	}

	e.OverviewJump(0, 0.5) // 文件的中间在这一屏的中间
	if e.Frame(&f); f.Columns[0].Pane.Overview.Top != 101-14 {
		t.Fatalf("after jump: top %d", f.Columns[0].Pane.Overview.Top)
	}
}

func TestOverviewRefreshesWhenHEADChanges(t *testing.T) {
	e, _, _ := newTestEditor(t, "a\nb\nc\n")
	defer e.Close()
	tb := e.cur()
	tb.doc.marks = gitMarks{version: 1, generation: 1, lines: []linediff.Kind{linediff.Modified, linediff.Same, linediff.Same}}
	if len(e.overviewRuns(tb)) != 1 {
		t.Fatal("missing initial mark")
	}
	// Git can change the base while the working text version and line count stay the same.
	tb.doc.marks.lines = []linediff.Kind{linediff.Same, linediff.Same, linediff.Modified}
	tb.doc.marks.generation++
	if got := e.overviewRuns(tb); len(got) != 1 || got[0].From != 2 {
		t.Fatalf("stale overview: %+v", got)
	}
}
