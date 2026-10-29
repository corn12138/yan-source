package keymap

import "testing"

func TestWhen(t *testing.T) {
	ctx := func(n string) bool { return n == "a" || n == "b" }
	for expr, want := range map[string]bool{
		"a":                        true,
		"c":                        false,
		"!c":                       true,
		"a && b":                   true,
		"a && c":                   false,
		"c || b":                   true,
		"!(a && c) && b":           true,
		"c || a && !b":             false,
		"resourceExtname == .go":   false,
		"resourceExtname != '.go'": true,
		"terminalFocus && terminalHasBeenCreated || terminalFocus && terminalProcessSupported": false,
	} {
		w, err := ParseWhen(expr)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		if got := w.Eval(ctx); got != want {
			t.Errorf("%s = %v", expr, got)
		}
	}
	for _, bad := range []string{"a &&", "(a", "a )", "&& a"} {
		if _, err := ParseWhen(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}
