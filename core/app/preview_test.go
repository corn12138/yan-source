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

func TestPreviewReusesUnlockedSideColumn(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		source, target, count int
		locked                []int
		existing              bool
	}{
		{name: "right", source: 0, target: 1, count: 2},
		{name: "left", source: 1, target: 0, count: 2},
		{name: "right before left", source: 1, target: 2, count: 3},
		{name: "skip locked right", source: 0, target: 2, count: 3, locked: []int{1}},
		{name: "locked right uses left", source: 1, target: 0, count: 3, locked: []int{2}},
		{name: "existing source tab", source: 0, target: 1, count: 2, existing: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, root := newFolderEditor(t, map[string]string{"readme.md": "# A\n\ntext\n", "other.go": "package other\n"})
			e.profile.GUI = true
			for len(e.cols) < tc.count {
				e.cols = append(e.cols, newColumn())
			}
			for _, c := range e.cols {
				if err := e.openIn(c, 0, filepath.Join(root, "other.go"), false); err != nil {
					t.Fatal(err)
				}
			}
			for _, i := range tc.locked {
				e.cols[i].locked = true
			}
			c, target := e.cols[tc.source], e.cols[tc.target]
			if err := e.openIn(c, 1, filepath.Join(root, "readme.md"), false); err != nil {
				t.Fatal(err)
			}
			source := e.cur()
			source.scrollY, source.scrollX, source.wrap = 1, 3, -1
			var existing *tab
			if tc.existing {
				e.addTab(target, 1, source.doc, source)
				existing = e.cur()
				e.activate(c, 1)
			}
			before := len(target.tabs)
			weights := make([]float64, len(e.cols))
			for i, col := range e.cols {
				weights[i] = col.weight
			}
			if err := e.previewToSide(); err != nil {
				t.Fatal(err)
			}
			dest := target.tab()
			if len(e.cols) != tc.count || e.cur() != source || source.markdown || dest == source || dest.doc != source.doc || !dest.markdown || dest.previewLine != 1 {
				t.Fatal("side preview changed source focus, document identity or column layout")
			}
			want := before + 1
			if existing != nil {
				want = before
				if dest != existing {
					t.Fatal("existing tab was replaced")
				}
			}
			if len(target.tabs) != want {
				t.Fatalf("tabs %d, want %d", len(target.tabs), want)
			}
			for i, col := range e.cols {
				if col.weight != weights[i] {
					t.Fatal("reusing a column changed its width")
				}
				if col.locked && (len(col.tabs) != 1 || col.tab().markdown) {
					t.Fatal("locked column changed")
				}
			}
			if err := e.previewToSide(); err != nil || len(target.tabs) != want || len(e.cols) != tc.count {
				t.Fatal("repeated preview added a tab or column")
			}
		})
	}
}

func TestPreviewSkipsLockedExistingPreview(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"readme.md": "# A\n"})
	e.profile.GUI = true
	e.OpenFile(filepath.Join(root, "readme.md"))
	if err := e.previewToSide(); err != nil {
		t.Fatal(err)
	}
	locked := e.cols[1]
	locked.locked = true
	before := locked.tab()
	if err := e.previewToSide(); err != nil {
		t.Fatal(err)
	}
	if len(e.cols) != 3 || locked.tab() != before || len(locked.tabs) != 1 || !e.cols[1].tab().markdown || e.cols[1] == locked {
		t.Fatal("no new unlocked preview column, or locked preview changed")
	}
}
