// Command notices 写出随二进制分发的第三方声明：THIRD_PARTY_NOTICES.md、随包附带的原生库的许可
// （packaging/licenses），再加上每个二进制里实际链接的 Go 模块（读二进制里的构建信息）的
// LICENSE / NOTICE 原文和 Go 标准库的许可。找不到某个模块的许可文件时报错，不写残缺的声明。
//
//	go run ./internal/tools/notices -o build/THIRD_PARTY_LICENSES.txt [-native packaging/licenses] bin/yan bin/yan-app
package main

import (
	"bytes"
	"debug/buildinfo"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

func main() {
	out := flag.String("o", "", "输出文件")
	native := flag.String("native", "", "放原生库许可的目录（窗口版带着 libghostty-vt 时给）")
	notices := flag.String("notices", "THIRD_PARTY_NOTICES.md", "仓库里的第三方声明")
	flag.Parse()
	if *out == "" || flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "用法：notices -o 输出 [-native 目录] 二进制…")
		os.Exit(2)
	}
	if err := run(*out, *native, *notices, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "notices:", err)
		os.Exit(1)
	}
}

type module struct{ path, version string }

func run(out, native, notices string, bins []string) error {
	var b bytes.Buffer
	b.WriteString("yan（砚）第三方声明与许可\n")
	b.WriteString("本文件随二进制一起分发。它列出编进或随包附带的第三方代码，以及它们的版权与许可原文。\n")
	fmt.Fprintf(&b, "适用于：%s\n\n", strings.Join(baseNames(bins), "、"))

	head, err := os.ReadFile(notices)
	if err != nil {
		return err
	}
	section(&b, "仓库里的第三方声明（THIRD_PARTY_NOTICES.md）")
	b.Write(head)

	if native != "" {
		files, err := filepath.Glob(filepath.Join(native, "*.txt"))
		if err != nil || len(files) == 0 {
			return fmt.Errorf("%s 里没有原生库的许可", native)
		}
		slices.Sort(files)
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				return err
			}
			section(&b, "随包附带的原生库："+strings.TrimSuffix(filepath.Base(f), ".txt"))
			b.Write(data)
		}
	}

	mods, goVersion, err := linked(bins)
	if err != nil {
		return err
	}
	goroot, err := goEnv("GOROOT")
	if err != nil {
		return err
	}
	lic, err := os.ReadFile(filepath.Join(goroot, "LICENSE"))
	if err != nil {
		return err
	}
	section(&b, "Go 标准库与运行时（"+goVersion+"）")
	b.Write(lic)

	cache, err := goEnv("GOMODCACHE")
	if err != nil {
		return err
	}
	for _, m := range mods {
		dir := filepath.Join(cache, escape(m.path)+"@"+escape(m.version))
		texts, err := licenseFiles(dir)
		if err != nil {
			return fmt.Errorf("%s@%s：%w（先 go mod download）", m.path, m.version, err)
		}
		section(&b, "Go 模块 "+m.path+" "+m.version)
		for i, t := range texts {
			if i > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "---- %s ----\n", t.name)
			b.Write(t.data)
		}
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, b.Bytes(), 0o644)
}

func section(b *bytes.Buffer, title string) {
	if !bytes.HasSuffix(b.Bytes(), []byte("\n")) {
		b.WriteString("\n")
	}
	b.WriteString("\n" + strings.Repeat("=", 78) + "\n" + title + "\n" + strings.Repeat("=", 78) + "\n\n")
}

// linked 读出二进制里链接的模块（去重、排序）和编译它们的 Go 版本。
func linked(bins []string) ([]module, string, error) {
	seen := map[module]bool{}
	var goVersion string
	for _, bin := range bins {
		info, err := buildinfo.ReadFile(bin)
		if err != nil {
			return nil, "", fmt.Errorf("%s：%w", bin, err)
		}
		goVersion = info.GoVersion
		for _, d := range info.Deps {
			if d.Replace != nil {
				d = d.Replace
			}
			seen[module{d.Path, d.Version}] = true
		}
	}
	var mods []module
	for m := range seen {
		mods = append(mods, m)
	}
	slices.SortFunc(mods, func(a, b module) int { return strings.Compare(a.path, b.path) })
	return mods, goVersion, nil
}

type text struct {
	name string
	data []byte
}

// licenseFiles 返回模块根目录里的许可与声明文件。
func licenseFiles(dir string) ([]text, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []text
	for _, e := range entries {
		n := strings.ToUpper(e.Name())
		if e.IsDir() || !(strings.HasPrefix(n, "LICENSE") || strings.HasPrefix(n, "LICENCE") ||
			strings.HasPrefix(n, "COPYING") || strings.HasPrefix(n, "NOTICE") || strings.HasPrefix(n, "UNLICENSE")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		out = append(out, text{e.Name(), data})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s 里没有许可文件", dir)
	}
	return out, nil
}

// escape 按模块缓存的规则转义路径：大写字母写成 ! 加小写。
func escape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsUpper(r) {
			b.WriteByte('!')
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func goEnv(key string) (string, error) {
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return "", fmt.Errorf("go env %s：%w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func baseNames(paths []string) []string {
	var out []string
	for _, p := range paths {
		out = append(out, filepath.Base(p))
	}
	return out
}
