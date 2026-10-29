package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherBatchesAndCloses(t *testing.T) {
	root := t.TempDir()
	got := make(chan Changes, 4)
	w, err := NewWatcher(func(c Changes) { got <- c })
	if err != nil {
		t.Skip("no file events on this platform")
	}
	if !w.Add(root) || !w.Add(root) || w.Watched() != 1 {
		t.Fatalf("watched = %d", w.Watched())
	}
	for _, n := range []string{"a", "b", "c"} {
		os.WriteFile(filepath.Join(root, n), []byte(n), 0o644)
	}
	select {
	case c := <-got:
		if len(c.Files) != 3 || len(c.Dirs) != 1 || c.Dirs[0] != root {
			t.Fatalf("batch = %+v", c)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no batch")
	}
	w.Close()
	if w.Add(t.TempDir()) {
		t.Fatal("a closed watcher must not watch")
	}
	os.WriteFile(filepath.Join(root, "d"), nil, 0o644)
	select {
	case c := <-got:
		t.Fatalf("callback after Close: %+v", c)
	case <-time.After(3 * WatchSettle):
	}
}
