// Command yan-app 是「砚」的窗口版：同一个内核，MyGo 画的原生窗口。
//
//	yan-app [目录 | 文件[:行号]]
//
// 开发时用 go run ./cmd/yan-app 目录；打包成 .app 用 go tool mygo build（见 mygo.json）。
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/internal/crashlog"
	"github.com/corn12138/yan-source/internal/launch"
	"github.com/corn12138/yan-source/internal/process"
	"github.com/corn12138/yan-source/internal/version"
	"github.com/corn12138/yan-source/ui/gui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "yan-app:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Println("yan-app", version.String())
		return nil
	}
	if !gui.PrimaryInstance() {
		return nil
	}
	process.EnableLoginShellPath()
	defer func() {
		if err := process.StopLoginShellPath(); err != nil {
			fmt.Fprintln(os.Stderr, "yan-app:", err)
		}
	}()
	crashlog.Keep(filepath.Dir(config.UserPath()))
	wd, _ := os.Getwd()
	request := launch.Parse(args, wd)
	var t launch.Target
	var extra []launch.Target
	if len(request.Targets) > 0 {
		t = request.Targets[0]
		extra = request.Targets[1:]
	} else if !request.NewWindow && wd != "" && wd != "/" {
		if home, _ := os.UserHomeDir(); filepath.Clean(wd) != filepath.Clean(home) {
			t = launch.ResolvePath(wd)
		}
	}

	// 所有窗口共用一份状态：菜单里选的设置、最近打开的文件夹、各文件夹的会话，谁存都不会盖掉别的窗口记下的。
	state := config.LoadState(config.StatePath())
	ed, fe, warns := newEditor(t.Root, state)
	gui.SetWindowMaker(func() (*app.Editor, *gui.Frontend) {
		ed, fe, warns := newEditor("", state)
		if len(warns) > 0 {
			ed.Notify("配置：" + strings.Join(warns, "；"))
		}
		return ed, fe
	})
	switch {
	case t.Folder:
		if err := ed.OpenFolder(t.Root); err != nil {
			return err
		}
	case t.Root != "":
		ed.SetRoot(t.Root)
	}
	if t.Folder && t.Readme && ed.RestoreSession() {
		t.File = "" // 上次开着的标签都回来了
	}
	if t.File != "" {
		if err := ed.OpenFile(t.File); err != nil {
			return err
		}
		if t.Line > 0 {
			ed.GotoLine(t.Line)
		}
	}
	if len(warns) > 0 {
		ed.Notify("配置：" + strings.Join(warns, "；"))
	}
	if selftest != nil {
		selftest(ed, fe)
	}
	return fe.Run(ed, extra...)
}

// newEditor 读配置（root 不为空时叠上那里的 .yan.toml），造一个窗口的编辑器和前端；返回配置的告警。
func newEditor(root string, state *config.State) (*app.Editor, *gui.Frontend, []string) {
	cfg, cfgWarns := config.Load(root)
	state.Apply(cfg)
	fe := gui.New(gui.Options{
		Theme: cfg.Theme, Appearance: cfg.Appearance,
		FontFamily: cfg.FontFamily, FontSize: float32(cfg.FontSize), LineHeight: float32(cfg.LineHeight),
		TabWidth: cfg.TabWidth,
	})
	ed, warns := app.New(cfg, fe.Layouter(), fe.Host(), fe.Profile())
	ed.SetState(state)
	ed.OfferGuide() // once, on the first launch: state.json remembers the answer
	return ed, fe, append(cfgWarns, warns...)
}

// selftest 只在 -tags selftest 的验证构建里有（见 selftest.go）；发布的应用里是 nil。
var selftest func(*app.Editor, *gui.Frontend)
