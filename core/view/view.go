// Package view 是内核与前端之间的接缝：不含任何单元格或像素假设。
//
// 内核只问前端两件事：一行字怎么排（LineLayouter），以及每个代码窗格有多大（Editor.SetPaneSize）；
// 然后把这一帧该显示什么装进 Frame。Frame 分两部分：
//   - 外壳（侧边栏、标签条、状态栏、浮层）只给内容，不给坐标：终端和图形前端各自排版、各自做命中测试，
//     点击后把「点了第几个标签」这样的语义动作交回内核；
//   - 代码窗格给到行：每行的文字、样式区间、光标，坐标是窗格内的 Unit。
//
// TUI 的 Unit 是单元格；GUI 的 Unit 也是「格」：一格宽是代码字体里 0 的宽度，一行高是行高。
package view

import "github.com/corn12138/yan-source/core/style"

type (
	Unit     int32
	Size     struct{ W, H Unit }
	Rect     struct{ X, Y, W, H Unit }
	PaneKind uint8
)

const (
	PaneEmpty    PaneKind = iota // 列里没有标签：显示欢迎页
	PaneCode                     // 文本编辑器
	PaneMessage                  // 打不开的文件（二进制、图片在终端里）：只显示一条说明
	PaneImage                    // 图片（窗口版画出来；终端里只显示一条说明）
	PaneMarkdown                 // Markdown 预览（只在窗口版）：Markdown 是文字，前端自己解析、排版
	PaneTerminal                 // 终端标签（只在窗口版）：Terminal 是它的号，Path 是它开在哪个文件夹
)

// Contains 报告点 (x, y) 是否落在矩形内。
func (r Rect) Contains(x, y Unit) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// LayoutStyle 是排版参数。
type LayoutStyle struct {
	TabWidth int
	Wrap     Unit // 一个视觉行最多几格；0 = 不折行
}

type (
	// Row 是一个视觉行：逻辑行内的字节区间。不折行时一行只有一个 Row。
	// Stop 是它在 LineLayout.Stops 里的第一个下标：每个视觉行的 Stop 从这一行的开头算 x（折下来的行从 0 格画起），
	// 最后一个落在这一行的末尾，所以折行处的字节既是上一行的末尾，也是下一行的开头。
	Row struct {
		Start, End int32
		X0, Height Unit
		Stop       int32
	}
	// Stop 是一个字素边界与它的 x 坐标；每个视觉行最后一个 Stop 位于这一行的末尾。
	Stop struct {
		Byte int32
		X    Unit
	}
	LineLayout struct {
		Rows  []Row
		Stops []Stop
	}
)

// LineLayouter 由各前端实现；实现必须复用 dst 中的切片，避免每帧分配。
type LineLayouter interface {
	Layout(dst *LineLayout, line []byte, st LayoutStyle)
	Epoch() uint64 // 字体或宽度设置变化时递增，让缓存失效
}

// RowOf 返回字节偏移 b 所在的视觉行：折行处的字节算下一行的开头。
func (l *LineLayout) RowOf(b int) int {
	r := 0
	for i := 1; i < len(l.Rows); i++ {
		if int(l.Rows[i].Start) > b {
			break
		}
		r = i
	}
	return r
}

// rowStops 返回第 r 个视觉行的 Stop。
func (l *LineLayout) rowStops(r int) []Stop {
	if len(l.Rows) == 0 {
		return l.Stops
	}
	r = max(min(r, len(l.Rows)-1), 0)
	end := len(l.Stops)
	if r+1 < len(l.Rows) {
		end = int(l.Rows[r+1].Stop)
	}
	return l.Stops[l.Rows[r].Stop:end]
}

// XAt 返回字节偏移 b 在它那个视觉行里的 x（b 不在边界上时取它之前最近的边界）。
func (l *LineLayout) XAt(b int) Unit {
	x := Unit(0)
	for _, s := range l.rowStops(l.RowOf(b)) {
		if int(s.Byte) > b {
			break
		}
		x = s.X
	}
	return x
}

// ByteAt 返回第一个视觉行里最接近 x 的字素边界。
func (l *LineLayout) ByteAt(x Unit) int { return l.RowByteAt(0, x) }

