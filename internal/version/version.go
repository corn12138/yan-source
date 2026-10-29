// Package version 给出砚的版本：发布构建从打了标签的提交编译，Go 会把标签和提交号记进二进制
// （go build 的 VCS 信息），不需要另外传 -ldflags。
package version

import (
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/corn12138/yan-source/core/i18n"
)

// String 返回版本和提交号，如「v0.1.0-alpha.1（3f2a9c1e0b7d）」；不是从 git 仓库编译时是「dev」。
func String() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	v := "dev"
	if m := info.Main.Version; m != "" && m != "(devel)" {
		v = m
	}
	var rev string
	dirty := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value[:min(12, len(s.Value))]
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return v
	}
	if dirty && !strings.Contains(v, "dirty") {
		return i18n.F("%s (%s, with uncommitted changes)", v, rev)
	}
	return i18n.F("%s (%s)", v, rev)
}

// Label 是界面上的版本标签：打了预发布标签的构建是「预览版 v0.1.0-alpha.2」，正式版是「v1.0.0」，
// 没打标签、有未提交修改或不是从 git 仓库编译的是「开发版」。
func Label() string {
	v := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		v = info.Main.Version
	}
	return labelFor(v)
}

// pseudo 匹配 Go 给没打标签的提交编的伪版本号里的时间和提交号，如 v0.0.0-20261009155342-a442fb4181f3。
var pseudo = regexp.MustCompile(`[-.]\d{14}-[0-9a-f]{12}`)

func labelFor(v string) string {
	switch {
	case v == "" || v == "(devel)" || strings.Contains(v, "+dirty") || pseudo.MatchString(v):
		return i18n.T("Development Build")
	case strings.Contains(v, "-"):
		return i18n.F("Preview build %s", v)
	}
	return v
}

// Release returns only a clean tagged version, so development builds never compare as releases.
func Release() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if labelFor(info.Main.Version) != i18n.T("Development Build") {
			return strings.TrimPrefix(info.Main.Version, "v")
		}
	}
	return ""
}
