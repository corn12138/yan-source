package app

import (
	"math"
	"path/filepath"
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/git"
	"github.com/corn12138/yan-source/core/view"
)

// History file paths are resolved inside their original repository. Row positions
// are only presentation: delayed UI actions carry the tab identity or full hash.
type commitFilesState struct {
	open, loading, loaded bool
	files                 []git.CommitFile
	closed                map[string]bool
	err                   string
}

func commitFileRows(root string, files []git.CommitFile, tree bool, closed map[string]bool) []view.CommitFile {
	var out []view.CommitFile
	if !tree {
		for _, f := range files {
			rel, _ := filepath.Rel(root, f.Path)
			out = append(out, view.CommitFile{Path: f.Path, Name: filepath.ToSlash(rel), Status: f.Status})
		}
		return out
	}
	type node struct {
		path, name string
		file       *git.CommitFile
		kids       map[string]*node
	}
	top := &node{path: root, kids: map[string]*node{}}
	for _, f := range files {
		rel, err := filepath.Rel(root, f.Path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		n := top
		for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
			if n.kids == nil {
				n.kids = map[string]*node{}
			}
			if n.kids[part] == nil {
				n.kids[part] = &node{path: filepath.Join(n.path, part), name: part}
			}
			n = n.kids[part]
		}
		n.file = &f
	}
	var walk func(*node, int)
	walk = func(n *node, depth int) {
		kids := make([]*node, 0, len(n.kids))
		for _, k := range n.kids {
			kids = append(kids, k)
		}
		slices.SortFunc(kids, func(a, b *node) int {
			if (a.file == nil) != (b.file == nil) {
				if a.file == nil {
					return -1
				}
				return 1
			}
			return strings.Compare(a.name, b.name)
		})
		for _, k := range kids {
			row := view.CommitFile{Path: k.path, Name: k.name, Depth: depth, Folder: k.file == nil, Collapsed: closed[k.path]}
			if k.file != nil {
				row.Status = k.file.Status
			}
			out = append(out, row)
			if row.Folder && !row.Collapsed {
				walk(k, depth+1)
			}
		}
	}
	walk(top, 0)
	return out
}

func (e *Editor) toggleCommitFiles() error {
	e.commitFilesOpen = !e.commitFilesOpen
	e.saveCommitFiles()
	return nil
}
func (e *Editor) saveCommitFiles() {
	if e.state != nil {
		e.state.CommitFilesOpen = e.commitFilesOpen
		e.state.CommitFilesWidth = e.CommitFilesWidth()
		e.saveState()
	}
}
func (e *Editor) CommitFilesWidth() float64 {
	if e.commitFilesWidth < 140 || math.IsNaN(e.commitFilesWidth) || math.IsInf(e.commitFilesWidth, 0) {
		return 220
	}
	return e.commitFilesWidth
}
func (e *Editor) SetCommitFilesWidth(w float64) {
	if math.IsNaN(w) || math.IsInf(w, 0) {
		return
	}
	e.commitFilesWidth = max(140, min(w, 600))
	e.saveCommitFiles()
}
func (e *Editor) commitFilesFrame(t *tab) *view.CommitFiles {
	dt := t.diff
	if dt == nil || dt.commit == nil {
		return nil
	}
	defer e.bindDiff(dt)()
	files := make([]git.CommitFile, len(dt.secs))
	for i, s := range dt.secs {
		files[i] = git.CommitFile{Path: s.path, Orig: s.orig, Status: s.status}
	}
	out := &view.CommitFiles{Identity: dt.path, Visible: e.commitFilesOpen, Width: e.CommitFilesWidth(), Count: len(files), Loading: dt.loading, Error: dt.loadError}
	out.Rows = commitFileRows(dt.repoRoot, files, e.repository().scm.tree, dt.fileFolders)
	top := t.scrollY
	if !dt.copyLeft && e.splitPanes(t) {
		top = dt.rightTop
	}
	for i := min(top, len(dt.lines)-1); i >= 0; i-- {
		l := dt.lines[i]
		if l.sec >= 0 && l.sec < len(dt.secs) && l.kind != diffReview {
			out.Selected = dt.secs[l.sec].path
			break
		}
	}
	return out
}

