module github.com/corn12138/yan-source

go 1.27.1

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/charlievieth/fastwalk v1.0.14
	github.com/clipperhouse/displaywidth v0.11.0
	github.com/clipperhouse/uax29/v2 v2.7.0
	github.com/dop251/goja v0.0.0-20261024144015-2a9269b6b6de
	github.com/ebitengine/purego v0.11.1
	github.com/egoist/mygo v0.4.8
	github.com/fsnotify/fsnotify v1.10.1
	github.com/gdamore/tcell/v3 v3.5.0
	github.com/go-git/go-git/v5 v5.19.3
	github.com/sahilm/fuzzy v0.1.3
	github.com/yuin/goldmark v1.7.13
	golang.org/x/net v0.57.0
	golang.org/x/sys v0.48.0
)

require (
	github.com/dlclark/regexp2/v2 v2.8.1 // indirect
	github.com/gdamore/encoding v1.0.1 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.9.2 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/google/pprof v0.0.0-20230207041349-798e818bf904 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	golang.org/x/image v0.46.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

tool github.com/egoist/mygo/cmd/mygo

// 和 Mayoi v0.8.2 一样用 sorrycc/mygo：MyGo v0.4.8 加上终端的三个提交——选中即复制（egoist/mygo#290，已合进 MyGo）、
// 剪贴板里只有图片时 ⌘V 发一个空的括起来的粘贴（#344）、「全选」也复制并且能在开着的终端里改（#345）。
// #344、#345 合进 MyGo 发布之后，去掉这一行，回到 egoist/mygo。见 docs/DECISIONS.md。
replace github.com/egoist/mygo => github.com/sorrycc/mygo v0.4.9-0.20261014065302-a816c1636ddf
