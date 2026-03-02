package gateway

import (
	"strings"
	"testing"
)

func TestShouldAutoAsync(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect bool
	}{
		// 规则 1: 过短消息 (<6 rune)
		{"short_greeting", "你好", false},
		{"short_cmd", "查看状态", false},
		{"short_word", "hello", false},

		// 规则 2: 疑问句
		{"question_cn", "这个 API 怎么用？", false},
		{"question_en", "What is this function?", false},
		{"question_word_cn", "什么是 goroutine", false},
		{"question_word_en", "How does the server work", false},
		// 疑问句 + 强关键词但短文本 → 仍然是疑问句
		{"question_short_strong", "这个项目怎么重构？", false},
		// 疑问词 + 祈使标记 → 不算疑问句 → 走强关键词
		{"question_with_imperative", "帮我看看这个项目怎么部署到生产环境", true},
		// 长疑问句 + 强关键词 → 例外，视为任务
		{"question_long_strong_override", "帮我重构这个模块的全部错误处理逻辑并审计所有变更然后部署到测试环境？", true},

		// 规则 3: 强关键词（≥6 rune）
		{"strong_refactor", "帮我重构错误处理这个模块", true},
		{"strong_deploy", "部署到生产环境并检查日志", true},
		{"strong_coder", "让 coder 帮我实现这个功能", true},
		{"strong_batch", "批量修改所有文件的导入路径", true},
		{"strong_migrate", "migrate the database schema", true},
		{"strong_docker", "使用 docker 构建新的镜像", true},

		// 规则 4: ≥15 rune + 中关键词
		{"medium_create", "创建一个新的 HTTP 服务端点用于处理用户认证请求", true},
		{"medium_fix", "修复登录页面的样式问题并调整按钮的位置布局", true},
		{"medium_short", "修改配置", false},             // 太短，<6 rune
		{"medium_below_threshold", "创建一个文件", false}, // 中关键词但 <15 rune
		{"medium_generate_en", "generate a new controller for handling auth", true},

		// 规则 5: ≥40 rune + 弱关键词
		{"weak_long_write", "我需要你把这个文件的第三段代码改一下，现在的逻辑不太对，具体来说就是循环条件那里应该用小于等于而不是小于", true},

		// 规则 6: ≥80 rune 超长消息
		{"very_long_no_keywords", string(make([]rune, 80)), true},

		// 规则 7: 其余情况
		{"no_keyword_medium_len", "看看这个文件的内容然后告诉我", false},
		{"status_check", "查看当前系统运行状态和内存使用情况", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldAutoAsync(tt.input)
			if got != tt.expect {
				t.Errorf("shouldAutoAsync(%q) = %v, want %v (runeLen=%d)",
					tt.input, got, tt.expect, len([]rune(tt.input)))
			}
		})
	}
}

func TestIsQuestion(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		expect bool
	}{
		{"ends_with_question_mark_cn", "这是什么？", true},
		{"ends_with_question_mark_en", "What is this?", true},
		{"question_word_cn", "为什么会报错", true},
		{"question_word_en_how", "how does it work", true},
		{"imperative_overrides", "帮我看看为什么报错", false},
		{"not_question", "执行部署操作", false},
		// 长文本 (>30 rune) + 强关键词 + ? → 例外（不算疑问句）
		{"long_strong_with_qmark", "帮我重构这个模块的全部错误处理逻辑并审计所有变更然后部署到测试环境？", false},
		// 短文本 + 强关键词 + ? → 仍然是疑问句
		{"short_strong_with_qmark", "这个项目怎么重构？", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lower := strings.ToLower(tt.text)
			runeLen := len([]rune(tt.text))
			got := isQuestion(tt.text, lower, runeLen)
			if got != tt.expect {
				t.Errorf("isQuestion(%q) = %v, want %v (runeLen=%d)",
					tt.text, got, tt.expect, runeLen)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		text     string
		keywords []string
		expect   bool
	}{
		{"hello world", []string{"hello"}, true},
		{"hello world", []string{"foo", "world"}, true},
		{"hello world", []string{"foo", "bar"}, false},
		{"", []string{"foo"}, false},
	}

	for _, tt := range tests {
		got := containsAny(tt.text, tt.keywords)
		if got != tt.expect {
			t.Errorf("containsAny(%q, %v) = %v, want %v", tt.text, tt.keywords, got, tt.expect)
		}
	}
}
