package gui

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/corn12138/yan-source/internal/updatehttp"
	"github.com/corn12138/yan-source/internal/version"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/plugins/updater/native"
)

// Yan checks for a new version itself, every updateInterval, and shows it in the status bar,
// the Help menu and the welcome page rather than opening the updater's window by itself; a
// click opens it (after Mayoi e2dabc1, MIT).
const (
	updateInterval = time.Hour
	// updateFirstWait is how long after the launch, or after the computer wakes, a check
	// waits, for the network to come up.
	updateFirstWait = 10 * time.Second
	updateTimeout   = time.Minute
)

var (
	updatesReady bool
	// updateChecks says whether the hourly checks run: the Help menu's "自动检查更新", kept
	// in Yan's state.
	updateChecks atomic.Bool
	// updateFound is the newest version the last check found, "" when up to date, and
	// updateShown the one the window shows, "" when the user skipped it. Main thread only.
	updateFound, updateShown string
)

func setupUpdates() {
	// Install before either the plugin or hourly checks starts background requests.
	http.DefaultClient = updatehttp.Client(http.DefaultClient)
	// The macOS bundle version is numeric; comparisons must retain alpha.N from the release tag.
	if v := version.Release(); v != "" {
		mygo.App.SetVersion(v)
	}
	mygo.Use(native.New(updater.Options{Language: "zh-Hans", DisableAutomaticChecks: true,
		Strings: map[string]updater.Strings{"zh-Hans": {CheckError: "检查更新失败。请检查网络或系统代理后重试，也可从发布页下载安装包。"}},
	}))
	updatesReady = true
	updater.OnChange(func() { setUpdate(updateFound) }) // Skip This Version hides the version
	startUpdates()
}

// startUpdates checks for updates every hour from now on, in builds that can update.
func startUpdates() {
	if !mygo.Updater.Enabled() {
		return
	}
	wake := make(chan struct{}, 1)
	mygo.Power.OnResume(func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	})
	go func() {
		timer := time.NewTimer(updateFirstWait)
		for {
			select {
			case <-timer.C:
				if updateChecks.Load() {
					checkUpdate()
				}
				timer.Reset(updateInterval)
			case <-wake: // timers stop while the computer sleeps
				timer.Reset(updateFirstWait)
			}
		}
	}()
}

// checkUpdate asks for the newest version; a check that fails keeps what the last one found.
func checkUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), updateTimeout)
	defer cancel()
	up, err := mygo.Updater.Check(ctx)
	if err != nil {
		return
	}
	found := ""
	if up != nil {
		found = up.Version
	}
	mygo.RunOnMain(func() { setUpdate(found) })
}

// setUpdate keeps the version a check found and shows it anew, unless it was skipped.
func setUpdate(found string) {
	updateFound = found
	shown := found
	if shown != "" && shown == skippedVersion() {
		shown = ""
	}
	if shown == updateShown {
		return
	}
	updateShown = shown
	if f := windows[mygo.FocusedWindow()]; f != nil {
		setAppMenu(f.menu())
	}
	for _, g := range groups {
		if f := g.cur(); f != nil {
			f.do(func() {}) // 状态栏、欢迎页重画
		}
	}
}

// skippedVersion is the version Skip This Version skipped, which the updater keeps in
// updater.json among the app's user data.
func skippedVersion() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, "updater.json"))
	if err != nil {
		return ""
	}
	var st struct {
		SkippedVersion string `json:"skippedVersion"`
	}
	_ = json.Unmarshal(data, &st) // a damaged file skips nothing
	return st.SkippedVersion
}

// installUpdate opens the updater's window for the version found.
func installUpdate() {
	if updatesReady {
		updater.CheckForUpdates()
	}
}

func (f *Frontend) updateMenu() *mygo.MenuItem {
	label := "检查更新…"
	if updateShown != "" {
		label = "更新到 " + updateShown + "…"
	}
	return &mygo.MenuItem{Label: label, Click: func(*mygo.MenuItem, *mygo.Window) { installUpdate() }}
}

func (f *Frontend) automaticUpdatesMenu() *mygo.MenuItem {
	on := updatesReady && f.ed.UpdateChecks()
	return &mygo.MenuItem{Label: "自动检查更新（每小时）", Type: mygo.MenuItemCheckbox, Checked: on,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			if updatesReady {
				f.do(func() {
					f.ed.SetUpdateChecks(!f.ed.UpdateChecks())
					updateChecks.Store(f.ed.UpdateChecks())
					setAppMenu(f.menu())
				})
			}
		},
	}
}
