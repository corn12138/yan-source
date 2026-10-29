package gui

import (
	"os"
	"testing"
	"time"
)

// Tests never touch the user's configuration: settings, state and the untitled text kept for
// restart live in a throwaway directory for the whole run.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "yan-test-config-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	// Pele under the race detector on a loaded machine can take far longer than a user's
	// diagram should; the tests check drawing, not the limit.
	mermaidTimeout = 2 * time.Minute
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