func (e *Editor) commitTab(col int, identity string) *tab {
	if col < 0 || col >= len(e.cols) {
		return nil
	}
	t := e.cols[col].tab()
	if t == nil || t.diff == nil || t.diff.commit == nil || t.diff.path != identity {
		return nil
	}
	return t
}

// CommitFileAction targets the rendered tab, never whichever tab replaced it.
func (e *Editor) CommitFileAction(col int, identity, path string, pin bool) {
	e.sync()
	defer e.sync()
	t := e.commitTab(col, identity)
	if t == nil {
		return
	}
	// A file list in an inactive column must load that tab's selection before moving it.
	e.FocusColumn(col)
	rows := e.commitFilesFrame(t).Rows
	i := slices.IndexFunc(rows, func(r view.CommitFile) bool { return r.Path == path })
	if i < 0 {
		return
	}
	if rows[i].Folder {
		if t.diff.fileFolders == nil {
			t.diff.fileFolders = map[string]bool{}
		}
		t.diff.fileFolders[path] = !t.diff.fileFolders[path]
		return
	}
	e.revealCommitFile(t, path)
	if pin {
		t.preview = false
	}
}
func (e *Editor) revealCommitFile(t *tab, path string) {
	dt := t.diff
	if dt.loading {
		dt.revealFile = path
		return
	}
	for _, s := range dt.secs {
		if s.path == path && dt.collapsed[s.key()] {
			dt.collapsed[s.key()] = false
			e.buildChanges(t.doc, dt)
			break
		}
	}
	for i, l := range dt.lines {
		if l.kind != diffFile || dt.pathOf(l) != path {
			continue
		}
		t.scrollY, dt.rightTop, dt.syncedTop = i, i, i
		dt.alignRight, dt.glide = true, true
		t.doc.buf.SetSelections(edit.Single(edit.Cursor(t.doc.buf.Snapshot().LineStart(i))))
		t.sels = t.doc.buf.Selections()
		return
	}
}

func (e *Editor) expandedCommitRows() []view.SCMCommit {
	sc := e.repository().scm
	if e.repository().git.repo == nil {
		return nil
	}
	var out []view.SCMCommit
	for _, c := range sc.history.rows {
		fs := sc.history.files[c.Hash]
		if fs != nil {
			c.Expanded, c.FilesLoading, c.FilesError = fs.open, fs.loading, fs.err
		}
		out = append(out, c)
		if fs == nil || !fs.open {
			continue
		}
		for _, r := range commitFileRows(e.repository().git.repo.Root, fs.files, sc.tree, fs.closed) {
			out = append(out, view.SCMCommit{Hash: c.Hash, File: &r})
		}
	}
	return out
}
func (e *Editor) selectedHistoryRow() (view.SCMCommit, bool) {
	i := e.repository().scm.sel - len(scmFlat(e.scmGroups()))
	rows := e.shownCommits()
	if i < 0 || i >= len(rows) {
		return view.SCMCommit{}, false
	}
	return rows[i], true
}
func (e *Editor) selectHistoryRow(hash, path string) {
	for i, r := range e.shownCommits() {
		if r.Hash == hash && (path == "" && r.File == nil || r.File != nil && r.File.Path == path) {
			e.repository().scm.sel = len(scmFlat(e.scmGroups())) + i
			return
		}
	}
}

