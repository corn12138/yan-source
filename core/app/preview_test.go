package app

import (
	"github.com/corn12138/yan-source/core/config"
	"path/filepath"
	"testing"
)

func TestPreviewSideOwnershipAndSession(t *testing.T) {
	for _, osName := range []string{"darwin", "windows", "linux"} {
		t.Run(osName, func(t *testing.T) {
			e, root := newFolderEditor(t, map[string]string{"readme.md": "# A\n\ntext\n\n# B\n"})
			e.profile.GUI, e.profile.OS = true, osName
			e.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
			path := filepath.Join(root, "readme.md")
			e.OpenFile(path)
			e.cur().scrollY = 2
			original := e.cur()
			sels := e.sels(original).Main()
			if err := e.previewToSide(); err != nil {
				t.Fatal(err)
			}
			if e.cur() != original || len(e.cols) != 2 || !e.cols[1].tab().markdown || original.markdown || e.cols[1].tab().doc != original.doc {
				t.Fatal("preview changed source identity/focus")
			}
			if err := e.previewToSide(); err != nil || len(e.cols) != 2 {
				t.Fatal("duplicate preview column")
			}
			e.ScrollSourceTo(0, "wrong.md", 4)
			if original.scrollY != 2 {
				t.Fatal("stale path scrolled a different document")
			}
			e.ScrollSourceTo(0, path, 4)
			if e.sels(original).Main() != sels || e.col != 0 {
				t.Fatal("scroll moved caret or focus")
			}
			e.SetPreviewTop(1, path, 4)
			e.SaveSession()
			restored, _ := newFolderEditorAt(t, root)
			restored.profile.GUI = true
			restored.SetState(e.state)
			if !restored.RestoreSession() || !restored.cols[1].tab().markdown || restored.cols[1].tab().previewLine != 4 || restored.col != 0 {
				t.Fatal("preview session not restored")
			}
			tui, _ := newFolderEditorAt(t, root)
			tui.SetState(e.state)
			if !tui.RestoreSession() || tui.cols[1].tab().markdown {
				t.Fatal("TUI must restore editable source")
			}
		})
	}
}

func TestPreviewRejectsUnsupportedWithoutSplitting(t *testing.T) {
	e, _, _ := newTestEditor(t, "hello")
	e.profile.GUI = true
	if err := e.previewToSide(); err == nil || len(e.cols) != 1 {
		t.Fatal("unsupported preview created a column")
	}
}
