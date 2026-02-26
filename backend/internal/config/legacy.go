package config

// Legacy 配置兼容系统 — 对应 src/config/legacy.ts + legacy.shared.ts + legacy.rules.ts
//
// 提供两个核心能力:
//   1. findLegacyConfigIssues — 扫描原始配置, 报告已废弃字段
//   2. applyLegacyMigrations — 自动将旧字段迁移到新位置
//
// 依赖: 纯 Go (map 操作), 无外部依赖
// TS 来源: ~400 行 (legacy.ts 44 + legacy.shared.ts 126 + legacy.rules.ts 132)

import (
	"fmt"
	"strings"
)

// ── 类型定义 ──

// LegacyConfigIssue 表示一个已废弃的配置字段
type LegacyConfigIssue struct {
	Path    string
	Message string
}

// legacyConfigRule 规则 (path + message + 可选 match)
type legacyConfigRule struct {
	path    []string
	message string
	match   func(value interface{}, root map[string]interface{}) bool
}

// LegacyConfigMigration 迁移定义
type LegacyConfigMigration struct {
	ID       string
	Describe string
	Apply    func(raw map[string]interface{}, changes *[]string)
}

// LegacyMigrationResult 迁移结果
type LegacyMigrationResult struct {
	Next    map[string]interface{}
	Changes []string
}

// ── 共享工具函数 ──

// isRecord 判断值是否为 map[string]interface{}
func isRecord(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}

// getRecord 安全转换为 map, 失败返回 nil
func getRecord(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

// ensureRecord 确保 root[key] 是 map, 不存在则创建
func ensureRecord(root map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := root[key]; ok {
		if m, isMap := existing.(map[string]interface{}); isMap {
			return m
		}
	}
	next := make(map[string]interface{})
	root[key] = next
	return next
}

// mergeMissing 递归合并缺失的 key (不覆盖已有值)
func mergeMissing(target, source map[string]interface{}) {
	for key, value := range source {
		if value == nil {
			continue
		}
		existing, exists := target[key]
		if !exists {
			target[key] = value
			continue
		}
		existingMap := getRecord(existing)
		valueMap := getRecord(value)
		if existingMap != nil && valueMap != nil {
			mergeMissing(existingMap, valueMap)
		}
	}
}

// getAgentsList 从 agents 中获取 list 数组
func getAgentsList(agents map[string]interface{}) []interface{} {
	if agents == nil {
		return nil
	}
	list, ok := agents["list"]
	if !ok {
		return nil
	}
	arr, isArr := list.([]interface{})
	if !isArr {
		return nil
	}
	return arr
}

// resolveDefaultAgentIdFromRaw 从原始配置解析默认 agent ID
func resolveDefaultAgentIdFromRaw(raw map[string]interface{}) string {
	agents := getRecord(raw["agents"])
	list := getAgentsList(agents)

	// 找 default=true 的 agent
	for _, item := range list {
		entry := getRecord(item)
		if entry == nil {
			continue
		}
		if entry["default"] == true {
			if id, ok := entry["id"].(string); ok && strings.TrimSpace(id) != "" {
				return strings.TrimSpace(id)
			}
		}
	}

	// 从 routing.defaultAgentId
	routing := getRecord(raw["routing"])
	if routing != nil {
		if routingDefault, ok := routing["defaultAgentId"].(string); ok && strings.TrimSpace(routingDefault) != "" {
			return strings.TrimSpace(routingDefault)
		}
	}

	// 第一个有 id 的 agent
	for _, item := range list {
		entry := getRecord(item)
		if entry == nil {
			continue
		}
		if id, ok := entry["id"].(string); ok && strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
	}

	return "main"
}

// ensureAgentEntry 确保 list 中存在指定 id 的 agent 条目
func ensureAgentEntry(list *[]interface{}, id string) map[string]interface{} {
	normalized := strings.TrimSpace(id)
	for _, item := range *list {
		entry := getRecord(item)
		if entry == nil {
			continue
		}
		if entryID, ok := entry["id"].(string); ok && strings.TrimSpace(entryID) == normalized {
			return entry
		}
	}
	created := map[string]interface{}{"id": normalized}
	*list = append(*list, created)
	return created
}

// mapLegacyAudioTranscription 映射旧版音频转录配置
func mapLegacyAudioTranscription(value interface{}) map[string]interface{} {
	transcriber := getRecord(value)
	if transcriber == nil {
		return nil
	}
	command, ok := transcriber["command"].([]interface{})
	if !ok || len(command) == 0 {
		return nil
	}
	rawExecutable := strings.TrimSpace(fmt.Sprintf("%v", command[0]))
	if rawExecutable == "" {
		return nil
	}

	// 提取可执行文件名
	parts := strings.FieldsFunc(rawExecutable, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	executableName := parts[len(parts)-1]

	// 白名单
	allowList := map[string]bool{"whisper": true}
	if !allowList[executableName] {
		return nil
	}

	args := make([]string, 0, len(command)-1)
	for _, part := range command[1:] {
		args = append(args, fmt.Sprintf("%v", part))
	}

	result := map[string]interface{}{
		"command": rawExecutable,
		"type":    "cli",
	}
	if len(args) > 0 {
		result["args"] = args
	}
	if timeout, ok := transcriber["timeoutSeconds"].(float64); ok {
		result["timeoutSeconds"] = timeout
	}
	return result
}

// deepCloneMap 深拷贝 map (替代 TS structuredClone)
func deepCloneMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			dst[k] = deepCloneMap(val)
		case []interface{}:
			dst[k] = deepCloneSlice(val)
		default:
			dst[k] = v
		}
	}
	return dst
}

