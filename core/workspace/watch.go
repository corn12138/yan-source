package workspace

import (
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// The watching follows Mayoi (MIT, internal/workspace/tree.go at 46990a1): it watches the
// directories the explorer shows and those of open files, never the whole tree, and gathers
// a burst of events — a checkout, a build — into one batch.

const (
	// WatchSettle is how long the watcher waits after the last event before reporting a batch.
	WatchSettle = 120 * time.Millisecond
	// MaxWatchedDirs bounds the directories watched: kqueue holds a descriptor for every
	// entry of a watched directory, so an unbounded set exhausts the process limit.
	MaxWatchedDirs = 512
)

// Changes is one batch of events, as absolute paths. Dirs gained or lost entries;
// Files were written or created; Gone were removed or renamed away.
type Changes struct {
	Dirs, Files, Gone []string
}

// Watcher reports changes to watched directories. Its callback runs on the watcher's own
// goroutine, never the owner's; Add and Close are safe from any goroutine.
type Watcher struct {
	w       *fsnotify.Watcher
	changed func(Changes)

	mu      sync.Mutex
	watched map[string]bool
	dirs    map[string]bool
	files   map[string]bool
	gone    map[string]bool
	timer   *time.Timer
	closed  bool
}

// NewWatcher starts watching nothing; Add names the directories. A platform without
// file events returns an error, and the editor falls back to checking on focus.
func NewWatcher(changed func(Changes)) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{w: fw, changed: changed, watched: map[string]bool{}}
	go w.loop()
	return w, nil
}

// Add watches dir, unless it is watched already or the bound is reached.
// It reports whether dir is watched afterwards.
func (w *Watcher) Add(dir string) bool {
	if w == nil {
		return false
	}
	dir = filepath.Clean(dir)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return false
	}
	if w.watched[dir] {
		return true
	}
	if len(w.watched) >= MaxWatchedDirs || w.w.Add(dir) != nil {
		return false
	}
	w.watched[dir] = true
	return true
}

// Watched reports how many directories are watched.
func (w *Watcher) Watched() int {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.watched)
}

// Close stops the watcher; no callback starts after it returns.
func (w *Watcher) Close() {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.closed = true
	if w.timer != nil {
		w.timer.Stop()
	}
	w.mu.Unlock()
	w.w.Close()
}

func (w *Watcher) loop() {
	for {
		select {
		case ev, ok := <-w.w.Events:
			if !ok {
				return
			}
			w.event(ev)
		case _, ok := <-w.w.Errors:
			if !ok {
				return
			}
			// An overflow loses events; the focus check still catches what was missed.
		}
	}
}

func (w *Watcher) event(ev fsnotify.Event) {
	name := filepath.Clean(ev.Name)
	created := ev.Has(fsnotify.Create)
	removed := ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename)
	// Chmod is kept: a writer that restores the old modification time still changes attributes.
	written := ev.Has(fsnotify.Write) || ev.Has(fsnotify.Chmod)
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if w.dirs == nil {
		w.dirs, w.files, w.gone = map[string]bool{}, map[string]bool{}, map[string]bool{}
	}
	if created || removed {
		w.dirs[filepath.Dir(name)] = true
	}
	if removed {
		w.gone[name] = true
		delete(w.files, name)
		if w.watched[name] { // A watched directory went away: let a recreated one be watched again.
			delete(w.watched, name)
		}
	}
	if created || written {
		w.files[name] = true
		delete(w.gone, name)
	}
	if w.timer == nil {
		w.timer = time.AfterFunc(WatchSettle, w.flush)
	}
}

func (w *Watcher) flush() {
	w.mu.Lock()
	dirs, files, gone := w.dirs, w.files, w.gone
	w.dirs, w.files, w.gone, w.timer = nil, nil, nil, nil
	closed := w.closed
	w.mu.Unlock()
	if closed || len(dirs)+len(files)+len(gone) == 0 {
		return
	}
	w.changed(Changes{Dirs: keys(dirs), Files: keys(files), Gone: keys(gone)})
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
