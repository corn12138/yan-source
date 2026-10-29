package app

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func guideEditor(t *testing.T, p keymap.Profile) *Editor {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	e, _ := newFolderEditor(t, map[string]string{"main.go": "package main\n"})
	e.profile = p
	e.compileKeys(e.cfg)
	e.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	return e
}

func lessonIndex(id string) int {
	return slices.IndexFunc(guideLessons, func(l guideLesson) bool { return l.id == id })
}

// Every step listens for a command that exists: a renamed command must not leave a lesson stuck.
func TestGuideCommandsExist(t *testing.T) {
	e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, macGUI)
	for _, l := range guideLessons {
		for _, s := range l.steps {
			if len(s.cmds) == 0 && s.check == nil {
				t.Errorf("%s: step %q has no way to complete", l.id, s.text)
			}
			for _, id := range append(slices.Clone(s.cmds), s.show) {
				if id == "" {
					continue
				}
				if _, ok := e.commands[id]; !ok {
					t.Errorf("%s: step %q waits for unknown command %s", l.id, s.text, id)
				}
			}
		}
	}
	for _, tip := range guideTips {
		if got := e.expandTip(tip); strings.ContainsAny(got, "{}") {
			t.Errorf("tip not expanded: %q", got)
		}
	}
}

// A lesson opens its practice file outside the project, advances as the user really does each
// step — by key or by palette — and is remembered when done.
func TestGuideLessonRunsThroughRealCommands(t *testing.T) {
	e := guideEditor(t, macGUI)
	e.Run("yan.guide.open")
	var f view.Frame
	e.Frame(&f)
	if f.Sidebar.View != view.SideLearn || len(f.Sidebar.Learn.Lessons) != len(guideLessons) || f.Sidebar.Learn.Active != -1 {
		t.Fatalf("guide %+v", f.Sidebar.Learn)
	}
	e.Handle(key(keymap.KeyEnter, 0)) // the first lesson, chosen in the list
	if e.guide.active != 0 || !strings.Contains(e.cur().doc.buf.Path(), filepath.Join("learn", "编辑基础.go")) {
		t.Fatalf("lesson not started: %+v, file %s", e.guide, e.cur().doc.buf.Path())
	}
	e.GotoLine(5)
	e.Frame(&f)
	if s := f.Sidebar.Learn.Steps[0]; !s.Current || s.Keys == "" {
		t.Fatalf("first step %+v", s)
	}
	e.Run("editor.action.commentLine") // not this step's command: nothing moves
	if e.guide.step != 0 {
		t.Fatal("a different command completed the step")
	}
	e.Handle(cmdKey('l', keymap.ModSuper)) // the step's own key
	for _, id := range []string{"editor.action.copyLinesDownAction", "editor.action.moveLinesUpAction", "editor.action.commentLine", "editor.action.deleteLines", "undo"} {
		e.Run(id)
	}
	if !e.lessonDone("editing") || e.guide.sel != lessonIndex("multicursor") || !strings.Contains(e.status, "多光标") {
		t.Fatalf("after the lesson: done %v, sel %d, status %q", e.lessonDone("editing"), e.guide.sel, e.status)
	}
	// A fresh start of the lesson restores the practice text.
	e.StartLesson(0)
	if !strings.Contains(content(e), "func greet()") || e.cur().doc.buf.Dirty() {
		t.Fatalf("practice text %q", content(e))
	}
}

// A step can complete by what the editor shows, not only by a command.
func TestGuideStepByCondition(t *testing.T) {
	e := guideEditor(t, macGUI)
	e.StartLesson(lessonIndex("multicursor"))
	e.GotoLine(5)
	e.Run("editor.action.addSelectionToNextFindMatch")
	e.Run("editor.action.addSelectionToNextFindMatch")
	if e.guide.step != 0 || e.guide.count != 2 {
		t.Fatalf("after two of three: %+v", e.guide)
	}
	e.Run("editor.action.addSelectionToNextFindMatch")
	typeText(e, "n")
	if e.guide.step != 2 {
		t.Fatalf("typing with several cursors did not complete the step: %+v", e.guide)
	}
}

// The terminal edition skips the steps only the window can do, and a step can be skipped.
func TestGuideSkipsWindowStepsInTheTerminal(t *testing.T) {
	e := guideEditor(t, keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}})
	e.StartLesson(lessonIndex("layout"))
	for _, id := range []string{"workbench.action.splitEditor", "workbench.action.moveEditorToNextGroup", "workbench.action.focusNextGroup", "view.lockColumn"} {
		e.Run(id)
	}
	if !e.lessonDone("layout") {
		t.Fatalf("terminal steps were not skipped: %+v", e.guide)
	}
	e.StartLesson(lessonIndex("settings")) // what cannot be done here is skipped
	for range 4 {
		e.Run("yan.guide.skip")
	}
	if !e.lessonDone("settings") {
		t.Fatal("skipping every step must finish the lesson")
	}
	e.Run("yan.guide.reset")
	if e.lessonDone("settings") || e.guide.active != -1 {
		t.Fatal("reset did not clear the progress")
	}
}

// The guide opens without a folder too, as on the welcome page.
func TestGuideWithoutAFolder(t *testing.T) {
	e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, macGUI)
	e.Run("yan.guide.open")
	var f view.Frame
	e.Frame(&f)
	if !f.Sidebar.Visible || f.Sidebar.View != view.SideLearn || f.Sidebar.Learn.Tip == "" {
		t.Fatalf("sidebar %+v", f.Sidebar)
	}
}
