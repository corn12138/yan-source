package app

// Choosing the code font, its size and the line height from the window (the palette's 字体
// commands and the View menu), instead of only from [ui] in config.toml. The choice is kept in
// the state file like the other menu choices, and applies at once. Only the window draws fonts:
// the terminal version follows its terminal's.

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/corn12138/yan-source/core/config"
	"github.com/corn12138/yan-source/core/i18n"
)

// codeFont is a monospaced family the picker offers when its files are installed: files are
// the lower-case starts of their file names, without spaces, hyphens or underscores. CJK fonts
// draw Chinese at two columns, so comments in Chinese keep the code's grid.
type codeFont struct {
	family string
	files  []string
	cjk    bool
}

var codeFonts = []codeFont{
	{"Menlo", []string{"menlo"}, false},
	{"Monaco", []string{"monaco"}, false},
	{"SF Mono", []string{"sfmono"}, false},
	{"JetBrains Mono", []string{"jetbrainsmono"}, false},
	{"Fira Code", []string{"firacode"}, false},
	{"Fira Mono", []string{"firamono"}, false},
	{"Cascadia Code", []string{"cascadiacode"}, false},
	{"Cascadia Mono", []string{"cascadiamono"}, false},
	{"Consolas", []string{"consola"}, false},
	{"Source Code Pro", []string{"sourcecodepro"}, false},
	{"IBM Plex Mono", []string{"ibmplexmono"}, false},
	{"Roboto Mono", []string{"robotomono"}, false},
	{"Ubuntu Mono", []string{"ubuntumono"}, false},
	{"DejaVu Sans Mono", []string{"dejavusansmono"}, false},
	{"Liberation Mono", []string{"liberationmono"}, false},
	{"Hack", []string{"hackregular", "hacknerdfont"}, false},
	{"Inconsolata", []string{"inconsolata"}, false},
	{"Iosevka", []string{"iosevka"}, false},
	{"Victor Mono", []string{"victormono"}, false},
	{"Maple Mono", []string{"maplemono"}, false},
	{"Courier New", []string{"couriernew", "cour."}, false},
	{"Sarasa Mono SC", []string{"sarasa"}, true},
	{"Maple Mono NF CN", []string{"maplemononfcn"}, true},
	{"LXGW WenKai Mono", []string{"lxgwwenkaimono"}, true},
	{"Noto Sans Mono CJK SC", []string{"notosansmonocjk", "notosanscjk"}, true},
}

// fontDirs are where this system keeps installed fonts (a variable for tests).
var fontDirs = func() []string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []string{"/System/Library/Fonts", "/Library/Fonts", filepath.Join(home, "Library/Fonts")}
	case "windows":
		dirs := []string{filepath.Join(os.Getenv("WINDIR"), "Fonts")}
		if l := os.Getenv("LOCALAPPDATA"); l != "" {
			dirs = append(dirs, filepath.Join(l, "Microsoft", "Windows", "Fonts"))
		}
		return dirs
	}
	return []string{"/usr/share/fonts", "/usr/local/share/fonts", filepath.Join(home, ".local/share/fonts"), filepath.Join(home, ".fonts")}
}

// installedCodeFonts are the families of codeFonts with a file in fontDirs.
func installedCodeFonts() []codeFont {
	var names []string
	for _, dir := range fontDirs() {
		filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				n := strings.ToLower(d.Name())
				names = append(names, strings.NewReplacer(" ", "", "-", "", "_", "").Replace(n))
			}
			return nil
		})
	}
	var out []codeFont
	for _, f := range codeFonts {
		if slices.ContainsFunc(names, func(n string) bool {
			return slices.ContainsFunc(f.files, func(p string) bool { return strings.HasPrefix(n, p) })
		}) {
			out = append(out, f)
		}
	}
	return out
}

// fontsInWindow reports whether fonts can be chosen here; the terminal version says why not.
func (e *Editor) fontsInWindow() bool {
	if !e.profile.GUI {
		e.status = i18n.T("The terminal version uses the terminal's own font: change it in the terminal's settings")
		return false
	}
	return true
}

