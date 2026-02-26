// theme.go — TUI 主题系统 + 代码高亮
//
// 对齐 TS: src/tui/theme/theme.ts(137L) + src/tui/theme/syntax-theme.ts(52L)
// 差异 TH-01/TH-02 (P1): 完整主题系统 + chroma v2 代码高亮。
//
// W5 产出文件 #3。
package tui

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/lipgloss"
)

// ---------- 调色板（对齐 TS palette）----------

// Palette TUI 调色板，对齐 TS theme.ts L12-33。
var Palette = struct {
	Text          lipgloss.Color
	Dim           lipgloss.Color
	Accent        lipgloss.Color
	AccentSoft    lipgloss.Color
	Border        lipgloss.Color
	UserBg        lipgloss.Color
	UserText      lipgloss.Color
	SystemText    lipgloss.Color
	ToolPendingBg lipgloss.Color
	ToolSuccessBg lipgloss.Color
	ToolErrorBg   lipgloss.Color
	ToolTitle     lipgloss.Color
	ToolOutput    lipgloss.Color
	Quote         lipgloss.Color
	QuoteBorder   lipgloss.Color
	Code          lipgloss.Color
	CodeBlock     lipgloss.Color
	CodeBorder    lipgloss.Color
	Link          lipgloss.Color
	Error         lipgloss.Color
	Success       lipgloss.Color
}{
	Text:          lipgloss.Color("#E8E3D5"),
	Dim:           lipgloss.Color("#7B7F87"),
	Accent:        lipgloss.Color("#F6C453"),
	AccentSoft:    lipgloss.Color("#F2A65A"),
	Border:        lipgloss.Color("#3C414B"),
	UserBg:        lipgloss.Color("#2B2F36"),
	UserText:      lipgloss.Color("#F3EEE0"),
	SystemText:    lipgloss.Color("#9BA3B2"),
	ToolPendingBg: lipgloss.Color("#1F2A2F"),
	ToolSuccessBg: lipgloss.Color("#1E2D23"),
	ToolErrorBg:   lipgloss.Color("#2F1F1F"),
	ToolTitle:     lipgloss.Color("#F6C453"),
	ToolOutput:    lipgloss.Color("#E1DACB"),
	Quote:         lipgloss.Color("#8CC8FF"),
	QuoteBorder:   lipgloss.Color("#3B4D6B"),
	Code:          lipgloss.Color("#F0C987"),
	CodeBlock:     lipgloss.Color("#1E232A"),
	CodeBorder:    lipgloss.Color("#343A45"),
	Link:          lipgloss.Color("#7DD3A5"),
	Error:         lipgloss.Color("#F97066"),
	Success:       lipgloss.Color("#7DD3A5"),
}

// ---------- 主题样式（对齐 TS theme 导出）----------

