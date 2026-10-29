# 写作依据、时间线与覆盖索引

这一页用于仓库维护和发表前核对，不是连载正文。文章按问题组织，下面的提交表用来保留完整先后顺序，避免把主题顺序误读成精确日程。

## 范围和证据层级

- 截止日期：2026-10-14。提交范围：`42baaae..063fc58`，含起点，共 29 个本地提交。
- 最近发布标签：`v0.1.0-alpha.6` → `35488c3`。标签存在由本地 Git 核对；远端发布与下载结果只引用已有报告，本次未联网重验。
- 工作区采样：`core/app/changes.go` 等「所有改动」视图尚未提交；仅在第 14 篇记开发方向。之后代码继续变化，不改变本页截止点。
- 源码说明当前实现；测试文件说明测试意图，不能单靠文件存在宣称执行通过；历史报告说明当时执行结果，不能替代当前重跑。
- 早期设计是历史动机与方案依据；发生冲突时，当前源码、具体版本说明和后续 ADR 优先于旧路线图。
- 没有补写无依据的逐日工时、功能上线时间、访谈反馈、真实用户规模或性能排行榜。

## 提交时间线

日期为 Git 作者时间转换到 `America/New_York`，不是文件修改日期，也不代表功能从该时刻才开始开发。ADR 的 2026-10-12 决策日期保留其原记录，不据此虚构当天提交。

| 日期 | 提交 | 记录内容 | 连载篇目 |
|---|---|---|---|
| 2026-10-09 | `42baaae` | 设计文档、v0.0 终端骨架、文本与编辑闭环 | 01–04 |
| 2026-10-09 | `a442fb4` | M3 工作区、文件树、索引与导航 | 05 |
| 2026-10-13 | `16bac25` | MyGo 窗口与 Mayoi 功能整合 | 06–08 |
| 2026-10-13 | `145e1cb` | macOS 包、声明、图标、race 门禁与发布脚本 | 09 |
| 2026-10-13 | `41b9c3b` | 预览发布文档 | 09 |
| 2026-10-13 | `1cdd394` | race 真实失败传播、工作负载与测试契约修正 | 09 |
| 2026-10-13 | `47e3f2d` | zip 去除扩展属性问题；alpha.1 标签指向此处 | 09 |
| 2026-10-13 | `5e3ea10` | alpha.1 发布报告 | 07、09 |
| 2026-10-13 | `1c5081c` | 状态栏与窗口标题显示版本；alpha.2 标签 | 09 |
| 2026-10-13 | `fa72502` | 触控板、worktree、编辑命令、提交图 | 08 |
| 2026-10-13 | `ca546d6` | 墨滴光标应用图标 | 08 |
| 2026-10-13 | `730d97c` | 提交历史与 worktree 审查修正 | 08 |
| 2026-10-13 | `a157da9` | alpha.3 说明与标签 | 08 |
| 2026-10-13 | `9ce9b65` | MyGo 升级到 0.4.8 | 06、10 |
| 2026-10-13 | `25b7e9c` | VS Code 格式 keybindings.json | 10 |
| 2026-10-13 | `4f9942d` | Markdown 预览 | 10 |
| 2026-10-13 | `258e1b7` | CHANGELOG、版本脚本、PR CI | 14 |
| 2026-10-14 | `b5b19ea` | 自动换行、新窗口 | 11 |
| 2026-10-14 | `401ef82` | merge、rebase、reset、revert、cherry-pick | 12 |
| 2026-10-14 | `3055fff` | 资源管理器复制粘贴、拖放、清空选择 | 12 |
| 2026-10-14 | `f6eef0e` | alpha.4 发布说明初稿 | 10–12 |
| 2026-10-14 | `1be5ee5` | 项目标签、聚合与拆分窗口、恢复 | 11 |
| 2026-10-14 | `5318fa1` | 点号目录里的新文件 diff | 12 |
| 2026-10-14 | `0cc0769` | alpha.4 发布提交与标签 | 10–12 |
| 2026-10-14 | `6521799` | 终端选中复制、项目排序/提醒、窄窗口 | 11、13 |
| 2026-10-14 | `832c33f` | alpha.5 发布提交与标签 | 13 |
| 2026-10-14 | `795823f` | 截图粘贴、全选复制、MyGo fork | 13 |
| 2026-10-14 | `35488c3` | alpha.6 发布提交与标签 | 13、14 |
| 2026-10-14 | `063fc58` | 行号栏改动展开、还原、暂存；未纳入 alpha.6 | 14 |

