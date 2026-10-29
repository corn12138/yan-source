// Package locale reads the operating system's language preferences, most preferred first, for
// the interface language to follow. Matching them to a catalog is core/i18n's job.
package locale

import "github.com/corn12138/yan-source/core/i18n"

// Terminal is the language preferences of the terminal edition: the POSIX locale environment the
// shell gives it (LANGUAGE, LC_ALL, LC_MESSAGES, LANG), on every system, since a terminal
// program's text follows the terminal's locale rather than the desktop's.
func Terminal() []string { return i18n.EnvLanguages(nil) }
