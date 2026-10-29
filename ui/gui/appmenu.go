package gui

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/i18n"
)

// menuRevision counts the menus asked for: a deferred one applies only while it is the latest.
var menuRevision uint64

// setAppMenu puts menu in the application's menu bar. On Linux the menu bar is a GTK widget,
// and replacing it while GTK is drawing the window (the view runs inside that callback)
// destroys a child GTK is still walking: the replacement waits for the main loop instead, and
// only the latest one applies (after Mayoi #55, MIT).
func setAppMenu(menu *mygo.Menu) {
	if !nativeMenuBar() {
		return // Windows and Linux: the title bar's menu button opens the menu in the window
	}
	menuRevision++
	if runtime.GOOS != "linux" {
		mygo.App.SetMenu(menu)
		return
	}
	revision := menuRevision
	go mygo.RunOnMain(func() {
		if revision == menuRevision {
			mygo.App.SetMenu(menu)
		}
	})
}

// nativeMenuBar reports whether the app's menu is the system's menu bar: on macOS. On Windows and
// Linux a native menu bar takes a whole row of the borderless window, so the title bar has a
// menu button that opens the same menu in the window instead (after Mayoi PR #57 by bet4it).
var nativeMenuBar = func() bool { return runtime.GOOS == "darwin" }

// populateMenu fills an in-window menu with the items of the app's menu: their submenus, check
// marks and shortcuts, and the roles the native menu bar would do, done by Yan's commands.
func (f *Frontend) populateMenu(m *ui.Menu, items []*mygo.MenuItem) {
	sep := false // a separator waits for an item after it
	for _, it := range items {
		if it == nil || it.Hidden {
			continue
		}
		if it.Type == mygo.MenuItemSeparator {
			sep = true
			continue
		}
		label, run := it.Label, func() {}
		switch it.Role {
		case mygo.RoleAppMenu:
			continue // macOS's own menu
		case mygo.RoleWindowMenu:
			if sep {
				m.Separator()
				sep = false
			}
			m.Submenu(i18n.T("Window"), func(sub *ui.Menu) {
				if sub.Item(i18n.T("Minimize")).Chosen() {
					f.win.Minimize()
				}
				if sub.Item(i18n.T("Toggle Full Screen")).Chosen() {
					f.win.SetFullScreen(!f.win.IsFullScreen())
				}
				sub.Separator()
				if sub.Item(i18n.T("Close Window")).Chosen() {
					f.win.Close()
				}
			})
			continue
		case mygo.RoleCut, mygo.RoleCopy, mygo.RolePaste, mygo.RoleSelectAll:
			id := map[mygo.MenuRole]string{mygo.RoleCut: "editor.action.clipboardCutAction", mygo.RoleCopy: "editor.action.clipboardCopyAction",
				mygo.RolePaste: "editor.action.clipboardPasteAction", mygo.RoleSelectAll: "editor.action.selectAll"}[it.Role]
			label = map[mygo.MenuRole]string{mygo.RoleCut: i18n.T("Cut"), mygo.RoleCopy: i18n.T("Copy"), mygo.RolePaste: i18n.T("Paste"), mygo.RoleSelectAll: i18n.T("Select All")}[it.Role]
			run = func() { f.ed.Run(id) }
		case mygo.RoleToggleFullScreen:
			label, run = i18n.T("Toggle Full Screen"), func() { f.win.SetFullScreen(!f.win.IsFullScreen()) }
		default:
			if it.Role != "" {
				continue
			}
			click := it.Click
			item := it
			run = func() {
				if click != nil {
					click(item, f.win)
				}
			}
		}
		if label == "" {
			continue
		}
		if sep {
			m.Separator()
			sep = false
		}
		if len(it.Submenu) > 0 {
			subItems := it.Submenu
			m.Submenu(label, func(sub *ui.Menu) { f.populateMenu(sub, subItems) })
			continue
		}
		mi := m.Item(label).Disabled(it.Disabled)
		if it.Type == mygo.MenuItemCheckbox || it.Type == mygo.MenuItemRadio {
			mi.Checked(it.Checked)
		}
		if mods, key, ok := acceleratorKey(it.Accelerator); ok {
			mi.Shortcut(mods, key)
		}
		if mi.Chosen() {
			run()
		}
	}
}

// acceleratorKey reads a menu item's accelerator ("Ctrl+Shift+P") as the in-window menu shows it.
func acceleratorKey(acc string) (ui.Modifiers, ui.Key, bool) {
	if acc == "" {
		return 0, 0, false
	}
	parts := strings.Split(acc, "+")
	var mods ui.Modifiers
	for _, p := range parts[:len(parts)-1] {
		switch p {
		case "Ctrl":
			mods |= ui.Ctrl
		case "Alt":
			mods |= ui.Alt
		case "Shift":
			mods |= ui.Shift
		case "Cmd":
			mods |= ui.Super
		}
	}
	k := parts[len(parts)-1]
	switch {
	case len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z':
		return mods, ui.KeyA + ui.Key(k[0]-'A'), true
	case len(k) == 1 && k[0] >= 'a' && k[0] <= 'z':
		return mods, ui.KeyA + ui.Key(k[0]-'a'), true
	case len(k) == 1 && k[0] >= '0' && k[0] <= '9':
		return mods, ui.Key0 + ui.Key(k[0]-'0'), true
	case len(k) >= 2 && k[0] == 'F':
		if n, err := strconv.Atoi(k[1:]); err == nil && n >= 1 && n <= 12 {
			return mods, ui.KeyF1 + ui.Key(n-1), true
		}
	}
	return 0, 0, false
}