## 按主题检查是否遗漏

| 主题 | 已纳入的点 | 篇目与主要依据 |
|---|---|---|
| 立项与选型 | 代码/中文文档阅读、学习 Go、VS Code 交互；Gio/Wails/终端三路线；轻快目标与测量口径 | 01；[DESIGN](../DESIGN.md)、[DECISIONS](../DECISIONS.md) |
| 范围与路线 | M0–M8、原 v0.1/0.2/0.3 顺序、非目标、后续范围变化 | 01、06、14；ADR-001–018 |
| 文本存储 | M1a 平铺叶子、前缀摘要、2 KiB 块、不可变快照；B+ 树尚未实现 | 02；[flat.go](../../core/text/flat.go) |
| 编辑事务 | UTF-8 字节坐标、ChangeSet、逆序应用、位置映射、多光标、撤销分组 | 02、04、07；[core/edit](../../core/edit/) |
| 文件格式 | UTF-8/BOM、LF/CRLF、权限、原子保存、非 UTF-8 只读、8/256 MiB 分档 | 02；[file.go](../../core/text/file.go) |
| 架构 | 状态所有者、Snapshot/Post/Frame、语义输入、Host、两个前端隔离、依赖测试 | 03；[editor.go](../../core/app/editor.go)、[archtest](../../internal/archtest/archtest_test.go) |
| 终端输入 | tcell、Ctrl 回退、连按、焦点、shell 往返 | 03–04；[ui/tui](../../ui/tui/)、[core/keymap](../../core/keymap/) |
| 中文 | 字素/字节/宽度区别、候选光标、IME 提交/取消/撤销/换标签、真实输入法缺口 | 04；[GUI 测试](../../ui/gui/gui_test.go)、[预览报告](../macos-preview-release-report.md) |
| 工作区 | fastwalk、懒树、分桶 gitignore、20 万索引上限、单文件模式 | 05；[core/workspace](../../core/workspace/) |
| 导航 | fuzzy、最近文件、行号、命令前缀、全角兼容、取消旧任务、标签与分栏 | 05–06；[overlay.go](../../core/app/overlay.go)、[tabs.go](../../core/app/tabs.go) |
| 配置 | 默认/用户/项目、state.json、配置重读边界、会话与最近文件夹、开发隔离 | 05、10、14；[core/config](../../core/config/)、[Makefile](../../Makefile) |
| GUI 转向 | Gio → MyGo，Frame 语义化、纯 Go 构建与原生动态库区别 | 06；ADR-014 |
| 上游复用 | Mayoi 包移植与行为重写、归属声明、固定版本、fork 退出条件 | 06、09、13；[MAYOI](../MAYOI.md)、[go.mod](../../go.mod) |
| 高亮 | 手写方案 → Chroma、体积预算、符号、括号着色、增量正确性与完整重算 | 06、09；[highlight.go](../../core/syntax/highlight/highlight.go) |
| 编辑体验 | 闭合、缩进、行操作、注释、排序/驼峰、同词/括号、Vim、主题与字号 | 06–08；[editing.go](../../core/app/editing.go)、[vim.go](../../core/app/vim.go) |
| 搜索与安全 | 正则/全词/大小写/替换分组、include/exclude、脏缓冲区搜索、磁盘冲突、epoch、版本与原文校验 | 07；[safety_test.go](../../core/app/safety_test.go) |
| 基础 Git | CLI、状态、blame、行/词 diff、SCM、暂存提交、推拉、丢弃的不同语义 | 08；[core/git](../../core/git/)、[scm.go](../../core/app/scm.go) |
| Git 历史 | 提交图、分支标签、HEAD/合并/未推送、200 条限制、worktree 定向处理 | 08；[alpha.3](../releases/v0.1.0-alpha.3.md) |
| 发布安全 | R1–R7、race 不回退、正则超时、签名/公证区别、库与许可、哈希、下载重验、原生/无头分层 | 07、09；[预览报告](../macos-preview-release-report.md) |
| 版本与形象 | 图标、版本标签、关于窗口、预览/开发/稳定标识 | 08–09；[alpha.2](../releases/v0.1.0-alpha.2.md)、[version.go](../../internal/version/version.go) |
| 键位兼容 | JSONC、连按、移除绑定、when、runCommands、支持子集、平台键位和叫法 | 10；[keybindings.go](../../core/app/keybindings.go) |
| 预览 | 图片、SVG、Markdown 常见内容、异步版本、本地/网页链接、禁止隐藏编辑、无同步滚动 | 06、10；[ui/gui/markdownview.go](../../ui/gui/markdownview.go) |
| 换行与窗口 | 逻辑/视觉行、点击/滚动、剩余导航差异、多窗口、项目聚合/拆分/恢复、窄窗口优先级 | 11；[alpha.4](../releases/v0.1.0-alpha.4.md) |
| 文件交互 | 新建改名回收站、路径复制/显示、文件复制粘贴、拖放、空白焦点、空目录 | 06、12；[files.go](../../core/app/files.go)、[filecopy.go](../../core/app/filecopy.go) |
| 高级 Git | merge/rebase/reset/revert/cherry-pick、继续中止、Git 消息、点号路径修复 | 12；[gitops.go](../../core/app/gitops.go) |
| 终端细节 | 多终端、主题、路径跳转、选择复制、全选、热更新、图片协议、响铃/通知、项目排序 | 13；[terminal.go](../../ui/gui/terminal.go) |
| 工程与运维 | crash log、make dev、CHANGELOG、bump/release/publish、PR CI 的范围与取消 | 12、14；[CONTRIBUTING](../../CONTRIBUTING.md) |
| 截止点后续 | hunk peek/revert/stage 已提交未进 alpha.6；所有改动视图为工作区快照 | 14；`063fc58` 与当次工作区读取 |
| 未完成与验证缺口 | LSP、B+ 树、Windows/Linux、自动更新、公证、Vim 宏/Ex、选区查找、标签拖动、真实 IME 等 | 14；源码与各版本已知限制交叉核对 |

