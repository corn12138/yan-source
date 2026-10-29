// Package archtest 守住架构边界：core 不依赖任何 UI 工具包；两个前端互不依赖；
// 内核、终端版和窗口版都不含 cgo（MyGo 用 purego 调系统 API，libghostty-vt 运行时加载）。
package archtest

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

const mod = "github.com/corn12138/yan-source"

func deps(t *testing.T, patterns ...string) []string {
	t.Helper()
	args := append([]string{"list", "-deps", "-f",
		`{{.ImportPath}}{{if and (not .Standard) .CgoFiles}} [cgo]{{end}}`}, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(out)), "\n")
}

func TestCoreIsHeadless(t *testing.T) {
	for _, d := range deps(t, mod+"/core/...") {
		if strings.HasPrefix(d, "github.com/gdamore/tcell") || strings.HasPrefix(d, "gioui.org") ||
			strings.HasPrefix(d, "github.com/egoist/mygo") ||
			strings.HasPrefix(d, mod+"/ui") || strings.HasPrefix(d, mod+"/platform") {
			t.Errorf("core 依赖了界面或平台包：%s", d)
		}
	}
}

func TestFrontendsStayApart(t *testing.T) {
	for _, d := range deps(t, mod+"/ui/tui") {
		if strings.HasPrefix(d, "github.com/egoist/mygo") || strings.HasPrefix(d, mod+"/ui/gui") {
			t.Errorf("终端前端依赖了窗口前端：%s", d)
		}
	}
	for _, d := range deps(t, mod+"/ui/gui") {
		if strings.HasPrefix(d, "github.com/gdamore/tcell") || strings.HasPrefix(d, mod+"/ui/tui") {
			t.Errorf("窗口前端依赖了终端前端：%s", d)
		}
	}
}

// optionalCgo 是有 cgo 时才用、没有时有纯 Go 替代的包：在 Linux 上 purego 能用 cgo 时用 cgo 的 dlopen，
// CGO_ENABLED=0（Makefile 一律这样编）时用它自己的 fakecgo。以后的 Linux、Windows 窗口版也这样编。
var optionalCgo = map[string]bool{"github.com/ebitengine/purego/internal/cgo": true}

func TestEverythingIsCgoFree(t *testing.T) {
	for _, d := range deps(t, mod+"/core/...", mod+"/ui/...", mod+"/platform/...", mod+"/cmd/...") {
		if strings.HasSuffix(d, " [cgo]") && !optionalCgo[strings.TrimSuffix(d, " [cgo]")] {
			t.Errorf("出现 cgo 依赖：%s", d)
		}
	}
}
