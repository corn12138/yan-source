package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/corn12138/yan-source/core/view"
)

// 这些测试守着「不丢数据」：自动保存不覆盖别人改过的文件，换了文件夹的旧搜索结果不能改到新文件夹，
// 过时的搜索结果不替换。只碰临时目录。

// runErr 像 Run 一样运行命令，并返回它的错误（Run 把错误写到状态栏）。
func runErr(e *Editor, id string) error {
	e.sync()
	defer e.sync()
	return e.commands[id].Run(e)
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// externalWrite 像别的程序一样改文件，并把修改时间往后挪（大小可能不变）。
func externalWrite(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	os.Chtimes(p, later, later)
}

// conflictSetup 打开 a.txt（base），在编辑器里改成「local base」，再让别的程序把磁盘上的改成「external change」。
func conflictSetup(t *testing.T, autoSave bool, files map[string]string) (*Editor, string) {
	t.Helper()
	files["a.txt"] = "base\n"
	e, root := newFolderEditor(t, files)
	e.autoSave = autoSave
	p := filepath.Join(root, "a.txt")
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	typeText(e, "local ")
	externalWrite(t, p, "external change\n")
	return e, p
}

func assertKept(t *testing.T, e *Editor, p string) {
	t.Helper()
	if got := readFile(t, p); got != "external change\n" {
		t.Fatalf("disk = %q: the external change was overwritten", got)
	}
	d := e.docOf(p)
	if d == nil || !d.buf.Dirty() || d.buf.Snapshot().String(0, d.buf.Snapshot().Len()) != "local base\n" {
		t.Fatal("the unsaved edit must stay in the editor")
	}
}

func TestAutoSaveOnBlurKeepsExternalChange(t *testing.T) {
	e, p := conflictSetup(t, true, map[string]string{})
	e.Handle(FocusInput{Gained: true}) // 回到窗口：发现冲突
	if !strings.Contains(e.status, "被别的程序改过") {
		t.Fatalf("status = %q", e.status)
	}
	e.Handle(FocusInput{Gained: false}) // 离开窗口：自动保存
	assertKept(t, e, p)
	if !strings.Contains(e.status, "没有保存") {
		t.Fatalf("status should explain the refused save: %q", e.status)
	}
}

func TestAutoSaveTimerKeepsExternalChange(t *testing.T) {
	e, p := conflictSetup(t, true, map[string]string{})
	typeText(e, "!") // 停下 1 秒后自动保存
	pump(t, e, func() bool { return strings.Contains(e.status, "没有保存") })
	if got := readFile(t, p); got != "external change\n" {
		t.Fatalf("disk = %q", got)
	}
}

func TestAutoSaveOnTabSwitchCloseAndFolderSwitch(t *testing.T) {
	e, p := conflictSetup(t, true, map[string]string{"b.txt": "b\n"})
	root := filepath.Dir(p)
	// 换到别的标签
	e.OpenFile(filepath.Join(root, "b.txt"))
	assertKept(t, e, p)
	if !strings.Contains(e.status, "没有保存") {
		t.Fatalf("tab switch must surface the refused save: %q", e.status)
	}
	// 关掉它：存不了就先问，不丢也不覆盖
	e.OpenFile(p)
	e.Run("workbench.action.closeActiveEditor")
	assertKept(t, e, p)
	if len(tabNames(e)) != 2 {
		t.Fatalf("tab closed despite the conflict: %v", tabNames(e))
	}
	// 换文件夹：不换
	if err := e.SwitchFolder(t.TempDir()); err == nil {
		t.Fatal("switching folders must not drop a conflicted buffer")
	}
	assertKept(t, e, p)
	// 保存全部
	if err := runErr(e, "workbench.action.files.saveAll"); err == nil {
		t.Fatal("save all should report the conflict")
	}
	assertKept(t, e, p)
}

func TestPreGitSaveKeepsExternalChange(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git")
	}
	e, p := conflictSetup(t, true, map[string]string{})
	root := filepath.Dir(p)
	for _, args := range [][]string{{"init", "-q"}, {"add", "."}, {"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-qm", "init"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	e.startGit(root)
	pump(t, e, func() bool { return e.git.repo != nil })
	if err := e.stage([]string{p}, false); err == nil || !strings.Contains(err.Error(), "被别的程序改过") {
		t.Fatalf("stage = %v", err)
	}
	assertKept(t, e, p)
}

func TestExplicitSaveRefusesThenOverwrite(t *testing.T) {
	e, p := conflictSetup(t, false, map[string]string{}) // 没开自动保存
	e.Handle(FocusInput{Gained: false})
	assertKept(t, e, p)
	if err := runErr(e, "workbench.action.files.save"); err == nil {
		t.Fatal("explicit save must not silently overwrite")
	}
	assertKept(t, e, p)
	if err := runErr(e, cmdOverwrite); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, p); got != "local base\n" || e.cur().doc.buf.Dirty() {
		t.Fatalf("overwrite: disk %q", got)
	}
}

func TestRevertToDiskIsUndoable(t *testing.T) {
	e, p := conflictSetup(t, true, map[string]string{})
	if err := runErr(e, cmdRevert); err != nil {
		t.Fatal(err)
	}
	if content(e) != "external change\n" || e.cur().doc.buf.Dirty() {
		t.Fatalf("revert = %q", content(e))
	}
	e.Handle(ctrl('z'))
	if content(e) != "local base\n" {
		t.Fatalf("undo revert = %q", content(e))
	}
	if got := readFile(t, p); got != "external change\n" {
		t.Fatalf("disk = %q", got)
	}
}

func TestAutoSaveDeletedAndRecreated(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "base\n"})
	e.autoSave = true
	p := filepath.Join(root, "a.txt")
	e.OpenFile(p)
	typeText(e, "local ")
	os.Remove(p)
	e.Handle(FocusInput{Gained: false})
	if _, err := os.Stat(p); err == nil {
		t.Fatal("auto-save must not resurrect a deleted file")
	}
	externalWrite(t, p, "recreated\n")
	e.Handle(FocusInput{Gained: true})
	e.Handle(FocusInput{Gained: false})
	if got := readFile(t, p); got != "recreated\n" || content(e) != "local base\n" {
		t.Fatalf("disk %q buffer %q", got, content(e))
	}
	// 用户明确保存一个被删掉的文件：重新建出来
	os.Remove(p)
	if err := runErr(e, "workbench.action.files.save"); err != nil || readFile(t, p) != "local base\n" {
		t.Fatalf("explicit save of a deleted file: %v", err)
	}
}

