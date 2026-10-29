package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/internal/cli"
	"github.com/egoist/mygo"
)

func (h host) InstallCLI() {
	f := h.f
	go func() {
		home, err := os.UserHomeDir()
		if err != nil {
			f.do(func() { f.ed.Notify(err.Error()) })
			return
		}
		dir := filepath.Join(home, ".local", "bin")
		result, err := mygo.Dialog.Message(mygo.MessageOptions{Parent: f.win, Type: mygo.MessageInfo,
			Message: i18n.T("Install the yan command?"), Detail: i18n.F("The command is installed to %s; afterwards \"yan folder\" or \"yan file:line\" opens Yan. An existing command with the same name is kept.", filepath.Join(dir, "yan")),
			Buttons: []string{i18n.T("Install"), i18n.T("Cancel")}, DefaultButton: 0, CancelButton: 1})
		if err != nil || result.Button != 0 {
			return
		}
		exe, err := os.Executable()
		if err == nil {
			_, err = cli.Install(dir, exe, runtime.GOOS)
		}
		if err != nil {
			f.do(func() { f.ed.Notify(err.Error()) })
			return
		}
		detail := i18n.F("Installed to %s. Reinstall after moving the app.", filepath.Join(dir, "yan"))
		if !slices.Contains(filepath.SplitList(os.Getenv("PATH")), dir) {
			detail += i18n.T("\nIf the terminal says the command is not found, add ~/.local/bin to PATH, or use ~/.local/bin/yan directly.")
		}
		mygo.Dialog.Message(mygo.MessageOptions{Parent: f.win, Type: mygo.MessageInfo, Message: i18n.T("yan command installed"), Detail: detail, Buttons: []string{i18n.T("OK")}})
	}()
}