// RowByteAt 返回第 r 个视觉行里最接近 x 的字素边界（点在字的右半边时算到下一个边界）。
// 折下来的行的末尾就是下一行的开头，所以不是最后一行时，最远只到这一行最后一个字之前。
func (l *LineLayout) RowByteAt(r int, x Unit) int {
	stops := l.rowStops(r)
	if len(stops) == 0 {
		return 0
	}
	if r < len(l.Rows)-1 && len(stops) > 1 {
		stops = stops[:len(stops)-1]
	}
	for i := 1; i < len(stops); i++ {
		prev, cur := stops[i-1], stops[i]
		if x < cur.X {
			if x-prev.X < cur.X-x {
				return int(prev.Byte)
			}
			return int(cur.Byte)
		}
	}
	return int(stops[len(stops)-1].Byte)
}

// RowFrame 是一个要画的视觉行。
type RowFrame struct {
	Line     int32  // 逻辑行号
	Text     []byte // 只读
	Spans    []style.Span
	Base     style.ID  // 整行底色
	Guides   []Guide   // 缩进参考线
	Num      int32     // 行号栏显示的数：0 表示 Line+1，-1 表示不显示（改动视图用）
	Mark     GitMark   // 行号旁的 git 改动标记
	ReviewID int       // Saved review comment on this source line or diff card.
	Ghost    string    // 行尾的灰字（光标所在行的 blame）
	Block    DiffBlock // 改动视图里一处改动的标题行能做的块操作
	// Glow 是改动视图里刚被块操作写进去的行：前端让它亮一下再淡掉（PaneFrame.GlowGen 变时开始）。
	Glow bool
	// Filler 是并排两窗格对齐时，这一边没有、给对面那一行留的空行。
	Filler bool
}

// SplitDiff is a side-by-side diff the window draws as two panes with a gutter between them.
// Each side's rows include Over rows above the first one in view and as many below, so that
// going to a change can glide there.
type SplitDiff struct {
	Aligned, Synced bool // rows lined up across the sides; the sides scroll together
	Left, Right     SplitSide
	Bands           []SplitBand
	Glide           bool // the view moved to a change or a file: glide there rather than jump
}

// SplitSide is the rows of one side of a split diff. Lines are the diff's line of each row (for
// clicks); Current is the diff's line of the caret.
type SplitSide struct {
	Rows    []RowFrame
	Lines   []int32
	Over    int
	Gutter  Unit
	Current int32
}

// SplitBand is a change between the sides, as rows of each side's list (ends exclusive; from
// equal to to where a side has no lines of it, at the place they would be).
type SplitBand struct {
	LeftFrom, LeftTo, RightFrom, RightTo int
	Line                                 int32 // the diff's line of the change's header: its actions
	Arrow                                bool  // the change can be reverted, or copied across two files
	Compare                              bool  // a comparison of two files: the arrow copies left into right
}

// DiffBlock is what the header row of a change of a diff offers: nothing, revert (and append)
// for the work tree's changes, or taking either side for a comparison of two files.
type DiffBlock uint8

const (
	DiffBlockNone DiffBlock = iota
	DiffBlockRevert
	DiffBlockCompare
	DiffBlockMerge  // a side of a three-way merge: take or ignore this side of a conflict
	DiffBlockStaged // a staged change: unstage it
	DiffBlockFile   // a file's header in the tab of all the changes: open it, stage or unstage it
	DiffBlockFileStaged
	DiffBlockFileConflict
	DiffBlockFileDeleted   // a tracked file deleted in the work tree: restore it
	DiffBlockFileUntracked // a file Git does not track: remove it
	DiffBlockMergeApply    // a side's change no conflict holds, not in the result yet: apply it
)

// GitMark 是一行相对 HEAD 的改动。
type GitMark uint8

const (
	MarkNone         GitMark = iota
	MarkAdded                // 新增的行
	MarkModified             // 改过的行
	MarkDeleted              // 上面删掉了行
	MarkDeletedBelow         // 下面删掉了行（最后一行）
)

// Guide 是一条缩进参考线：在第 X 格（未减水平滚动）。Active 是光标所在的块的那一条，画亮一些。
type Guide struct {
	X      Unit
	Active bool
}

// Caret 是一个光标，坐标是窗格内的 Unit（x 从窗格左边算，含行号栏；y 是第几行）。
type Caret struct {
	X, Y, Height Unit
	Visible      bool
	Block        bool // vim 普通模式：光标是盖住一个字的块
}

