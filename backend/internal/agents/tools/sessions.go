// tools/sessions.go — 会话工具集。
// TS 参考：8× sessions-*.ts (~1,700L)
package tools

import (
	"context"
	"fmt"
	"time"
)

// SessionManager 会话管理器接口。
type SessionManager interface {
	ListSessions(ctx context.Context) ([]SessionInfo, error)
	GetHistory(ctx context.Context, sessionKey string, limit int) ([]SessionMessage, error)
	SendMessage(ctx context.Context, sessionKey, content string) error
	SpawnSession(ctx context.Context, agentID, prompt string) (string, error)
	GetStatus(ctx context.Context, sessionKey string) (*SessionStatus, error)
	SetStatus(ctx context.Context, sessionKey string, status *SessionStatus) error
}

// SessionInfo 会话信息。
type SessionInfo struct {
	Key       string `json:"key"`
	AgentID   string `json:"agentId"`
	Title     string `json:"title,omitempty"`
	Slug      string `json:"slug,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// SessionMessage 会话消息。
type SessionMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// SessionStatus 会话状态。
type SessionStatus struct {
	Status    string `json:"status"` // active | idle | paused
	AgentID   string `json:"agentId,omitempty"`
	Message   string `json:"message,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// CreateSessionsListTool 创建会话列表工具。
func CreateSessionsListTool(mgr SessionManager) *AgentTool {
	return &AgentTool{
		Name:        "sessions_list",
		Label:       "List Sessions",
		Description: "List all active sessions.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			if mgr == nil {
				return nil, fmt.Errorf("session manager not configured")
			}
			sessions, err := mgr.ListSessions(ctx)
			if err != nil {
				return nil, fmt.Errorf("list sessions: %w", err)
			}
			return JsonResult(map[string]any{
				"sessions": sessions,
				"count":    len(sessions),
			}), nil
		},
	}
}

// CreateSessionsHistoryTool 创建会话历史工具。
func CreateSessionsHistoryTool(mgr SessionManager) *AgentTool {
	return &AgentTool{
		Name:        "sessions_history",
		Label:       "Session History",
		Description: "Get the conversation history for a specific session.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				// camelCase 与 TS sessions-history-tool.ts schema 对齐
				"sessionKey": map[string]any{
					"type":        "string",
					"description": "The session key",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Maximum number of messages (default: 50)",
				},
			},
			"required": []any{"sessionKey"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			sessionKey, err := ReadStringParam(args, "sessionKey", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}
			limit := 50
			if l, ok, _ := ReadNumberParam(args, "limit", &NumberParamOptions{Integer: true}); ok && l > 0 {
				limit = int(l)
			}
			if mgr == nil {
				return nil, fmt.Errorf("session manager not configured")
			}
			messages, err := mgr.GetHistory(ctx, sessionKey, limit)
			if err != nil {
				return nil, fmt.Errorf("get history: %w", err)
			}
			return JsonResult(map[string]any{
				"sessionKey": sessionKey,
				"messages":   messages,
				"count":      len(messages),
			}), nil
		},
	}
}

