package gui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo/ui"
)

func TestMultiRepositoryGUI(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "api"), filepath.Join(root, "web")
	nestedGUIRepo(t, a)
	nestedGUIRepo(t, b)
	os.WriteFile(filepath.Join(a, "a.txt"), []byte("api change\n"), 0o644)
	os.WriteFile(filepath.Join(b, "b.txt"), []byte("web change\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("workbench.view.scm")
	wait := func(ready func() bool) {
		t.Helper()
		until := time.Now().Add(8 * time.Second)
		for !ready() && time.Now().Before(until) {
			h.settle()
		}
		if !ready() {
			t.Fatalf("UI: %v", h.tt.Texts())
		}
	}
	wait(func() bool { return len(h.f.frame.Sidebar.SCM.Panels) == 2 && h.f.frame.Sidebar.Changes == 2 })
	if _, ok := h.tt.Find("仓库 api"); !ok {
		t.Fatal("missing api header")
	}
	if _, ok := h.tt.Find("仓库 web"); !ok {
		t.Fatal("missing web header")
	}
	if err := h.tt.Click("提交说明 · api"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("API draft")
	h.settle()
	if err := h.tt.Click("提交说明 · web"); err != nil {
		t.Fatal(err)
	}
	h.tt.Type("WEB draft")
	h.settle()
	panels := h.f.frame.Sidebar.SCM.Panels
	if panels[0].State.Message != "API draft" || panels[1].State.Message != "WEB draft" {
		t.Fatalf("drafts: %q %q", panels[0].State.Message, panels[1].State.Message)
	}
	if h.ed.Root() != root {
		t.Fatal("switched workspace")
	}
	h.shot("multi-repository-drafts")
	// Menu targets outlive the frame that created them.
	api := repositoryEditor{Editor: h.ed, id: panels[0].State.RepositoryID}
	api.SCMStage(filepath.Join(a, "a.txt"), false)
	h.ed.WithRepository(panels[1].State.RepositoryID, func(ed *app.Editor) { ed.SetCommitMessage("WEB still typing") })
	wait(func() bool {
		for _, g := range h.f.frame.Sidebar.SCM.Panels[0].State.Groups {
			if g.Kind == view.GroupStaged {
				return true
			}
		}
		return false
	})
	if h.f.frame.Sidebar.SCM.Panels[1].State.Message != "WEB still typing" {
		t.Fatal("draft was overwritten by another panel")
	}
	if err := h.tt.Click("仓库 api"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !h.f.frame.Sidebar.SCM.Panels[0].State.Collapsed {
		t.Fatal("collapse did not persist")
	}
	if err := h.tt.Click("仓库 api"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if h.f.frame.Sidebar.SCM.Panels[0].State.Message != "API draft" {
		t.Fatal("collapse lost draft")
	}
	h.shot("multi-repository-staged")
}

func TestRepositorySubmitPlatformHooks(t *testing.T) {
	for _, os := range []string{"darwin", "windows", "linux"} {
		want := ui.Ctrl
		if os == "darwin" {
			want = ui.Cmd
		}
		if got := scmSubmitModifier(guiProfile(os)); got != want {
			t.Fatalf("%s submit=%v want=%v", os, got, want)
		}
	}
}
