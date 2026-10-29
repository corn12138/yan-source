package app

import (
	"os"
	"testing"
)

// Tests never touch the user's configuration: settings, state and the untitled text kept for
// restart live in a throwaway directory for the whole run.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "yan-test-config-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
