# 砚 yan

极简、轻快的代码与文档编辑器。Go 写成，同一个内核有终端版（`yan`）和 macOS 窗口版（`yan-app`），交互照着 VS Code 来。

源码采用 [MIT 许可](LICENSE)。公开源码从 alpha.32 的干净快照开始，包含 `CHANGELOG.md` 中的 Unreleased 改动；[源码分发说明](docs/open-source.md)解释历史记录与安装包的对应关系。

> 当前进度：**M3 工作区与导航**之后，并入了 [Mayoi](https://github.com/sorrycc/Mayoi) 的交互与特色功能：
> MyGo 窗口版、Chroma 语法高亮、查找替换与跨文件搜索、git 标记 / blame / 源代码管理 / 改动视图、
> 分栏、预览标签、粘性作用域与面包屑、Vim 模式、主题、自动保存与会话恢复。对照表见 [docs/MAYOI.md](docs/MAYOI.md)。
> 设计与路线见 [docs/DESIGN.md](docs/DESIGN.md)、[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)、[docs/DECISIONS.md](docs/DECISIONS.md)。

从 alpha.11 起可在「帮助 → 检查更新…」安装签名更新，同一菜单可关闭自动检查。旧版本需手动安装一次。

## 安装 macOS 预览版（v0.1.0-alpha.35）

> **预览版**：只支持 Apple Silicon（M 系列芯片）的 Mac，macOS 13 或更新；**ad-hoc 签名、没有经过 Apple 公证**。
> 能日常试用，但还有没在原生窗口里验证过的地方，见 [docs/macos-preview-release-report.md](docs/macos-preview-release-report.md)。
> 打开重要项目前请先提交或备份。

1. 在 [Releases](https://github.com/corn12138/yan-releases/releases) 里下载 `yan-v0.1.0-alpha.35-macos-arm64.dmg`（备用：`…app.zip`）和 `SHA256SUMS`。
   安装包仓库公开，无需登录；也可以 `gh release download v0.1.0-alpha.35 -R corn12138/yan-releases`。
2. 核对文件：在下载目录里运行 `shasum -a 256 -c SHA256SUMS --ignore-missing`，每个文件都应显示 `OK`。
3. 打开 dmg，把 **yan** 拖到旁边的「应用程序」（Applications）。
4. 第一次打开：在「应用程序」里双击 yan。macOS 会说无法验证开发者，点「完成」（不要点「移到废纸篓」）。
   然后打开「系统设置 › 隐私与安全性」，往下翻到「安全性」，在「已阻止 "yan" 以保护你的 Mac」旁点「仍要打开」，
   用密码或触控 ID 确认，再在弹出的对话框里点「打开」。以后直接双击就行。
   不需要关闭 Gatekeeper，也不需要在终端里删除隔离属性。
5. 用「文件 › 打开文件夹…」（Cmd+O）打开一个项目。状态栏右下角和窗口标题里的标签（如「预览版 v0.1.0-alpha.35」）说明是哪一版；
   完整版本号和提交号在「帮助 › 关于砚」；第三方声明在「帮助 › 第三方声明」。

自动保存默认开：停止编辑 1 秒、切换标签、关标签、切到别的应用时保存。别的程序在这期间改过同一个文件时，砚**不会覆盖**它，
会在状态栏提示；用命令面板里的「文件：覆盖磁盘上的版本」或「文件：还原为磁盘上的版本」（可撤销）自己选。

## 构建与运行

```bash
git clone https://github.com/corn12138/yan-source.git
cd yan-source
make build           # 终端版 bin/yan（纯 Go，无 cgo，stripped 约 8.4 MB）
./bin/yan             # 打开当前目录（有 README 会自动打开，并恢复上次的标签）
./bin/yan ~/code/app  # 打开文件夹
./bin/yan main.go:42  # 打开文件并跳到第 42 行
make run-app          # 窗口版 bin/yan-app（MyGo，同样无 cgo，约 18 MB）
make dev              # 同上，但配置和会话放在 build/dev-config，不碰 ~/.config/yan
make bundle           # 打包成 build/ 下的 yan.app 和 .dmg（go tool mygo build），带上 libghostty-vt 和第三方声明
make check            # gofmt + vet + 测试 + 竞态检测 + 架构边界 + 体积门禁（终端版 ≤10 MB）
make race             # 只跑竞态检测（要 C 工具链；编不了 -race 时报「没跑」并失败）
make release VERSION=v0.1.0-alpha.35   # 从打了这个标签、没有未提交修改的提交做 dist/ 下的发布包和 SHA256SUMS
```

发布流程（CHANGELOG、scripts/bump.sh、CI）见 [CONTRIBUTING.md](CONTRIBUTING.md)。

> Makefile 固定 `CGO_ENABLED=0`，所以不需要接受 Xcode 许可。系统自带的 `make` 在没接受许可时仍会报错，
> 可以先 `export DEVELOPER_DIR=/Library/Developer/CommandLineTools`，或者直接用 `CGO_ENABLED=0 go build ./cmd/yan-app`。
> `make run-app` 跑的开发版第一次打开内嵌终端时，MyGo 会把 libghostty-vt 下载到缓存；打包的 yan.app 自带它，不下载。

## 已实现

| 功能 | Terminal.app | Ghostty（kitty 键盘协议） |
|---|---|---|
| 快速打开（`:行号` 跳行，`>` 切命令面板） | Ctrl+P | Cmd+P |
| 命令面板 | Ctrl+K P 或 F1 | Cmd+Shift+P |
| 跳到行 | Ctrl+G | Ctrl+G |
| 显示/隐藏资源管理器 | Ctrl+B | Cmd+B |
| 聚焦资源管理器（↑↓ ← → Enter Space Esc） | Ctrl+K E | Cmd+Shift+E |
| 关闭标签（有未保存修改时按两次） | Ctrl+W | Cmd+W |
| 下一个 / 上一个标签 | Ctrl+K ] / Ctrl+K [ | Cmd+Shift+] / [ |
| 保存 / 退出 | Ctrl+S / Ctrl+Q（有未保存修改时按两次） | Cmd+S / Ctrl+Q |
| 撤销 / 重做 | Ctrl+Z / Ctrl+Y | Cmd+Z / Cmd+Shift+Z |
| 复制 / 剪切 / 粘贴（无选区时整行） | Ctrl+C / Ctrl+X / Ctrl+V 或 Cmd+V | Cmd+C / Cmd+X / Cmd+V |
| 全选 | Ctrl+A | Cmd+A |
| 选中下一个匹配（多光标） | Ctrl+D | Cmd+D |
| 按词移动 / 删除 | Option+← / → ，Option+Backspace | 同左 |
| 行首 / 行尾、文件首 / 尾 | Home / End，Ctrl+Home / Ctrl+End | Cmd+← / → ，Cmd+↑ / ↓ |
| 扩展选区 | Shift + 移动键；鼠标拖选；双击选词，三击选行 | 同左 |
| **终端：切到 shell 运行项目** | Ctrl+K T（或 Ctrl+`） | 同左 |

- 中文输入：输入法一次提交的整句合并为一次插入、一个撤销步；终端光标始终在插入点，候选框跟着走。
- 文件：UTF-8 / UTF-8 BOM、LF / CRLF 原样保留；原子保存（临时文件 + fsync + rename），保留权限。
  非 UTF-8 或超过 8 MB 的文件只读打开。
- 终端：在项目目录下打开你的 `$SHELL`，`exit` 或 Ctrl+D 回到编辑器，文件与光标原样保留；回来时自动刷新文件树和索引。
- 资源管理器：目录在前、按需展开；被 gitignore 忽略的条目变暗，`.git`、`node_modules` 隐藏；支持嵌套 `.gitignore` 和 `!` 规则。
- 快速打开：后台并行建索引（上限 20 万个文件），模糊匹配也在后台算、打字即取消旧的；空查询列出最近打开；
  中文输入法打出的 `：` `》` 前缀也认。
- 外部修改：切回窗口时检查磁盘，没改过的文件静默重载（可撤销）；有未保存修改时提醒，保存（包括自动保存）不会覆盖磁盘上被别的程序改过的文件，
  由你选「覆盖磁盘上的版本」或「还原为磁盘上的版本」。
- 配置：`~/.config/yan/config.toml` 和项目里的 `.yan.toml`（命令面板「打开用户配置文件」会生成带注释的模板），保存后立即生效。
  主题、字号、自动保存、Vim 这类在应用里切换的设置，以及会话和最近的文件夹，记在 `~/.config/yan/state.json`。
  Claude 的 Clawd 和 Matrix 的数字雨可在「显示 → 主题 → 装饰效果」关闭，或配置 `[ui] theme_effects = false`；
  菜单里选过的值优先于配置文件。失焦或系统要求减少动态效果时，装饰保持静止。
- 界面语言：「设置 → 通用 → 界面语言」（⌘, 打开设置，Windows、Linux 上是 Ctrl+,）选「跟随系统」（默认）、简体中文或 English。
  跟随系统时取系统语言偏好里第一个砚支持的语言，都不支持时用英文。选择立即记在 `state.json`，下次启动砚时生效；
  在那之前所有窗口保持启动时的语言，设置里会提示重启。也可在用户 `config.toml` 写 `[ui] language = "system"`、`"zh-Hans"` 或 `"en"`，
  设置里选过的优先。界面语言属于整个应用：项目的 `.yan.toml` 写了会被忽略并给出提示。终端版用同一个选择，跟随系统时看
  `LANGUAGE`、`LC_ALL`、`LC_MESSAGES`、`LANG`。
- Mermaid：窗口版的 Markdown 预览把 `mermaid` 代码块画成随主题着色的图，`.mmd`、`.mermaid` 文件也能预览；
  配置 `[ui] preview_mermaid = false` 改回显示代码。
- Markdown 预览支持 `$…$` 行内公式、`$$…$$` 块公式和 `math` 代码块；本地渲染，随主题着色，悬停块公式可看源码或复制 TeX。无法绘制时保留源码并说明原因；`[ui] preview_math = false` 可关闭。

### 来自 Mayoi 的功能（两个版本都有，窗口版用 Cmd，终端版见命令面板里的键位）

| 功能 | 窗口版 |
|---|---|
| 查找 / 替换（正则、大小写、全词） | Cmd+F / Cmd+Alt+F |
| 跨文件搜索 / 替换 | Cmd+Shift+F / Cmd+Shift+H |
| 源代码管理（暂存、提交、推送、切分支） | Ctrl+Shift+G；提交 Cmd+Enter |
| 重新打开关掉的标签 | Cmd+Shift+T |
| 向右分栏 / 在栏间切换 | Cmd+\ / Cmd+Alt+← → |
| 跳到符号 / 打开最近的文件夹 | Cmd+Shift+O / Ctrl+R |
| 上下移动行、复制行、删行 | Alt+↑↓、Shift+Alt+↑↓、Cmd+Shift+K |
| 行注释、缩进 | Cmd+/、Cmd+] / Cmd+[ |
| 字号 | Cmd+= / Cmd+- / Cmd+0 |
| SVG、Markdown、Mermaid 预览 | Cmd+Shift+V |
| 内嵌终端 / 新终端 | Ctrl+` / Ctrl+Shift+` |
| Vim 模式、主题、自动保存 | 命令面板或「显示」「文件」菜单 |

- 编辑器里：自动闭合括号引号、缩进参考线、配对括号与同词高亮、git 改动标记（改动间跳转 Alt+F5）、当前行 blame、粘性作用域行与面包屑。
- 资源管理器：单击预览、双击固定；右键新建、改名、删到废纸篓、复制路径、在 Finder 中显示；文件显示 git 状态。
- 改动视图：在源代码管理里点文件，看它相对 HEAD 的改动；回车或双击跳到文件对应行。

## 代码结构

```text
cmd/yan          终端版入口
cmd/yan-app      窗口版入口
core/text        不可变分块 rope、快照、修改集、原子保存（M1a：平铺叶子 + 前缀和）
core/text/width  唯一宽度预言机（与 tcell 同一套规则）
core/edit        Buffer、多选区、快照式撤销、按字素/按词移动
core/keymap      组合键规范化、连按 trie（Ctrl+K X）、mod 别名、按焦点区分上下文
core/workspace   gitignore（按目录分桶）、fastwalk 并行索引、懒加载目录树
core/search/fuzzy  可取消的模糊匹配（快速打开）
core/config      TOML 配置：默认值 → 用户配置 → 项目 .yan.toml
core/view        内核与前端的接缝：语义化的 Frame（侧边栏、分栏、窗格、状态栏、浮层）；core/view/cell 单元格布局
core/style       样式 ID 与主题配色
core/syntax      Chroma 增量高亮、文件符号（移植自 Mayoi）
core/git         git CLI 封装与行 diff（移植自 Mayoi）
core/search/glob 搜索的包含/排除规则（移植自 Mayoi）
core/app         所有者 goroutine：输入分发、命令（与 VS Code 同名）、编辑、查找、搜索、git、Vim、帧构建
ui/tui           tcell v3 终端前端：事件翻译、单元格布局、绘制
ui/gui           MyGo 窗口前端：工作台、代码窗格、浮层、侧边栏、菜单、终端面板
platform/        pbcopy / pbpaste、废纸篓等系统能力
internal/launch  命令行参数解析（文件夹、文件、:行号）
internal/archtest  架构边界测试：core 不得依赖 UI 包，两个前端互不依赖，全部不含 cgo
internal/version 版本号（来自 git 标签，Go 记进二进制的 VCS 信息）
internal/tools   发布用的小工具：notices（第三方声明）
packaging/icon.svg  应用图标的矢量源文件；resources/icon.png 是它的 1024 px 导出
packaging/licenses  随 .app 附带的原生库（libghostty-vt 及其静态库）的许可原文
resources/       打进 yan.app/Contents/Resources 的文件（图标；打包时生成的第三方声明）
scripts/         release-macos.sh：发布包
```

## 许可

砚自己的代码采用 [MIT 许可](LICENSE)。移植自 Mayoi（MIT）和 Lucide（ISC）的部分及其许可声明见
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)；发布的二进制里编进的全部第三方代码的许可原文由 `make notices` 生成为
`THIRD_PARTY_LICENSES.txt`，随 yan.app 和发布页一起分发。

提交代码或报告问题前，请阅读 [安全与隐私说明](SECURITY.md)。
