package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/config"
)

// 端到端：真实的 tcell 屏幕接在 tcell 自带的虚拟终端上，按字节注入按键，读回屏幕和文件。
func TestEndToEnd(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	term := vt.NewMockTerm(vt.MockOptSize{X: 50, Y: 8})
	scr, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	fe := New(cfg.TabWidth)
	fe.screen = scr
	ed, warns := app.New(cfg, fe.Layouter(), nopHost{}, fe.Profile())
	if len(warns) > 0 {
		t.Fatalf("keymap warnings: %v", warns)
	}
	if err := ed.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	done := runMockEditor(t, fe, ed)

	send := func(s string) { term.SendRaw([]byte(s)) }
	row := func(y int) string {
		var b strings.Builder
		for x := 0; x < 50; x++ {
			c := term.GetCell(vt.Coord{X: vt.Col(x), Y: vt.Row(y)})
			if c.W == 0 && c.C == "" {
				continue
			}
			b.WriteString(c.C)
		}
		return strings.TrimRight(b.String(), " ")
	}

	// The first frame is drawn on another goroutine: wait for it rather than a fixed time, as a
	// race-detector build on a busy machine can take longer.
	for deadline := time.Now().Add(5 * time.Second); !strings.Contains(row(1), "hello") && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
	}
	if got := row(1); !strings.Contains(got, "hello") {
		t.Fatalf("first line not drawn: %q", got)
	}
	send("中文 ")    // 输入法提交的中文：插在文首
	send("\x1b[B") // ↓
	send("\x1b[F") // End
	send("!")      //
	send("\x13")   // Ctrl+S
	// Input and atomic saves run on the owner goroutine; bytes sent are not a completion signal.
	waitMock(t, "saved text", func() bool {
		got, err := os.ReadFile(path)
		return err == nil && string(got) == "中文 hello\nworld!\n"
	})
	waitMock(t, "saved status and edited row", func() bool {
		return strings.Contains(row(7), "已保存") && strings.HasSuffix(row(2), "world!")
	})
	send("\x1a") // Ctrl+Z：撤销 "!"
	waitMock(t, "undo row", func() bool { return strings.HasSuffix(row(2), "world") })
	send("\x11") // Ctrl+Q：有未保存修改，只提醒
	waitMock(t, "unsaved warning", func() bool { return strings.Contains(row(7), "未保存") })
	send("\x11") // 再按一次：退出
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("editor did not quit")
	}
}

func waitMock(t *testing.T, label string, ready func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); !ready(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", label)
		}
	}
}

func runMockEditor(t *testing.T, fe *Frontend, ed *app.Editor) <-chan error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done, stopped := make(chan error, 1), make(chan struct{})
	go func() { defer close(stopped); done <- fe.Run(ctx, ed) }()
	t.Cleanup(func() {
		// A failed assertion still stops the owner before its files and watchers are removed.
		cancel()
		select {
		case <-stopped:
			ed.Close()
		case <-time.After(5 * time.Second):
			t.Error("test editor did not stop")
		}
	})
	return done
}

type nopHost struct{}

func (nopHost) SetClipboard(string) error  { return nil }
func (nopHost) Clipboard() (string, error) { return "", nil }
func (nopHost) Shell(string) error         { return nil }
func (nopHost) Trash(string) error         { return nil }

func TestFolderModeEndToEnd(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src"), 0o755)
	os.WriteFile(filepath.Join(root, "src", "server.go"), []byte("package src\n"), 0o644)
	os.WriteFile(filepath.Join(root, "README.md"), []byte("# 你好\n"), 0o644)

	term := vt.NewMockTerm(vt.MockOptSize{X: 100, Y: 20})
	scr, err := tcell.NewTerminfoScreenFromTty(term)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	fe := New(cfg.TabWidth)
	fe.screen = scr
	ed, _ := app.New(cfg, fe.Layouter(), nopHost{}, fe.Profile())
	if err := ed.OpenFolder(root); err != nil {
		t.Fatal(err)
	}
	ed.OpenFile(filepath.Join(root, "README.md"))
	done := runMockEditor(t, fe, ed)
	send := func(s string) { term.SendRaw([]byte(s)); time.Sleep(120 * time.Millisecond) }
	row := func(y int) string {
		var b strings.Builder
		for x := 0; x < 100; x++ {
			b.WriteString(term.GetCell(vt.Coord{X: vt.Col(x), Y: vt.Row(y)}).C)
		}
		return b.String()
	}
	// 绘制在别的 goroutine 里：慢的机器上（比如竞态检测）多等一会儿，不靠固定的睡眠
	eventually := func(ok func() bool) bool {
		for deadline := time.Now().Add(5 * time.Second); !ok(); time.Sleep(20 * time.Millisecond) {
			if time.Now().After(deadline) {
				return false
			}
		}
		return true
	}
	if !eventually(func() bool {
		r := row(0)
		return strings.Contains(r, "资源管理器") && strings.Contains(r, "README.md")
	}) {
		t.Fatalf("header/tab row = %q", row(0))
	}
	// Terminal cells are flushed incrementally; the header can arrive before the code row.
	if !eventually(func() bool {
		r := row(1)
		return strings.Contains(r, "▸ src") && strings.Contains(r, "│") && strings.Contains(r, "你好")
	}) {
		t.Fatalf("row 1 = %q", row(1))
	}
	send("\x10")   // Ctrl+P
	send("server") //
	var overlay string
	if !eventually(func() bool {
		overlay = ""
		for y := 1; y < 6; y++ {
			overlay += row(y) + "\n"
		}
		return strings.Contains(overlay, "快速打开") && strings.Contains(overlay, "server.go") && strings.Contains(overlay, "╭")
	}) {
		t.Fatalf("overlay not drawn:\n%s", overlay)
	}
	// 真实光标在浮层输入行：输入法候选框跟着它。光标在格子画完之后才移过去，也要等一下。
	if !eventually(func() bool { return term.Pos().Y == 2 }) {
		t.Fatalf("cursor at %+v, want row 2", term.Pos())
	}
	send("\r")
	if !eventually(func() bool { return strings.Contains(row(0), "server.go") }) {
		t.Fatalf("server.go tab missing: %q", row(0))
	}
	send("\x11")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("did not quit")
	}
}
