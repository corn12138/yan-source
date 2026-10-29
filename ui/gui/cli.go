package gui

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"

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
			Message: "安装 yan 命令？", Detail: "将命令安装到 " + filepath.Join(dir, "yan") + "，以后可用 yan 目录 或 yan 文件:行号 打开砚。已有同名命令会保留。",
			Buttons: []string{"安装", "取消"}, DefaultButton: 0, CancelButton: 1})
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
		detail := "已安装到 " + filepath.Join(dir, "yan") + "。应用移动位置后，请重新安装。"
		if !slices.Contains(filepath.SplitList(os.Getenv("PATH")), dir) {
			detail += "\n若终端提示找不到命令，请将 ~/.local/bin 加入 PATH；也可以直接使用 ~/.local/bin/yan。"
		}
		mygo.Dialog.Message(mygo.MessageOptions{Parent: f.win, Type: mygo.MessageInfo, Message: "yan 命令已安装", Detail: detail, Buttons: []string{"好"}})
	}()
}
