package gateway

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------- Escalation Request ----------

func TestEscalationRequest_Success(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	bc := NewBroadcaster()
	auditLogger := NewEscalationAuditLogger()
	mgr := NewEscalationManager(bc, auditLogger, nil)
	defer mgr.Close()

	err := mgr.RequestEscalation("esc_001", "full", "Need write access", "run-1", "session-1", "", "", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status := mgr.GetStatus()
	if !status.HasPending {
		t.Error("expected hasPending=true")
	}
	if status.Pending == nil || status.Pending.ID != "esc_001" {
		t.Error("pending request ID mismatch")
	}
	if status.Pending.RequestedLevel != "full" {
		t.Errorf("expected level 'full', got %q", status.Pending.RequestedLevel)
	}
}

func TestEscalationRequest_DuplicateRejected(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	if err := mgr.RequestEscalation("esc_001", "full", "reason", "", "", "", "", 30); err != nil {
		t.Fatalf("first request should succeed: %v", err)
	}
	if err := mgr.RequestEscalation("esc_002", "full", "reason", "", "", "", "", 30); err == nil {
		t.Error("duplicate request should fail")
	}
}

func TestEscalationRequest_InvalidLevel(t *testing.T) {
	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	if err := mgr.RequestEscalation("esc_001", "invalid", "reason", "", "", "", "", 30); err == nil {
		t.Error("invalid level should fail")
	}
}

// ---------- Escalation Approve ----------

func TestEscalationApprove(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	mgr.RequestEscalation("esc_001", "full", "reason", "run-1", "", "", "", 30)

	if err := mgr.ResolveEscalation(true, 15); err != nil {
		t.Fatalf("approve should succeed: %v", err)
	}

	status := mgr.GetStatus()
	if status.HasPending {
		t.Error("pending should be cleared after approve")
	}
	if !status.HasActive {
		t.Error("expected hasActive=true after approve")
	}
	if status.Active.Level != "full" {
		t.Errorf("expected level 'full', got %q", status.Active.Level)
	}
	if status.ActiveLevel != "full" {
		t.Errorf("expected activeLevel 'full', got %q", status.ActiveLevel)
	}
}

// ---------- Escalation Deny ----------

func TestEscalationDeny(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	mgr.RequestEscalation("esc_001", "allowlist", "reason", "", "", "", "", 30)

	if err := mgr.ResolveEscalation(false, 0); err != nil {
		t.Fatalf("deny should succeed: %v", err)
	}

	status := mgr.GetStatus()
	if status.HasPending {
		t.Error("pending should be cleared after deny")
	}
	if status.HasActive {
		t.Error("should NOT have active grant after deny")
	}
}

// ---------- Auto De-escalate (TTL) ----------

func TestEscalationAutoDeescalate(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	mgr.RequestEscalation("esc_001", "full", "reason", "", "", "", "", 1) // 1 minute TTL

	// Override TTL to very short for testing
	mgr.mu.Lock()
	mgr.pending = nil
	mgr.active = &ActiveEscalationGrant{
		ID:        "esc_001",
		Level:     "full",
		GrantedAt: time.Now(),
		ExpiresAt: time.Now().Add(100 * time.Millisecond),
	}
	mgr.startDeescalateTimerLocked(100 * time.Millisecond)
	mgr.mu.Unlock()

	// Wait for auto-deescalation
	time.Sleep(300 * time.Millisecond)

	status := mgr.GetStatus()
	if status.HasActive {
		t.Error("should not have active grant after TTL expiry")
	}
}

// ---------- Task Complete ----------

func TestEscalationTaskComplete(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	mgr.RequestEscalation("esc_001", "full", "reason", "run-1", "", "", "", 30)
	mgr.ResolveEscalation(true, 30)

	// Task complete with matching runID
	mgr.TaskComplete("run-1")

	status := mgr.GetStatus()
	if status.HasActive {
		t.Error("should not have active grant after task complete")
	}
}

func TestEscalationTaskComplete_WrongRunID(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	mgr.RequestEscalation("esc_001", "full", "reason", "run-1", "", "", "", 30)
	mgr.ResolveEscalation(true, 30)

	// Task complete with wrong runID should NOT deescalate
	mgr.TaskComplete("run-other")

	status := mgr.GetStatus()
	if !status.HasActive {
		t.Error("should still have active grant (wrong runID)")
	}
}

// ---------- Audit Logger ----------

func TestEscalationAuditLog(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	logger := NewEscalationAuditLogger()

	logger.Log(EscalationAuditEntry{
		Timestamp:      time.Now(),
		Event:          AuditEventRequest,
		RequestID:      "esc_001",
		RequestedLevel: "full",
		Reason:         "test reason",
		TTLMinutes:     30,
	})
	logger.Log(EscalationAuditEntry{
		Timestamp:      time.Now(),
		Event:          AuditEventApprove,
		RequestID:      "esc_001",
		RequestedLevel: "full",
		TTLMinutes:     30,
	})

	entries, err := logger.ReadRecent(10)
	if err != nil {
		t.Fatalf("read recent failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	// Most recent first
	if entries[0].Event != AuditEventApprove {
		t.Errorf("expected first entry to be 'approve', got %q", entries[0].Event)
	}
	if entries[1].Event != AuditEventRequest {
		t.Errorf("expected second entry to be 'request', got %q", entries[1].Event)
	}

	// Verify file exists
	auditFile := filepath.Join(tmpHome, ".openacosmi", "escalation-audit.log")
	if _, err := os.Stat(auditFile); os.IsNotExist(err) {
		t.Error("audit log file should exist")
	}
}

// ---------- Gateway Method Handlers ----------

func TestEscalationHandlers_RequestAndStatus(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	bc := NewBroadcaster()
	auditLogger := NewEscalationAuditLogger()
	mgr := NewEscalationManager(bc, auditLogger, nil)
	defer mgr.Close()

	r := NewMethodRegistry()
	r.RegisterAll(EscalationHandlers())

	// 1. Request escalation
	req := &RequestFrame{Method: "security.escalation.request", Params: map[string]interface{}{
		"level":      "full",
		"reason":     "Need full access for deployment",
		"ttlMinutes": float64(15),
	}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{EscalationMgr: mgr}, respond)
	if !gotOK {
		t.Fatal("request should succeed")
	}
	result := gotPayload.(map[string]interface{})
	if result["status"] != "pending" {
		t.Errorf("expected status 'pending', got %v", result["status"])
	}
	id, _ := result["id"].(string)
	if !strings.HasPrefix(id, "esc_") {
		t.Errorf("expected ID prefix 'esc_', got %q", id)
	}

	// 2. Check status
	req = &RequestFrame{Method: "security.escalation.status", Params: map[string]interface{}{}}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{EscalationMgr: mgr}, respond)
	if !gotOK {
		t.Fatal("status should succeed")
	}
	statusResult := gotPayload.(EscalationStatus)
	if !statusResult.HasPending {
		t.Error("expected hasPending=true")
	}
}

func TestEscalationHandlers_Resolve(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	r := NewMethodRegistry()
	r.RegisterAll(EscalationHandlers())

	// Setup: create a pending request
	mgr.RequestEscalation("esc_001", "full", "reason", "", "", "", "", 30)

	// Approve
	req := &RequestFrame{Method: "security.escalation.resolve", Params: map[string]interface{}{
		"approve":    true,
		"ttlMinutes": float64(30),
	}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{EscalationMgr: mgr}, respond)
	if !gotOK {
		t.Fatal("resolve should succeed")
	}
	result := gotPayload.(map[string]interface{})
	if result["status"] != "approved" {
		t.Errorf("expected status 'approved', got %v", result["status"])
	}

	// Verify active grant
	status := mgr.GetStatus()
	if !status.HasActive {
		t.Error("expected active grant after approval")
	}
}

func TestEscalationHandlers_Revoke(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	r := NewMethodRegistry()
	r.RegisterAll(EscalationHandlers())

	// Setup: request + approve
	mgr.RequestEscalation("esc_001", "full", "reason", "", "", "", "", 30)
	mgr.ResolveEscalation(true, 30)

	// Revoke
	req := &RequestFrame{Method: "security.escalation.revoke", Params: map[string]interface{}{}}
	var gotOK bool
	respond := func(ok bool, _ interface{}, _ *ErrorShape) {
		gotOK = ok
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{EscalationMgr: mgr}, respond)
	if !gotOK {
		t.Fatal("revoke should succeed")
	}

	status := mgr.GetStatus()
	if status.HasActive {
		t.Error("should not have active grant after revoke")
	}
}

func TestEscalationHandlers_ResolveNoPending(t *testing.T) {
	mgr := NewEscalationManager(nil, nil, nil)
	defer mgr.Close()

	r := NewMethodRegistry()
	r.RegisterAll(EscalationHandlers())

	req := &RequestFrame{Method: "security.escalation.resolve", Params: map[string]interface{}{
		"approve": true,
	}}
	var gotOK bool
	var gotErr *ErrorShape
	respond := func(ok bool, _ interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{EscalationMgr: mgr}, respond)
	if gotOK {
		t.Error("resolve without pending should fail")
	}
	if gotErr == nil || gotErr.Code != ErrCodeBadRequest {
		t.Errorf("expected bad_request error, got %v", gotErr)
	}
}
