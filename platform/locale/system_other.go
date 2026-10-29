//go:build !darwin

package locale

// System is the user's language preferences from the locale environment. Windows and Linux
// desktop editions will read their desktop's settings through their own providers here.
func System() []string { return Terminal() }
