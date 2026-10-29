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

// SelfTestMath exercises the packaged native renderer with disposable documents and settings.
func (f *Frontend) SelfTestMath(dir string, report func(string)) {
	go func() {
		time.Sleep(2 * time.Second)
		onMain := func(fn func()) { done := make(chan struct{}); f.do(func() { fn(); close(done) }); <-done }
		failed := false
		check := func(ok bool, label string) {
			if ok {
				report("PASS " + label)
			} else {
				failed = true
				report("FAIL " + label)
			}
		}
		path := filepath.Join(dir, "math.md")
		source := "# Formula preview\n\n中文 and English: $E=mc^2$, $\\frac{a}{b}$, $\\sqrt{x^2+y^2}$.\n\n$$\\int_0^\\infty e^{-x^2}\\,dx=\\frac{\\sqrt{\\pi}}{2}$$\n\n```math\n\\begin{pmatrix}1&2\\\\3&4\\end{pmatrix}\n```\n\n| Left | Center | Right |\n|:--|:--:|--:|\n| $a_1$ | $b^2$ | $\\sqrt{c}$ |\n\n$$\\nosuchmacro$$\n\nIt costs $5 and $6.\n"
		check(os.WriteFile(path, []byte(source), 0o644) == nil, "created formula fixture")
		onMain(func() {
			f.ed.Run("yan.guide.dismissOffer")
			check(f.ed.OpenFile(path) == nil, "opened formula source")
			f.ed.Run("markdown.showPreviewToSide")
		})
		wait := func(enabled bool) bool {
			for end := time.Now().Add(30 * time.Second); time.Now().Before(end); {
				// Occluded windows intentionally defer frames. Capture advances their native
				// content while this unattended test runs behind another application.
				if _, err := f.win.CapturePage(); err != nil {
					report("FAIL capture during wait: " + err.Error())
					return false
				}
				ready := false
				onMain(func() {
					m := f.markdowns[path]
					ready = m != nil && m.loaded && !m.running && m.wait == nil && m.cache.math == enabled
				})
				if ready {
					return true
				}
				time.Sleep(100 * time.Millisecond)
			}
			onMain(func() {
				var fr view.Frame
				f.ed.Frame(&fr)
				kinds := []view.PaneKind{}
				for _, col := range fr.Columns {
					kinds = append(kinds, col.Pane.Kind)
				}
				_, _, _, generation := f.ed.Settings()
				report(fmt.Sprintf("WAIT setting=%v preview-created=%v active-project=%v visible=%v focused=%v kinds=%v settings=%d/%d", f.ed.PreviewMath(), f.markdowns[path] != nil, f.g.cur() == f, f.win.IsVisible(), mygo.FocusedWindow() == f.win, kinds, f.settingsGen, generation))
			})
			return false
		}
		check(wait(true), "rendered native formulas")
		shots := filepath.Join(dir, "shots")
		os.MkdirAll(shots, 0o755)
		for _, appearance := range []string{"light", "dark"} {
			for _, size := range [][2]int{{1440, 900}, {1100, 720}} {
				onMain(func() { f.ed.Run("yan.appearance." + appearance); f.win.SetSize(size[0], size[1]) })
				check(wait(true), "rendered "+appearance)
				time.Sleep(800 * time.Millisecond)
				png, err := f.win.CapturePage()
				if err == nil {
					err = os.WriteFile(filepath.Join(shots, fmt.Sprintf("%s-%d.png", appearance, size[0])), png, 0o644)
				}
				check(err == nil, "captured "+appearance)
			}
		}
		onMain(func() { f.ed.Run("workbench.action.zoomIn"); f.ed.Run("workbench.action.zoomIn") })
		time.Sleep(800 * time.Millisecond)
		png, err := f.win.CapturePage()
		if err == nil {
			err = os.WriteFile(filepath.Join(shots, "larger-font.png"), png, 0o644)
		}
		check(err == nil, "captured larger font")
		onMain(func() {
			var fr view.Frame
			f.ed.Frame(&fr)
			check(len(fr.Columns) == 2 && fr.Columns[1].Pane.Kind == view.PaneMarkdown, "adjacent preview preserved")
			f.ed.FocusColumn(0)
			f.ed.Run("cursorBottom")
			f.ed.Handle(app.TextInput{Text: "\n$x+2$\n"})
		})
		time.Sleep(500 * time.Millisecond)
		check(wait(true), "preview follows source edits")
		onMain(func() { f.ed.Run("undo") })
		time.Sleep(500 * time.Millisecond)
		check(wait(true), "preview follows undo")
		onMain(func() {
			var fr view.Frame
			f.ed.Frame(&fr)
			check(string(fr.Columns[1].Pane.Markdown) == source, "undo preserves formula delimiters")
		})
		check(os.WriteFile(filepath.Join(dir, ".yan.toml"), []byte("[ui]\npreview_math=false\n"), 0o644) == nil, "disabled formulas")
		check(wait(false), "native config watcher restores source")
		onMain(func() {
			for _, b := range f.markdowns[path].blocks {
				check(b.formula == nil, "no display formula after disabling")
			}
		})
		check(os.WriteFile(filepath.Join(dir, ".yan.toml"), []byte("[ui]\npreview_math=true\n"), 0o644) == nil, "enabled formulas")
		check(wait(true), "native config watcher restores formulas")
		if failed {
			report("DONE with failures")
		} else {
			report("DONE")
		}
		onMain(func() { mygo.App.Quit() })
	}()
}
