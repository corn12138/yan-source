package app

import (
	"context"
	"path/filepath"
	"slices"
	"time"

	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/view"
)

// Contexts are keyed by worktree root, not Common: linked worktrees share objects
// but have independent indexes, branches and drafts. Retired contexts never revive,
// so a delayed menu or dialog cannot operate on a replacement at the same path.
type repositoryContext struct {
	id              uint64
	git             *gitState
	scm             *scmState
	dead, collapsed bool
}
type repositoryState struct {
	root     *repositoryContext
	contexts []*repositoryContext
	selected string
	nextID   uint64
	cancel   context.CancelFunc
	kick     chan struct{}
}

func (e *Editor) rootRepository() *repositoryContext {
	if e.repositories.root == nil {
		e.repositories.nextID++
		e.repositories.root = &repositoryContext{id: e.repositories.nextID, git: &e.git, scm: &e.scm}
	}
	return e.repositories.root
}
func (e *Editor) repositoryContexts() []*repositoryContext {
	if len(e.repositories.contexts) > 0 {
		return e.repositories.contexts
	}
	return []*repositoryContext{e.rootRepository()}
}
func (e *Editor) repositoryForRoot(root string) *repositoryContext {
	for _, rc := range e.repositoryContexts() {
		if !rc.dead && rc.git.repo != nil && rc.git.repo.Root == root {
			return rc
		}
	}
	return nil
}
func (e *Editor) repositoryForPath(path string) *repositoryContext {
	var found *repositoryContext
	for _, rc := range e.repositoryContexts() {
		if !rc.dead && rc.git.repo != nil && rc.git.repo.Contains(path) && (found == nil || len(rc.git.repo.Root) > len(found.git.repo.Root)) {
			found = rc
		}
	}
	return found
}
func (e *Editor) repository() *repositoryContext {
	if e.repoScope != nil {
		return e.repoScope
	}
	if rc := e.repositoryForRoot(e.repositories.selected); rc != nil {
		return rc
	}
	if t := e.cur(); t != nil {
		if t.diff != nil && t.diff.repoRoot != "" {
			if rc := e.repositoryForRoot(t.diff.repoRoot); rc != nil {
				return rc
			}
			return &repositoryContext{dead: true, git: &gitState{ready: e.git.ready}, scm: &scmState{}}
		} else if rc := e.repositoryForPath(t.doc.buf.Path()); rc != nil {
			return rc
		}
	}
	for _, rc := range e.repositoryContexts() {
		if !rc.dead && rc.git.repo != nil {
			return rc
		}
	}
	return e.rootRepository()
}
func (e *Editor) bindRepository(rc *repositoryContext) func() {
	old := e.repoScope
	if rc == nil {
		rc = &repositoryContext{dead: true, git: &gitState{ready: e.git.ready}, scm: &scmState{}}
	}
	e.repoScope = rc
	return func() { e.repoScope = old }
}
func (e *Editor) inRepository(rc *repositoryContext, fn func()) { defer e.bindRepository(rc)(); fn() }
func (e *Editor) runRepositoryItem(rc *repositoryContext, fn func(*Editor) error) error {
	if rc == nil {
		return fn(e)
	}
	if rc.dead {
		return userError(i18n.T("This repository is no longer available; select again"))
	}
	defer e.bindRepository(rc)()
	return fn(e)
}

// WithRepository is the frontend action boundary. The opaque ID comes from the
// rendered frame; stale clicks are rejected even if the path has been reused.
func (e *Editor) WithRepository(id uint64, fn func(*Editor)) {
	e.InRepository(id, func(ed *Editor) {
		if r := ed.repository().git.repo; r != nil {
			ed.repositories.selected = r.Root
		}
		fn(ed)
	})
}

