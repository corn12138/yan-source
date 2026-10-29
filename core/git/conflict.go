package git

// Reading and restoring a conflicted file's versions, for the three-way merge view (after
// the behaviour of Mayoi PR #50 by Garfielk, unmerged; rewritten).

import (
	"os"
	"path/filepath"
	"strings"
)

// ConflictStage is the text of a conflicted path's stage n: 1 the common ancestor, 2 ours, 3 theirs.
// ok is false when that stage has no such file (added on one side only, or deleted).
func (r *Repo) ConflictStage(n int, path string) (text string, ok bool, err error) {
	return r.Show(":"+string(rune('0'+n)), path)
}

// RestoreConflict puts a path back in conflict, its markers and stages as the stopped command
// left them (git checkout -m).
func (r *Repo) RestoreConflict(path string) error {
	_, err := r.run(nil, "checkout", "-m", "--", r.Rel(path))
	return err
}

// OpHeadName is what names the commit the stopped command brings in: the branch or tag it is
// on, or its short hash; "" when nothing is stopped.
func (r *Repo) OpHeadName(op Op) string {
	head := map[Op]string{OpMerge: "MERGE_HEAD", OpCherryPick: "CHERRY_PICK_HEAD", OpRevert: "REVERT_HEAD", OpRebase: "REBASE_HEAD"}[op]
	if head == "" {
		return ""
	}
	if _, err := os.Stat(filepath.Join(r.Dir, head)); err != nil {
		return ""
	}
	if out, err := r.run(nil, "name-rev", "--name-only", "--no-undefined", head); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return strings.TrimPrefix(name, "remotes/")
		}
	}
	if out, err := r.run(nil, "rev-parse", "--short", head); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}
