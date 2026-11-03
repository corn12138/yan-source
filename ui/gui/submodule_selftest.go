//go:build selftest

package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo"
)

// SelfTestSubmodule checks native input focus and rendering using a disposable
// repo under dir/repo with an initialized, dirty modules/lib submodule.
func (f *Frontend) SelfTestSubmodule(dir string, report func(string)) {
	go func() {
		time.Sleep(time.Second)
		onMain := func(fn func()) { done := make(chan struct{}); f.do(func() { fn(); close(done) }); <-done }
		defer onMain(func() { mygo.App.Quit() })
		failed := false
		check := func(ok bool, label string) bool {
			if ok {
				report("PASS " + label)
			} else {
				failed = true
				report("FAIL " + label)
			}
			return ok
		}
		root, module := filepath.Join(dir, "repo"), filepath.Join(dir, "repo", "modules", "lib")
		var parentID, childID uint64
		onMain(func() {
			check(f.ed.Root() == root, "disposable workspace")
			f.ed.Run("yan.guide.dismissOffer")
			f.ed.Run("workbench.view.scm")
		})
		if failed {
			return
		}
		wait := func(label string, ready func(view.SCM) bool) bool {
			stable := 0
			for end := time.Now().Add(20 * time.Second); time.Now().Before(end); {
				// Capturing advances the native input's deferred focus even when occluded.
				if _, err := f.win.CapturePage(); err != nil {
					return check(false, label+": "+err.Error())
				}
				ok := false
				onMain(func() { var fr view.Frame; f.ed.Frame(&fr); ok = ready(fr.Sidebar.SCM) })
				if ok {
					stable++
				} else {
					stable = 0
				}
				if stable == 3 {
					return check(true, label)
				}
				time.Sleep(100 * time.Millisecond)
			}
			onMain(func() {
				var fr view.Frame
				f.ed.Frame(&fr)
				report(fmt.Sprintf("STATE %s repository=%d focused=%v focusRoot=%v current=%v", label, fr.Sidebar.SCM.RepositoryID, fr.Sidebar.SCM.MessageFocused, f.focusRoot, f.g.cur() == f))
			})
			return check(false, label)
		}
		if !wait("discovered parent and dirty submodule", func(s view.SCM) bool {
			found := false
			for _, p := range s.Panels {
				if p.State.Root == root {
					parentID = p.State.RepositoryID
					for _, g := range p.State.Groups {
						for _, r := range g.Rows {
							found = found || r.Path == module && r.Submodule && r.Detail != ""
						}
					}
				} else if p.State.Root == module {
					childID = p.State.RepositoryID
				}
			}
			return parentID != 0 && childID != 0 && found
		}) {
			return
		}
		onMain(func() {
			f.ed.WithRepository(parentID, func(ed *app.Editor) { ed.SetCommitMessage("Parent draft"); ed.FocusCommitMessage(true) })
		})
		wait("parent native input focused", func(s view.SCM) bool { return s.Root == root && s.MessageFocused })
		onMain(func() { f.openSubmoduleChanges(repositoryEditor{Editor: f.ed, id: parentID}, module) })
		wait("navigation retains workspace and parent draft", func(s view.SCM) bool {
			return s.Root == module && !s.Panels[1].State.Collapsed && f.ed.Root() == root && s.Panels[0].State.Message == "Parent draft"
		})
		onMain(func() {
			f.ed.WithRepository(childID, func(ed *app.Editor) { ed.SetCommitMessage("Child draft"); ed.FocusCommitMessage(true) })
		})
		wait("child native input focused", func(s view.SCM) bool { return s.Root == module && s.MessageFocused })
		onMain(func() { f.ed.SelectRepository(parentID); f.focusRoot = true })
		wait("old native input cannot reclaim selection", func(s view.SCM) bool {
			return s.Root == root && s.Panels[0].State.Message == "Parent draft" && s.Panels[1].State.Message == "Child draft"
		})
		onMain(func() { f.ed.ToggleRepository(childID) })
		wait("arrow folds without changing history target", func(s view.SCM) bool { return s.Root == root && s.Panels[1].State.Collapsed })
		onMain(func() { f.ed.SelectRepository(childID); f.focusRoot = true })
		wait("name selects without unfolding", func(s view.SCM) bool { return s.Root == module && s.Panels[1].State.Collapsed })
		onMain(func() { f.ed.Run("list.focusMessage") })
		wait("keyboard focus reveals retained child draft", func(s view.SCM) bool {
			return s.Root == module && s.MessageFocused && !s.Panels[1].State.Collapsed && s.Message == "Child draft"
		})
		shots := filepath.Join(dir, "shots")
		check(os.MkdirAll(shots, 0o755) == nil, "screenshot directory")
		for _, appearance := range []string{"light", "dark"} {
			for _, width := range []int{1100, 760} {
				onMain(func() { f.ed.Run("yan.appearance." + appearance); f.win.SetSize(width, 720) })
				time.Sleep(400 * time.Millisecond)
				png, err := f.win.CapturePage()
				if err == nil {
					err = os.WriteFile(filepath.Join(shots, fmt.Sprintf("%s-%d.png", appearance, width)), png, 0o644)
				}
				check(err == nil, "captured "+appearance)
			}
		}
		if failed {
			report("DONE with failures")
		} else {
			report("DONE")
		}
	}()
}
