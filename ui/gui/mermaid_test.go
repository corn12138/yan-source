package gui

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/corn12138/yan-source/core/style"
)

func testMermaidStyle() mermaidStyle { return testMermaidStyleFor(true) }

func testMermaidStyleFor(dark bool) mermaidStyle {
	p := newPalette(style.ThemeFor("", dark))
	return mermaidStyle{
		bg: p.editor, fg: p.text, muted: p.muted, line: p.muted, surface: p.surface, surfaceAlt: p.surfaceHover,
		border: p.border, accent: p.accent, danger: p.danger, mono: "monospace", monoWeight: 400,
	}
}

func TestRenderMermaid(t *testing.T) {
	st := testMermaidStyle()
	for name, src := range map[string]string{
		"flowchart": "flowchart LR\nA[Start] --> B{Ok?}\nB -- yes --> C[Done]\nB -- no --> A",
		"sequence":  "sequenceDiagram\nAlice->>Bob: Hello\nBob-->>Alice: Hi",
		"class":     "classDiagram\nclass Animal{\n+String name\n}\nAnimal <|-- Duck",
		"gantt":     "gantt\ntitle Plan\ndateFormat YYYY-MM-DD\nsection A\nTask :a1, 2024-01-01, 30d",
		"pie":       "pie title Pets\n\"Dogs\" : 386\n\"Cats\" : 85",
		"git":       "gitGraph\ncommit\nbranch dev\ncommit\ncheckout main\nmerge dev",
	} {
		d := renderMermaid(src, st)
		if d.err != "" {
			t.Errorf("%s: %s", name, d.err)
			continue
		}
		if d.svg == nil || d.w <= 0 || d.h <= 0 || len(d.labels) == 0 {
			t.Errorf("%s: svg %v %vx%v, %d labels", name, d.svg != nil, d.w, d.h, len(d.labels))
		}
		for _, l := range d.labels {
			if l.x < -d.w || l.x > 2*d.w || l.y < -d.h || l.y > 2*d.h {
				t.Errorf("%s: a label at %v,%v outside %vx%v", name, l.x, l.y, d.w, d.h)
			}
		}
	}
}

func TestRenderMermaidError(t *testing.T) {
	d := renderMermaid("sequenceDiagram\nA->>B hi", testMermaidStyle())
	if d.err == "" || strings.Contains(d.err, "\n") || !strings.Contains(d.err, "line 2") {
		t.Errorf("err = %q", d.err)
	}
	if d := renderMermaid("not a diagram", testMermaidStyle()); d.err == "" {
		t.Error("no error for text that is no diagram")
	}
}

// A label on a fill that a style gave its shape keeps a readable color: the theme's text or
// background, whichever contrasts more, unless the style colored the text itself.
func TestMermaidStyledFillLabels(t *testing.T) {
	source := "flowchart LR\n subgraph S[\"sub\"]\n A[aa]\n end\n A --> B[bb] --> C[cc] --> D[dd]\n" +
		"style S fill:#ffe\n style B fill:#fdd,stroke:#c00\n style C fill:#fdd,color:#f00\n style D fill:#333\n"
	for _, dark := range []bool{true, false} {
		st := testMermaidStyleFor(dark)
		svg, err := peleRender(source, st)
		if err != nil {
			t.Fatal(err)
		}
		_, texts, err := mermaidSVG(svg, st)
		if err != nil {
			t.Fatal(err)
		}
		colors := map[string]ui.Color{}
		for _, x := range texts {
			for _, r := range x.runs {
				colors[r.text] = r.color
			}
		}
		want := map[string]ui.Color{"sub": st.bg, "aa": st.fg, "bb": st.bg, "cc": ui.Hex("#ff0000"), "dd": st.fg}
		if !dark { // the theme's own colors read on the light fills; only the dark fill needs the light background
			want = map[string]ui.Color{"sub": st.muted, "aa": st.fg, "bb": st.fg, "cc": ui.Hex("#ff0000"), "dd": st.bg}
		}
		for text, w := range want {
			if got := colors[text]; got != w {
				t.Errorf("dark=%v: %s in %+v, want %+v", dark, text, got, w)
			}
		}
	}
}

