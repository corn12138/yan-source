package app

import (
	"errors"
	"slices"
)

// Locked columns follow Mayoi (MIT, cmd/mayoi/columnlock.go and columns.go removeColumn at 46990a1):
// a locked column keeps what was put there on purpose — a terminal, a reference file — and implicit
// opens (explorer, quick open, search, Source Control, history, new terminals) go to the nearest
// unlocked column instead. Explicit drops and session restoration still target a locked column.
// There is always one unlocked column; a locked column stays when emptied, and removing an empty
// unlocked column gives its width to another unlocked one so locked widths do not change.

// fileColumn returns the column an implicit open goes to: the active one unless it is locked,
// else the last unlocked column used, the nearest unlocked one to the left, any unlocked one,
// or a new one at the far left.
func (e *Editor) fileColumn() *column {
	c := e.column()
	if !c.locked {
		return c
	}
	if e.fileCol != nil && !e.fileCol.locked && slices.Contains(e.cols, e.fileCol) {
		return e.fileCol
	}
	for i := e.col - 1; i >= 0; i-- {
		if !e.cols[i].locked {
			e.fileCol = e.cols[i]
			return e.fileCol
		}
	}
	for _, c := range e.cols {
		if !c.locked {
			e.fileCol = c
			return c
		}
	}
	// Every column is locked: files get a new column at the left, taking no locked width.
	nc := newColumn()
	nc.size, nc.weight = e.cols[0].size, e.cols[0].weight
	e.cols = slices.Insert(e.cols, 0, nc)
	e.col++
	e.fileCol = nc
	return nc
}

// toggleColumnLock locks or unlocks the active column. Locking leaves the keyboard where it is
// and makes sure files still have an unlocked column; unlocking an empty column removes it.
func (e *Editor) toggleColumnLock() error {
	c := e.column()
	c.locked = !c.locked
	if c.locked {
		if e.fileCol == c {
			e.fileCol = nil
		}
		e.fileColumn()
		e.col = slices.Index(e.cols, c) // a new column on the left moved this one
		return nil
	}
	if len(c.tabs) == 0 && len(e.cols) > 1 {
		e.removeColumn(c)
	}
	return nil
}

// removable reports whether an empty column may go: never a locked one, nor the last unlocked one.
func (e *Editor) removable(c *column) bool {
	if c.locked || len(e.cols) < 2 {
		return false
	}
	for _, o := range e.cols {
		if o != c && !o.locked {
			return true
		}
	}
	return false
}

// removeColumn takes an empty column away. While a column is locked, its width goes to another
// unlocked column (the file column first, else the nearest), so locked columns keep their size.
func (e *Editor) removeColumn(c *column) {
	at := slices.Index(e.cols, c)
	if at < 0 || !e.removable(c) {
		return
	}
	if slices.ContainsFunc(e.cols, func(o *column) bool { return o.locked }) {
		var heir *column
		for d := 1; d < len(e.cols) && heir == nil; d++ {
			for _, i := range []int{at - d, at + d} {
				if i >= 0 && i < len(e.cols) && !e.cols[i].locked && heir == nil {
					heir = e.cols[i]
				}
			}
		}
		if e.fileCol != nil && e.fileCol != c && !e.fileCol.locked && slices.Contains(e.cols, e.fileCol) {
			heir = e.fileCol
		}
		if heir != nil {
			heir.weight += c.weight
		}
	}
	e.cols = slices.Delete(e.cols, at, at+1)
	if e.fileCol == c {
		e.fileCol = nil
	}
	if e.col >= at {
		e.col = max(e.col-1, 0)
	}
}

// lockedTerminalColumn returns the rightmost column made ready for a terminal: reused when it
// holds only terminals and is locked (or is from, the terminal's own column), else a new
// locked column at the right.
func (e *Editor) lockedTerminalColumn(from *column) *column {
	to := e.cols[len(e.cols)-1]
	terminalOnly := true
	for _, t := range to.tabs {
		terminalOnly = terminalOnly && t.term != nil
	}
	if !(terminalOnly && (to.locked || to == from)) {
		col := e.col
		to = e.splitColumn(len(e.cols)-1, false)
		e.col = col
	}
	if !to.locked {
		to.locked = true
		if e.fileCol == to {
			e.fileCol = nil
		}
	}
	return to
}

// moveTerminalToLockedRight moves the active terminal tab, or the panel's terminal, into a locked
// column at the right. The tab moves with its id, so the window keeps its running shell.
func (e *Editor) moveTerminalToLockedRight() error {
	c := e.column()
	if t := c.tab(); t != nil && t.term != nil {
		to := e.lockedTerminalColumn(c)
		if to != c {
			e.moveTab(c, c.active, to, len(to.tabs))
		}
		e.fileColumn() // the last file column may have just emptied
		return nil
	}
	h, ok := e.host.(interface{ MovePanelTerminal(col, at int) bool })
	if !ok {
		return errors.New("没有开着的终端")
	}
	to := e.lockedTerminalColumn(nil)
	at := slices.Index(e.cols, to)
	if !h.MovePanelTerminal(at, len(to.tabs)) {
		if len(to.tabs) == 0 {
			to.locked = false
			e.removeColumn(to)
		}
		return errors.New("没有开着的终端")
	}
	return nil
}

// welcomeColumn is the one column that shows the welcome page when no column holds a tab: the
// first unlocked one, else the first; -1 while any tab is open. Other empty columns show a short
// note, so empty columns beside a file or each other do not repeat the page.
func (e *Editor) welcomeColumn() int {
	first := -1
	for i, c := range e.cols {
		if len(c.tabs) > 0 {
			return -1
		}
		if !c.locked {
			return i
		}
		if first < 0 {
			first = i
		}
	}
	return first
}

// closableColumn reports whether column c could be closed once empty: unlocked, it must not be
// the last column for files.
func (e *Editor) closableColumn(c *column) bool {
	if len(e.cols) < 2 {
		return false
	}
	return slices.ContainsFunc(e.cols, func(o *column) bool { return o != c && !o.locked })
}

// CloseColumn closes the empty column col (unlocking it first): an empty column has no tabs to
// close, so its header offers this instead. A column holding tabs is left alone.
func (e *Editor) CloseColumn(col int) {
	if col < 0 || col >= len(e.cols) {
		return
	}
	e.sync()
	defer e.sync()
	c := e.cols[col]
	if len(c.tabs) > 0 || !e.closableColumn(c) {
		return
	}
	c.locked = false
	e.removeColumn(c)
}

// ColumnLocked reports whether column col is locked (for the tab menu's check mark).
func (e *Editor) ColumnLocked(col int) bool {
	return col >= 0 && col < len(e.cols) && e.cols[col].locked
}