func TestAutoSaveCleanBufferReloads(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "base\n"})
	e.autoSave = true
	p := filepath.Join(root, "a.txt")
	e.OpenFile(p)
	externalWrite(t, p, "external\n")
	e.Handle(FocusInput{Gained: true})
	e.Handle(FocusInput{Gained: false})
	if content(e) != "external\n" || readFile(t, p) != "external\n" {
		t.Fatalf("buffer %q", content(e))
	}
	typeText(e, "x") // 重载之后正常自动保存
	e.Handle(FocusInput{Gained: false})
	if got := readFile(t, p); got != content(e) || !strings.Contains(got, "x") {
		t.Fatalf("disk = %q, buffer %q", got, content(e))
	}
}

func TestSaveFailureIsVisible(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	e.autoSave = true
	p := filepath.Join(root, "a.txt")
	e.OpenFile(p)
	typeText(e, "x")
	os.Chmod(root, 0o555) // 建不了临时文件：原子保存失败
	t.Cleanup(func() { os.Chmod(root, 0o755) })
	e.OpenFile(filepath.Join(root, "b.txt")) // 换标签时自动保存
	if !strings.Contains(e.status, "保存 a.txt 失败") {
		t.Fatalf("status = %q", e.status)
	}
	if !e.docOf(p).buf.Dirty() || readFile(t, p) != "a\n" {
		t.Fatal("failed save must keep the buffer dirty and the file intact")
	}
}

func TestAutoSaveOffLeavesDiskAlone(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n"})
	p := filepath.Join(root, "a.txt")
	e.OpenFile(p)
	typeText(e, "x")
	e.Handle(FocusInput{Gained: false})
	if readFile(t, p) != "a\n" {
		t.Fatal("auto-save is off: blur must not write")
	}
}

