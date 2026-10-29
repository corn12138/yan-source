package config

import "github.com/corn12138/yan-source/core/i18n"

// StartLocale chooses the process's interface language, once, before any window, menu or
// updater exists: the preference saved in st or written in the user configuration, resolved
// against the system's ordered language preferences. Every window of the process keeps it;
// a preference saved later applies from the next launch. It returns the problems it skipped,
// in the language it chose.
func StartLocale(st *State, system []string) []string {
	cfg, _ := Load("") // the user's file alone: a project never sets the interface language
	pref, _ := st.LanguagePreference(cfg)
	i18n.SetSystemLanguages(system)
	i18n.SetLocale(i18n.Resolve(pref, system))
	_, warns := st.LanguagePreference(cfg)
	return warns
}
