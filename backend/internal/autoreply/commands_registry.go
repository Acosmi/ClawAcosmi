package autoreply

import (
	"regexp"
	"sort"
	"strings"
	"sync"
)

// TS 对照: auto-reply/commands-registry.ts (521L) — 简化版

// ---------- 全局命令注册表 ----------

var globalCommands = make(map[string]*ChatCommandDefinition)

// RegisterCommand 注册聊天命令。
func RegisterCommand(cmd *ChatCommandDefinition) {
	globalCommands[cmd.Key] = cmd
	InvalidateTextAliasCache()
}

// ListChatCommands 列出所有注册命令。
// TS 对照: commands-registry.ts L40-55
// skillCommands 可选参数用于注入技能命令。
func ListChatCommands(skillCommands ...[]*ChatCommandDefinition) []*ChatCommandDefinition {
	commands := make([]*ChatCommandDefinition, 0, len(globalCommands))
	for _, cmd := range globalCommands {
		commands = append(commands, cmd)
	}
	// 注入 skill 命令
	for _, skills := range skillCommands {
		commands = append(commands, skills...)
	}
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Key < commands[j].Key
	})
	return commands
}

// FindCommand 查找命令。
// TS 对照: commands-registry.ts L57-72
func FindCommand(key string) *ChatCommandDefinition {
	key = strings.ToLower(strings.TrimSpace(key))
	if cmd, ok := globalCommands[key]; ok {
		return cmd
	}
	// 搜索别名
	for _, cmd := range globalCommands {
		for _, alias := range cmd.TextAliases {
			if strings.ToLower(alias) == key {
				return cmd
			}
		}
	}
	return nil
}

// IsCommandEnabled 判断命令是否启用。
// TS 对照: commands-registry.ts L74-85
func IsCommandEnabled(key string) bool {
	return FindCommand(key) != nil
}

// NormalizeCommandBody 规范化命令文本。
// 去除 @botname 前缀并规范化空白。
// TS 对照: commands-registry.ts L87-123
func NormalizeCommandBody(body string, opts *CommandNormalizeOptions) string {
	if body == "" {
		return body
	}
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return trimmed
	}
	// 去除 @botname 前缀
	if opts != nil && opts.BotUsername != "" {
		botPrefix := "@" + opts.BotUsername
		lowerBody := strings.ToLower(trimmed)
		lowerPrefix := strings.ToLower(botPrefix)
		if strings.HasPrefix(lowerBody, lowerPrefix) {
			rest := trimmed[len(botPrefix):]
			if rest == "" || rest[0] == ' ' || rest[0] == '/' {
				trimmed = strings.TrimSpace(rest)
			}
		}
	}
	return trimmed
}

// IsCommandMessage 判断文本是否是命令消息。
// TS 对照: commands-registry.ts L125-145
func IsCommandMessage(body string, opts *CommandNormalizeOptions) bool {
	normalized := NormalizeCommandBody(body, opts)
	if normalized == "" {
		return false
	}
	if normalized[0] != '/' {
		return false
	}
	// 提取命令名 /command
	rest := normalized[1:]
	spaceIdx := strings.IndexByte(rest, ' ')
	cmdName := rest
	if spaceIdx >= 0 {
		cmdName = rest[:spaceIdx]
	}
	cmdName = strings.ToLower(cmdName)
	// 查找是否是已注册命令
	return FindCommand(cmdName) != nil
}

// TextCommandResult 文本命令解析结果。
type TextCommandResult struct {
	Command *ChatCommandDefinition
	Name    string
	Rest    string
}

// ResolveTextCommand 解析文本命令。
// TS 对照: commands-registry.ts L147-195
func ResolveTextCommand(body string, opts *CommandNormalizeOptions) *TextCommandResult {
	normalized := NormalizeCommandBody(body, opts)
	if normalized == "" || normalized[0] != '/' {
		return nil
	}
	rest := normalized[1:]
	spaceIdx := strings.IndexByte(rest, ' ')
	cmdName := rest
	cmdRest := ""
	if spaceIdx >= 0 {
		cmdName = rest[:spaceIdx]
		cmdRest = strings.TrimSpace(rest[spaceIdx+1:])
	}
	cmd := FindCommand(cmdName)
	if cmd == nil {
		return nil
	}
	return &TextCommandResult{
		Command: cmd,
		Name:    cmdName,
		Rest:    cmdRest,
	}
}