// ---- 搜索不越过文件夹 ----

// holdPending 把已经寄来、还没处理的回调拿走（模拟后台结果晚到）。
func holdPending(e *Editor) []func(*Editor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	held := e.pending
	e.pending = nil
	return held
}

func TestStaleSearchResultsIgnoredAfterSwitchFolder(t *testing.T) {
	e, a := newFolderEditor(t, map[string]string{"x.txt": "hello from A\n"})
	b := t.TempDir()
	os.WriteFile(filepath.Join(b, "y.txt"), []byte("hello from B\n"), 0o644)

	// A 的搜索在后台跑完，结果寄来了，但还没交给所有者。
	e.Run("workbench.action.findInFiles")
	e.RunPending()
	e.fs.query = "hello"
	e.searchSoon(0)
	time.Sleep(300 * time.Millisecond) // 两个小文件：后台早就找完、寄出最后一批了
	held := holdPending(e)
	if len(held) == 0 || !e.fs.running {
		t.Fatal("A's search results should be waiting in the queue")
	}

	if err := e.SwitchFolder(b); err != nil {
		t.Fatal(err)
	}
	pump(t, e, func() bool { return !e.ws.indexing })
	e.Run("workbench.action.findInFiles") // 和在 A 里一样的步骤：重置前的实现会用回同一个代号
	e.fs.query = "hello"
	e.searchSoon(0)
	var f view.Frame
	pump(t, e, func() bool { e.Frame(&f); return !e.fs.running })

	// A 的结果现在才到。
	e.mu.Lock()
	e.pending = append(e.pending, held...)
	e.mu.Unlock()
	e.RunPending()
	for _, sf := range e.fs.files {
		if !strings.HasPrefix(sf.path, b) {
			t.Fatalf("A's result leaked into B: %s", sf.path)
		}
	}
	if len(e.fs.files) != 1 {
		t.Fatalf("B results = %d", len(e.fs.files))
	}
	e.fs.with = "bye"
	e.ReplaceSearch(-1)
	if got := readFile(t, filepath.Join(a, "x.txt")); got != "hello from A\n" {
		t.Fatalf("A was modified from B: %q", got)
	}
	if got := readFile(t, filepath.Join(b, "y.txt")); got != "bye from B\n" {
		t.Fatalf("B = %q", got)
	}
}

func TestReplaceRejectsResultsOutsideWorkspace(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"x.txt": "hello\n"})
	other := filepath.Join(t.TempDir(), "o.txt")
	os.WriteFile(other, []byte("hello\n"), 0o644)
	e.fs.query = "hello"
	e.searchSoon(0)
	pump(t, e, func() bool { return !e.fs.running })
	e.fs.files = append(e.fs.files, &searchFile{path: other, rel: "o.txt", matches: e.fs.files[0].matches})
	e.fs.with = "bye"
	e.ReplaceSearch(-1)
	if readFile(t, other) != "hello\n" {
		t.Fatal("replaced a file outside the workspace")
	}
	_ = root
}

func TestGitCallbackIgnoredAfterSwitchFolder(t *testing.T) {
	e, _ := newFolderEditor(t, map[string]string{"x.txt": "x"})
	post := e.postHere()
	e.SwitchFolder(t.TempDir())
	e.scm.message = "B"
	post(func(e *Editor) { e.scm.message = "from A" })
	e.RunPending()
	if e.scm.message != "B" {
		t.Fatal("a callback from the old folder mutated the new one")
	}
}

// ---- 过时的结果不替换 ----

func searchFor(t *testing.T, e *Editor, q string, regex bool) {
	t.Helper()
	e.Run("workbench.action.findInFiles")
	e.fs.query, e.fs.regex = q, regex
	e.searchSoon(0)
	pump(t, e, func() bool { return !e.fs.running })
	if len(e.fs.files) == 0 {
		t.Fatalf("no results for %q", q)
	}
}

