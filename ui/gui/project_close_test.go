package gui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/app"
	"github.com/egoist/mygo"
)

func closingProjects(t *testing.T) (*group, []*harness) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	hs := []*harness{newHarness(t, map[string]string{"a.txt": "one"}, Options{}), newHarness(t, map[string]string{"a.txt": "two"}, Options{}), newHarness(t, map[string]string{"a.txt": "three"}, Options{})}
	g := &group{}
	for _, h := range hs {
		h.open("a.txt")
		h.f.g = g
		g.projects = append(g.projects, h.f)
	}
	old := closeDialog
	t.Cleanup(func() { closeDialog = old })
	return g, hs
}

func TestCloseOtherProjectsConfirmation(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "dialog error"}[failed], func(t *testing.T) {
			g, hs := closingProjects(t)
			calls := 0
			closeDialog = func(o mygo.MessageOptions) (mygo.MessageResult, error) {
				calls++
				if o.DefaultButton != 1 || o.CancelButton != 1 {
					t.Fatal("bulk close should default to cancel")
				}
				if g.mayClose() {
					t.Error("nested window close escaped confirmation")
				}
				g.closeOthers(0)
				if failed {
					return mygo.MessageResult{}, errors.New("unavailable")
				}
				return mygo.MessageResult{Button: 1}, nil
			}
			g.closeOthers(-1)
			g.closeOthers(3)
			g.closeOthers(0)
			if calls != 1 || len(g.projects) != 3 || g.cur() != hs[0].f {
				t.Fatal("cancel changed projects or nested confirmation")
			}
		})
	}
}

func TestCloseOtherProjectsStopsAtUnsavedDecision(t *testing.T) {
	for _, choice := range []string{"cancel", "save-conflict", "save", "discard"} {
		t.Run(choice, func(t *testing.T) {
			g, hs := closingProjects(t)
			hs[1].ed.Handle(app.TextInput{Text: "edited "})
			if choice == "save-conflict" {
				os.WriteFile(filepath.Join(hs[1].root, "a.txt"), []byte("external"), 0600)
			}
			calls := 0
			closeDialog = func(o mygo.MessageOptions) (mygo.MessageResult, error) {
				calls++
				if calls == 1 {
					return mygo.MessageResult{Button: 0}, nil
				}
				if calls != 2 || len(g.projects) != 3 {
					t.Fatal("closed another project before settling the dirty one")
				}
				button := 0
				if choice == "cancel" {
					button = 2
				}
				if choice == "discard" {
					button = 1
				}
				return mygo.MessageResult{Button: button}, nil
			}
			g.closeOthers(0)
			if calls != 2 {
				t.Fatalf("dialogs=%d", calls)
			}
			stopped := choice == "cancel" || choice == "save-conflict"
			if stopped {
				if len(g.projects) != 3 || !hs[1].ed.Dirty() {
					t.Fatal("cancel or save failure lost pending projects")
				}
			} else if len(g.projects) != 1 || g.cur() != hs[0].f {
				t.Fatal("did not keep the selected project")
			}
			disk, _ := os.ReadFile(filepath.Join(hs[1].root, "a.txt"))
			want := "two"
			if choice == "save" {
				want = "edited two"
			}
			if choice == "save-conflict" {
				want = "external"
			}
			if string(disk) != want {
				t.Fatalf("disk=%q want=%q", disk, want)
			}
		})
	}
}

func TestCloseOtherProjectsKeepsNewProjects(t *testing.T) {
	g, hs := closingProjects(t)
	added := newHarness(t, nil, Options{})
	added.f.g = g
	closeDialog = func(mygo.MessageOptions) (mygo.MessageResult, error) {
		g.projects = append(g.projects, added.f)
		return mygo.MessageResult{Button: 0}, nil
	}
	g.closeOthers(0)
	if len(g.projects) != 2 || g.projects[0] != hs[0].f || g.projects[1] != added.f {
		t.Fatal("closed a project opened after confirmation began")
	}
}
