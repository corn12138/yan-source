package app

import (
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/keymap"
)

func TestBranchNameProblem(t *testing.T) {
	have := []git.Branch{{Name: "main"}, {Name: "origin/topic", Remote: true}}
	for name, bad := range map[string]bool{
		"feature/login": false, "topic": false, "": true, "-x": true, "a b": true, "a..b": true,
		"a~1": true, "x.lock": true, ".hidden": true, "a/.b": true, "end/": true, "main": true, "@": true,
	} {
		if got := branchNameProblem(name, have) != ""; got != bad {
			t.Errorf("%q: problem %v, want %v", name, got, bad)
		}
	}
}

// The branch list always offers 新建分支…, which asks for a name, says what is wrong with it as
// it is typed, and creates the branch.
func TestCreateBranchFromThePicker(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a\n"})
	e := newEditorIn(t, root)
	pump(t, e, func() bool { return e.git.status != nil })
	e.Run("git.checkout")
	pump(t, e, func() bool { return len(e.scm.branches) > 0 })
	e.setOverlayInput("")
	if e.ov.items[0].label != "新建分支…" {
		t.Fatalf("items %+v", e.ov.items)
	}
	e.Handle(key(keymap.KeyEnter, 0))
	if e.ov == nil || e.ov.title != "新分支的名字（从 main）" {
		t.Fatal("no prompt")
	}
	typeText(e, "main")
	if e.ov.items[0].label != "已经有分支 main" {
		t.Fatalf("hint %q", e.ov.items[0].label)
	}
	e.setOverlayInput("topic/one")
	e.Handle(key(keymap.KeyEnter, 0))
	pump(t, e, func() bool { return e.branch() == "topic/one" })
}

// The hourly update checks are on unless turned off, which is kept.
func TestUpdateChecksSetting(t *testing.T) {
	e, _, _ := newTestEditor(t, "x")
	path := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(path))
	if !e.UpdateChecks() {
		t.Fatal("off by default")
	}
	e.SetUpdateChecks(false)
	if config.LoadState(path).UpdateChecksOff != true || e.UpdateChecks() {
		t.Fatal("not kept")
	}
}