func TestReplacementsChecksMatchedText(t *testing.T) {
	re := regexp.MustCompile("cat|dog")
	if cs := replacements("dog", re, "X", &searchMatch{start: 0, end: 3, text: "cat"}); len(cs) != 0 {
		t.Fatalf("replaced different text at the same offsets: %v", cs)
	}
}

func TestReplaceOneRejectsStaleOpenBuffer(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "cat\n"})
	p := filepath.Join(root, "a.txt")
	searchFor(t, e, "cat|dog", true)
	e.fs.with = "X"
	e.SearchClick(1, true) // 打开匹配所在的文件
	// 同样长度的改动：cat → dog
	b := e.cur().doc.buf
	e.FocusColumn(0)
	e.Run("editor.action.selectAll")
	typeText(e, "dog\n")
	if content(e) != "dog\n" {
		t.Fatalf("setup: %q", content(e))
	}
	e.ReplaceSearch(1)
	if got := b.Snapshot().String(0, b.Snapshot().Len()); got != "dog\n" {
		t.Fatalf("stale result replaced: %q", got)
	}
	if !strings.Contains(e.status, "已经变了") {
		t.Fatalf("status = %q", e.status)
	}
	if readFile(t, p) != "cat\n" {
		t.Fatal("disk touched")
	}
	// 重新搜过之后能换
	pump(t, e, func() bool { return !e.fs.running && e.fs.key == e.fs.searchKey() })
	e.ReplaceSearch(1)
	if got := b.Snapshot().String(0, b.Snapshot().Len()); got != "X\n" {
		t.Fatalf("fresh result not replaced: %q", got)
	}
}

func TestReplaceOneRejectsStaleUnopenedFile(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "cat\n"})
	p := filepath.Join(root, "a.txt")
	searchFor(t, e, "cat|dog", true)
	e.fs.with = "X"
	externalWrite(t, p, "dog\n") // 一样长
	e.ReplaceSearch(1)
	if got := readFile(t, p); got != "dog\n" {
		t.Fatalf("stale result replaced on disk: %q", got)
	}
	if !strings.Contains(e.status, "已经变了") {
		t.Fatalf("status = %q", e.status)
	}
}

func TestReplaceWaitsForQueryChange(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "cat dog\n"})
	p := filepath.Join(root, "a.txt")
	searchFor(t, e, "cat", false)
	e.fs.with = "X"
	e.SetSearchField(0, "dog") // 还在等打字停下：结果还是「cat」的
	e.ReplaceSearch(1)
	if got := readFile(t, p); got != "cat dog\n" {
		t.Fatalf("replaced with results of an old query: %q", got)
	}
	if !strings.Contains(e.status, "搜索条件改了") {
		t.Fatalf("status = %q", e.status)
	}
}

// ---- 输入法提交 ----

func TestIMECommitGoesToComposingTab(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "a\n", "b.txt": "b\n"})
	pa, pb := filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")
	e.OpenFile(pa)
	target := e.InputTarget()
	e.OpenFile(pb)
	e.Handle(TextInput{Text: "你好", IME: true, Target: target})
	if content(e) != "b\n" || e.docOf(pa).buf.Snapshot().String(0, 7) != "你好a" {
		t.Fatalf("b = %q", content(e))
	}
	if e.cur().doc.buf.Path() != pb {
		t.Fatal("focus should stay on the tab the user switched to")
	}
	// 组字时的标签关了：不插进别的文件，字留在状态栏
	e.OpenFile(pa)
	target = e.InputTarget()
	e.Run("workbench.action.closeActiveEditor")
	e.Run("workbench.action.closeActiveEditor") // 有未保存的修改：再按一次放弃
	e.Handle(TextInput{Text: "世界", IME: true, Target: target})
	if content(e) != "b\n" || !strings.Contains(e.status, "世界") {
		t.Fatalf("b = %q status %q", content(e), e.status)
	}
}

