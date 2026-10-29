package git

// The repository panel's reads: branches with their upstream and tracking counts, a ref's
// history and the commits between two refs, and a remote's fetch and push URLs.
//
// 移植自 Mayoi（MIT，Copyright (c) 2026 chencheng），internal/git/panels.go 的 PanelBranches、
// LogRef、LogRange、RemoteURLs、ConfiguredRemoteURLs 与参数检查；分支结构改名为 PanelBranch，
// 不动砚已有的 Branch。

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PanelBranch is a branch as the repository panel shows it.
type PanelBranch struct {
	Name    string // "main", or "origin/main" for a remote's
	Remote  bool
	Current bool
	Hash    string
	Subject string
	Time    time.Time
	// Upstream is the branch it tracks, as "origin/main", and UpstreamRef its full ref;
	// Ahead and Behind count the commits each side lacks; Gone says the upstream was deleted.
	Upstream, UpstreamRef string
	Ahead, Behind         int
	Gone                  bool
}

// Ref is the branch's full ref name.
func (b PanelBranch) Ref() string {
	if b.Remote {
		return "refs/remotes/" + b.Name
	}
	return "refs/heads/" + b.Name
}

// PanelBranches returns the local branches and those of the remotes, the one checked out
// first, then the one committed to last.
func (r *Repo) PanelBranches() ([]PanelBranch, error) {
	out, err := r.run(nil, "for-each-ref", "--sort=-committerdate",
		"--format=%(refname)%1f%(committerdate:unix)%1f%(objectname)%1f%(upstream:short)%1f%(upstream:track)%1f%(HEAD)%1f%(symref)%1f%(upstream)%1f%(subject)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	var bs []PanelBranch
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.SplitN(line, "\x1f", 9)
		if len(f) != 9 || f[6] != "" { // origin/HEAD and other symbolic refs
			continue
		}
		b := PanelBranch{Hash: f[2], Upstream: f[3], UpstreamRef: f[7], Current: f[5] == "*", Subject: f[8]}
		switch {
		case strings.HasPrefix(f[0], "refs/heads/"):
			b.Name = strings.TrimPrefix(f[0], "refs/heads/")
		case strings.HasPrefix(f[0], "refs/remotes/"):
			b.Name, b.Remote = strings.TrimPrefix(f[0], "refs/remotes/"), true
		default:
			continue
		}
		if seconds, err := strconv.ParseInt(f[1], 10, 64); err == nil {
			b.Time = time.Unix(seconds, 0)
		}
		b.Gone = f[4] == "[gone]"
		for _, track := range strings.Split(strings.Trim(f[4], "[]"), ", ") {
			direction, count, _ := strings.Cut(track, " ")
			switch direction {
			case "ahead":
				b.Ahead, _ = strconv.Atoi(count)
			case "behind":
				b.Behind, _ = strconv.Atoi(count)
			}
		}
		bs = append(bs, b)
	}
	sort.SliceStable(bs, func(i, j int) bool { return bs[i].Current && !bs[j].Current })
	return bs, nil
}

// PanelRemote is a remote with every fetch and push URL it has.
type PanelRemote struct {
	Name                string
	FetchURLs, PushURLs []string
	URL                 string // its first URL as configured
}

// ConfiguredRemoteURLs returns the first URL configured for each remote, in one read.
func (r *Repo) ConfiguredRemoteURLs() (map[string]string, error) {
	out, err := r.run(nil, "config", "-z", "--get-regexp", `^remote\..*\.url$`)
	if e := (*Error)(nil); errors.As(err, &e) && len(out) == 0 && e.Stderr == "" {
		return map[string]string{}, nil // no remote has a URL: config says nothing and exits 1
	} else if err != nil {
		return nil, err
	}
	urls := map[string]string{}
	for _, rec := range strings.Split(string(out), "\x00") {
		key, url, ok := strings.Cut(rec, "\n")
		if !ok {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(key, "remote."), ".url")
		if _, seen := urls[name]; !seen {
			urls[name] = url
		}
	}
	return urls, nil
}

// RemoteURLs reads a remote's fetch and push URLs, rewriting resolved.
func (r *Repo) RemoteURLs(name string) (PanelRemote, error) {
	if err := r.panelRemote(name); err != nil {
		return PanelRemote{}, err
	}
	fetch, err := r.run(nil, "remote", "get-url", "--all", name)
	if err != nil {
		return PanelRemote{}, err
	}
	push, err := r.run(nil, "remote", "get-url", "--push", "--all", name)
	if err != nil {
		return PanelRemote{}, err
	}
	return PanelRemote{Name: name, FetchURLs: outputLines(fetch), PushURLs: outputLines(push)}, nil
}

func outputLines(out []byte) []string {
	s := strings.TrimSuffix(string(out), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// panelArgument rejects option-like values and line or control separators before they reach
// Git as positional arguments.
func panelArgument(kind, value string) error {
	if value == "" || strings.HasPrefix(value, "-") || strings.ContainsAny(value, "\x00\r\n") {
		return fmt.Errorf("invalid %s %q", kind, value)
	}
	return nil
}

func (r *Repo) panelRemote(name string) error {
	if err := panelArgument("remote", name); err != nil {
		return err
	}
	_, err := r.run(nil, "check-ref-format", "refs/remotes/"+name+"/branch")
	return err
}

// panelCommit resolves one revision before it makes a range, so that no caller's text becomes
// a log option or more refs.
func (r *Repo) panelCommit(ref string) (string, error) {
	if err := panelArgument("revision", ref); err != nil {
		return "", err
	}
	out, err := r.run(nil, "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *Repo) panelLog(revision string, n int) ([]Commit, error) {
	if n <= 0 {
		return nil, fmt.Errorf("commit limit must be positive")
	}
	out, err := r.run(nil, "log", "-z", "--date-order", "--decorate=full", "-n", strconv.Itoa(n), commitFormat, revision, "--")
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, rec := range strings.Split(string(out), "\x00") {
		if c, ok := parseCommit(rec); ok {
			commits = append(commits, c)
		}
	}
	return commits, nil
}

// LogRef returns up to n commits reachable from ref, merges included.
func (r *Repo) LogRef(ref string, n int) ([]Commit, error) {
	hash, err := r.panelCommit(ref)
	if err != nil {
		return nil, err
	}
	return r.panelLog(hash, n)
}

// LogRange returns up to n commits reachable from to but not from from.
func (r *Repo) LogRange(from, to string, n int) ([]Commit, error) {
	a, err := r.panelCommit(from)
	if err != nil {
		return nil, err
	}
	b, err := r.panelCommit(to)
	if err != nil {
		return nil, err
	}
	return r.panelLog(a+".."+b, n)
}
