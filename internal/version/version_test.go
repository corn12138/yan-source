package version

import "testing"

func TestLabel(t *testing.T) {
	for v, want := range map[string]string{
		"v0.1.0-alpha.2":       "预览版 v0.1.0-alpha.2",
		"v1.0.0":               "v1.0.0",
		"(devel)":              "开发版",
		"":                     "开发版",
		"v0.1.0-alpha.2+dirty": "开发版",
		"v0.0.0-20261009155342-a442fb4181f3+dirty":     "开发版",
		"v0.1.0-alpha.2.0.20261013091928-47e3f2dac460": "开发版",
	} {
		if got := labelFor(v); got != want {
			t.Errorf("labelFor(%q) = %q, want %q", v, got, want)
		}
	}
}
