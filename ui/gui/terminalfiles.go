package gui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// File link resolution follows Mayoi PR #77 (xierenyuan, MIT). It bypasses
// the search index so ignored build output can still lead to a local file.
func (f *Frontend) openTerminalFile(s *termSession, link string) bool {
	if s.closed {
		return false
	}
	dir := s.dir
	if s.t != nil {
		if reported := s.t.Dir(); filepath.IsAbs(reported) {
			dir = reported
		}
	}
	home, _ := os.UserHomeDir()
	path, line, column, ok := terminalFileTarget(link, dir, home)
	if !ok || f.ed.OpenFile(path) != nil {
		return false
	}
	if line > 0 {
		f.ed.GotoLocation(line, column)
	}
	f.focusRoot = true
	return true
}

func terminalFileTarget(link, dir, home string) (path string, line, column int, ok bool) {
	if link == "" || strings.Contains(link, "://") || strings.HasPrefix(strings.ToLower(link), "mailto:") {
		return
	}
	resolve := func(name string) string {
		if strings.HasPrefix(name, "~/") || strings.HasPrefix(name, `~\`) {
			if home == "" {
				return ""
			}
			name = filepath.Join(home, name[2:])
		}
		if !filepath.IsAbs(name) {
			if !filepath.IsAbs(dir) {
				return ""
			}
			name = filepath.Join(dir, name)
		}
		return filepath.Clean(name)
	}
	path = resolve(link)
	info, err := os.Stat(path)
	if err != nil {
		// A literal colon belongs to an existing filename before it can be
		// interpreted as a one-based source location.
		name, n, found := terminalLocationSuffix(link)
		if !found {
			return "", 0, 0, false
		}
		line = n
		if base, n, found := terminalLocationSuffix(name); found {
			name, line, column = base, n, line
		}
		path = resolve(name)
		info, err = os.Stat(path)
	}
	ok = err == nil && info.Mode().IsRegular()
	return
}

func terminalLocationSuffix(name string) (string, int, bool) {
	at := strings.LastIndexByte(name, ':')
	if at <= 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(name[at+1:])
	return name[:at], n, err == nil && n > 0
}

func (f *Frontend) pasteTerminalFiles(s *termSession) bool {
	if s.closed || s.t == nil {
		return false
	}
	paths := (host{f}).ClipboardFiles()
	if len(paths) == 0 {
		// The terminal still owns ordinary text and empty/image clipboard paste.
		return false
	}
	s.t.Paste(quotePaths(paths))
	return true
}
