package app

// The repository panel at the bottom of Source Control, after Mayoi's (MIT, cmd/mayoi/
// scmpanels.go): Branches lists the local branches, Remotes the remotes with their URLs and
// branches. Everything is read only when shown: the panel's refs when it opens, a branch's
// history and its incoming and outgoing commits when the branch opens, a page at a time.
// Whether the panel is open and which tab it shows are kept for the next start.

import (
	"fmt"
	"strings"
	"time"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// repoPanel is a repository's panel: what was read, for which state of its refs.
type repoPanel struct {
	key                     string // the refs it was read for
	request                 *int   // the read in flight; a result for another is dropped
	loading, loaded         bool
	remotesLoaded           bool
	err                     string
	branches                []git.PanelBranch
	remotes                 []git.PanelRemote
	open                    map[string]bool // rows opened (all start closed)
	logs                    map[string]*panelLog
	readRemotes, wasRemotes bool
}

// panelLog is a run of commits a row shows: a ref's history, or the commits from one ref to
// another.
type panelLog struct {
	from, to        string
	n               int
	commits         []git.Commit
	loading, loaded bool
	err             string
	request         *int
}

// repoPanelOpen and repoPanelRemotes say whether the panel is open and shows the remotes.
func (e *Editor) repoPanelOpen() bool    { return e.repoPanel }
func (e *Editor) repoPanelRemotes() bool { return e.repoRemotes }

// toggleRepoPanel opens or closes the repository panel.
func (e *Editor) toggleRepoPanel() error {
	e.repoPanel = !e.repoPanel
	e.keepRepoPanel()
	return nil
}

// SCMPanelTab shows the panel's branches or its remotes.
func (e *Editor) SCMPanelTab(remotes bool) {
	e.repoPanel, e.repoRemotes = true, remotes
	e.keepRepoPanel()
}

// keepRepoPanel keeps the panel's state for the next start.
func (e *Editor) keepRepoPanel() {
	if e.state != nil {
		e.state.RepositoryPanel, e.state.RepositoryRemotes = e.repoPanel, e.repoRemotes
		e.saveState()
	}
}

// refreshRepoPanel reads the panel's refs again when they changed, while it is open.
func (e *Editor) refreshRepoPanel() {
	g := e.repository().git
	if g.repo == nil || g.status == nil || !e.repoPanelOpen() {
		return
	}
	p := &e.repository().scm.panel
	remotes := e.repoPanelRemotes()
	key := fmt.Sprintf("%s|%s|%d|%d|%s|%v|%d", g.status.Head, g.status.Branch, g.status.Ahead, g.status.Behind, g.status.Upstream, remotes, e.repository().scm.history.gen)
	if p.key == key {
		return
	}
	p.key, p.loading, p.err = key, true, ""
	if p.open == nil {
		p.open, p.logs = map[string]bool{}, map[string]*panelLog{}
	}
	for _, l := range p.logs {
		l.request = nil // read each open log again, keeping what it shows until then
	}
	req := new(int)
	p.request = req
	openRemotes := map[string]bool{}
	for k, o := range p.open {
		if name, ok := strings.CutPrefix(k, "remote:"); ok && o {
			openRemotes[name] = true
		}
	}
	r := g.repo
	post := e.postGit()
	go func() {
		branches, err := r.PanelBranches()
		var rs []git.PanelRemote
		if err == nil && remotes {
			var names []string
			var urls map[string]string
			if names, err = r.Remotes(); err == nil {
				urls, err = r.ConfiguredRemoteURLs()
			}
			for _, name := range names {
				rm := git.PanelRemote{Name: name}
				if openRemotes[name] && err == nil {
					rm, err = r.RemoteURLs(name)
				}
				rm.URL = urls[name]
				rs = append(rs, rm)
			}
		}
		post(func(e *Editor) {
			p := &e.repository().scm.panel
			if p.request != req {
				return
			}
			p.loading = false
			if err != nil {
				p.err = err.Error()
				return
			}
			p.branches, p.loaded = branches, true
			if remotes {
				p.remotes, p.remotesLoaded = rs, true
			}
		})
	}()
}

// panelLogOf is the log of a row, read in the background the first time it is shown (and
// after the refs changed, or a further page was asked for).
func (e *Editor) panelLogOf(key, from, to string) *panelLog {
	p := &e.repository().scm.panel
	l := p.logs[key]
	if l == nil {
		l = &panelLog{from: from, to: to, n: historyPage}
		p.logs[key] = l
	}
	if p.loading || l.loading || l.request == p.request && (l.loaded || l.err != "") {
		return l
	}
	l.loading, l.request = true, p.request
	r, req, n := e.repository().git.repo, p.request, l.n
	post := e.postGit()
	go func() {
		var commits []git.Commit
		var err error
		if from == "" {
			commits, err = r.LogRef(to, n)
		} else {
			commits, err = r.LogRange(from, to, n)
		}
		post(func(e *Editor) {
			p := &e.repository().scm.panel
			if p.request != req || p.logs[key] != l {
				return
			}
			l.loading = false
			if l.n != n {
				l.request = nil // a further page was asked for meanwhile
				return
			}
			if err != nil {
				l.err = err.Error()
				return
			}
			l.commits, l.loaded, l.err = commits, true, ""
		})
	}()
	return l
}

// repoPanelRows are the panel's rows, as Source Control shows them under its history.
func (e *Editor) repoPanelRows() []view.SCMPanelRow {
	e.refreshRepoPanel()
	p := &e.repository().scm.panel
	if !e.repoPanelOpen() || e.repository().git.repo == nil {
		return nil
	}
	var rows []view.SCMPanelRow
	add := func(r view.SCMPanelRow) { rows = append(rows, r) }
	addLog := func(key, from, to string, depth int) {
		l := e.panelLogOf(key, from, to)
		switch {
		case l.loading && !l.loaded:
			add(view.SCMPanelRow{Key: key + ":loading", Kind: "status", Label: i18n.T("Reading commits…"), Depth: depth})
			return
		case l.err != "":
			add(view.SCMPanelRow{Key: key, Kind: "retryLog", Label: i18n.T("Couldn't read commits: click to retry"), Detail: l.err, Depth: depth})
			return
		case len(l.commits) == 0:
			add(view.SCMPanelRow{Key: key + ":empty", Kind: "status", Label: i18n.T("No commits"), Depth: depth})
		}
		for _, c := range l.commits {
			add(view.SCMPanelRow{Key: key + ":" + c.Hash, Kind: "commit", Label: c.Subject, Detail: git.ShortHash(c.Hash) + " · " + c.Author + " · " + relTime(c.Time, time.Now()), Hash: c.Hash, Depth: depth})
		}
		if len(l.commits) >= l.n {
			add(view.SCMPanelRow{Key: key, Kind: "more", Label: i18n.T("More Commits…"), Depth: depth})
		}
	}
	addBranch := func(b git.PanelBranch, depth int) {
		key := "branch:" + b.Ref()
		detail := b.Upstream
		switch {
		case b.Gone:
			detail += i18n.T("(deleted on remote)")
		case b.Upstream != "":
			if b.Behind > 0 {
				detail += fmt.Sprintf(" ↓%d", b.Behind)
			}
			if b.Ahead > 0 {
				detail += fmt.Sprintf(" ↑%d", b.Ahead)
			}
		}
		if !b.Time.IsZero() {
			detail += " · " + relTime(b.Time, time.Now())
		}
		open := p.open[key]
		add(view.SCMPanelRow{Key: key, Kind: "branch", Label: b.Name, Detail: strings.TrimPrefix(detail, " · "), Depth: depth, Open: open, Current: b.Current, Remote: b.Remote})
		if !open {
			return
		}
		if b.Upstream != "" && !b.Gone {
			for _, in := range []bool{true, false} {
				label, n, from, to := i18n.T("To Push"), b.Ahead, b.UpstreamRef, b.Ref()
				if in {
					label, n, from, to = i18n.T("To Pull"), b.Behind, b.Ref(), b.UpstreamRef
				}
				if n == 0 {
					continue
				}
				k := key + ":" + map[bool]string{true: "in", false: "out"}[in]
				add(view.SCMPanelRow{Key: k, Kind: "range", Label: i18n.F("%s (%d commits)", label, n), Depth: depth + 1, Open: p.open[k]})
				if p.open[k] {
					addLog(k, from, to, depth+2)
				}
			}
		}
		addLog(key+":history", "", b.Ref(), depth+1)
	}
	if !p.loaded && p.err == "" || e.repoPanelRemotes() && !p.remotesLoaded && p.err == "" {
		return append(rows, view.SCMPanelRow{Key: "status", Kind: "status", Label: i18n.T("Reading…"), Depth: 1})
	}
	if p.err != "" {
		return append(rows, view.SCMPanelRow{Key: "retry", Kind: "retry", Label: i18n.T("Couldn't read branches: click to retry"), Detail: p.err, Depth: 1})
	}
	if !e.repoPanelRemotes() {
		for _, b := range p.branches {
			if !b.Remote {
				addBranch(b, 1)
			}
		}
		if len(rows) == 0 {
			add(view.SCMPanelRow{Key: "none", Kind: "status", Label: i18n.T("No branches yet"), Depth: 1})
		}
		return rows
	}
	for _, rm := range p.remotes {
		key := "remote:" + rm.Name
		open := p.open[key]
		detail := ""
		if !open && rm.URL != "" {
			detail = shortRemoteURL(rm.URL)
		}
		add(view.SCMPanelRow{Key: key, Kind: "remote", Label: rm.Name, Detail: detail, Depth: 1, Open: open})
		if !open {
			continue
		}
		same := len(rm.FetchURLs) == len(rm.PushURLs)
		for i := range rm.FetchURLs {
			same = same && rm.FetchURLs[i] == rm.PushURLs[i]
		}
		kind := i18n.T("Pull")
		if same {
			kind = i18n.T("Pull, Push")
		}
		for _, u := range rm.FetchURLs {
			add(view.SCMPanelRow{Key: key + ":fetch:" + u, Kind: "url", Label: u, Detail: kind, Depth: 2})
		}
		for _, u := range rm.PushURLs {
			if same {
				break
			}
			add(view.SCMPanelRow{Key: key + ":push:" + u, Kind: "url", Label: u, Detail: i18n.T("Push"), Depth: 2})
		}
		for _, b := range p.branches {
			if b.Remote && strings.HasPrefix(b.Name, rm.Name+"/") {
				addBranch(b, 2)
			}
		}
	}
	if len(p.remotes) == 0 {
		add(view.SCMPanelRow{Key: "addRemote", Kind: "addRemote", Label: i18n.T("No remotes: click to add one"), Depth: 1})
	}
	return rows
}

// SCMPanelActivate is a click on a row of the panel: a branch, remote or range opens or closes,
// a commit opens in a tab, and the rest do what they say.
func (e *Editor) SCMPanelActivate(row view.SCMPanelRow) {
	e.sync()
	defer e.sync()
	p := &e.repository().scm.panel
	var err error
	switch row.Kind {
	case "branch", "range", "remote":
		p.open[row.Key] = !p.open[row.Key]
		if row.Kind == "remote" {
			p.key = "" // its URLs are read when it opens
		}
	case "commit":
		e.showHistoryTab("", row.Hash, true)
	case "more":
		if l := p.logs[row.Key]; l != nil {
			l.n += historyPage
			l.request = nil
		}
	case "retryLog":
		if l := p.logs[row.Key]; l != nil {
			l.request, l.err = nil, ""
		}
	case "retry":
		p.key = ""
	case "addRemote":
		err = e.addRemote()
	}
	if err != nil {
		e.status = err.Error()
	}
}

// SCMPanelBranch runs a branch row's menu: "checkout", "delete" or "compare" (with HEAD).
func (e *Editor) SCMPanelBranch(name string, remote bool, action string) {
	var err error
	switch action {
	case "checkout":
		if remote { // 远程的分支：检出同名的本地分支，git 会让它跟踪远程的
			if _, local, ok := strings.Cut(name, "/"); ok {
				name = local
			}
		}
		err = e.switchBranch(name, false)
	case "delete":
		err = e.deleteBranchNamed(name, false)
	case "compare":
		err = e.compareRevisions("HEAD", name)
	}
	if err != nil {
		e.status = err.Error()
	}
}

// SCMPanelRemote runs a remote row's menu: "url", "prune" or "remove".
func (e *Editor) SCMPanelRemote(name, action string) {
	rm := git.Remote{Name: name}
	for _, r := range e.repository().scm.panel.remotes {
		if r.Name == name {
			rm.URL = r.URL
		}
	}
	var err error
	switch action {
	case "url":
		err = e.setRemoteURLOf(rm)
	case "prune":
		err = e.pruneRemoteNamed(name)
	case "remove":
		err = e.removeRemoteNamed(rm)
	}
	if err != nil {
		e.status = err.Error()
	}
}

// shortRemoteURL is where a remote's URL points, as host and path:
// git@github.com:me/app.git and https://github.com/me/app both read github.com/me/app.
func shortRemoteURL(url string) string {
	s := url
	if _, rest, ok := strings.Cut(s, "://"); ok {
		s = rest
	} else if user, rest, ok := strings.Cut(s, "@"); ok && !strings.Contains(user, "/") {
		s = strings.Replace(rest, ":", "/", 1)
	}
	if i := strings.Index(s, "@"); i >= 0 && i < strings.Index(s+"/", "/") {
		s = s[i+1:]
	}
	return strings.TrimSuffix(strings.TrimSuffix(s, "/"), ".git")
}
