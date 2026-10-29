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
	literal, skip := false, false
	for _, arg := range args {
		if skip {
			skip = false
			continue
		}
		if !literal {
			if arg == "--" {
				literal = true
				continue
			}
			if arg == "--new-window" || arg == "-n" {
				r.NewWindow = true
				continue
			}
			if appleDefault(arg) {
				skip = true // its value: "-AppleLanguages (en)" sets the languages for this launch
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

// appleDefault reports whether arg sets a macOS user default for this launch, "-AppleLanguages"
// or "-NSDocumentRevisionsDebugMode" followed by its value, which Foundation reads by itself.
func appleDefault(arg string) bool {
	return len(arg) > 3 && (strings.HasPrefix(arg, "-Apple") || strings.HasPrefix(arg, "-NS")) && !strings.Contains(arg, "=")
}
