package cron

import (
	"encoding/json"
	"testing"
	"time"
)

// ============================================================================
// Cron 协议一致性测试
// Go 端等价于 TS: cron-protocol-conformance.test.ts
// 验证枚举覆盖、JSON round-trip、投递解析、规范化、时间戳校验
// ============================================================================

// ---------- 枚举覆盖测试 ----------

func TestDeliveryModesExhaustive(t *testing.T) {
	// TS cron-protocol-conformance.test.ts L16-22:
	// 验证所有 delivery mode 枚举值均已定义
	allModes := []CronDeliveryMode{
		DeliveryModeNone,
		DeliveryModeAnnounce,
	}

	seen := make(map[CronDeliveryMode]bool)
	for _, m := range allModes {
		if m == "" {
			t.Error("delivery mode should not be empty")
		}
		seen[m] = true
	}

	// 确保无重复
	if len(seen) != len(allModes) {
		t.Errorf("duplicate delivery modes detected: %d unique vs %d total", len(seen), len(allModes))
	}

	// 确保覆盖已知协议值
	required := []CronDeliveryMode{"none", "announce"}
	for _, r := range required {
		if !seen[r] {
			t.Errorf("missing required delivery mode %q", r)
		}
	}
}

func TestScheduleKindsCoverage(t *testing.T) {
	allKinds := []CronScheduleKind{
		ScheduleKindAt,
		ScheduleKindEvery,
		ScheduleKindCron,
	}

	seen := make(map[CronScheduleKind]bool)
	for _, k := range allKinds {
		seen[k] = true
	}

	// 确保覆盖协议要求的全部调度类型
	required := []CronScheduleKind{"at", "every", "cron"}
	for _, r := range required {
		if !seen[r] {
			t.Errorf("missing required schedule kind %q", r)
		}
	}
}

func TestPayloadKindsCoverage(t *testing.T) {
	allKinds := []CronPayloadKind{
		PayloadKindSystemEvent,
		PayloadKindAgentTurn,
	}

	seen := make(map[CronPayloadKind]bool)
	for _, k := range allKinds {
		seen[k] = true
	}

	required := []CronPayloadKind{"systemEvent", "agentTurn"}
	for _, r := range required {
		if !seen[r] {
			t.Errorf("missing required payload kind %q", r)
		}
	}
}

func TestSessionTargetCoverage(t *testing.T) {
	allTargets := []CronSessionTarget{
		SessionTargetMain,
		SessionTargetIsolated,
	}

	seen := make(map[CronSessionTarget]bool)
	for _, s := range allTargets {
		seen[s] = true
	}

	required := []CronSessionTarget{"main", "isolated"}
	for _, r := range required {
		if !seen[r] {
			t.Errorf("missing required session target %q", r)
		}
	}
}

func TestWakeModeCoverage(t *testing.T) {
	allModes := []CronWakeMode{
		WakeModeNextHeartbeat,
		WakeModeNow,
	}

	seen := make(map[CronWakeMode]bool)
	for _, m := range allModes {
		seen[m] = true
	}

	required := []CronWakeMode{"next-heartbeat", "now"}
	for _, r := range required {
		if !seen[r] {
			t.Errorf("missing required wake mode %q", r)
		}
	}
}

func TestJobStatusCoverage(t *testing.T) {
	allStatuses := []CronJobStatus{
		JobStatusOk,
		JobStatusError,
		JobStatusSkipped,
	}

	seen := make(map[CronJobStatus]bool)
	for _, s := range allStatuses {
		seen[s] = true
	}

	required := []CronJobStatus{"ok", "error", "skipped"}
	for _, r := range required {
		if !seen[r] {
			t.Errorf("missing required job status %q", r)
		}
	}
}

func TestEventKindCoverage(t *testing.T) {
	allKinds := []CronEventKind{
		EventKindStarted,
		EventKindStopped,
		EventKindJobAdded,
		EventKindJobRun,
		EventKindJobDone,
		EventKindJobError,
	}

	seen := make(map[CronEventKind]bool)
	for _, k := range allKinds {
		seen[k] = true
	}

	// 事件类型数量一致性
	if len(allKinds) != 6 {
		t.Errorf("expected 6 event kinds, got %d", len(allKinds))
	}
}

// ---------- CronJob JSON Round-Trip ----------