// Peek 是一处改动的框。
type Peek struct {
	After        int // 画在第几行（Rows 的下标）下面
	Index, Count int // 第几处改动（从 0 数）、一共几处
	Removed      []string
	Added        int
	Title        string
}

// Overview 是窗格右边那一条：都用行号，Lines 是一共几行。
type Overview struct {
	Lines        int
	Top, Visible int // 眼前这一屏：从第几行起、几行
	Caret        int
	Changes      []OverviewRun
	Reviews      []int
	Matches      []int // 有查找匹配的行，按顺序（很多时每一小段只留一个）
}

// OverviewRun 是连着的几行同一种改动：[From, To)。
type OverviewRun struct {
	From, To int
	Mark     GitMark
}

// Hint 是欢迎页上的一条快捷键提示。
type Hint struct {
	Keys, Title string
	Command     string // what a click on it runs (the window's welcome page)
}

// PaneFrame 是一列里正在显示的内容。
type CommitFile struct {
	Path, Name        string
	Status            byte
	Depth             int
	Folder, Collapsed bool
}

type CommitFiles struct {
	Identity, Selected string
	Visible, Loading   bool
	Error              string
	Width              float64
	Count              int
	Rows               []CommitFile
}

type PaneFrame struct {
	CommitFiles *CommitFiles
	Kind        PaneKind
	Size        Size // 内核排版时用的尺寸（前端用 SetPaneSize 给出）
	Gutter      Unit // 行号栏宽度
	ScrollX     Unit // 水平滚动量（不折行时）
	TopLine     int  // Logical source line, independent of frontend pixel geometry.
	// Split is a side-by-side diff in two panes (the window only); nil for one pane.
	Split *SplitDiff
	// GlowGen changes when a diff's block action wrote lines that glow (RowFrame.Glow).
	GlowGen uint64
	// PreviewGen counts the times a Markdown preview was opened in place: on a new one the
	// preview scrolls to TopLine again.
	PreviewGen uint64
	LineCount  int
	Current    int32
	Rows       []RowFrame
	// Caret 是主光标：有键盘焦点的列里，它就是唯一的真实光标，也是输入法候选框的锚点。
	Caret     Caret
	Secondary []Caret // 多光标的其余光标
	Message   string  // PaneMessage 的说明
	Hints     []Hint  // PaneEmpty 的快捷键提示
	// Sticky 是钉在窗格顶上的行（光标所在的、或第一行所在的那些块的开头），盖在最上面几行上。
	Sticky []RowFrame
	// Peek 是行号旁一处改动下面开着的框（没开时为 nil）：画在 Rows[After] 下面（-1 表示那一行不在眼前），
	// 写着这几行在 HEAD 里是什么（Removed），改动有几行（Added）。框里的按钮跑 yan.hunk.* 命令。
	Peek *Peek
	// Overview 是窗格右边那一条（照着 Mayoi）：整个文件里改了的行、查找的匹配、光标所在的行、眼前这一屏，
	// 按占文件的比例画；点它跑 Editor.OverviewJump。只有一行、或不是文字时为 nil。
	Overview *Overview
	// 图片：文件路径；SVG 预览时是它的文字。Zoom 是放大倍数，0 表示适应窗格。
	Path string
	SVG  []byte
	Zoom float64
	// Markdown 预览：文档的文字和它的版本（版本没变就不必重新解析），Root 是开头为 / 的链接、图片相对的目录
	// （文件所在的 git 仓库，或者打开的文件夹，同 GitHub）。
	Markdown []byte
	Version  uint64
	Root     string
	// Terminal 是 PaneTerminal 里那个终端的号（前端按号开 shell、画它）。
	Terminal int
}

// Crumb 是面包屑里的一个块：点它跳到开始它的那一行。
type Crumb struct {
	Label string
	Line  int
}

// Tab 是标签条上的一个标签。
type Tab struct {
	Name     string // 文件名
	Path     string // 绝对路径
	Dirty    bool
	Preview  bool // 预览标签：下一个预览的文件会顶替它（名字用斜体）
	ReadOnly bool
	History  bool // Virtual commit/comparison identity is not a filesystem location.
	Diff     bool // 改动视图：面包屑末尾有换排法的按钮
	Terminal int  // 终端标签的号；文件的标签是 0
}

