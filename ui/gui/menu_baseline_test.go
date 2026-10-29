package gui

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

// The commands the menu bar offered at a09d29b, where the Quiet UI redesign started: an item
// may move to another menu but its command stays reachable from the menu bar. An intentional,
// documented change regenerates the file with UPDATE_MENU_BASELINE=1.
const menuBaseline = "testdata/menu-baseline.tsv"

func menuCommands(items []*mygo.MenuItem, path string, out map[string]string) {
	for _, it := range items {
		if it.ID != "" {
			out[it.ID] = path + " › " + it.Label
		}
		menuCommands(it.Submenu, path+" › "+it.Label, out)
	}
}

func TestMenuBaselineKept(t *testing.T) {
	h := newHarness(t, sample, Options{})
	now := map[string]string{}
	menuCommands(h.f.menu().Items(), "", now)
	if os.Getenv("UPDATE_MENU_BASELINE") != "" {
		var rows []string
		for id, where := range now {
			rows = append(rows, id+"\t"+strings.TrimPrefix(where, " › "))
		}
		slices.Sort(rows)
		os.MkdirAll("testdata", 0o755)
		if err := os.WriteFile(menuBaseline, []byte("# id\tmenu path\n"+strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(menuBaseline)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		id, where, _ := strings.Cut(line, "\t")
		if _, ok := now[id]; !ok {
			t.Errorf("menu bar no longer offers %s (was %s)", id, where)
		}
	}
}
