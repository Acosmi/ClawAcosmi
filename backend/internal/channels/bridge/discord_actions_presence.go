package bridge

import (
	"context"
	"fmt"
	"strings"
)

// Discord presence action handler — 继承自 src/agents/tools/discord-actions-presence.ts (112L)

// Activity type 映射
var discordActivityTypeMap = map[string]int{
	"playing":   0,
	"streaming": 1,
	"listening": 2,
	"watching":  3,
	"custom":    4,
	"competing": 5,
}

var discordValidStatuses = map[string]bool{
	"online": true, "dnd": true, "idle": true, "invisible": true,
}

func handleDiscordPresenceAction(ctx context.Context, action string, params map[string]interface{}, actionGate ActionGate, deps DiscordActionDeps) (ToolResult, error) {
	if action != "setPresence" {
		return ToolResult{}, fmt.Errorf("unknown presence action: %s", action)
	}

	if !actionGate("presence") {
		return ToolResult{}, fmt.Errorf("Discord presence changes are disabled")
	}

	if !deps.IsGatewayConnected(ctx) {
		return ToolResult{}, fmt.Errorf("Discord gateway is not connected. The bot may not be running")
	}

	// 解析 status
	status, _ := ReadStringParam(params, "status", false)
	if status == "" {
		status = "online"
	}
	if !discordValidStatuses[status] {
		validList := make([]string, 0, len(discordValidStatuses))
		for k := range discordValidStatuses {
			validList = append(validList, k)
		}
		return ToolResult{}, fmt.Errorf("invalid status %q. Must be one of: %s", status, strings.Join(validList, ", "))
	}

	// 解析 activity
	activityTypeRaw, _ := ReadStringParam(params, "activityType", false)
	activityName, _ := ReadStringParam(params, "activityName", false)

	var activities []DiscordBridgeActivity
	if activityTypeRaw != "" || activityName != "" {
		if activityTypeRaw == "" {
			validTypes := make([]string, 0, len(discordActivityTypeMap))
			for k := range discordActivityTypeMap {
				validTypes = append(validTypes, k)
			}
			return ToolResult{}, fmt.Errorf("activityType is required when activityName is provided. Valid types: %s", strings.Join(validTypes, ", "))
		}
		typeNum, ok := discordActivityTypeMap[strings.ToLower(activityTypeRaw)]
		if !ok {
			validTypes := make([]string, 0, len(discordActivityTypeMap))
			for k := range discordActivityTypeMap {
				validTypes = append(validTypes, k)
			}
			return ToolResult{}, fmt.Errorf("invalid activityType %q. Must be one of: %s", activityTypeRaw, strings.Join(validTypes, ", "))
		}

		activity := DiscordBridgeActivity{
			Name: activityName,
			Type: typeNum,
		}

		// streaming URL
		if typeNum == 1 {
			if url, _ := ReadStringParam(params, "activityUrl", false); url != "" {
				activity.URL = url
			}
		}

		if state, _ := ReadStringParam(params, "activityState", false); state != "" {
			activity.State = state
		}

		activities = append(activities, activity)
	}

	if err := deps.SetPresence(ctx, status, activities); err != nil {
		return ErrorResult(err), err
	}

	// 返回结果
	actResult := make([]map[string]interface{}, 0, len(activities))
	for _, a := range activities {
		m := map[string]interface{}{
			"type": a.Type,
			"name": a.Name,
		}
		if a.URL != "" {
			m["url"] = a.URL
		}
		if a.State != "" {
			m["state"] = a.State
		}
		actResult = append(actResult, m)
	}

	return OkResult(map[string]interface{}{
		"ok":         true,
		"status":     status,
		"activities": actResult,
	}), nil
}
