//go:build selftest

package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/view"
)

// SelfTestTerminal 是原生窗口验证构建用的（-tags selftest，不随发布包分发）：在真窗口里用随包的
// libghostty-vt 开一个终端，跑命令、改大小、Ctrl+C、关掉，最后关窗口退出；每一步交给 report。
func (f *Frontend) SelfTestTerminal(dir string, report func(string)) {
	onMain := func(fn func()) {
		done := make(chan struct{})
		f.do(func() { fn(); close(done) })
		<-done
	}
	wait := func(s *termSession, want string, n int) bool {
		for range 100 {
			if strings.Count(s.t.Text(), want) >= n {
				return true
			}
			time.Sleep(100 * time.Millisecond)
		}
		return false
	}
	go func() {
		time.Sleep(2 * time.Second)
		var s *termSession
		var status string
		onMain(func() {
			f.newTerminal(dir)
			if len(f.terms) > 0 {
				s = f.terms[len(f.terms)-1]
			}
			status = f.frame.Status.Message
		})
		if s == nil {
			report("terminal open: FAIL " + status)
			// 终端开不了也不能影响编辑：打字、保存
			var msg string
			onMain(func() {
				f.ed.OpenFile(filepath.Join(dir, "a.txt"))
				f.ed.Handle(app.TextInput{Text: "still editing "})
				f.ed.Run("workbench.action.files.save")
				var fr view.Frame
				f.ed.Frame(&fr)
				msg = fr.Status.Message
			})
			b, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
			report(fmt.Sprintf("editing after the failure: %q, status %q", b, msg))
			return
		}
		report("terminal open: ok")
		s.t.Send([]byte("echo yan-term-$((6*7))\r"))
		report(fmt.Sprint("command output: ", wait(s, "yan-term-42", 1)))
		onMain(func() { f.panelToColumn(s, f.frame.Active, 0) })
		time.Sleep(400 * time.Millisecond)
		var id int
		onMain(func() { id = s.id })
		s.t.Send([]byte("echo tab-session-$((6*7))\r"))
		report(fmt.Sprint("editor terminal retains shell: ", id != 0 && wait(s, "tab-session-42", 1)))
		onMain(func() { f.columnToPanel(id) })
		time.Sleep(400 * time.Millisecond)
		var moved bool
		onMain(func() { moved = s.id == 0 && len(f.terms) == 1 && f.terms[0] == s })
		report(fmt.Sprint("panel restores same shell: ", moved))
		s.t.Resize(100, 30)
		c, r := s.t.Size()
		report(fmt.Sprintf("resize: %dx%d", c, r))
		s.t.Send([]byte("sleep 30; echo not-interrupted\r"))
		time.Sleep(time.Second)
		s.t.Send([]byte{3})
		s.t.Send([]byte("echo after-ctrl-c-$((1+1))\r"))
		report(fmt.Sprint("ctrl+c returns to the prompt: ", wait(s, "after-ctrl-c-2", 1), ", interrupted: ", strings.Count(s.t.Text(), "not-interrupted") == 1)) // 只有敲进去的那行命令
		onMain(func() { f.removeTerminal(s) })
		report(fmt.Sprint("closed, terminals left: ", len(f.terms)))
		onMain(func() { f.newTerminal(dir) }) // 开着终端退出
		report("quitting with a terminal open")
		onMain(func() { f.win.Close() })
	}()
}

// SelfTestClose 像点了红色关闭按钮一样关窗口（没有未保存的修改时就退出）。
func (f *Frontend) SelfTestClose() { f.do(func() { f.win.Close() }) }

// SelfTestWindows 报告开着几个窗口（在主线程上数）。
func (f *Frontend) SelfTestWindows() int {
	n := make(chan int)
	f.do(func() { n <- len(windows) })
	return <-n
}

// SelfTestCloseOthers 关掉除了 f 以外的窗口。
func (f *Frontend) SelfTestCloseOthers() {
	f.do(func() {
		for w := range windows {
			if w != f.win {
				w.Close()
			}
		}
	})
}

// SelfTestProjects 报告开着几个窗口、前面那个窗口里有哪些项目、现在是哪个（在主线程上看）。
func (f *Frontend) SelfTestProjects() (windowsOpen int, projects []string, active string) {
	type res struct {
		n     int
		names []string
		cur   string
	}
	ch := make(chan res)
	f.do(func() {
		r := res{n: len(windows)}
		if g := f.g; g != nil {
			for _, pf := range g.projects {
				r.names = append(r.names, projectName(pf.ed.Root()))
			}
			r.cur = projectName(g.cur().ed.Root())
		}
		ch <- r
	})
	r := <-ch
	return r.n, r.names, r.cur
}

// SelfTestAddProject 在 f 的窗口里加一个项目，打开 dir。
func (f *Frontend) SelfTestAddProject(dir string) { f.do(func() { f.g.add(dir) }) }

// SelfTestRun 在主线程上让窗口里现在那个项目运行命令 id。
func (f *Frontend) SelfTestRun(id string) { f.do(func() { f.g.cur().ed.Run(id) }) }