// InRepository reads frame-bound state without changing the user's target choice.
func (e *Editor) InRepository(id uint64, fn func(*Editor)) {
	for _, rc := range e.repositoryContexts() {
		if rc.id != id || rc.dead {
			continue
		}
		e.inRepository(rc, func() { fn(e) })
		return
	}
	e.status = i18n.T("The repository list changed; select again")
}
func (e *Editor) postGit() func(func(*Editor)) {
	rc, epoch := e.repository(), e.wsEpoch
	return func(fn func(*Editor)) {
		e.Post(func(e *Editor) {
			if epoch == e.wsEpoch && !rc.dead {
				e.inRepository(rc, func() { fn(e) })
			}
		})
	}
}
func (e *Editor) gitChoose(title string, items ...ovItem) {
	rc := e.repository()
	if rc.git.repo != nil {
		title += " · " + e.repositoryName(rc.git.repo.Root)
	}
	e.choose(title, items...)
	e.ov.repository = rc
}
func (e *Editor) gitPrompt(title, initial string, hint func(string) string, accept func(*Editor, string) error) {
	rc := e.repository()
	if rc.git.repo != nil {
		title += " · " + e.repositoryName(rc.git.repo.Root)
	}
	e.prompt(title, initial, hint, accept)
	e.ov.repository = rc
}
func (e *Editor) repositoryName(root string) string {
	if rel, err := filepath.Rel(e.root, root); err == nil && rel != "." {
		return filepath.ToSlash(rel)
	}
	return filepath.Base(root)
}
func (e *Editor) chooseRepository() error {
	items := []ovItem{{label: i18n.T("Follow the Current File Automatically"), run: func(e *Editor) error { e.repositories.selected = ""; return nil }}}
	for _, rc := range e.repositoryContexts() {
		if rc.git.repo == nil {
			continue
		}
		root := rc.git.repo.Root
		items = append(items, ovItem{label: e.repositoryName(root), detail: root, run: func(e *Editor) error {
			if rc.dead {
				return userError(i18n.T("This repository is no longer available"))
			}
			e.repositories.selected = root
			return nil
		}})
	}
	e.choose(i18n.T("Select Repository"), items...)
	return nil
}
func (e *Editor) ToggleRepository(id uint64) {
	e.WithRepository(id, func(e *Editor) { rc := e.repository(); rc.collapsed = !rc.collapsed })
}
func (e *Editor) scmFrame(dst *view.SCM) {
	active := e.repository()
	e.scmOneFrame(dst)
	dst.Root, dst.RepositoryID, dst.AutoRepository = "", active.id, e.repositories.selected == ""
	if active.git.repo != nil {
		dst.Root = active.git.repo.Root
	}
	dst.RepositoryError = e.git.nestedError
	for _, rc := range e.repositoryContexts() {
		if rc.git.repo == nil || rc.dead {
			continue
		}
		var panel view.SCM
		e.inRepository(rc, func() { e.scmOneFrame(&panel) })
		panel.Root, panel.RepositoryID = rc.git.repo.Root, rc.id
		panel.Collapsed = rc.collapsed
		if rc != active {
			panel.Selected = -1
			panel.Commits = nil
			panel.MoreCommits = false
			panel.HistoryLoading = false
			panel.MessageFocused = false
		}
		dst.Panels = append(dst.Panels, view.SCMPanel{Name: e.repositoryName(panel.Root), State: panel})
	}
}
func (e *Editor) closeRepositories() {
	e.rootRepository().dead = true
	if e.repositories.cancel != nil {
		e.repositories.cancel()
	}
	for _, rc := range e.repositoryContexts() {
		rc.dead = true
		if rc.git.cancel != nil {
			rc.git.cancel()
			rc.git.cancel = nil
		}
		if rc.git.blame.timer != nil {
			rc.git.blame.timer.Stop()
		}
	}
}
func (e *Editor) startRepositories(root string) {
	e.closeRepositories()
	next := e.repositories.nextID
	e.git = gitState{}
	e.scm = scmState{}
	e.repositories = repositoryState{nextID: next, kick: make(chan struct{}, 1)}
	rc := e.rootRepository()
	ctx, cancel := context.WithCancel(context.Background())
	e.repositories.cancel = cancel
	kick := e.repositories.kick
	go func() {
		for {
			r, err := git.Open(root)
			var found []*git.Repo
			if err == nil {
				found = []*git.Repo{r}
			} else {
				found, err = git.Nested(ctx, root, git.NestedDepth)
			}
			if err == nil {
				seen := map[string]bool{}
				for i := 0; i < len(found); i++ {
					if ctx.Err() != nil {
						return
					}
					repo := found[i]
					seen[repo.Root] = true
					subs, subErr := repo.Submodules()
					if subErr != nil {
						err = subErr
						break
					}
					for _, sub := range subs {
						if !seen[sub.Root] {
							seen[sub.Root] = true
							found = append(found, sub)
						}
					}
				}
			}
			if ctx.Err() != nil {
				return
			}
			e.Post(func(e *Editor) {
				if ctx.Err() != nil || e.repositories.root != rc {
					return
				}
				e.git.ready = true
				if err != nil {
					e.git.nestedError = i18n.F("Couldn't read repositories: %s", err.Error())
					return
				}
				e.git.nestedError = ""
				e.reconcileRepositories(ctx, r, found)
			})
			// Discovery runs again when the folder's watcher sees a .git or .gitmodules come or go,
			// after a git command and when the window comes back (kick); the timer is only a
			// fallback for folders the watcher does not reach.
			timer := time.NewTimer(repoDiscoverEvery)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-kick:
				timer.Stop()
			case <-timer.C:
			}
		}
	}()
}
func (e *Editor) reconcileRepositories(ctx context.Context, root *git.Repo, found []*git.Repo) {
	old := e.repositories.contexts
	var next []*repositoryContext
	var paths []string
	for _, r := range found {
		rc := e.repositoryForRoot(r.Root)
		if rc == nil {
			if root != nil && r.Root == root.Root && e.rootRepository().git.repo == nil {
				rc = e.rootRepository()
			} else {
				e.repositories.nextID++
				rc = &repositoryContext{id: e.repositories.nextID, git: &gitState{}, scm: &scmState{}}
			}
			rc.git.repo, rc.git.ready = r, true
			rc.git.kick = make(chan struct{}, 1)
			watch, cancel := context.WithCancel(ctx)
			rc.git.cancel = cancel
			go watchGit(watch, e, rc, r, rc.git.kick)
		}
		next = append(next, rc)
		if root == nil {
			paths = append(paths, r.Root)
		}
	}
	for _, rc := range old {
		if !slices.Contains(next, rc) {
			for _, d := range e.docs {
				if e.repositoryForPath(d.buf.Path()) == rc {
					d.marks = gitMarks{}
				}
			}
			rc.dead = true
			for _, c := range e.cols {
				for _, t := range c.tabs {
					if dt := t.diff; dt != nil && dt.repoRoot == rc.git.repo.Root {
						dt.loading, dt.awaitRepo = false, false
						dt.loadError = i18n.T("The original repository is no longer available")
						if dt.changes {
							e.buildChanges(t.doc, dt)
						}
					}
				}
			}
			if rc.git.cancel != nil {
				rc.git.cancel()
			}
		}
	}
	if e.repositories.root.dead {
		e.git = gitState{ready: true}
		e.scm = scmState{}
		e.repositories.root = nil
		e.rootRepository()
	}
	e.repositories.contexts = next
	e.git.nested = paths
	if e.repositoryForRoot(e.repositories.selected) == nil {
		e.repositories.selected = ""
	}
	for _, c := range e.cols {
		for _, t := range c.tabs {
			if t.diff != nil && t.diff.commit != nil && (t.diff.awaitRepo || t.diff.loadError != "" && e.repositoryForRoot(t.diff.repoRoot) != nil) {
				t.diff.awaitRepo, t.diff.loading = false, false
				e.loadCommit(t)
			}
		}
	}
}

