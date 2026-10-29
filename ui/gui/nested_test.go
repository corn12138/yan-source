package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
)

func nestedGUIRepo(t *testing.T, root string) {
	t.Helper()
	os.MkdirAll(root, 0o755)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "--allow-empty", "-qm", "initial"}} {
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s %v", args, out, err)
		}
	}
}

func TestNestedRepositoriesGUI(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "group", "project")
	nestedGUIRepo(t, child)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("workbench.view.scm")
	wait := func(ready func() bool) {
		t.Helper()
		until := time.Now().Add(8 * time.Second)
		for !ready() && time.Now().Before(until) {
			h.settle()
		}
		if !ready() {
			t.Fatalf("UI did not settle: %v", h.tt.Texts())
		}
	}
	wait(func() bool { return h.tt.HasText("group/project") })
	if h.ed.Root() != root || h.f.frame.Sidebar.SCM.NoRepo {
		t.Fatal("repository selection changed workspace")
	}
	if len(h.f.frame.Sidebar.SCM.Panels) != 1 || h.f.frame.Sidebar.SCM.Panels[0].State.Root != child {
		t.Fatal("missing repository panel")
	}
	h.shot("nested-repositories")
	if err := h.tt.Click("仓库 group/project"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.f.frame.Sidebar.SCM.Panels[0].State.Collapsed {
		t.Fatal("repository header did not collapse")
	}

}

func TestNestedProjectReusePreservesParent(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	root := t.TempDir()
	child := filepath.Join(root, "child")
	nestedGUIRepo(t, child)
	h := newHarnessAt(t, root, Options{})
	old := makeWindow
	t.Cleanup(func() { makeWindow = old })
	SetWindowMaker(func() (*app.Editor, *Frontend) {
		f := New(Options{})
		ed, _ := app.New(config.Default(), f.Layouter(), f.Host(), f.Profile())
		f.ed = ed
		t.Cleanup(ed.Close)
		return ed, f
	})
	on := true
	h.ed.SetState(&config.State{ProjectTabs: &on})
	g := &group{}
	g.attach(h.f, h.ed)
	g.projects = []*Frontend{h.f}
	os.WriteFile(filepath.Join(root, "draft.txt"), []byte("draft"), 0o644)
	h.open("draft.txt")
	h.ed.Handle(app.TextInput{Text: "parent draft"})
	if err := (host{h.f}).OpenRepository(child); err != nil {
		t.Fatal(err)
	}
	if len(g.projects) != 2 || g.cur().ed.Root() != child || h.ed.Root() != root || !h.ed.Dirty() {
		t.Fatalf("opening child: projects=%d current=%q parent=%q dirty=%v", len(g.projects), g.cur().ed.Root(), h.ed.Root(), h.ed.Dirty())
	}
	target := g.cur()
	target.ed.SetCommitMessage("child draft")
	g.show(0)
	if err := (host{h.f}).OpenRepository(child); err != nil {
		t.Fatal(err)
	}
	if len(g.projects) != 2 || g.cur() != target {
		t.Fatal("existing child project was duplicated")
	}
	tt := ui.NewTester(g.view, 1200, 780)
	var frame view.Frame
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		target.ed.RunPending()
		tt.Frame()
		target.ed.Frame(&frame)
		if !frame.Sidebar.SCM.NoRepo {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if frame.Sidebar.SCM.Message != "child draft" {
		t.Fatalf("existing project message: %+v", frame.Sidebar.SCM)
	}
}
