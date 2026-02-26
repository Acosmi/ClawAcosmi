// tools/cron_tool.go — 定时任务工具。
// TS 参考：src/agents/tools/cron-tool.ts (475L)
package tools

import (
	"context"
	"fmt"
)

// CronManager 定时任务管理接口。
type CronManager interface {
	ListJobs(ctx context.Context) ([]CronJob, error)
	CreateJob(ctx context.Context, job CronJobInput) (string, error)
	UpdateJob(ctx context.Context, jobID string, updates map[string]any) error
	DeleteJob(ctx context.Context, jobID string) error
	GetJob(ctx context.Context, jobID string) (*CronJob, error)
}

// CronJob 定时任务。
type CronJob struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Schedule  string `json:"schedule"`
	Command   string `json:"command,omitempty"`
	AgentID   string `json:"agentId,omitempty"`
	Prompt    string `json:"prompt,omitempty"`
	Enabled   bool   `json:"enabled"`
	LastRunAt string `json:"lastRunAt,omitempty"`
	NextRunAt string `json:"nextRunAt,omitempty"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// CronJobInput 创建定时任务输入。
type CronJobInput struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Command  string `json:"command,omitempty"`
	AgentID  string `json:"agentId,omitempty"`
	Prompt   string `json:"prompt,omitempty"`
	Enabled  bool   `json:"enabled"`
}

// CreateCronTool 创建定时任务管理工具。
func CreateCronTool(mgr CronManager) *AgentTool {
	return &AgentTool{
		Name:        "cron",
		Label:       "Cron",
		Description: "Manage scheduled tasks: list, create, update, delete, get cron jobs.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type": "string", "enum": []any{"list", "create", "update", "delete", "get"},
					"description": "Cron action",
				},
				"job_id":   map[string]any{"type": "string", "description": "Job ID (for get/update/delete)"},
				"name":     map[string]any{"type": "string", "description": "Job name (for create/update)"},
				"schedule": map[string]any{"type": "string", "description": "Cron expression (for create/update)"},
				"command":  map[string]any{"type": "string", "description": "Command to execute (alternative to prompt)"},
				"prompt":   map[string]any{"type": "string", "description": "Agent prompt (alternative to command)"},
				"agent_id": map[string]any{"type": "string", "description": "Agent ID for prompt execution"},
				"enabled":  map[string]any{"type": "boolean", "description": "Whether the job is enabled"},
			},
			"required": []any{"action"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			action, err := ReadStringParam(args, "action", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}
			if mgr == nil {
				return nil, fmt.Errorf("cron manager not configured")
			}

			switch action {
			case "list":
				jobs, err := mgr.ListJobs(ctx)
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"jobs": jobs, "count": len(jobs)}), nil
			case "get":
				jobID, err := ReadStringParam(args, "job_id", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				job, err := mgr.GetJob(ctx, jobID)
				if err != nil {
					return nil, err
				}
				return JsonResult(job), nil
			case "create":
				name, err := ReadStringParam(args, "name", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				schedule, err := ReadStringParam(args, "schedule", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				command, _ := ReadStringParam(args, "command", nil)
				prompt, _ := ReadStringParam(args, "prompt", nil)
				agentID, _ := ReadStringParam(args, "agent_id", nil)
				enabled := true
				if v, ok := args["enabled"].(bool); ok {
					enabled = v
				}
				id, err := mgr.CreateJob(ctx, CronJobInput{
					Name: name, Schedule: schedule, Command: command,
					Prompt: prompt, AgentID: agentID, Enabled: enabled,
				})
				if err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "created", "id": id}), nil
			case "update":
				jobID, err := ReadStringParam(args, "job_id", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				updates := map[string]any{}
				for _, key := range []string{"name", "schedule", "command", "prompt", "agent_id"} {
					if v, _ := ReadStringParam(args, key, nil); v != "" {
						updates[key] = v
					}
				}
				if v, ok := args["enabled"].(bool); ok {
					updates["enabled"] = v
				}
				if err := mgr.UpdateJob(ctx, jobID, updates); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "updated", "id": jobID}), nil
			case "delete":
				jobID, err := ReadStringParam(args, "job_id", &StringParamOptions{Required: true})
				if err != nil {
					return nil, err
				}
				if err := mgr.DeleteJob(ctx, jobID); err != nil {
					return nil, err
				}
				return JsonResult(map[string]any{"status": "deleted", "id": jobID}), nil
			default:
				return nil, fmt.Errorf("unknown cron action: %s", action)
			}
		},
	}
}
