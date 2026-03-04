package runner

// ============================================================================
// spawn_media_agent — 委托合约驱动的媒体运营子智能体生成工具
// 参照 spawn_coder_agent.go 模式，角色定义为媒体运营。
//
// 设计文档: docs/xinshenji/impl-tracking-media-subagent.md §P0-1
// ============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/Acosmi/ClawAcosmi/internal/agents/llmclient"
)

// ---------- 工具输入 ----------

// spawnMediaAgentInput spawn_media_agent 工具输入 JSON schema。
type spawnMediaAgentInput struct {
	TaskBrief   string          `json:"task_brief"`
	Scope       []ScopeEntry    `json:"scope"`
	Constraints json.RawMessage `json:"constraints,omitempty"`
	TimeoutMs   *uint32         `json:"timeout_ms,omitempty"`
}

// ---------- 工具定义 ----------

// SpawnMediaAgentToolDef 返回 spawn_media_agent 的 LLM 工具定义。
func SpawnMediaAgentToolDef() llmclient.ToolDef {
	return llmclient.ToolDef{
		Name: "spawn_media_agent",
		Description: "Spawn an oa-media sub-agent for media operations. " +
			"The sub-agent handles trending topic discovery, content composition, " +
			"and multi-platform publishing (WeChat MP, Xiaohongshu, website). " +
			"All publications require approval before going live.",
		InputSchema: json.RawMessage(`{
	"type": "object",
	"properties": {
		"task_brief": {
			"type": "string",
			"description": "Task description for the media agent (≤500 chars). Be specific about what content to create or which platform to publish to."
		},
		"scope": {
			"type": "array",
			"description": "Allowed file paths and permissions for the sub-agent.",
			"items": {
				"type": "object",
				"properties": {
					"path": { "type": "string", "description": "File or directory path (relative to workspace)" },
					"permissions": {
						"type": "array",
						"items": { "type": "string", "enum": ["read", "write", "execute"] }
					}
				},
				"required": ["path", "permissions"]
			}
		},
		"constraints": {
			"type": "object",
			"description": "Execution constraints for the sub-agent.",
			"properties": {
				"no_network": { "type": "boolean", "description": "Deny network access (default: false, media agent typically needs network)" },
				"no_spawn": { "type": "boolean", "description": "Deny process spawning" },
				"max_bash_calls": { "type": "integer", "description": "Maximum bash calls allowed" }
			}
		},
		"timeout_ms": {
			"type": "integer",
			"description": "Sub-agent timeout in milliseconds (default: 120000, longer for media operations)."
		}
	},
	"required": ["task_brief", "scope"]
}`),
	}
}

// ---------- 工具执行 ----------

// createMediaContract parses input and creates a delegation contract.
func createMediaContract(input spawnMediaAgentInput, params ToolExecParams) (*DelegationContract, string) {
	var constraints ContractConstraints
	if len(input.Constraints) > 0 {
		if err := json.Unmarshal(input.Constraints, &constraints); err != nil {
			return nil, fmt.Sprintf("[spawn_media_agent] Invalid constraints: %s", err)
		}
	}

	issuedBy := params.SessionID
	if issuedBy == "" {
		issuedBy = "main-agent"
	}
	contract, err := NewDelegationContract(issuedBy, input.TaskBrief, "", input.Scope, constraints)
	if err != nil {
		return nil, fmt.Sprintf("[spawn_media_agent] Contract validation failed: %s", err)
	}

	// Media agent default timeout: 120s (longer than coder's 60s)
	if input.TimeoutMs != nil && *input.TimeoutMs > 0 {
		contract.TimeoutMs = *input.TimeoutMs
	} else {
		contract.TimeoutMs = 120_000
	}
	return contract, ""
}

