package git

import (
	"errors"
	"strings"
)

// Branch and remote management (GI-27), as Mayoi's Git menus offer it: rename and delete
// branches, add, remove and re-point remotes, and prune remote branches gone upstream.

// ErrNotMerged is a branch delete git refused because the branch has commits no other branch
// has: deleting it would lose them. DeleteBranch with force deletes it anyway.
var ErrNotMerged = errors.New("分支还有没合并的提交")

// RenameBranch renames a local branch; git refuses a name already taken.
func (r *Repo) RenameBranch(old, name string) error {
	_, err := r.run(nil, "branch", "-m", old, name)
	return err
}

// DeleteBranch deletes a local branch. Without force, a branch not merged into its upstream or
// HEAD is kept and ErrNotMerged returned.
func (r *Repo) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.run(nil, "branch", flag, name)
	if err != nil && !force && strings.Contains(err.Error(), "not fully merged") {
		return ErrNotMerged
	}
	return err
}

// Remote is a remote and the URL it fetches from.
type Remote struct {
	Name, URL string
}

// RemoteList returns the remotes with their fetch URLs.
func (r *Repo) RemoteList() ([]Remote, error) {
	out, err := r.run(nil, "remote", "-v")
	if err != nil {
		return nil, err
	}
	var rs []Remote
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) == 3 && f[2] == "(fetch)" {
			rs = append(rs, Remote{Name: f[0], URL: f[1]})
		}
	}
	return rs, nil
}

// AddRemote adds a remote.
func (r *Repo) AddRemote(name, url string) error {
	_, err := r.run(nil, "remote", "add", name, url)
	return err
}

// RemoveRemote removes a remote and its remote-tracking branches.
func (r *Repo) RemoveRemote(name string) error {
	_, err := r.run(nil, "remote", "remove", name)
	return err
}

// SetRemoteURL points a remote at another URL.
func (r *Repo) SetRemoteURL(name, url string) error {
	_, err := r.run(nil, "remote", "set-url", name, url)
	return err
}

// PruneRemote deletes the remote-tracking branches of a remote whose branches are gone.
func (r *Repo) PruneRemote(name string) error {
	_, err := r.run(nil, "remote", "prune", name)
	return err
}

// Sync fast-forwards the branch checked out from its upstream, then pushes its own commits to
// that same branch, whatever the push defaults say.
//
// 移植自 Mayoi（MIT，Copyright (c) 2026 chencheng），#53（yushengyu、yunqian）的 internal/git Sync。
func (r *Repo) Sync() error {
	out, err := r.run(nil, "for-each-ref", "--format=%(HEAD)%1f%(refname)%1f%(upstream:remotename)%1f%(upstream:remoteref)", "refs/heads")
	if err != nil {
		return err
	}
	var branch, remote, upstream string
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(line, "\x1f")
		if len(f) == 4 && f[0] == "*" {
			branch, remote, upstream = f[1], f[2], f[3]
			break
		}
	}
	if branch == "" {
		return errors.New("先检出一个有提交的分支，再同步")
	}
	if remote == "" || upstream == "" {
		return errors.New("当前分支没有上游，不能同步")
	}
	if _, err := r.run(nil, "pull", "--ff-only", "--", remote, upstream); err != nil {
		return err
	}
	// FETCH_HEAD has the remote's commits that came while syncing.
	out, err = r.run(nil, "rev-list", "--count", "FETCH_HEAD.."+branch)
	if err != nil || strings.TrimSpace(string(out)) == "0" {
		return err
	}
	_, err = r.run(nil, "push", "--", remote, branch+":"+upstream)
	return err
}