func deepCloneSlice(src []interface{}) []interface{} {
	if src == nil {
		return nil
	}
	dst := make([]interface{}, len(src))
	for i, v := range src {
		switch val := v.(type) {
		case map[string]interface{}:
			dst[i] = deepCloneMap(val)
		case []interface{}:
			dst[i] = deepCloneSlice(val)
		default:
			dst[i] = v
		}
	}
	return dst
}

// ── 规则数据 ──

var legacyConfigRules = []legacyConfigRule{
	{path: []string{"whatsapp"}, message: "whatsapp config moved to channels.whatsapp (auto-migrated on load)."},
	{path: []string{"telegram"}, message: "telegram config moved to channels.telegram (auto-migrated on load)."},
	{path: []string{"discord"}, message: "discord config moved to channels.discord (auto-migrated on load)."},
	{path: []string{"slack"}, message: "slack config moved to channels.slack (auto-migrated on load)."},
	{path: []string{"signal"}, message: "signal config moved to channels.signal (auto-migrated on load)."},
	{path: []string{"imessage"}, message: "imessage config moved to channels.imessage (auto-migrated on load)."},
	{path: []string{"msteams"}, message: "msteams config moved to channels.msteams (auto-migrated on load)."},
	{path: []string{"routing", "allowFrom"}, message: "routing.allowFrom was removed; use channels.whatsapp.allowFrom instead (auto-migrated on load)."},
	{path: []string{"routing", "bindings"}, message: "routing.bindings was moved; use top-level bindings instead (auto-migrated on load)."},
	{path: []string{"routing", "agents"}, message: "routing.agents was moved; use agents.list instead (auto-migrated on load)."},
	{path: []string{"routing", "defaultAgentId"}, message: "routing.defaultAgentId was moved; use agents.list[].default instead (auto-migrated on load)."},
	{path: []string{"routing", "agentToAgent"}, message: "routing.agentToAgent was moved; use tools.agentToAgent instead (auto-migrated on load)."},
	{path: []string{"routing", "groupChat", "requireMention"}, message: `routing.groupChat.requireMention was removed; use channels.whatsapp/telegram/imessage groups defaults (e.g. channels.whatsapp.groups."*".requireMention) instead (auto-migrated on load).`},
	{path: []string{"routing", "groupChat", "mentionPatterns"}, message: "routing.groupChat.mentionPatterns was moved; use agents.list[].groupChat.mentionPatterns or messages.groupChat.mentionPatterns instead (auto-migrated on load)."},
	{path: []string{"routing", "queue"}, message: "routing.queue was moved; use messages.queue instead (auto-migrated on load)."},
	{path: []string{"routing", "transcribeAudio"}, message: "routing.transcribeAudio was moved; use tools.media.audio.models instead (auto-migrated on load)."},
	{path: []string{"telegram", "requireMention"}, message: `telegram.requireMention was removed; use channels.telegram.groups."*".requireMention instead (auto-migrated on load).`},
	{path: []string{"identity"}, message: "identity was moved; use agents.list[].identity instead (auto-migrated on load)."},
	{path: []string{"agent"}, message: "agent.* was moved; use agents.defaults (and tools.* for tool/elevated/exec settings) instead (auto-migrated on load)."},
	{path: []string{"tools", "bash"}, message: "tools.bash was removed; use tools.exec instead (auto-migrated on load)."},
	{
		path:    []string{"agent", "model"},
		message: "agent.model string was replaced by agents.defaults.model.primary/fallbacks and agents.defaults.models (auto-migrated on load).",
		match:   func(value interface{}, _ map[string]interface{}) bool { _, ok := value.(string); return ok },
	},
	{
		path:    []string{"agent", "imageModel"},
		message: "agent.imageModel string was replaced by agents.defaults.imageModel.primary/fallbacks (auto-migrated on load).",
		match:   func(value interface{}, _ map[string]interface{}) bool { _, ok := value.(string); return ok },
	},
	{path: []string{"agent", "allowedModels"}, message: "agent.allowedModels was replaced by agents.defaults.models (auto-migrated on load)."},
	{path: []string{"agent", "modelAliases"}, message: "agent.modelAliases was replaced by agents.defaults.models.*.alias (auto-migrated on load)."},
	{path: []string{"agent", "modelFallbacks"}, message: "agent.modelFallbacks was replaced by agents.defaults.model.fallbacks (auto-migrated on load)."},
	{path: []string{"agent", "imageModelFallbacks"}, message: "agent.imageModelFallbacks was replaced by agents.defaults.imageModel.fallbacks (auto-migrated on load)."},
	{path: []string{"messages", "tts", "enabled"}, message: "messages.tts.enabled was replaced by messages.tts.auto (auto-migrated on load)."},
	{path: []string{"gateway", "token"}, message: "gateway.token is ignored; use gateway.auth.token instead (auto-migrated on load)."},
}

