package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/corn12138/yan-source/core/config"
)

type terminalLocationHost struct {
	embeddedHost
	panels []string
}

func (h *terminalLocationHost) NewTerminal(dir string) { h.panels = append(h.panels, dir) }

func TestTerminalLocationRouting(t *testing.T) {
	for _, platform := range []string{"darwin", "windows", "linux"} {
		t.Run(platform, func(t *testing.T) {
			e, _, path := newTestEditor(t, "unchanged")
			defer e.Close()
			e.profile.OS = platform
			h := &terminalLocationHost{}
			e.host = h
			dir := filepath.Dir(path)
			e.Run("workbench.action.terminal.new")
			if len(h.panels) != 1 || h.panels[0] != dir || e.cur().term != nil {
				t.Fatal("default must open panel at the file's directory")
			}
			e.Run("view.terminalLocation")
			e.Run("workbench.action.terminal.new")
			first := e.cur().term
			if first == nil || first.dir != dir || len(h.panels) != 1 {
				t.Fatal("new terminal did not use editor preference")
			}
			e.Run("workbench.action.terminal.toggleTerminal")
			if h.shell != dir || e.cur().term != first {
				t.Fatal("toggle must still use shell/panel without replacing editor session")
			}
			e.Run("view.terminalLocation")
			e.Run("workbench.action.terminal.new")
			if len(h.panels) != 2 || e.cur().term != first {
				t.Fatal("panel preference moved an existing terminal")
			}
			e.Run("workbench.action.createTerminalEditor")
			if e.cur().term == nil || e.cur().term == first || len(h.panels) != 2 {
				t.Fatal("explicit editor command must ignore panel default")
			}
		})
	}
}

func TestTerminalLocationExplorerAndLockedColumn(t *testing.T) {
	e, root, left, right := lockedEditor(t)
	defer e.Close()
	e.Run("view.terminalLocation")
	e.Run("workbench.action.terminal.new")
	if e.column() != left || len(right.tabs) != 0 || e.cur().term.dir != root {
		t.Fatal("default terminal entered locked column")
	}
	dir := filepath.Join(root, "sub")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "nested.txt"), "x")
	e.ws.tree.Refresh()
	e.ws.tree.Reveal("sub/nested.txt")
	e.refreshVisible()
	for _, rel := range []string{"sub", "sub/nested.txt"} {
		found := false
		for i, n := range e.ws.visible {
			if n.Rel == rel {
				e.SelectTree(i)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing tree entry %s", rel)
		}
		e.Run("openInIntegratedTerminal")
		if e.cur().term == nil || e.cur().term.dir != dir {
			t.Fatalf("%s opened in wrong directory", rel)
		}
	}
	e.ClearTreeSelection()
	e.Run("openInIntegratedTerminal")
	if e.cur().term.dir != root {
		t.Fatal("workspace menu did not use workspace root")
	}
}

func TestTerminalLocationTUIFallback(t *testing.T) {
	e, h, path := newTestEditor(t, "unchanged")
	defer e.Close()
	e.Run("view.terminalLocation")
	e.Run("workbench.action.terminal.new")
	if h.shell != filepath.Dir(path) || e.cur().term != nil {
		t.Fatal("non-embedded frontend must still yield to shell")
	}
	e.host = nil
	if err := e.openTerminal(filepath.Dir(path)); err == nil {
		t.Fatal("hostless terminal must report an error")
	}
}

func TestTerminalLocationSettings(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(config.UserPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, config.UserPath(), "[terminal]\nlocation = \"editor\"\n")
	cfg, warns := config.Load("")
	if len(warns) != 0 || cfg.TerminalLocation != "editor" {
		t.Fatalf("config: %+v %v", cfg, warns)
	}
	e, _ := New(cfg, fakeLayouter{}, &fakeHost{}, macGUI)
	defer e.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(statePath))
	gen := e.settingsGen
	e.Run("view.terminalLocation")
	if e.TerminalLocation() != "" || e.settingsGen == gen {
		t.Fatal("menu did not update settings")
	}
	e.reloadConfig()
	if e.TerminalLocation() != "" {
		t.Fatal("reload lost menu override")
	}
	restored := config.Default()
	restored.TerminalLocation = "editor"
	config.LoadState(statePath).Apply(restored)
	if restored.TerminalLocation != "" {
		t.Fatal("restart lost explicit panel preference")
	}
	for _, tc := range []struct {
		value, want string
		warning     bool
	}{
		{"panel", "", false}, {"editor", "editor", false}, {"", "", false}, {"elsewhere", "", true},
	} {
		writeFile(t, config.UserPath(), "[terminal]\nlocation = \""+tc.value+"\"\n")
		c, w := config.Load("")
		if c.TerminalLocation != tc.want || (len(w) > 0) != tc.warning {
			t.Fatalf("%q: location %q, warnings %v", tc.value, c.TerminalLocation, w)
		}
	}
	invalid := "elsewhere"
	configState := &config.State{TerminalLocation: &invalid}
	restored.TerminalLocation = "editor"
	configState.Apply(restored)
	if restored.TerminalLocation != "editor" {
		t.Fatal("invalid saved state replaced valid configuration")
	}
}