// A historical tab stays attached to its original root even while another repo
// is selected. Missing roots produce an unavailable state, never a fallback write.
func (e *Editor) bindDiff(dt *diffTab) func() {
	if dt == nil || dt.repoRoot == "" {
		return func() {}
	}
	rc := e.repositoryForRoot(dt.repoRoot)
	if rc == nil {
		rc = &repositoryContext{dead: true, git: &gitState{ready: e.git.ready}, scm: &scmState{}}
	}
	return e.bindRepository(rc)
}

func (e *Editor) moveRepositoryRow(by int) {
	rc := e.repository()
	count := len(scmFlat(e.scmGroups())) + len(e.shownCommits())
	next := rc.scm.sel + by
	if len(e.repositories.contexts) > 1 && (next < 0 || next >= count) {
		i := slices.Index(e.repositories.contexts, rc)
		if by < 0 {
			i--
		} else {
			i++
		}
		if i >= 0 && i < len(e.repositories.contexts) {
			rc = e.repositories.contexts[i]
			e.repositories.selected = rc.git.repo.Root
			rc.collapsed = false
			e.inRepository(rc, func() {
				rc.scm.sel = 0
				if by < 0 {
					rc.scm.sel = max(0, len(scmFlat(e.scmGroups()))+len(e.shownCommits())-1)
				}
			})
			return
		}
	}
	rc.scm.sel = max(0, min(next, count-1))
}