func externalWriteSameMetadata(t *testing.T, p, value string) {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if int64(len(value)) != info.Size() {
		t.Fatal("fixture must preserve size")
	}
	if err := os.WriteFile(p, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
}

func TestAutoSavePreservedMetadataConflict(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "before\n"})
	e.autoSave = true
	p := filepath.Join(root, "a.txt")
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	typeText(e, "local ")
	externalWriteSameMetadata(t, p, "AFTER!\n")
	e.Handle(FocusInput{Gained: false})
	if readFile(t, p) != "AFTER!\n" || content(e) != "local before\n" || !e.cur().doc.buf.Dirty() {
		t.Fatal("auto-save overwrote external or local text")
	}
	if !strings.Contains(e.status, "没有保存") {
		t.Fatalf("missing conflict feedback: %q", e.status)
	}
}

func TestFocusReloadPreservedMetadata(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "before\n"})
	p := filepath.Join(root, "a.txt")
	if err := e.OpenFile(p); err != nil {
		t.Fatal(err)
	}
	externalWriteSameMetadata(t, p, "AFTER!\n")
	e.Handle(FocusInput{Gained: true})
	if content(e) != "AFTER!\n" || e.cur().doc.buf.Dirty() {
		t.Fatal("clean buffer did not reload")
	}
	e.Run("undo")
	if content(e) != "before\n" || !e.cur().doc.buf.Dirty() || readFile(t, p) != "AFTER!\n" {
		t.Fatal("reload undo lost a version")
	}
}

func TestReplaceOneRejectsPreservedMetadata(t *testing.T) {
	for _, open := range []bool{false, true} {
		t.Run(map[bool]string{false: "unopened", true: "opened-after-search"}[open], func(t *testing.T) {
			e, root := newFolderEditor(t, map[string]string{"a.txt": "cat old\n"})
			p := filepath.Join(root, "a.txt")
			searchFor(t, e, "cat", false)
			e.fs.with = "dog"
			externalWriteSameMetadata(t, p, "cat NEW\n")
			if open {
				e.SearchClick(1, true)
			}
			e.ReplaceSearch(1)
			if readFile(t, p) != "cat NEW\n" || !strings.Contains(e.status, "已经变了") {
				t.Fatalf("stale action: disk %q status %q", readFile(t, p), e.status)
			}
			if open && (content(e) != "cat NEW\n" || e.cur().doc.buf.Dirty()) {
				t.Fatal("stale action edited a newly opened buffer")
			}
		})
	}
}

func TestReplaceRechecksLoadedVersion(t *testing.T) {
	e, root := newFolderEditor(t, map[string]string{"a.txt": "cat old\n"})
	p := filepath.Join(root, "a.txt")
	searchFor(t, e, "cat", false)
	f := e.fs.files[0]
	if !e.current(f) {
		t.Fatal("fixture must initially match")
	}
	externalWriteSameMetadata(t, p, "cat NEW\n")
	e.fs.with = "dog"
	if _, err := e.replaceIn(f, &f.matches[0]); err == nil {
		t.Fatal("change after result validation must be refused")
	}
	if readFile(t, p) != "cat NEW\n" {
		t.Fatal("late change overwritten")
	}
	if n, err := e.replaceIn(f, nil); err != nil || n != 1 || readFile(t, p) != "dog NEW\n" {
		t.Fatalf("whole-file replace must use fresh text: %d %v", n, err)
	}
}

func TestSearchReplaceBOMCoordinates(t *testing.T) {
	for _, open := range []bool{false, true} {
		t.Run(map[bool]string{false: "unopened", true: "opened"}[open], func(t *testing.T) {
			e, root := newFolderEditor(t, map[string]string{"a.txt": "\xef\xbb\xbfcat\r\ncat\r\n"})
			p := filepath.Join(root, "a.txt")
			searchFor(t, e, "cat", false)
			if got := e.fs.files[0].matches[0].start; got != 0 {
				t.Fatalf("BOM shifted match by %d bytes", got)
			}
			if open {
				e.SearchClick(1, true)
			}
			e.fs.with = "dog"
			e.ReplaceSearch(1)
			if open {
				if err := runErr(e, "workbench.action.files.save"); err != nil {
					t.Fatal(err)
				}
			}
			if got := readFile(t, p); got != "\xef\xbb\xbfdog\r\ncat\r\n" {
				t.Fatalf("wrong replacement bytes: %q", got)
			}
		})
	}
}
