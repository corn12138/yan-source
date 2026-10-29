//go:build selftest

package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

func (f *Frontend) SelfTestWindowWorkflows(dir string, report func(string)) {
	go func() {
		time.Sleep(2 * time.Second)
		onMain := func(fn func()) { done := make(chan struct{}); f.do(func() { fn(); close(done) }); <-done }
		failed := false
		check := func(ok bool, label string) {
			if !ok {
				failed = true
				report("FAIL " + label)
			} else {
				report("PASS " + label)
			}
		}
		onMain(func() {
			if f.ed.AutoSave() {
				f.ed.Run("workbench.action.toggleAutoSave")
			}
			if !f.ed.ProjectTabs() {
				f.ed.Run("yan.projectTabs.toggle")
			}
			f.ed.OpenFile(filepath.Join(dir, "dirty.txt"))
			f.ed.Handle(app.TextInput{Text: "unsaved "})
			f.ed.Run("workbench.action.toggleSidebarPosition")
			err := f.openFolder(filepath.Join(dir, "other"), true)
			check(err == nil && len(windows) == 2, "explicit native window with project tabs enabled")
			f.bringForward()
		})
		time.Sleep(time.Second)
		onMain(func() {
			f.win.Hide()
		})
		time.Sleep(time.Second)
		onMain(func() { check(!f.win.IsVisible(), "original hidden before command-line request") })
		exe, err := os.Executable()
		if err == nil {
			err = exec.Command(exe, filepath.Join(dir, "dirty.txt:2")).Run()
		}
		onMain(func() { check(err == nil, "second process delivered file request") })
		time.Sleep(time.Second)
		onMain(func() {
			var frame view.Frame
			f.ed.Frame(&frame)
			atLine := false
			for _, it := range frame.Status.Right {
				if strings.HasPrefix(it.Text, i18n.T("Ln 2, ")) {
					atLine = true
				}
			}
			check(len(windows) == 2 && f.win.IsVisible() && f.win.IsFocused(), "existing hidden window restored and focused")
			check(atLine && f.ed.Dirty(), "line navigation preserved unsaved buffer")
			check(frame.Sidebar.Right, "right sidebar in native window")
			err := f.openFolder(filepath.Join(dir, "alias"), true)
			check(err == nil && len(windows) == 2, "symlink folder reused")
			f.refreshDock()
			items := mygo.App.Dock.Menu().Items()
			check(len(items) == 3, "Dock contains both recent folders")
			other := f.folderProject(filepath.Join(dir, "other"))
			other.bringForward()
		})
		time.Sleep(time.Second)
		onMain(func() {
			f.folderProject(filepath.Join(dir, "other")).win.Hide()
		})
		time.Sleep(time.Second)
		onMain(func() {
			other := f.folderProject(filepath.Join(dir, "other"))
			check(!other.win.IsVisible(), "Dock target is hidden before callback")
			for _, item := range mygo.App.Dock.Menu().Items() {
				if strings.HasSuffix(item.Label, "/other") {
					item.Click(item, nil)
				}
			}
		})
		time.Sleep(time.Second)
		onMain(func() {
			other := f.folderProject(filepath.Join(dir, "other"))
			check(other.win.IsVisible() && other.win.IsFocused() && len(windows) == 2, "Dock callback restores existing window")
			f.ed.Run("workbench.action.files.saveAll")
			if failed {
				report("RESULT FAIL")
			} else {
				report("RESULT PASS")
			}
			for w := range windows {
				if w != f.win {
					w.Destroy()
				}
			}
			f.win.Destroy()
		})
	}()
}
