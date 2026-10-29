package gui

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
	"github.com/corn12138/yan-source/core/view"
)

// harness 是不开窗口的 GUI：MyGo 的 Tester 在内存里画帧、点击、打字。
type harness struct {
	t    *testing.T
	f    *Frontend
	ed   *app.Editor
	tt   *ui.Tester
	root string
}

func newHarness(t *testing.T, files map[string]string, opts Options) *harness {
	t.Helper()
	root := t.TempDir()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return newHarnessAt(t, root, opts)
}

func newHarnessAt(t *testing.T, root string, opts Options) *harness {
	t.Helper()
	f := New(opts)
	cfg := config.Default()
	off := false
	cfg.AutoSave = &off // 截图和断言要看「未保存」的样子
	cfg.Theme, cfg.Appearance = opts.Theme, opts.Appearance
	ed, warns := app.New(cfg, f.Layouter(), f.Host(), f.Profile())
	if len(warns) > 0 {
		t.Fatalf("warnings: %v", warns)
	}
	f.ed = ed
	t.Cleanup(ed.Close)
	if err := ed.OpenFolder(root); err != nil {
		t.Fatal(err)
	}
	h := &harness{t: t, f: f, ed: ed, root: root}
	h.tt = ui.NewTester(f.view, 1200, 780)
	h.settle()
	return h
}

// settle 处理后台 Post 的结果，再画几帧，直到界面稳定。
func (h *harness) settle() {
	deadline := time.Now().Add(2 * time.Second)
	for {
		h.tt.Frame()
		select {
		case <-h.ed.Wake():
			h.ed.RunPending()
			continue
		default:
		}
		if time.Now().After(deadline) {
			return
		}
		select {
		case <-h.ed.Wake():
			h.ed.RunPending()
		case <-time.After(60 * time.Millisecond):
			h.tt.Frame()
			h.tt.Frame()
			return
		}
	}
}

// shot 在 SHOTS 指定的目录里存一张截图。
func (h *harness) shot(name string) {
	dir := os.Getenv("SHOTS")
	if dir == "" {
		return
	}
	out, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		h.t.Fatal(err)
	}
	defer out.Close()
	png.Encode(out, h.tt.Image())
}

func (h *harness) open(rel string) {
	h.t.Helper()
	if err := h.ed.OpenFile(filepath.Join(h.root, rel)); err != nil {
		h.t.Fatal(err)
	}
	h.settle()
}

var sample = map[string]string{
	"README.md": "# 你好，砚\n\n一个极简的编辑器。\n",
	"main.go": `package main

import "fmt"

// main 打个招呼。
func main() {
	for i := 0; i < 3; i++ {
		fmt.Println("你好", i)
	}
}
`,
	"core/edit/edit.go": "package edit\n",
	"core/app/app.go":   "package app\n",
}

func TestTypingGoesToTheEditor(t *testing.T) {
	h := newHarness(t, sample, Options{Appearance: "dark"})
	h.open("main.go")
	h.tt.Type("// 中文 ok\n")
	h.settle()
	b, _ := os.ReadFile(filepath.Join(h.root, "main.go"))
	_ = b
	var got string
	for _, txt := range h.tt.Texts() {
		got += txt + "\n"
	}
	h.shot("typing")
	if !h.f.frame.Columns[0].Tabs[0].Dirty {
		t.Fatalf("typing should edit main.go; texts:\n%s", got)
	}
}

func TestQuickOpenFindsFiles(t *testing.T) {
	h := newHarness(t, sample, Options{Appearance: "dark"})
	h.tt.Key(ui.Cmd, ui.KeyP)
	h.settle()
	if !h.f.frame.Overlay.Visible {
		t.Fatal("⌘P should open quick open")
	}
	h.tt.Type("edit")
	h.settle()
	h.shot("quickopen")
	if len(h.f.frame.Overlay.Items) == 0 || h.f.frame.Overlay.Items[0].Label != "edit.go" {
		t.Fatalf("items = %+v", h.f.frame.Overlay.Items)
	}
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if h.f.frame.Overlay.Visible {
		t.Fatal("Enter should close quick open")
	}
	c := h.f.frame.Columns[0]
	if c.Active < 0 || c.Tabs[c.Active].Name != "edit.go" {
		t.Fatalf("tabs = %+v", c.Tabs)
	}
	// 工作台拿回键盘：打字进了 edit.go。
	h.tt.Type("x")
	h.settle()
	if !h.f.frame.Columns[0].Tabs[c.Active].Dirty {
		t.Fatal("after quick open, typing should go to the editor")
	}
}