// Column 是编辑区的一列：自己的标签条和正在显示的窗格。
type Column struct {
	Tabs   []Tab
	Active int     // 正在显示的标签；-1 表示没有
	Weight float64 // 占编辑区宽度的份数（和别的列比）
	Locked bool    // 隐式打开的文件、终端不进这一列
	// Closable: an empty column that can go (it is not the last column for files). Only one
	// empty column shows the welcome page; the others carry no Hints.
	Closable bool
	// Hidden: the editor area is maximized and this column is not part of it. Its tabs,
	// terminals included, carets and scroll stay as they are; only the frontend skips it.
	Hidden bool
	// Merge: the column shows a side or the result of a three-way merge.
	Merge  bool
	Pane   PaneFrame
	Crumbs []Crumb // 光标所在的块（最里面两个），接在文件路径后面
}

// TreeRow 是资源管理器里的一行。
type TreeRow struct {
	Name     string
	Depth    int
	Dir      bool
	Expanded bool
	Ignored  bool // 被 .gitignore 忽略：画暗一些
	Git      byte // git 的字母：M 修改、A 新增、D 删除、U 未跟踪、C 冲突；目录是下面最要紧的那个
	Modified bool // 文件开着且有未保存的修改
}

// SideView 是侧边栏显示的视图。
type SideView uint8

const (
	SideExplorer SideView = iota
	SideSearch
	SideSCM
	SideLearn
)

// Sidebar 是侧边栏：资源管理器、搜索或源代码管理。资源管理器的 Rows 是全部可见的行；
// Top 是键盘导航时内核保证选中行可见的首行（按 SetSidebarRows 给的行数算），前端可以用它，也可以自己滚动。
type Sidebar struct {
	Right    bool
	Visible  bool
	View     SideView
	Title    string // 打开的文件夹名
	Busy     bool   // 正在建索引
	Rows     []TreeRow
	Selected int
	Top      int
	Changes  int // git 改动的文件数（源代码管理按钮上的数字）
	SCM      SCM
	Search   Search
	Learn    Learn
}

// Learn is the feature guide: its lessons, the steps of the one running, and a tip.
type Learn struct {
	Lessons  []LearnLesson
	Selected int // the lesson chosen in the list
	Active   int // the lesson running, -1 for none
	Done     int // how many lessons are finished
	Steps    []LearnStep
	Tip      string
	// Home is the name of the folder a practice folder replaced in this window: the guide
	// offers to go back to it. Empty when the window is not in a practice folder.
	Home string
}

// LearnLesson is a lesson in the guide's list.
type LearnLesson struct {
	Module, Title string
	Done          bool
	Steps         int
	// Unreleased: the lesson, or one of its steps, teaches a feature the official release does
	// not have yet (see LearnStep.Unreleased).
	Unreleased bool
	Folder     bool // it opens a practice folder in place of the window's folder
}

// LearnStep is a step of the lesson running: what to do, the key for it in the user's keymap,
// and how far along it is (Count of Times for a step done several times).
type LearnStep struct {
	Text, Keys    string
	Done, Current bool
	WindowOnly    bool // only the window version can do it: the terminal edition skipped it
	Count, Times  int
	// Command is the command that does the step, Title its name in the command palette; the
	// frontend finds it in its own menus too. Empty for a step done by editing.
	Command, Title string
	// Since is the version that brought the step's feature ("unreleased" before its release);
	// Unreleased is set when the official release does not have it yet, so a badge says so.
	Since      string
	Unreleased bool
}

// GuideOffer is the window's one-time offer of the feature guide, on a first launch.
type GuideOffer struct {
	Visible bool
	Lessons int
}

// SearchField 是搜索视图里有键盘焦点的地方。
type SearchField uint8

const (
	SearchNone SearchField = iota
	SearchQuery
	SearchReplace
	SearchInclude
	SearchExclude
	SearchResults
)

// SearchRow 是搜索结果的一行：文件标题（Match 为假）或一处匹配。
type SearchRow struct {
	Match     bool
	Path      string // 文件标题：绝对路径、文件名、目录、几处匹配、是否折叠
	Name, Dir string
	Count     int
	Collapsed bool
	Line      int    // 匹配：第几行（从 0 数）
	Pre, Hit  string // 匹配前的文字、匹配本身（这一行里的部分）
	Post      string
	With      string // 替换时：换成什么
	Replacing bool
}

