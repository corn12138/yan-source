package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSubmodulesOnlyInitializedGitlinks(t *testing.T) {
	r, run := repo(t)
	source, _ := repo(t)
	run("-c", "protocol.file.allow=always", "submodule", "add", "-q", source.Root, "module with space")
	run("commit", "-qm", "module")
	modules, err := r.Submodules()
	if err != nil || len(modules) != 1 || modules[0].Root != r.Abs("module with space") {
		t.Fatalf("modules: %v %v", modules, err)
	}
	run("submodule", "deinit", "-f", "module with space")
	modules, err = r.Submodules()
	if err != nil || len(modules) != 0 {
		t.Fatalf("uninitialized: %v %v", modules, err)
	}
	// A directory can resolve to its parent repository; that is not a child module.
	if _, err := os.Stat(filepath.Join(r.Abs("module with space"), ".git")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	r = &Repo{Root: t.TempDir()}
	if modules, err = r.Submodules(); err != nil || len(modules) != 0 {
		t.Fatalf("without modules file: %v %v", modules, err)
	}
}
