package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

func TestSessionRestore(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "one\ntwo\nthree\n", "b.txt": "b"})
	statePath := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(statePath))
	e.OpenFile(filepath.Join(root, "a.txt"))
	e.Handle(key(keymap.KeyDown, 0))
	e.Run("workbench.action.splitEditor")
	e.OpenFile(filepath.Join(root, "b.txt"))
	e.ResizeColumns(1, 0.2, 0.1) // 左边那一列宽一些：也记下
	e.SaveSession()

	e2, _ := newFolderEditorAt(t, root)
	e2.SetState(config.LoadState(statePath))
	if !e2.RestoreSession() {
		t.Fatal("no session restored")
	}
	if e2.Columns() != 2 || strings.Join(tabNames(e2), " ") != "a.txt b.txt*" {
		t.Fatalf("columns %d tabs %v", e2.Columns(), tabNames(e2))
	}
	if w0, w1 := e2.cols[0].weight, e2.cols[1].weight; w0 < 0.69 || w0 > 0.71 || w1 < 0.29 || w1 > 0.31 {
		t.Fatalf("column widths %v %v", w0, w1)
	}
	e2.FocusColumn(0)
	if s := e2.cur().doc.buf.Snapshot(); s.LineAt(e2.cur().doc.buf.Selections().Main().Head) != 1 {
		t.Fatal("caret not restored")
	}
}

func TestAutoSaveAndThemes(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.txt")
	os.WriteFile(p, []byte("a\n"), 0o644)
	e, _ := New(config.Default(), fakeLayouter{}, &fakeHost{}, keymap.Profile{Mods: []keymap.Mod{keymap.ModSuper}, GUI: true})
	e.SetPaneSize(0, view.Size{W: 80, H: 20})
	e.SetState(config.LoadState(filepath.Join(t.TempDir(), "state.json")))
	e.OpenFile(p)
	if !e.AutoSave() {
		t.Fatal("the window version saves on its own by default")
	}
	typeText(e, "x")
	pump(t, e, func() bool { b, _ := os.ReadFile(p); return string(b) == "xa\n" })
	// 换主题、放大：前端看 gen
	_, _, _, gen := e.Settings()
	e.Run("yan.theme.claude")
	e.Run("workbench.action.zoomIn")
	theme, _, size, gen2 := e.Settings()
	if theme != "claude" || size != config.Default().FontSize+1 || gen2 == gen {
		t.Fatalf("theme %q size %v gen %d→%d", theme, size, gen, gen2)
	}
	// 保存配置文件立即生效：项目里的 .yan.toml 把 Ctrl+K K 绑到删除行
	e.SetRoot(root)
	cfgPath := filepath.Join(root, ".yan.toml")
	os.WriteFile(cfgPath, nil, 0o644)
	e.OpenFile(cfgPath)
	typeText(e, "[keys.editor]\n\"ctrl+k k\" = \"editor.action.deleteLines\"\n")
	e.Run("workbench.action.files.save")
	if !strings.Contains(e.status, "配置已生效") {
		t.Fatalf("status = %q", e.status)
	}
	if _, m := e.keys.Resolve(keymap.CtxEditor, []keymap.Chord{{Mods: keymap.ModCtrl, Key: keymap.KeyRune, Rune: 'k'}}); m != keymap.MatchPrefix {
		t.Fatal("new binding not compiled")
	}
}

func TestSwitchFolder(t *testing.T) {
	e, a := newFolderEditor(t, map[string]string{"a.txt": "a", "README.md": "# a"})
	statePath := filepath.Join(t.TempDir(), "state.json")
	e.SetState(config.LoadState(statePath))
	e.OpenFile(filepath.Join(a, "a.txt"))
	b := t.TempDir()
	os.WriteFile(filepath.Join(b, "README.md"), []byte("# b"), 0o644)
	if err := e.SwitchFolder(b); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tabNames(e), " "); got != "README.md*" || e.Root() != b {
		t.Fatalf("tabs %s root %s", got, e.Root())
	}
	// 回到 a：上次的标签回来了；最近打开里有 b
	if err := e.SwitchFolder(a); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tabNames(e), " "); got != "a.txt*" {
		t.Fatalf("tabs %s", got)
	}
	if r := e.RecentFolders(); len(r) != 1 || r[0] != b {
		t.Fatalf("recent = %v", r)
	}
	// 有没存的修改时不换
	typeText(e, "x")
	if err := e.SwitchFolder(b); err == nil {
		t.Fatal("dirty files should block switching folders")
	}
}

func TestThemeActivity(t *testing.T) {
	e, _ := newFolderEditor(t, nil)
	defer e.Close()
	if e.BackgroundActivity() != "" {
		t.Fatal("idle editor reports activity")
	}
	e.fs.running = true
	if e.BackgroundActivity() == "" {
		t.Fatal("hidden search missing from activity")
	}
	e.git.busy = "正在提交…"
	if e.BackgroundActivity() != e.git.busy {
		t.Fatal("Git activity should take precedence")
	}
	e.git.busy, e.fs.running = "", false
	if e.BackgroundActivity() != "" {
		t.Fatal("finished activity remained")
	}
}