// CreateSessionsSendTool 创建会话发送工具。
// TS 参考：sessions-send-tool.ts — SessionsSendToolSchema
// 参数名对齐（camelCase）：session_key→sessionKey, content→message
// 新增参数：label, agentId, timeoutSeconds
func CreateSessionsSendTool(mgr SessionManager) *AgentTool {
	return &AgentTool{
		Name:        "sessions_send",
		Label:       "Session Send",
		Description: "Send a message into another session. Use sessionKey or label to identify the target.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				// camelCase 与 TS SessionsSendToolSchema 完全对齐
				"sessionKey": map[string]any{
					"type":        "string",
					"description": "The session key of the target session.",
				},
				"label": map[string]any{
					"type":        "string",
					"description": "Human-readable label of the target session (alternative to sessionKey).",
				},
				"agentId": map[string]any{
					"type":        "string",
					"description": "Agent ID to scope label lookup (optional).",
				},
				"message": map[string]any{
					"type":        "string",
					"description": "The message to send into the target session.",
				},
				"timeoutSeconds": map[string]any{
					"type":        "number",
					"description": "Seconds to wait for a reply (0 = fire-and-forget, default: 30).",
					"minimum":     0,
				},
			},
			"required": []any{"message"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			// 读取 camelCase 参数（与 TS schema 一致）
			sessionKey, _ := ReadStringParam(args, "sessionKey", nil)
			label, _ := ReadStringParam(args, "label", nil)
			message, err := ReadStringParam(args, "message", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}

			// sessionKey 和 label 不能同时提供
			if sessionKey != "" && label != "" {
				return JsonResult(map[string]any{
					"status": "error",
					"error":  "Provide either sessionKey or label (not both).",
				}), nil
			}

			// 必须提供 sessionKey 或 label 之一
			targetKey := sessionKey
			if targetKey == "" && label != "" {
				// label 解析：直接用 label 作为后备标识
				targetKey = label
			}
			if targetKey == "" {
				return JsonResult(map[string]any{
					"status": "error",
					"error":  "Either sessionKey or label is required",
				}), nil
			}

			if mgr == nil {
				return nil, fmt.Errorf("session manager not configured")
			}
			if err := mgr.SendMessage(ctx, targetKey, message); err != nil {
				return nil, fmt.Errorf("send message: %w", err)
			}
			return JsonResult(map[string]any{
				"status":     "sent",
				"sessionKey": targetKey,
			}), nil
		},
	}
}

// CreateSessionsSpawnTool 创建会话生成工具。
func CreateSessionsSpawnTool(mgr SessionManager) *AgentTool {
	return &AgentTool{
		Name:        "sessions_spawn",
		Label:       "Spawn Session",
		Description: "Create a new session with a specific agent and initial prompt.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"agentId": map[string]any{
					"type": "string", "description": "The agent ID",
				},
				"prompt": map[string]any{
					"type": "string", "description": "Initial prompt for the session",
				},
			},
			"required": []any{"prompt"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			agentID, _ := ReadStringParam(args, "agentId", nil)
			prompt, err := ReadStringParam(args, "prompt", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}
			if mgr == nil {
				return nil, fmt.Errorf("session manager not configured")
			}
			sessionKey, err := mgr.SpawnSession(ctx, agentID, prompt)
			if err != nil {
				return nil, fmt.Errorf("spawn session: %w", err)
			}
			return JsonResult(map[string]any{
				"status":      "spawned",
				"session_key": sessionKey,
			}), nil
		},
	}
}

// CreateSessionStatusTool 创建会话状态工具。
// TS 参考: session-status-tool.ts (472L)
func CreateSessionStatusTool(mgr SessionManager) *AgentTool {
	return &AgentTool{
		Name:        "session_status",
		Label:       "Session Status",
		Description: "Get or update the status of the current session.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []any{"get", "set"},
					"description": "Action: get current status or set a new status",
				},
				"sessionKey": map[string]any{
					"type": "string", "description": "Session key (optional, defaults to current)",
				},
				"status": map[string]any{
					"type":        "string",
					"enum":        []any{"active", "idle", "paused"},
					"description": "New status (for set action)",
				},
				"message": map[string]any{
					"type": "string", "description": "Status message",
				},
			},
			"required": []any{"action"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			action, err := ReadStringParam(args, "action", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}
			sessionKey, _ := ReadStringParam(args, "sessionKey", nil)
			if mgr == nil {
				return nil, fmt.Errorf("session manager not configured")
			}

			switch action {
			case "get":
				status, err := mgr.GetStatus(ctx, sessionKey)
				if err != nil {
					return nil, fmt.Errorf("get status: %w", err)
				}
				return JsonResult(status), nil
			case "set":
				statusStr, _ := ReadStringParam(args, "status", nil)
				message, _ := ReadStringParam(args, "message", nil)
				status := &SessionStatus{
					Status:    statusStr,
					Message:   message,
					UpdatedAt: time.Now().UTC().Format(time.RFC3339),
				}
				if err := mgr.SetStatus(ctx, sessionKey, status); err != nil {
					return nil, fmt.Errorf("set status: %w", err)
				}
				return JsonResult(map[string]any{
					"status":      "updated",
					"session_key": sessionKey,
				}), nil
			default:
				return nil, fmt.Errorf("unknown action: %s", action)
			}
		},
	}
}