// ── 核心函数 ──

// FindLegacyConfigIssues 扫描配置并报告已废弃字段
func FindLegacyConfigIssues(raw interface{}) []LegacyConfigIssue {
	root := getRecord(raw)
	if root == nil {
		return nil
	}

	var issues []LegacyConfigIssue
	for _, rule := range legacyConfigRules {
		var cursor interface{} = root
		for _, key := range rule.path {
			m := getRecord(cursor)
			if m == nil {
				cursor = nil
				break
			}
			cursor = m[key]
		}
		if cursor != nil {
			if rule.match == nil || rule.match(cursor, root) {
				issues = append(issues, LegacyConfigIssue{
					Path:    strings.Join(rule.path, "."),
					Message: rule.message,
				})
			}
		}
	}
	return issues
}

// ApplyLegacyMigrations 应用所有旧版配置迁移
func ApplyLegacyMigrations(raw interface{}) LegacyMigrationResult {
	root := getRecord(raw)
	if root == nil {
		return LegacyMigrationResult{Next: nil, Changes: nil}
	}

	next := deepCloneMap(root)
	var changes []string
	for _, migration := range legacyConfigMigrations {
		migration.Apply(next, &changes)
	}
	if len(changes) == 0 {
		return LegacyMigrationResult{Next: nil, Changes: nil}
	}
	return LegacyMigrationResult{Next: next, Changes: changes}
}
