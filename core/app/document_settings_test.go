package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func TestDocumentSettings(t *testing.T) {
	e, _, _ := newTestEditor(t, "first\nsecond")
	defer e.Close()
	e.Run("yan.eol.crlf")
	e.Run("cursorEnd")
	e.Handle(key(keymap.KeyEnter, 0))
	if content(e) != "first\r\n\r\nsecond" {
		t.Fatalf("enter: %q", content(e))
	}
	e.Run("undo")
	e.Run("undo")
	if content(e) != "first\nsecond" {
		t.Fatal("conversion did not undo in one step")
	}
	e.Run("yan.indent.choose")
	e.ov.items[2].run(e)
	if e.indentOf(e.cur()).size != 4 {
		t.Fatal("indent not applied")
	}
	var frame view.Frame
	e.Frame(&frame)
	found := 0
	for _, it := range frame.Status.Right {
		if it.Command == "yan.indent.choose" || it.Command == "yan.eol.choose" {
			found++
		}
	}
	if found != 2 {
		t.Fatal("status actions missing")
	}
}

func TestSidebarPositionPersistedAndShared(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"x": "x"})
	defer e.Close()
	path := filepath.Join(t.TempDir(), "state.json")
	state := config.LoadState(path)
	e.SetState(state)
	e2, _ := newFolderEditorAt(t, root)
	defer e2.Close()
	e2.SetState(state)
	e.Run("workbench.action.toggleSidebarPosition")
	var f view.Frame
	e2.Frame(&f)
	if !f.Sidebar.Right {
		t.Fatal("other project did not see position")
	}
	cfg := config.Default()
	config.LoadState(path).Apply(cfg)
	if !cfg.SidebarRight {
		t.Fatal("position not persisted")
	}
	e.Run("workbench.action.toggleSidebarPosition")
	e.Frame(&f)
	if f.Sidebar.Right {
		t.Fatal("cannot return to left")
	}
}

func TestInvalidFolderPreservesTabs(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"x": "original"})
	defer e.Close()
	e.OpenFile(filepath.Join(root, "x"))
	for _, path := range []string{filepath.Join(root, "missing"), filepath.Join(root, "x")} {
		if err := e.SwitchFolder(path); err == nil {
			t.Fatal("invalid folder accepted")
		}
		if e.Root() != root || content(e) != "original" {
			t.Fatal("tabs lost on invalid folder")
		}
	}
	os.WriteFile(filepath.Join(root, ".yan.toml"), []byte("[explorer]\nright = true\n"), 0644)
	cfg, warns := config.Load(root)
	if len(warns) > 0 || !cfg.SidebarRight {
		t.Fatalf("config: %v", warns)
	}
}