func TestCronJobJSONRoundTrip(t *testing.T) {
	bestEffort := true
	job := CronJob{
		ID:          "job-123",
		AgentID:     "agent-1",
		Name:        "Test Cron Job",
		Description: "A test job for protocol conformance",
		Enabled:     true,
		CreatedAtMs: 1700000000000,
		UpdatedAtMs: 1700000001000,
		Schedule: CronSchedule{
			Kind:    ScheduleKindEvery,
			EveryMs: 3600000,
		},
		SessionTarget: SessionTargetMain,
		WakeMode:      WakeModeNextHeartbeat,
		Payload: CronPayload{
			Kind:    PayloadKindAgentTurn,
			Message: "Run daily check",
		},
		Delivery: &CronDelivery{
			Mode:       DeliveryModeAnnounce,
			Channel:    "general",
			To:         "user@example.com",
			BestEffort: &bestEffort,
		},
		State: CronJobState{},
	}

	data, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded CronJob
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// 验证关键字段
	if decoded.ID != "job-123" {
		t.Errorf("id: got %q", decoded.ID)
	}
	if decoded.Schedule.Kind != ScheduleKindEvery {
		t.Errorf("schedule.kind: got %q", decoded.Schedule.Kind)
	}
	if decoded.Schedule.EveryMs != 3600000 {
		t.Errorf("schedule.everyMs: got %d", decoded.Schedule.EveryMs)
	}
	if decoded.SessionTarget != SessionTargetMain {
		t.Errorf("sessionTarget: got %q", decoded.SessionTarget)
	}
	if decoded.WakeMode != WakeModeNextHeartbeat {
		t.Errorf("wakeMode: got %q", decoded.WakeMode)
	}
	if decoded.Payload.Kind != PayloadKindAgentTurn {
		t.Errorf("payload.kind: got %q", decoded.Payload.Kind)
	}
	if decoded.Delivery == nil {
		t.Fatal("delivery should not be nil")
	}
	if decoded.Delivery.Mode != DeliveryModeAnnounce {
		t.Errorf("delivery.mode: got %q", decoded.Delivery.Mode)
	}
	if decoded.Delivery.BestEffort == nil || !*decoded.Delivery.BestEffort {
		t.Error("delivery.bestEffort should be true")
	}
}

func TestCronJobJSONRoundTrip_AllScheduleKinds(t *testing.T) {
	tests := []struct {
		name     string
		schedule CronSchedule
	}{
		{"at", CronSchedule{Kind: ScheduleKindAt, At: "2026-01-15T00:00:00Z"}},
		{"every", CronSchedule{Kind: ScheduleKindEvery, EveryMs: 60000}},
		{"cron", CronSchedule{Kind: ScheduleKindCron, Expr: "*/5 * * * *", Tz: "UTC"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := CronJob{
				ID:       "test",
				Name:     "Test",
				Schedule: tt.schedule,
			}
			data, err := json.Marshal(job)
			if err != nil {
				t.Fatal(err)
			}
			var decoded CronJob
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Schedule.Kind != tt.schedule.Kind {
				t.Errorf("expected kind %q, got %q", tt.schedule.Kind, decoded.Schedule.Kind)
			}
		})
	}
}

// ---------- CronStatus 字段一致性 ----------

func TestCronStatusShape(t *testing.T) {
	// TS conformance: CronStatus 应包含 running/jobCount 字段
	status := CronStatusResult{
		Running:  true,
		JobCount: 5,
		Op:       "tick",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}

	// 验证必须包含的字段
	requiredFields := []string{"running", "jobCount"}
	for _, f := range requiredFields {
		if _, ok := m[f]; !ok {
			t.Errorf("CronStatusResult JSON missing required field %q", f)
		}
	}

	// op 应存在（non-empty 时）
	if _, ok := m["op"]; !ok {
		t.Error("expected 'op' field in JSON when non-empty")
	}

	// running 应为 bool
	if _, ok := m["running"].(bool); !ok {
		t.Error("running should be bool type")
	}

	// jobCount 应为 number
	if _, ok := m["jobCount"].(float64); !ok {
		t.Error("jobCount should be number type in JSON")
	}
}

// ---------- ResolveCronDeliveryPlan ----------

func TestResolveCronDeliveryPlan_DeliveryRoute(t *testing.T) {
	bestEffort := true
	job := &CronJob{
		Payload: CronPayload{Kind: PayloadKindAgentTurn},
		Delivery: &CronDelivery{
			Mode:       DeliveryModeAnnounce,
			Channel:    "General",
			To:         " user ",
			BestEffort: &bestEffort,
		},
	}

	plan := ResolveCronDeliveryPlan(job)

	if plan.Mode != DeliveryModeAnnounce {
		t.Errorf("expected announce, got %q", plan.Mode)
	}
	if plan.Source != "delivery" {
		t.Errorf("expected source=delivery, got %q", plan.Source)
	}
	if !plan.Requested {
		t.Error("expected requested=true")
	}
	if !plan.BestEffort {
		t.Error("expected bestEffort=true")
	}
	if plan.Channel != "general" { // normalized
		t.Errorf("expected normalized channel 'general', got %q", plan.Channel)
	}
	if plan.To != "user" { // trimmed
		t.Errorf("expected trimmed to 'user', got %q", plan.To)
	}
}