var (
	// ChatTheme 聊天区域样式。
	ChatTheme = struct {
		Text     lipgloss.Style
		Dim      lipgloss.Style
		Accent   lipgloss.Style
		AccentSf lipgloss.Style
		Success  lipgloss.Style
		Error    lipgloss.Style
		Header   lipgloss.Style
		System   lipgloss.Style
		UserBg   lipgloss.Style
		UserText lipgloss.Style
		Border   lipgloss.Style
		Bold     lipgloss.Style
		Italic   lipgloss.Style
	}{
		Text:     lipgloss.NewStyle().Foreground(Palette.Text),
		Dim:      lipgloss.NewStyle().Foreground(Palette.Dim),
		Accent:   lipgloss.NewStyle().Foreground(Palette.Accent),
		AccentSf: lipgloss.NewStyle().Foreground(Palette.AccentSoft),
		Success:  lipgloss.NewStyle().Foreground(Palette.Success),
		Error:    lipgloss.NewStyle().Foreground(Palette.Error),
		Header:   lipgloss.NewStyle().Foreground(Palette.Accent).Bold(true),
		System:   lipgloss.NewStyle().Foreground(Palette.SystemText),
		UserBg:   lipgloss.NewStyle().Background(Palette.UserBg),
		UserText: lipgloss.NewStyle().Foreground(Palette.UserText),
		Border:   lipgloss.NewStyle().Foreground(Palette.Border),
		Bold:     lipgloss.NewStyle().Bold(true),
		Italic:   lipgloss.NewStyle().Italic(true),
	}

	// ToolTheme 工具执行样式。
	ToolTheme = struct {
		Title     lipgloss.Style
		Output    lipgloss.Style
		PendingBg lipgloss.Style
		SuccessBg lipgloss.Style
		ErrorBg   lipgloss.Style
	}{
		Title:     lipgloss.NewStyle().Foreground(Palette.ToolTitle).Bold(true),
		Output:    lipgloss.NewStyle().Foreground(Palette.ToolOutput),
		PendingBg: lipgloss.NewStyle().Background(Palette.ToolPendingBg),
		SuccessBg: lipgloss.NewStyle().Background(Palette.ToolSuccessBg),
		ErrorBg:   lipgloss.NewStyle().Background(Palette.ToolErrorBg),
	}

	// OverlayTheme 选择列表/Overlay 样式。
	OverlayTheme = struct {
		SelectedPrefix lipgloss.Style
		SelectedText   lipgloss.Style
		Description    lipgloss.Style
		ScrollInfo     lipgloss.Style
		NoMatch        lipgloss.Style
		SearchPrompt   lipgloss.Style
		SearchInput    lipgloss.Style
		MatchHighlight lipgloss.Style
		FilterLabel    lipgloss.Style
	}{
		SelectedPrefix: lipgloss.NewStyle().Foreground(Palette.Accent),
		SelectedText:   lipgloss.NewStyle().Foreground(Palette.Accent).Bold(true),
		Description:    lipgloss.NewStyle().Foreground(Palette.Dim),
		ScrollInfo:     lipgloss.NewStyle().Foreground(Palette.Dim),
		NoMatch:        lipgloss.NewStyle().Foreground(Palette.Dim),
		SearchPrompt:   lipgloss.NewStyle().Foreground(Palette.AccentSoft),
		SearchInput:    lipgloss.NewStyle().Foreground(Palette.Text),
		MatchHighlight: lipgloss.NewStyle().Foreground(Palette.Accent).Bold(true),
		FilterLabel:    lipgloss.NewStyle().Foreground(Palette.Dim),
	}

	// StatusTheme 状态栏样式。
	StatusTheme = struct {
		Connected    lipgloss.Style
		Disconnected lipgloss.Style
		Reconnecting lipgloss.Style
	}{
		Connected:    lipgloss.NewStyle().Foreground(Palette.Success),
		Disconnected: lipgloss.NewStyle().Foreground(Palette.Error),
		Reconnecting: lipgloss.NewStyle().Foreground(Palette.AccentSoft),
	}

	// MarkdownTheme Markdown 渲染样式。
	MarkdownTheme = struct {
		Heading     lipgloss.Style
		Link        lipgloss.Style
		LinkURL     lipgloss.Style
		Code        lipgloss.Style
		CodeBlock   lipgloss.Style
		CodeBorder  lipgloss.Style
		Quote       lipgloss.Style
		QuoteBorder lipgloss.Style
		HR          lipgloss.Style
		ListBullet  lipgloss.Style
	}{
		Heading:     lipgloss.NewStyle().Foreground(Palette.Accent).Bold(true),
		Link:        lipgloss.NewStyle().Foreground(Palette.Link),
		LinkURL:     lipgloss.NewStyle().Foreground(Palette.Dim),
		Code:        lipgloss.NewStyle().Foreground(Palette.Code),
		CodeBlock:   lipgloss.NewStyle().Foreground(Palette.Code),
		CodeBorder:  lipgloss.NewStyle().Foreground(Palette.CodeBorder),
		Quote:       lipgloss.NewStyle().Foreground(Palette.Quote),
		QuoteBorder: lipgloss.NewStyle().Foreground(Palette.QuoteBorder),
		HR:          lipgloss.NewStyle().Foreground(Palette.Border),
		ListBullet:  lipgloss.NewStyle().Foreground(Palette.AccentSoft),
	}
)

// ---------- 辅助函数 ----------

