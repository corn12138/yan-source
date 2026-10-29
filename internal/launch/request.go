package launch

import (
	"os"
	"path/filepath"
	"strings"
)

type Request struct {
	Targets   []Target
	NewWindow bool
}

func Parse(args []string, wd string) Request {
	var r Request
	literal := false
	for _, arg := range args {
		if !literal {
			if arg == "--" {
				literal = true
				continue
			}
			if arg == "--new-window" || arg == "-n" {
				r.NewWindow = true
				continue
			}
			if strings.HasPrefix(arg, "-") {
				continue
			}
		}
		path, line := arg, 0
		if !filepath.IsAbs(path) {
			path = filepath.Join(wd, path)
		}
		// A real filename containing a colon takes precedence over line notation.
		if _, err := os.Stat(path); err != nil {
			path, line = SplitLine(path)
		}
		t := ResolvePath(path)
		t.Line = line
		r.Targets = append(r.Targets, t)
	}
	return r
}
