// tools/agent_step.go — Agent 步骤执行。
// TS 参考：src/agents/tools/agent-step.ts (59L)
package tools

import (
	"context"
	"fmt"
	"time"
)

// ---------- 类型定义 ----------

// AgentMessage Agent 消息。
type AgentMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AgentStepResult Agent 步骤执行结果。
type AgentStepResult struct {
	Reply    string         `json:"reply,omitempty"`
	Messages []AgentMessage `json:"messages,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// AgentRunner Agent 运行器接口。
type AgentRunner interface {
	RunStep(ctx context.Context, agentID, sessionKey, message string) (*AgentStepResult, error)
	GetHistory(ctx context.Context, sessionKey string) ([]AgentMessage, error)
}

// ---------- 读取最新助手回复 ----------

// ReadLatestAssistantReply 获取最新的 assistant 回复。
// TS 参考: agent-step.ts readLatestAssistantReply
func ReadLatestAssistantReply(ctx context.Context, runner AgentRunner, sessionKey string) (string, error) {
	history, err := runner.GetHistory(ctx, sessionKey)
	if err != nil {
		return "", fmt.Errorf("failed to get history: %w", err)
	}

	// 从后往前找第一条 assistant 消息
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" && history[i].Content != "" {
			return history[i].Content, nil
		}
	}

	return "", nil
}

// ---------- 执行 Agent 步骤 ----------

// RunAgentStepOptions 选项。
type RunAgentStepOptions struct {
	AgentID    string
	SessionKey string
	Message    string
	Timeout    time.Duration
}

// RunAgentStep 执行一个 agent 步骤（带超时）。
// TS 参考: agent-step.ts runAgentStep
func RunAgentStep(ctx context.Context, runner AgentRunner, opts RunAgentStepOptions) (*AgentStepResult, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := runner.RunStep(ctx, opts.AgentID, opts.SessionKey, opts.Message)
	if err != nil {
		if ctx.Err() != nil {
			return &AgentStepResult{
				Error: fmt.Sprintf("agent step timed out after %s", timeout),
			}, nil
		}
		return nil, err
	}

	return result, nil
}
