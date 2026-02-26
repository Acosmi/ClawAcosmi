package slack

import (
	"context"
	"time"
)

// Slack 连接探测 — 继承自 src/slack/probe.ts (60L)

// SlackProbe 探测结果
type SlackProbe struct {
	OK        bool   `json:"ok"`
	Status    *int   `json:"status,omitempty"`
	Error     string `json:"error,omitempty"`
	ElapsedMs int64  `json:"elapsedMs,omitempty"`
	Bot       *struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"bot,omitempty"`
	Team *struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"team,omitempty"`
}

// ProbeSlack 使用 auth.test 探测 Slack 连接状态。
func ProbeSlack(token string, timeoutMs int) SlackProbe {
	if timeoutMs <= 0 {
		timeoutMs = 2500
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	client := NewSlackWebClient(token)
	start := time.Now()

	resp, err := client.AuthTest(ctx)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		status200 := 200
		return SlackProbe{
			OK:        false,
			Status:    &status200,
			Error:     err.Error(),
			ElapsedMs: elapsed,
		}
	}

	return SlackProbe{
		OK:        true,
		Status:    intPtr(200),
		ElapsedMs: elapsed,
		Bot: &struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		}{ID: resp.UserID},
		Team: &struct {
			ID   string `json:"id,omitempty"`
			Name string `json:"name,omitempty"`
		}{ID: resp.TeamID},
	}
}

func intPtr(v int) *int { return &v }