## 几处容易写错的历史差异

| 旧记录或容易产生的说法 | 本系列采用的口径 |
|---|---|
| GUI 仍然计划使用 Gio | 历史方案；已改用 MyGo |
| 手写词法器，终端门禁 6 MB | 当前使用 Chroma，Makefile 上限为 10,485,760 bytes |
| 用了 rope 就已具有完整 B+ 树性能 | 当前是 M1a，叶子引用数组与前缀摘要仍有重建开销 |
| 不用 cgo，因此完全没有原生库 | 常规构建无 cgo；GUI 终端运行时仍加载 libghostty-vt |
| 原子保存保证不会覆盖外部编辑 | 原子写入与保存前冲突检查分开，时间/大小标记也非文件事务锁 |
| GUI 组字测试通过，所有中文输入法通过 | 模拟事件验证与真实系统输入法矩阵分开 |
| codesign 成功，所以已通过受信任安装验证 | 报告是 ad-hoc，spctl 拒绝、无公证 |
| alpha.1 缺口里列的换行/历史/多窗口仍未做 | 后续版本已实现；alpha.1 报告保留当时语境 |
| alpha.5 已有全选复制和现存终端配置同步 | 这两点在 alpha.6 才补齐 |
| 行级改动属于 alpha.6 | 它在 alpha.6 之后的 `063fc58` |
| 旧版本性能就是今天的性能 | 仅引用明确版本历史样本；无当前性能横评 |
| 私有仓库有 release，所以人人可下载 | 不承诺公众下载；本轮没有查询远端可见性 |

## 本轮写作验证

本轮只新增博客稿与索引，并在文档导读和 CHANGELOG 增加入口。校验文章数量、相对链接、Markdown 代码围栏、时间线对截止提交的覆盖与文档空白错误；不运行应用、不发布、不提交，不把这次文档检查写成产品功能验收。