// Search 是在文件中搜索的视图。
type Search struct {
	Query, With, Include, Exclude string
	MatchCase, Word, Regex        bool
	Replacing, Details            bool // 显示替换框；显示「包含、排除」框
	Focus                         SearchField
	Running                       bool
	Summary                       string // 「3 个文件中有 12 个结果」
	Error                         bool
	Rows                          []SearchRow
	Selected, Top                 int
}

// SCMGroupKind 是源代码管理里的分组。
type SCMGroupKind uint8

const (
	GroupMerge SCMGroupKind = iota
	GroupStaged
	GroupChanges
	GroupUntracked
)

// SCMRow 是源代码管理里的一个文件。
type SCMRow struct {
	Path   string // 绝对路径
	Name   string
	Dir    string // 相对仓库根的目录
	Letter byte   // M、A、D、R、U（未跟踪）、C（冲突）
	Staged bool
	// 以树的形式查看时：Folder 是文件夹的一行（Name 是它的名字，只有一个子文件夹的连起来写，如 internal/git），
	// Depth 是缩进几级，Collapsed 是折起来了。
	Folder    bool
	Depth     int
	Collapsed bool
	Picked    bool // 多选里选中的文件
}

// SCMGroup 是一组文件。
type SCMGroup struct {
	Kind  SCMGroupKind
	Title string
	Rows  []SCMRow // none while Folded
	// Folded: the group is folded (SCMToggleSection); Count is its files all the same.
	Folded bool
	Count  int
}

// SCMPanelRow is a row of the repository panel under Source Control's history. Kind is
// "branch", "remote", "range" (incoming or outgoing commits), "url", "commit", "more",
// "status", "retry", "retryLog" or "addRemote".
type SCMPanelRow struct {
	Key, Kind, Label, Detail string
	Hash                     string // a commit's
	Depth                    int
	Open, Current, Remote    bool
}

// SCMRefKind 是提交上的引用是什么。
type SCMRefKind uint8

const (
	RefBranch SCMRefKind = iota // 本地分支
	RefRemote                   // 远程分支，如 origin/main
	RefTag
	RefHead // 分离的 HEAD
)

// SCMRef 是提交上的一个分支或标签；和本地分支同名、指着同一个提交的远程分支并进它的 Remotes。
type SCMRef struct {
	Name    string
	Kind    SCMRefKind
	Current bool     // 检出的分支
	Remotes []string // 如 ["origin"]
}

// SCMCommit 是提交历史的一行，带着它在提交图里的样子。
// 点在第 Lane 道，颜色序号 Color（style.Palette.Lane）。Above、Below 是这一行上沿、下沿每一道的颜色序号，
// -1 表示那一道空着。上沿的道 Joins 为真时通到点上，否则直通到中间；下沿的道 From 为真时从点上分出来，
// Through 为真时是上沿同一道直通下来的（两样都可能）；两样都不是的从中间直通到下沿。
type SCMCommit struct {
	File                               *CommitFile
	Expanded, FilesLoading             bool
	FilesError                         string
	Hash, Short, Subject, Author, When string
	Refs                               []SCMRef
	Head, Unpushed, Merge              bool
	Lane, Color                        int
	Above, Below                       []int
	Joins, From, Through               []bool
}

type SCMRepository struct {
	Path, Name string
}

// SCM 是源代码管理视图。
type SCMPanel struct {
	Name  string
	State SCM
}

type SCM struct {
	Root                      string
	RepositoryID              uint64
	AutoRepository, Collapsed bool
	Panels                    []SCMPanel

	Repositories      []SCMRepository
	RepositoryLoading bool
	RepositoryError   string
	NoRepo            bool // 文件夹不在 git 仓库里
	Groups            []SCMGroup
	Selected          int // 所有分组的行连起来数；无根仓库时是 Repositories 的下标
	Message           string
	Amend             bool
	MessageFocused    bool
	CommitLabel       string // 提交、全部提交、修改上次提交
	Branch            string
	Ahead, Behind     int
	Upstream          bool
	Busy              string
	// 提交历史，在改动下面；Selected 数到改动的行之后接着数提交。Lanes 是提交图最多有几道。
	Commits        []SCMCommit // none while CommitsFolded
	CommitsFolded  bool
	Lanes          int
	AllBranches    bool // 显示所有分支和标签，不只检出的分支
	HistoryLoading bool
	// Op 是遇到冲突停下来的合并、变基、拣选或还原（中文叫法），没有时为 ""：前端给「继续」「中止」。
	Op string
	// MoreCommits 是历史后面还有提交：前端在最后给「显示更多提交」（yan.scm.moreCommits）。
	MoreCommits bool
	// Panel 是仓库面板开着；PanelRemotes 是它显示远程（否则分支）；PanelRows 是它的行。
	Panel, PanelRemotes bool
	PanelRows           []SCMPanelRow
}

