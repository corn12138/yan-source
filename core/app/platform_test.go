package app

import (
	"strings"
	"testing"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/keymap"
)

// Windows、Linux 的窗口版：键位编得过、不留 Cmd，按 VS Code 在那里的键，提示写成 Ctrl+…；
// macOS 的窗口版不变。
func TestPlatformBindings(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			p := keymap.Profile{Mods: []keymap.Mod{keymap.ModCtrl}, GUI: true, OS: goos}
			e, warns := New(config.Default(), fakeLayouter{}, &fakeHost{}, p)
			if len(warns) > 0 {
				t.Fatalf("warnings: %v", warns)
			}
			for ctx, m := range platformBindings(defaultBindings(), p) {
				for seq := range m {
					if strings.Contains(seq, "cmd+") {
						t.Errorf("%v: %q keeps Cmd", ctx, seq)
					}
				}
			}
			for _, tc := range []struct {
				ctx  keymap.Context
				seq  string
				want string
			}{
				{keymap.CtxGlobal, "ctrl+shift+p", "workbench.action.showCommands"},
				{keymap.CtxGlobal, "ctrl+s", "workbench.action.files.save"},
				{keymap.CtxGlobal, "ctrl+n", "workbench.action.files.newUntitledFile"},
				{keymap.CtxGlobal, "ctrl+k ctrl+o", "workbench.action.files.openFolder"},
				{keymap.CtxGlobal, "ctrl+k shift+o", "yan.openFolderNewWindow"},
				{keymap.CtxGlobal, "alt+3", "workbench.action.openEditorAtIndex3"},
				{keymap.CtxGlobal, "ctrl+shift+g", "workbench.view.scm"},
				{keymap.CtxEditor, "ctrl+h", "editor.action.startFindReplaceAction"},
				{keymap.CtxEditor, "f3", "editor.action.nextMatchFindAction"},
				{keymap.CtxEditor, "ctrl+left", "cursorWordLeft"},
				{keymap.CtxEditor, "ctrl+backspace", "deleteWordLeft"},
				{keymap.CtxEditor, "alt+c", "toggleFindCaseSensitive"},
				{keymap.CtxEditor, "f12", "editor.action.revealDefinition"},
				{keymap.CtxEditor, "alt+left", "workbench.action.navigateBack"},
				{keymap.CtxEditor, "alt+right", "workbench.action.navigateForward"},
				{keymap.CtxGlobal, "ctrl+-", "workbench.action.zoomOut"},
			} {
				seq, _ := keymap.Parse(tc.seq, keymap.ModCtrl)
				if got, m := e.keys.Resolve(tc.ctx, seq); m != keymap.MatchExact || got != tc.want {
					t.Errorf("%s = %q (%v), want %q", tc.seq, got, m, tc.want)
				}
			}
			// Ctrl+Alt 是 AltGr，要打字：查找替换的开关不占它
			seq, _ := keymap.Parse("ctrl+alt+f", keymap.ModCtrl)
			if got, m := e.keys.Resolve(keymap.CtxEditor, seq); m != keymap.MatchNone {
				t.Errorf("ctrl+alt+f = %q", got)
			}
			if got := e.KeyHint("workbench.action.files.save"); got != "Ctrl+S" {
				t.Errorf("hint = %q", got)
			}
			if got := e.commands["revealFileInOS"].Title; strings.Contains(got, "访达") {
				t.Errorf("reveal title = %q", got)
			}
		})
	}
	t.Run("darwin", func(t *testing.T) {
		p := keymap.Profile{Mods: []keymap.Mod{keymap.ModSuper}, GUI: true, OS: "darwin"}
		e, warns := New(config.Default(), fakeLayouter{}, &fakeHost{}, p)
		if len(warns) > 0 {
			t.Fatalf("warnings: %v", warns)
		}
		if got := e.KeyHint("workbench.action.files.save"); got != "⌘S" {
			t.Errorf("hint = %q", got)
		}
		seq, _ := keymap.Parse("cmd+3", keymap.ModSuper)
		if got, _ := e.keys.Resolve(keymap.CtxGlobal, seq); got != "workbench.action.openEditorAtIndex3" {
			t.Errorf("cmd+3 = %q", got)
		}
		if got := e.commands["revealFileInOS"].Title; got != "在访达中显示" {
			t.Errorf("reveal title = %q", got)
		}
	})
}

func TestShellQuote(t *testing.T) {
	for _, c := range []struct{ goos, in, want string }{
		{"darwin", "/a/b.go", "/a/b.go"},
		{"darwin", "/a b/it's.go", `'/a b/it'\''s.go'`},
		{"linux", "/tmp/中文.txt", "'/tmp/中文.txt'"},
		{"windows", `C:\src\a.go`, `C:\src\a.go`},
		{"windows", `C:\My Files\a.go`, `'C:\My Files\a.go'`},
		{"windows", `C:\O'Brien\$(echo).txt`, `'C:\O''Brien\$(echo).txt'`},
		{"darwin", "/tmp/$(echo).txt", "'/tmp/$(echo).txt'"},
		{"linux", "", "''"},
	} {
		if got := ShellQuote(c.goos, c.in); got != c.want {
			t.Errorf("ShellQuote(%s, %q) = %q, want %q", c.goos, c.in, got, c.want)
		}
	}
}
