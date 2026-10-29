package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLauncherPreservesArgumentsAndDoesNotClobber(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX launcher")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "app's binary")
	output := filepath.Join(dir, "received")
	os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s\\n' \"$PWD\" \"$@\" > \"$YAN_LAUNCH_TEST\"\n"), 0755)
	t.Setenv("YAN_LAUNCH_TEST", output)
	bin := filepath.Join(dir, "bin")
	installed, err := Install(bin, exe, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(installed, "a b.go:7", "$(touch nope)", "--new-window")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("launcher: %v %s", err, out)
	}
	var got string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, _ := os.ReadFile(output)
		got = string(b)
		if strings.Contains(got, "--new-window") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got != dir+"\na b.go:7\n$(touch nope)\n--new-window\n" {
		t.Fatalf("args %q", got)
	}
	if _, err = Install(bin, exe, "darwin"); err != nil {
		t.Fatalf("idempotent install: %v", err)
	}
	os.WriteFile(installed, []byte("user program"), 0755)
	if _, err = Install(bin, exe, "darwin"); err == nil {
		t.Fatal("overwrote existing program")
	}
	b, _ := os.ReadFile(installed)
	if string(b) != "user program" {
		t.Fatal("existing command changed")
	}
	os.Remove(installed)
	os.Symlink(exe, installed)
	if _, err = Install(bin, exe, "darwin"); err == nil {
		t.Fatal("overwrote symlink")
	}
	if _, err = Install(filepath.Join(dir, "windows"), exe, "windows"); err == nil {
		t.Fatal("missing platform implementation accepted")
	}
}
