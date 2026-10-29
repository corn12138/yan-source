package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/text"
)

func TestLongLineOpenKeepsCurrentTab(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"keep.txt": "keep\n", "bundle.map": strings.Repeat("x", 26<<20)})
	e.autoSave = false
	if err := e.OpenFile(filepath.Join(root, "keep.txt")); err != nil {
		t.Fatal(err)
	}
	typeText(e, "mine ")
	current, n := e.cur(), len(e.column().tabs)
	for _, open := range []func(string) error{e.OpenFile, func(p string) error { return e.PreviewFileIn(0, p) }} {
		if err := open(filepath.Join(root, "bundle.map")); !errors.Is(err, text.ErrLineTooLong) {
			t.Fatalf("open: %v", err)
		}
		if e.cur() != current || len(e.column().tabs) != n || content(e) != "mine keep\n" || !e.cur().doc.buf.Dirty() {
			t.Fatal("failed open changed current document")
		}
	}
}

func TestLongLineReloadKeepsBufferAndBaseline(t *testing.T) {
	for _, watcher := range []bool{false, true} {
		for _, dirty := range []bool{false, true} {
			t.Run(fmt.Sprintf("watcher=%v/dirty=%v", watcher, dirty), func(t *testing.T) {
				e, root := newFolderEditor(t, map[string]string{"keep.txt": "original\n"})
				e.autoSave = false
				p := filepath.Join(root, "keep.txt")
				if err := e.OpenFile(p); err != nil {
					t.Fatal(err)
				}
				if dirty {
					typeText(e, "mine ")
				}
				before := content(e)
				d := e.cur().doc
				version, stamp := d.buf.Snapshot().Version(), d.buf.DiskStamp()
				external := strings.Repeat("x", text.MaxLineBytes+1)
				externalWrite(t, p, external)
				if watcher {
					e.status = ""
					e.checkDocs([]*document{d})
					pump(t, e, func() bool { return e.status != "" })
				} else {
					e.checkDisk()
				}
				if content(e) != before || d.buf.Snapshot().Version() != version || d.buf.DiskStamp() != stamp || d.buf.Dirty() != dirty {
					t.Fatal("refused reload changed buffer or saved baseline")
				}
				if !dirty && !strings.Contains(e.status, "64 KiB") {
					t.Fatalf("missing explanation: %s", e.status)
				}
				if err := e.revert(); !errors.Is(err, text.ErrLineTooLong) {
					t.Fatalf("revert: %v", err)
				}
				if !dirty {
					typeText(e, "mine ")
				}
				if err := e.saveDoc(d); err == nil {
					t.Fatal("save overwrote refused external version")
				}
				if readFile(t, p) != external {
					t.Fatal("external data lost")
				}
				e.Run("undo")
				if content(e) != "original\n" {
					t.Fatalf("undo lost original: %q", content(e))
				}
				externalWrite(t, p, "recovered\n")
				e.checkDisk()
				if content(e) != "recovered\n" {
					t.Fatal("valid later disk version did not recover")
				}
				e.Run("undo")
				if content(e) != "original\n" {
					t.Fatal("reload is no longer undoable")
				}
			})
		}
	}
}

func TestSessionSkipsLongLineAndKeepsOtherTabs(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"bundle.map": strings.Repeat("x", text.MaxLineBytes+1), "ok.txt": "normal\n"})
	st := config.LoadState(filepath.Join(t.TempDir(), "state.json"))
	st.Sessions = map[string]config.Session{root: {Columns: []config.SessionColumn{{Tabs: []config.SessionTab{{Path: "bundle.map"}, {Path: "ok.txt"}}}}}}
	e.SetState(st)
	if !e.RestoreSession() || e.docOf(filepath.Join(root, "bundle.map")) != nil || len(e.column().tabs) != 1 || content(e) != "normal\n" {
		t.Fatal("session did not keep only the safe tab")
	}
	if !strings.Contains(e.status, "64 KiB") {
		t.Fatalf("missing restore notice: %s", e.status)
	}
	e.SaveSession()
	if got := st.Sessions[root].Columns[0].Tabs; len(got) != 1 || got[0].Path != "ok.txt" {
		t.Fatal("refused file persisted in session")
	}
}