func TestExplorerClickPreviews(t *testing.T) {
	h := newHarness(t, sample, Options{Appearance: "light"})
	if err := h.tt.Click("main.go"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	c := h.f.frame.Columns[0]
	if len(c.Tabs) != 1 || c.Tabs[0].Name != "main.go" || !c.Tabs[0].Preview {
		t.Fatalf("tabs = %+v", c.Tabs)
	}
	h.shot("explorer-light")
}

func TestShots(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("SHOTS=目录 时才截图")
	}
	for _, theme := range []struct{ name, theme, appearance string }{
		{"mayoi-dark", "", "dark"}, {"mayoi-light", "", "light"}, {"claude-dark", "claude", "dark"},
		{"claude-light", "claude", "light"}, {"matrix", "matrix", ""},
	} {
		h := newHarness(t, sample, Options{Theme: theme.theme, Appearance: theme.appearance})
		h.open("README.md")
		h.open("main.go")
		h.ed.Run("workbench.action.splitEditor")
		h.settle()
		h.ed.Run("workbench.action.openEditorAtIndex1")
		h.settle()
		h.shot("theme-" + theme.name)
		if !strings.Contains(strings.Join(h.tt.Texts(), " "), "main.go") {
			t.Fatalf("%s: no main.go tab", theme.name)
		}
	}
}

func TestShotDecorations(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("SHOTS=目录 时才截图")
	}
	h := newHarness(t, sample, Options{Appearance: "dark"})
	h.open("main.go")
	h.ed.GotoLine(7) // for 循环：光标在 for 上，参考线、同词 i、括号
	h.tt.Key(0, ui.KeyEnd)
	h.settle()
	h.shot("decorations")
}

func TestFindWidget(t *testing.T) {
	h := newHarness(t, sample, Options{Appearance: "dark"})
	h.open("main.go")
	h.tt.Key(ui.Cmd, ui.KeyF)
	h.settle()
	if !h.f.frame.Find.Visible {
		t.Fatal("⌘F should open find")
	}
	h.tt.Type("i")
	h.settle()
	if h.f.frame.Find.Query != "i" || h.f.frame.Find.Count == "" {
		t.Fatalf("find = %+v", h.f.frame.Find)
	}
	h.tt.Key(ui.Alt|ui.Cmd, ui.KeyF) // 打开替换
	h.settle()
	h.shot("find")
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	if h.f.frame.Find.Visible {
		t.Fatal("Esc should close find")
	}
	h.tt.Type("z") // 键盘回到编辑器
	h.settle()
	if !h.f.frame.Columns[0].Tabs[0].Dirty {
		t.Fatal("after closing find, typing should edit the file")
	}
}

func TestShotGit(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("SHOTS=目录 时才截图")
	}
	root := t.TempDir()
	for p, c := range sample {
		full := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(c), 0o644)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=砚", "-c", "user.email=t@t", "commit", "-q", "-m", "第一次提交"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("todo\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.open("main.go")
	h.ed.GotoLine(8)
	h.tt.Key(0, ui.KeyEnd)
	h.tt.Type(" // 改")
	h.tt.Key(0, ui.KeyEnter)
	h.tt.Type("fmt.Println()")
	h.tt.Key(ui.Cmd, ui.KeyS)
	h.ed.GotoLine(3)
	for i := 0; i < 8; i++ {
		time.Sleep(120 * time.Millisecond)
		h.settle()
	}
	h.shot("git")
}