// ParseCommandArgs 解析命令参数。
// TS 对照: commands-registry.ts L197-250
func ParseCommandArgs(cmd *ChatCommandDefinition, raw string) CommandArgs {
	result := CommandArgs{
		Raw:    raw,
		Values: make(CommandArgValues),
	}
	if cmd == nil || !cmd.AcceptsArgs || raw == "" {
		return result
	}
	if cmd.ArgsParsing == ArgsParsingNone || len(cmd.Args) == 0 {
		// 无参数定义时，把整个 raw 存入第一个参数
		if len(cmd.Args) > 0 {
			result.Values[cmd.Args[0].Name] = raw
		}
		return result
	}
	// 位置解析
	tokens := strings.Fields(raw)
	argIdx := 0
	for i := 0; i < len(tokens) && argIdx < len(cmd.Args); i++ {
		argDef := cmd.Args[argIdx]
		if argDef.CaptureRemaining {
			result.Values[argDef.Name] = strings.Join(tokens[i:], " ")
			argIdx++
			break
		}
		result.Values[argDef.Name] = tokens[i]
		argIdx++
	}
	return result
}

// SerializeCommandArgs 序列化命令参数。
// TS 对照: commands-registry.ts L252-280
func SerializeCommandArgs(cmd *ChatCommandDefinition, values CommandArgValues) string {
	if cmd == nil || !cmd.AcceptsArgs {
		return ""
	}
	if cmd.FormatArgs != nil {
		return cmd.FormatArgs(values)
	}
	var parts []string
	for _, argDef := range cmd.Args {
		val := NormalizeArgValue(values[argDef.Name])
		if val != "" {
			parts = append(parts, val)
		}
	}
	return strings.Join(parts, " ")
}

// GetCommandDetection 构建命令检测器。
// TS 对照: commands-registry.ts L282-320
func GetCommandDetection() *CommandDetection {
	exact := make(map[string]struct{})
	for _, cmd := range globalCommands {
		exact["/"+strings.ToLower(cmd.Key)] = struct{}{}
		if cmd.NativeName != "" {
			exact["/"+strings.ToLower(cmd.NativeName)] = struct{}{}
		}
		for _, alias := range cmd.TextAliases {
			exact["/"+strings.ToLower(alias)] = struct{}{}
		}
	}
	return &CommandDetection{Exact: exact}
}

// ListNativeCommands 列出所有原生命令规格。
// TS 对照: commands-registry.ts L400-420
func ListNativeCommands() []NativeCommandSpec {
	var specs []NativeCommandSpec
	for _, cmd := range globalCommands {
		if cmd.Scope == CommandScopeText {
			continue
		}
		name := cmd.NativeName
		if name == "" {
			name = cmd.Key
		}
		specs = append(specs, NativeCommandSpec{
			Name:        name,
			Description: cmd.Description,
			AcceptsArgs: cmd.AcceptsArgs,
			Args:        cmd.Args,
		})
	}
	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Name < specs[j].Name
	})
	return specs
}

// ---------- 按配置过滤命令 ----------

// CommandsEnabledConfig 命令启用配置。
// TS 对照: OpenAcosmiConfig.commands = { config?: boolean, debug?: boolean, bash?: boolean }
// Go 端使用 map 替代，key 为命令 key，value 为是否启用。
type CommandsEnabledConfig struct {
	Config *bool // /config 命令 (默认禁用)
	Debug  *bool // /debug 命令 (默认禁用)
	Bash   *bool // /bash 命令 (默认禁用)
}

// IsCommandEnabledForConfig 判断命令在给定配置下是否启用。
// TS 对照: commands-registry.ts L100-111 isCommandEnabled
// config/debug/bash 命令默认禁用，需要显式开启。
func IsCommandEnabledForConfig(cfg *CommandsEnabledConfig, commandKey string) bool {
	if cfg == nil {
		// 无配置时，特殊命令默认禁用
		switch commandKey {
		case "config", "debug", "bash":
			return false
		default:
			return true
		}
	}
	switch commandKey {
	case "config":
		return cfg.Config != nil && *cfg.Config
	case "debug":
		return cfg.Debug != nil && *cfg.Debug
	case "bash":
		return cfg.Bash != nil && *cfg.Bash
	default:
		return true
	}
}

