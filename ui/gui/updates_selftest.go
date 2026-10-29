//go:build selftest

package gui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/app"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
)

// SelfTestUpdateRestart exercises native updater creation and the app's actual relaunch sequence.
// Only the dialog answer is simulated; configuration and edited files live under dir.
func (f *Frontend) SelfTestUpdateRestart(dir string, report func(string)) {
	mygo.App.SetPath(mygo.PathUserData, filepath.Join(dir, "updater-data"))
	go func() {
		time.Sleep(2 * time.Second)
		onMain := func(fn func()) { done := make(chan struct{}); f.do(func() { fn(); close(done) }); <-done }
		marker := filepath.Join(dir, "restart-pid")
		if b, err := os.ReadFile(marker); err == nil {
			prior, _ := strconv.Atoi(strings.TrimSpace(string(b)))
			data, err := os.ReadFile(filepath.Join(dir, "edited.txt"))
			if err == nil && string(data) == "saved on restart original" && prior != os.Getpid() {
				report("PASS native relaunch saved dirty buffer into a new process")
			} else {
				report(fmt.Sprintf("FAIL restart: data %q err %v same pid %v", data, err, prior == os.Getpid()))
			}
			onMain(func() { mygo.App.Quit() })
			return
		}
		onMain(func() {
			updater.SetAutomaticChecks(false)
			f.updateMenu().Click(nil, f.win)
		})
		time.Sleep(time.Second)
		onMain(func() {
			found := false
			for _, w := range mygo.Windows() {
				if w != f.win {
					found = true
					w.Close()
				}
			}
			if found {
				report("PASS native update window opened from menu")
			} else {
				report("FAIL no native update window")
			}
			if f.ed.AutoSave() {
				f.ed.Run("workbench.action.toggleAutoSave")
			}
			if err := f.ed.OpenFile(filepath.Join(dir, "edited.txt")); err != nil {
				report("FAIL open file " + err.Error())
				return
			}
			f.ed.Handle(app.TextInput{Text: "saved on restart "})
			closeDialog = func(mygo.MessageOptions) (mygo.MessageResult, error) {
				report("PASS restart requested save decision")
				return mygo.MessageResult{Button: 0}, nil
			}
			if err := os.WriteFile(marker, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
				report("FAIL marker " + err.Error())
				return
			}
			mygo.App.Relaunch()
		})
	}()
}

// SelfTestPublishedUpdate replaces only the disposable bundle from which this verification build runs.
func (f *Frontend) SelfTestPublishedUpdate(dir string, report func(string)) {
	mygo.App.SetPath(mygo.PathUserData, filepath.Join(dir, "updater-data"))
	go func() {
		time.Sleep(2 * time.Second)
		// Update lifecycle work must run even while the verification window is occluded.
		onMain := func(fn func()) { done := make(chan struct{}); mygo.RunOnMain(func() { fn(); close(done) }); <-done }
		onMain(func() { updater.SetAutomaticChecks(false) })
		defer onMain(func() { mygo.App.Quit() })
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		want := os.Getenv("YAN_SELFTEST_UPDATE_VERSION")
		if want == "" || !strings.HasPrefix(mygo.App.Version(), want) {
			report("FAIL packaged updater version " + mygo.App.Version())
			return
		}
		up, err := mygo.Updater.Check(ctx)
		if err != nil || up != nil {
			report(fmt.Sprintf("FAIL current public feed: %v, update %v", err, up != nil))
			return
		}
		report("PASS current tagged client is up to date")
		mygo.App.SetVersion("0.0.0")
		up, err = mygo.Updater.Check(ctx)
		if err != nil || up == nil || up.Version != want {
			report(fmt.Sprintf("FAIL older client check: %v", err))
			return
		}
		report("PASS older client found public signed release without authentication")
		if err = up.Install(ctx, nil); err != nil {
			report("FAIL install " + err.Error())
			return
		}
		exe, err := os.Executable()
		if err != nil {
			report("FAIL executable " + err.Error())
			return
		}
		out, err := exec.Command(exe, "--version").CombinedOutput()
		if err != nil || !strings.Contains(string(out), "v"+want+"（") {
			report(fmt.Sprintf("FAIL installed executable: %s %v", out, err))
			return
		}
		report("PASS downloaded signed app replaced the verification bundle: " + strings.TrimSpace(string(out)))
	}()
}