// OverlayKind 是浮层种类。
type OverlayKind uint8

const (
	OverlayFinder  OverlayKind = iota // 快速打开
	OverlayPalette                    // 命令面板
	OverlayBranch                     // 切换分支
	OverlayRecent                     // 最近打开的文件夹
	OverlayPrompt                     // 问一个名字（新建文件、改名）
	OverlayChoice                     // 从几项里选一项（代替确认对话框，两个前端都能画）
	OverlayTheme
)

// OverlayItem 是浮层里的一个候选项。
type OverlayItem struct {
	Label, Detail, Hint   string
	Font                  string // a font family: the window shows a sample in it
	Theme                 string
	LabelHits, DetailHits []int // 命中字符的字节下标
	// Unreleased marks a command the official release does not have yet (features.json).
	Unreleased bool
}

// Overlay 是一个浮层：标题、输入框、候选列表。前端画边框和输入框，内核只给内容。
type Overlay struct {
	Visible    bool
	Kind       OverlayKind
	Title      string
	Input      string
	Items      []OverlayItem
	Selected   int // Items 中的下标；-1 表示无
	Top        int // 键盘移动时保证选中项可见的首项（每屏 MaxOverlayItems 项）
	Footer     string
	Appearance string
}

// MaxOverlayItems 是浮层一屏显示的候选数。
const MaxOverlayItems = 12

// StatusItem 是状态栏右侧的一项。
type StatusItem struct {
	Text    string
	Command string
	// Keep 是放不下时它的去留：窄的窗口里先去掉 Keep 小的（编码最先，位置最后，照着 Mayoi v0.8.0）。
	Keep int
}

// 状态栏右边各项的去留，大的留得久。
const (
	KeepEncoding = iota + 1
	KeepEOL
	KeepIndent
	KeepLanguage
	KeepCursors
	KeepVim
	KeepPosition
)

// Status 是状态栏。
type Status struct {
	Message string // 临时消息（提示、错误）；非空时代替 Left 显示
	// Action 是跟着 Message 的一个按钮（如提交之后的「撤销」）：按下运行命令 Command；没有时为空。
	Action StatusAction
	Left   string // 当前文件的相对路径，或项目根
	Branch string // git 分支（领先 ↑、落后 ↓）；不在工作区里时是 ""
	// Sync 是分支旁的同步按钮（git.sync）：有上游时拉取后推送，没有时发布；Publish 是还没有上游。
	Sync, Publish bool
	Busy          string // 正在后台跑的 git 命令
	Right         []StatusItem
}

// StatusAction 是状态栏消息旁的按钮。
type StatusAction struct {
	RepositoryID uint64

	Label, Command string
}

// Focus 是键盘焦点所在的区域。
type Focus uint8

const (
	FocusEditor Focus = iota
	FocusSidebar
	FocusOverlay
	FocusFind // 查找框里的输入框（Find.Focus 说是哪一个）
)

// FindField 是查找框里的输入框。
type FindField uint8

const (
	FindNone FindField = iota
	FindQuery
	FindReplace
)

// Find 是查找框：浮在有焦点那一列的右上角。
type Find struct {
	Visible, Replacing     bool
	Query, With            string
	MatchCase, Word, Regex bool
	Count                  string // 「3 / 10」「无结果」
	Error                  bool   // Count 是错误（没找到、正则写错）
	Focus                  FindField
	SelectGen              uint64 // A new request selects the focused native input, not the document.
}

type ReviewDraft struct {
	ID                 int
	Ref, Text, Snippet string
	Saved              bool
}

// Frame 是一帧的完整显示模型，每帧复用。
type Frame struct {
	Review      *ReviewDraft
	ReviewCount int
	Version     uint64
	Title       string // 窗口标题
	Focus       Focus
	Sidebar     Sidebar
	// Maximized: only the columns not Hidden are drawn, without the sidebar or terminal panel.
	Maximized bool
	Columns   []Column
	Active    int // 有键盘焦点（或最后有）的列
	Status    Status
	Overlay   Overlay
	Find      Find
	Guide     GuideOffer
}
