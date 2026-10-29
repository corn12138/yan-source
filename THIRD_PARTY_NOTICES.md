# 第三方声明

砚（yan）包含或改写了下列项目的代码。按各自许可证的要求，在此保留它们的版权与许可声明。

## Mayoi

- 来源：https://github.com/sorrycc/Mayoi （v0.5.1；后来又从 v0.7.0 移植了下面注明的部分）
- 许可：MIT
- 用到的部分：
  - 整包移植（保留原有测试，改了包名和导入路径）：
    - `core/git/linediff`（`internal/linediff`）、`core/search/glob`（`internal/workspace/glob.go`）：除包名、注释外没有改动。
    - `core/syntax/highlight`（`internal/highlight`）：加了 nil 接收者的检查和说明偏移单位的注释。
    - 提交文件导航参考 `8ae1d55`、`3759c8b` 的 `cmd/mayoi/committree.go`、SCM 展开和测试；重写为砚的仓库绑定动作、共享 Frame、GUI 文件列表与 TUI 历史导航。
    - 查找框重复唤起全选与跨列/跨窗口归属回归，参考 `e57d684`、`9ed2adb` 的 `cmd/mayoi/find.go`、`view.go` 与测试；适配为砚的共享 Frame 请求和 Frontend 所有权检查。
    - `core/git`（`internal/git`）：加了 git 2.45 下从子模块读出整个提交时的处理。
    - `core/syntax/symbols`（`internal/symbols`）：改成读砚的 `text.Snapshot`，列号从 rune 改成字节偏移，测试跟着改。
    - `core/git/commitactions.go` 与测试（快照 `8708da764770` 的 `internal/git/commitactions.go`）：提交补丁、修订解析与远端链接；
      `cmd/mayoi/commitactions.go` 的比较和导出交互重写为 `core/app/commit_tools.go`、`ui/gui/commit_tools.go`。
    - `core/git/submodules.go`：适配 `d28bc2500f47` 的已初始化 gitlink 发现；`core/app/repositories.go`、GUI/TUI 多仓库面板
      参考同版本 `cmd/mayoi/gitcontext.go`、`repositorypicker.go`，重写为砚的共享内核、稳定动作 ID 和语义 Frame。
    - `core/git/nested.go` 与深度/排除规则测试（`d28bc2500f47` 的 `internal/git/nested.go`、`nested_test.go`）：
      改为本地文件系统与可取消扫描；`core/app/nested.go`、`ui/gui/nested.go` 和 TUI 入口适配砚的独立项目生命周期。
    - `core/git/worktree.go`（v0.7.0 的 `internal/git/worktree.go`）：除包注释外没有改动；`Switch` 里认出被别的 worktree 占着的分支。
    - `core/app/graph.go`（`cmd/mayoi/graph.go`，提交图的排布）：只改了包名。
    - `core/keymap/when.go`、`core/keymap/jsonc.go`（v0.7.0 的 `internal/keymap/when.go` 和 `keymap.go` 里的 `StripJSONC`）：
      when 条件与带注释的 JSON，除注释外没有改动。
    - `core/git/git.go` 里的合并、变基、拣选、还原、重置、继续、中止（v0.8.0 的 `internal/git/git.go`）与它们的测试：没有改动；
      v0.8.2 读暂存区用 `:0:<路径>`（点开头的文件夹里的新文件）的修正与它的测试，也没有改动。
    - `internal/crashlog`（v0.8.0 的 `cmd/mayoi/crashlog.go`）与它的测试：适配包名和入口，收紧日志权限并拒绝符号链接日志路径。
    - `core/workspace/watch.go`、`core/app/freshness.go`（`46990a1ca9945de7b90b94104c32f2d7770461f5` 的 `internal/workspace/tree.go` 监听与合并、
      `cmd/mayoi/app.go` 的 `diskChanged`、`cmd/mayoi/search.go` 的 `searchAgain`）：改为砚的所有者 goroutine、项目代号和缓冲区版本校验，
      读文件放到后台；`core/app/keybindings.go` 的运行时 when、`type`/`vscode.open`/`sendSequence` 参数与 `runCommands` 串行等待参考同版本
      `cmd/mayoi/keymap.go` 的 `runBinding`、`pumpChain`，接入砚的守卫命令与 `keymap.Profile`。
    - `core/app/columnlock.go`（同版本 `cmd/mayoi/columnlock.go` 与 `columns.go` 的 `removeColumn`）、`ui/gui/view.go` 的 `tabMiddle`
      （`cmd/mayoi/tabs.go`）：锁定列路由、空列与宽度规则、终端移到锁定右列及中键关闭，改写为砚的列模型与 Frame。
    - `ui/gui/crumbs.go`（`cmd/mayoi/crumbs.go`）与 `ui/gui/projects.go` 的 `railLabels`（`cmd/mayoi/projects.go`），同版本：
      面包屑目录弹出树、项目方块缩写消歧，接入砚的预览接口与项目外观设置。
    - `ui/gui/markdown.go` 与解析测试更新适配至 `46990a1ca9945de7b90b94104c32f2d7770461f5`：提示块、锚点、表格宽度、源码行映射及阅读统计。图像只在本机读取；渲染、预览会话和双向滚动适配砚的 Frame/前端边界。
    - `ui/gui/clawd.go`、`mascot.go`、`rain.go`（`cmd/mayoi/`，来源提交 `e8cfddd`）：Clawd 像素图、姿势时序和 Matrix 数字雨；
      接入砚的主题设置、后台活动与焦点生命周期，省略 Qoder/Qoduck；保留姿势回归测试。
  - 照着改写到砚的内核上：编辑操作与自动闭合（`core/app/editing.go`）、Vim 模式（`core/app/vim.go`）、
    缩进参考线与括号/同词高亮（`decorations.go`）、查找替换（`find.go`）、跨文件搜索（`search.go`）、
    git 标记、行内 blame 与源代码管理（`git.go`、`scm.go`）、改动视图（`diff.go`）、粘性作用域与面包屑（`sticky.go`）、
    主题配色（`core/style/theme.go`）、GUI 工作台的布局、图标与菜单（`ui/gui/...`）；
    主题选择器与颜色过渡参考 `d81bfc4` 的 `cmd/mayoi/themepicker.go`、`theme.go`，改写为 `core/app/themepicker.go`、`ui/gui/overlay.go` 和 `themefade.go`；预览状态独立于持久配置，省略 Qoder/Qoduck；
    从 v0.7.0：排序行、驼峰、缩进方式（`editing.go`）、切到别的 worktree 占着的分支（`scm.go`）、
    提交历史与提交图（`history.go`、`ui/gui/scm.go`）、Windows 与 Linux 的键位和叫法（`platform.go`）、
    触控板横划只沿主方向滚（`ui/gui/code.go`）、VS Code 格式的 keybindings.json（`core/keymap/vscode.go`、`core/app/keybindings.go`）、
    Markdown 预览的排版（`ui/gui/markdownview.go`）；从 v0.8.x：提交上的合并、变基、重置、还原、拣选与冲突时的继续、中止
    （`core/app/gitops.go`、`ui/gui/scm.go`）、空文件夹的资源管理器（`ui/gui/sidebar.go`）、
    项目标签（`core/app/projects.go`、`ui/gui/projects.go`）、
    全文概览条、标签拖动、分栏调整、单标签换行与编辑区终端的交互（`core/app/overview.go`、`tabs.go`、
    `wrap.go`、`terminals.go`、`ui/gui/panedrop.go`、`terminal.go`、`termtabs.go`）；
    窗口复用、Dock 最近目录、侧边栏位置、状态栏设置与显式命令安装（`ui/gui/open.go`、`core/app/document_settings.go`、`internal/cli`）。
    - Windows 子进程控制台抑制（`ce553d24b746` 的 `internal/vfs/hidewindow_windows.go`，改为 `internal/process`，覆盖三个 Git 进程入口）。
    - 行级评审、引用迁移与终端交接（`cmd/mayoi/review.go`，`7be5390a752e`、`b4fff084971c`、`461d0138915f`；
      参考快照 `8708da764770`）：重写为 `core/review`、`core/app/review.go` 与 `ui/gui/review.go`，保留共享内核和平台键位。
  - `core/git/panels.go`（`internal/git/panels.go` 的 `PanelBranches`、`LogRef`、`LogRange`、`RemoteURLs`、
    `ConfiguredRemoteURLs` 与参数检查）：分支结构改名为 `PanelBranch`；仓库面板（`cmd/mayoi/scmpanels.go`）的
    懒读取、翻页与代号丢弃改写为 `core/app/repopanel.go`、`ui/gui/repopanel.go`。
  - 状态栏 Git 同步（[#53](https://github.com/sorrycc/Mayoi/pull/53)，yushengyu、yunqian）：`internal/git` 的 `Sync` 移植为 `core/git/manage.go`；
    Linux 菜单替换推迟到绘制之后（[#55](https://github.com/sorrycc/Mayoi/pull/55)）改写为 `ui/gui/appmenu.go`。
  - 每小时检查更新并在状态栏、菜单、欢迎页提示（`e2dabc1` 的 `cmd/mayoi/updates.go`）改写为 `ui/gui/updates.go`；
    开放 PR [#60](https://github.com/sorrycc/Mayoi/pull/60)（xierenyuan）的常驻「新建分支…」与名字校验按行为重写为 `core/app/branches.go`。
  - 并排两窗格、同步滚动与行对齐开关、沟槽弯带与箭头、平滑跳转与写入发光（v0.11.5 / `23d597f` 的 `cmd/mayoi/diffview.go`、
    `diffpaint.go`）按行为改写为 `core/app/splitdiff.go`、`ui/gui/splitdiff.go`。
  - 开放 PR [#57](https://github.com/sorrycc/Mayoi/pull/57)（bet4it）的标题栏菜单按钮、Linux 窗口按钮的 GTK 样式与外观选择修正，
    按行为重写为 `ui/gui/appmenu.go` 的 `populateMenu`、`ui/gui/titlebar_linux.go` 与 `ui/gui/theme.go`。
  - [PR #50](https://github.com/sorrycc/Mayoi/pull/50)（2026-10-20 已合并进 Mayoi 主线 `91a8b15`）（作者 Garfielk / Elijah Azulay，IDE 式差异与三方合并，
    头提交 `997e322`）：按行为重写，不复制代码。空白过滤与任意两个文件比较（`compareDiffLines`、`withoutBlankHunks`、
    Compare Two Files）改写为 `core/app/diffcompare.go`。改动/文件导航的边界提示与跨文件（`stepHunk`、`stepFile`、`diffCrossFiles`）改写为 `core/app/diffnav.go`。词/字/行高亮与上下文行数（`diffHighlight`、`diffContext`）改写为 `core/app/diff.go` 的 `diffOpts`；`core/git/linediff` 的 `Chars` 照着该 PR 的同名函数。块还原/追加与比较两边互写（`applyArrow`、`undoDiff`）改写为 `core/app/diffblock.go`、`ui/gui/diffblock.go`。三方合并视图（`mergeRegions`、`sync`、`takeMergeSide`、`dropMergeSide`、`resolveUsingMerge`、`completeMerge`、`restoreMerge`）改写为 `core/app/mergeregions.go`、`core/app/merge.go`、`core/git/conflict.go`，以砚的列显示三栏。冲突文件清单与跨重启记录（`mergefiles.go`）改写为 `core/app/mergefiles.go`。PR 的代码版权归其作者，按 Mayoi 的 MIT 许可提交。
  - 默认终端位置按 [d81abe4](https://github.com/sorrycc/Mayoi/commit/d81abe43b4a142ccc7a6e59be6d486402f7c0691)（yunqian）的行为改写为 `core/app/terminals.go`、配置与 GUI 菜单；采用砚的 TOML/state 优先级，保留 TUI shell 回退和锁定列路由。
  - 终端文件路径解析改写自开放 [PR #77](https://github.com/sorrycc/Mayoi/pull/77)（xierenyuan / 阿平，`fce5318`）的 `cmd/mayoi/terminallink.go`，落在 `ui/gui/terminalfiles.go`；行列定位由共享内核处理。文件剪贴板优先粘贴参考 `e18f9ba`，沿用砚的平台 shell 引用规则及 MyGo 的 `OnPaste`。
  - 转到符号遇到行首 `func() {` 时越界的修正照开放 [PR #83](https://github.com/sorrycc/Mayoi/pull/83)（NotAFlightRisk，`b36c7d0`），落在 `core/syntax/symbols/symbols.go`。
  - Git/hooks 的登录 shell PATH 恢复参考 [`2650b9b`](https://github.com/sorrycc/Mayoi/commit/2650b9bfa90c87ffe0d48fffb140dd2fc428ff1c)（yunqian），按砚的 Git 子进程边界重写；仅 GUI 启用，探测与子进程清理有界，不改全局环境。
  - Markdown 预览的脚注改写自 [`88e00cf`](https://github.com/sorrycc/Mayoi/commit/88e00cfaeb4c52b33a74e4eefc70dec92ab8720b)，标点后的裸网址成链接照 [`3d03b96`](https://github.com/sorrycc/Mayoi/commit/3d03b96c4f8778469f95c3e4cf34f0ebf93ead20)（`ui/gui/markdownlinkify.go`），Mermaid 样式填充上的标签换色照 [`b906ff6`](https://github.com/sorrycc/Mayoi/commit/b906ff61990603dfa0f8169791aa6cb2032ce6fc)（均为 yunqian）。
  - 每个相关文件开头都注明了出处。

```text
MIT License

Copyright (c) 2026 chencheng <sorrycc@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## MyGo update archive format

`internal/updatepack` adapts the archive writer from MyGo `internal/update/update.go`. The GUI uses
MyGo's native updater plugin and its download/signature/replacement implementation.

```text
MIT License

Copyright (c) 2026 MyGo contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Lucide

- 来源：https://lucide.dev （`ui/gui/icons.go` 里的图标路径）
- 许可：ISC

```text
ISC License

Copyright (c) for portions of Lucide are held by Cole Bemis 2013-2022 as part of Feather (MIT).
All other copyright (c) for Lucide are held by Lucide Contributors 2022.

Permission to use, copy, modify, and/or distribute this software for any
purpose with or without fee is hereby granted, provided that the above
copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY
SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
```

## Pele

- 来源：https://github.com/obsidianmd/pele （1.9.0，npm 包 `pele`）
- 用到的部分：`ui/gui/pele.js.gz`，由 `scripts/pele.sh` 用 esbuild 打包成一个脚本、原样嵌进窗口版，在 goja 里运行，
  把 Mermaid 图排成 SVG。脚本开头保留了下面的许可声明；其中缩放控件的图标来自 Lucide v1.14.0（ISC，见上一节）。
  只运行这份打包的脚本；文档里的 Mermaid 文字只作为数据交给它，砚不执行文档里的 JavaScript。
- 许可：MIT

```text
MIT License

Copyright (c) 2026 kepano

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## 编进二进制的 Go 模块、随包附带的原生库

发布的二进制里实际链接了哪些 Go 模块，由 `internal/tools/notices` 读二进制里的构建信息得出，
连同各模块的 LICENSE / NOTICE 原文、Go 标准库的许可，写进 `THIRD_PARTY_LICENSES.txt`：
它在 `yan.app/Contents/Resources/` 里，也作为发布页上的单独文件提供（`make notices` 生成）。主要的几个：

| 模块 | 许可 |
|---|---|
| `github.com/egoist/mygo`（GUI 前端） | MIT |
| `github.com/ebitengine/purego`（MyGo 调系统 API） | Apache-2.0 |
| `github.com/alecthomas/chroma/v2`（语法高亮） | MIT |
| `github.com/gdamore/tcell/v3`（终端前端） | Apache-2.0 |
| `github.com/go-git/go-git/v5`（读 git 忽略规则） | Apache-2.0 |

窗口版的内嵌终端在运行时加载 libghostty-vt（`yan.app/Contents/Resources/libghostty-vt.dylib`）。它由 MyGo v0.4.1
指定、从 Ghostty 提交 `befcdfd2c3a1` 编译，里面静态链接了 simdutf、Highway、uucode 和 Zig 标准库。
它们的版权与许可原文在 [`packaging/licenses/`](packaging/licenses/)，也一并写进 `THIRD_PARTY_LICENSES.txt`。
源码仓库本身不包含 libghostty-vt。

砚自己的代码采用 [MIT 许可](LICENSE)；本文件保留第三方部分各自的版权与许可。

## Yan

Yan's own source uses the following license. Third-party portions retain the notices above.

```text
MIT License

Copyright (c) 2026 corn12138

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## MathJax

- MathJax 3.2.2: https://github.com/mathjax/MathJax-src/tree/3.2.2
- Copyright (c) 2009-2021 The MathJax Consortium. Licensed under Apache License 2.0.
- `ui/gui/mathjax.js.gz` bundles the TeX-to-SVG renderer and its path fonts for offline formula previews. `scripts/mathjax.sh` reproduces the bundle; the full license is in `packaging/licenses/mathjax.txt` and the compressed bundle banner, and is included in distributed notices.
- The formula parser and positioning in `ui/gui/math.go`, `markdown.go`, `markdownview.go`, their tests, and the bundling script adapt Mayoi commit `1010308b542c4bfa25e15f84f4992914fac04223` (MIT, Copyright (c) 2026 chencheng). Yan adds configuration layers, cancellation, bounded rendering, macro isolation, and its own preview editing and clipboard integration.
