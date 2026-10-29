//go:build selftest

package gui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// SelfTestTour walks the feature guide in the real window. Phase "first": the launch offer,
// the maximize lesson done through its commands, the merge lesson in its practice repository
// and back, the palette's release badge, and a lesson left half done before quitting.
// Phase "resume": the next launch resumes that lesson at the same step and does not offer
// the guide again. Captures go to dir/shots.
func (f *Frontend) SelfTestTour(dir, phase string, report func(string)) {
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
			png, err := f.win.CapturePage()
			if err == nil {
				err = os.WriteFile(filepath.Join(shots, phase+"-"+name+".png"), png, 0o644)
			}
			check(err == nil, "captured "+name)
		}
		frame := func() (fr view.Frame) { onMain(func() { fr = f.frame }); return }
		lesson := func(title string) int {
			for i, l := range frame().Sidebar.Learn.Lessons {
				if l.Title == title {
					return i
				}
			}
			return -1
		}
		onMain(func() { f.win.SetSize(1100, 720) })
		settle()
		switch phase {
		case "first":
			check(frame().Guide.Visible, "first launch offers the guide")
			capture("offer")
			onMain(func() { f.ed.Run("yan.guide.open") })
			settle()
			fr := frame()
			check(!fr.Guide.Visible && fr.Sidebar.View == view.SideLearn, "taking the offer opens the guide and ends the offer")

			maximize := lesson(i18n.T("Maximize Editor Area")) // lesson reads the frame on the main thread itself
			onMain(func() { f.ed.StartLesson(maximize) })
			settle()
			onMain(func() { f.ed.Run("workbench.action.splitEditor") })
			onMain(func() { f.ed.Run("workbench.action.toggleMaximizeEditorGroup") })
			settle()
			fr = frame()
			check(fr.Maximized && strings.HasPrefix(fr.Status.Message, i18n.F("Next step: %s", "")), "maximize step done; the status names the next step: "+fr.Status.Message)
			capture("maximized")
			onMain(func() { f.ed.Run("workbench.action.toggleMaximizeEditorGroup") })
			settle()
			l := frame().Sidebar.Learn
			check(!frame().Maximized && l.Lessons[lesson(i18n.T("Maximize Editor Area"))].Done, "restoring finishes the lesson")
			capture("maximize-done")

			home := filepath.Base(f.ed.Root())
			merge := lesson(i18n.T("Three-Way Merge"))
			onMain(func() { f.ed.StartLesson(merge) })
			for range 40 {
				settle()
				if gs := frame().Sidebar.SCM.Groups; len(gs) > 0 || frame().Status.Branch != "" {
					break
				}
			}
			check(strings.Contains(f.ed.Root(), filepath.Join("yan", "learn")), "the merge lesson opens the practice repository: "+f.ed.Root())
			check(frame().Sidebar.Learn.Home == home, "the guide offers the way back to "+home)
			onMain(func() { f.ed.Run("workbench.view.scm") })
			for range 40 {
				settle()
				if len(frame().Columns) == 3 {
					break
				}
				onMain(func() {
					if gs := f.frame.Sidebar.SCM.Groups; len(gs) > 0 && len(gs[0].Rows) > 0 && gs[0].Rows[0].Letter == 'C' {
						f.ed.SCMClick(0, false)
					}
				})
			}
			check(len(frame().Columns) == 3, "the conflict opens as a three-way merge")
			onMain(func() { f.ed.Run("yan.guide.open") })
			settle()
			capture("merge")
			onMain(func() { f.ed.Run("yan.merge.acceptTheirs") })
			onMain(func() { f.ed.Run("yan.merge.markResolved") })
			settle()
			onMain(func() { f.ed.Run("yan.guide.open") })
			settle()
			check(frame().Sidebar.Learn.Lessons[lesson(i18n.T("Three-Way Merge"))].Done, "resolving and marking finishes the merge lesson")
			onMain(func() { f.ed.Run("yan.guide.leave") })
			settle()
			check(filepath.Base(f.ed.Root()) == home, "back in "+f.ed.Root())

			onMain(func() {
				f.ed.Run("workbench.action.showCommands")
				f.ed.SetOverlayInput(i18n.T("Learn: Previous Step"))
			})
			settle()
			fr = frame()
			check(len(fr.Overlay.Items) > 0 && !fr.Overlay.Items[0].Unreleased, "the released Previous Step command has no unreleased badge")
			capture("palette")
			onMain(func() { f.ed.CloseOverlay() })

			editing := lesson(i18n.T("Editing Basics"))
			onMain(func() {
				f.ed.StartLesson(editing)
				f.ed.Run("expandLineSelection")
				f.ed.Run("editor.action.copyLinesDownAction")
				f.ed.Run("workbench.action.files.save")
			})
			settle()
			check(frame().Sidebar.Learn.Steps[2].Current, "two steps of 编辑基础 done before quitting")
		case "resume":
			check(!frame().Guide.Visible, "the next launch does not offer the guide again")
			onMain(func() { f.ed.Run("yan.guide.open") })
			settle()
			l := frame().Sidebar.Learn
			check(l.Active == lesson(i18n.T("Editing Basics")) && len(l.Steps) > 2 && l.Steps[2].Current, "the guide resumes 编辑基础 at step 3")
			col := frame().Columns[frame().Active]
			check(len(col.Tabs) > 0 && col.Tabs[col.Active].Name == "编辑基础.go", "its practice file is open again")
			capture("resumed")
		}
		if failed {
			report("DONE with failures")
		} else {
			report("DONE")
		}
		onMain(func() { mygo.App.Quit() })
	}()
}