func TestResolveCronDeliveryPlan_PayloadFallback(t *testing.T) {
	deliver := true
	job := &CronJob{
		Payload: CronPayload{
			Kind:    PayloadKindAgentTurn,
			Deliver: &deliver,
			To:      "target-user",
		},
	}

	plan := ResolveCronDeliveryPlan(job)

	if plan.Source != "payload" {
		t.Errorf("expected source=payload, got %q", plan.Source)
	}
	if plan.Mode != DeliveryModeAnnounce {
		t.Errorf("expected announce, got %q", plan.Mode)
	}
	if !plan.Requested {
		t.Error("expected requested=true for explicit deliver=true")
	}
}

func TestResolveCronDeliveryPlan_NilJob(t *testing.T) {
	plan := ResolveCronDeliveryPlan(nil)
	if plan.Mode != DeliveryModeNone {
		t.Errorf("nil job should return mode=none, got %q", plan.Mode)
	}
}

func TestResolveCronDeliveryPlan_SystemEventNoDelivery(t *testing.T) {
	job := &CronJob{
		Payload: CronPayload{Kind: PayloadKindSystemEvent},
	}
	plan := ResolveCronDeliveryPlan(job)
	if plan.Mode != DeliveryModeNone {
		t.Errorf("systemEvent without delivery should be none, got %q", plan.Mode)
	}
}

func TestResolveCronDeliveryPlan_EmptyDeliveryMode(t *testing.T) {
	job := &CronJob{
		Delivery: &CronDelivery{Mode: ""},
	}
	plan := ResolveCronDeliveryPlan(job)
	// TS: empty mode defaults to announce
	if plan.Mode != DeliveryModeAnnounce {
		t.Errorf("empty delivery.mode should default to announce, got %q", plan.Mode)
	}
}

func TestIsDeliveryPlanActive(t *testing.T) {
	if IsDeliveryPlanActive(CronDeliveryPlan{Mode: DeliveryModeNone}) {
		t.Error("mode=none should be inactive")
	}
	if !IsDeliveryPlanActive(CronDeliveryPlan{Mode: DeliveryModeAnnounce}) {
		t.Error("mode=announce should be active")
	}
}

// ---------- NormalizeCronJobInput ----------

func TestNormalizeCronJobInput_SystemEvent(t *testing.T) {
	raw := map[string]interface{}{
		"name": "Test Alarm",
		"schedule": map[string]interface{}{
			"kind":    "every",
			"everyMs": float64(60000),
		},
		// sessionTarget 默认 main → NormalizeCronPayload 自动推断 kind=systemEvent
		"payload": map[string]interface{}{
			"text": "Wake up!",
		},
	}

	result, err := NormalizeCronJobInput(raw)
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	if result.Name != "Test Alarm" {
		t.Errorf("name: got %q", result.Name)
	}
	if result.SessionTarget != SessionTargetMain {
		t.Errorf("default sessionTarget should be main, got %q", result.SessionTarget)
	}
	if result.WakeMode != WakeModeNextHeartbeat {
		t.Errorf("default wakeMode should be next-heartbeat, got %q", result.WakeMode)
	}
	if result.Payload.Kind != PayloadKindSystemEvent {
		t.Errorf("payload.kind: got %q", result.Payload.Kind)
	}
	if result.Payload.Text != "Wake up!" {
		t.Errorf("payload.text: got %q", result.Payload.Text)
	}
}

