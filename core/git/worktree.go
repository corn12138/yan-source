package git

// 移植自 Mayoi v0.7.0（MIT，Copyright (c) 2026 chencheng），internal/git/worktree.go：
// 切到已经检出在别的 worktree 里的分支时，找出那个 worktree；它的文件夹没了时清理掉。

import (
	"fmt"
	"strings"
)

// WorktreeInUse reports a branch checked out in another worktree.
type WorktreeInUse struct{ Branch, Path string }

func (e *WorktreeInUse) Error() string {
	return fmt.Sprintf("branch %q is checked out in %s", e.Branch, e.Path)
}

// WorktreeForBranch returns the checkout owning a local branch, in the same
// filesystem namespace as the repository. NUL records preserve unusual paths.
func (r *Repo) WorktreeForBranch(branch string) (string, error) {
	out, err := r.run(nil, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return "", err
	}
	return worktreeForBranch(r.Root, string(out), branch), nil
}

func worktreeForBranch(root, out, branch string) string {
	var path string
	for _, field := range strings.Split(out, "\x00") {
		switch {
		case field == "":
			path = ""
		case strings.HasPrefix(field, "worktree "):
			path = strings.TrimPrefix(field, "worktree ")
		case field == "branch refs/heads/"+branch && path != "":
			return path
		}
	}
	return ""
}

// RemoveWorktree forgets the worktree at path, which keeps its branch
// checked out until it does. Unlike git worktree prune it leaves the other
// worktrees whose folders are missing alone: one on a drive that is not
// mounted is still in use. A locked worktree is not removed.
func (r *Repo) RemoveWorktree(path string) error {
	_, err := r.run(nil, "worktree", "remove", path)
	return err
}
