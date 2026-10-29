package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/view"
)

// A folded group keeps its count and its files for acting on the whole group; folding is kept
// for the next start, and selecting a commit from elsewhere unfolds the commits.
func TestSCMSectionsFold(t *testing.T) {
	root := gitRepo(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("A\n"), 0o644)
	os.WriteFile(filepath.Join(root, "b.txt"), []byte("B\n"), 0o644)
	path := filepath.Join(t.TempDir(), "state.json")
	e := newEditorIn(t, root)
	e.SetState(config.LoadState(path))
	pump(t, e, func() bool { return e.git.status != nil && len(e.repository().scm.history.rows) > 0 })
	e.Run("workbench.view.scm")
	e.SCMToggleSection("changes")
	e.SCMToggleSection("commits")
	var f view.Frame
	e.Frame(&f)
	s := f.Sidebar.SCM
	if len(s.Groups) != 1 || !s.Groups[0].Folded || s.Groups[0].Count != 2 || len(s.Groups[0].Rows) != 0 {
		t.Fatalf("groups: %+v", s.Groups)
	}
	if !s.CommitsFolded || len(s.Commits) != 0 || s.Selected != 0 {
		t.Fatalf("commits folded %v, %d shown, selected %d", s.CommitsFolded, len(s.Commits), s.Selected)
	}
	if got := len(e.SCMGroupFiles(view.GroupChanges)); got != 2 {
		t.Fatalf("group files: %d", got)
	}
	// The next start keeps both folded.
	if st := config.LoadState(path); len(st.SCMFolded) != 2 {
		t.Fatalf("kept %v", st.SCMFolded)
	}
	// Staging the whole folded group stages both files.
	e.SCMStageGroup(view.GroupChanges)
	pump(t, e, func() bool {
		e.Frame(&f)
		gs := f.Sidebar.SCM.Groups
		return len(gs) == 1 && gs[0].Kind == view.GroupStaged && gs[0].Count == 2
	})
	// Choosing a commit by its hash (a context menu's command) unfolds the commits.
	e.SCMCommitCommand(e.repository().scm.history.commits[0].Hash, "noop.command")
	e.Frame(&f)
	if f.Sidebar.SCM.CommitsFolded || len(f.Sidebar.SCM.Commits) == 0 {
		t.Fatal("selecting a commit left the commits folded")
	}
}
