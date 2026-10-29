package crashlog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

func TestKeepCrashes(t *testing.T) {
	defer debug.SetCrashOutput(nil, debug.CrashOptions{})
	dir := t.TempDir()
	crashes := filepath.Join(dir, "crashes")
	os.MkdirAll(crashes, 0o755)
	for i := range maxCrashes {
		os.WriteFile(filepath.Join(crashes, fmt.Sprintf("2020-01-%02d-000000.log", i+1)), []byte("old"), 0o644)
	}
	path := filepath.Join(dir, "crash.log")
	os.WriteFile(path, []byte("panic: boom"), 0o644)
	when := time.Date(2026, 10, 14, 9, 30, 5, 0, time.Local)
	os.Chtimes(path, when, when)

	Keep(dir)

	if b, err := os.ReadFile(filepath.Join(crashes, "2026-10-14-093005.log")); string(b) != "panic: boom" {
		t.Fatalf("moved crash = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(crashes, "2020-01-01-000000.log")); !os.IsNotExist(err) {
		t.Errorf("oldest crash kept: %v", err)
	}
	if entries, _ := os.ReadDir(crashes); len(entries) != maxCrashes {
		t.Errorf("%d crashes kept, want %d", len(entries), maxCrashes)
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() != 0 {
		t.Errorf("crash.log after launch: %v, %v", fi, err)
	}

	// An empty crash.log, from a launch that did not crash, stays put.
	Keep(dir)
	if entries, _ := os.ReadDir(crashes); len(entries) != maxCrashes {
		t.Errorf("empty crash.log moved: %d crashes", len(entries))
	}
}

func TestKeepPrivatePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	defer debug.SetCrashOutput(nil, debug.CrashOptions{})
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%v", legacy), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "config")
			crashes := filepath.Join(dir, "crashes")
			active := filepath.Join(dir, "crash.log")
			retained := filepath.Join(crashes, "2020-01-01-000000.log")
			if legacy {
				if err := os.MkdirAll(crashes, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(crashes, 0o755); err != nil {
					t.Fatal(err)
				}
				for _, path := range []string{active, retained} {
					if err := os.WriteFile(path, []byte("panic: private data"), 0o644); err != nil {
						t.Fatal(err)
					}
					if err := os.Chmod(path, 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			Keep(dir)
			wantDir := os.FileMode(0o700)
			if legacy {
				// The caller's directory can be shared with other configuration files.
				wantDir = 0o755
			}
			assertMode(t, dir, wantDir)
			assertMode(t, crashes, 0o700)
			assertMode(t, active, 0o600)
			if legacy {
				entries, err := os.ReadDir(crashes)
				if err != nil || len(entries) != 2 {
					t.Fatalf("retained crashes: %v, %v", entries, err)
				}
				for _, e := range entries {
					assertMode(t, filepath.Join(crashes, e.Name()), 0o600)
				}
			}
			if err := os.Chmod(active, 0o644); err != nil {
				t.Fatal(err)
			}
			Keep(dir)
			assertMode(t, active, 0o600)
		})
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != want {
		t.Fatalf("%s mode = %o, want %o", path, fi.Mode().Perm(), want)
	}
}

func TestKeepDoesNotFollowSymlinks(t *testing.T) {
	defer debug.SetCrashOutput(nil, debug.CrashOptions{})
	for _, location := range []string{"active", "retained", "archive-dir", "config-dir", "dangling-active"} {
		t.Run(location, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "config")
			if err := os.MkdirAll(filepath.Join(dir, "crashes"), 0o755); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			target := filepath.Join(outside, "unrelated.log")
			if err := os.WriteFile(target, []byte("do not change"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(target, 0o644); err != nil {
				t.Fatal(err)
			}
			link, dest := filepath.Join(dir, "crash.log"), target
			switch location {
			case "retained":
				link = filepath.Join(dir, "crashes", "2000-01-01-000000.log")
			case "archive-dir":
				link, dest = filepath.Join(dir, "crashes"), outside
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
			case "config-dir":
				link, dest = filepath.Join(t.TempDir(), "config-link"), outside
				dir = link
			case "dangling-active":
				dest = filepath.Join(outside, "missing.log")
			}
			if err := os.Symlink(dest, link); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			Keep(dir)
			if got, err := os.ReadFile(target); err != nil || string(got) != "do not change" {
				t.Fatalf("symlink target modified: %q, %v", got, err)
			}
			if runtime.GOOS != "windows" {
				assertMode(t, target, 0o644)
			}
			if got, err := os.Readlink(link); err != nil || got != dest {
				t.Fatalf("symlink replaced: %q, %v", got, err)
			}
			if entries, err := os.ReadDir(outside); err != nil || len(entries) != 1 {
				t.Fatalf("wrote through symlink: %v, %v", entries, err)
			}
		})
	}
}
