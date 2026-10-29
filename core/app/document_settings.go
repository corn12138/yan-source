package app

// Status settings and sidebar placement follow Mayoi v0.8.x, adapted to shared core commands.

import (
	"github.com/corn12138/yan-source/core/edit"
	"github.com/corn12138/yan-source/core/i18n"
	"github.com/corn12138/yan-source/core/text"
)

func (e *Editor) SidebarRight() bool {
	if e.state != nil && e.state.SidebarRight != nil {
		return *e.state.SidebarRight
	}
	return e.cfg.SidebarRight
}

func (e *Editor) toggleSidebarPosition() error {
	right := !e.SidebarRight()
	e.cfg.SidebarRight = right
	if e.state != nil {
		e.state.SidebarRight = &right
		e.saveState()
	}
	e.settingsGen++
	return nil
}

// PanelPosition is where the window puts the terminal panel: "" below the editors, "left" or
// "right" at the window's edge, beyond a sidebar on the same side.
func (e *Editor) PanelPosition() string { return e.cfg.PanelPosition }

func (e *Editor) setPanelPosition(pos string) error {
	e.cfg.PanelPosition = pos
	if e.state != nil {
		e.state.PanelPosition = &pos
		e.saveState()
	}
	e.settingsGen++
	return nil
}

func (e *Editor) chooseIndent() error {
	e.choose(i18n.T("Indentation (for further editing)"),
		ovItem{label: i18n.T("Use Tabs"), run: func(e *Editor) error { e.Run("editor.action.indentUsingTabs"); return nil }},
		ovItem{label: i18n.T("Use 2 Spaces"), run: func(e *Editor) error { e.Run("yan.indentUsingSpaces2"); return nil }},
		ovItem{label: i18n.T("Use 4 Spaces"), run: func(e *Editor) error { e.Run("yan.indentUsingSpaces4"); return nil }},
		ovItem{label: i18n.T("Use 8 Spaces"), run: func(e *Editor) error { e.Run("yan.indentUsingSpaces8"); return nil }})
	return nil
}

func (e *Editor) chooseEOL() error {
	e.choose(i18n.T("Change End of Line Sequence"),
		ovItem{label: "LF", run: func(e *Editor) error { return e.setEOL(text.EOLLF) }},
		ovItem{label: "CRLF", run: func(e *Editor) error { return e.setEOL(text.EOLCRLF) }})
	return nil
}

func (e *Editor) setEOL(eol text.EOL) error {
	return e.editDoc(func(b *edit.Buffer) { b.SetEOL(eol) })
}