func TestSearchAndSCMViews(t *testing.T) {
	root := t.TempDir()
	for p, c := range sample {
		full := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(c), 0o644)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=砚", "-c", "user.email=t@t", "commit", "-q", "-m", "第一次提交"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(root, "main.go"), []byte(sample["main.go"]+"// 新的一行\n"), 0o644)
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("你好 todo\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	h.settle()
	h.tt.Type("你好")
	for i := 0; i < 6; i++ {
		time.Sleep(60 * time.Millisecond)
		h.settle()
	}
	sr := h.f.frame.Sidebar.Search
	if h.f.frame.Sidebar.View != view.SideSearch || !strings.Contains(sr.Summary, "个结果") {
		t.Fatalf("search = %+v", sr)
	}
	h.shot("search")
	h.tt.Key(ui.Ctrl|ui.Shift, ui.KeyG)
	for i := 0; i < 6; i++ {
		time.Sleep(60 * time.Millisecond)
		h.settle()
	}
	scm := h.f.frame.Sidebar.SCM
	if h.f.frame.Sidebar.View != view.SideSCM || len(scm.Groups) != 2 {
		t.Fatalf("scm = %+v", scm)
	}
	h.shot("scm")
	h.ed.SCMClick(0, false) // 点改动的文件：看它的改动
	h.settle()
	h.shot("diff")

	if c := h.f.frame.Columns[0]; len(c.Tabs) != 1 || c.Tabs[0].Name != "main.go (改动)" {
		t.Fatalf("tabs = %+v", c.Tabs)
	}
}

func TestShotSticky(t *testing.T) {
	if os.Getenv("SHOTS") == "" {
		t.Skip("SHOTS=目录 时才截图")
	}
	src, _ := os.ReadFile("../../core/app/editing.go")
	h := newHarness(t, map[string]string{"editing.go": string(src)}, Options{Appearance: "dark"})
	h.open("editing.go")
	at := strings.Count(string(src[:strings.Index(string(src), "for _, bl := range blocks(s, sels)")]), "\n")
	h.ed.GotoLine(at + 6)
	h.settle()
	h.ed.Handle(app.ScrollInput{Column: 0, DY: 14})
	h.settle()
	h.shot("sticky")
}

func TestImageViewer(t *testing.T) {
	root := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 160, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 160; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y * 2), 200, uint8(255 - x)})
		}
	}
	out, _ := os.Create(filepath.Join(root, "pic.png"))
	png.Encode(out, img)
	out.Close()
	os.WriteFile(filepath.Join(root, "logo.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="96" height="96"><circle cx="12" cy="12" r="10" fill="#d97757"/></svg>`), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.open("pic.png")
	for i := 0; i < 5; i++ {
		time.Sleep(60 * time.Millisecond)
		h.settle()
	}
	if p := h.f.frame.Columns[0].Pane; p.Kind != view.PaneImage || h.f.images[p.Path] == nil || h.f.images[p.Path].bmp == nil {
		t.Fatalf("image not shown: %+v", p.Kind)
	}
	h.shot("image")
	h.open("logo.svg")
	h.ed.Run("yan.svg.togglePreview")
	h.settle()
	if p := h.f.frame.Columns[0].Pane; p.Kind != view.PaneImage || p.SVG == nil {
		t.Fatalf("svg preview: %+v", p.Kind)
	}
	h.shot("svg")
}

func TestVimInWindow(t *testing.T) {
	h := newHarness(t, sample, Options{Appearance: "dark"})
	h.open("main.go")
	h.ed.Run("yan.toggleVim")
	h.settle()
	h.tt.TypeKey(0, ui.KeyJ, "j")
	h.tt.TypeKey(0, ui.KeyJ, "j")
	h.tt.TypeKey(0, ui.KeyD, "d")
	h.tt.TypeKey(0, ui.KeyD, "d")
	h.settle()
	if p := h.f.frame.Columns[0].Pane; !p.Caret.Block || p.Rows[2].Text == nil || string(p.Rows[2].Text) != "" {
		t.Fatalf("after jjdd: caret %+v row2 %q", p.Caret, p.Rows[2].Text)
	}
	h.tt.Key(ui.Ctrl, ui.KeyR) // 普通模式里 ⌃R 是重做，不是打开最近
	h.settle()
	if h.f.frame.Overlay.Visible {
		t.Fatal("⌃R opened the recent folders")
	}
	h.shot("vim")
}

// 窗口获得、失去焦点要传到内核：回来时重载外面改过的文件，有冲突时两边都留着；离开时自动保存。
func TestWindowFocusReloadsAndKeepsConflicts(t *testing.T) {
	h := newHarness(t, map[string]string{"a.txt": "base\n", "b.txt": "b\n"}, Options{Appearance: "dark"})
	pa, pb := filepath.Join(h.root, "a.txt"), filepath.Join(h.root, "b.txt")
	later := func(p, s string) {
		os.WriteFile(p, []byte(s), 0o644)
		at := time.Now().Add(2 * time.Second)
		os.Chtimes(p, at, at)
	}
	shows := func(s string) bool {
		c := h.f.frame.Columns[h.f.frame.Focus]
		for _, r := range c.Pane.Rows {
			if strings.Contains(string(r.Text), s) {
				return true
			}
		}
		return false
	}
	h.open("a.txt")
	later(pa, "from outside\n")
	h.f.focusChanged(true)
	h.settle()
	if !shows("from outside") || h.f.frame.Columns[0].Tabs[0].Dirty {
		t.Fatal("a clean buffer should reload when the window regains focus")
	}

	h.ed.Run("workbench.action.toggleAutoSave") // 测试默认关着
	h.open("b.txt")
	h.tt.Type("mine ")
	h.settle()
	later(pb, "theirs\n")
	h.f.focusChanged(true)
	h.f.focusChanged(false)
	h.settle()
	if b, _ := os.ReadFile(pb); string(b) != "theirs\n" {
		t.Fatalf("blur auto-save overwrote an external change: %q", b)
	}
	if !shows("mine b") || !h.f.frame.Columns[0].Tabs[1].Dirty {
		t.Fatal("the unsaved edit must stay in the editor")
	}

	h.open("a.txt")
	h.tt.Type("ok ")
	h.f.focusChanged(false)
	h.settle()
	if b, _ := os.ReadFile(pa); !strings.Contains(string(b), "ok") || h.f.frame.Columns[0].Tabs[0].Dirty {
		t.Fatalf("blur should auto-save a clean-on-disk file: %q", b)
	}
}

// 中文输入法：组字时不改文档；提交是一步撤销；Esc 取消不留字；有选区时替换选区；
// 组字途中换了标签，提交的字落进开始组字的那个文件。（这里模拟的是 MyGo 送来的组字事件，不是系统的拼音输入法。）
func TestInputMethodComposition(t *testing.T) {
	h := newHarness(t, map[string]string{"a.txt": "ab\n", "b.txt": "b\n"}, Options{Appearance: "dark"})
	line := func(col int) string {
		if rows := h.f.frame.Columns[col].Pane.Rows; len(rows) > 0 {
			return string(rows[0].Text)
		}
		return ""
	}
	h.open("a.txt")
	h.tt.Compose("ni", 2)
	h.tt.Compose("nihao", 5)
	h.settle()
	if line(0) != "ab" {
		t.Fatalf("composition must not edit the document: %q", line(0))
	}
	if h.f.frame.Columns[0].Tabs[0].Dirty {
		t.Fatal("composition made the document dirty")
	}
	h.tt.Type("你好")
	h.tt.Compose("shijie", 6)
	h.tt.Type("世界")
	h.settle()
	if line(0) != "你好世界ab" {
		t.Fatalf("after two commits: %q", line(0))
	}
	h.tt.Command("undo")
	h.settle()
	if line(0) != "你好ab" {
		t.Fatalf("one undo should remove exactly one composition: %q", line(0))
	}
	h.tt.Command("undo")
	h.settle()
	if line(0) != "ab" {
		t.Fatalf("second undo: %q", line(0))
	}

	// Esc：输入法自己取消组字（送来空的组字），什么都不插
	h.tt.Compose("zhong", 5)
	h.tt.Compose("", 0)
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	if line(0) != "ab" || h.f.preedit != "" {
		t.Fatalf("cancelled composition left text: %q preedit %q", line(0), h.f.preedit)
	}

	// 有选区时提交替换选区
	h.ed.Run("editor.action.selectAll")
	h.tt.Compose("zhong", 5)
	h.tt.Type("中")
	h.settle()
	if line(0) != "中" {
		t.Fatalf("commit should replace the selection: %q", line(0))
	}

	// 组字途中换到 b.txt：提交的字留在 a.txt
	h.tt.Compose("wen", 3)
	h.open("b.txt")
	h.tt.Type("文")
	h.settle()
	if line(0) != "b" {
		t.Fatalf("committed text landed in the wrong file: b.txt = %q", line(0))
	}
	h.open("a.txt")
	if line(0) != "中文" {
		t.Fatalf("a.txt = %q", line(0))
	}
}

// 状态栏右边有版本标签（测试里不是打了标签的构建，所以是「开发版」）。
func TestVersionBadge(t *testing.T) {
	for _, look := range []string{"dark", "light"} {
		h := newHarness(t, sample, Options{Appearance: look})
		h.open("main.go")
		if h.f.badge != "开发版" || !h.tt.HasText("开发版") {
			t.Fatalf("%s: badge %q not shown", look, h.f.badge)
		}
		h.shot("badge-" + look)
	}
}

// 触控板横着划：每一下都带点向下，文字只横着走，不往下漂；不足一格的也攒着，慢慢划也能滚动。
func TestSwipeSidewaysAlongItsAxis(t *testing.T) {
	var src strings.Builder
	for range 200 {
		src.WriteString(strings.Repeat("long line ", 40) + "\n")
	}
	h := newHarness(t, map[string]string{"long.txt": src.String()}, Options{})
	h.open("long.txt")
	top := h.f.frame.Columns[0].Pane.Rows[0].Line
	step := h.f.font.charW * 0.4 // 一下不足一格
	for range 20 {
		h.f.paneInput(0, ui.InputEvent{Kind: ui.InputScroll, DX: step, DY: step / 2})
	}
	h.settle()
	pf := h.f.frame.Columns[0].Pane
	if pf.ScrollX < 7 {
		t.Fatalf("ScrollX = %d, want about 8", pf.ScrollX)
	}
	if pf.Rows[0].Line != top {
		t.Fatalf("first line %d, want %d: the swipe drifted down", pf.Rows[0].Line, top)
	}
	// 竖着划带点横向，也不横着漂。
	for range 10 {
		h.f.paneInput(0, ui.InputEvent{Kind: ui.InputScroll, DX: -h.f.font.charW, DY: h.f.font.lineH * 3})
	}
	h.settle()
	if got := h.f.frame.Columns[0].Pane; got.ScrollX != pf.ScrollX || got.Rows[0].Line == top {
		t.Fatalf("vertical swipe: ScrollX %d (was %d), first line %d", got.ScrollX, pf.ScrollX, got.Rows[0].Line)
	}
}

// 窗口版的键位环境跟着系统：macOS 是 Cmd，Windows、Linux 是 Ctrl，提示也跟着写。
func TestGUIProfileFollowsTheSystem(t *testing.T) {
	for goos, want := range map[string]keymap.Mod{"darwin": keymap.ModSuper, "windows": keymap.ModCtrl, "linux": keymap.ModCtrl} {
		p := guiProfile(goos)
		if !p.GUI || len(p.Mods) != 1 || p.Mods[0] != want || p.MacKeys() != (goos == "darwin") {
			t.Errorf("%s: %+v", goos, p)
		}
	}
}

// 源代码管理里的提交历史：合并画成两道，点一个提交打开它的浮层。
func TestCommitGraphView(t *testing.T) {
	root := t.TempDir()
	day := 0
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=砚", "-c", "user.email=t@t"}, args...)...)
		cmd.Dir = root
		day++
		date := fmt.Sprintf("2030-01-%02dT12:00:00Z", day)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(name, s string) { os.WriteFile(filepath.Join(root, name), []byte(s), 0o644) }
	git("init", "-q", "-b", "main")
	write("main.go", sample["main.go"])
	git("add", ".")
	git("commit", "-q", "-m", "第一次提交")
	git("switch", "-q", "-c", "feature")
	write("feature.go", "package main\n")
	git("add", ".")
	git("commit", "-q", "-m", "加一个功能")
	git("switch", "-q", "main")
	write("main.go", sample["main.go"]+"// 改\n")
	git("commit", "-q", "-am", "改 main.go")
	git("tag", "v0.1")
	git("merge", "-q", "--no-ff", "-m", "合并 feature", "feature")

	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("workbench.view.scm")
	for i := 0; i < 20 && len(h.f.frame.Sidebar.SCM.Commits) < 4; i++ {
		time.Sleep(60 * time.Millisecond)
		h.settle()
	}
	cs := h.f.frame.Sidebar.SCM.Commits
	if len(cs) != 4 || !cs[0].Merge || !cs[0].Head || h.f.frame.Sidebar.SCM.Lanes != 2 {
		t.Fatalf("commits = %+v", cs)
	}
	h.shot("history")
	if err := h.tt.Click("加一个功能"); err != nil {
		t.Fatal(err)
	}
	// 点一个提交：预览标签里是它的说明和它改的文件
	rows := func() string {
		var shown []string
		for _, r := range h.f.frame.Columns[0].Pane.Rows {
			shown = append(shown, string(r.Text))
		}
		return strings.Join(shown, "\n")
	}
	for i := 0; i < 20 && !strings.Contains(rows(), "feature.go  +"); i++ {
		time.Sleep(60 * time.Millisecond)
		h.settle()
	}
	h.shot("commit-tab")
	if !strings.HasPrefix(h.f.frame.Columns[0].Tabs[0].Name, "提交 ") {
		t.Fatalf("tabs %+v", h.f.frame.Columns[0].Tabs)
	}
	if all := rows(); !strings.Contains(all, "加一个功能") || !strings.Contains(all, "feature.go  +") {
		t.Fatalf("commit tab rows:\n%s", all)
	}
	// 右键的「检出、合并…」是原来的浮层
	for i, cm := range h.f.frame.Sidebar.SCM.Commits {
		if cm.Subject == "加一个功能" {
			h.ed.SCMCommitActions(i) // 没有改动：提交历史就是头几行
		}
	}
	for i := 0; i < 20 && !h.f.frame.Overlay.Visible; i++ {
		time.Sleep(60 * time.Millisecond)
		h.settle()
	}
	if o := h.f.frame.Overlay; !o.Visible || o.Kind != view.OverlayChoice || o.Items[0].Label != "feature.go" {
		t.Fatalf("overlay = %+v", o)
	}
	h.shot("commit")
}

// Markdown 预览：⇧⌘V 排好显示；同一个文件在旁边一列边改边更新；点本地链接在砚里打开那个文件。
func TestMarkdownPreviewPane(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for x := 0; x < 64; x++ {
		for y := 0; y < 32; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), 120, 200, 255})
		}
	}
	var pic strings.Builder
	png.Encode(&pic, img)
	md := "# 砚的说明\n\n一段 **粗体**、*斜体* 和 `代码`，还有 [main.go](main.go)。\n\n- 一\n- [x] 二\n\n> 引用\n\n" +
		"```go\nfunc main() {}\n```\n\n| 左 | 右 |\n| :-- | --: |\n| a | b |\n\n![图](pic.png)\n"
	h := newHarness(t, map[string]string{"README.md": md, "main.go": sample["main.go"], "pic.png": pic.String()}, Options{Appearance: "dark"})
	h.open("README.md")
	h.tt.Key(ui.Cmd|ui.Shift, ui.KeyV)
	path := filepath.Join(h.root, "README.md")
	loaded := func() bool { m := h.f.markdowns[path]; return m != nil && m.loaded }
	for i := 0; i < 30 && !loaded(); i++ {
		time.Sleep(30 * time.Millisecond)
		h.settle()
	}
	if p := h.f.frame.Columns[0].Pane; p.Kind != view.PaneMarkdown || !loaded() {
		t.Fatalf("pane %v, loaded %v", p.Kind, loaded())
	}
	m := h.f.markdowns[path]
	var heading, image bool
	for _, b := range m.blocks {
		heading = heading || b.kind == markdownHeading && markdownRunsText(b.runs) == "砚的说明"
		image = image || b.kind == markdownImageBlock && b.runs[0].bitmap != nil
	}
	if !heading || !image {
		t.Fatalf("blocks %+v", m.blocks)
	}
	h.shot("markdown")
	h.tt.Type("zz") // 预览里打字不改看不见的文字
	h.settle()
	if p := h.f.frame.Columns[0].Pane; strings.Contains(string(p.Markdown), "zz") || !strings.Contains(h.f.frame.Status.Message, "预览里不能编辑") {
		t.Fatalf("typed into the preview; status %q", h.f.frame.Status.Message)
	}

	// 旁边一列开着文字：在那里打字，预览跟着变
	h.ed.Run("workbench.action.splitEditor")
	h.settle()
	h.ed.GotoLine(1)
	h.tt.Type("新的第一行\n\n")
	for i := 0; i < 30 && markdownRunsText(h.f.markdowns[path].blocks[0].runs) != "新的第一行"; i++ {
		time.Sleep(40 * time.Millisecond)
		h.settle()
	}
	if got := markdownRunsText(h.f.markdowns[path].blocks[0].runs); got != "新的第一行" {
		t.Fatalf("preview did not follow the edit: %q", got)
	}

	// 回到预览那一列，点本地链接
	h.ed.FocusColumn(0)
	h.settle()
	if err := h.tt.Click("打开链接 main.go"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if c := h.f.frame.Columns[h.f.frame.Active]; c.Tabs[c.Active].Name != "main.go" {
		t.Fatalf("link opened %+v", c.Tabs)
	}
}

