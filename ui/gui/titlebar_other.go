//go:build !linux

package gui

import "github.com/egoist/mygo/ui"

// styleWindowControls styles the window's buttons on Linux; elsewhere the system draws them.
func styleWindowControls(muted, text ui.Color) {}
