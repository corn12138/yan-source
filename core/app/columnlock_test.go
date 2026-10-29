package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/config"
)

// lockedEditor leaves a file column beside an empty locked column, as Mayoi's lockedHarness.
func lockedEditor(t *testing.T) (*Editor, string, *column, *column) {
	t.Helper()
	e, root := newFolderEditor(t, map[string]string{"main.go": "package main\n", "README.md": "# r\n", "go.mod": "module m\n"})
	e.host = &embeddedHost{}
	if err := e.OpenFile(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	left := e.column()
	e.cols = append(e.cols, newColumn())
	right := e.cols[1]
	e.col = 1
	e.Run("view.lockColumn")
	if !right.locked || e.column() != right || len(e.cols) != 2 {
		t.Fatalf("lock: locked %v, active %d, %d columns", right.locked, e.col, len(e.cols))
	}
	return e, root, left, right
}

func TestLockedColumnRoutesImplicitOpens(t *testing.T) {
	e, root, left, right := lockedEditor(t)
	for _, tc := range []struct {
		name string
		open func()
	}{
		{"file", func() { e.OpenFile(filepath.Join(root, "README.md")) }},
		{"preview", func() { e.previewFile(filepath.Join(root, "go.mod")) }},
		{"terminal", func() { e.Run("workbench.action.createTerminalEditor") }},
		{"new file", func() { e.NewFile(filepath.Join(root, "new.txt")) }},
	} {
		e.col = 1
		tc.open()
		if e.column() != left || len(right.tabs) != 0 {
			t.Fatalf("%s: opened in column %d, %d tabs in the locked column", tc.name, e.col, len(right.tabs))
		}
	}
	// An explicit move still targets the locked column, and a later open of that file goes left.
	e.MoveTab(0, 1, 1, 0)
	if len(right.tabs) != 1 || right.tabs[0].doc.buf.Path() != filepath.Join(root, "README.md") {
		t.Fatal("an explicit move did not use the locked column")
	}
	e.col = 1
	e.OpenFile(filepath.Join(root, "README.md"))
	if e.column() != left || len(right.tabs) != 1 {
		t.Fatal("opening a file shown in the locked column went there")
	}
}

func TestLockedColumnKeepsWorkspaceAndWidth(t *testing.T) {
	e, root, left, right := lockedEditor(t)
	left.weight, right.weight = 3, 2
	// Closing the last file leaves both columns: the locked one stays, and so does the last
	// unlocked one.
	for len(left.tabs) > 0 {
		e.closeTab(left, 0, true)
	}
	if len(e.cols) != 2 || left.weight != 3 || right.weight != 2 {
		t.Fatalf("closing the workspace changed the layout: %d columns", len(e.cols))
	}
	// An extra file column can go; its width returns to a file column, not the locked one.
	e.col = 0
	e.OpenFile(filepath.Join(root, "main.go"))
	e.Run("workbench.action.splitEditor")
	mid := e.column()
	if mid == left || mid == right {
		t.Fatal("no new column")
	}
	e.closeTab(mid, 0, true)
	if len(e.cols) != 2 || right.weight != 2 || left.weight != 3 {
		t.Fatalf("after closing the middle column: %d columns, weights %v %v", len(e.cols), left.weight, right.weight)
	}
	// Moving the last file into the locked column keeps the workspace.
	e.moveTab(left, 0, right, 0)
	if len(e.cols) != 2 || len(left.tabs) != 0 || e.cols[0] != left {
		t.Fatal("moving the last file collapsed the workspace")
	}
	// Unlocking an empty locked column removes it; locking the only column makes a new one.
	e.closeTab(right, 0, true)
	e.col = 1
	e.Run("view.lockColumn")
	if len(e.cols) != 1 || e.cols[0] != left {
		t.Fatalf("unlocking an empty column kept it: %d columns", len(e.cols))
	}
	e.Run("view.lockColumn")
	if len(e.cols) != 2 || !left.locked || e.cols[1] != left || e.column() != left {
		t.Fatal("locking the only column must add a file column and keep focus")
	}
}

func TestTerminalMovesToLockedRightKeepingItsShell(t *testing.T) {
	e, _, left, _ := lockedEditor(t)
	e.col = 0
	e.Run("workbench.action.createTerminalEditor")
	id := e.cur().term.id
	if e.column() != left {
		t.Fatal("the terminal did not open in the file column")
	}
	e.Run("view.terminalLockRight")
	last := e.cols[len(e.cols)-1]
	if !last.locked || last.tab() == nil || last.tab().term == nil || last.tab().term.id != id {
		t.Fatal("the terminal did not move, with its id, into the locked right column")
	}
	if len(e.cols) != 2 {
		t.Fatalf("an empty terminal-ready locked column must be reused: %d columns", len(e.cols))
	}
	// A right column holding files is not reused: a new locked column is made.
	e.col = 0
	e.Run("workbench.action.createTerminalEditor")
	second := e.cur().term.id
	last.locked = false
	e.OpenFile(filepath.Join(e.root, "go.mod"))
	e.moveTab(e.column(), e.column().active, last, 0)
	e.col = 0
	for i, tb := range e.column().tabs {
		if tb.term != nil && tb.term.id == second {
			e.activate(e.column(), i)
		}
	}
	e.Run("view.terminalLockRight")
	if n := len(e.cols); n != 3 || !e.cols[2].locked || e.cols[2].tab().term.id != second {
		t.Fatalf("a mixed right column was reused: %d columns", n)
	}
}

func TestLockedColumnsRestoreWithSession(t *testing.T) {
	e, root, _, right := lockedEditor(t)
	path := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(path))
	right.weight = 0.5
	e.col = 0
	e.SaveSession()
	e.state.Save()

	e2 := newEditorIn(t, root)
	e2.SetState(config.LoadState(path))
	if !e2.RestoreSession() {
		t.Fatal("no session")
	}
	if len(e2.cols) != 2 || e2.cols[0].locked || !e2.cols[1].locked || len(e2.cols[1].tabs) != 0 || e2.cols[1].weight != 0.5 {
		t.Fatalf("restored %d columns, locks %v %v", len(e2.cols), e2.cols[0].locked, e2.cols[len(e2.cols)-1].locked)
	}

	// A state where every column is locked restores an editable column.
	st := config.LoadState(path)
	s := st.Sessions[root]
	for i := range s.Columns {
		s.Columns[i].Locked = true
	}
	st.Sessions[root] = s
	e3 := newEditorIn(t, root)
	e3.SetState(st)
	e3.RestoreSession()
	if e3.column().locked || len(e3.cols) != 3 {
		t.Fatalf("all-locked state: active locked %v, %d columns", e3.column().locked, len(e3.cols))
	}
	os.Remove(path)
}

