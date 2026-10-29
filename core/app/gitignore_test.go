package app

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/keymap"
)

// Files without a Chroma language still comment with their own marker: Cmd+/ in .gitignore.
func TestToggleCommentInFilesWithoutALanguage(t *testing.T) {
	for name, want := range map[string]string{
		".gitignore": "# H5Build\n", ".env.local": "# H5Build\n", ".dockerignore": "# H5Build\n", "app.properties": "# H5Build\n",
	} {
		e, root := newFolderEditor(t, map[string]string{name: "H5Build\n"})
		if err := e.OpenFile(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
		e.Handle(cmdKey('/', keymap.ModSuper))
		if got := content(e); got != want {
			t.Fatalf("%s: %q (status %q)", name, got, e.status)
		}
		e.Run("editor.action.commentLine")
		if got := content(e); got != "H5Build\n" {
			t.Fatalf("%s uncomment: %q", name, got)
		}
	}
}

// Commenting out a rule in .gitignore, in the editor or elsewhere, stops dimming what it matched
// and brings its files into quick open.
func TestGitignoreChangesApplyToTheExplorer(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{".gitignore": "node_modules\nH5Build\n", "H5Build/index.html": "<p>\n", "src/a.ts": "a\n"})
	if e.ws.watcher == nil {
		t.Skip("no file events on this platform")
	}
	node := func(name string) bool {
		for _, n := range e.ws.tree.Root.Children {
			if n.Name == name {
				return n.Ignored
			}
		}
		t.Fatalf("no %s in the tree", name)
		return false
	}
	if !node("H5Build") || slices.Contains(e.ws.index, "H5Build/index.html") {
		t.Fatal("H5Build must start ignored")
	}
	// In the editor: comment the line and save.
	if err := e.OpenFile(filepath.Join(root, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	e.GotoLine(2)
	e.Run("editor.action.commentLine")
	e.Run("workbench.action.files.save")
	pump(t, e, func() bool { return !node("H5Build") && slices.Contains(e.ws.index, "H5Build/index.html") })
	// Elsewhere: ignore it again.
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("node_modules\nH5Build\n"), 0o644)
	pump(t, e, func() bool { return node("H5Build") && !slices.Contains(e.ws.index, "H5Build/index.html") })
	if node("src") {
		t.Fatal("src must stay visible")
	}
}

// Without file events, regaining focus reads the ignore rules again.
func TestFocusReloadsIgnoreRules(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{".gitignore": "H5Build\n", "H5Build/x": "x\n"})
	if e.ws.watcher != nil {
		e.ws.watcher.Close()
	}
	os.WriteFile(filepath.Join(root, ".gitignore"), []byte("# H5Build\n"), 0o644)
	e.Handle(FocusInput{Gained: true})
	for _, n := range e.ws.tree.Root.Children {
		if n.Name == "H5Build" && n.Ignored {
			t.Fatal("focus did not reload the ignore rules")
		}
	}
}
