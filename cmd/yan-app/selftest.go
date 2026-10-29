//go:build selftest

package main

// 原生窗口的验证构建（go build -tags selftest，不随发布包分发）：在真的 macOS 窗口里，由内核 API
// 代替键盘在 dirty.txt 里打字，然后每 300 毫秒把两栏的状态写进 $YAN_SELFTEST/selftest.log。
// 外面的脚本改文件、切到别的应用再切回来，看窗口失去、获得焦点时内核是怎么做的。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/view"
	"github.com/corn12138/yan-source/ui/gui"
)

func init() { selftest = runSelftest }

func runSelftest(ed *app.Editor, fe *gui.Frontend) {
	dir := os.Getenv("YAN_SELFTEST")
	if dir == "" {
		return
	}
	log, err := os.OpenFile(filepath.Join(dir, "selftest.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	enc := json.NewEncoder(log)
	if os.Getenv("YAN_SELFTEST_MODE") == "git-login-path" {
		runGitPathSelftest(dir, fe, func(s string) { enc.Encode(map[string]string{"GitLoginPath": s}) })
		return
	}
	if os.Getenv("YAN_SELFTEST_MODE") == "git-login-path-quit" {
		runGitPathQuitSelftest(dir, fe, func(s string) { enc.Encode(map[string]string{"GitLoginPathQuit": s}) })
		return
	}
	if os.Getenv("YAN_SELFTEST_MODE") == "untitled-recovery" {
		fe.SelfTestUntitledRecovery(os.Getenv("YAN_SELFTEST_PHASE"), func(s string) { enc.Encode(map[string]string{"UntitledRecovery": s}) })
		return
	}
	if os.Getenv("YAN_SELFTEST_MODE") == "quiet-ui" {
		fe.SelfTestQuietUI(dir, func(s string) { enc.Encode(map[string]string{"QuietUI": s}) })
		return
	}
	if os.Getenv("YAN_SELFTEST_MODE") == "tour" {
		fe.SelfTestTour(dir, os.Getenv("YAN_SELFTEST_PHASE"), func(s string) { enc.Encode(map[string]string{"Tour": s}) })
		return
	}
	if os.Getenv("YAN_SELFTEST_MODE") == "review" {
		fe.SelfTestReview(dir, func(s string) { enc.Encode(map[string]string{"Review": s}) })
		return
	}
	if os.Getenv("YAN_SELFTEST_MODE") == "update-feed" {
		fe.SelfTestPublishedUpdate(dir, func(s string) { enc.Encode(map[string]string{"PublishedUpdate": s}) })
		return
	}
	if os.Getenv("YAN_SELFTEST_MODE") == "update-restart" {
		fe.SelfTestUpdateRestart(dir, func(s string) { enc.Encode(map[string]string{"UpdateRestart": s}) })
		return
	}
	if os.Getenv("YAN_SELFTEST_MODE") == "window-workflows" {
		fe.SelfTestWindowWorkflows(dir, func(s string) { enc.Encode(map[string]string{"WindowWorkflow": s}) })
		return
	}
	if os.Getenv("YAN_SELFTEST_MODE") == "terminal" {
		fe.SelfTestTerminal(dir, func(s string) { enc.Encode(map[string]string{"At": time.Now().Format("15:04:05.000"), "Terminal": s}) })
		return
	}
	type pane struct {
		Tab   string
		Dirty bool
		Line  string
	}
	type entry struct {
		At     string
		Event  string `json:",omitempty"`
		Panes  []pane
		Status string
	}
	snapshot := func(e *app.Editor, event string) {
		var f view.Frame
		e.Frame(&f)
		en := entry{At: time.Now().Format("15:04:05.000"), Event: event, Status: f.Status.Message}
		for _, c := range f.Columns {
			p := pane{}
			if c.Active >= 0 && c.Active < len(c.Tabs) {
				p.Tab, p.Dirty = c.Tabs[c.Active].Name, c.Tabs[c.Active].Dirty
			}
			if len(c.Pane.Rows) > 0 {
				p.Line = string(c.Pane.Rows[0].Text)
			}
			en.Panes = append(en.Panes, p)
		}
		enc.Encode(en)
	}
	switch os.Getenv("YAN_SELFTEST_MODE") {
	case "session": // 开两栏、关窗口退出；下次（observe）看标签回来没有
		go func() {
			time.Sleep(2 * time.Second)
			ed.Post(func(e *app.Editor) {
				e.OpenFile(filepath.Join(dir, "a.txt"))
				e.Run("workbench.action.splitEditor")
				e.OpenFile(filepath.Join(dir, "b.txt"))
				snapshot(e, "before quit")
			})
			time.Sleep(time.Second)
			fe.SelfTestClose()
		}()
		return
	case "windows": // 自动换行的长行、⇧⌘N 新建窗口、关掉新窗口后应用还在
		go func() {
			time.Sleep(2 * time.Second)
			ed.Post(func(e *app.Editor) {
				e.OpenFile(filepath.Join(dir, "long.txt"))
				if !e.WordWrap() {
					e.Run("editor.action.toggleWordWrap")
				}
				var f view.Frame
				e.Frame(&f)
				enc.Encode(map[string]any{"Event": "wrapped", "Rows": len(f.Columns[0].Pane.Rows), "Width": f.Columns[0].Pane.Size.W})
				e.Run("workbench.action.newWindow")
			})
			time.Sleep(2 * time.Second)
			enc.Encode(map[string]any{"Event": "after new window", "Windows": fe.SelfTestWindows()})
			fe.SelfTestCloseOthers()
			time.Sleep(time.Second)
			enc.Encode(map[string]any{"Event": "after closing it", "Windows": fe.SelfTestWindows()})
			fe.SelfTestClose()
		}()
		return
	case "projects": // 项目标签：打开、加项目、⇧⌘N 加空项目、关掉（分成几个窗口）、再打开（并回一个窗口）、关一个项目
		go func() {
			time.Sleep(2 * time.Second)
			report := func(event string) {
				n, ps, cur := fe.SelfTestProjects()
				enc.Encode(map[string]any{"Event": event, "Windows": n, "Projects": ps, "Active": cur})
			}
			step := func(event string, fn func()) {
				fn()
				time.Sleep(1500 * time.Millisecond)
				report(event)
			}
			step("tabs on", func() { fe.SelfTestRun("yan.projectTabs.toggle") })
			step("added a folder", func() { fe.SelfTestAddProject(filepath.Join(dir, "second")) })
			step("new window adds a project", func() { fe.SelfTestRun("workbench.action.newWindow") })
			step("previous project", func() { fe.SelfTestRun("yan.project.previous") })
			step("tabs off: a window each", func() { fe.SelfTestRun("yan.projectTabs.toggle") })
			step("tabs on: gathered", func() { fe.SelfTestRun("yan.projectTabs.toggle") })
			step("closed a project", func() { fe.SelfTestRun("yan.project.close") })
			fe.SelfTestClose()
		}()
		return
	case "observe-projects": // 上次的项目都回来了
		go func() {
			time.Sleep(3 * time.Second)
			n, ps, cur := fe.SelfTestProjects()
			enc.Encode(map[string]any{"Event": "after relaunch", "Windows": n, "Projects": ps, "Active": cur})
			fe.SelfTestClose() // 一个窗口：关掉它就退出
		}()
		return
	case "observe":
		go func() {
			time.Sleep(2 * time.Second)
			ed.Post(func(e *app.Editor) { snapshot(e, "after relaunch") })
			time.Sleep(time.Second)
			fe.SelfTestClose()
		}()
		return
	}
	go func() {
		time.Sleep(2 * time.Second)
		ed.Post(func(e *app.Editor) {
			e.OpenFile(filepath.Join(dir, "dirty.txt"))
			e.Run("workbench.action.splitEditor")
			e.OpenFile(filepath.Join(dir, "clean.txt"))
			e.FocusColumn(0)
			snapshot(e, "ready")
		})
		// 有 typing 这个文件时每 400 毫秒打一个字：自动保存的 1 秒计时一直等不到，只剩失去焦点时保存。
		for range time.Tick(400 * time.Millisecond) {
			typing := false
			if _, err := os.Stat(filepath.Join(dir, "typing")); err == nil {
				typing = true
			}
			ed.Post(func(e *app.Editor) {
				if typing {
					e.Handle(app.TextInput{Text: "x"})
				}
				snapshot(e, "")
			})
		}
	}()
}