// 空文件夹的资源管理器说它是空的，按钮新建文件。
func TestEmptyFolderExplorer(t *testing.T) {
	h := newHarness(t, map[string]string{}, Options{})
	h.ed.Run("workbench.view.explorer")
	h.settle()
	if !strings.Contains(strings.Join(h.tt.Texts(), " "), "这个文件夹是空的") {
		t.Fatalf("texts %q", h.tt.Texts())
	}
	if err := h.tt.Click("新建文件…"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if o := h.f.frame.Overlay; !o.Visible || o.Kind != view.OverlayPrompt {
		t.Fatalf("overlay %+v", o)
	}
}

// 资源管理器里 ⌘C、⌘V 走窗口版的文件剪贴板（不开窗口时记在前端）。
func TestExplorerCopyPasteThroughTheHost(t *testing.T) {
	h := newHarness(t, map[string]string{"a.txt": "a\n"}, Options{})
	h.ed.Run("workbench.view.explorer")
	h.settle()
	if err := h.tt.Click("a.txt"); err != nil {
		t.Fatal(err)
	}
	h.ed.FocusSidebar()
	h.tt.Key(ui.Cmd, ui.KeyC)
	if len(h.f.copiedFiles) != 1 || filepath.Base(h.f.copiedFiles[0]) != "a.txt" {
		t.Fatalf("copied %q", h.f.copiedFiles)
	}
	h.tt.Key(ui.Cmd, ui.KeyV)
	h.settle()
	if _, err := os.Stat(filepath.Join(h.root, "a copy.txt")); err != nil {
		t.Fatalf("paste: %v (status %q)", err, h.f.frame.Status.Message)
	}
}

// 窄窗口：状态栏先去掉编码，位置留着；命令中心先不写文件夹（照着 Mayoi v0.8.0）。
func TestNarrowWindowStatusAndCenter(t *testing.T) {
	h := newHarness(t, map[string]string{"main.go": sample["main.go"]}, Options{})
	h.open("main.go")
	texts := func(tt *ui.Tester) string {
		for range 3 {
			tt.Frame() // 第一帧量出宽度，下一帧按它排
		}
		return strings.Join(tt.Texts(), "|")
	}
	if wide := texts(h.tt); !strings.Contains(wide, "UTF-8") || !strings.Contains(wide, filepath.Base(h.root)) {
		t.Fatalf("wide window: %s", wide)
	}
	narrow := ui.NewTester(h.f.view, 380, 600) // 比最小的窗口还窄：每一项都不够放
	got := texts(narrow)
	if strings.Contains(got, "UTF-8") || !strings.Contains(got, "行 1") {
		t.Fatalf("narrow status bar: %s", got)
	}

	// 命令中心：文件名加文件夹名放不下（最窄的窗口是 480），就先不写文件夹；放得下时两个都写（上面的 h）
	root := filepath.Join(t.TempDir(), "a-rather-long-project-folder-name-that-cannot-fit-beside-the-file")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "main.go"), []byte(sample["main.go"]), 0o644)
	h2 := newHarnessAt(t, root, Options{})
	h2.open("main.go")
	if got := texts(ui.NewTester(h2.f.view, 480, 600)); strings.Contains(got, "main.go|a-rather-long-project-folder-name-that-cannot-fit-beside-the-file|") {
		t.Fatalf("narrow command center kept the folder: %s", got)
	}
}

