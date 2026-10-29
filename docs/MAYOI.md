# 并入 Mayoi：对照表

砚在 2026-10 把 [Mayoi](https://github.com/sorrycc/Mayoi)（v0.5.1，MIT）的交互和特色功能并了进来（[ADR-014](./DECISIONS.md#adr-014-gui-前端改用-mygo取代-gio)、[ADR-015](./DECISIONS.md#adr-015-并入-mayoi-的交互与特色功能)）。
这一页记录每样东西从哪来、落在哪、两个前端各做到了什么，以及还没做的。许可声明见仓库根的 [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md)。

## 全量功能台账

初始基线为 2026-10-19 上游 `46990a1`；后续更新逐批补充，完整功能登记与缺口见 [逐项功能台账](mayoi-feature-ledger.md)，
命令注册项见 [命令索引](mayoi-command-index.md)。本页保留已移植来源；开放项以台账为准。

## 2026-10-27 Markdown 预览：脚注、标点后的网址、样式填充上的 Mermaid 标签（UP-59～61）

- 脚注照 [`88e00cf`](https://github.com/sorrycc/Mayoi/commit/88e00cfaeb4c52b33a74e4eefc70dec92ab8720b)：引用是上标数字，链接到正文末尾分隔线下的编号列表；
  脚注小一号、浅色，每条带回到各处引用的 ↩。锚点名同 GitHub（`user-content-fn-N`、`user-content-fnref-N`），复用砚已有的页内链接滚动。
- 裸网址照 [`3d03b96`](https://github.com/sorrycc/Mayoi/commit/3d03b96c4f8778469f95c3e4cf34f0ebf93ead20)：goldmark 的 linkify 只认空格、分隔符后的网址，
  `链接：https://…` 这类全角标点、汉字、数字后的不成链接；另加一个 AST 变换，像 GitHub 一样只要前面不是字母就成链接，代码、已有链接里的不动。
- Mermaid 照 [`b906ff6`](https://github.com/sorrycc/Mayoi/commit/b906ff61990603dfa0f8169791aa6cb2032ce6fc)：`style`/`classDef` 给节点、子图填了色而主题文字在上面对比度不足 3 时，
  标签换成主题文字色或背景色中对比更高的那个；文字自己带 `color` 的不改。砚另补浅色主题的断言。

## 2026-10-26 Mermaid 预览（UP-55）

移植上游 [`f1498a9`](https://github.com/sorrycc/Mayoi/commit/f1498a9d04caa7616d702bec6b425456da801de5) 与 Linux 修正
[`2ba2637`](https://github.com/sorrycc/Mayoi/commit/2ba263790582069d766860103ffac45c7142e399) 的 `cmd/mayoi/mermaid.go` 到 `ui/gui/mermaid.go`：
Pele 1.9.0（MIT，`scripts/pele.sh` 打包，与上游的 `pele.js.gz` 逐字节相同）在 goja 里把图排成 SVG，砚换上主题颜色，标签用窗口自己的文字画。
Markdown 预览的 `mermaid` 代码块画成图，悬停有「显示源码」「复制代码」，画不出时在代码下说明原因；比正文宽的图可以用满窗格宽度。
`.mmd`、`.mermaid` 文件复用 Markdown 预览，整份文本当一个图。开关是 `[ui] preview_mermaid`（上游 `previewMermaid`），只在配置文件里。
只在窗口版：终端版不链接 goja（`go list -deps ./cmd/yan` 无 goja），但 goja 把 Chroma 共用的 regexp2 升到 v2.8.1，终端版大了约 200 KB，仍在门禁内。
无头截图覆盖浅色/深色下用户的中文流程图；原生 Windows/Linux 只经交叉编译，未做桌面验收。

## 2026-10-26 转到定义（砚自有）

上游主线 `6d98a49` 与开放 PR 都没有转到定义；砚自己做：F12、平台链接修饰键点击（同终端文件链接，走 `keymap.Profile.Mods`），
按住修饰键时悬停的名字画下划线，后退/前进按 VS Code 的平台键位。没有语言服务器，定义按 `core/syntax/symbols` 读行认名字，
先当前文件后同语言的文件夹文件。顺带移植开放 [PR #83](https://github.com/sorrycc/Mayoi/pull/83)（`b36c7d0`）：行首 `func() {` 不再让符号提取越界。
原生 Windows/Linux 的 Ctrl 点击与 Alt+←→ 只经单测与交叉编译，未做桌面验收。

## 2026-10-23 Git 与 hooks 的登录 shell 路径

参考 [`2650b9b`](https://github.com/sorrycc/Mayoi/commit/2650b9bfa90c87ffe0d48fffb140dd2fc428ff1c)，
GUI 在创建编辑器前启动一次登录交互 shell 探测。所有 Git 入口等待同一份有界结果，
可执行文件查找与 hooks 使用一致的 PATH；shell 路径在前，继承的绝对目录补在后面并去重。
应用自身和内嵌终端的环境不被改写，终端版沿用调用者环境；Windows 保留原有查找和隐藏控制台策略。
探测在用户 HOME 中执行，限制时间与输出，清理自己的进程组并限制子进程占住输出管道的等待；
退出或重启会取消尚未完成的探测并等待清理，启动失败同样收尾。
格式错误、退出失败或超时均回退继承环境；不把相对目录解释为仓库里的工具，重启后重新探测。
针对性测试覆盖假 shell、并发首次调用、自定义 Git 查找和真实提交 hook；独立 macOS 应用通过系统启动，
在精简 PATH 下提交测试仓库并验证 hook 找到恢复目录中的工具；另一独立应用在探测挂起时关窗退出，假 shell 及其子进程在超时前被清理。Windows/Linux 原生桌面仍未验收。

## 2026-10-23 终端文件交接

参考主线 `e18f9ba` 与仍开放的 [PR #77](https://github.com/sorrycc/Mayoi/pull/77)（`fce5318`）。
终端文件链接优先用 shell 的 OSC 7 当前目录，未报告绝对目录时回退启动目录；支持家目录和一基行列，
既有冒号文件名优先于位置后缀。只打开现存普通文件，沿用共享文档与锁列路由，网页仍由系统处理。
文件剪贴板优先于其文字表示，经平台 shell 引用后粘贴；普通文字与图片的空 bracketed paste 保持原样，不发送回车。
覆盖路径解析、Unicode/CRLF 定位、忽略文件、未保存缓冲区和锁列；真实 libghostty 控件以模拟连接验证
修饰键点击、OSC 7 目录切换、URL 回退及粘贴字节。系统文件剪贴板、真实 shell 集成和 Windows/Linux 原生桌面仍需单独验收。
Windows 路径使用 `filepath`，引用默认 PowerShell；Windows 反斜杠路径的终端文本识别受当前 MyGo 控件限制，
此批没有声称已完成 Windows 终端发布，也不把上游开放 PR 记为已合并。

## 2026-10-23 主题选择与后续跟踪

上游主线复核到 [`db73304`](https://github.com/sorrycc/Mayoi/commit/db73304b2552424b5afd23c3ca3dc065ca57a111)。
主题选择参考 `d81bfc4`，通过共享内核和 Frame 给 GUI/TUI 提供即时预览、搜索、确认和回滚。
GUI 从显示菜单或欢迎页进入；输入查询后左右键留给文本编辑，Matrix 保持深色并禁用外观选择。
预览不改配置或 state，不影响另一个窗口，也不会被保存会话/字号顺带提交；取消后显示最新配置。
颜色过渡可中断，并遵循减少动态效果和失焦状态。验收与开放边界见 [主题说明](theme-effects.md#color-theme-picker)。
命令行、终端文件链接、子模块、Markdown 默认预览与系统拖放等后续工作继续逐项记在台账。

## 2026-10-23 提交文件导航

参考 `8ae1d55`、`3759c8b`：提交标签可用工具栏或 `git.toggleCommitFiles` 显示右侧文件树，默认隐藏。
与对应仓库的 SCM 共用树/列表偏好；目录折叠按提交标签分别记录。面板宽度与显示开关保存到 state.json，窄列自动隐藏。
选文件定位历史差异，滚动反向选中列表里的文件；Enter 固定预览。SCM 提交行可展开文件，左右键展开/折叠、上下键选择、Enter 固定。
GUI/TUI 共享 SCM 行；所有操作绑定仓库 ID、完整提交号或虚拟标签身份，旧回调不会作用于替换后的标签。
已覆盖根提交、重命名、删除、二进制、多仓库以及并排/单栏导航；原生 Windows/Linux 运行验收仍开放。

## 2026-10-23 查找与拖动稳定性

跟踪到上游 `e1519fb`（新增命令行 socket 控制，登记 UP-48，尚未内化）。
参考 `e57d684`，查找/替换命令经共享 Frame 发出一次性选中请求；重开与已聚焦时都可直接覆盖输入，普通点击不触发全选。
中文组字期间不把原生输入框的临时内容发布成查询。沿用现有 MyGo 版本和跨平台键位接口。
对照 `9ed2adb`，砚的 Frame 快照已保证一帧只在一列绑定查找框，拖动负载已按 Frontend 隔离。
新增跨列点击与跨窗口文件/终端负载的落点回归，并在终端解析入口复核所有者；跨窗口会话留在原窗口，不共享原生控件。
验证范围是 GUI 测试器的事件与所有权检查；系统原生窗口间拖放、系统输入法及 Windows/Linux 原生运行仍需人工验收。

## 2026-10-22 跟踪与终端默认位置

最新复核到 [Mayoi v0.11.11 / abf7db6](https://github.com/sorrycc/Mayoi/commit/abf7db664a4a6cd1ab1bf4376bc83a9c97829acb)。
新增变化登记在台账 UP-29～47，尚待核验的项目保留开放，不视为已经移植。
本批内化 `d81abe4` 的默认终端位置：显示菜单「默认在编辑区新建终端」、命令 `view.terminalLocation`，
或 `[terminal] location = "editor"`（`"panel"`/空值为面板）。菜单选择保存在 state.json，优先于 TOML。
资源管理器的文件、文件夹和空白菜单都可「在终端中打开」；文件使用父目录，文件夹使用自身目录。
新终端使用未锁定的文件列；显式「在编辑区新建终端」与面板的「＋」各保留原目的地。
已打开的会话不迁移、不重启；TUI 仍交给外部 shell。Windows/Linux 保留同一命令与配置接口，原生验收另列。

## 最新内化：Markdown 阅读与预览

右侧预览保留源码焦点，支持 GitHub 五种提示块、标题锚点、代码复制、内容列宽表格、可选中文字和阅读统计。
预览会话和阅读位置保留；源码与预览按源码块双向滚动，滚动不改变源码光标和选区。
命令 `markdown.showPreviewToSide` 也支持 SVG；平台按键仍由 keymap.Profile 分配。

## 已发布：多仓库同屏管理

已内化 Mayoi 的仓库上下文、仓库选择器和已初始化子模块发现：GUI 同屏显示多个仓库的改动与独立提交框，
TUI 共享同一套语义数据和操作。提交、菜单确认、撤销、历史差异和会话恢复绑定原仓库；工作树按各自根目录区分。
平台快捷键继续通过 `keymap.Profile`，没有引入仅 macOS 可用的核心依赖。详见 [多仓库说明](multi-repository-scm.md)。

## 怎么并的

- **内核还是砚的**：不可变 rope、多选区、快照式撤销、单一所有者 goroutine 都没动。Mayoi 的编辑行为是在 `core/app` 里照着重写的，坐标一律是 UTF-8 字节偏移。
- **两个前端共享行为**：功能都在内核里，`view.Frame` 只描述「要画什么」。终端版（tcell）和窗口版（MyGo）只负责画和把鼠标、键盘翻译成语义动作。
- **能整包搬的就整包搬**，并保留 Mayoi 的测试。

## 整包移植

| Mayoi | 砚 | 说明 |
|---|---|---|
| `internal/highlight` | `core/syntax/highlight` | Chroma 增量高亮、括号着色（[ADR-016](./DECISIONS.md#adr-016-语法高亮改用-chroma终端版体积门禁放宽到-10-mb)） |
| `internal/symbols` | `core/syntax/symbols` | 文件内符号（快速打开里 `@`、面包屑） |
| `internal/git` | `core/git` | git CLI 封装：状态、暂存、提交、blame、分支；补了 git 2.45 下子模块的 `show` |
| `internal/linediff` | `core/git/linediff` | Myers 行 diff 与词级 diff |
| `internal/glob` | `core/search/glob` | 搜索的包含/排除规则 |
| `internal/git/worktree.go`（v0.7.0） | `core/git/worktree.go` | 找出占着分支的 worktree、清理文件夹没了的 worktree |
| `cmd/mayoi/graph.go` | `core/app/graph.go` | 提交图的排布（每个分支一道、一种颜色） |
| `internal/keymap/when.go`、`StripJSONC`（v0.7.0） | `core/keymap/when.go`、`jsonc.go` | keybindings.json 的 when 条件、带注释的 JSON |

## 照着重写

| 功能 | 内核 | 终端版 | 窗口版 |
|---|---|---|---|
| 编辑操作：缩进/反缩进、行注释、上下移动/复制行、删行、上下插行、选行、跳到配对括号、智能回车 | `core/app/editing.go` | ✓ | ✓ |
| 自动闭合括号与引号（打右括号跳过、退格成对删除） | `editing.go` | ✓ | ✓ |
| 缩进参考线（当前块高亮）、配对括号、同词高亮 | `decorations.go` | ✓ | ✓ |
| 查找替换：正则、大小写、全词、`$1` 替换 | `find.go` | ✓ | ✓（原生输入框） |
| 跨文件搜索与替换：包含/排除、按文件折叠、未保存内容也搜 | `search.go` | ✓ | ✓ |
| git 行标记、在改动间跳转、当前行 blame | `git.go` | ✓ | ✓ |
| 源代码管理视图：暂存/取消/丢弃（进废纸篓）、提交、修订上次提交、撤销提交、推送/拉取、切分支 | `scm.go` | ✓ | ✓ |
| 改动视图（相对 HEAD，折叠没改的行，词级颜色） | `diff.go` | ✓ | ✓ |
| 源代码管理以树的形式查看（只有一个子文件夹的连起来写），文件夹折起、暂存、取消暂存、放弃；提交之后状态栏的「撤销」；显示更多提交；不在仓库里时「初始化仓库」 | `scm.go`、`history.go` | ✓ | ✓ |
| 并排的改动视图：左边旧的、右边新的，改过的行对着；每边放得下 80 格时自动并排，面包屑末尾的按钮换自动、并排、一栏 | `diff.go` 的 `writeSplit` | ✓ | ✓ |
| 所有改动：一个标签里每个改了的文件一段（已暂存、更改、未跟踪），文件标题折起；改动视图里的翻页器键（j k ] [ . , 空格 f b d u g G z e s），s 暂存（取消暂存）一处改动或整个文件 | `changes.go` | ✓ | ✓ |
| 行号旁的改动：点标记（⌥F3、⇧⌥F3）在改动下面开框看 HEAD 里的样子，还原（撤销的一步）、只暂存这几行（⌃K R、⌃K S） | `hunks.go` | ✓（键） | ✓（按钮） |
| 粘性作用域行、面包屑、跳到符号 | `sticky.go` | ✓ | ✓ |
| 预览标签、重新打开关掉的标签、分栏（`mod+\`）、在栏间移动标签 | `tabs.go` | ✓ | ✓ |
| 资源管理器：新建、改名、删到废纸篓、复制路径、在 Finder 中显示 | `files.go` | 键盘 | ✓（右键菜单） |
| 资源管理器 ⌘C、⌘V 复制粘贴文件（和访达之间也行）；点空白处或 Esc 不选；空白处的右键菜单（v0.6.0） | `filecopy.go` | 键盘 | ✓ |
| 从资源管理器、访达把文件拖到一列或标签条指定位置打开；拖到列边缘分栏；从资源管理器拖进访达复制（v0.6.0） | `filecopy.go` 的 `DropFiles` | — | ✓ |
| Vim 模式（动作、文本对象、操作符、可视模式、`.` 重复、寄存器） | `vim.go` | ✓ | ✓ |
| Claude 的 Clawd、Matrix 欢迎页数字雨；失焦或减少动态效果时静止，可在「显示 → 主题 → 装饰效果」关闭 | `config`、`settings.go` | — | `ui/gui/clawd.go`、`mascot.go`、`rain.go` |
| 主题：Mayoi 暗/亮、Matrix、Claude 暗/亮；字号缩放 | `core/style/theme.go`、`settings.go` | 配色 | ✓ |
| 行级评审：原始引用、位置迁移、过期标记、会话恢复、差异卡片、复制与终端交接 | `core/review`、`review.go` | 命令与卡片；发送走剪贴板 | 原生意见框、行侧按钮、概览标记、面板与编辑区终端 |
| 自动保存、会话恢复、最近的文件夹 | `settings.go`、`core/config/state.go` | ✓ | ✓ |
| 图片查看与 SVG 预览 | `files.go` | 只显示路径 | ✓ |
| Markdown 预览（⇧⌘V，v0.7.0）：标题、列表与任务列表、引用、高亮的代码、表格、本地图片、链接；边改边更新，同一个文件在旁边一列看文字；点本地链接在砚里打开 | `files.go`、`frame.go`（只给文字） | 说一声 | `ui/gui/markdown.go`（解析，移植）、`markdownview.go` |
| 自动换行、Alt/Option+Z 单标签切换，随文件夹会话恢复 | `wrap.go`、`settings.go` | ✓ | ✓ |
| 全文概览条：Git 改动、查找匹配、光标和视口；点按与拖动跳转 | `overview.go` | — | `ui/gui/code.go` |
| 标签排序、跨列拖动、边缘分栏；列宽和侧边栏拖动调整 | `tabs.go`、`filecopy.go` | 列命令 | `ui/gui/view.go`、`panedrop.go` |
| 终端标签页：面板与编辑区拖动/菜单互移，保留 shell；拆分、重新打开与工作区恢复启动新 shell；拖文件输入带引号的路径 | `terminals.go`、`settings.go` | — | `ui/gui/termtabs.go`、`terminal.go` |
| 原生多窗口，各自独立的编辑器、文件夹和终端 | `workbench.action.newWindow` | — | `ui/gui/gui.go` |
| 内嵌终端面板 | — | 切到 shell（ADR-013） | MyGo 终端插件；打包的应用自带 libghostty-vt，在原生窗口里验证过 |
| 标题栏里的命令中心、活动栏图标、Lucide 图标、文件类型图标 | — | — | `ui/gui` |
| 排序行、转换为驼峰、缩进方式（制表符、2/4/8 个空格）（v0.7.0） | `editing.go` | ✓ | ✓（「选择」菜单） |
| 切到另一个 worktree 占着的分支：在这个窗口打开那个 worktree；文件夹没了时清理再切换（v0.7.0） | `scm.go`、`overlay.go`（选一项的浮层） | ✓ | ✓ |
| 提交历史与提交图：分支、标签、HEAD、合并、没推送的；显示所有分支；打开提交看它改的文件、复制提交号、检出、建分支、打标签 | `history.go`、`graph.go` | 近似的线 | ✓ |
| 触控板横划只沿主方向滚（v0.7.0） | — | — | `ui/gui/code.go` |
| 在提交上合并、变基、重置（软、混合、硬）、还原、拣选；遇到冲突时「继续」「中止」，提交说明里放好 git 拟的（v0.8.0） | `gitops.go`、`core/git` | 命令面板 | ✓ |
| 项目标签：一个窗口里几个文件夹，左边一列方块（名字的头一两个字、没保存的点），⌃⌘] ⌃⌘[ ⌃⌘1…9 换项目，各自的标签、终端、侧边栏和面板；打开时并起窗口、关掉时分开，下次打开都回来（v0.8.0–v0.8.1） | `projects.go`（命令、设置） | — | `ui/gui/projects.go` |
| 终端选中就复制（拖、双击、三击、全选），`[terminal] copy_on_select = false` 关掉，开着的终端也跟着改；剪贴板里只有图片时 ⌘V 把截图交给 Claude Code（v0.8.2；同 Mayoi 用 sorrycc/mygo：MyGo v0.4.8 加上 #290、#344、#345） | `config` | — | `ui/gui/terminal.go` |
| 项目标签：拖方块排序；看不见的项目的终端响铃、发通知时方块描上强调色，提示里说为什么（v0.8.1） | — | — | `ui/gui/projects.go` |
| 窄窗口：状态栏放不下时先去掉编码（位置最后），命令中心先不写文件夹再截短文件名（v0.8.0） | `view.StatusItem.Keep` | — | `ui/gui/view.go` |
| 崩溃时把栈写进 `~/.config/yan/crash.log`，留最近 10 次（v0.8.0） | — | — | `internal/crashlog` |
| 空文件夹的资源管理器：说一声，给新建文件、新建文件夹的按钮（v0.8.1） | — | 说一声 | ✓ |
| keybindings.json（VS Code 的格式，v0.7.0）：连按、`-命令` 拿掉键、when 按砚的四个上下文求值、`runCommands`；保存后立即生效 | `keybindings.go`、`core/keymap/vscode.go` | ✓ | ✓（「文件」菜单） |
| Windows、Linux 的键位（VS Code 在那里的）与「文件资源管理器」「回收站」的叫法（v0.7.0，先留好口子） | `platform.go`、`keymap.Profile.OS` | — | 跟着 `runtime.GOOS` |

## 预览版（v0.1.0-alpha.1）里验证到什么程度

- **原生窗口里验证过**（打包好的 yan.app，从「应用程序」位置经 LaunchServices 启动）：打开文件夹、会话恢复、窗口失去焦点时自动保存、
  回到窗口时重载外部修改、有冲突时不覆盖、内嵌终端（跑命令、改大小、Ctrl+C、关闭、开着终端退出）、终端库加载不了时编辑照常。
  这些是用 `-tags selftest` 的验证构建由内核 API 代替键盘完成的，切换焦点用的是真的应用切换。
- **只在无头测试里验证过**：键盘打字、中文输入法的组字/提交/取消/替换选区/一步撤销/组字途中换标签（模拟 MyGo 送来的组字事件）、
  查找替换、跨文件搜索、源代码管理、改动视图、图片与 SVG、菜单命令。
- **没有验证**：系统自带拼音输入法在真窗口里的实际表现（候选框位置等）、关窗口时「要先保存吗」对话框、
  Gatekeeper 的「仍要打开」这一步（要在系统设置里点）。详见 [macos-preview-release-report.md](./macos-preview-release-report.md)。

## 布局与终端标签批次的验证（2026-10-15）

- 内核测试覆盖独立终端 ID、移动保留身份、拆分创建新会话、关闭恢复、工作区恢复及终端版跳过 GUI 终端。
- 无头 GUI 测试覆盖标签拖动、列边缘落点、关闭释放，以及失败终端可关闭。
- `-tags selftest` 的终端集成测试使用真实终端模拟器和模拟连接，覆盖拖动往返、输出保留、文件/终端键盘切换及多列焦点。
- 临时应用副本的真实 macOS 窗口验证了 shell 执行、标签/面板互移、调整大小、Ctrl+C、关闭和退出；使用独立配置。
- 尚未人工验证 Finder 的真实拖入事件和系统输入法候选窗。本批次的路径引号通过平台单元测试验证，不能替代系统拖放验收。

## 上游跟踪（2026-10-18）

已核对上游 `ce553d24b746`。最新的 [Windows 子进程不弹控制台修复](https://github.com/sorrycc/Mayoi/commit/ce553d24b7463e795c9d261932f8145dd4955f74)
已移植到 `internal/process`，覆盖状态、gutter 和 diff 的 Git 进程；Windows 测试仅交叉编译。新增的行级评审意见及发往终端见
[7be5390a752e](https://github.com/sorrycc/Mayoi/commit/7be5390a752e)、
[b4fff084971c](https://github.com/sorrycc/Mayoi/commit/b4fff084971c) 与
[461d0138915f](https://github.com/sorrycc/Mayoi/commit/461d0138915f)。已接入共享内核与两个前端，验收见 [review-comments.md](./review-comments.md)。
本轮参考快照为 `8708da764770273c59a056d4ce20c416833e7c46`；新增 SSH、设置同步和 AI 网关仍是候选，不包含在本批次。

## 项目总目录中的仓库（2026-10-18）

已跟进上游 [d28bc2500f47 / #49](https://github.com/sorrycc/Mayoi/commit/d28bc2500f4715c35ab12df289656c170600dc4a)。
移植两层子仓库发现，使用本地文件系统和可取消的后台扫描；GUI/TUI 显示入口并打开独立项目。
已打开的项目复用，未保存内容与提交说明留在所属项目。具体行为和验收见 [nested-repositories.md](./nested-repositories.md)。
多仓库同屏聚合和递归已初始化子模块已在 alpha.15 完成，见 [多仓库说明](multi-repository-scm.md)。

## 提交工具批次

继续核对上游 `8708da764770273c59a056d4ce20c416833e7c46`：移植 `internal/git/commitactions.go` 和原测试，
把 `cmd/mayoi/commitactions.go` 的交互改写到共享内核。提交补丁、双向比较、自选起点、远端页面、
比较标签恢复与“显示所有分支”持久化见 [commit-tools.md](./commit-tools.md)。Windows/Linux 使用现有 Git 进程封装和平台宿主接口。

## 窗口与文件打开批次

新窗口打开、窗口复用、Dock 最近目录、右侧边栏、状态栏设置和命令安装已实现，
验收状态见 [window-workflows.md](./window-workflows.md)。

## 主题装饰批次

Clawd 与 Matrix 数字雨已移植；保留 Qoder/Qoduck 的排除范围。配置、关闭入口和验证边界见
[theme-effects.md](./theme-effects.md)。

## 自动更新与 Windows 进程批次

更新采用 MyGo 原生窗口，源码采用 MIT 许可，安装包通过独立仓库分发。验证与发布边界见
[automatic-updates.md](./automatic-updates.md)。

## 尚待内化

全部开放项及逐项验收条件见 [功能台账](mayoi-feature-ledger.md)。优先级依次为文件/键盘可靠性、锁列与终端布局、
Git 管理面板和差异细节、Vim 兼容、字体/国际化、SSH、账户同步与 AI、原生 Windows/Linux 发布。
Qoder/Qoduck 按用户要求跳过；砚保留 TOML 配置及 GUI/TUI 共享内核。TUI 图像协议属于砚自己的后续扩展。

### File and replacement integrity (2026-10-19)

The ED-02/17 and SE-12 contracts now have targeted content-version regression tests.
Saved snapshots remain the conflict baseline across local edits and reload undo/redo;
metadata-preserving external writes are detected before atomic replacement. Search uses
BOM-free editor offsets, rejects stale single-result versions and re-reads bulk replacements.
This extends Yan's existing save guard without importing upstream UI or platform dependencies.
The remaining file-watching and configuration/keybinding contracts stay open in the ledger.
