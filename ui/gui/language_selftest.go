//go:build selftest

package gui

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// SelfTestLanguage checks the interface language in the real app: the language this launch
// resolved and the titles macOS shows in the menu bar; then, with an unsaved document and a
// terminal open and a second window, it saves the preference YAN_SELFTEST_LANG_CHOOSE in
// Settings and checks that it is saved for the next launch while nothing running changes
// language, restarts or loses work. YAN_SELFTEST_PHASE names the screenshots under dir/shots.
func (f *Frontend) SelfTestLanguage(dir string, report func(string)) {
	go func() {
		time.Sleep(2 * time.Second)
		onMain := func(fn func()) { done := make(chan struct{}); f.do(func() { fn(); close(done) }); <-done }
		settle := func() { time.Sleep(700 * time.Millisecond) }
		phase := os.Getenv("YAN_SELFTEST_PHASE")
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
		capture := func(w *mygo.Window, name string) {
			png, err := w.CapturePage()
			if err == nil {
				err = os.WriteFile(filepath.Join(shots, phase+"-"+name+".png"), png, 0o644)
			}
			check(err == nil, "captured "+name)
		}
		launch := i18n.Current()
		report("ACTIVE " + string(launch))
		report("SYSTEM " + strings.Join(i18n.SystemLanguages(), ","))
		onMain(func() { report("SAVED " + string(f.ed.LanguagePreference())) })
		var titles []string
		mygo.RunOnMain(func() { titles = nativeMenuTitles() })
		report("MENUBAR " + strings.Join(titles, " | "))

		previewPath := filepath.Join(dir, "language-preview.md")
		check(os.WriteFile(previewPath, []byte("# Preview\n\nA bilingual fixture: 简体中文 and English.\n\n- [ ] Task\n"), 0o644) == nil, "created preview fixture")
		onMain(func() {
			check(f.ed.OpenFile(previewPath) == nil, "opened preview source")
			var before, after view.Frame
			f.ed.Frame(&before)
			source := before.Active
			f.ed.Run("workbench.action.splitEditor")
			f.ed.FocusColumn(source)
			f.ed.Run("markdown.showPreviewToSide")
			f.ed.Frame(&after)
			check(len(after.Columns) == len(before.Columns)+1 && after.Active == source && after.Columns[source+1].Pane.Kind == view.PaneMarkdown, "preview reuses the adjacent column and keeps source focus")
			f.ed.Run("markdown.showPreviewToSide")
			var repeated view.Frame
			f.ed.Frame(&repeated)
			check(len(repeated.Columns) == len(after.Columns) && len(repeated.Columns[source+1].Tabs) == len(after.Columns[source+1].Tabs), "repeated preview adds no column or tab")
		})
		settle()
		capture(f.win, "preview")
		onMain(func() { f.ed.Run("actions.find"); f.ed.SetFindQuery("English") })
		settle()
		capture(f.win, "find")
		onMain(func() { f.ed.Run("yan.find.close") })

		// Work that a restart would put at risk: an unsaved untitled document, an edited file
		// and a terminal.
		onMain(func() {
			if f.ed.AutoSave() {
				f.ed.Run("workbench.action.toggleAutoSave")
			}
			f.ed.OpenFile(filepath.Join(dir, "dirty.txt"))
			f.ed.Handle(app.TextInput{Text: "edited "})
			f.ed.Run("workbench.action.files.newUntitledFile")
			f.ed.Handle(app.TextInput{Text: "draft"})
			f.ed.Run("workbench.action.terminal.new")
		})
		settle()
		var second *Frontend
		// Restored project tabs may already contain another phase's folders. Fresh paths
		// ensure this checks new native windows rather than focusing a restored project.
		secondPath := filepath.Join(dir, "other-"+phase)
		thirdPath := filepath.Join(dir, "third-"+phase)
		os.MkdirAll(secondPath, 0o755)
		os.MkdirAll(thirdPath, 0o755)
		windowsBeforeChange := 0
		onMain(func() {
			before := len(windows)
			err := f.openFolder(secondPath, true)
			second = f.folderProject(secondPath)
			check(err == nil && second != nil && len(windows) == before+1, "second window before the change")
			windowsBeforeChange = len(windows)
			f.bringForward()
		})
		settle()
		onMain(func() { f.ed.Run("workbench.action.showCommands"); f.ed.SetOverlayInput("git") })
		settle()
		capture(f.win, "palette")
		onMain(func() { f.ed.Run("yan.settings.open") })
		settle()
		capture(f.win, "settings")
		for _, appearance := range []string{"light", "dark"} {
			for _, size := range [][2]int{{760, 520}, {1200, 780}} {
				onMain(func() {
					f.ed.Run("yan.appearance." + appearance)
					f.win.SetSize(size[0], size[1])
				})
				settle()
				name := "settings-" + appearance + "-normal"
				if size[0] == 760 {
					name = "settings-" + appearance + "-compact"
				}
				capture(f.win, name)
			}
		}

		choose := i18n.Preference(os.Getenv("YAN_SELFTEST_LANG_CHOOSE"))
		if choose != "" {
			onMain(func() {
				var fr view.Frame
				f.ed.Frame(&fr)
				for i, it := range fr.Overlay.Items {
					if it.Group != fr.Overlay.Items[0].Group {
						break
					}
					if want := []i18n.Preference{i18n.System, i18n.Preference(i18n.SimplifiedChinese), i18n.Preference(i18n.English)}; i < len(want) && want[i] == choose {
						f.ed.ChooseOverlay(i)
					}
				}
			})
			settle()
			capture(f.win, "chosen")
			var fr view.Frame
			onMain(func() { f.ed.Frame(&fr) })
			pending := i18n.Resolve(choose, i18n.SystemLanguages()) != launch
			report("NOTICE " + fr.Overlay.Notice)
			check(fr.Overlay.Visible && fr.Overlay.Kind == view.OverlaySettings, "Settings stays open after a choice")
			check((fr.Overlay.Notice != "") == pending, "restart notice only when the next launch differs")
			st := config.LoadState(config.StatePath())
			check(st.Language != nil && *st.Language == string(choose), "saved in state.json for the next launch")
			check(i18n.Current() == launch, "this process keeps its language")
			onMain(func() {
				second.ed.Run("yan.settings.open")
				var fr2 view.Frame
				second.ed.Frame(&fr2)
				check(fr2.Overlay.Notice == fr.Overlay.Notice && fr2.Overlay.Title == fr.Overlay.Title, "the other window agrees about the saved choice")
				second.bringForward()
			})
			settle()
			capture(second.win, "second-window")
			// A window opened after the change still starts in the launch language.
			var third *Frontend
			onMain(func() {
				if err := f.openFolder(thirdPath, true); err == nil {
					third = f.folderProject(thirdPath)
				}
				title := func(e *app.Editor) string {
					for _, c := range e.Commands() {
						if c.ID == "workbench.action.files.save" {
							return c.Title
						}
					}
					return ""
				}
				ok := third != nil && title(third.ed) == i18n.T("File: Save")
				if third != nil {
					report("THIRD-WINDOW save command: " + title(third.ed))
				}
				check(ok, "a window opened after the change uses the launch language")
				f.bringForward()
			})
			settle()
			// Switching projects in a window changes neither the language nor the preference.
			onMain(func() {
				check(len(windows) == windowsBeforeChange+1, "no window closed by the change")
				before := f.ed.LanguagePreference()
				if !f.ed.ProjectTabs() { // projects as tabs: this gathers the folders into one window
					f.ed.Run("yan.projectTabs.toggle")
				}
				err := f.openFolder(filepath.Join(dir, "project"), false)
				now := f
				if f.g != nil {
					now = f.g.cur()
				}
				if err != nil {
					report("project switch: " + err.Error())
				}
				check(err == nil && now != f && now.ed.Root() == filepath.Join(dir, "project") && i18n.Current() == launch && now.ed.LanguagePreference() == before, "switching projects keeps the language")
			})
			settle()
		}
		onMain(func() {
			dirty := 0
			for _, g := range groups {
				for _, p := range g.projects {
					if p.ed.Dirty() {
						dirty++
					}
				}
			}
			check(dirty > 0, "unsaved work is still open")
		})
		mygo.RunOnMain(func() { titles = nativeMenuTitles() })
		report("MENUBAR-AFTER " + strings.Join(titles, " | "))
		// Quitting still asks about the unsaved file: cancel keeps everything running.
		asked := make(chan struct{}, 1)
		closeDialog = func(mygo.MessageOptions) (mygo.MessageResult, error) {
			select {
			case asked <- struct{}{}:
			default:
			}
			return mygo.MessageResult{Button: 2}, nil
		}
		openWindows := 0
		onMain(func() { openWindows = len(windows); mygo.App.Quit() })
		settle()
		select {
		case <-asked:
			check(true, "quitting asks about unsaved changes")
		default:
			check(false, "quitting asks about unsaved changes")
		}
		onMain(func() { check(len(windows) == openWindows && openWindows > 0, "cancelled quit keeps the windows") })
		if failed {
			report("DONE with failures")
		} else {
			report("DONE")
		}
		closeDialog = func(mygo.MessageOptions) (mygo.MessageResult, error) { return mygo.MessageResult{Button: 1}, nil }
		onMain(func() { mygo.App.Quit() })
	}()
}

