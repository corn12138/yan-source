//go:build selftest

package gui

import (
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/corn12138/yan-source/core/app"
)

// SelfTestUntitledRecovery checks that untitled text outlives a real quit. Phase "keep" types a draft
// into two projects without a folder and closes the window the way quitting does; any close question
// means the drafts were not kept. Phase "restore" runs in the relaunched app with the same
// configuration and finds both drafts back, still unsaved.
func (f *Frontend) SelfTestUntitledRecovery(phase string, report func(string)) {
	go func() {
		time.Sleep(2 * time.Second)
		onMain := func(fn func()) { done := make(chan struct{}); f.do(func() { fn(); close(done) }); <-done }
		switch phase {
		case "keep":
			onMain(func() {
				if f.ed.Root() != "" {
					report("FAIL started with a folder")
					return
				}
				f.ed.Run("workbench.action.files.newUntitledFile")
				f.ed.Handle(app.TextInput{Text: "alpha draft"})
				g := f.g
				g.add("")
				g.cur().ed.Run("workbench.action.files.newUntitledFile")
				g.cur().ed.Handle(app.TextInput{Text: "beta draft"})
				closeDialog = func(o mygo.MessageOptions) (mygo.MessageResult, error) {
					report("FAIL closing asked: " + o.Detail)
					return mygo.MessageResult{Button: 2}, nil
				}
				report("READY drafts typed in " + strings.Repeat("a project ", len(g.projects)))
			})
			time.Sleep(500 * time.Millisecond)
			onMain(func() { f.g.win.Close() })
		case "restore":
			onMain(func() {
				var texts []string
				for _, g := range groups {
					for _, pf := range g.projects {
						texts = append(texts, pf.ed.SelfTestUntitled()...)
					}
				}
				slices.Sort(texts)
				if slices.Equal(texts, []string{"alpha draft", "beta draft"}) {
					report("PASS both drafts restored after relaunch, unsaved")
				} else {
					report("FAIL restored " + strings.Join(texts, " | "))
				}
			})
			onMain(func() { mygo.App.Quit() })
		}
	}()
}
