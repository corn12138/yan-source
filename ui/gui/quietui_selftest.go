//go:build selftest

package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/corn12138/yan-source/core/app"
)

// SelfTestQuietUI walks the Quiet UI in the real window at its two target sizes, light and dark,
// and saves what the window draws under dir/shots. In a merge beside an editor terminal it
// maximizes and restores, and checks with a live shell that the terminal kept its session.
func (f *Frontend) SelfTestQuietUI(dir string, report func(string)) {
	go func() {
		time.Sleep(2 * time.Second)
		onMain := func(fn func()) { done := make(chan struct{}); f.do(func() { fn(); close(done) }); <-done }
		settle := func() { time.Sleep(700 * time.Millisecond) }
		shots := filepath.Join(dir, "shots")
		os.MkdirAll(shots, 0o755)
		failed := false
		check := func(ok bool, label string) {
			if ok {
				report("PASS " + label)
			} else {
				failed = true
				report("FAIL " + label)
			}
		}
		capture := func(name string) {
			png, err := f.win.CapturePage() // from this goroutine: it waits for the main thread itself
			if err == nil {
				err = os.WriteFile(filepath.Join(shots, name+".png"), png, 0o644)
			}
			check(err == nil, "captured "+name)
		}
		closeDialog = func(mygo.MessageOptions) (mygo.MessageResult, error) { return mygo.MessageResult{Button: 1}, nil }
		onMain(func() {
			if f.ed.AutoSave() {
				f.ed.Run("workbench.action.toggleAutoSave")
			}
			f.ed.OpenFile(filepath.Join(f.ed.Root(), "README.md"))
			f.ed.OpenFile(filepath.Join(f.ed.Root(), "main.go"))
			f.ed.Handle(app.TextInput{Text: "// 未保存的修改\n"})
		})
		for _, appearance := range []string{"light", "dark"} {
			for _, size := range [][2]int{{1440, 900}, {1100, 720}} {
				name := fmt.Sprintf("%s-%dx%d", appearance, size[0], size[1])
				onMain(func() {
					f.ed.Run("yan.appearance." + appearance)
					f.win.SetSize(size[0], size[1])
					f.ed.Run("workbench.view.explorer")
				})
				settle()
				var w, h int
				var bar, left float32
				onMain(func() { w, h = f.win.Size(); bar, left = f.nativeBar.Height, f.nativeBar.Left })
				check(w == size[0] && h == size[1], fmt.Sprintf("%s window is %dx%d", name, w, h))
				report(fmt.Sprintf("INFO %s native window controls %.0f high and %.0f wide on the left; yan's title bar %.0f high",
					name, bar, left, max(bar, titleBarH)))
				capture(name + "-workspace")
				onMain(func() { f.ed.Run("workbench.view.scm") })
				settle()
				capture(name + "-scm")
			}
		}

		// A merge beside an editor terminal at the compact size.
		onMain(func() { f.ed.Run("yan.appearance.light"); f.win.SetSize(1100, 720) })
		opened := false
		for i := 0; i < 40 && !opened; i++ {
			onMain(func() {
				if gs := f.frame.Sidebar.SCM.Groups; len(gs) > 0 && len(gs[0].Rows) > 0 && gs[0].Rows[0].Letter == 'C' {
					f.ed.SCMClick(0, false)
				}
				for _, c := range f.frame.Columns {
					opened = opened || c.Merge
				}
			})
			time.Sleep(150 * time.Millisecond)
		}
		check(opened, "merge view opened from Source Control")
		var s *termSession
		onMain(func() {
			f.ed.Run("workbench.action.splitEditor")
			f.ed.Run("workbench.action.createTerminalEditor")
		})
		settle()
		onMain(func() {
			for _, c := range f.frame.Columns {
				for _, t := range c.Tabs {
					if t.Terminal != 0 {
						s = f.tabTerms[t.Terminal]
					}
				}
			}
			f.ed.FocusColumn(1)
		})
		shellSays := func(want string) bool {
			for i := 0; i < 40; i++ {
				if s != nil && s.t != nil && strings.Contains(s.t.Text(), want) {
					return true
				}
				time.Sleep(150 * time.Millisecond)
			}
			return false
		}
		check(s != nil && s.t != nil, "editor terminal started beside the merge")
		if s == nil || s.t == nil {
			report("DONE")
			onMain(func() { mygo.App.Quit() })
			return
		}
		time.Sleep(time.Second) // the shell's prompt
		s.t.Send([]byte("echo QUIET_ONE_$((40+2))\r"))
		check(shellSays("QUIET_ONE_42"), "shell answered before maximizing")
		capture("light-1100x720-merge-terminal")
		onMain(func() { f.ed.Run("workbench.action.toggleMaximizeEditorGroup") })
		settle()
		var maximized bool
		onMain(func() { maximized = f.frame.Maximized })
		check(maximized, "merge maximized")
		capture("light-1100x720-merge-maximized")
		onMain(func() { f.ed.Run("yan.appearance.dark") })
		settle()
		capture("dark-1100x720-merge-maximized")
		onMain(func() { f.ed.Run("yan.appearance.light"); f.ed.Run("workbench.action.toggleMaximizeEditorGroup") })
		settle()
		var kept bool
		onMain(func() {
			for _, c := range f.frame.Columns {
				for _, t := range c.Tabs {
					if t.Terminal != 0 && f.tabTerms[t.Terminal] == s {
						kept = true
					}
				}
			}
		})
		check(kept && !s.closed && !f.frame.Maximized, "restored layout with the same terminal session")
		s.t.Send([]byte("echo QUIET_TWO_$((40+3))\r"))
		check(shellSays("QUIET_TWO_43") && strings.Contains(s.t.Text(), "QUIET_ONE_42"), "same shell answered after restoring, earlier output still there")
		capture("light-1100x720-merge-restored")
		if failed {
			report("DONE with failures")
		} else {
			report("DONE")
		}
		onMain(func() { mygo.App.Quit() })
	}()
}
