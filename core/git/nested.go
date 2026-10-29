package git

// Adapted from Mayoi (MIT), internal/git/nested.go at d28bc2500f47.

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const NestedDepth = 2

// Nested finds work trees below dir, stopping at each repository boundary.
// Symlink children, hidden folders and node_modules are excluded so a workspace
// scan cannot escape into another tree or traverse dependency repositories.
// An unreadable directory fails the scan; callers can retain the previous list.
func Nested(ctx context.Context, dir string, depth int) ([]*Repo, error) {
	var repos []*Repo
	level := []string{dir}
	for range depth {
		var next []string
		for _, parent := range level {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			entries, err := os.ReadDir(parent)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Name() == "node_modules" {
					continue
				}
				path := filepath.Join(parent, entry.Name())
				_, err := os.Lstat(filepath.Join(path, ".git"))
				if os.IsNotExist(err) {
					next = append(next, path)
					continue
				}
				if err != nil {
					return nil, err
				}
				// Both ordinary .git directories and linked-worktree .git files count.
				// A broken marker is a boundary too, never a reason to search below it.
				if r, err := Open(path); err == nil && filepath.Clean(r.Root) == filepath.Clean(path) {
					repos = append(repos, r)
				}
			}
		}
		level = next
	}
	slices.SortFunc(repos, func(a, b *Repo) int { return strings.Compare(a.Root, b.Root) })
	return repos, ctx.Err()
}
