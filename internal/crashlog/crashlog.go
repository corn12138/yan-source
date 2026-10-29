// Package crashlog 让窗口版崩溃时留下记录：从访达或 Dock 打开时 stderr 没人看。
// Adapted from Mayoi v0.8.0 (MIT, Copyright (c) 2026 chencheng, cmd/mayoi/crashlog.go).
package crashlog

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
)

// maxCrashes is how many crash logs the crashes folder keeps.
const maxCrashes = 10

// Keep has the runtime write the panic or fatal error that ends
// the app, with every goroutine's stack, to crash.log in dir as well as
// to stderr, which nothing keeps when the app starts from the Finder or
// the Dock. A crash.log a crash left is moved first into the crashes
// folder, named by its time, so that each launch starts an empty one.
func Keep(dir string) {
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return
	}
	if fi, err := os.Lstat(dir); err != nil || !fi.IsDir() {
		return
	}
	crashes := filepath.Join(dir, "crashes")
	if !privateCrashDir(crashes) {
		return
	}
	pruneCrashes(crashes)
	path := filepath.Join(dir, "crash.log")
	f, err := openPrivateLog(path)
	if err == nil {
		fi, statErr := f.Stat()
		if statErr != nil {
			f.Close()
			return
		}
		if fi.Size() > 0 {
			f.Close()
			name := fi.ModTime().Format("2006-01-02-150405") + ".log"
			if os.Rename(path, filepath.Join(crashes, name)) != nil {
				return
			}
			pruneCrashes(crashes)
			f = nil
		}
	} else if !os.IsNotExist(err) {
		return
	}
	if f == nil {
		// Exclusive creation cannot truncate a file or follow a link placed here
		// after rotation. Crash traces may contain paths and document contents.
		f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return
		}
	}
	debug.SetCrashOutput(f, debug.CrashOptions{})
	f.Close() // SetCrashOutput keeps its own copy
}

func privateCrashDir(dir string) bool {
	if os.MkdirAll(dir, 0o700) != nil {
		return false
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	defer f.Close()
	opened, err := f.Stat()
	return err == nil && os.SameFile(fi, opened) && f.Chmod(0o700) == nil
}

// Permissions must also cover retained logs. Compare the opened file with its
// non-following lookup before chmod so a substituted link cannot change its target.
func openPrivateLog(path string) (*os.File, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, os.ErrInvalid
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err == nil && !os.SameFile(fi, opened) {
		err = os.ErrInvalid
	}
	if err == nil {
		err = f.Chmod(0o600)
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

// pruneCrashes removes all but the newest maxCrashes logs of dir.
func pruneCrashes(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".log") {
			if f, err := openPrivateLog(filepath.Join(dir, e.Name())); err == nil {
				f.Close()
			}
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	for len(names) > maxCrashes {
		os.Remove(filepath.Join(dir, names[0]))
		names = names[1:]
	}
}
