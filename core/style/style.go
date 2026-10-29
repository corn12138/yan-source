// Package style 定义语义样式与主题；颜色由前端按主题映射成终端颜色或像素颜色。
package style

// ID 是语义样式，按 iota 编号。内核只说「这是什么」，不说「什么颜色」。
type ID uint8

const (
	Text ID = iota
	Gutter
	GutterCurrent
	CurrentLine
	Selection
	Tab
	TabActive
	Status
	StatusAccent
	Dim
	Overlay
	OverlaySelected
	// 语法高亮（与 Mayoi 的 highlight.Kind 一一对应）
	Keyword
	Type
	Function
	String
	Number
	Comment
	Operator
	Punctuation
	Constant
	Builtin
	Tag
	Attribute
	Variable
	Heading
	Deleted
	Inserted
	// 改动视图
	DiffAdded       // 加上的行
	DiffRemoved     // 删掉的行
	DiffAddedWord   // 改过的行里新的词
	DiffRemovedWord // 改过的行里旧的词
	DiffHeader      // @@ 标题、折起来的行
	MergeConflict   // 三方合并里还没处理的冲突
	// 装饰
	SearchHit     // 查找的匹配
	SearchCurrent // 当前匹配
	Bracket       // 与光标处配对的括号
	Word          // 光标处单词的其他出现
	Composing     // 输入法正在组字的文字
	Match         // 模糊匹配命中的字符
	Border        // 浮层边框与分隔线
	TreeFocused   // 文件树中选中的项（侧边栏有焦点时）
	TreeCursor    // 文件树中选中的项（侧边栏无焦点时）
	Count         // 样式总数，主题表按它分配
)

// Span 是行内字节区间上的一个样式。
type Span struct {
	Start, End int32
	Style      ID
}

// Layer 说明一个样式管前景、背景还是两者：同一个字节上，前景取最上层管前景的样式，
// 背景取最上层管背景的样式——选中的关键字仍是关键字的颜色。
type Layer uint8

const (
	LayerFg Layer = 1 << iota
	LayerBg
	LayerBoth = LayerFg | LayerBg
)

// LayerOf 返回样式管哪一层。
func LayerOf(id ID) Layer {
	switch id {
	case CurrentLine, Selection, SearchHit, SearchCurrent, Bracket, Word, DiffAddedWord, DiffRemovedWord, DiffAdded, DiffRemoved, MergeConflict:
		return LayerBg // 改动的底色：并排时只铺半行，不盖语法颜色
	case Keyword, Type, Function, String, Number, Comment, Operator, Punctuation, Constant,
		Builtin, Tag, Attribute, Variable, Heading, Deleted, Inserted, Match, Dim, Composing:
		return LayerFg
	}
	return LayerBoth
}
