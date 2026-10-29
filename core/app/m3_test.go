package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func newFolderEditor(t *testing.T, files map[string]string) (*Editor, string) {
	t.Helper()
	root := t.TempDir()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return newFolderEditorAt(t, root)
}

// newFolderEditorAt 打开已经建好的文件夹。
func newFolderEditorAt(t *testing.T, root string) (*Editor, string) {
	t.Helper()
	e, warns := New(config.Default(), fakeLayouter{}, &fakeHost{}, keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl, keymap.ModSuper}})
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if err := e.OpenFolder(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	e.SetPaneSize(0, view.Size{W: 90, H: 28})
	var f view.Frame
	e.Frame(&f)
	pump(t, e, func() bool { return !e.ws.indexing })
	return e, root
}

// pump 像前端事件循环一样处理后台 Post，直到 cond 成立。
func pump(t *testing.T, e *Editor, cond func() bool) {
	t.Helper()
	// Generous: git-backed waits share the machine with the whole parallel suite under make check.
	deadline := time.After(10 * time.Second)
	for !cond() {
		select {
		case <-e.Wake():
			e.RunPending()
		case <-deadline:
			t.Fatal("timed out waiting for background work")
		}
	}
}

func typeText(e *Editor, s string) { e.Handle(TextInput{Text: s}) }

func TestQuickOpenFuzzyInBackground(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{
		"core/edit/edit.go": "package edit\n", "core/app/editor.go": "package app\n", "README.md": "# hi\n",
	})
	e.Handle(ctrl('p'))
	if e.ov == nil || e.ov.kind != view.OverlayFinder {
		t.Fatal("Ctrl+P should open quick open")
	}
	typeText(e, "edit")
	pump(t, e, func() bool { return len(e.ov.items) > 0 })
	if e.ov.items[0].label != "edit.go" {
		t.Fatalf("top hit = %q", e.ov.items[0].label)
	}
	e.Handle(key(keymap.KeyEnter, 0))
	if e.ov != nil || e.cur() == nil || e.cur().doc.buf.Path() != filepath.Join(root, "core/edit/edit.go") {
		t.Fatal("Enter should open the file and close the overlay")
	}
	// 空查询列出最近打开
	e.Handle(ctrl('p'))
	if len(e.ov.items) != 1 || e.ov.items[0].label != "edit.go" {
		t.Fatalf("recent = %+v", e.ov.items)
	}
	e.Handle(key(keymap.KeyEsc, 0))
	if e.ov != nil {
		t.Fatal("Esc should close")
	}
}

func TestGotoLineWithChineseColon(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "1\n2\n3\n4\n5\n"})
	e.OpenFile(filepath.Join(root, "a.txt"))
	e.Handle(ctrl('g'))
	typeText(e, "4") // Ctrl+G 预填了 ":"
	e.Handle(key(keymap.KeyEnter, 0))
	s := e.cur().doc.buf.Snapshot()
	if got := s.LineAt(e.cur().doc.buf.Selections().Main().Head); got != 3 {
		t.Fatalf("line = %d, want 3", got)
	}
	e.Handle(ctrl('p'))
	typeText(e, "：2") // 中文输入法打出的全角冒号
	e.Handle(key(keymap.KeyEnter, 0))
	if got := s.LineAt(e.cur().doc.buf.Selections().Main().Head); got != 1 {
		t.Fatalf("line = %d, want 1", got)
	}
}

func TestPaletteSearchesChineseTitles(t *testing.T) {
	e, _ := newFolderEditor(t, map[string]string{"a.txt": ""})
	e.Handle(ctrl('k'))
	e.Handle(TextInput{Text: "p"})
	if e.ov == nil || e.ov.kind != view.OverlayPalette {
		t.Fatal("Ctrl+K P should open the palette")
	}
	typeText(e, "侧边栏")
	if len(e.ov.items) == 0 || e.ov.items[0].detail != "workbench.action.toggleSidebarVisibility" {
		t.Fatalf("items = %+v", e.ov.items)
	}
	if e.ov.items[0].hint != "Ctrl+B" {
		t.Fatalf("hint = %q", e.ov.items[0].hint)
	}
	e.Handle(key(keymap.KeyEnter, 0))
	if e.sidebar {
		t.Fatal("toggle sidebar should have hidden it")
	}
	// ">" 从快速打开切到命令面板；退格到空再退格回到快速打开
	e.Handle(ctrl('p'))
	typeText(e, "》")
	if e.ov.kind != view.OverlayPalette {
		t.Fatal("'》' should switch to the palette")
	}
}

func TestTabsAndDirtyClose(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a", "b.txt": "b"})
	e.OpenFile(filepath.Join(root, "a.txt"))
	e.OpenFile(filepath.Join(root, "b.txt"))
	if got := strings.Join(tabNames(e), " "); got != "a.txt b.txt*" {
		t.Fatalf("tabs = %s", got)
	}
	e.OpenFile(filepath.Join(root, "a.txt")) // 已打开：只切换
	if got := strings.Join(tabNames(e), " "); got != "a.txt* b.txt" {
		t.Fatalf("reopening should switch, not duplicate: %s", got)
	}
	typeText(e, "x")
	e.Handle(ctrl('w'))
	if len(e.docs) != 2 || !strings.Contains(e.status, "未保存") {
		t.Fatalf("dirty close should warn first: %q", e.status)
	}
	e.Handle(ctrl('w'))
	if len(e.docs) != 1 || filepath.Base(e.cur().doc.buf.Path()) != "b.txt" {
		t.Fatal("second Ctrl+W should close")
	}
}

func TestTreeKeyboardNavigation(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"src/main.go": "package main\n", "README.md": ""})
	e.Handle(ctrl('k'))
	e.Handle(TextInput{Text: "e"}) // Ctrl+K E：聚焦资源管理器
	if e.area != areaTree {
		t.Fatal("explorer should have focus")
	}
	// 可见：src、README.md；选中 src，→ 展开，↓ 到 main.go，Enter 打开
	e.Handle(key(keymap.KeyRight, 0))
	e.Handle(key(keymap.KeyDown, 0))
	e.Handle(key(keymap.KeyEnter, 0))
	if e.cur() == nil || e.cur().doc.buf.Path() != filepath.Join(root, "src/main.go") || e.area != areaEditor {
		t.Fatalf("Enter should open src/main.go and focus the editor")
	}
	// 资源管理器有焦点时打字不会进文件
	e.area = areaTree
	typeText(e, "zzz")
	if content(e) != "package main\n" {
		t.Fatal("typing in the tree must not edit the file")
	}
}

func TestExternalChangeReload(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "old\n"})
	p := filepath.Join(root, "a.txt")
	e.OpenFile(p)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(p, []byte("new content\n"), 0o644)
	os.Chtimes(p, time.Now().Add(time.Second), time.Now().Add(time.Second))
	e.Handle(FocusInput{Gained: true})
	if content(e) != "new content\n" || e.cur().doc.buf.Dirty() {
		t.Fatalf("clean buffer should silently reload: %q", content(e))
	}
	e.Handle(ctrl('z')) // 重载也能撤销
	if content(e) != "old\n" {
		t.Fatalf("undo reload = %q", content(e))
	}
}
