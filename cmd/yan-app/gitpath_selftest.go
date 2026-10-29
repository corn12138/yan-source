//go:build selftest

package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/ui/gui"
)

const gitPathFixture = "disposable git path fixture\n"

func runGitPathSelftest(dir string, fe *gui.Frontend, report func(string)) {
	go func() {
		// The fixture lives in an isolated bundle/configuration and is explicitly marked;
		// this verification must never create a commit in a user's open project.
		marker, err := os.ReadFile(filepath.Join(dir, "git-path-fixture"))
		if err != nil || string(marker) != gitPathFixture {
			report("FAIL missing disposable fixture")
			return
		}
		inherited := os.Getenv("PATH")
		repo, err := git.Open(dir)
		if err == nil {
			err = repo.Commit("Verify desktop Git hook paths", false)
		}
		if err != nil {
			report("FAIL Git hook: " + err.Error())
		} else if b, err := os.ReadFile(filepath.Join(dir, "hook-result")); err != nil || strings.TrimSpace(string(b)) != "recovered helper" {
			report("FAIL hook did not find the recovered command")
		} else if os.Getenv("PATH") != inherited {
			report("FAIL application PATH changed")
		} else {
			report("PASS desktop launch committed through a hook using the login shell PATH without changing application PATH")
		}
		// Allow the native window loop to start before requesting its close.
		time.Sleep(time.Second)
		fe.SelfTestClose()
	}()
}

// runGitPathQuitSelftest quits while the fixture's login shell is still hanging, so
// the quit hook, not the probe timeout, has to end the shell's process group. The
// outside script checks the fixture's processes and the elapsed time.
func runGitPathQuitSelftest(dir string, fe *gui.Frontend, report func(string)) {
	go func() {
		marker, err := os.ReadFile(filepath.Join(dir, "git-path-fixture"))
		if err != nil || string(marker) != gitPathFixture {
			report("FAIL missing disposable fixture")
			return
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			if _, err := os.Stat(filepath.Join(dir, "probe-hanging")); err == nil {
				break
			}
			if time.Now().After(deadline) {
				report("FAIL login shell probe did not start")
				fe.SelfTestClose()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		// A close requested before the native window loop starts is lost.
		time.Sleep(time.Second)
		report("READY quitting while the login shell probe hangs")
		fe.SelfTestClose()
	}()
}
