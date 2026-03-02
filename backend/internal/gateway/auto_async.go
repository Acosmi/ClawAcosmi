package gateway

// auto_async.go — 自动异步检测
//
// shouldAutoAsync 分析用户消息文本，判断是否应自动启用异步模式。
// 规则设计: 积极但不过分 — 复杂多步任务自动异步，简单问答保持同步。

import (
	"strings"
	"unicode/utf8"
)

// strongAsyncKeywords — 强信号关键词，≥15 rune 即触发。
// 这些词汇暗示多步/耗时操作。
var strongAsyncKeywords = []string{
	// 中文
	"重构", "开发一个", "实现一个", "全量", "批量", "所有文件",
	"部署", "迁移", "项目", "整个系统", "全部模块", "所有模块",
	"多个文件", "每个文件", "全面分析", "审计", "子智能体", "coder",
	// 英文
	"refactor", "deploy", "migrate", "docker", "k8s",
	"project", "全面",
}

// mediumAsyncKeywords — 中信号关键词，≥30 rune 触发。
var mediumAsyncKeywords = []string{
	// 中文
	"写", "创建", "编写", "修改", "添加", "新增", "生成",
	"安装", "配置", "升级", "优化", "修复", "测试", "发送",
	// 英文
	"write", "create", "build", "implement", "generate",
	"install", "configure", "upgrade", "optimize", "fix", "test",
}

// writeIntentKeywords — 弱信号关键词，≥80 rune 触发。
var writeIntentKeywords = []string{
	"写", "改", "加", "删", "建", "做",
	"write", "edit", "add", "create", "make", "build", "fix", "update",
}

// questionWords — 疑问词，用于检测疑问句。
var questionWords = []string{
	// 中文
	"什么", "怎么", "为什么", "哪个", "哪些", "是否", "能否", "可以吗", "吗",
	"如何", "多少", "几个", "有没有",
	// 英文（小写匹配）
	"what ", "how ", "why ", "where ", "when ", "which ", "who ",
	"can you", "could you", "is there", "are there", "do you",
}

// imperativeMarkers — 祈使标记，覆盖疑问句判定。
var imperativeMarkers = []string{
	"帮我", "帮忙", "麻烦", "请", "给我",
	"please ", "help me",
}

// shouldAutoAsync 判断消息文本是否应自动启用异步模式。
//
// 阈值说明: 中文字符信息密度 ≈ 英文 3-4 倍，故 rune 阈值偏低。
// 例: "帮我重构错误处理" = 8 rune（中文），"refactor the error handling module" = 34 rune（英文）。
func shouldAutoAsync(text string) bool {
	text = strings.TrimSpace(text)
	runeLen := utf8.RuneCountInString(text)

	// 规则 1: 过短消息 → false（过滤招呼、单词命令）
	if runeLen < 6 {
		return false
	}

	lower := strings.ToLower(text)

	// 规则 2: 疑问句检测 → false（除非强关键词例外）
	if isQuestion(text, lower, runeLen) {
		return false
	}

	// 规则 3: 强关键词 → true（≥6 rune 已通过规则 1）
	if containsAny(lower, strongAsyncKeywords) {
		return true
	}

	// 规则 4: ≥15 rune + 中关键词 → true
	if runeLen >= 15 && containsAny(lower, mediumAsyncKeywords) {
		return true
	}

	// 规则 5: ≥40 rune + 弱关键词 → true
	if runeLen >= 40 && containsAny(lower, writeIntentKeywords) {
		return true
	}

	// 规则 6: ≥80 rune → true（超长消息必然复杂）
	if runeLen >= 80 {
		return true
	}

	// 规则 7: 其余 → false
	return false
}

// isQuestion 检测文本是否为疑问句。
// 例外: >40 rune + 含强关键词 → 仍视为任务（不返回 true）。
func isQuestion(text, lower string, runeLen int) bool {
	// 检测 ? / ？ 结尾
	if strings.HasSuffix(text, "?") || strings.HasSuffix(text, "？") {
		// 例外: 长文本 + 强关键词 → 仍视为任务（非疑问）
		if runeLen > 30 && containsAny(lower, strongAsyncKeywords) {
			return false
		}
		return true
	}

	// 检测疑问词
	if containsAny(lower, questionWords) {
		// 但如果含祈使标记 → 不算疑问句
		if containsAny(lower, imperativeMarkers) {
			return false
		}
		return true
	}

	return false
}

// containsAny 检查 text 是否包含 keywords 中的任意一个。
func containsAny(text string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	return false
}