func TestMermaidSVG(t *testing.T) {
	st := testMermaidStyle()
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50" width="100" height="50" style="--_bg:var(--pele-bg,#fff);--_s:var(--pele-surface,#f3f3f3);--_a:var(--pele-nope,#808080);--_r:var(--pele-radius,4px);font:16px sans-serif" fill="var(--_fg,#123456)">` +
		`<g transform="translate(10,20)"><rect x="0" y="0" width="10" height="10" fill="var(--_s)" style="rx:var(--_r)"/>` +
		`<rect fill="color-mix(in srgb,var(--_a) 50%,#000000)"/>` +
		`<text x="5" y="6" text-anchor="middle">A &amp; B</text>` +
		`<text text-anchor="middle" xml:space="preserve"><tspan x="0" y="-4" font-weight="var(--_w)">one</tspan><tspan font-family="var(--_fm,monospace)"> two</tspan><tspan x="0" y="16" font-size="12">three</tspan></text></g>` +
		`<g transform="rotate(90 0 0)"><text x="1" y="2">turned</text></g></svg>`
	out, texts, err := mermaidSVG(svg, st)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{`fill="` + hexColor(st.surface) + `"`, ` rx="4"`, `fill="#404040"`, `fill="#123456"`} {
		if !strings.Contains(s, want) {
			t.Errorf("no %s in %s", want, s)
		}
	}
	for _, gone := range []string{"<text", "tspan", "var(", "--_"} {
		if strings.Contains(s, gone) {
			t.Errorf("%s left in %s", gone, s)
		}
	}
	if _, err := ui.ParseSVG(out); err != nil {
		t.Fatal(err)
	}
	if len(texts) != 4 {
		t.Fatalf("%d chunks: %+v", len(texts), texts)
	}
	if r := texts[0].runs; len(r) != 1 || r[0].text != "A & B" || r[0].size != 16 || texts[0].anchor != "middle" {
		t.Errorf("first chunk %+v", texts[0])
	}
	if x, y := texts[0].m.apply(texts[0].x, texts[0].y); x != 15 || y != 26 {
		t.Errorf("first chunk at %v,%v", x, y)
	}
	if r := texts[1].runs; len(r) != 2 || r[0].style != pelebold || r[1].style != peleMono || r[1].text != " two" || texts[1].y != -4 {
		t.Errorf("second chunk %+v", texts[1])
	}
	if r := texts[2].runs; len(r) != 1 || r[0].size != 12 || texts[2].y != 16 {
		t.Errorf("third chunk %+v", texts[2])
	}
	if l := texts[3].shape(st); len(l) != 1 || l[0].angle != 90 {
		t.Errorf("turned %+v", l)
	}
}

func TestCSSResolve(t *testing.T) {
	vars := map[string]string{"--a": "var(--b)", "--b": "#112233"}
	for in, want := range map[string]string{
		"var(--a)":                               "#112233",
		"var(--x,#fff)":                          "#fff",
		"var(--x, var(--a))":                     "#112233",
		"1px solid var(--b)":                     "1px solid #112233",
		"color-mix(in srgb,#ffffff 25%,#000000)": "#404040",
		"none":                                   "none",
	} {
		if got := cssResolve(in, vars); got != want {
			t.Errorf("cssResolve(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseTransform(t *testing.T) {
	m := parseTransform("translate(10, 5) scale(2)")
	if x, y := m.apply(1, 1); x != 12 || y != 7 {
		t.Errorf("translate scale: %v,%v", x, y)
	}
	m = parseTransform("rotate(90 10 10)")
	if x, y := m.apply(20, 10); math.Abs(float64(x-10)) > 1e-4 || math.Abs(float64(y-20)) > 1e-4 {
		t.Errorf("rotate: %v,%v", x, y)
	}
}

// userFlowchart is the diagram a user asked the preview to draw: Chinese labels, decisions,
// labelled edges, edges back to earlier nodes and a dotted labelled edge.
const userFlowchart = `flowchart TD
    A[Host 接收用户输入] --> B[输入规范化 命令 Skills 提交 hooks]
    B --> C{shouldQuery}
    C -->|否| R[生成本地结果]
    C -->|是| D[准备本轮参数并入会话队列]
    D --> E[Engine 取出 turn 并建立取消边界]
    E --> F[kL / pu / mu 查询循环]
    F --> G[历史转换 上下文准备 自动压缩]
    G --> H[System / Messages / Tools / Model 请求组装]
    H --> I[模型请求与响应流状态机]
    I --> J{完整内容块}
    J -->|文本| K[事件输出与会话记录]
    J -->|tool_use| L[工具队列 并发与独占判断]
    L --> M[路由 输入校验 hooks 权限]
    M --> N[实际工具调用和结果映射]
    N --> O[tool_result 排空并回填消息]
    O --> F
    I --> P{停止与恢复判断}
    P -->|可恢复| F
    P -->|候选结束| Q[Stop hooks 目标与上限判断]
    Q -->|反馈要求继续| F
    Q -->|终止| R
    R --> S[最终 result 用量 终止原因]
    S --> T[清理监听器和迭代器 等待下一输入]
    N -. Agent 工具 .-> U[独立 agent 身份 上下文 工具和任务登记]
    U --> F
`

// TestMermaidUserFlowchart: every label of the user's flowchart is drawn, by the window's
// text, inside the diagram, and its dotted edge stays dotted.
func TestMermaidUserFlowchart(t *testing.T) {
	st := testMermaidStyle()
	d := renderMermaid(userFlowchart, st)
	if d.err != "" {
		t.Fatal(d.err)
	}
	if d.w <= 0 || d.h <= d.w {
		t.Fatalf("a top-down flowchart of %vx%v", d.w, d.h)
	}
	var drawn strings.Builder
	for _, l := range d.labels {
		if l.x < 0 || l.y < 0 || l.x > d.w || l.y > d.h {
			t.Errorf("%q at %v,%v outside %vx%v", l.text, l.x, l.y, d.w, d.h)
		}
		drawn.WriteString(strings.ReplaceAll(l.text, " ", ""))
	}
	for _, label := range []string{"Host接收用户输入", "shouldQuery", "完整内容块", "停止与恢复判断", "是", "否", "文本", "tool_use",
		"可恢复", "候选结束", "反馈要求继续", "终止", "Agent工具", "独立agent身份上下文工具和任务登记", "清理监听器和迭代器等待下一输入"} {
		if !strings.Contains(drawn.String(), label) {
			t.Errorf("%s not drawn in %s", label, drawn.String())
		}
	}
	svg, err := peleRender(userFlowchart, st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(svg, "dasharray") {
		t.Error("the dotted edge is solid")
	}
}

// TestMermaidThemeColors: a diagram takes the colors of the theme it is drawn in, light or dark,
// with none of Pele's CSS variables left for the SVG renderer to miss.
func TestMermaidThemeColors(t *testing.T) {
	for _, dark := range []bool{false, true} {
		st := testMermaidStyleFor(dark)
		svg, err := peleRender(userFlowchart, st)
		if err != nil {
			t.Fatal(err)
		}
		shapes, texts, err := mermaidSVG(svg, st)
		if err != nil {
			t.Fatal(err)
		}
		out := string(shapes)
		if strings.Contains(out, "var(") || strings.Contains(out, "color-mix(") {
			t.Errorf("dark %v: unresolved CSS in %.300s", dark, out)
		}
		for _, c := range []ui.Color{st.surface, st.border} {
			if !strings.Contains(out, hexColor(c)) {
				t.Errorf("dark %v: no %s in the shapes", dark, hexColor(c))
			}
		}
		for _, tx := range texts {
			for _, r := range tx.runs {
				if r.color != st.fg && r.color != st.muted {
					t.Errorf("dark %v: label %q in %v, not the theme's text", dark, r.text, r.color)
				}
			}
		}
	}
}

// TestMermaidCachedPerStyle: the same source in another theme is drawn again, not reused.
func TestMermaidCachedPerStyle(t *testing.T) {
	src := "# A\n\n```mermaid\nflowchart LR\n  A --> B\n```\n"
	dark := testMermaidStyleFor(true)
	c := &markdownCache{mermaid: &dark}
	first := parseMarkdown([]byte(src), "a.md", "", nil, c)
	again := parseMarkdown([]byte(src), "a.md", "", nil, c)
	if first[1].diagram == nil || first[1].diagram != again[1].diagram {
		t.Fatal("an unchanged diagram was drawn again")
	}
	if blocks := parseMarkdown([]byte(src), "a.md", "", nil); blocks[1].diagram != nil {
		t.Fatal("drawn without a style")
	}
	if blocks := parseMarkdown([]byte("flowchart LR\n  A --> B\n"), filepath.Join("x", "flow.MMD"), "", nil, c); len(blocks) != 1 || blocks[0].diagram == nil || blocks[0].diagram.err != "" {
		t.Fatalf("a .mmd file: %+v", blocks)
	}
}

func waitDiagrams(t *testing.T, h *harness, path string, n int) []markdownBlock {
	t.Helper()
	var got []markdownBlock
	for end := time.Now().Add(20 * time.Second); time.Now().Before(end); {
		h.settle()
		got = got[:0]
		if m := h.f.markdowns[path]; m != nil && m.loaded && !m.running {
			for _, b := range m.blocks {
				if b.diagram != nil {
					got = append(got, b)
				}
			}
			if len(got) == n {
				return got
			}
		}
	}
	t.Fatalf("%d diagrams drawn, want %d", len(got), n)
	return nil
}

// TestMermaidPreviewWindow: the window's Markdown preview draws the user's flowchart in light
// and dark, says why a broken diagram cannot be drawn, shows a diagram's source and copies its
// code from the buttons on hover, and shows only code with [ui] preview_mermaid off.
func TestMermaidPreviewWindow(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	source := "# 查询循环\n\n```mermaid\n" + userFlowchart + "```\n\n```mermaid\nsequenceDiagram\n  A->>B hi\n```\n"
	for _, appearance := range []string{"light", "dark"} {
		h := newHarness(t, map[string]string{"flow.md": source}, Options{Appearance: appearance})
		h.tt.SetSize(1200, 1900)
		h.open("flow.md")
		h.ed.Run("yan.svg.togglePreview")
		path := filepath.Join(h.root, "flow.md")
		diagrams := waitDiagrams(t, h, path, 2)
		if diagrams[0].diagram.err != "" || diagrams[1].diagram.err == "" {
			t.Fatalf("%s: errors %q, %q", appearance, diagrams[0].diagram.err, diagrams[1].diagram.err)
		}
		h.settle()
		box, ok := h.tt.Find("Mermaid 图")
		if !ok || box.W < 300 || box.H < box.W {
			t.Fatalf("%s: the flowchart is drawn at %+v", appearance, box)
		}
		if h.tt.HasText("flowchart TD") || h.tt.HasText("Host 接收用户输入") {
			t.Fatalf("%s: the flowchart shows as code", appearance)
		}
		if !h.tt.HasText("画不出这张 Mermaid 图：") || !h.tt.HasText("A->>B hi") {
			t.Fatalf("%s: the broken diagram neither says why nor shows its code", appearance)
		}
		h.shot("mermaid-flowchart-" + appearance)
		if appearance == "dark" {
			continue
		}

		h.tt.Move(box.X+box.W/2, box.Y+box.H/2)
		h.settle()
		if err := h.tt.Click("复制代码"); err != nil {
			t.Fatal(err)
		}
		h.settle()
		if got := h.tt.Clipboard(); got != userFlowchart {
			t.Fatalf("copied %q", got)
		}
		h.tt.Move(box.X+box.W/2, box.Y+box.H/2)
		h.settle()
		if err := h.tt.Click("显示源码"); err != nil {
			t.Fatal(err)
		}
		h.settle()
		if !h.tt.HasText("flowchart TD") {
			t.Fatal("Show Source left the diagram")
		}
		if err := h.tt.Click("显示图"); err != nil {
			t.Fatal(err)
		}
		h.settle()
		if h.tt.HasText("flowchart TD") {
			t.Fatal("Show Diagram left the source")
		}

		// The project's .yan.toml reaches the editor through the folder's watcher.
		if err := os.WriteFile(filepath.Join(h.root, ".yan.toml"), []byte("[ui]\npreview_mermaid = false\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		for end := time.Now().Add(30 * time.Second); !h.tt.HasText("flowchart TD"); {
			if time.Now().After(end) {
				t.Fatal("preview_mermaid = false still draws the diagram")
			}
			h.settle()
		}
	}
}

// TestMermaidFilePreview: a .mmd file previews as its diagram, drawn again as it is edited.
func TestMermaidFilePreview(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	h := newHarness(t, map[string]string{"flow.mmd": "stateDiagram-v2\n  [*] --> Idle\n  Idle --> Busy: start\n  Busy --> [*]\n"}, Options{})
	h.open("flow.mmd")
	h.ed.Run("yan.svg.togglePreview")
	path := filepath.Join(h.root, "flow.mmd")
	first := waitDiagrams(t, h, path, 1)[0].diagram
	if first.err != "" {
		t.Fatal(first.err)
	}
	h.shot("mermaid-file")
	h.ed.Run("yan.svg.togglePreview")
	h.settle()
	h.ed.Run("cursorBottom") // the platform's key for it differs: Cmd+Down on macOS, Ctrl+End elsewhere
	h.tt.Type("  Idle --> Off: stop\n")
	h.ed.Run("yan.svg.togglePreview")
	for end := time.Now().Add(20 * time.Second); ; {
		h.settle()
		if b := h.f.markdowns[path].blocks; len(b) == 1 && b[0].diagram != nil && b[0].diagram != first {
			if b[0].diagram.err != "" || !strings.Contains(b[0].codeText, "Off") {
				t.Fatalf("drawn again: %q from %q", b[0].diagram.err, b[0].codeText)
			}
			break
		}
		if time.Now().After(end) {
			t.Fatal("the edited diagram was not drawn again")
		}
	}
}