// ListChatCommandsForConfig 按配置过滤命令列表。
// TS 对照: commands-registry.ts L113-122 listChatCommandsForConfig
// skillCommands 可选参数用于注入技能命令。
func ListChatCommandsForConfig(cfg *CommandsEnabledConfig, skillCommands ...[]*ChatCommandDefinition) []*ChatCommandDefinition {
	all := ListChatCommands(skillCommands...)
	filtered := make([]*ChatCommandDefinition, 0, len(all))
	for _, cmd := range all {
		if IsCommandEnabledForConfig(cfg, cmd.Key) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

// ---------- 参数选项解析 ----------

// ResolvedCommandArgChoice 解析后的命令参数选项。
// TS 对照: commands-registry.ts L298
type ResolvedCommandArgChoice struct {
	Value string
	Label string
}

// ResolveCommandArgChoices 解析命令参数的选项列表。
// 支持静态 Choices 和动态 ChoicesProvider 回调。
// TS 对照: commands-registry.ts L300-328 resolveCommandArgChoices
func ResolveCommandArgChoices(arg *CommandArgDefinition, ctxOpts ...CommandArgChoiceContext) []ResolvedCommandArgChoice {
	if arg == nil {
		return nil
	}

	var raw []CommandArgChoice

	if arg.ChoicesProvider != nil {
		// 动态回调
		ctx := CommandArgChoiceContext{}
		if len(ctxOpts) > 0 {
			ctx = ctxOpts[0]
		}
		raw = arg.ChoicesProvider(ctx)
	} else if len(arg.Choices) > 0 {
		raw = arg.Choices
	}

	if len(raw) == 0 {
		return nil
	}

	result := make([]ResolvedCommandArgChoice, len(raw))
	for i, c := range raw {
		result[i] = ResolvedCommandArgChoice{
			Value: c.Value,
			Label: c.Label,
		}
	}
	return result
}

// ---------- Text Alias 缓存 ----------

// TextAliasSpec 文本别名规格。
// TS 对照: commands-registry.ts L33-37
type TextAliasSpec struct {
	Key         string // 命令 key
	Canonical   string // 规范别名 (如 "/help")
	AcceptsArgs bool
}

// textAliasCacheMu 保护 cachedTextAliasMap 和 textAliasMapOnce 的互斥锁
var textAliasCacheMu sync.Mutex

// cachedTextAliasMap 全局 text alias 缓存。
var cachedTextAliasMap map[string]*TextAliasSpec

// textAliasMapOnce 用于无竞争地初始化 cachedTextAliasMap。
// InvalidateTextAliasCache 会将其重置。
var textAliasMapOnce *sync.Once = &sync.Once{}

// buildTextAliasMap 构建 text alias map（必须在锁保护下调用）。
func buildTextAliasMap() map[string]*TextAliasSpec {
	aliasMap := make(map[string]*TextAliasSpec)
	for _, cmd := range globalCommands {
		canonical := "/" + cmd.Key
		if len(cmd.TextAliases) > 0 && strings.TrimSpace(cmd.TextAliases[0]) != "" {
			canonical = strings.TrimSpace(cmd.TextAliases[0])
		}
		acceptsArgs := cmd.AcceptsArgs
		for _, alias := range cmd.TextAliases {
			normalized := strings.ToLower(strings.TrimSpace(alias))
			if normalized == "" {
				continue
			}
			if _, exists := aliasMap[normalized]; !exists {
				aliasMap[normalized] = &TextAliasSpec{
					Key:         cmd.Key,
					Canonical:   canonical,
					AcceptsArgs: acceptsArgs,
				}
			}
		}
	}
	return aliasMap
}

// GetTextAliasMap 获取文本别名映射（带缓存，并发安全）。
// TS 对照: commands-registry.ts L44-69 getTextAliasMap
func GetTextAliasMap() map[string]*TextAliasSpec {
	textAliasCacheMu.Lock()
	once := textAliasMapOnce
	textAliasCacheMu.Unlock()

	once.Do(func() {
		textAliasCacheMu.Lock()
		cachedTextAliasMap = buildTextAliasMap()
		textAliasCacheMu.Unlock()
	})

	textAliasCacheMu.Lock()
	m := cachedTextAliasMap
	textAliasCacheMu.Unlock()
	return m
}

// InvalidateTextAliasCache 使 text alias 缓存失效（命令注册变更时调用）。
func InvalidateTextAliasCache() {
	textAliasCacheMu.Lock()
	cachedTextAliasMap = nil
	textAliasMapOnce = &sync.Once{}
	textAliasCacheMu.Unlock()
}

// MaybeResolveTextAlias 尝试将文本解析为命令别名。
// 返回规范化的命令路径 (如 "/help") 或空字符串。
// TS 对照: commands-registry.ts L457-476 maybeResolveTextAlias
func MaybeResolveTextAlias(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed[0] != '/' {
		return ""
	}
	normalized := strings.ToLower(trimmed)
	// 先检查是否是已知的精确命令
	detection := GetCommandDetection()
	if _, ok := detection.Exact[normalized]; ok {
		return normalized
	}
	// 提取命令 token
	spaceIdx := strings.IndexByte(normalized[1:], ' ')
	tokenKey := normalized
	if spaceIdx >= 0 {
		tokenKey = normalized[:spaceIdx+1]
	}
	// 检查 alias map
	if _, ok := GetTextAliasMap()[tokenKey]; ok {
		return tokenKey
	}
	return ""
}

// ---------- Native 命令名替换 ----------

// nativeNameOverrides 原生命令名称覆写表。
// TS 对照: commands-registry.ts L123-128 NATIVE_NAME_OVERRIDES
var nativeNameOverrides = map[string]map[string]string{
	"discord": {
		"tts": "voice",
	},
}

// ResolveNativeName 解析原生命令名（含 provider 覆写）。
// TS 对照: commands-registry.ts L130-141 resolveNativeName
func ResolveNativeName(cmd *ChatCommandDefinition, provider string) string {
	if cmd == nil || cmd.NativeName == "" {
		return ""
	}
	if provider != "" {
		if overrides, ok := nativeNameOverrides[provider]; ok {
			if override, ok := overrides[cmd.Key]; ok {
				return override
			}
		}
	}
	return cmd.NativeName
}

// ---------- Skill 命令定义构建 ----------

// BuildSkillCommandDefinitions 将技能命令转为命令定义。
// TS 对照: commands-registry.ts L75-88 buildSkillCommandDefinitions
func BuildSkillCommandDefinitions(specs []SkillCommandSpec) []*ChatCommandDefinition {
	if len(specs) == 0 {
		return nil
	}
	defs := make([]*ChatCommandDefinition, len(specs))
	for i, spec := range specs {
		defs[i] = &ChatCommandDefinition{
			Key:         "skill:" + spec.Name,
			NativeName:  spec.Name,
			Description: spec.Description,
			TextAliases: []string{"/" + spec.Name},
			AcceptsArgs: true,
			ArgsParsing: ArgsParsingNone,
			Scope:       CommandScopeBoth,
		}
	}
	return defs
}

// ---------- 按配置过滤 Native 命令规格 ----------

// ListNativeCommandSpecsForConfig 按配置过滤后生成 native 命令规格。
// TS 对照: commands-registry.ts L157-169 listNativeCommandSpecsForConfig
func ListNativeCommandSpecsForConfig(cfg *CommandsEnabledConfig, provider string) []NativeCommandSpec {
	filtered := ListChatCommandsForConfig(cfg)
	var specs []NativeCommandSpec
	for _, cmd := range filtered {
		if cmd.Scope == CommandScopeText || cmd.NativeName == "" {
			continue
		}
		name := ResolveNativeName(cmd, provider)
		if name == "" {
			name = cmd.Key
		}
		specs = append(specs, NativeCommandSpec{
			Name:        name,
			Description: cmd.Description,
			AcceptsArgs: cmd.AcceptsArgs,
			Args:        cmd.Args,
		})
	}
	return specs
}

// ---------- 命令参数菜单解析 ----------

// ResolvedArgMenu 解析后的命令参数菜单。
type ResolvedArgMenu struct {
	Arg     *CommandArgDefinition
	Choices []ResolvedCommandArgChoice
	Title   string
}

// ResolveCommandArgMenu 解析命令的参数菜单。
// TS 对照: commands-registry.ts L330-366 resolveCommandArgMenu
func ResolveCommandArgMenu(cmd *ChatCommandDefinition, args *CommandArgs) *ResolvedArgMenu {
	if cmd == nil || len(cmd.Args) == 0 {
		return nil
	}
	// 必须有 argsMenu 或 argsMenuAuto
	if cmd.ArgsMenu == nil && !cmd.ArgsMenuAuto {
		return nil
	}
	if cmd.ArgsParsing == ArgsParsingNone {
		return nil
	}

	// 确定要展示菜单的参数名
	var argName string
	var title string

	if cmd.ArgsMenuAuto {
		// auto 模式：找第一个有 choices 的参数
		for i := range cmd.Args {
			if len(cmd.Args[i].Choices) > 0 {
				argName = cmd.Args[i].Name
				break
			}
		}
	} else if cmd.ArgsMenu != nil {
		argName = cmd.ArgsMenu.Arg
		title = cmd.ArgsMenu.Title
	}
	if argName == "" {
		return nil
	}

	// 如果已提供该参数值，不展示菜单
	if args != nil && args.Values != nil {
		if v, ok := args.Values[argName]; ok && v != nil {
			return nil
		}
	}
	if args != nil && args.Raw != "" && args.Values == nil {
		return nil
	}

	// 查找参数定义
	var argDef *CommandArgDefinition
	for i := range cmd.Args {
		if cmd.Args[i].Name == argName {
			argDef = &cmd.Args[i]
			break
		}
	}
	if argDef == nil {
		return nil
	}

	choices := ResolveCommandArgChoices(argDef)
	if len(choices) == 0 {
		return nil
	}

	return &ResolvedArgMenu{
		Arg:     argDef,
		Choices: choices,
		Title:   title,
	}
}

// FindCommandByNativeName 通过原生命令名查找命令定义。
// TS 对照: commands-registry.ts findCommandByNativeName
func FindCommandByNativeName(nativeName, provider string) *ChatCommandDefinition {
	nativeName = strings.ToLower(strings.TrimSpace(nativeName))
	if nativeName == "" {
		return nil
	}
	for _, cmd := range globalCommands {
		resolved := ResolveNativeName(cmd, provider)
		if strings.ToLower(resolved) == nativeName {
			return cmd
		}
		if strings.ToLower(cmd.NativeName) == nativeName {
			return cmd
		}
		if strings.ToLower(cmd.Key) == nativeName {
			return cmd
		}
	}
	return nil
}

// BuildCommandTextFromArgs 从命令定义和参数构建命令文本。
// TS 对照: commands-registry.ts buildCommandTextFromArgs
func BuildCommandTextFromArgs(cmd *ChatCommandDefinition, args *CommandArgs) string {
	prefix := cmd.NativeName
	if prefix == "" {
		prefix = cmd.Key
	}
	if args == nil {
		return "/" + prefix
	}
	if args.Raw == "" && len(args.Values) == 0 {
		return "/" + prefix
	}
	serialized := args.Raw
	if serialized == "" {
		serialized = SerializeCommandArgs(cmd, args.Values)
	}
	if serialized != "" {
		return "/" + prefix + " " + serialized
	}
	return "/" + prefix
}

// ---------- Native Command Surface 判定 ----------

// NativeCommandSurfaceProvider 动态 native command surface 提供器（DI 注入）。
// 由 gateway 启动时注入 channels.ListNativeCommandChannels 的结果。
// 返回所有支持 native commands 的频道 ID（小写）。
var NativeCommandSurfaceProvider func() []string

// IsNativeCommandSurface 判断 surface 是否支持 native commands。
// TS 对照: commands-registry.ts L505-510 isNativeCommandSurface
// 通过 NativeCommandSurfaceProvider 动态获取（含核心+插件频道）。
func IsNativeCommandSurface(surface string) bool {
	if surface == "" {
		return false
	}
	if NativeCommandSurfaceProvider == nil {
		return false
	}
	lower := strings.ToLower(surface)
	for _, id := range NativeCommandSurfaceProvider() {
		if id == lower {
			return true
		}
	}
	return false
}

//nolint:unused
var commandPrefixRe = regexp.MustCompile(`(?i)^/[a-z]`)