// A search result dropped on a locked column opens there (an explicit drop), the match selected;
// a result list that changed since the drag began drops nothing.
func TestDropSearchMatchTargetsTheDroppedColumn(t *testing.T) {
	e, root, _, right := lockedEditor(t)
	writeFile(t, filepath.Join(root, "x.txt"), "a\nneedle\n")
	e.ws.index = append(e.ws.index, "x.txt")
	e.Run("workbench.action.findInFiles")
	e.SetSearchField(0, "needle")
	pump(t, e, func() bool { return !e.fs.running && len(e.fs.files) == 1 })
	e.DropSearchMatch(1, 1, 0, 0) // a stale line: nothing opens
	if len(right.tabs) != 0 {
		t.Fatal("a stale drop opened a file")
	}
	e.DropSearchMatch(1, 1, 1, 0)
	if len(right.tabs) != 1 || e.column() != right {
		t.Fatalf("the drop did not open in the locked column")
	}
	if sel := e.cur().doc.buf.Selections().Main(); sel.From() != 2 || sel.To() != 8 {
		t.Fatalf("selection %+v", sel)
	}
	e.DropSearchMatch(1, 1, 1, 1) // onto the right edge: a new column after it
	if len(e.cols) != 3 || e.column() != e.cols[2] {
		t.Fatalf("split drop: %d columns, active %d", len(e.cols), e.col)
	}
}