// 点行号旁的改动标记：下面开一个框，写着 HEAD 里的样子；点「还原」，这一行回到 HEAD 里的样子。
func TestHunkPeekBox(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\nthree\n"), 0o644)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=砚", "-c", "user.email=t@t", "commit", "-q", "-m", "first"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\nTWO changed\nthree\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.open("a.txt")
	for i := 0; i < 30 && h.f.frame.Columns[0].Pane.Rows[1].Mark == view.MarkNone; i++ {
		time.Sleep(50 * time.Millisecond)
		h.settle()
	}
	y := codePadY + 1.5*h.f.font.lineH // 第 2 行
	h.f.paneInput(0, ui.InputEvent{Kind: ui.InputPointerDown, X: 2, Y: y})
	h.f.paneInput(0, ui.InputEvent{Kind: ui.InputPointerUp, X: 2, Y: y})
	h.settle()
	pk := h.f.frame.Columns[0].Pane.Peek
	if pk == nil || strings.Join(pk.Removed, "") != "two" {
		t.Fatalf("peek %+v", pk)
	}
	if !strings.Contains(strings.Join(h.tt.Texts(), "|"), "two") {
		t.Fatalf("box does not show HEAD's line: %q", h.tt.Texts())
	}
	h.shot("hunk-peek")
	if err := h.tt.Click("还原"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if got := string(h.f.frame.Columns[0].Pane.Rows[1].Text); got != "two" {
		t.Fatalf("line 2 after revert: %q", got)
	}
	if h.f.frame.Columns[0].Pane.Peek != nil {
		t.Fatal("box still open after revert")
	}
}

// 源代码管理里「打开所有改动」：一个标签里每个改了的文件一段。
func TestAllChangesTab(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "main.go"), []byte(sample["main.go"]), 0o644)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=砚", "-c", "user.email=t@t", "commit", "-q", "-m", "first"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(root, "main.go"), []byte(strings.Replace(sample["main.go"], "fmt.Println", "fmt.Print", 1)), 0o644)
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("新的笔记\n"), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("workbench.view.scm")
	for i := 0; i < 30 && len(h.f.frame.Sidebar.SCM.Groups) < 2; i++ {
		time.Sleep(50 * time.Millisecond)
		h.settle()
	}
	if err := h.tt.Click("打开所有改动"); err != nil {
		t.Fatal(err)
	}
	var texts string
	rows := func() string { // 代码窗格是画出来的：看内核给的行
		var b strings.Builder
		for _, r := range h.f.frame.Columns[h.f.frame.Active].Pane.Rows {
			b.Write(r.Text)
			b.WriteByte('|')
		}
		return b.String()
	}
	for i := 0; i < 30; i++ {
		time.Sleep(50 * time.Millisecond)
		h.settle()
		if texts = rows(); strings.Contains(texts, "notes.txt  +1 −0  未跟踪") {
			break
		}
	}
	c := h.f.frame.Columns[h.f.frame.Active]
	if c.Tabs[c.Active].Name != "所有改动" || !strings.Contains(texts, "main.go  +1 −1  更改") || !strings.Contains(texts, "notes.txt  +1 −0  未跟踪") {
		t.Fatalf("tab %q, texts %s", c.Tabs[c.Active].Name, texts)
	}
	h.shot("all-changes")
}