func (e *Editor) SCMToggleCommitFiles(hash string) {
	e.sync()
	defer e.sync()
	if e.repository().git.repo == nil {
		return
	}
	e.side, e.area, e.repository().scm.focusMsg = sideSCM, areaTree, false
	e.findFocus = fieldNone
	h := &e.repository().scm.history
	k := slices.IndexFunc(h.commits, func(c git.Commit) bool { return c.Hash == hash })
	if k < 0 {
		return
	}
	if h.files == nil {
		h.files = map[string]*commitFilesState{}
	}
	fs := h.files[hash]
	if fs == nil {
		fs = &commitFilesState{closed: map[string]bool{}}
		h.files[hash] = fs
	}
	fs.open = !fs.open
	e.selectHistoryRow(hash, "")
	if !fs.open || fs.loading || fs.loaded {
		return
	}
	fs.loading = true
	r, c, post := e.repository().git.repo, h.commits[k], e.postGit()
	go func() {
		files, err := r.CommitFiles(c)
		post(func(e *Editor) {
			if e.repository().git.repo != r || e.repository().scm.history.files[hash] != fs {
				return
			}
			fs.loading = false
			if err != nil {
				fs.err = gitError(err)
				e.status = fs.err
				return
			}
			selected, hasSelection := e.selectedHistoryRow()
			fs.files, fs.loaded, fs.err = files, true, ""
			if hasSelection {
				path := ""
				if selected.File != nil {
					path = selected.File.Path
				}
				e.selectHistoryRow(selected.Hash, path)
			}
		})
	}()
}
func (e *Editor) SCMCommitFile(hash, path string, pin bool) {
	e.sync()
	defer e.sync()
	fs := e.repository().scm.history.files[hash]
	if fs == nil {
		return
	}
	r := e.repository().git.repo
	if r == nil {
		return
	}
	rows := commitFileRows(r.Root, fs.files, e.repository().scm.tree, fs.closed)
	i := slices.IndexFunc(rows, func(row view.CommitFile) bool { return row.Path == path })
	if i < 0 {
		return
	}
	e.side, e.area, e.repository().scm.focusMsg = sideSCM, areaTree, false
	e.findFocus = fieldNone
	e.selectHistoryRow(hash, path)
	if rows[i].Folder {
		fs.closed[path] = !fs.closed[path]
		return
	}
	t := e.showHistoryTabAt(r.Root, "", hash, !pin)
	e.revealCommitFile(t, path)
	if !pin {
		e.area = areaTree
	}
}
func (e *Editor) scmCommitExpand(open bool) error {
	row, ok := e.selectedHistoryRow()
	if !ok {
		return nil
	}
	if row.File == nil {
		if row.Expanded != open {
			e.SCMToggleCommitFiles(row.Hash)
		} else if open {
			rows := e.shownCommits()
			next := e.repository().scm.sel - len(scmFlat(e.scmGroups())) + 1
			if next < len(rows) && rows[next].Hash == row.Hash && rows[next].File != nil {
				e.moveRepositoryRow(1)
			}
		}
		return nil
	}
	f := row.File
	if f.Folder && f.Collapsed == open {
		e.SCMCommitFile(row.Hash, f.Path, false)
		return nil
	}
	if !open {
		parent := filepath.Dir(f.Path)
		e.selectHistoryRow(row.Hash, "")
		e.selectHistoryRow(row.Hash, parent)
	} else if f.Folder {
		e.moveRepositoryRow(1)
	}
	return nil
}

func (e *Editor) CommitFilesToggleTree(col int, identity string) {
	t := e.commitTab(col, identity)
	if t == nil {
		return
	}
	defer e.bindDiff(t.diff)()
	e.toggleSCMTree()
}

func (e *Editor) SCMOpenCommit(hash string, pin bool) {
	e.sync()
	defer e.sync()
	if slices.IndexFunc(e.repository().scm.history.commits, func(c git.Commit) bool { return c.Hash == hash }) < 0 {
		return
	}
	e.unfoldSCM("commits")
	e.side, e.area, e.repository().scm.focusMsg = sideSCM, areaTree, false
	e.findFocus = fieldNone
	e.selectHistoryRow(hash, "")
	if err := e.scmOpen(pin); err != nil {
		e.status = err.Error()
	}
}
