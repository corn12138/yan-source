package i18n

import (
	"os"
	"strings"
)

// Match is the supported locale a language identifier names: a BCP 47 tag ("zh-Hans-CN",
// "en-GB") or a POSIX locale ("zh_CN.UTF-8", "en_US@euro"). An explicit script wins over the
// region, so zh-Hant-CN is Traditional Chinese, which this release does not support; a bare
// "zh" and the Simplified regions CN and SG are Simplified Chinese.
func Match(tag string) (Locale, bool) {
	tag, _, _ = strings.Cut(tag, ".") // zh_CN.UTF-8
	tag, _, _ = strings.Cut(tag, "@") // en_US@euro
	parts := strings.FieldsFunc(tag, func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 {
		return "", false
	}
	lang := strings.ToLower(parts[0])
	var script, region string
	for _, p := range parts[1:] {
		switch {
		case len(p) == 4 && isAlpha(p) && script == "" && region == "":
			script = strings.ToLower(p)
		case (len(p) == 2 && isAlpha(p) || len(p) == 3 && isDigits(p)) && region == "":
			region = strings.ToUpper(p)
		}
	}
	switch lang {
	case "en":
		return English, true
	case "zh":
		switch script {
		case "hans":
			return SimplifiedChinese, true
		case "":
			switch region {
			case "", "CN", "SG":
				return SimplifiedChinese, true
			}
		}
	}
	return "", false
}

func isAlpha(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// EnvLanguages are the language preferences of a POSIX environment, most preferred first, by
// gettext's precedence: LANGUAGE (a colon-separated list, honoured only when a locale is set),
// then LC_ALL, LC_MESSAGES and LANG. The "C" and "POSIX" locales name no language.
func EnvLanguages(getenv func(string) string) []string {
	if getenv == nil {
		getenv = os.Getenv
	}
	var locale string
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(k); v != "" {
			locale = v
			break
		}
	}
	if locale == "" || locale == "C" || locale == "POSIX" || strings.HasPrefix(locale, "C.") {
		return nil
	}
	var tags []string
	for _, t := range strings.Split(getenv("LANGUAGE"), ":") {
		if t != "" {
			tags = append(tags, t)
		}
	}
	return append(tags, locale)
}