// 改动视图并排：左边旧的、右边新的。
func TestSideBySideDiff(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "main.go"), []byte(sample["main.go"]), 0o644)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=砚", "-c", "user.email=t@t", "commit", "-q", "-m", "first"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(root, "main.go"), []byte(strings.Replace(sample["main.go"], "fmt.Println", "fmt.Print", 1)), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.ed.Run("toggle.diff.renderSideBySide") // 自动 → 并排
	if err := h.ed.OpenFile(filepath.Join(root, "main.go")); err != nil {
		t.Fatal(err)
	}
	// Opening a change requires repository discovery; waiting only after the
	// command cannot recover when it was rejected before discovery completed.
	for end := time.Now().Add(15 * time.Second); h.f.frame.Status.Branch != "main" && time.Now().Before(end); {
		h.settle()
	}
	if h.f.frame.Status.Branch != "main" {
		t.Fatal("repository did not become ready")
	}
	h.ed.Run("git.openChange")
	var split string
	// The diff is read by git in the background; under a loaded test run it can take a few frames.
	for end := time.Now().Add(5 * time.Second); !strings.Contains(split, "fmt.Print(") && time.Now().Before(end); {
		h.settle()
		for _, r := range h.f.frame.Columns[0].Pane.Rows {
			if s := string(r.Text); strings.Contains(s, "Println") && strings.Contains(s, " │ ") {
				split = s
			}
		}
	}
	if !strings.Contains(split, "fmt.Print(") {
		t.Fatalf("no side-by-side row for the change: %q", split)
	}
	h.shot("side-by-side")
}

