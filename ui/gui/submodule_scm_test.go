package gui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/i18n"
)

func TestSubmoduleSCMInTheWindow(t *testing.T) {
	for _, appearance := range []string{"dark", "light"} {
		t.Run(appearance, func(t *testing.T) {
			if appearance == "light" {
				previous := i18n.Current()
				i18n.SetLocale(i18n.English)
				t.Cleanup(func() { i18n.SetLocale(previous) })
			}
			root, source := t.TempDir(), t.TempDir()
			nestedGUIRepo(t, root)
			nestedGUIRepo(t, source)
			git := func(dir string, args ...string) {
				t.Helper()
				prefix := []string{"-C", dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid"}
				if out, err := exec.Command("git", append(prefix, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("git %v: %s (%v)", args, out, err)
				}
			}
			if err := os.WriteFile(filepath.Join(source, "module.txt"), []byte("initial\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			git(source, "add", ".")
			git(source, "commit", "-qm", "initial module")
			git(root, "-c", "protocol.file.allow=always", "submodule", "add", "-q", source, "modules/lib")
			git(root, "commit", "-qm", "parent commit")
			module := filepath.Join(root, "modules", "lib")
			if err := os.WriteFile(filepath.Join(module, "module.txt"), []byte("dirty module\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			h := newHarnessAt(t, root, Options{Appearance: appearance})
			h.ed.Run("workbench.view.scm")
			for until := time.Now().Add(10 * time.Second); !h.tt.HasText(i18n.T("Content changes")) && time.Now().Before(until); {
				h.settle()
			}
			if !h.tt.HasText(i18n.T("Content changes")) {
				t.Fatalf("missing module state: %v", h.tt.Texts())
			}
			panels := h.f.frame.Sidebar.SCM.Panels
			if len(panels) != 2 {
				t.Fatalf("panels: %d", len(panels))
			}
			parentID := panels[0].State.RepositoryID
			focusParent := func() {
				h.ed.WithRepository(parentID, func(ed *app.Editor) { ed.SetCommitMessage("Parent draft"); ed.FocusCommitMessage(true) })
				h.settle()
				if !h.tt.Focused(i18n.T("Commit message") + " · " + filepath.Base(root)) {
					t.Fatal("parent input did not gain focus")
				}
			}
			focusParent()
			h.f.openSubmoduleChanges(repositoryEditor{Editor: h.ed, id: parentID}, module)
			h.settle()
			if h.f.frame.Sidebar.SCM.Root != module {
				t.Fatal("old input reclaimed selection after a deferred navigation callback")
			}
			childID := panels[1].State.RepositoryID
			h.ed.ToggleRepository(childID)
			h.f.focusRoot = true
			h.settle()
			h.ed.Run("list.focusMessage")
			h.settle()
			if !h.tt.Focused(i18n.T("Commit message") + " · modules/lib") {
				t.Fatal("keyboard could not focus a previously collapsed child input")
			}
			h.ed.SelectRepository(parentID)
			h.f.focusRoot = true
			h.settle()
			focusParent()
			if err := h.tt.RightClick("lib"); err != nil {
				t.Fatal(err)
			}
			h.settle()
			if err := h.tt.Click(i18n.T("Open Submodule Changes")); err != nil {
				t.Fatal(err)
			}
			h.settle()
			if h.f.frame.Sidebar.SCM.Root != module || h.ed.Root() != root || h.f.frame.Sidebar.SCM.Panels[0].State.Message != "Parent draft" {
				t.Fatal("module menu changed workspace, lost draft, or kept the parent selected")
			}
			h.shot("submodule-scm-" + appearance)
			h.tt.SetSize(760, 660)
			h.settle()
			if !h.tt.HasText(i18n.T("Content changes")) {
				t.Fatal("narrow window lost the submodule state")
			}
			h.shot("submodule-scm-narrow-" + appearance)
		})
	}
}
