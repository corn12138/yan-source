//go:build selftest

package gui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/text"
	"github.com/corn12138/yan-source/core/view"
	"github.com/egoist/mygo"
)

// SelfTestLongLine uses disposable files to check native refusal and continued editing.
func (f *Frontend) SelfTestLongLine(dir string, report func(string)) {
	go func() {
		time.Sleep(2 * time.Second)
		onMain := func(fn func()) { done := make(chan struct{}); f.do(func() { fn(); close(done) }); <-done }
		check := func(ok bool, label string) {
			if ok {
				report("PASS " + label)
			} else {
				report("FAIL " + label)
			}
		}
		safe, long := filepath.Join(dir, "keep.txt"), filepath.Join(dir, "bundle.map")
		data := []byte(strings.Repeat("x", 26<<20))
		check(os.WriteFile(safe, []byte("original\n"), 0o600) == nil, "created safe document")
		check(os.WriteFile(long, data, 0o600) == nil, "created 26 MiB single-line fixture")
		inspect := func(want string, dirty bool) bool {
			var fr view.Frame
			f.ed.Frame(&fr)
			if len(fr.Columns) != 1 {
				return false
			}
			c := fr.Columns[0]
			return len(c.Tabs) == 1 && c.Tabs[0].Path == safe && c.Tabs[0].Dirty == dirty && len(c.Pane.Rows) > 0 && string(c.Pane.Rows[0].Text) == want
		}
		onMain(func() {
			f.ed.Run("yan.guide.dismissOffer")
			check(f.ed.OpenFile(safe) == nil, "opened safe document")
			f.ed.Handle(app.TextInput{Text: "mine "})
			err := f.ed.OpenFile(long)
			check(errors.Is(err, text.ErrLineTooLong), "refused oversized line before native layout")
			if err != nil {
				f.ed.Notify(err.Error())
			}
			check(inspect("mine original", true), "kept dirty tab and content")
		})
		// An occluded native window can return its previous surface on the first
		// capture. Wait for the frontend frame to contain the refusal before recording it.
		var shot []byte
		var err error
		painted := false
		for end := time.Now().Add(5 * time.Second); time.Now().Before(end); {
			shot, err = f.win.CapturePage()
			onMain(func() {
				painted = len(f.frame.Columns) == 1 && len(f.frame.Columns[0].Tabs) == 1 && strings.Contains(f.frame.Status.Message, "64 KiB")
			})
			if painted && err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if painted && err == nil {
			time.Sleep(100 * time.Millisecond)
			shot, err = f.win.CapturePage()
			if err == nil {
				err = os.WriteFile(filepath.Join(dir, "refused.png"), shot, 0o600)
			}
		}
		check(painted && err == nil, "captured refusal in native window")
		check(os.WriteFile(safe, data, 0o600) == nil, "external writer created long line")
		onMain(func() {
			f.ed.Handle(app.FocusInput{Gained: true})
			f.ed.Run("workbench.action.files.revert")
			var fr view.Frame
			f.ed.Frame(&fr)
			check(strings.Contains(fr.Status.Message, "64 KiB"), "revert explains refusal")
			check(inspect("mine original", true), "refused reload preserves local edit")
			f.ed.Run("undo")
			check(inspect("original", false), "undo still restores original buffer")
			f.ed.Handle(app.TextInput{Text: "still responsive "})
			check(inspect("still responsive original", true), "editing remains responsive")
			f.ed.Run("undo")
		})
		b, err := os.ReadFile(safe)
		check(err == nil && len(b) == len(data), "external file was not overwritten")
		check(os.WriteFile(safe, []byte("recovered\n"), 0o600) == nil, "external file becomes safe again")
		onMain(func() {
			f.ed.Handle(app.FocusInput{Gained: true})
			check(inspect("recovered", false), "normal reload recovers")
			f.ed.Run("undo")
			check(inspect("original", true), "valid reload stays undoable")
			f.ed.Run("redo")
			mygo.App.Quit()
		})
	}()
}