// executeSpawnMediaAgent 处理 spawn_media_agent 工具调用。
func executeSpawnMediaAgent(
	ctx context.Context,
	inputJSON json.RawMessage,
	params ToolExecParams,
) (string, error) {
	var input spawnMediaAgentInput
	if err := json.Unmarshal(inputJSON, &input); err != nil {
		return fmt.Sprintf("[spawn_media_agent] Invalid input: %s", err), nil
	}

	contract, errMsg := createMediaContract(input, params)
	if errMsg != "" {
		return errMsg, nil
	}

	slog.Info("spawn_media_agent: contract created",
		"contractID", contract.ContractID,
		"taskBrief", contract.TaskBrief,
		"scopeCount", len(contract.Scope),
	)

	// Monotonic decay validation
	parentCaps := CapabilitySetFromToolExecParams(&params)
	contractCaps := CapabilitySetFromContract(contract)
	if err := parentCaps.ValidateMonotonicDecay(contractCaps); err != nil {
		return fmt.Sprintf("[spawn_media_agent] Permission monotonic decay violation: %s", err), nil
	}

	systemPrompt := buildMediaSystemPrompt(input.TaskBrief, contract, params.SessionKey, params.MediaSubsystem)

	if params.SpawnSubagent == nil {
		contractJSON, _ := json.MarshalIndent(contract, "", "  ")
		return fmt.Sprintf("[spawn_media_agent] Contract created but spawn callback not configured.\n\nContract:\n%s", contractJSON), nil
	}

	contract.Status = ContractActive
	outcome, err := params.SpawnSubagent(ctx, SpawnSubagentParams{
		Contract:     contract,
		Task:         input.TaskBrief,
		SystemPrompt: systemPrompt,
		TimeoutMs:    int64(contract.TimeoutMs),
		Label:        fmt.Sprintf("media-%s", contract.ContractID[:8]),
		Channel:      params.AgentChannel,
		AgentType:    "media",
	})
	if err != nil {
		contract.Status = ContractFailed
		return fmt.Sprintf("[spawn_media_agent] Sub-agent spawn failed: %s", err), nil
	}

	return formatMediaSpawnResult(contract, outcome), nil
}

// ---------- 系统提示词 ----------

// buildMediaSystemPrompt 通过 MediaSubsystem 接口构建媒体子智能体系统提示词。
// 若 MediaSubsystem 不可用，退回到内联极简 prompt。
func buildMediaSystemPrompt(task string, contract *DelegationContract, sessionKey string, mediaSub MediaSubsystemForAgent) string {
	var contractPrompt string
	if contract != nil {
		contractPrompt = contract.FormatForSystemPrompt()
	}
	if mediaSub != nil {
		return mediaSub.BuildSystemPrompt(task, contractPrompt, sessionKey)
	}
	// Fallback: 无 MediaSubsystem 时的极简 prompt
	return fmt.Sprintf("# oa-media Sub-Agent\n\nYou are oa-media. Task: %s\n\n%s", task, contractPrompt)
}

// ---------- 结果格式化 ----------

// formatMediaSpawnResult 格式化媒体子智能体执行结果。
func formatMediaSpawnResult(
	contract *DelegationContract,
	outcome *SubagentRunOutcome,
) string {
	if outcome == nil {
		return fmt.Sprintf("[spawn_media_agent] Contract %s: no outcome returned",
			contract.ContractID)
	}

	if tr := outcome.ThoughtResult; tr != nil {
		result := fmt.Sprintf("[Media Agent Result]\nContract: %s\nStatus: %s\n",
			contract.ContractID, tr.Status)
		if tr.Result != "" {
			result += fmt.Sprintf("\n%s\n", tr.Result)
		}
		if tr.ReasoningSummary != "" {
			result += fmt.Sprintf("\nReasoning: %s\n", tr.ReasoningSummary)
		}
		return result
	}

	result := fmt.Sprintf("[Media Agent Result]\nContract: %s\nStatus: %s\n",
		contract.ContractID, outcome.Status)
	if outcome.Error != "" {
		result += fmt.Sprintf("Error: %s\n", outcome.Error)
	}
	return result
}