// setFont applies and keeps a code font list, size or line height (zero values stay).
func (e *Editor) setFont(family string, size, lineHeight float64) {
	if family != "" {
		e.cfg.FontFamily = family
	}
	if size > 0 {
		e.cfg.FontSize = size
	}
	if lineHeight > 0 {
		e.cfg.LineHeight = lineHeight
	}
	if e.state != nil {
		if family != "" {
			e.state.FontFamily = &family
		}
		if size > 0 {
			e.state.FontSize = &size
		}
		if lineHeight > 0 {
			e.state.LineHeight = &lineHeight
		}
		e.saveState()
	}
	e.settingsGen++
}

// chooseFont lists the code fonts installed here, each shown in itself, with the system's own
// monospaced font and a typed list of families.
func (e *Editor) chooseFont() error {
	if !e.fontsInWindow() {
		return nil
	}
	cur := e.cfg.FontFamily
	item := func(label, family, detail string) ovItem {
		it := ovItem{label: label, detail: detail, font: family, run: func(e *Editor) error {
			e.setFont(family, 0, 0)
			e.status = i18n.F("Code font: %s", label)
			return nil
		}}
		if family == cur {
			it.hint = i18n.T("Current")
		}
		return it
	}
	items := []ovItem{item(i18n.T("System monospace font"), "monospace", "")}
	for _, f := range installedCodeFonts() {
		detail := ""
		if f.cjk {
			detail = i18n.T("Chinese at fixed width, aligned")
		}
		items = append(items, item(f.family, f.family, detail))
	}
	if !slices.ContainsFunc(items, func(it ovItem) bool { return it.font == cur }) {
		items = append(items, item(cur, cur, i18n.T("From config.toml")))
	}
	items = append(items, ovItem{label: i18n.T("Enter Font Name…"), detail: i18n.T("Separate several with commas; characters a font lacks use the next one"), run: func(e *Editor) error {
		e.prompt(i18n.T("Code font (comma-separated, in fallback order)"), cur, func(in string) string { return i18n.F("Use %s", strings.TrimSpace(in)) }, func(e *Editor, in string) error {
			if in = strings.TrimSpace(in); in != "" {
				e.setFont(in, 0, 0)
				e.status = i18n.F("Code font: %s", in)
			}
			return nil
		})
		return nil
	}})
	e.choose(i18n.T("Code Font"), items...)
	return nil
}

// chooseFontSize lists the code font's sizes.
func (e *Editor) chooseFontSize() error {
	if !e.fontsInWindow() {
		return nil
	}
	var items []ovItem
	for size := 10.0; size <= 24; size++ {
		label := i18n.F("%g pt", size)
		if size == config.Default().FontSize {
			label += i18n.T(" (Default)")
		}
		it := ovItem{label: label, run: func(e *Editor) error {
			e.setFont("", size, 0)
			e.status = i18n.F("Code font size: %g", size)
			return nil
		}}
		if size == e.cfg.FontSize {
			it.hint = i18n.T("Current")
		}
		items = append(items, it)
	}
	e.choose(i18n.T("Code Font Size"), items...)
	return nil
}

// chooseLineHeight lists line heights, as multiples of the font size.
func (e *Editor) chooseLineHeight() error {
	if !e.fontsInWindow() {
		return nil
	}
	var items []ovItem
	for _, lh := range []float64{1.2, 1.3, 1.4, 1.5, 1.55, 1.6, 1.7, 1.8, 2} {
		label := i18n.F("%g×", lh)
		if lh == config.Default().LineHeight {
			label += i18n.T(" (Default)")
		}
		it := ovItem{label: label, run: func(e *Editor) error {
			e.setFont("", 0, lh)
			e.status = i18n.F("Line height: %g×", lh)
			return nil
		}}
		if lh == e.cfg.LineHeight {
			it.hint = i18n.T("Current")
		}
		items = append(items, it)
	}
	e.choose(i18n.T("Line Height"), items...)
	return nil
}
