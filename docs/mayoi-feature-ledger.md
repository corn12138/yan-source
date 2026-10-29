# Mayoi 功能逐项内化台账

核对日期：2026-10-19。上游固定到 [`46990a1`](https://github.com/sorrycc/Mayoi/commit/46990a1ca9945de7b90b94104c32f2d7770461f5)；砚基线 `v0.1.0-alpha.15 / 8fc8546`。同日复核上游已到 `51b4f4e`，新增提交与未合并 PR 见 [UP 节](#up--上游新增与未合并-pr)。

本表覆盖上游 FEATURES.md 全部章节，并补 SETTINGS.md 的配置、快捷键和账户契约。行是可验收的用户行为；同一功能下的细项写在最后一列。命令注册表另存 [mayoi-command-index.md](mayoi-command-index.md)，用于发现文档之外的入口。源码存在不等于每项已做原生交互验收。

状态：**I** 已有对应实现（源码核对；既有测试见每节，原生验收不能由单测替代）；**P** 部分实现或细节尚需专项核验；**M** 尚缺；**V** 本轮实现并通过本地回归（原生平台验收单列）；**X** 外部服务方案/账户阻塞，先做可替换接口；**D** 已接受的产品差异或明确排除。所有 P/M/X 都是开放项，不计入完成率。

平台：core 功能服务 GUI/TUI；图像、富文本预览、内嵌终端和装饰为 GUI 能力，TUI 保留源码/命令入口。Windows/Linux 的接口和编译验证不能冒充原生桌面验收。

## 交付与验收规则

- 保留 headless core、Frame 协议和前端边界；按行为移植，不整库合并。
- 每批功能完成：针对性回归、`make check`、必要的 GUI 操作截图和 Windows 交叉编译；一批一次 Linux CI，先合并再开下一批；发布阻断修复可与已合并功能合成一次安装包。
- 每行补实现、测试和版本证据后才能关闭；P 项必须先复现差异，不能仅凭文档判断已有功能失效。

## EX · 资源管理器

上游源码：[`sidebar.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/sidebar.go)、[`filecopy.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/filecopy.go)、[`filedrop.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/filedrop.go)。

砚实现边界：[`core/app/workspace.go`](../core/app/workspace.go)、[`core/app/files.go`](../core/app/files.go)、[`core/app/filecopy.go`](../core/app/filecopy.go)、[`ui/gui/sidebar.go`](../ui/gui/sidebar.go)。测试入口：[`core/app/files_test.go`](../core/app/files_test.go)、[`core/app/filecopy_test.go`](../core/app/filecopy_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| EX-01 | 懒加载目录树 | I | 展开和刷新目录 |
| EX-02 | 跟随磁盘增删改 | V | fsnotify 监听已展开目录与打开文件所在目录（上限 512），120ms 合并；树按目录刷新、索引后台整体替换。`core/workspace/watch_test.go`、`core/app/freshness_test.go`（新建/删除/子目录）。原生 Windows/Linux 事件未实机验收；事件溢出时由聚焦检查兜底 |
| EX-03 | 忽略项变淡与 Git 文件状态/目录圆点 | V | 修改 `.gitignore`（砚内保存或外部）后重读该目录规则并刷新树、索引和搜索；聚焦时兜底重读。`TestGitignoreChangesApplyToTheExplorer`、`TestFocusReloadsIgnoreRules` |
| EX-04 | 新建文件/多级目录、重命名、废纸篓 | I | 保留文件路径更新与确认流程 |
| EX-05 | 显示文件位置/复制绝对及相对路径 | I | 系统名称通过平台接口 |
| EX-06 | 工具栏刷新/折叠目录 | I | 空项目和已展开目录均可操作 |
| EX-07 | 空白处及 Escape 取消选择 | P | 逐项确认 Escape 和空白区域一致性 |
| EX-08 | 空白区域新建/粘贴/终端菜单 | P | 确认所有工作区菜单入口齐全 |
| EX-09 | 系统及编辑器文件复制粘贴 | I | 同名副本与目录复制 |
| EX-10 | 文件拖入标签/列边缘及拖出应用 | I | 保留明确放置位置 |
| EX-11 | SCM 文件拖动 | I | 与 Explorer 共用 `transfer.FileData` 拖动载荷（删除的文件不可拖）；数据拖动走系统拖放会话，无头测试无法模拟，待原生验收 |
| EX-12 | 搜索结果拖动并选中匹配 | V | 共用 `tabDrag` 载荷（MyGo 每个落点只登记一种拖动类型），带行号；落在列/标签条/列边打开并选中，显式落到锁列也可；结果已更新则拒绝。`TestDropSearchMatchTargetsTheDroppedColumn`、GUI `TestDragSearchResultToColumnEdge` |

## ED · 编辑、标签与阅读

上游源码：[`editor.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/editor.go)、[`editing.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/editing.go)、[`autoclose.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/autoclose.go)、[`selection.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/selection.go)、[`decorations.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/decorations.go)、[`sticky.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/sticky.go)、[`crumbs.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/crumbs.go)、[`reopen.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/reopen.go)。

砚实现边界：[`core/app/editing.go`](../core/app/editing.go)、[`core/app/tabs.go`](../core/app/tabs.go)、[`core/app/decorations.go`](../core/app/decorations.go)、[`core/app/sticky.go`](../core/app/sticky.go)。测试入口：[`core/app/editing_test.go`](../core/app/editing_test.go)、[`core/app/tabs_test.go`](../core/app/tabs_test.go)、[`core/app/sticky_test.go`](../core/app/sticky_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| ED-01 | 标签、行号、当前行与语法高亮 | I | 中文/长行/多列 |
| ED-02 | 磁盘重载与未保存冲突保护 | V | 保存前按原始字节核对基线；同大小/mtime、自动保存拒绝覆盖、BOM/CRLF与不可读基线回归通过。聚焦时重载；持续监听仍属EX-02 |
| ED-03 | 预览标签替换及编辑/双击/Enter固定 | I | 每列独立；会话恢复 |
| ED-04 | 关闭后按原位置重新打开文件和终端 | I | 终端重开新 shell，不重放命令 |
| ED-05 | 鼠标中键关闭后台标签 | V | 按下不显示、松开在标签内才关；`TestMiddleClickClosesBackgroundTab`。真实三键鼠标未做原生交互验收 |
| ED-06 | 全局换行及单标签覆盖 | I | 缩放/列宽变化与会话恢复 |
| ED-07 | 延时、换标签/列、失焦自动保存 | I | 无名文件仍等待另存 |
| ED-08 | 关闭脏文件的保存与取消 | I | 不丢失未保存内容 |
| ED-09 | 缩进/反缩进与智能 Enter | I | 按文件缩进设置 |
| ED-10 | 括号引号成对、包裹选区、跳过闭合符 | I | 输入法组合不破坏成对状态 |
| ED-11 | 行注释/移动/复制/删除/上下插行 | V | 无 Chroma 语言的文件按文件名/扩展名取注释符（.gitignore、.env*、.editorconfig 等）。`TestToggleCommentInFilesWithoutALanguage`、GUI `TestCommentKeyInGitignore` |
| ED-12 | 无选择时剪切整行 | I | 撤销恢复整行 |
| ED-13 | Home 到首个非空字符 | I | 保持普通选择快捷键 |
| ED-14 | 状态栏修改缩进及行尾 | I | CRLF/LF与保存一致 |
| ED-15 | 排序升降序、camelCase | I | 选择范围/全文件两种输入 |
| ED-16 | 独立选择及共享文档撤销 | I | 同文档多列共享文本/历史，选择独立 |
| ED-17 | 撤销磁盘重载及回到已保存状态 | V | 重载可撤销/重做，dirty标志正确；撤销不改变磁盘比较基线 |
| ED-18 | 缩进参考线、括号配对、同词高亮 | I | 滚动及选区变化 |
| ED-19 | 选区空白符、圆角、失焦变淡 | P | 失焦已实现；空白显示与圆角需逐项核验 |
| ED-20 | 粘性作用域及符号面包屑点击跳转 | I | 长函数和嵌套作用域 |
| ED-21 | 面包屑目录下拉树与键盘导航 | V | 移植上游 crumbs.go：弹出框自带目录树（展开状态独立于资源管理器），后台读目录并按代号丢弃旧结果，方向键/Enter，文件在同一列预览（锁列也可，`PreviewFileIn`）；项目外文件同样可用。`TestBreadcrumbFolderList`、`TestCrumbDirs` |
| ED-22 | 砚自有：转到定义（F12、平台修饰键点击、悬停下划线）与后退/前进 | V | 上游无此功能。`core/syntax/symbols/define.go`（var/let/const、`:=`、Go 分组、结构体/类/接口/枚举成员、顶层赋值、`#define`、参数）与 `core/app/definition.go`：先当前文件（就近局部、外层参数、声明），后台按语言族找文件夹，`pkg.` 限定名优先同名目录，多个时浮层选择，光标移动后丢弃迟到结果；`core/app/navigation.go` 后退/前进（macOS Ctrl+-/Ctrl+Shift+-，Windows/Linux Alt+←→）；修饰键取 `Editor.LinkModifier`（`keymap.Profile.Mods`）。`TestDefinitionInTheFile`、`TestDefinitionInTheFolder`、`TestDefinitionLinkClick`（mac/Windows/Linux 窗口与终端配置）、`TestNavigateBackAndForward`、`TestPlatformBindings`、GUI `TestGoToDefinitionInTheWindow`（截图 go-to-definition-link）。启发式，不看类型与导入；终端版取决于终端是否报告 Ctrl 点击；原生 Windows/Linux 未验收 |

## VI · Vim

上游源码：[`vim.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/vim.go)、[`vimreg.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/vimreg.go)、[`vimline.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/vimline.go)。

砚实现边界：[`core/app/vim.go`](../core/app/vim.go)。测试入口：[`core/app/vim_test.go`](../core/app/vim_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| VI-01 | 每标签 normal/insert/visual/visual-line | I | 鼠标选区与模式状态一致 |
| VI-02 | 基本词/行/段落/括号/find/till/计数移动 | I | 中文和空文件边界 |
| VI-03 | 新增 g_、括号方向、标签/段落文本对象 | P | 逐键对照上游兼容用例；不可把基础 motions 算全 |
| VI-04 | d/c/y/缩进/注释/大小写操作符 | I | 操作符加移动/文本对象/重复 |
| VI-05 | 插入、粘贴、连接、替换、点重复 | I | 插入与前置修改一起撤销 |
| VI-06 | 命名/追加/编号/黑洞/特殊寄存器 | M | 当前只有共享剪贴板基础 yank/paste |
| VI-07 | 录制宏、次数回放、@@与失败停止 | M | 回放上限、取消和单次撤销边界 |
| VI-08 | 标记随编辑移动、跳转历史、Ctrl O/I | M | 文档身份与编辑位置变换 |
| VI-09 | 增减数字及完整滚屏/对齐命令 | P | 基础滚屏存在；逐键补齐 |
| VI-10 | Vim 增量正反搜索、历史及 Vim 正则 | M | 当前不能视为完整 Vim 搜索行 |
| VI-11 | 冒号命令 w/q/x/wa/qa/行号 | M | 复用保存与关闭确认语义 |
| VI-12 | 范围替换/排序/移动复制/normal/global | M | 多行原子撤销和失败停止 |
| VI-13 | ZZ/ZQ、gv/gi、特殊寄存器显示 | P | 拆分兼容测试后补齐 |
| VI-14 | Windows/Linux vimCtrlKeys 冲突开关 | M | 默认保留工作台 Ctrl 键，用户可显式交给 Vim |

## GI · Git 基础与历史

上游源码：[`git.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/git.go)、[`hunks.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/hunks.go)、[`scm.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/scm.go)、[`history.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/history.go)、[`graph.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/graph.go)、[`commitactions.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/commitactions.go)。

砚实现边界：[`core/app/scm.go`](../core/app/scm.go)、[`core/app/history.go`](../core/app/history.go)、[`core/app/commit_tools.go`](../core/app/commit_tools.go)、[`core/app/hunks.go`](../core/app/hunks.go)。测试入口：[`core/app/git_test.go`](../core/app/git_test.go)、[`core/app/hunks_test.go`](../core/app/hunks_test.go)、[`core/app/commit_tools_test.go`](../core/app/commit_tools_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| GI-01 | Git 行号栏改动与全局概览尺 | I | 增加/删除/修改及选择导航 |
| GI-02 | 点击查看 HEAD、局部还原与暂存 | I | 还原可撤销；暂存基于最新 index |
| GI-03 | 上下改动快捷键及菜单 | I | 砚保留现有 Alt F3；可自定义 |
| GI-04 | 停顿后行内 blame | I | 查询过期取消与关闭配置 |
| GI-05 | SCM冲突/已暂存/改动/未跟踪分组 | I | 空仓库和干净仓库状态 |
| GI-06 | 按行/分组/文件夹暂存撤销丢弃 | I | 丢弃确认与未跟踪文件废纸篓 |
| GI-07 | SCM Shift范围/主修饰键多选/全选 | V | `core/app/scmpick.go`：Shift 点击/Shift+↑↓ 选一段、主修饰键（macOS ⌘，其他 Ctrl，按 `keymap.Profile`）点击加减、主修饰键+A 全选；空格、Delete（二次确认）、行上暂存/取消暂存/放弃按钮只对选中的文件；普通点击回到单选。`TestSCMMultiSelect`、`TestSCMDiscardPicked`、GUI `TestSCMMultiSelectInTheWindow`（ClickWith 修饰键，截图） |
| GI-08 | Commit/Commit All/Amend 与保存脏文件 | I | 目标仓库固定；暂存全量需确认 |
| GI-09 | 提交摘要50/72字提示 | I | `ui/gui/scm.go` 的 `subjectLength`：超过 50 字显示字数，超过 72 字用警告色；源码核对，缺专项测试 |
| GI-10 | Commit & Push、Undo及恢复消息 | I | 取消提交保留 index |
| GI-11 | AI根据改动和最近提交生成消息 | X | 流式取消/限额；需自有服务或明确选择提供方 |
| GI-12 | 树视图及单目录链压缩 | I | 文件夹操作包含所有子项 |
| GI-13 | Git进度/完成提示/可撤销提交 | I | 失败可见且不误报成功 |
| GI-14 | 初始化空仓库 | I | 仅对用户打开的根目录 |
| GI-15 | 提交图、合并/HEAD/未推送标记 | I | 分支/tag与远程装饰 |
| GI-16 | 更多提交及所有分支 | I | 每仓库保存分页状态 |
| GI-17 | 点击提交预览、固定与提交信息 | I | 首父提交差异和根提交 |
| GI-18 | 提交说明折叠 | V | 提交的标签里说明超过 8 行时只显示前 5 行和「⋯ 说明还有 N 行」，回车/双击展开，「⋯ 收起说明」收起（`diffLine.bodyToggle`、`diffTab.bodyOpen`）。`TestCommitMessageFolds` |
| GI-19 | checkout/创建分支tag/复制提交信息 | I | detached HEAD确认 |
| GI-20 | merge/rebase/reset/revert/cherry-pick | I | 继续/终止及失败保留现场 |
| GI-21 | 导出/复制含二进制补丁 | I | 根提交和合并首父 |
| GI-22 | 双向/选定提交比较及恢复 | I | 固定完整hash和仓库，不能跨库混比 |
| GI-23 | 远程提交链接 | I | GitHub/GitLab/Bitbucket与remote选择 |
| GI-24 | 分支选择器、本地远程跟踪、占用worktree | I | 保存后切换；已占用目录复用 |
| GI-25 | pull仅快进、首次push设置upstream | I | 凭据失败提示终端处理 |
| GI-26 | REPOSITORY 分支/远程持久面板 | V | `core/git/panels.go`（移植 Mayoi 的 `PanelBranches`、`LogRef`、`LogRange`、`RemoteURLs`、`ConfiguredRemoteURLs` 与参数检查）、`core/app/repopanel.go`、`ui/gui/repopanel.go`：源代码管理历史下面的「仓库」面板，分支/远程两个标签；打开面板才读引用，展开分支才读历史与待推送/待拉取，每次一页、「更多提交」翻页；按引用状态带代号丢弃过时结果；分支右键检出/与 HEAD 比较/删除，远程右键改地址/清理/删除；开关与标签记在 state。`TestShortRemoteURL`、`TestRepositoryPanelBranches`、`TestRepositoryPanelRemotes`、`TestPanelReads`、GUI `TestRepositoryPanelInTheWindow`（截图）。终端版暂不显示面板 |
| GI-27 | 分支改名/安全删除/远程增删改URL/prune | V | `core/git/manage.go` + Git 菜单/命令面板：改名、`branch -d` 拒绝未合并并二次确认后 `-D`、远程添加/改地址/删除（确认）/prune；按 SCM 选中仓库执行、经 gitDo（runCommands 可等待）。`TestRenameAndSafelyDeleteBranches`、`TestManageRemotes`。统一的分支/远程常驻面板属 GI-26 |

## MR · 多仓库

上游源码：[`gitcontext.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/gitcontext.go)、[`repositorypicker.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/repositorypicker.go)、[`submodules_test.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/submodules_test.go)。

砚实现边界：[`core/app/repositories.go`](../core/app/repositories.go)、[`core/git/submodules.go`](../core/git/submodules.go)、[`ui/gui/repositories.go`](../ui/gui/repositories.go)。测试入口：[`core/app/repositories_test.go`](../core/app/repositories_test.go)、[`ui/gui/repositories_test.go`](../ui/gui/repositories_test.go)、[`ui/tui/nested_test.go`](../ui/tui/nested_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| MR-01 | 父目录发现多个worktree并同屏 | I | alpha.15已发布，旧待办必须移除 |
| MR-02 | 递归已初始化子模块 | I | 未初始化项只留父库gitlink |
| MR-03 | 每仓库折叠、branch、草稿和提交动作 | I | 两个库同名文件与独立草稿 |
| MR-04 | 自动跟随/固定选择、独立历史分页 | I | 切换文件不改变固定选择 |
| MR-05 | 所有diff和review固定仓库 | I | 删除仓库后拒绝回落到别的库 |
| MR-06 | 破坏操作提示仓库名及过期回调隔离 | I | 移除重加仓库后旧确认失效 |
| MR-07 | 子模块提交后父gitlink单独暂存 | I | 不当作普通blob暂存 |
| MR-08 | 工作区相对路径会话 | V | 会话里文件夹内的文件、终端目录、仓库根记成相对路径（`sessionRel`/`sessionAbs`），外面的仍记绝对；新位置没有记录时，找同名、旧位置已不在、相对文件都在新位置的记录接上，保存时去掉旧位置。`TestSessionPathsRelative`、`TestSessionFollowsAMovedFolder`。评审意见的路径仍是绝对的 |
| MR-09 | 按git元数据变化发现、避免定时全索引 | V | 仓库发现改为事件驱动：文件夹监听看到 `.git`/`.gitmodules` 出现或消失、git 命令之后、窗口拿回焦点时重找；定时只作兜底（2 分钟，原 5 秒）。状态读取（1.5 秒看索引/HEAD 修改时间）不变。`TestRepositoryFoundWhenItAppears` |

## DF · 差异与评审

上游源码：[`diffview.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/diffview.go)、[`diffrows.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/diffrows.go)、[`diffhunks.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/diffhunks.go)、[`review.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/review.go)。

砚实现边界：[`core/app/diff.go`](../core/app/diff.go)、[`core/app/changes.go`](../core/app/changes.go)、[`core/app/review.go`](../core/app/review.go)、[`ui/gui/review.go`](../ui/gui/review.go)。测试入口：[`core/app/changes_test.go`](../core/app/changes_test.go)、[`core/app/split_test.go`](../core/app/split_test.go)、[`core/app/review_test.go`](../core/app/review_test.go)、[`ui/gui/review_test.go`](../ui/gui/review_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| DF-01 | 所有改动按暂存/工作区分组 | I | 一个标签按文件折叠 |
| DF-02 | SCM点击定位与滚动反向选择文件 | V | 所有改动开着时，源代码管理单击滚到该文件（折起的先展开，不另开标签；双击仍开单文件改动）；所有改动滚动后，顶上的文件在源代码管理里选中（键盘不在源代码管理时）。`TestSCMRevealsInAllChanges` |
| DF-03 | 文件头图标/重命名原路径/增减条 | V | 所有改动的文件头：折叠符、状态字母（同源代码管理）、重命名「原路径 → 路径」、+/− 计数、五格增减条（绿/红/灰，`statBar`）。`TestStatBar`、`TestDiffFileHeadersAndScopes`、GUI `TestAllChangesPolishInTheWindow`（截图）。图标用状态字母代替文件类型图标 |
| DF-04 | 大文件默认折叠和邻近视口懒读取 | V | 所有改动：改动超过 2000 行的文件第一次出现时折起；超过 1 MB 的连文字都不读，文件头写大小「展开时读取」，展开时才读；用户展开过的之后照旧展开；每次读带代号，过时的结果丢掉（`loadGen`）。按「展开」懒读取，没有按视口远近预读。`TestBigFilesFoldAndReadLazily` |
| DF-05 | 三行上下文折叠及展开 | I | 逐块展开及重新折叠 |
| DF-06 | Show Full Files、作用域@@头 | V | 「改动视图：显示整个文件」每个改动视图单独开关；代码文件的 @@ 后写新文件所在块（同面包屑按缩进找，≤5 万行）。`TestDiffFullFiles`、`TestDiffFileHeadersAndScopes` |
| DF-07 | 悬停hunk按钮、逐块取消暂存/丢弃 | V | @@ 行右侧按钮：未暂存「暂存/还原/追加」、已暂存「取消暂存」，文件头「打开文件/暂存文件（取消暂存）」、冲突「三方合并」；还原经文档可撤销故不再确认。常显而非悬停。`TestDiffStickyHeaderAndHunkButtons`、GUI 截图 |
| DF-08 | 粘性文件头和按钮 | V | 所有改动里文件标题滚出顶部后钉在上面，带它的按钮，点击回到标题（`diffStickyLine`）。`TestDiffStickyHeaderAndHunkButtons`、GUI `all-changes-sticky-header` 截图 |
| DF-09 | inline/split/auto及逐字高亮 | I | 宽度门槛和Unicode字节范围 |
| DF-10 | Diff换行和增减计数 | I | 单标签覆盖保持 |
| DF-11 | 双击定位与pager键 | I | j/k、方括号、文件/页/半页、折叠、打开、暂存 |
| DF-12 | 多行评审选区与c/n/N/Delete | I | 只读diff不误编辑文件 |
| DF-13 | 评审行卡片、编辑删除、删除撤销 | I | 源码/diff/历史提交 |
| DF-14 | 评审随文本定位、过期原文与会话 | I | 多仓库身份隔离 |
| DF-15 | 状态栏评审列表与全局上下导航 | I | 删除/暂存/历史行打开正确diff |
| DF-16 | 复制/发送全部评审至终端 | I | 粘贴后不执行，清空可撤销 |
| DF-17 | 带路径行号的diff选区复制 | P | 需核对普通copy和评审copy格式 |

## MD · 图像与 Markdown

上游源码：[`viewer.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/viewer.go)、[`markdown.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/markdown.go)、[`markdownsync.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/markdownsync.go)。

砚实现边界：[`core/app/preview.go`](../core/app/preview.go)、[`ui/gui/viewer.go`](../ui/gui/viewer.go)、[`ui/gui/markdown.go`](../ui/gui/markdown.go)、[`ui/gui/markdownview.go`](../ui/gui/markdownview.go)、[`ui/gui/markdownsync.go`](../ui/gui/markdownsync.go)。测试入口：[`core/app/files_test.go`](../core/app/files_test.go)、[`ui/gui/markdown_test.go`](../ui/gui/markdown_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| MD-01 | PNG/JPEG/GIF/WebP/BMP棋盘底预览 | I | 缩放、平移、双击、文件更新 |
| MD-02 | SVG源码/实时图像切换 | I | 另一列编辑共享文档 |
| MD-03 | Markdown标题/强调/链接/嵌套任务列表 | I | 使用Goldmark而非执行HTML |
| MD-04 | 引用/高亮代码/表格/本地图像 | I | 远程图片只显示alt，不发网络请求 |
| MD-05 | 根路径按文件所在仓库解析 | I | 多仓库选择器不得改变相对图像根 |
| MD-06 | 隐藏注释/front matter、br、其余HTML文字 | I | 不把预览当浏览器执行 |
| MD-07 | 一键右侧预览，保留源码焦点 | V | 重复调用复用，支持SVG，错误不新增空列 |
| MD-08 | 面包屑预览/右侧预览入口 | V | 命令名称与平台键位解耦 |
| MD-09 | GitHub五种提示块 | V | 嵌套、未知标记退回普通引用 |
| MD-10 | 代码语言标签/复制原始代码 | V | 保留原换行，不能复制高亮装饰 |
| MD-11 | 标题锚点及重复标题 | V | 中文/URL编码/重名不冲突 |
| MD-12 | 表格按内容列宽、条纹、标题分隔 | V | 窄窗口和中文内容 |
| MD-13 | 预览文字选择与复制 | V | 预览只读，选择不修改源码 |
| MD-14 | 字词数及阅读时间 | V | 排除代码；中日文分别计字符 |
| MD-15 | 预览状态和阅读位置恢复 | V | GUI恢复；TUI继续显示源码 |
| MD-16 | 源码和预览双向滚动 | V | 按源码块同步，不修改光标/选区；独立预览滚动状态 |
| MD-17 | 预览内块级就地编辑（砚自有，对标飞书/Typora） | V | 双击渲染块变为该块 Markdown 编辑框（原生输入法），每次修改经 `EditMarkdownLines` 按版本写回文档，与源码共用撤销/保存/自动保存；Esc、⌘/Ctrl+Enter 或失焦回到渲染；单击/拖动仍选字。`core/app/markdown_edit_test.go`、`ui/gui/markdownedit_test.go`。真实系统双击与输入法候选窗未做原生交互验收 |
| MD-18 | 编辑框格式快捷键、列表/引用回车续写、斜杠插入块、粘贴图片存文件 | V | ⌘/Ctrl+B/I/E/⇧X 切换包裹，⌘/Ctrl+K 链接，Enter 续写列表/有序/待办/引用（空项结束），Tab/⇧Tab 嵌套；行首 `/` 菜单（中文名、英文/拼音前缀）；剪贴板只有图片时存到文档旁 `assets/` 并插入链接。输入法组字期间不拦截。`ui/gui/markdownformat_test.go`、`markdownedit_test.go`、`core/app/markdown_edit_test.go`。设计见 [markdown-live-editing.md](markdown-live-editing.md)；真实剪贴板图片与输入法未做原生验收 |
| MD-19 | 在文末/块间新增块、拖动块重排、表格单元格编辑 | M | 依赖 MD-18 的编辑器基础 |

## LY · 列与终端布局

上游源码：[`columns.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/columns.go)、[`columnlock.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/columnlock.go)、[`panel.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/panel.go)、[`tabs.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/tabs.go)。

砚实现边界：[`core/app/tabs.go`](../core/app/tabs.go)、[`core/app/terminals.go`](../core/app/terminals.go)、[`ui/gui/termtabs.go`](../ui/gui/termtabs.go)、[`ui/gui/view.go`](../ui/gui/view.go)。测试入口：[`core/app/tabs_test.go`](../core/app/tabs_test.go)、[`core/app/terminals_test.go`](../core/app/terminals_test.go)、[`ui/gui/termtabs_test.go`](../ui/gui/termtabs_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| LY-01 | 拆分列、每列独立光标滚动 | I | 共享文档修改同步 |
| LY-02 | 标签重排/跨列移动/边缘新列 | I | 明确拖动不复制PTY |
| LY-03 | 列宽及侧栏拖动尺寸 | I | 小窗口限宽与会话恢复 |
| LY-04 | 锁定列及最近未锁定列路由 | V | `fileColumn` 统一路由资源管理器/快速打开/搜索/SCM/历史/差异/新终端；显式拖放仍可进锁列。`core/app/columnlock_test.go`、`ui/gui/columnlock_test.go` |
| LY-05 | 移动终端到锁定右列 | V | 标签按 id 移动、面板终端经 `MovePanelTerminal` 转标签，复用仅含终端的锁定右列；`TestTerminalMovesToLockedRightKeepingItsShell`、原生 `TestNativeTerminalMovesToLockedRight`（libghostty，模拟连接） |
| LY-06 | 锁列为空保持和至少一个未锁列 | V | `removeColumn` 不删锁列和最后未锁列，宽度给未锁列；全锁时左侧新建文件列 |
| LY-07 | 锁列/最后文件列会话恢复 | V | 会话存 `locked`，空锁列也恢复；全锁状态恢复出可编辑列。`TestLockedColumnsRestoreWithSession` |
| LY-08 | 底部终端与编辑区终端标签 | I | 移动保留进程；拆分创建新shell |
| LY-09 | 终端面板左右/底部位置 | V | `[terminal] position` 与菜单（存 state.json），VS Code `workbench.action.positionPanel*` 命令；侧边时在同侧侧边栏外，独立宽度、内侧分隔条，至少给编辑区留 320；换位置不换终端会话。`TestPanelPositionSettings`、原生 `TestNativeTerminalPanelSides`（libghostty，模拟连接） |
| LY-10 | 终端多实例、调整大小/最大化 | I | 隐藏时仍运行 |
| LY-11 | 终端路径拖入并正确shell转义 | I | 空格/引号/中文路径 |
| LY-12 | 图片剪贴板空bracketed paste | I | 只在图片无文字时触发 |
| LY-13 | 终端选中即复制配置及实时修改 | I | 双击、全选和已存在实例 |
| LY-14 | Windows/Linux终端Ctrl与工作台键分流 | P | 基础接口存在；Ctrl Shift C/V和全部例外需专项矩阵 |

## SE · 查找、搜索与快速打开

上游源码：[`quickopen.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/quickopen.go)、[`commandpalette.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/commandpalette.go)、[`find.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/find.go)、[`search.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/search.go)。

砚实现边界：[`core/app/overlay.go`](../core/app/overlay.go)、[`core/app/find.go`](../core/app/find.go)、[`core/app/search.go`](../core/app/search.go)。测试入口：[`core/app/find_test.go`](../core/app/find_test.go)、[`core/app/search_test.go`](../core/app/search_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| SE-01 | 模糊快速打开、最近优先、忽略构建输出 | I | Git跟踪与未跟踪索引 |
| SE-02 | 冒号行号、@符号、>命令面板 | I | 快捷键提示与最近命令 |
| SE-03 | 命令勾选状态及菜单归属 | P | 命令可搜索；完整显示项需核对 |
| SE-04 | 文件查找替换的大小写/整词/正则/$1 | I | 零宽和中文边界 |
| SE-05 | 大文件异步查找和取消旧结果 | P | 后台搜索与按 gen 丢弃旧结果已有；逐文件重搜按序号丢弃晚到结果（SE-08）。大文件性能基准仍需专项验证 |
| SE-06 | 工作区搜索的预览、折叠及键盘导航 | I | 结果选区与焦点区分 |
| SE-07 | 单行选区作为搜索初值 | I | 多行不误用 |
| SE-08 | 未保存文档参与并随编辑更新 | V | 磁盘变化、删除、编辑停顿 150ms 后逐文件重搜；每个文件只收最新一次请求，切项目经代号丢弃。`TestSearchResultsFollowDiskAndUnsavedEdits`、`TestDiskEventsOfAPreviousFolderAreDropped` |
| SE-09 | 二进制/大小/匹配数限制 | I | 限制提示不冒充完整搜索 |
| SE-10 | include/exclude glob与花括号 | I | 按工作区相对路径匹配 |
| SE-11 | 全部/文件/单项替换及预览 | I | 未打开文件保权限/行尾，确认不可撤销 |
| SE-12 | 替换前重新匹配，已移动结果不误替换 | V | 同大小/mtime、加载前晚到修改、未打开/后打开文件、批量重新匹配及BOM偏移回归通过；写入临时文件后再次验证，平台无跨进程原子compare-and-swap保证 |

## UP · 上游新增与未合并 PR

2026-10-27 最新复核：主线 [`d888678`](https://github.com/sorrycc/Mayoi/commit/d888678834f4d9035c04da589f044746e92049e6)（v0.12.3 之后）；砚基线 alpha.30 / `a09d29b`。`6d98a49..d888678`：Markdown 脚注、标点后裸网址、Mermaid 样式填充标签（UP-59～61，本批移植）；LaTeX 公式（UP-62）、并排预览放到旁边一列（UP-63）、关闭其他项目前先问（UP-64）、面板与列表淡入及每帧减负（UP-65）、MyGo 0.7.1 与非 ASCII 性能（UP-66）待定；上游测试修正、版本号、日文目录不移植。#76（UP-50）、#77（UP-51）已并入上游主线；#78 头更新为 `2c37c4a`；新开放 #89（UP-67）、#90（UP-68）。#6、#72、#74、#80、#83、#84、#85 头未变，未当成主线功能。

上游 `origin/main` 由 `46990a1` 前进到 [`51b4f4e`](https://github.com/sorrycc/Mayoi/commit/51b4f4e)（v0.11.3）。20 个 fork 的默认分支均无独有提交；未合并工作只在开放 PR 分支：[#50](https://github.com/sorrycc/Mayoi/pull/50)（Garfielk，IDE 式差异与三方合并，+12k 行，基于 `8708da7` 并合入 v0.11.1）、[#46](https://github.com/sorrycc/Mayoi/pull/46)（ooc7b，herdr 终端）、[#6](https://github.com/sorrycc/Mayoi/pull/6)（Jerryx333，侧边预览；与现 main 无共同祖先）。未合并 PR 的代码归各作者，移植时按行为重写并注明作者与 PR，合并状态变化后再复核。

2026-10-22 复核：固定到 [`abf7db6`](https://github.com/sorrycc/Mayoi/commit/abf7db664a4a6cd1ab1bf4376bc83a9c97829acb)（v0.11.11）。
对 `94abd9d..abf7db6` 的提交、FEATURES.md 与 SETTINGS.md 做增量登记；下列新增 P 项表示已识别、尚需行为核验，不是已经发现砚存在同一缺陷。
本批只实现 UP-29；后续优先核验 UP-44、UP-45，再做 UP-43，其次 UP-41；装饰后置，外部崩溃上报需独立方案。

2026-10-21 复核：上游又前进到 `5239063`（#60 合并、底部面板最大化修正）。同日再复核到 [`94abd9d`](https://github.com/sorrycc/Mayoi/commit/94abd9d)：#57（`80fcae9`，见 UP-24）、#62（`946ac72`）、#63（`94abd9d`）合并；开放 PR 只剩 #46。

2026-10-20 复核：上游前进到 [`6a39bd6`](https://github.com/sorrycc/Mayoi/commit/6a39bd6)（v0.11.5 之后一个提交）。#50 已合并（`91a8b15`）并有打磨提交 `23d597f`，现属 Mayoi 主线（MIT）；新合并 #53（状态栏 Git 同步）、#55（Linux 菜单崩溃）。开放 PR：#60（xierenyuan，状态栏打开共享分支浮层）、#57（bet4it，Linux 标题栏菜单按钮）、#46、#6。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| UP-01 | `0580cb5` Markdown 原位切换文本/预览时保持位置 | V | 核对发现两向都不保持（预览回到上次的位置，文字留在旧位置）；`togglePreview` 现在把文字顶行交给预览、预览顶行交给文字，`PaneFrame.PreviewGen` 让窗口版每次打开都滚到那一行。`TestMarkdownToggleKeepsThePlace` |
| UP-02 | `97bb61f` 修改行词级高亮以「旧行→新行」为键 | I | 核对：砚在一处改动里把第 i 个删除行与第 i 个新增行配对比较（`writeHunks`/`writeSplit` 的 `base[h.A+i]`↔`cur[h.B+i]`），与上游主线 `diffSpans` 的 peer 一致，不按行号错比 |
| UP-03 | PR #6 侧边预览与同步滚动 | I | 已由 MD-07/16 覆盖；`markdown.showPreviewToSide` 别名待核对 |
| UP-04 | PR #46 herdr 终端列表、附着、改名、分离 | X | 依赖外部 herdr 程序，默认关闭；接入前需确认是否采用 herdr |
| UP-05 | PR #50 差异中直接编辑工作区一侧 | P | 已做「在旁边编辑」：改动视图里 i、`@@` 行「✎ 编辑」或命令，在右边一列打开工作区文件到这处改动的行；编辑的就是文件文档（输入法、撤销、保存照旧），改动视图跟着重读；已暂存、提交里的只读。`TestDiffEditBeside`。未做：在改动视图的行上直接打字（要在 Frame 里把删除行画成文件文档上的虚行） |
| UP-06 | PR #50 改动块箭头：还原块 / Ctrl 点击追加对侧行 | V | `core/app/diffblock.go`：工作区改动（单文件、所有改动里未暂存）还原块/追加对侧行，比较两个文件左右互写；经文件文档（没开时在改动视图后面开标签）一次撤销单元，改动视图里撤销/重做走被改的文档；视图落后于文件时拒绝；已暂存、提交、新增/删除文件拒绝。GUI 在 `@@` 行右侧显示按钮（替代箭头+Ctrl 点击），键 r/a/R、命令面板。另修：改动视图里撤销曾清空视图。`TestReplaceLines`、`TestDiffBlockRevertAndAppend`、`TestAllChangesBlockRevert`、`TestCompareAcceptLeftAndRight`、`TestDiffBlockRefusals`、`TestUndoInADiffKeepsItsText`、GUI `TestDiffBlockButtonsInTheWindow` |
| UP-07 | PR #50 改动/文件导航边界提示与跨文件 | V | `core/app/diffnav.go`：文件最后/第一处改动先提示、同向再按一次才跨文件（其他移动解除），改动视图两端提示并停住，状态栏「第 i/n 处改动」；单文件改动视图按源代码管理顺序打开下一个改动文件；`. ,` 两端提示；`[diff] cross_files = ask/always/never`；Alt+F5 与 `workbench.action.compareEditor.*Change`、`yan.diff.next/previousFile` 在改动视图里走同一导航；删除行回车打开到改动行；所有改动首次读完光标在第一处改动。`TestDiffChangeNavigationBoundaries`、`TestDiffChangeNavigationCrossSetting`、`TestOneFileDiffCrossesToTheNextFile`、GUI `TestDiffNavigationBoundaryInTheWindow` |
| UP-08 | PR #50 三方合并视图（Ours/Result/Theirs，逐冲突接受/忽略，标记解决，`checkout -m` 还原） | V | `core/app/mergeregions.go`（三方区域：两边改动在基线上相接即合并，区域在结果里的位置按结果文字重算，撤销/手改自然跟随）、`core/app/merge.go`：三列（我们的/结果/他们的，等宽，两边随结果滚动），结果即文件文档（输入法、撤销、保存照旧）从共同祖先开始并已应用不冲突的改动；逐冲突用这边/两边都用/忽略（算已处理），全部用一边；选择与忽略标记一起撤销重做；标记已解决＝保存＋暂存（未处理时先确认），继续闸门沿用；保存并关闭后再开接着结果；恢复冲突走 `git checkout -m`（确认）。源代码管理、所有改动回车打开。GUI 两侧按钮、Git 菜单「合并冲突」、状态栏「标记为已解决」。`TestMergeRegions`、`TestMergeResultRanges`、`TestMergeViewOpens`、`TestMergeViewChoicesAndUndo`、`TestMergeViewSides`、`TestMergeViewResolveAndContinue`、`TestMergeViewRestoreConflict`、`TestMergeViewFromSourceControl`、GUI `TestMergeViewInTheWindow`（截图）。一边删除文件的冲突不进三方合并 |
| UP-09 | PR #50 冲突文件清单逐个处理并跨重启保留 | V | `core/app/mergefiles.go`：冲突文件（未解决在前、已解决打勾）浮层，命令遇到冲突停下来、状态读到后自动列出；已解决的记在工作树 git 目录 `yan-merge-resolved.json`（按命令与其提交号认，别的命令或做完即删）；「应用并打开下一个」＝暂存＋打开下一个冲突文件；已解决的可打开或恢复成冲突。`TestMergeFilesWorkedFileByFile`（含新建编辑器模拟重启、继续后记录删除）、`TestMergeFilesRestoreResolved`、GUI 截图 merge-files。未做：Mayoi 的「自动应用」开关（砚总是先应用不冲突的改动） |
| UP-10 | PR #50 差异视图配置：词/字/行高亮、上下文 0～10、同步滚动、行对齐开关 | V | 词/字/行高亮、上下文、排法见前（`diffOpts`）；窗口版并排改为两个窗格（`core/app/splitdiff.go`、`ui/gui/splitdiff.go`）：行对齐时缺的一边留空行，关掉后每边自成一列（右边随左边的行速走）；同步滚动关掉后滚轮只滚指着的一边，再打开从右边所在的行对齐；键盘走到哪右边跟到哪。`[diff] sync_scroll/aligned_rows`、菜单开关存 state。`TestSplitDiffPanes`、GUI `TestSplitDiffInTheWindow`（截图）。终端版仍是一份「左 │ 右」文字 |
| UP-11 | PR #50 空白过滤（精确/行尾/全部/空行） | V | `core/app/diffcompare.go` 的 `diffHunks`：按过滤后的键比较、块仍指向原行；单文件、所有改动、逐块暂存（`stageHunkOf` 同一份块）与评审摘录共用；Git 菜单/命令面板选择，存 state，`[diff] whitespace`。`TestWhitespaceHunks`、`TestDiffWhitespaceFilter`（行尾空白隐藏、暂存只暂存可见块、文件不变）、`TestDiffWhitespaceSetting`、GUI `TestCompareFilesAndWhitespaceInTheWindow` |
| UP-12 | PR #50 任意两个本地文件比较 | V | `CompareFiles`：两边都读打开文档的未保存文字、随编辑刷新；删除行回车打开左边文件；拒绝暂存、同一文件和文件夹。入口：Git 菜单/命令面板（窗口版系统对话框，终端版两次输入路径）、资源管理器「选为比较对象 / 与已选文件比较」。`TestCompareTwoFiles`、`TestExplorerCompareWithSelected`、GUI 截图。左右互写见 UP-06 |
| UP-13 | `e2dabc1` 每小时检查更新，并在状态栏、菜单、欢迎页提示新版本 | V | `ui/gui/updates.go`：更新插件不再自己定时开窗（`DisableAutomaticChecks`），砚每小时（启动、唤醒后 10 秒）`Updater.Check`；找到的版本（跳过的除外）显示在状态栏、欢迎页、帮助菜单「更新到 X…」，点了才开更新窗口；「自动检查更新（每小时）」存 state（`UpdateChecksOff`）。`TestUpdateChecksSetting`。真实检查与开窗只经现有 `update-feed` 自测入口，未另做原生验收 |
| UP-14 | `73e71a8` 以本机名称登录 Mon、登录前注销旧令牌 | X | 依赖 Mon 账户服务（CF-15～19 同一外部依赖），不接入 |
| UP-15 | #53 状态栏分支旁 Git 同步：先只快进拉取、再推送到上游；没有上游时发布 | V | `core/git/manage.go` 的 `Sync`（移植 #53，MIT）、`git.sync` 命令、`view.Status.Sync/Publish`，窗口版分支旁按钮（无上游时显示发布）。`TestSync`、`TestGitSyncFromTheStatusBar` |
| UP-16 | `23d597f`/#50 改动内的行配对，行内高亮对配对的行（行移动后不再错比） | I | 上游主线（`diffrows.go` 的 `lineRows`）按改动内位置配对，相似度配对只在 PR 分支、未进主线；砚同样按位置配对（见 UP-02），无差异 |
| UP-17 | `23d597f` 并排 diff 中间沟槽：每处改动一条弯带连接两侧，箭头在带里，悬停点亮、写入的行短暂发光；跳转平滑滑动 | V | 两窗格中间 40 DIP 沟槽：每处改动一条三次曲线弯带（改、加、删三种颜色，一边没有行时收成一点），带里「»」按第一行放，点击还原（比较两个文件时左写进右），按住 Ctrl/Alt 点击追加；指着箭头点亮要换掉的左边行；块操作写入的行发光 0.9 秒后淡掉（一栏改动视图也发光）；跳到改动、文件时两窗格平滑滑过去（核心多给上下各一屏的行）。`TestSplitArrowRevertsAndGlows`、`TestSplitDiffPanes`、GUI 截图 split-diff-*；两窗格右边行号旁的「+ 意见」按钮（悬停出现、有意见的常显），没选中时复制只复制上次点的那一边的行（`copySplitLine`）。`TestSplitCopyOneSide` |
| UP-18 | `23d597f`/#50 三方合并：不冲突改动可逐个「应用这处」，「自动应用」开关，进度线，两侧沟槽 | P | 已做：两侧不冲突改动未在结果里时显示「» 应用这处」，命令应用两边/一边全部（一步撤销），「打开时自动应用」开关存 state（`mergeManual`）。`TestMergeApplyChanges`。未做：进度线、两侧沟槽弯带（纯窗口版绘制，同 UP-17） |
| UP-19 | #55 Linux 改菜单时 GTK 正在绘制会崩溃：菜单栏在绘制之后替换，只应用最新一次 | V | `ui/gui/appmenu.go` 的 `setAppMenu`（Linux 经 `RunOnMain` 推迟、按版本号丢弃旧的），替换所有 `App.SetMenu`。只经 Linux 交叉编译，未做原生验收；之后 Windows、Linux 不再设原生菜单栏（UP-24），Linux 上的延后替换已不再用到 |
| UP-20 | `6a39bd6` 按住 Command 滚轮不再缩放终端字号 | I | 砚固定的 MyGo 终端没有滚轮缩放，无需改动 |
| UP-21 | v0.11.5 整个文件的改动：删除的跟踪文件「恢复文件」、未跟踪文件「删除文件…」 | V | 所有改动文件头：删除的跟踪文件「恢复文件…」（从暂存区）、未跟踪「删除文件…」（移到 `app.TrashName`）、其余「放弃修改…」，都先问，经 `discard`（开着的文档重读）；`view.DiffBlockFileDeleted/Untracked`。`TestDiffWholeFileActions` |
| UP-22 | v0.11.5 diff 折叠/展开、换行、改宽度后顶部仍是同一行；手选排法不被宽度改变 | V | `keepDiffPlace`：每次重排（展开折叠、换排法、窗格变宽变窄、文件或所有改动重读）前记下顶上一行与光标所在行（文件、种类、行号），重排后找回；手选并排/一栏本来就不随宽度变。`TestDiffKeepsItsPlace` |
| UP-23 | 开放 PR #60 点状态栏分支打开与源代码管理共用的可搜索分支浮层，常驻「新建分支…」 | V | 状态栏分支点开 `git.checkout` 浮层；没输入时第一项常驻「新建分支…」，问名字时边打边说哪里不对（`branchNameProblem`：git 引用规则、重名），绑定发起时的仓库；输入的名字不合规时不给「新建分支 X」。`TestBranchNameProblem`、`TestCreateBranchFromThePicker` |
| UP-24 | 开放 PR #57 Linux/Windows 标题栏菜单按钮、窗口按钮整理、浅色深色不受桌面配色左右 | V | Windows、Linux 不设原生菜单栏，标题栏左边「菜单」按钮在窗口里打开同一份菜单（`populateMenu`：子菜单、勾选、快捷键，剪切复制粘贴全选与全屏、窗口子菜单改由砚的命令做）；Linux 用 purego 给 GTK 加 CSS：收起标题栏空白标签（不再挡住旁边按钮的点击），窗口按钮去掉图标阴影、颜色跟主题（换主题时替换并释放旧的 provider）；选了浅色/深色时调色板不跟桌面环境。`TestAcceleratorKey`、`TestTitleBarMenuButton`（截图）。只经 Linux、Windows 交叉编译，未做原生验收；砚标题栏没有终端按钮，不涉及上游的终端按钮挪位 |
| UP-25 | `5239063` 终端面板在底部最大化时占满窗口（不再挤成一格宽） | I | 核对：砚底部面板最大化只把编辑区各列的高度收成 0（`ui/gui/view.go`），宽度不动，没有这个问题 |
| UP-26 | #62 三方合并三栏一起滚、沟槽弯带不越出沟槽 | V | 核对：砚的两侧随结果滚，按区域对到同一处（`sideLine`），没有上游那次回退；补了守护测试 `TestMergeSidesScrollWithTheResult`（结果滚过我们加的行、他们删的行，两边顶行都对上，不滚过自己的末尾）。弯带：砚的合并视图还没有沟槽弯带（UP-18），不涉及 |
| UP-27 | #63 窗口变窄时合并的操作按需收进「⋯」菜单，最不需要的先收 | V | 砚的合并没有标题栏（标记已解决在状态栏，保存走快捷键），操作是冲突行右边的按钮；现在所有改动/合并行的按钮（`diffBlockFit`）放不下时按 `diffBlockDrop` 的次序收进行尾「⋯」（「更多操作」），菜单里按原顺序、照样能用。`TestDiffBlockFit`、GUI `TestMergeViewInTheWindow`（逐步收窄窗口，截图 merge-view-narrow） |
| UP-28 | 砚自有：关终端时结束它里面还在跑的程序 | V | 用户报：终端里跑着项目，关掉再开，进程还在后台跑却看不到。MyGo 关终端只挂断、两秒后只杀 shell 自己的进程组，接住或忽略 SIGHUP 的作业（另一进程组）留下。`internal/termsession`：挂断两秒后结束 shell 会话里剩下的进程，退出应用时等它收尾（`OnWillQuit`）；shell 进程号经 `ui/gui/termpid.go` 从 MyGo 读。`TestEndKillsWhatIgnoresTheHangup`、`-tags selftest` 的 `TestNativeClosedTerminalEndsItsPrograms`（真终端，去掉修复即失败）。Linux 只经 vet/交叉编译；Windows 由 ConPTY 结束控制台程序，未验收；自己 setsid 出去的守护进程不动 |
| UP-29 | `d81abe4` 新终端默认放编辑区或面板 | V | `core/app/terminals.go`、`core/config`、GUI 菜单与资源管理器：`[terminal] location`、`view.terminalLocation`、`openInIntegratedTerminal`；文件/目录/空白路径、锁定列、显式打开与 TUI 回退。`TestTerminalLocationRouting/ExplorerAndLockedColumn/TUIFallback/Settings`、`TestTerminalLocationMenu`；原生 Windows/Linux 未验收 |
| UP-30 | `c6370d3` 终端标签溢出仍可点击 | P | 待对照 `panel.go` 的布局和命中区域，验收多标签、窄面板、滚动与关闭；不能仅凭砚已有标签条关闭此项 |
| UP-31 | `7954376` 崩溃日志含版本/系统、后台 panic 和 macOS 系统报告归档 | P | 砚已有 `internal/crashlog`；需逐项核对元信息、后台任务恢复边界和系统报告导入，不能直接吞掉共享状态写入时的 panic |
| UP-32 | `7954376` 用户同意后向 Mon 匿名发送崩溃报告 | X | 上游服务方案不直接接入。需独立端点、报告预览、路径脱敏、同意/拒绝持久化与禁用机制；未发送任何报告 |
| UP-33 | `b27e992`、`bf62d50` 窄 diff 文件头保留状态、扩展名和重命名关系 | P | 与砚 UP-27 操作按钮溢出不是同一契约；需窄列文件头、长路径与重命名截图验证 |
| UP-34 | `e18f9ba` Finder 复制的文件粘贴为 shell 引用后的完整路径 | V | `OnPaste` 经宿主文件剪贴板优先于文本，复用平台 ShellQuote；`TestNativeTerminalFilePaste` 用真实控件和模拟连接验证多路径、空格/引号、普通文本与图片空粘贴、不自动提交。Windows 引用有平台单测；系统剪贴板与原生 Windows/Linux 运行仍待验收 |
| UP-35 | `62b737f`、`06419e4` Git 2.23 以前切分支、分离 HEAD、取消暂存与放弃修改的回退 | P | 核对 `core/git` 命令能力探测与 checkout/reset 路径；失败不得误改索引/工作区，需旧 Git 或命令桩回归 |
| UP-36 | `a5a9fa0` macOS 长按键重复，不弹重音菜单 | P | 平台限定行为应隔离在 GUI Darwin 文件；需原生编辑器/终端长按、中文 IME 验收，Windows/Linux 保留原行为 |
| UP-37 | `34c00c4`、`046b499` MyGo 0.6.0 与 Linux Pango tab/cairo 修正 | P | 砚仍固定旧 fork；升级前验证终端剪贴板、会话关闭进程号读取、原生库、布局及三平台编译，不直接移除 replace |
| UP-38 | `ffa68d2` 未命名文档退出后恢复文本 | V | 第一批：无路径的未命名文档（⌘N）、另存为先写目标再绑定路径。第二批：每个编辑器一个备份文件（`core/recovery`，同目录临时文件、fsync、改名、目录 fsync，0600/0700，坏文件移到一边），后台写代号防晚到覆盖，关窗口/退出/重启更新/换文件夹前同步写，写成了未命名文档不再问；文件夹按路径认、没开文件夹的各有编号，进程间加锁，别的 yan 占着时另外备份。`core/recovery` 测试、`TestKeepAndRestoreDrafts` 等、`TestWindowCloseKeepsDrafts`；独立 macOS 应用两个无文件夹项目退出后重开都找回。终端版无文件夹时找回留给窗口版；有路径的脏文件不在范围内；Windows/Linux 原生未验收 |
| UP-39 | `2c6328d` Markdown HTML 和远程图片/徽章 | P | 补 HTML 子集、布局、远程加载缓存/超时与失败状态核验；未实现任意脚本执行，也不应引入浏览器执行环境 |
| UP-40 | `c74bc57` Vim 模式徽标文字切换不交叉淡出 | P | 核对砚标签是否受颜色/透明度动画影响；快速切换模式需截图，不能因上游 Windows 跳测而省略砚的平台验收 |
| UP-41 | `d81bfc4` 主题选择器实时预览、左右换外观、Esc 回滚与减弱动画 | V | `core/app/themepicker.go`、GUI overlay/themefade：独立预览状态、模糊搜索、确认持久化/取消、查询光标与组字、Matrix 深色、多个窗口与外部配置、减弱动画；核心/GUI 回归和深浅色/窄窗口截图已验证，原生 OS 外观/IME 与 Windows/Linux 运行仍待验收 |
| UP-42 | `f3fec56` Grok Bot 主题与吉祥物 | M | 新增装饰，后置。Qoder/Qoduck 仍按用户要求排除，不因新主题一并引入 |
| UP-43 | `8ae1d55`、`3759c8b` 提交文件树：diff 旁侧面板、SCM 提交展开、键盘导航、宽度与开关 | V | 已实现共享 Frame 文件行、GUI 默认隐藏面板/树列表/宽度与窄列布局、SCM 展开和 GUI/TUI 键盘导航；仓库/完整提交号/标签身份隔离，历史删除/重命名/二进制测试。原生平台验收待完成 |
| UP-44 | `e57d684` 重开或已聚焦查找框时全选查询 | V | 已实现共享 Frame 单次选中请求、查找/替换重复唤起、中文组字期间不发布临时查询；GUI 测试器与三平台按键路由覆盖，系统输入法/原生平台验收待完成 |
| UP-45 | `9ed2adb` 查找框跨列及跨窗口拖动文件/终端的归属修正 | V | Frame 快照单列绑定及窗口负载隔离已存在；补跨列点击、标签条/正文/两侧分列/终端面板拒绝外窗负载回归，终端解析复核所有者。原生跨窗拖放待验收；砚不接入 Mon 上报 |
| UP-46 | `6a54bc3`、`b872c91` 测试按核分片、等待后台完成、lint 与 CI 节流 | P | 砚已有 PR-only CI 与取消旧运行；评估明确的后台完成等待以替代固定时间，保持测试断言。不要照搬 Windows 跳测作为通过，也不增加高频云运行 |
| UP-47 | `ed606ab` 欢迎页 GitHub 仓库入口 | P | 如采纳，应指向可公开访问的项目入口并使用砚自己的文案 |
| UP-48 | `e1519fb` 命令行 socket：运行/列出命令、定位行、diff、新窗口、SSH、等待文件关闭 | P | 后续独立设计命令与进程生命周期；本批不扩展 CLI，不复制上游窗口架构；开放 [PR #85](https://github.com/sorrycc/Mayoi/pull/85)（`3576a8d`）把命令装到登录 shell 的 PATH，一并复核 |
| UP-49 | [#74](https://github.com/sorrycc/Mayoi/pull/74) 合并三栏跟随换行，结果栏可横向滚动 | P | 上游未合并，头 `941b516`；核验砚长行、窄列、对齐/滚动和编辑光标位置，不能破坏三方版本映射 |
| UP-50 | [#76](https://github.com/sorrycc/Mayoi/pull/76) 子模块 SCM 交互 | P | 已并入上游主线 `20e7968`（约 1100 行，单独一批）；与 MR-02/07 现有 gitlink 保护逐项对照，补导航和父子仓库操作的回归 |
| UP-51 | [#77](https://github.com/sorrycc/Mayoi/pull/77) 点击终端文件链接在编辑器打开 | V | 按 PR `fce5318`（已并入上游主线 `f4f71e5`）内化 OSC 7/启动目录、绝对/家目录、行列与字面冒号名；`TestTerminalFileTarget`、`TestGotoLocation`、GUI 文档/锁列回归、`TestNativeTerminalFileLinks` 真实控件点击通过。URL 继续交系统。Windows 使用 filepath，反斜杠文本识别及原生桌面仍待验收；上游 PR 未合并 |
| UP-52 | [#78](https://github.com/sorrycc/Mayoi/pull/78) Markdown/MDX 默认预览 | P | 上游未合并，头 `2c37c4a`；需可配置、源码切换、只读预览和会话恢复；不执行 MDX 中的 JavaScript |
| UP-53 | [#80](https://github.com/sorrycc/Mayoi/pull/80) 文件和目录拖出到其他应用 | P | 上游未合并，头 `718b1e3`；砚已有 transfer.FileData，专项核验剪贴板格式、目录、已删除文件、原生系统拖放 |
| UP-54 | `2650b9b` 桌面启动的 Git/hooks 使用登录 shell PATH | V | GUI 启用一次有界探测，Git 可执行文件与 hooks 共用路径，失败沿用继承环境；不写全局 PATH。`internal/process/loginpath*_test.go` 覆盖噪声/异常/超时/输出限额/子进程管道与并发，`TestCommitHookUsesLoginPath` 验证真实 hook。独立 macOS 系统启动测试通过；另一独立应用在探测挂起时退出，退出钩子在 3 秒超时前结束假 shell 及占住管道的子进程（强制结束和逃出进程组的子进程不在保证内）。Windows 保留继承环境，Linux/Windows 原生桌面仍开放 |
| UP-55 | `f1498a9`/`2ba2637` Markdown 的 mermaid 块与 `.mmd`/`.mermaid` 预览画成随主题着色的图，可看源码/复制，`previewMermaid` 关闭 | V | `ui/gui/mermaid.go`：Pele 1.9.0（MIT，`scripts/pele.sh` 可复现）在 goja 里只跑打包脚本，文档文字只当数据；`[ui] preview_mermaid` 关闭；`.mmd`/`.mermaid` 走 Markdown 预览；宽图可超出正文栏。终端版不链接 goja（regexp2 升级使其 +约 200 KB，门禁内），窗口版 +约 6.4 MB；终端版无预览，`.mmd` 提示去窗口版。`TestMermaidUserFlowchart`/`ThemeColors`/`PreviewWindow`/`FilePreview` 覆盖中文标签、判断、边标签、回环、虚线、深浅色、错误说明、显示源码/复制与开关，含无头截图。原生 Windows/Linux 桌面未验收 |
| UP-56 | `6d98a49` 超长单行文件保护 | V | `core/text.Load` 在排版前拒绝单行内容超过 64 KiB 的磁盘文件（BOM、LF/CRLF 不计）；GUI/TUI 打开、预览、恢复、重载与还原共用入口。拒绝重载不改变缓冲区、撤销或保存基线；恢复跳过问题文件并提示。边界、26 MiB source map、脏文件/保存冲突/恢复测试及原生 macOS 验收见 `docs/up56-long-line-acceptance.md`；Windows/Linux 原生桌面仍开放 |
| UP-57 | [#83](https://github.com/sorrycc/Mayoi/pull/83) 转到符号遇行首 `func() {` 不崩溃 | V | 上游未合并，头 `b36c7d0`；砚 `core/syntax/symbols` 有同一越界（已复现，转到定义扫文件夹时也会碰到），空参数不当接收者。`TestFuncLiteralLine`、`TestFindAnonymousFunc` |
| UP-58 | [#84](https://github.com/sorrycc/Mayoi/pull/84) Homebrew cask 安装 | P | 砚经 `corn12138/yan-releases` 分发；是否提供 cask、放在哪个仓库另行决定 |
| UP-59 | `b906ff6` Mermaid 节点、子图被 `style`/`classDef` 填色后标签仍可读 | V | 对比度不足 3 时换主题文字色或背景色中对比更高的，文字自带 `color` 不改；`TestMermaidStyledFillLabels` 覆盖深浅两种主题 |
| UP-60 | `3d03b96` 标点（全角冒号、括号）、汉字、数字后的裸网址成链接 | V | `ui/gui/markdownlinkify.go` AST 变换，去尾标点、未配对的 `)` 与实体；代码与已有链接不动。`TestMarkdownLinkifyAfterPunctuation` |
| UP-61 | `88e00cf` Markdown 脚注：上标引用、末尾列表、回链 | V | 锚点同 GitHub，复用页内链接滚动；脚注小一号、浅色。`TestMarkdownFootnotes`、`TestMarkdownFootnoteLinks`（点引用到脚注、点 ↩ 回来）及截图 |
| UP-62 | `1010308` Markdown LaTeX 公式预览 | V | `ui/gui/math.go`、`markdown.go`、`markdownview.go`：MathJax 3.2.2 本地 SVG，行内/块/`math` 围栏、深浅色、表格对齐、源码复制与编辑撤销；`[ui] preview_math` 支持用户/项目配置和热重载。输入/时间/数量上限、宏隔离、超时恢复；依赖固定 `4bd079439ee7`，终端回归及真实 macOS 窗口验收通过。详见 `docs/up62-math-acceptance.md`；原生 Windows/Linux 尚未验收 |
| UP-63 | `64bd32c` 「在右侧打开预览」复用右侧最近的未锁定列，否则左侧；没有其他未锁定列时才拆分 | V | 2026-10-28：`core/app/preview.go` 保留源码焦点、共享文档和既有标签，跳过锁列；`TestPreviewReusesUnlockedSideColumn` 覆盖左右优先级、锁列、已有标签、列宽和重复调用，`TestPreviewSkipsLockedExistingPreview` 防止复用锁定的预览列；原有会话恢复/双向滚动回归保留。macOS 原生中英文窗口通过右侧 Markdown 复用、源码焦点与重复调用验证（`SelfTestLanguage`）；左侧/锁列/其他格式的原生布局验收及 Windows/Linux 桌面验收仍开放；已随 alpha.33 发布 |
| UP-64 | `d888678` 关闭其他项目前先问一次 | V | 默认取消的总确认；逐个处理未保存文件，取消或保存失败停止后续关闭；新增项目不被关闭。自动测试覆盖取消/保存/冲突/新增项目，macOS 原生验证总确认及保存冲突保留项目 |
| UP-65 | `58b5041`/`68ee016` 面板、列表、跳转落点淡入，每帧少做事 | P | 动效属设计取舍；其中的性能改动（作用域查找、词法一次读行）可单独评估 |
| UP-66 | `8122013`/`f914000` MyGo 0.7.1（需 Go 1.27.3）与非 ASCII 文本每帧减负 | P | 依赖升级单独一批：砚用 sorrycc/mygo 分支，需重新核对补丁与体积 |
| UP-67 | [#89](https://github.com/sorrycc/Mayoi/pull/89) 标签菜单「复制相对路径」 | P | 上游未合并，头 `93f6d75`；砚已有复制相对路径命令，核对标签右键菜单 |
| UP-68 | [#90](https://github.com/sorrycc/Mayoi/pull/90) 窗口缩放，支持小数级别 | P | 上游未合并，头 `8159b32`；砚已有代码字号缩放，核对整窗缩放的平台差异 |

## WN · 窗口、项目与命令行

上游源码：[`app.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/app.go)、[`projects.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/projects.go)、[`session.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/session.go)、[`recentpicker.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/recentpicker.go)。

砚实现边界：[`core/app/projects.go`](../core/app/projects.go)、[`ui/gui/projects.go`](../ui/gui/projects.go)、[`ui/gui/open.go`](../ui/gui/open.go)、[`internal/launch/request.go`](../internal/launch/request.go)。测试入口：[`ui/gui/projects_test.go`](../ui/gui/projects_test.go)、[`ui/gui/open_test.go`](../ui/gui/open_test.go)、[`internal/launch/request_test.go`](../internal/launch/request_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| WN-01 | 当前窗口/新窗口打开与欢迎页 | I | 现有项目不丢失 |
| WN-02 | 最近目录、清空、模糊查找、快捷新窗 | I | 失效路径过滤 |
| WN-03 | 已有目录窗口前置、第二进程投递 | I | 规范路径与别名目录 |
| WN-04 | Dock最近目录及文件夹投递 | I | macOS适配，不侵入core |
| WN-05 | 项目轨道开关与多窗口合并/拆分 | I | 已运行终端随项目保留 |
| WN-06 | 项目顺序拖动及目录拖入 | I | 退出恢复顺序 |
| WN-07 | 项目独立标签/列/Explorer/SCM/布局 | I | 切换状态隔离 |
| WN-08 | 项目未保存与终端通知标记 | I | 切回项目清除注意标记 |
| WN-09 | 项目favicon/logo自动发现 | V | 移植上游候选路径与 HTML/TSX 声明图标解析（`core/workspace/logo.go`，只读文件头 64 KB、只读普通文件）；后台查找、按项目缓存；SVG/位图/ICO 内嵌 PNG 可显示，ICNS 与纯 BMP 的 ICO 回落到缩写。`TestLogoFiles`、`TestExtractIconHref`、`TestIcoPNGTakesTheLargest`、`TestProjectLogoFound`、`TestProjectTabs` 截图 |
| WN-10 | 缩写冲突消歧 | V | 移植上游 `railLabels`；一个词取两字母，中文取首字，冲突时取首个不同的字母/字。`TestRailLabels`、`TestProjectInitials` |
| WN-11 | 项目自定义两字符/emoji/颜色 | V | `yan.project.customize`（方块右键或命令面板）：最多两个字素（表情算一个）+ 六种颜色或自动，存 state.json 的 `projectLooks`。`TestCustomizeProjectLook` |
| WN-12 | 自定义图标复制/拖入/重置 | V | 右键「选择图标…」或把图片拖到方块上：按内容哈希复制到配置目录 `icons/`（同图只存一份），无项目使用时删除；「重置图标」。优先级：自定义标识 > 自定义图标 > 发现的 logo > 缩写。`TestProjectIconCopies` |
| WN-13 | 项目关闭/其他项目关闭/脏文档确认 | I | 取消不能丢内容 |
| WN-14 | 侧栏右侧与尺寸/窗口会话 | I | 跨项目独立状态 |
| WN-15 | 安装/卸载yan命令 | I | 受支持平台明确提示 |
| WN-16 | 命令立即返回并启动/定位窗口 | I | 文件进入所属项目 |

## SS · Remote SSH

上游源码：[`remote.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/remote.go)、[`remotepicker.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/remotepicker.go)、[`remotesearch.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/remotesearch.go)、[`remoteterm.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/remoteterm.go)、[`transfer.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/transfer.go)、[`ports.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/ports.go)。

砚实现边界：[`core/app/workspace.go`](../core/app/workspace.go)、[`core/git/git.go`](../core/git/git.go)。测试入口：[`internal/archtest/archtest_test.go`](../internal/archtest/archtest_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| SS-01 | 系统SSH配置/Include/最近host发现 | M | 不能把本地路径接口直接用于远程 |
| SS-02 | host/user/port解析与连接进度取消 | M | 无密码采集；已知主机校验 |
| SS-03 | 远程目录选择和窗口/项目打开 | M | host+path作为身份 |
| SS-04 | ControlMaster共享与最后窗口关闭释放 | M | macOS/Linux实现；Windows能力接口 |
| SS-05 | 远程Explorer/搜索/Git/blame/diff | M | 设计文件系统及命令执行适配层 |
| SS-06 | 远程目录预读缓存 | M | 限并发、过期重读、忽略目录 |
| SS-07 | 异步打开/完整临时文件上传保存 | M | 保owner/mode/link且中断不破坏原文件 |
| SS-08 | 保存前版本冲突与还原撤销 | M | 用户明确选择覆盖，不自动覆盖 |
| SS-09 | inotify/轮询/前后台刷新 | M | 不支持高精度时间时明示限制 |
| SS-10 | 断连只读banner和保留脏内容 | M | 重连后重新验证远端版本 |
| SS-11 | 指数重连/手动重连/认证失败不重试 | M | 关闭窗口取消计时与任务 |
| SS-12 | 远程终端重启与输出/目录保留 | M | 不会自动重放历史命令 |
| SS-13 | 本地远程/跨host复制上传下载 | M | 临时落地、命名冲突与原子完成 |
| SS-14 | 传输进度取消及部分文件清理 | M | 不能删掉传输前已有文件 |
| SS-15 | 远程终端文件/截图上传粘贴 | M | 临时目录生命周期和shell转义 |
| SS-16 | localhost链接自动转发/手动端口 | M | 端口冲突换本地端口 |
| SS-17 | 端口列表打开/复制/关闭及重连 | M | 最后连接关闭时清理 |
| SS-18 | 远程删除确认、隐藏本地文件管理器 | M | 远程无废纸篓必须明确永久删除 |
| SS-19 | 远程最近项目与启动会话重连 | M | 离线仍可显示已保存项目 |

## CF · 外观、配置、账户与发布

上游源码：[`theme.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/theme.go)、[`account.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/account.go)、[`syncview.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/syncview.go)、[`aicommit.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/aicommit.go)、[`keymap.go`](https://github.com/sorrycc/Mayoi/blob/46990a1ca9945de7b90b94104c32f2d7770461f5/cmd/mayoi/keymap.go)。

砚实现边界：[`core/config/config.go`](../core/config/config.go)、[`core/app/keybindings.go`](../core/app/keybindings.go)、[`core/keymap/when.go`](../core/keymap/when.go)、[`ui/gui/updates.go`](../ui/gui/updates.go)。测试入口：[`core/app/keybindings_test.go`](../core/app/keybindings_test.go)、[`core/app/platform_test.go`](../core/app/platform_test.go)、[`ui/gui/updates_test.go`](../ui/gui/updates_test.go)。

| ID | 功能契约 | 状态 | 验收/剩余工作 |
| --- | --- | --- | --- |
| CF-01 | Mayoi/Matrix/Claude与系统明暗 | I | Matrix固定暗色 |
| CF-02 | Matrix雨、Clawd与减少动态效果 | I | 后台暂停并允许关闭 |
| CF-03 | Qoder主题和Qoduck | D | 用户明确跳过，未被本轮广泛请求撤销 |
| CF-04 | 配置文件格式 | D | 保留TOML与项目覆盖，不迁移settings.json |
| CF-05 | 字体family/size/line-height | I | 跨平台设置入口 |
| CF-06 | 字体weight/features与独立terminal字体 | M | 共享字体默认，独立字号不受编辑器zoom影响 |
| CF-07 | 英文/中文/日文和实时切换 | P | alpha.33 已支持跟随系统、简体中文和英文，重启后统一生效；日文与实时切换未做，不能算完整上游契约 |
| CF-08 | 保存配置实时应用所有窗口 | V | 每个窗口监听用户配置目录，项目 .yan.toml 走项目监听；项目覆盖保留；格式错时沿用进程内上次可读内容并常驻状态栏。`TestExternalConfigChangesApplyToEveryWindow`、`TestMalformedReloadKeepsLastValidConfiguration`（含 keybindings 重载）。配置目录启动后才创建时，需保存或聚焦后生效 |
| CF-09 | VS Code JSONC、chord、解绑与命令别名 | I | 错误可见，保留有效配置 |
| CF-10 | 动态when选区/终端/侧栏/系统上下文 | V | 含运行时名字的绑定编成守卫命令，按下时求值，不成立回落到原键或照常打字；系统名按 `keymap.Profile.Platform()`。`TestWhenSelectionIsEvaluatedAtKeyPress`、`TestWhenPlatformFollowsProfile` |
| CF-11 | vscode.open/type/terminal.sendSequence args | V | 读配置时校验参数；type 按焦点进编辑器/输入框或终端；sendSequence 经宿主发往最近终端，无终端报错；路径支持绝对/~/file:///项目相对。`TestBindingArgsTypeSendSequenceAndOpen`、`TestBindingArgsAreCheckedOnLoad`。GUI 终端真实发送未做原生交互验收 |
| CF-12 | runCommands等待异步动作和遇错停止 | V | gitDo 计数等待后台 Git 完成，同步错误和后台失败都停止剩余命令；切项目清空。`TestRunCommandsWaitsAndStopsOnFailure` |
| CF-13 | 输入框/终端/图片/diff的用户键位优先 | P | 守卫不成立时 TryKey 交还按键，输入法组字与 Option/AltGr 打字不受影响；terminalFocus 可求值。GUI 终端控件先消费的按键仍到不了内核，需 MyGo 终端层前置拦截 |
| CF-14 | 全平台菜单命名及快捷键适配 | I | Profile.OS/FileManager/TrashName |
| CF-15 | 账户设备码GitHub/Google登录 | X | 不得默认接入他人Mon账号服务；先提供可配置服务边界 |
| CF-16 | 系统安全凭据存储及登出 | X | Keychain/Credential Manager/Secret Service |
| CF-17 | 设置逐键同步/冲突解决/忽略项 | X | 独立于本机会话；需服务协议和验证账户 |
| CF-18 | keybindings双端冲突并排合并 | X | 保留本地与远端两个版本 |
| CF-19 | 同步退避/失焦前推送/状态面板 | X | 离线不丢设置；支持取消与登出 |
| CF-20 | AI模型选择和取消/限额提示 | X | 与提交草稿及仓库身份绑定 |
| CF-21 | 签名自动更新/手动检查/跳过/提醒 | I | MIT 源码与独立公开安装包仓库 |
| CF-22 | 差量更新 | D | 当前完整签名包；作为后续优化，不影响更新功能 |
| CF-23 | macOS安装包/匿名签名更新验证 | I | alpha.15公开发布，安装态验收单独记录 |
| CF-24 | Linux GTK安装脚本/桌面项/deb | M | 先建构建矩阵，真实Linux桌面验收 |
| CF-25 | Windows GUI包与原生集成 | M | 交叉编译接口不算Windows桌面验收 |
| CF-26 | 从 VS Code/Cursor 等导入设置与快捷键并立即生效（砚自有，用户需求） | V | 按平台查找各编辑器 User 目录或手动指定文件；逐键映射到 config.toml（保留注释），清除 state.json 中会覆盖的同名项，快捷键追加在用户自己的之后，未对应的键列出。`core/config/vscode_test.go`、`core/app/importsettings_test.go`；字体族/行高实时生效 `ui/gui/gitignore_test.go` |
| CF-27 | 功能导览（砚自有，对标 JetBrains「Learn IDE Features」） | V | 侧边栏「学习」视图：7 课按模块列出，课内练习文件放在配置目录 `learn/`，步骤显示用户键位里的实际快捷键；命令真正运行（键、菜单、命令面板、守卫与参数绑定）或编辑器达到条件时自动打勾；可跳过、进度存 state.json；提示轮换；终端版跳过窗口专属步骤；欢迎页与帮助菜单入口。`core/app/guide_test.go`（含所有课程命令存在性）、`ui/gui/learn_test.go` 截图 |

## SETTINGS.md 配置键逐项映射

保留砚的 TOML/项目覆盖；相同能力不要求沿用 JSON 名字。

| 上游键 | 台账/砚入口或缺口 |
| --- | --- |
| `fontFamily` | CF-05 / ui.font_family |
| `fontSize` | CF-05 / ui.font_size |
| `fontWeight` | CF-06 / 缺 |
| `fontFeatures` | CF-06 / 缺 |
| `lineHeight` | CF-05 / ui.line_height |
| `terminalFontFamily` | CF-06 / 缺 |
| `terminalFontSize` | CF-06 / 缺 |
| `terminalCopyOnSelect` | LY-13 / terminal.copy_on_select |
| `theme` | CF-01～03 / ui.theme |
| `appearance` | CF-01 / ui.appearance |
| `sidebarRight` | WN-14 / explorer.right |
| `panelPosition` | LY-09 / 缺 |
| `projectTabs` | WN-05 / state |
| `projectTabStyle` | WN-09～12 / 缺 |
| `locale` | CF-07 / ui.language；设置面板选择优先，启动时生效 |
| `autoSave` | ED-07 / editor.auto_save |
| `wordWrap` | ED-06 / editor.word_wrap |
| `vim` | VI-01 / editor.vim |
| `vimCtrlKeys` | VI-14 / 缺 |
| `blame` | GI-04 / git.blame |
| `diffWhitespace` | UP-11 / diff.whitespace（菜单选择存 state） |
| `diffCrossFiles` | UP-07 / diff.cross_files |
| `diffLayout` | DF-09、UP-10 / diff.layout（菜单选择存 state） |
| `diffHighlight` | UP-10 / diff.highlight |
| `diffContext` | UP-10 / diff.context（0～10） |
| `diffSyncScroll` / `diffAlignedRows` | UP-10 / diff.sync_scroll、diff.aligned_rows（菜单选择存 state） |
| `scmTree` | GI-12 / 运行时开关，配置持久化待补 |
| `scmAllBranches` | GI-16 / state.AllBranches |
| `syncIgnored` | CF-16 / 外部依赖 |
| `aiModel` | CF-19 / 外部依赖 |
| `scmRepositoryOpen` / `scmRemotes`（源码保留的面板状态） | GI-26 / state.repositoryPanel、repositoryRemotes |

## 分批执行顺序

当前执行：UP-56 超长单行文件保护；UP-62 已随 alpha.34 发布，UP-63/64 与界面语言已随 alpha.33 发布。本批选择文件打开可靠性，避免与 Git 子模块、依赖升级混合。下一批优先评估子模块 SCM（UP-50），再核验长行三方合并（UP-49）；CLI（UP-48）、Vim 与依赖升级（UP-66）独立推进。以下 B1～B9 保留模块范围，外部服务与原生平台仍需各自验收。

1. **B1 Markdown 阅读闭环（本轮）**：MD-07～16。右侧预览、提示块、代码复制、锚点、表格、选择复制、阅读统计、会话与滚动；验证重复调用、中文、源码不被预览编辑、重启恢复。
2. **B2 文件与键盘可靠性（保存冲突、文件/搜索新鲜度、配置与键位已修；SE-05 性能与 CF-13 终端前置拦截仍开放）**：ED-02/17、SE-04/08/11/12、CF-08～12，先独立复现外部改动/异步回调/快捷键范围，再补遗漏；验收不能覆盖或丢失脏文档。
3. **B3 布局与项目轨道（锁列 LY-04～07、ED-05、LY-09、EX-11～12、WN-09～12、ED-21 已做，B3 完成）**：LY-04～07/09、EX-11～12、ED-05/21、WN-09～12；包含锁列完整路由矩阵、侧边面板和会话。
4. **B4 Git 管理完整度**：GI-07/09/18/26～27、DF-02～04/06～08/16、MR-08～09；真实多仓库测试，拒绝对错误仓库执行操作。
5. **B5 Vim 兼容**：VI 开放项。先对上游逐键兼容样例建差异集，再按寄存器→宏→标记/搜索→Ex 原子编辑顺序接入；保留跨平台 Ctrl 策略。
6. **B6 字体与国际化**：CF-06～07，翻译键与命令搜索统一；不重写底层文本/IME。
7. **B7 SSH**：SS-01～19（上游 internal/remote/{conn,fs,transfer,forward}.go 与 internal/vfs/vfs.go），先做本地/远程文件系统和进程适配边界，再真实测试连接、冲突保存、断连、传输和端口；没有可用测试主机时标记验收阻塞。
8. **B8 账户同步/AI**：CF-14～19/GI-11，上游 internal/cloudsync/cloudsync.go 与 internal/mon/client.go 绑定 Mon 应用标识；需要用户确定自有服务或明确提供方及可用账户；可先做提供方接口和本地冲突算法，不能用假登录/假同步宣称完成。
9. **B9 原生平台发布**：CF-23～24；保留平台接口，分别做真实系统验收，再发布对应安装包。

## 统计与证据

本次登记 276 项：D=3，I=111，M=31，P=34，V=87，X=10（增量登记至 UP-68，另有 ED-22）。此数字是契约登记数，不是功能完成百分比。

当前基线多仓库证据见 [multi-repository-scm.md](multi-repository-scm.md)。本轮 MD-07～16 已通过核心与 GUI 点击/选择/复制/双向滚动测试、截图检查、完整 make check（含 race/架构/体积）及 Windows amd64 核心/GUI 测试交叉编译。原生 Windows/Linux 桌面和真实人工阅读验收仍未执行。

新增测试：`core/app/preview_test.go`、`ui/gui/markdown_workflow_test.go` 与更新的 `ui/gui/markdown_test.go`。ED-02/17、SE-12 的新增证据为 `core/text/file_test.go`、`core/edit/edit_test.go`、`core/app/safety_test.go`；完整 `make check`（含race/架构/体积）及 Windows amd64 core/app 测试交叉编译通过；发布证据见对应版本的交付记录。

B2 第二轮（alpha.16 之后，分支 `codex/freshness-keybindings`）：2026-10-19 复核上游 `origin/main` 仍为 `46990a1`，无新提交、无新增契约。EX-02、SE-08、CF-08、CF-10～12 转 V；证据为 `core/workspace/watch_test.go`、`core/app/freshness_test.go`、`core/app/keybindings_runtime_test.go` 及更新的 `keybindings_test.go`，完整 `make check`、Windows amd64 交叉编译。监听由 fsnotify（BSD-3）提供；Windows ReadDirectoryChangesW 与 Linux inotify 路径只经交叉编译，未做原生验收。

B3 第一批（分支 `codex/columns-panels`）：LY-04～07、ED-05 转 V，移植自上游 `cmd/mayoi/columnlock.go`、`columns.go` 的 `removeColumn` 与 `tabs.go` 的 `tabMiddle`；证据见对应行，截图在验证目录 `b3-column-lock/shots`。

文件保护采用保存前比较原始字节，并在准备完临时文件后验证。通用文件系统没有跨进程比较并替换保证，因此不能将它描述为任意非协作写入下的绝对原子保护。

## Contributor workflow

Read AGENTS.md, this ledger, docs/MAYOI.md and CONTRIBUTING.md before choosing an open batch. Verify exact upstream and Yan revisions before editing, and preserve unrelated work. Track each contract by ID, reproduce partial gaps before fixing them, keep the headless core and Windows/Linux seams, and distinguish local tests from native/cloud acceptance. Qoder/Qoduck remain excluded. Run make check before committing and follow the contribution and release workflow. Do not claim completion for external-service or native-platform work without the necessary evidence.

UP-62 本批接入本地 MathJax、用户/项目配置开关、公式源码复制与编辑；验收记录见 `docs/up62-math-acceptance.md`。原生 Windows/Linux 窗口尚未验收。
