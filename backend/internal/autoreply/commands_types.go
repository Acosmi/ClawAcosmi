package autoreply

// TS 对照: auto-reply/commands-registry.types.ts

// ---------- 命令类型定义 ----------

// CommandScope 命令作用域。
type CommandScope string

const (
	CommandScopeText   CommandScope = "text"
	CommandScopeNative CommandScope = "native"
	CommandScopeBoth   CommandScope = "both"
)

// CommandCategory 命令类别。
type CommandCategory string

const (
	CategorySession    CommandCategory = "session"
	CategoryOptions    CommandCategory = "options"
	CategoryStatus     CommandCategory = "status"
	CategoryManagement CommandCategory = "management"
	CategoryMedia      CommandCategory = "media"
	CategoryTools      CommandCategory = "tools"
	CategoryDocks      CommandCategory = "docks"
)

// CommandArgType 命令参数类型。
type CommandArgType string

const (
	ArgTypeString  CommandArgType = "string"
	ArgTypeNumber  CommandArgType = "number"
	ArgTypeBoolean CommandArgType = "boolean"
)

// CommandArgChoice 命令参数选项。
type CommandArgChoice struct {
	Value string
	Label string
}

// CommandArgChoiceContext 动态选项回调上下文。
// TS 对照: commands-registry.types.ts L16-22
type CommandArgChoiceContext struct {
	Provider string
	Model    string
}

// CommandArgChoicesProvider 动态选项回调。
// TS 对照: commands-registry.types.ts L26
type CommandArgChoicesProvider func(ctx CommandArgChoiceContext) []CommandArgChoice

// CommandArgDefinition 命令参数定义。
type CommandArgDefinition struct {
	Name             string
	Description      string
	Type             CommandArgType
	Required         bool
	Choices          []CommandArgChoice
	ChoicesProvider  CommandArgChoicesProvider // 动态回调（与 Choices 互斥）
	CaptureRemaining bool
}

// CommandArgMenuSpec 命令参数菜单规格。
type CommandArgMenuSpec struct {
	Arg   string
	Title string
}

// CommandArgValue 命令参数值。
type CommandArgValue = any

// CommandArgValues 命令参数值映射。
type CommandArgValues map[string]CommandArgValue

// CommandArgs 解析后的命令参数。
type CommandArgs struct {
	Raw    string
	Values CommandArgValues
}

// CommandArgsParsing 参数解析模式。
type CommandArgsParsing string

const (
	ArgsParsingNone       CommandArgsParsing = "none"
	ArgsParsingPositional CommandArgsParsing = "positional"
)

// CommandArgsFormatter 命令参数格式化器。
type CommandArgsFormatter func(values CommandArgValues) string

// ChatCommandDefinition 聊天命令定义。
type ChatCommandDefinition struct {
	Key          string
	NativeName   string
	Description  string
	TextAliases  []string
	AcceptsArgs  bool
	Args         []CommandArgDefinition
	ArgsParsing  CommandArgsParsing
	FormatArgs   CommandArgsFormatter
	ArgsMenu     *CommandArgMenuSpec
	ArgsMenuAuto bool
	Scope        CommandScope
	Category     CommandCategory
}

// NativeCommandSpec 原生命令规格（导出给频道）。
type NativeCommandSpec struct {
	Name        string
	Description string
	AcceptsArgs bool
	Args        []CommandArgDefinition
}

// CommandNormalizeOptions 命令规范化选项。
type CommandNormalizeOptions struct {
	BotUsername string
}

// CommandDetection 命令检测结构。
type CommandDetection struct {
	Exact map[string]struct{}
	// Regex 用于快速判断文本是否可能包含命令。
	// Go 中使用 strings 包函数替代正则以提升性能。
}

// ShouldHandleTextCommandsParams 判断是否处理文本命令的参数。
type ShouldHandleTextCommandsParams struct {
	Surface       string
	CommandSource string // "text" | "native"
}