// nativeMenuTitles are the titles macOS shows in the menu bar, each with its submenu's
// item titles, read from NSApp's main menu itself.
func nativeMenuTitles() []string {
	sel := objc.RegisterName
	title := func(item objc.ID) string {
		s := item.Send(sel("title"))
		if s == 0 {
			return ""
		}
		return objc.Send[string](s, sel("UTF8String"))
	}
	itemsOf := func(menu objc.ID) []objc.ID {
		if menu == 0 {
			return nil
		}
		arr := menu.Send(sel("itemArray"))
		n := objc.Send[uint](arr, sel("count"))
		out := make([]objc.ID, 0, n)
		for i := range n {
			out = append(out, arr.Send(sel("objectAtIndex:"), i))
		}
		return out
	}
	nsApp := objc.ID(objc.GetClass("NSApplication")).Send(sel("sharedApplication"))
	var out []string
	for _, top := range itemsOf(nsApp.Send(sel("mainMenu"))) {
		var kids []string
		for _, it := range itemsOf(top.Send(sel("submenu"))) {
			if t := title(it); t != "" {
				kids = append(kids, t)
			}
		}
		if len(kids) > 6 {
			kids = append(kids[:6], "…")
		}
		out = append(out, title(top)+": "+strings.Join(kids, ", "))
	}
	return out
}