// 窗格右边那一条：改了的行、光标、眼前这一屏；点在它的中间，文件的中间滚到眼前。
func TestOverviewStrip(t *testing.T) {
	root := t.TempDir()
	var b strings.Builder
	for i := range 300 {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	os.WriteFile(filepath.Join(root, "a.txt"), []byte(b.String()), 0o644)
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"}, {"add", "."}, {"-c", "user.name=砚", "-c", "user.email=t@t", "commit", "-q", "-m", "first"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	os.WriteFile(filepath.Join(root, "a.txt"), []byte(strings.Replace(b.String(), "line 200\n", "LINE 200\n", 1)), 0o644)
	h := newHarnessAt(t, root, Options{Appearance: "dark"})
	h.open("a.txt")
	for i := 0; i < 30 && (h.f.frame.Columns[0].Pane.Overview == nil || len(h.f.frame.Columns[0].Pane.Overview.Changes) == 0); i++ {
		time.Sleep(50 * time.Millisecond)
		h.settle()
	}
	if ov := h.f.frame.Columns[0].Pane.Overview; ov == nil || len(ov.Changes) != 1 || ov.Changes[0].From != 200 {
		t.Fatalf("overview %+v", ov)
	}
	h.shot("overview")
	st := &h.f.cols[0]
	x, y := st.size.W-4, st.size.H/2
	if !h.f.onStrip(0, x) || h.f.onStrip(0, st.size.W/2) {
		t.Fatal("onStrip")
	}
	h.f.paneInput(0, ui.InputEvent{Kind: ui.InputPointerDown, X: x, Y: y})
	h.f.paneInput(0, ui.InputEvent{Kind: ui.InputPointerMove, X: x, Y: y * 1.2})
	h.f.paneInput(0, ui.InputEvent{Kind: ui.InputPointerUp, X: x, Y: y * 1.2})
	h.settle()
	ov := h.f.frame.Columns[0].Pane.Overview
	if mid := ov.Top + ov.Visible/2; mid < 170 || mid > 195 {
		t.Fatalf("after dragging the strip to 0.6: lines %d+%d", ov.Top, ov.Visible)
	}
	if ov.Caret != 0 {
		t.Fatalf("the strip moved the caret to %d", ov.Caret)
	}
}

// 拖标签：把 main.go 拖到 README.md 前面；右键菜单的「关闭其他」。
func TestDragTabs(t *testing.T) {
	h := newHarness(t, sample, Options{Appearance: "dark"})
	h.open("README.md")
	h.ed.Run("workbench.action.keepEditor")
	h.open("main.go")
	h.ed.Run("workbench.action.keepEditor")
	h.settle()
	tabs := func() string {
		var ns []string
		for _, tb := range h.f.frame.Columns[0].Tabs {
			ns = append(ns, tb.Name)
		}
		return strings.Join(ns, ",")
	}
	if tabs() != "README.md,main.go" {
		t.Fatalf("tabs %s", tabs())
	}
	// 标签的 × 的名字只在标签上有：按在 × 左边的名字上，拖到 README.md 的名字上
	from, ok := h.tt.Find("关闭 main.go")
	to, ok2 := h.tt.Find("关闭 README.md")
	if !ok || !ok2 {
		t.Fatal("tabs not found")
	}
	fx, tx, y := from.X-24, to.X-24, from.Y+from.H/2
	h.tt.Press(fx, y)
	for k := 1; k <= 8; k++ { // 一路拖过去
		h.tt.Move(fx+(tx-fx)*float32(k)/8, y)
		h.settle()
	}
	h.shot("tab-drag")
	h.tt.Release(tx, y)
	h.settle()
	if tabs() != "main.go,README.md" {
		t.Fatalf("after the drag: %s", tabs())
	}
	r, _ := h.tt.Find("关闭 README.md")
	h.tt.RightClickAt(r.X-24, r.Y+r.H/2)
	if err := h.tt.ChooseMenuItem("关闭其他"); err != nil {
		t.Fatalf("%v (menu %q)", err, h.tt.Menu())
	}
	h.settle()
	if tabs() != "README.md" {
		t.Fatalf("after close others: %s", tabs())
	}
}

// 拖两列之间的边：左边那一列变宽；拖侧边栏的边：侧边栏变宽。
func TestResizeColumnsAndSidebar(t *testing.T) {
	h := newHarness(t, sample, Options{Appearance: "dark"})
	h.open("main.go")
	h.ed.Run("workbench.action.splitEditor")
	h.settle()
	h.settle()
	r := h.f.cols[1].size // 右边那一列的代码窗格
	x, y := r.X+2, r.Y+r.H/2
	h.tt.Press(x, y)
	for k := 1; k <= 5; k++ {
		h.tt.Move(x+float32(k)*20, y)
		h.settle()
	}
	h.tt.Release(x+100, y)
	h.settle()
	h.shot("resize-columns")
	w0, w1 := h.f.frame.Columns[0].Weight, h.f.frame.Columns[1].Weight
	if w0 <= w1 {
		t.Fatalf("weights after dragging the edge right: %v %v (pane %+v)", w0, w1, r)
	}
	if got := h.f.cols[1].size.X; got < r.X+80 {
		t.Fatalf("the right column starts at %v, was %v", got, r.X)
	}
	sw := h.f.opts.SidebarWidth
	sx := sw - 2
	h.tt.Press(sx, y)
	for k := 1; k <= 3; k++ {
		h.tt.Move(sx+float32(k)*20, y)
		h.settle()
	}
	h.tt.Release(sx+60, y)
	h.settle()
	if h.f.opts.SidebarWidth < sw+50 {
		t.Fatalf("sidebar %v, was %v", h.f.opts.SidebarWidth, sw)
	}
}

// 拖到终端上的文件：路径加上 shell 的引号，空格连起来，末尾留一个空格。
func TestQuotePaths(t *testing.T) {
	if got := quotePaths([]string{"/a/b.go", "/c d/e.txt"}); got != "/a/b.go '/c d/e.txt' " {
		t.Fatalf("quotePaths = %q", got)
	}
}

func TestDragTabToColumnEdge(t *testing.T) {
	h := newHarness(t, sample, Options{})
	defer h.ed.Close()
	h.open("README.md")
	h.open("main.go")
	from, _ := h.tt.Find("关闭 main.go")
	pane := h.f.cols[0].size
	x, y := from.X-24, from.Y+from.H/2
	tx, ty := pane.X+pane.W-12, pane.Y+pane.H/2
	h.tt.Press(x, y)
	for k := 1; k <= 10; k++ {
		h.tt.Move(x+(tx-x)*float32(k)/10, y+(ty-y)*float32(k)/10)
		h.settle()
	}
	// Hold over the edge across several frames, so the drop target cannot flicker away.
	h.settle()
	h.settle()
	h.tt.Release(tx, ty)
	h.settle()
	if len(h.f.frame.Columns) != 2 {
		t.Fatal("edge drop did not create column")
	}
	if col := h.f.frame.Columns[1]; len(col.Tabs) != 1 || col.Tabs[0].Name != "main.go" {
		t.Fatalf("wrong split contents: %+v", col.Tabs)
	}
}
