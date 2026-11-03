//go:build selftest

package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// SelfTestTerminalFonts exercises font eviction in real windows with disposable shells.
func (f *Frontend) SelfTestTerminalFonts(dir string, report func(string)) {
	go func() {
		time.Sleep(2 * time.Second)
		onMain := func(fn func()) {
			done := make(chan struct{})
			f.do(func() { defer close(done); fn() })
			<-done
		}
		defer onMain(func() { mygo.App.Quit() })
		check := func(ok bool, label string) bool {
			if ok {
				report("PASS " + label)
			} else {
				report("FAIL " + label)
			}
			return ok
		}
		capture := func(name string) bool {
			time.Sleep(100 * time.Millisecond)
			// Capture advances occluded windows too, including EndFrame's font eviction.
			png, err := f.win.CapturePage()
			if err == nil && name != "" {
				err = os.WriteFile(filepath.Join(dir, name+".png"), png, 0o644)
			}
			return check(err == nil, "native frame "+name)
		}
		wait := func(s *termSession, marker string) bool {
			for end := time.Now().Add(10 * time.Second); time.Now().Before(end); {
				if strings.Contains(s.t.Text(), marker) {
					return true
				}
				time.Sleep(50 * time.Millisecond)
			}
			return false
		}
		var sessions []*termSession
		path := filepath.Join(dir, "font-check.txt")
		if !check(os.WriteFile(path, []byte("English and 中文 日本語 한국어 remain visible after font recycling.\n"), 0o644) == nil, "created editor fixture") {
			return
		}
		onMain(func() {
			f.ed.Run("yan.guide.dismissOffer")
			check(f.ed.OpenFile(path) == nil, "opened editor fixture")
			f.win.SetSize(1200, 800)
			for range 2 {
				f.newTerminal(dir)
			}
			sessions = append(sessions, f.terms...)
		})
		if !check(len(sessions) == 2, "opened two independent shells") {
			return
		}
		for i, s := range sessions {
			s.t.Send([]byte(fmt.Sprintf("printf 'shell-%%s 中文 日本語 한국어\\n' %d\r", i)))
			if !check(wait(s, fmt.Sprintf("shell-%d 中文", i)), "shell output") {
				return
			}
			onMain(func() { f.termActive = i })
			capture("")
		}
		for cycle := range 3 {
			onMain(func() {
				// Exceed MyGo's 512-font budget without changing the terminal's own font
				// or output, so an inactive terminal must recover its retained glyphs.
				for i := range 640 {
					ui.ShapeText("cache 中文", ui.Font{Size: 10 + float32(i)/8}, 0)
				}
			})
			capture("")
			for i := range sessions {
				onMain(func() { f.termActive = i })
				if !capture(fmt.Sprintf("recycled-%d-shell-%d", cycle, i)) {
					return
				}
			}
		}
		for _, appearance := range []string{"light", "dark"} {
			onMain(func() { f.ed.Run("yan.appearance." + appearance) })
			for _, size := range []float64{9, 14, 20, 28, 20, 14, 9, 14} {
				onMain(func() { _, _, current, _ := f.ed.Settings(); f.ed.ZoomFont(size - current) })
				if !capture("") {
					return
				}
			}
			onMain(func() { f.panelToColumn(sessions[0], f.frame.Active, 0) })
			capture(appearance + "-editor-terminal")
			onMain(func() { f.columnToPanel(sessions[0].id) })
			capture(appearance + "-panel-terminal")
		}
		for i, s := range sessions {
			s.t.Send([]byte(fmt.Sprintf("printf 'responsive-%%s\\n' %d\r", i)))
			check(wait(s, fmt.Sprintf("responsive-%d", i)), "original shell still responds")
		}
		onMain(func() {
			for _, s := range sessions {
				f.removeTerminal(s)
			}
			check(len(f.terms) == 0 && len(f.tabTerms) == 0, "all test terminals closed")
		})
	}()
}