// Dim 渲染暗色文本。
func Dim(s string) string {
	return ChatTheme.Dim.Render(s)
}

// Bold 渲染粗体文本。
func Bold(s string) string {
	return ChatTheme.Bold.Render(s)
}

// Italic 渲染斜体文本。
func Italic(s string) string {
	return ChatTheme.Italic.Render(s)
}

// ---------- 代码高亮（差异 TH-02：chroma v2）----------

// chromaStyle 是自定义 chroma Style，对齐 TS syntax-theme.ts。
// 使用 VS Code Dark+ 风格配色。
var chromaStyle = registerChromaStyle()

func registerChromaStyle() *chroma.Style {
	s := styles.Register(chroma.MustNewStyle("openacosmi-tui", chroma.StyleEntries{
		// 关键字
		chroma.Keyword:          "#C586C0",
		chroma.KeywordConstant:  "#569CD6",
		chroma.KeywordType:      "#4EC9B0",
		chroma.KeywordNamespace: "#C586C0",

		// 内置/类型
		chroma.NameBuiltin: "#4EC9B0",
		chroma.NameClass:   "#4EC9B0",

		// 函数
		chroma.NameFunction: "#DCDCAA",

		// 变量/参数
		chroma.NameVariable: "#9CDCFE",

		// 字面量
		chroma.LiteralString:       "#CE9178",
		chroma.LiteralStringEscape: "#D7BA7D",
		chroma.LiteralStringRegex:  "#D16969", // S2-8: 对齐 TS regexp
		chroma.LiteralStringSymbol: "#B5CEA8", // S2-8: 对齐 TS symbol
		chroma.LiteralNumber:       "#B5CEA8",
		chroma.LiteralOther:        "#C586C0", // S2-8: 对齐 TS formula

		// 注释
		chroma.Comment:        "#6A9955",
		chroma.CommentPreproc: "#C586C0",
		chroma.CommentSpecial: "#608B4E", // S2-8: 对齐 TS doctag

		// 操作符/标点
		chroma.Operator:    "#D4D4D4",
		chroma.Punctuation: "#D4D4D4",

		// 标签 (HTML/XML)
		chroma.NameTag:       "#569CD6",
		chroma.NameAttribute: "#9CDCFE",

		// 装饰器/元信息
		chroma.NameDecorator: "#9CDCFE", // S2-8: 对齐 TS meta

		// Diff
		chroma.GenericInserted: "#B5CEA8",
		chroma.GenericDeleted:  "#F44747",

		// 默认
		chroma.Background:        "#E8E3D5 bg:#1E232A",
		chroma.GenericEmph:       "italic",
		chroma.GenericStrong:     "bold",
		chroma.GenericSubheading: "#D7BA7D", // S2-8: 修正对齐 TS bullet/section
	}))
	return s
}

// HighlightCode 使用 chroma v2 高亮代码。
// 对齐 TS theme.ts L45-60 highlightCode()。
// 返回带 ANSI 转义码的高亮文本。
func HighlightCode(code, lang string) string {
	// 获取 lexer
	var lexer chroma.Lexer
	if lang != "" {
		lexer = lexers.Get(lang)
	}
	if lexer == nil {
		lexer = lexers.Analyse(code)
	}
	if lexer == nil {
		// 无法识别语言，返回带代码色的原文
		return lipgloss.NewStyle().Foreground(Palette.Code).Render(code)
	}
	lexer = chroma.Coalesce(lexer)

	// tokenize
	iterator, err := lexer.Tokenise(nil, code)
	if err != nil {
		return lipgloss.NewStyle().Foreground(Palette.Code).Render(code)
	}

	// 使用 terminal256 formatter
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	var buf strings.Builder
	if err := formatter.Format(&buf, chromaStyle, iterator); err != nil {
		return lipgloss.NewStyle().Foreground(Palette.Code).Render(code)
	}

	return strings.TrimRight(buf.String(), "\n")
}

// SupportsLanguage 检查 chroma 是否支持该语言。
// 对齐 TS theme.ts L49 supportsLanguage()。
func SupportsLanguage(lang string) bool {
	if lang == "" {
		return false
	}
	return lexers.Get(lang) != nil
}
