package review

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFollowAndOriginalQuote(t *testing.T) {
	n := Note{From: 1, To: 3, Lines: []string{"目标", "return 1"}, Snippet: []string{"-return 0", "+return 1"}}
	lines := []string{"new", "header", "目标", "return 1", "tail"}
	n.Follow(len(lines), func(i int) string { return lines[i] })
	if n.From != 2 || n.To != 4 || n.Outdated {
		t.Fatalf("moved %+v", n)
	}
	lines[3] = "return 2"
	n.Follow(len(lines), func(i int) string { return lines[i] })
	if !n.Outdated || n.Snippet[1] != "+return 1" {
		t.Fatalf("lost original quote %+v", n)
	}
	lines[3] = "return 1"
	n.Follow(len(lines), func(i int) string { return lines[i] })
	if n.Outdated {
		t.Fatal("restored quote remains outdated")
	}
	n.From = 1000000000
	n.Follow(len(lines), func(i int) string { return lines[i] })
	if n.From != 2 {
		t.Fatal("obsolete persisted coordinates")
	}
}

func TestFormatSideRevisionAndFence(t *testing.T) {
	root := t.TempDir()
	ns := []Note{{Path: filepath.Join(root, "dir", "test.md"), From: 2, To: 4, Diff: true, OldSide: true, Staged: true, Outdated: true, Snippet: []string{"-```go", "-value"}, Text: "Fix this\nwith a test"}, {Path: filepath.Join(root, "b.go"), Commit: "abc123", From: 0, To: 1, Snippet: []string{"x"}, Text: "Review"}}
	got := Format(ns, root)
	for _, want := range []string{"1. dir/test.md:3-4 (removed lines", "staged; outdated", "````diff\n-```go", "Fix this\nwith a test", "2. b.go:1 (in commit abc123)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.HasSuffix(got, "\n") {
		t.Fatal("must not append a submit newline")
	}
}

func TestComparisonReferenceNamesBothEnds(t *testing.T) {
	n := Note{Path: "file.go", Commit: "abc..def", From: 0, To: 1}
	if got := n.Ref(""); !strings.Contains(got, "comparison abc → def") {
		t.Fatal(got)
	}
}
