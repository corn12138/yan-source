package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/view"
)

type embeddedHost struct{ fakeHost }

func (*embeddedHost) EmbeddedTerminal() bool { return true }

func TestTerminalTabsMoveSplitAndReopen(t *testing.T) {
	e, _, path := newTestEditor(t, "file content")
	defer e.Close()
	e.host = &embeddedHost{}
	e.Run("workbench.action.createTerminalEditor")
	first := e.cur().term.id
	e.Run("workbench.action.createTerminalEditor")
	second := e.cur().term.id
	if first == second {
		t.Fatal("terminals share an identity")
	}
	e.SetTerminalTitle(second, "f.txt")
	// A terminal title matching an existing file must not intercept opening that file.
	if err := e.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	if e.cur().term != nil || content(e) != "file content" {
		t.Fatal("file was shadowed by terminal")
	}
	e.MoveTab(0, 2, 1, 0)
	if e.cur().term.id != second || e.Columns() != 2 {
		t.Fatal("move lost terminal identity")
	}
	e.Run("workbench.action.splitEditor")
	third := e.cur().term.id
	if third == second || e.Columns() != 3 {
		t.Fatal("split reused the original shell")
	}
	e.CloseTab(2, 0)
	e.Run("workbench.action.reopenClosedEditor")
	if e.cur().term == nil || e.cur().term.id == third {
		t.Fatal("reopen did not create a fresh shell")
	}
	if e.cur().term.dir != filepath.Dir(path) {
		t.Fatal("reopen lost directory")
	}
	var fr view.Frame
	e.Frame(&fr)
	if fr.Columns[fr.Active].Pane.Kind != view.PaneTerminal || fr.Focus != view.FocusEditor {
		t.Fatal("terminal not active")
	}
	before := len(e.closed)
	e.TerminalExited(e.cur().term.id)
	if len(e.closed) != before {
		t.Fatal("exited shells should not enter closed history")
	}
}

func TestTerminalHomeReopenAndFrameReset(t *testing.T) {
	e, _, _ := newTestEditor(t, "text")
	defer e.Close()
	e.host = &embeddedHost{}
	e.CloseTab(0, 0)
	id := e.OpenTerminalTab(0, 0, "", "terminal")
	var fr view.Frame
	e.Frame(&fr)
	e.CloseTab(0, 0)
	e.Frame(&fr)
	if fr.Columns[0].Pane.Terminal != 0 {
		t.Fatal("empty pane kept old terminal")
	}
	e.Run("workbench.action.reopenClosedEditor")
	if e.cur().term == nil || e.cur().term.id == id || e.cur().term.dir != "" {
		t.Fatal("home terminal not reopened")
	}
}

func TestTerminalSessionAndTabWrap(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "long line"})
	defer e.Close()
	e.host = &embeddedHost{}
	path := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(path))
	if err := e.OpenFile(filepath.Join(root, "a.txt")); err != nil {
		t.Fatal(err)
	}
	e.Run("yan.toggleTabWordWrap")
	e.OpenTerminalTab(0, 0, root, "shell")
	e.SaveSession()
	e2, _ := newFolderEditorAt(t, root)
	defer e2.Close()
	e2.host = &embeddedHost{}
	e2.SetState(config.LoadState(path))
	if !e2.RestoreSession() || e2.cur().term == nil || len(e2.column().tabs) != 2 {
		t.Fatal("terminal session not restored")
	}
	if e2.column().tabs[1].wrap != 1 {
		t.Fatal("tab wrap not restored")
	}
	// TUI must skip the terminal entry instead of treating its directory as a file.
	e3, _ := newFolderEditorAt(t, root)
	defer e3.Close()
	e3.SetState(config.LoadState(path))
	if !e3.RestoreSession() || len(e3.column().tabs) != 1 || e3.cur().term != nil {
		t.Fatal("TUI restored a GUI terminal")
	}
	if b, err := os.ReadFile(filepath.Join(root, "a.txt")); err != nil || string(b) != "long line" {
		t.Fatal("terminal persistence changed the file")
	}
}

func TestTerminalEdgeMovePreservesIdentity(t *testing.T) {
	e, _, path := newTestEditor(t, "file")
	defer e.Close()
	id := e.OpenTerminalTab(0, 0, filepath.Dir(path), "shell")
	e.MoveTabToSplit(0, 0, 0, true)
	if e.Columns() != 2 || e.col != 0 || e.cur().term.id != id {
		t.Fatal("left-edge move lost terminal")
	}
	e.MoveTabToSplit(0, 0, 0, true)
	if e.Columns() != 2 {
		t.Fatal("single tab created redundant column")
	}
	e.MoveTabToSplit(0, 0, 1, false)
	if e.Columns() != 2 || e.col != 1 || e.cur().term.id != id {
		t.Fatal("cross-column edge move lost terminal")
	}
}

func TestTerminalDoesNotKeepHiddenFindFocus(t *testing.T) {
	e, _, path := newTestEditor(t, "file")
	defer e.Close()
	e.Run("actions.find")
	e.OpenTerminalTab(0, 1, filepath.Dir(path), "terminal")
	var fr view.Frame
	e.Frame(&fr)
	if fr.Find.Visible || fr.Focus != view.FocusEditor {
		t.Fatal("file find kept focus in terminal")
	}
	e.Run("workbench.action.splitEditor")
	e.FocusColumn(0)
	e.ClickTab(0, 0, false)
	e.Run("actions.find")
	e.FocusColumn(1)
	e.Frame(&fr)
	if fr.Find.Visible || fr.Focus != view.FocusEditor {
		t.Fatal("column switch kept hidden find focus")
	}
	e.Run("actions.find")
	e.Frame(&fr)
	if fr.Find.Visible || fr.Focus != view.FocusEditor {
		t.Fatal("find opened an invisible field")
	}
}
