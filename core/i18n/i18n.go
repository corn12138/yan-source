// Package i18n holds Yan's interface languages: the preference a user saves, the locale the
// process runs in, and the message catalogs.
//
// Messages are keyed by their English text, which is also the fallback: a message missing from a
// catalog shows in English. A key that would mean two things takes a context, written
// "context\x04text" (gettext's convention) and looked up with TC. Translate where display text is
// built, never at package initialization: the locale is chosen once per process, in main, before
// the first window, menu or updater exists (SetLocale), and stays the same until the next launch.
package i18n

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// Locale is an interface language Yan has a catalog for.
type Locale string

const (
	English           Locale = "en"
	SimplifiedChinese Locale = "zh-Hans"
)

// Locales are the supported interface languages, in the order settings list them.
var Locales = []Locale{SimplifiedChinese, English}

// NativeName is the language's name in itself, which stays readable whatever language the
// interface is in, so a user can find their way back from an unfamiliar one.
func (l Locale) NativeName() string {
	switch l {
	case SimplifiedChinese:
		return "简体中文"
	}
	return "English"
}

// Preference is the interface language a user chose: System or a Locale.
type Preference string

// System follows the operating system's ordered language preferences.
const System Preference = "system"

// ParsePreference reads a saved or configured preference. Only the canonical values are
// accepted, so an arbitrary identifier never names a catalog.
func ParsePreference(s string) (Preference, bool) {
	switch Preference(s) {
	case System, Preference(English), Preference(SimplifiedChinese):
		return Preference(s), true
	}
	return "", false
}

// Locale is the language the preference names; false for System.
func (p Preference) Locale() (Locale, bool) {
	switch p {
	case Preference(English), Preference(SimplifiedChinese):
		return Locale(p), true
	}
	return "", false
}

// Resolve is the locale of preference p on a system whose ordered language preferences are
// system: an explicit choice wins, System takes the first supported system language, and
// English is the fallback.
func Resolve(p Preference, system []string) Locale {
	if l, ok := p.Locale(); ok {
		return l
	}
	for _, tag := range system {
		if l, ok := Match(tag); ok {
			return l
		}
	}
	return English
}

var (
	active atomic.Pointer[Locale]
	// systemTags are the system's language preferences the process launched with.
	systemTags atomic.Pointer[[]string]
)

// SetLocale chooses the process's locale; main calls it once, before building any interface.
func SetLocale(l Locale) {
	if l != SimplifiedChinese {
		l = English
	}
	active.Store(&l)
}

// Current is the process's locale (English until SetLocale).
func Current() Locale {
	if l := active.Load(); l != nil {
		return *l
	}
	return English
}

// SetSystemLanguages keeps the system's language preferences the process launched with, for
// settings to name the language Follow System resolves to.
func SetSystemLanguages(tags []string) {
	tags = append([]string(nil), tags...)
	systemTags.Store(&tags)
}

// SystemLanguages are the preferences SetSystemLanguages kept.
func SystemLanguages() []string {
	if t := systemTags.Load(); t != nil {
		return *t
	}
	return nil
}

// T is msg in the current locale.
func T(msg string) string { return In(Current(), msg) }

// TC is msg in the current locale, where the same English means something else elsewhere.
func TC(context, msg string) string { return In(Current(), context+"\x04"+msg) }

// F formats the translation of format with args.
func F(format string, args ...any) string { return fmt.Sprintf(T(format), args...) }

// N marks msg for the catalog checks without translating it: for tables built before the
// locale is chosen, whose entries are passed through T where they are shown.
func N(msg string) string { return msg }

// In is msg in locale l: its translation, else its English text.
func In(l Locale, msg string) string {
	if l == SimplifiedChinese {
		if s, ok := zhHans[msg]; ok {
			return s
		}
	}
	if _, text, ok := strings.Cut(msg, "\x04"); ok {
		return text
	}
	return msg
}

// Error is an error whose message is translated when it is read, so a package-level error
// keeps its identity for errors.Is and still shows in the process's locale.
func Error(msg string) error { return lazyError(msg) }

type lazyError string

func (e lazyError) Error() string { return T(string(e)) }

// Preference is the explicit choice of l.
func (l Locale) Preference() Preference { return Preference(l) }

// Join lists items as a sentence of the current locale does: "a, b" or "a、b".
func Join(items []string) string { return strings.Join(items, TC("list separator", ", ")) }
