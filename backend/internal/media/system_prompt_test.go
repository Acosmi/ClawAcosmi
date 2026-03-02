package media

// ============================================================================
// media/system_prompt_test.go — oa-media 系统提示词单元测试
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P4-4
// ============================================================================

import (
	"strings"
	"testing"
)

// mockContract implements ContractFormatter for testing.
type mockContract struct {
	output string
}

func (m *mockContract) FormatForSystemPrompt() string { return m.output }

// ---------- Test cases ----------

func TestBuildMediaPrompt_Default(t *testing.T) {
	prompt := BuildMediaSystemPrompt(MediaPromptParams{
		Task: "采集AI热点并创建公众号文章",
	})

	// 验证 12 个 section 标题全部存在。
	sections := []string{
		"# oa-media 子智能体",
		"## 能力（工具集）",
		"## 内容创作指南",
		"## 平台规范",
		"## 审批流程（关键）",
		"## 社交互动规则",
		"## 工具使用",
		"## 质量标准",
		"## 任务执行",
		"## 输出格式：ThoughtResult JSON",
		"## 能力边界",
		"## 会话上下文",
	}
	for _, s := range sections {
		if !strings.Contains(prompt, s) {
			t.Errorf("missing section heading: %q", s)
		}
	}
}

func TestBuildMediaPrompt_TaskInjection(t *testing.T) {
	task := "采集AI领域热点TOP5，选最热话题生成800字公众号图文"
	prompt := BuildMediaSystemPrompt(MediaPromptParams{Task: task})

	if !strings.Contains(prompt, task) {
		t.Error("task not injected into prompt")
	}
}

func TestBuildMediaPrompt_EmptyTask(t *testing.T) {
	prompt := BuildMediaSystemPrompt(MediaPromptParams{})

	if !strings.Contains(prompt, "{{TASK_DESCRIPTION}}") {
		t.Error("empty task should use placeholder")
	}
}

func TestBuildMediaPrompt_WithContract(t *testing.T) {
	contract := &mockContract{output: "## Contract\n\ncontract-id: abc-123\n"}
	prompt := BuildMediaSystemPrompt(MediaPromptParams{
		Task:     "test task",
		Contract: contract,
	})

	if !strings.Contains(prompt, "contract-id: abc-123") {
		t.Error("contract content not in prompt")
	}
}

func TestBuildMediaPrompt_WithSessionKey(t *testing.T) {
	prompt := BuildMediaSystemPrompt(MediaPromptParams{
		Task:                "test task",
		RequesterSessionKey: "session-key-xyz",
	})

	if !strings.Contains(prompt, "session-key-xyz") {
		t.Error("session key not in prompt")
	}
}

func TestBuildMediaPrompt_PlatformRules(t *testing.T) {
	prompt := BuildMediaSystemPrompt(MediaPromptParams{Task: "test"})

	// 验证各平台规范数值存在。
	checks := []struct {
		label string
		text  string
	}{
		{"wechat title limit", "≤64 字符"},
		{"xhs title limit", "≤20 字符"},
		{"xhs body limit", "≤1000 字符"},
		{"xhs image limit", "≤9 张"},
		{"xhs rate limit", "≥5 seconds"},
		{"website format", "Markdown"},
	}
	for _, c := range checks {
		if !strings.Contains(prompt, c.text) {
			t.Errorf("missing platform rule %q: %q", c.label, c.text)
		}
	}
}

func TestBuildMediaPrompt_HITLWorkflow(t *testing.T) {
	prompt := BuildMediaSystemPrompt(MediaPromptParams{Task: "test"})

	// 验证 HITL 审批流程关键词。
	must := []string{
		"禁止直接发布",
		"禁止跳过审批",
		"仅在被指示时发布",
		"DraftStore",
	}
	for _, m := range must {
		if !strings.Contains(prompt, m) {
			t.Errorf("HITL workflow missing phrase: %q", m)
		}
	}
}

func TestBuildMediaPrompt_NoBashLeakage(t *testing.T) {
	prompt := BuildMediaSystemPrompt(MediaPromptParams{Task: "test"})

	// 确保不包含 bash/文件系统权限泄漏。
	forbidden := []string{
		"`bash`",
		"`read_file`",
		"`write_file`",
		"git commit",
		"git push",
	}
	for _, f := range forbidden {
		if strings.Contains(prompt, f) {
			t.Errorf("prompt should NOT contain %q (permission leakage)", f)
		}
	}
}
