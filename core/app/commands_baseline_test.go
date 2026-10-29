package app

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
)

// The command inventory of a09d29b, where the Quiet UI redesign started: a redesign may move
// a command to another place but not drop it or change its default keys. An intentional,
// documented change regenerates the file with UPDATE_COMMAND_BASELINE=1.
const commandBaseline = "testdata/commands-baseline.tsv"

var baselineProfiles = []struct {
	name string
	p    keymap.Profile
}{
	{"macOS window", keymap.Profile{Mods: []keymap.Mod{keymap.ModSuper}, GUI: true, OS: "darwin"}},
	{"Windows window", keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}, GUI: true, OS: "windows"}},
	{"Linux window", keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}, GUI: true, OS: "linux"}},
	{"terminal", keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}}},
}

func commandInventory(t *testing.T) []string {
	t.Helper()
	var eds []*Editor
	for _, bp := range baselineProfiles {
		e, warns := New(config.Default(), fakeLayouter{}, &fakeHost{}, bp.p)
		if len(warns) > 0 {
			t.Fatalf("%s: %v", bp.name, warns)
		}
		t.Cleanup(e.Close)
		eds = append(eds, e)
	}
	var rows []string
	for _, c := range eds[0].Commands() {
		row := []string{c.ID, fmt.Sprint(c.Hidden), c.Title}
		for _, e := range eds {
			row = append(row, e.KeyHint(c.ID))
		}
		rows = append(rows, strings.Join(row, "\t"))
	}
	slices.Sort(rows)
	return rows
}

func TestCommandsBaselineKept(t *testing.T) {
	rows := commandInventory(t)
	if os.Getenv("UPDATE_COMMAND_BASELINE") != "" {
		head := "# id\thidden\ttitle"
		for _, bp := range baselineProfiles {
			head += "\t" + bp.name
		}
		os.MkdirAll(filepath.Dir(commandBaseline), 0o755)
		if err := os.WriteFile(commandBaseline, []byte(head+"\n"+strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(commandBaseline)
	if err != nil {
		t.Fatal(err)
	}
	now := map[string][]string{}
	for _, r := range rows {
		f := strings.Split(r, "\t")
		now[f[0]] = f
	}
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		want := strings.Split(line, "\t")
		got, ok := now[want[0]]
		switch {
		case !ok:
			t.Errorf("command %s (%s) was removed", want[0], want[2])
		case got[1] != want[1]:
			t.Errorf("command %s: hidden from the palette %s, was %s", want[0], got[1], want[1])
		default:
			for i, bp := range baselineProfiles {
				if got[3+i] != want[3+i] {
					t.Errorf("command %s: %s key %q, was %q", want[0], bp.name, got[3+i], want[3+i])
				}
			}
		}
	}
}
