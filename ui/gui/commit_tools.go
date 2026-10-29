package gui

import (
	"github.com/egoist/mygo"

	"github.com/corn12138/yan-source/core/i18n"
)

var patchSaveDialog = mygo.Dialog.Save

func (h host) SavePatchDialog(path string, done func(string, error)) {
	go func() {
		path, err := patchSaveDialog(mygo.SaveDialogOptions{Parent: h.f.win, Title: i18n.T("Export Patch"), DefaultPath: path, CreateDirectories: true})
		done(path, err)
	}()
}

func (h host) OpenURL(link string) {
	if h.f.hasServices {
		h.f.services.OpenURL(link)
		return
	}
	if err := mygo.Shell.OpenPath(link); err != nil {
		h.f.ed.Notify(err.Error())
	}
}
