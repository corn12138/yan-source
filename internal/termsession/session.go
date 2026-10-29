// Package termsession 结束一个关掉的终端里还在跑的程序。
//
// 内嵌终端的 shell 自成一个会话（setsid）。关终端时 MyGo 挂断伪终端（SIGHUP），两秒后只杀 shell 自己的进程组；
// 交互式 shell 给每个作业另开进程组，所以接住或忽略 SIGHUP 的程序（开发服务器、监视器）会留在后台，
// 再也看不到、停不掉。这里在挂断之后，把那个会话（会话号就是 shell 的进程号）里还活着的进程都结束掉。
// 用 setsid 自己脱离出去的守护进程（如 docker compose up -d）不在这个会话里，不动它们。
package termsession

import (
	"os"
	"sync"
	"time"
)

var pending sync.WaitGroup

// End 在后台等会话 sid 里的进程退出（挂断之后），过了 grace 还在的都杀掉。
func End(sid int, grace time.Duration) {
	if sid <= 0 {
		return
	}
	pending.Add(1)
	go func() {
		defer pending.Done()
		end(sid, grace)
	}()
}

func end(s int, grace time.Duration) {
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if len(members(s)) == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, pid := range members(s) {
		kill(pid)
	}
}

// Wait 等 End 收尾，最多等 d：退出应用时用，免得进程先退出、留下没杀掉的程序。
func Wait(d time.Duration) {
	done := make(chan struct{})
	go func() {
		pending.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// members 是会话 s 里还活着的进程。
func members(s int) []int {
	var out []int
	for _, pid := range all() {
		if pid != os.Getpid() && sid(pid) == s {
			out = append(out, pid)
		}
	}
	return out
}
