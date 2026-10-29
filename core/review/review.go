// Package review keeps review comments independent of editor and terminal frontends.
// Location tracking and message formatting are adapted from Mayoi (MIT), cmd/mayoi/review.go.
package review

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Note retains the original quote even after its lines move or disappear.
// From and To are zero-based, half-open coordinates on the indicated side.
type Note struct {
	Repository string `json:"repository,omitempty"`

	Path    string `json:"path"`
	Commit  string `json:"commit,omitempty"`
	Staged  bool   `json:"staged,omitempty"`
	OldSide bool   `json:"oldSide,omitempty"`
	// Base distinguishes HEAD from the index for removed worktree lines.
	Base     string   `json:"base,omitempty"`
	From     int      `json:"from"`
	To       int      `json:"to"`
	Diff     bool     `json:"diff,omitempty"`
	Snippet  []string `json:"snippet"`
	Lines    []string `json:"lines"`
	Text     string   `json:"text"`
	Outdated bool     `json:"outdated,omitempty"`
}

func (n Note) Valid() bool {
	return n.Path != "" && n.From >= 0 && n.To > n.From && n.To-n.From == len(n.Lines) && len(n.Snippet) > 0 && strings.TrimSpace(n.Text) != ""
}

func (n Note) SameLocation(o Note) bool {
	return (n.Repository == o.Repository || n.Repository == "" || o.Repository == "") && n.Path == o.Path && n.Commit == o.Commit && n.Staged == o.Staged && n.OldSide == o.OldSide && n.Base == o.Base && n.From == o.From && n.To == o.To
}

// Follow reads lines lazily: a keystroke in a large document must not copy it all.
func (n *Note) Follow(count int, line func(int) string) {
	at, ok := Relocate(n.Lines, count, line, n.From)
	n.Outdated = !ok
	if ok {
		n.From, n.To = at, at+len(n.Lines)
	}
}

func Relocate(lines []string, count int, line func(int) string, from int) (int, bool) {
	matches := func(at int) bool {
		if len(lines) == 0 || at < 0 || at > count-len(lines) {
			return false
		}
		for k, want := range lines {
			if line(at+k) != want {
				return false
			}
		}
		return true
	}
	if matches(from) {
		return from, true
	}
	// Clamp an obsolete line number from persisted state before searching outwards.
	start := max(0, min(from, count))
	for d := 0; d <= max(start, count-start); d++ {
		if matches(start - d) {
			return start - d, true
		}
		if d > 0 && matches(start+d) {
			return start + d, true
		}
	}
	return from, false
}

func (n Note) Ref(root string) string {
	path := n.Path
	if root != "" {
		if rel, err := filepath.Rel(root, path); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			path = rel
		}
	}
	path = filepath.ToSlash(path)
	ref := fmt.Sprintf("%s:%d", path, n.From+1)
	if n.To-n.From > 1 {
		ref += fmt.Sprintf("-%d", n.To)
	}
	var tags []string
	if n.OldSide {
		tags = append(tags, "removed lines, numbered as before the change")
	}
	if n.Commit != "" {
		if from, to, ok := strings.Cut(n.Commit, ".."); ok {
			tags = append(tags, "comparison "+from+" → "+to)
		} else {
			tags = append(tags, "in commit "+n.Commit)
		}
	} else if n.Staged {
		tags = append(tags, "staged")
	}
	if n.Outdated {
		tags = append(tags, "outdated: the lines have changed since")
	}
	if len(tags) > 0 {
		ref += " (" + strings.Join(tags, "; ") + ")"
	}
	return ref
}

func Fence(lines []string) string {
	longest := 2
	for _, line := range lines {
		run := 0
		for _, r := range line {
			if r == '`' {
				run++
				longest = max(longest, run)
			} else {
				run = 0
			}
		}
	}
	return strings.Repeat("`", longest+1)
}

func Format(notes []Note, root string) string {
	var b strings.Builder
	for i, n := range notes {
		if i > 0 {
			b.WriteByte('\n')
		}
		if len(notes) > 1 {
			fmt.Fprintf(&b, "%d. ", i+1)
		}
		b.WriteString(n.Ref(root) + "\n")
		fence := Fence(n.Snippet)
		lang := strings.TrimPrefix(filepath.Ext(n.Path), ".")
		// A path is data, not a Markdown fence language or a terminal control sequence.
		if strings.ContainsAny(lang, "`\r\n\x1b") {
			lang = ""
		}
		if n.Diff {
			lang = "diff"
		}
		b.WriteString(fence + lang + "\n")
		for _, line := range n.Snippet {
			b.WriteString(line + "\n")
		}
		b.WriteString(fence + "\n")
		if text := strings.TrimSpace(n.Text); text != "" {
			b.WriteString(text + "\n")
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}
