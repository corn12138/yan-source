package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// 提交浮层里的合并：遇到冲突停下来，源代码管理说一声，提交说明里是 git 拟的；解决、暂存、「继续」提交完。
// 再合并一次冲突的，「中止」回到合并之前。重置问软、混合还是硬；HEAD 已有的提交只能还原，不能合并。
func TestCommitOperations(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "one\n"})
	day := 0
	gitIn := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		day++
		date := fmt.Sprintf("2030-02-%02dT12:00:00Z", day)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(s string) { os.WriteFile(filepath.Join(root, "a.txt"), []byte(s), 0o644) }
	gitIn("switch", "-q", "-c", "side")
	write("side\n")
	gitIn("commit", "-q", "-am", "side change")
	gitIn("switch", "-q", "main")
	write("main\n")
	gitIn("commit", "-q", "-am", "main change")

	e := newEditorIn(t, root)
	e.Run("workbench.view.scm")
	e.Run("yan.scm.toggleAllBranches")
	pump(t, e, func() bool { return len(e.scm.history.commits) == 3 })
	open := func(subject string) {
		t.Helper()
		e.closeOverlay()
		for i, c := range e.scm.history.commits {
			if c.Subject == subject {
				e.openCommit(i)
			}
		}
		pump(t, e, func() bool { return e.ov != nil && e.ov.kind == view.OverlayChoice })
	}
	labels := func() string {
		var ls []string
		for _, it := range e.ov.choices {
			ls = append(ls, it.label)
		}
		return strings.Join(ls, "|")
	}
	choose := func(prefix string) {
		t.Helper()
		for i, it := range e.ov.items {
			if strings.HasPrefix(it.label, prefix) {
				e.ov.sel = i
				e.Handle(key(keymap.KeyEnter, 0))
				return
			}
		}
		t.Fatalf("no %q in %s", prefix, labels())
	}

	open("side change")
	if l := labels(); !strings.Contains(l, "合并到 main") || !strings.Contains(l, "拣选到 main") || strings.Contains(l, "还原这个提交") {
		t.Fatalf("side commit offers %s", l)
	}
	choose("合并到 main")
	pump(t, e, func() bool { return e.git.busy == "" && e.git.status != nil && e.git.status.Op == git.OpMerge })
	var f view.Frame
	e.Frame(&f)
	if f.Sidebar.SCM.Op != "合并" || !strings.Contains(e.scm.message, "Merge branch 'side'") {
		t.Fatalf("op %q, message %q, status %q", f.Sidebar.SCM.Op, e.scm.message, e.status)
	}
	if err := e.continueOp(); err == nil || !strings.Contains(err.Error(), "解决冲突") {
		t.Fatalf("continue with conflicts: %v", err)
	}
	write("both\n")
	gitIn("add", "a.txt")
	pump(t, e, func() bool {
		e.gitChanged()
		return e.git.status != nil && len(e.git.status.Entries) == 1 && !e.git.status.Entries[0].Conflicted()
	})
	if err := e.continueOp(); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return e.git.busy == "" && e.git.status != nil && e.git.status.Op == "" })
	if parents := gitIn("log", "-1", "--format=%P"); len(strings.Fields(parents)) != 2 {
		t.Fatalf("merge commit parents %q", parents)
	}

	// HEAD 已有的提交：还原，不能合并；重置问三种方式
	pump(t, e, func() bool { return len(e.scm.history.commits) == 4 })
	open("main change")
	if l := labels(); !strings.Contains(l, "还原这个提交") || strings.Contains(l, "合并到") {
		t.Fatalf("main commit offers %s", l)
	}
	choose("把 main 重置到这里")
	if l := labels(); !strings.Contains(l, "软重置") || !strings.Contains(l, "硬重置") {
		t.Fatalf("reset offers %s", l)
	}
	e.closeOverlay()

	// 又一个冲突的合并：中止
	gitIn("switch", "-q", "side")
	write("side again\n")
	gitIn("commit", "-q", "-am", "side again")
	gitIn("switch", "-q", "main")
	write("main again\n")
	gitIn("commit", "-q", "-am", "main again")
	head := gitIn("rev-parse", "HEAD")
	e.gitChanged()
	pump(t, e, func() bool {
		for _, c := range e.scm.history.commits {
			if c.Subject == "side again" {
				return true
			}
		}
		return false
	})
	open("side again")
	choose("合并到 main")
	pump(t, e, func() bool { return e.git.busy == "" && e.git.status != nil && e.git.status.Op == git.OpMerge })
	e.abortOp()
	choose("中止合并")
	pump(t, e, func() bool { return e.git.busy == "" && e.git.status != nil && e.git.status.Op == "" })
	if got := gitIn("rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD after abort %s, want %s", got, head)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "a.txt")); string(b) != "main again\n" {
		t.Fatalf("a.txt after abort %q", b)
	}
}