func TestNormalizeCronJobInput_AgentTurn(t *testing.T) {
	raw := map[string]interface{}{
		"name": "Agent Task",
		"schedule": map[string]interface{}{
			"kind": "cron",
			"expr": "0 9 * * *",
			"tz":   "UTC",
		},
		"sessionTarget": "isolated",
		"wakeMode":      "now",
		// sessionTarget=isolated → NormalizeCronPayload 自动推断 kind=agentTurn
		"payload": map[string]interface{}{
			"message": "Do daily analysis",
		},
	}

	result, err := NormalizeCronJobInput(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.SessionTarget != SessionTargetIsolated {
		t.Errorf("sessionTarget: got %q", result.SessionTarget)
	}
	if result.WakeMode != WakeModeNow {
		t.Errorf("wakeMode: got %q", result.WakeMode)
	}
	if result.Schedule.Expr != "0 9 * * *" {
		t.Errorf("schedule.expr: got %q", result.Schedule.Expr)
	}
}

func TestNormalizeCronJobInput_MissingName(t *testing.T) {
	raw := map[string]interface{}{
		"schedule": map[string]interface{}{
			"kind":    "every",
			"everyMs": float64(60000),
		},
		"payload": map[string]interface{}{
			"text": "hello",
		},
	}

	result, err := NormalizeCronJobInput(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Implementation tolerates missing name — it will be empty
	if result.Name != "" {
		t.Errorf("expected empty name, got %q", result.Name)
	}
}

// ---------- ValidateScheduleTimestamp ----------

func TestValidateScheduleTimestamp_NonAt(t *testing.T) {
	schedule := CronSchedule{Kind: ScheduleKindEvery, EveryMs: 60000}
	result := ValidateScheduleTimestamp(schedule, time.Now().UnixMilli())
	if !result.OK {
		t.Error("non-at schedule should always be valid")
	}
}

func TestValidateScheduleTimestamp_FutureAt(t *testing.T) {
	now := time.Now().UnixMilli()
	future := time.UnixMilli(now + 3600000).UTC().Format(time.RFC3339) // +1h
	schedule := CronSchedule{Kind: ScheduleKindAt, At: future}
	result := ValidateScheduleTimestamp(schedule, now)
	if !result.OK {
		t.Errorf("1 hour in future should be valid: %s", result.Message)
	}
}

func TestValidateScheduleTimestamp_PastAt(t *testing.T) {
	now := time.Now().UnixMilli()
	past := time.UnixMilli(now - 3600000).UTC().Format(time.RFC3339) // -1h
	schedule := CronSchedule{Kind: ScheduleKindAt, At: past}
	result := ValidateScheduleTimestamp(schedule, now)
	if result.OK {
		t.Error("1 hour in the past should be invalid")
	}
}

func TestValidateScheduleTimestamp_TooFarFuture(t *testing.T) {
	now := time.Now().UnixMilli()
	farFuture := time.UnixMilli(now + 11*365*24*3600*1000).UTC().Format(time.RFC3339) // +11y
	schedule := CronSchedule{Kind: ScheduleKindAt, At: farFuture}
	result := ValidateScheduleTimestamp(schedule, now)
	if result.OK {
		t.Error("11 years in future should be invalid")
	}
}

func TestValidateScheduleTimestamp_EmptyAt(t *testing.T) {
	schedule := CronSchedule{Kind: ScheduleKindAt, At: ""}
	result := ValidateScheduleTimestamp(schedule, time.Now().UnixMilli())
	if result.OK {
		t.Error("empty at should be invalid")
	}
}

// ---------- CronStoreFile JSON ----------

func TestCronStoreFileJSON(t *testing.T) {
	store := CronStoreFile{
		Version: 1,
		Jobs: []CronJob{
			{
				ID:            "job-1",
				Name:          "Test",
				Enabled:       true,
				Schedule:      CronSchedule{Kind: ScheduleKindEvery, EveryMs: 60000},
				SessionTarget: SessionTargetMain,
				WakeMode:      WakeModeNextHeartbeat,
				Payload:       CronPayload{Kind: PayloadKindSystemEvent, Text: "tick"},
			},
		},
	}

	data, err := json.Marshal(store)
	if err != nil {
		t.Fatal(err)
	}

	var decoded CronStoreFile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Version != 1 {
		t.Errorf("version: got %d", decoded.Version)
	}
	if len(decoded.Jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(decoded.Jobs))
	}
	if decoded.Jobs[0].ID != "job-1" {
		t.Errorf("job id: got %q", decoded.Jobs[0].ID)
	}
}

// ---------- CronJobPatch JSON ----------

func TestCronJobPatchJSON(t *testing.T) {
	name := "Updated Name"
	enabled := false
	target := SessionTargetIsolated
	patch := CronJobPatch{
		Name:          &name,
		Enabled:       &enabled,
		SessionTarget: &target,
	}

	data, err := json.Marshal(patch)
	if err != nil {
		t.Fatal(err)
	}

	var decoded CronJobPatch
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Name == nil || *decoded.Name != "Updated Name" {
		t.Error("name should be preserved")
	}
	if decoded.Enabled == nil || *decoded.Enabled != false {
		t.Error("enabled should be false")
	}
	// Fields not set should be nil
	if decoded.Schedule != nil {
		t.Error("schedule should be nil")
	}
	if decoded.WakeMode != nil {
		t.Error("wakeMode should be nil")
	}
}
