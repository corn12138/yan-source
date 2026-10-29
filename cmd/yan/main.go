// Command yan 是极简编辑器「砚」的入口。
//
//	yan 文件路径[:行号]
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/corn12138/yan-source/core/app"
	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/internal/launch"
	"github.com/corn12138/yan-source/internal/version"
	"github.com/corn12138/yan-source/platform/locale"
	"github.com/corn12138/yan-source/ui/tui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "yan:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "-v" || args[0] == "--version") {
		fmt.Println("yan", version.String())
		return nil
	}
	state := config.LoadState(config.StatePath())
	langWarns := config.StartLocale(state, locale.Terminal())
	if len(args) > 1 || (len(args) == 1 && (args[0] == "-h" || args[0] == "--help")) {
		fmt.Println(i18n.T("Usage:\n  yan              open the current directory\n  yan FOLDER       open a folder (Explorer, Quick Open)\n  yan FILE[:LINE]  open a file; create it when it doesn't exist"))
		if len(args) > 1 {
			return errors.New(i18n.T("too many arguments"))
		}
		return nil
	}
	arg := "."
	if len(args) == 1 {
		arg = args[0]
	}
	t := launch.Resolve(arg)
	root, file, folder, line := t.Root, t.File, t.Folder, t.Line

	cfg, cfgWarns := config.Load(root)
	cfgWarns = append(langWarns, cfgWarns...)
	state.Apply(cfg)
	fe := tui.New(cfg.TabWidth)
	fe.SidebarWidth = cfg.SidebarWidth
	if cfg.Appearance == "light" {
		fe.SetTheme(cfg.Theme, false)
	} else {
		fe.SetTheme(cfg.Theme, true)
	}
	ed, warns := app.New(cfg, fe.Layouter(), fe.Host(), fe.Profile())
	ed.SetState(state)
	warns = append(cfgWarns, warns...)
	if folder {
		if err := ed.OpenFolder(root); err != nil {
			return err
		}
	} else {
		ed.SetRoot(root)
	}
	if folder && t.Readme && ed.RestoreSession() {
		file = "" // 上次开着的标签都回来了
	}
	if file != "" {
		if err := ed.OpenFile(file); err != nil {
			return err
		}
		if line > 0 {
			ed.GotoLine(line)
		}
	}
	if len(warns) > 0 {
		ed.Notify(i18n.F("Configuration: %s", strings.Join(warns, i18n.TC("warning separator", "; "))))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGHUP)
	defer stop()
	err := fe.Run(ctx, ed)
	ed.SaveSession()
	return err
}
