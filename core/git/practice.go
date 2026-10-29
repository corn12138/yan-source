package git

import (
	"github.com/corn12138/yan-source/core/i18n"
	"os"
	"path/filepath"
	"strings"
)

// The feature guide's practice repositories: throwaway folders outside the user's projects,
// made afresh each time a lesson starts, so staging, committing and merging there touch
// nothing of the user's. Commits are signed by the guide, not the user, and no hooks run.

var practiceIdentity = []string{
	"-c", "user.name=yan guide", "-c", "user.email=guide@yan.invalid",
	"-c", "commit.gpgsign=false", "-c", "core.hooksPath=/dev/null", "-c", "init.defaultBranch=main",
}

func practice(dir string, args ...string) error {
	_, err := run(dir, nil, append(append([]string{}, practiceIdentity...), args...)...)
	return err
}

// writeFiles writes files (relative path → text) under dir.
func writeFiles(dir string, files map[string]string) error {
	for rel, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// PracticeChanges makes dir a repository whose first commit has base and whose work tree then
// has changed: changed files rewrite or add to base without being staged.
func PracticeChanges(dir string, base, changed map[string]string) error {
	if err := freshRepo(dir, base); err != nil {
		return err
	}
	return writeFiles(dir, changed)
}

// PracticeConflict makes dir a repository in the middle of a merge whose file path conflicts:
// base on main, ours committed on main after it, theirs on the branch "feature" being merged.
func PracticeConflict(dir, path, base, ours, theirs string) error {
	if err := freshRepo(dir, map[string]string{path: base}); err != nil {
		return err
	}
	steps := []func() error{
		func() error { return practice(dir, "checkout", "-q", "-b", "feature") },
		func() error { return writeFiles(dir, map[string]string{path: theirs}) },
		func() error { return practice(dir, "commit", "-qam", "Changes on feature") },
		func() error { return practice(dir, "checkout", "-q", "main") },
		func() error { return writeFiles(dir, map[string]string{path: ours}) },
		func() error { return practice(dir, "commit", "-qam", "Changes on main") },
	}
	for _, s := range steps {
		if err := s(); err != nil {
			return err
		}
	}
	if err := practice(dir, "merge", "-q", "--no-edit", "feature"); err == nil {
		return &Error{Args: []string{"merge"}, Stderr: i18n.T("The practice repository did not produce a conflict")}
	}
	return nil // the merge stops for the conflict, as it should
}

func freshRepo(dir string, files map[string]string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := practice(dir, "init", "-q"); err != nil {
		return err
	}
	// The user's own commits in the lesson need no identity or signing key of theirs either.
	for i := 0; i+1 < len(practiceIdentity); i += 2 {
		k, v, _ := strings.Cut(practiceIdentity[i+1], "=")
		if err := practice(dir, "config", k, v); err != nil {
			return err
		}
	}
	if err := writeFiles(dir, files); err != nil {
		return err
	}
	if err := practice(dir, "add", "-A"); err != nil {
		return err
	}
	return practice(dir, "commit", "-qm", "Practice baseline")
}
