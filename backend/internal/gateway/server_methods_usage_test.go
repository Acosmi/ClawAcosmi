package gateway

import (
	"strings"
	"testing"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// ---------- sessions.usage ----------

func TestUsageHandlers_SessionsUsage(t *testing.T) {
	r := NewMethodRegistry()
	r.RegisterAll(UsageHandlers())

	req := &RequestFrame{Method: "sessions.usage", Params: map[string]interface{}{
		"startDate": "2025-01-01",
		"endDate":   "2025-01-31",
	}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{Config: &types.OpenAcosmiConfig{}}, respond)
	if !gotOK {
		t.Fatal("sessions.usage should succeed")
	}
	result, ok := gotPayload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", gotPayload)
	}
	// Verify schema shape
	if result["startDate"] != "2025-01-01" {
		t.Errorf("expected startDate=2025-01-01, got %v", result["startDate"])
	}
	if result["endDate"] != "2025-01-31" {
		t.Errorf("expected endDate=2025-01-31, got %v", result["endDate"])
	}
	sessions, ok := result["sessions"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected sessions array, got %T", result["sessions"])
	}
	if len(sessions) != 0 {
		t.Errorf("expected empty sessions, got %d", len(sessions))
	}
	totals, ok := result["totals"].(*usageTotals)
	if !ok {
		t.Fatalf("expected totals, got %T", result["totals"])
	}
	if totals.TotalTokens != 0 {
		t.Errorf("expected totalTokens=0, got %v", totals.TotalTokens)
	}
	agg, ok := result["aggregates"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected aggregates map, got %T", result["aggregates"])
	}
	if _, ok := agg["messages"]; !ok {
		t.Error("aggregates should have messages")
	}
}

func TestUsageHandlers_SessionsUsageDefaultDates(t *testing.T) {
	r := NewMethodRegistry()
	r.RegisterAll(UsageHandlers())

	req := &RequestFrame{Method: "sessions.usage", Params: map[string]interface{}{}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{Config: &types.OpenAcosmiConfig{}}, respond)
	if !gotOK {
		t.Fatal("sessions.usage should succeed with default dates")
	}
	result := gotPayload.(map[string]interface{})
	// Should have today's date & 29 days ago
	endDate := result["endDate"].(string)
	today := time.Now().UTC().Format("2006-01-02")
	if endDate != today {
		t.Errorf("expected endDate=%s, got %s", today, endDate)
	}
}

// ---------- sessions.usage.timeseries ----------

func TestUsageHandlers_Timeseries(t *testing.T) {
	r := NewMethodRegistry()
	r.RegisterAll(UsageHandlers())

	req := &RequestFrame{Method: "sessions.usage.timeseries", Params: map[string]interface{}{
		"key": "test-session-123",
	}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{}, respond)
	if !gotOK {
		t.Fatal("sessions.usage.timeseries should succeed")
	}
	result := gotPayload.(map[string]interface{})
	points, ok := result["points"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected points array, got %T", result["points"])
	}
	if len(points) != 0 {
		t.Errorf("expected empty points, got %d", len(points))
	}
}

func TestUsageHandlers_TimeseriesMissingKey(t *testing.T) {
	r := NewMethodRegistry()
	r.RegisterAll(UsageHandlers())

	req := &RequestFrame{Method: "sessions.usage.timeseries", Params: map[string]interface{}{}}
	var gotOK bool
	var gotErr *ErrorShape
	respond := func(ok bool, _ interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{}, respond)
	if gotOK {
		t.Error("should fail without key")
	}
	if gotErr == nil || !strings.Contains(gotErr.Message, "key is required") {
		t.Errorf("expected key required error, got %v", gotErr)
	}
}

// ---------- sessions.usage.logs ----------

func TestUsageHandlers_Logs(t *testing.T) {
	r := NewMethodRegistry()
	r.RegisterAll(UsageHandlers())

	req := &RequestFrame{Method: "sessions.usage.logs", Params: map[string]interface{}{
		"key":   "test-session-456",
		"limit": float64(500),
	}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{}, respond)
	if !gotOK {
		t.Fatal("sessions.usage.logs should succeed")
	}
	result := gotPayload.(map[string]interface{})
	logs, ok := result["logs"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected logs array, got %T", result["logs"])
	}
	if len(logs) != 0 {
		t.Errorf("expected empty logs, got %d", len(logs))
	}
}
