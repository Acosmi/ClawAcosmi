package tui

import (
	"strings"
	"testing"
)

// ---------- FormatStatusSummary 补全段落测试 ----------

func TestFormatStatusSummaryHeartbeat(t *testing.T) {
	summary := map[string]interface{}{
		"heartbeat": map[string]interface{}{
			"agents": []interface{}{
				map[string]interface{}{
					"agentId": "agent-1",
					"enabled": true,
					"every":   "5m",
				},
				map[string]interface{}{
					"agentId": "agent-2",
					"enabled": false,
				},
			},
		},
	}
	lines := FormatStatusSummary(summary)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Heartbeat:") {
		t.Error("missing Heartbeat line")
	}
	if !strings.Contains(joined, "5m (agent-1)") {
		t.Error("missing enabled agent format")
	}
	if !strings.Contains(joined, "disabled (agent-2)") {
		t.Error("missing disabled agent format")
	}
}

func TestFormatStatusSummarySessionPaths(t *testing.T) {
	// 单路径
	summary := map[string]interface{}{
		"sessions": map[string]interface{}{
			"paths": []interface{}{"/data/sessions"},
		},
	}
	lines := FormatStatusSummary(summary)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Session store: /data/sessions") {
		t.Errorf("single path: got %s", joined)
	}

	// 多路径
	summary2 := map[string]interface{}{
		"sessions": map[string]interface{}{
			"paths": []interface{}{"/a", "/b", "/c"},
		},
	}
	lines2 := FormatStatusSummary(summary2)
	joined2 := strings.Join(lines2, "\n")

	if !strings.Contains(joined2, "Session stores: 3") {
		t.Errorf("multi path: got %s", joined2)
	}
}

func TestFormatStatusSummaryRecentSessions(t *testing.T) {
	summary := map[string]interface{}{
		"sessions": map[string]interface{}{
			"recent": []interface{}{
				map[string]interface{}{
					"key":             "sess-1",
					"kind":            "chat",
					"age":             float64(120000), // 2 分钟
					"model":           "gpt-4o",
					"totalTokens":     float64(1500),
					"contextTokens":   float64(8000),
					"remainingTokens": float64(6500),
					"percentUsed":     float64(18),
					"flags":           []interface{}{"pinned"},
				},
			},
		},
	}
	lines := FormatStatusSummary(summary)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Recent sessions:") {
		t.Error("missing Recent sessions header")
	}
	if !strings.Contains(joined, "sess-1") {
		t.Error("missing session key")
	}
	if !strings.Contains(joined, "[chat]") {
		t.Error("missing kind label")
	}
	if !strings.Contains(joined, "gpt-4o") {
		t.Error("missing model")
	}
	if !strings.Contains(joined, "flags: pinned") {
		t.Error("missing flags")
	}
}

func TestFormatStatusSummaryQueuedEvents(t *testing.T) {
	summary := map[string]interface{}{
		"queuedSystemEvents": []interface{}{
			"event-a", "event-b", "event-c", "event-d",
		},
	}
	lines := FormatStatusSummary(summary)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Queued system events (4)") {
		t.Errorf("missing queued events count: %s", joined)
	}
	// 只显示前 3 条
	if !strings.Contains(joined, "event-a") {
		t.Error("missing event-a in preview")
	}
	if strings.Contains(joined, "event-d") {
		t.Error("event-d should not appear in preview (max 3)")
	}
}

func TestFormatStatusSummaryLinkChannelAuthAge(t *testing.T) {
	summary := map[string]interface{}{
		"linkChannel": map[string]interface{}{
			"label":     "Discord",
			"linked":    true,
			"authAgeMs": float64(300000), // 5 分钟
		},
	}
	lines := FormatStatusSummary(summary)
	joined := strings.Join(lines, "\n")

	if !strings.Contains(joined, "Discord: linked") {
		t.Error("missing linked label")
	}
	if !strings.Contains(joined, "last refreshed") {
		t.Error("missing authAge in output")
	}
}

// ---------- formatTimeAgo 测试 ----------

func TestFormatTimeAgo(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{"just now", 3000, "just now"},
		{"seconds", 15000, "15s ago"},
		{"minutes", 180000, "3m ago"},
		{"hours", 7200000, "2h ago"},
		{"days", 172800000, "2d ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTimeAgo(tt.ms)
			if got != tt.want {
				t.Errorf("formatTimeAgo(%d): got %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}
