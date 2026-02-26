package input

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// ──────────────────────────────────────────────────────────────
// Mock notifier for testing
// ──────────────────────────────────────────────────────────────

type mockNotifier struct {
	approved       bool
	reason         string
	modifiedParams json.RawMessage
	lastRequest    *ApprovalRequest
	callCount      int
	shouldErr      bool
}

func (m *mockNotifier) RequestApproval(_ context.Context, req ApprovalRequest) (ApprovalResponse, error) {
	m.callCount++
	m.lastRequest = &req
	if m.shouldErr {
		return ApprovalResponse{}, context.DeadlineExceeded
	}
	return ApprovalResponse{
		Approved:       m.approved,
		Reason:         m.reason,
		ModifiedParams: m.modifiedParams,
		ApprovedBy:     "test-human",
	}, nil
}

// ──────────────────────────────────────────────────────────────
// Tests: Risk Classification
// ──────────────────────────────────────────────────────────────

func TestClassifyRisk_ReadOnly(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})

	readOnlyActions := []string{
		"capture_screen", "describe_scene", "locate_element",
		"read_text", "detect_dialog", "watch_for_change",
		"tui_read_prompt", "tui_read_diff", "tui_wait_prompt",
		"scroll", "macos_copy", "macos_select_all", "macos_find",
	}

	for _, action := range readOnlyActions {
		risk := gw.ClassifyRisk(action, nil)
		if risk != RiskLow {
			t.Errorf("ClassifyRisk(%q) = %v, want RiskLow", action, risk)
		}
	}
}

func TestClassifyRisk_WriteOperations(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})

	writeActions := []string{
		"click", "click_element", "tui_respond", "tui_dismiss_dialog",
		"macos_save", "macos_paste", "macos_cut",
	}

	for _, action := range writeActions {
		risk := gw.ClassifyRisk(action, nil)
		if risk != RiskMedium {
			t.Errorf("ClassifyRisk(%q) = %v, want RiskMedium", action, risk)
		}
	}
}

func TestClassifyRisk_DynamicEscalation_SensitiveText(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})

	params, _ := json.Marshal(map[string]string{"text": "sudo rm -rf /"})
	risk := gw.ClassifyRisk("type_text", params)
	if risk != RiskHigh {
		t.Errorf("ClassifyRisk(type_text with sudo) = %v, want RiskHigh", risk)
	}
}

func TestClassifyRisk_DynamicEscalation_CtrlC(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})

	params, _ := json.Marshal(map[string][]uint16{
		"keys": {uint16(KeyControl), uint16(KeyC)},
	})
	risk := gw.ClassifyRisk("press_key", params)
	if risk != RiskHigh {
		t.Errorf("ClassifyRisk(press_key Ctrl+C) = %v, want RiskHigh", risk)
	}
}

func TestClassifyRisk_UnknownAction(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})
	risk := gw.ClassifyRisk("unknown_action", nil)
	if risk != RiskMedium {
		t.Errorf("ClassifyRisk(unknown) = %v, want RiskMedium", risk)
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: CheckAndApprove flow
// ──────────────────────────────────────────────────────────────

func TestCheckAndApprove_LowRisk_AutoApproved(t *testing.T) {
	notifier := &mockNotifier{approved: true}
	gw := NewApprovalGateway(GatewayConfig{
		Enabled:  true,
		Notifier: notifier,
	})

	approved, _, err := gw.CheckAndApprove(context.Background(), "capture_screen", nil, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("low-risk action should be auto-approved")
	}
	if notifier.callCount != 0 {
		t.Error("notifier should NOT be called for low-risk actions")
	}
}

func TestCheckAndApprove_MediumRisk_Approved(t *testing.T) {
	notifier := &mockNotifier{approved: true}
	gw := NewApprovalGateway(GatewayConfig{
		Enabled:  true,
		Notifier: notifier,
	})

	params, _ := json.Marshal(map[string]int{"x": 100, "y": 200})
	approved, _, err := gw.CheckAndApprove(context.Background(), "click", params, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("human approved but gateway returned false")
	}
	if notifier.callCount != 1 {
		t.Errorf("notifier callCount = %d, want 1", notifier.callCount)
	}
	if notifier.lastRequest.RiskLevel != RiskMedium {
		t.Errorf("risk = %v, want RiskMedium", notifier.lastRequest.RiskLevel)
	}
}

func TestCheckAndApprove_MediumRisk_Denied(t *testing.T) {
	notifier := &mockNotifier{approved: false, reason: "looks risky"}
	gw := NewApprovalGateway(GatewayConfig{
		Enabled:  true,
		Notifier: notifier,
	})

	params, _ := json.Marshal(map[string]string{"input": "y"})
	approved, _, err := gw.CheckAndApprove(context.Background(), "tui_respond", params, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Error("human denied but gateway returned true")
	}
}

func TestCheckAndApprove_HighRisk_BlockedByGuardrails(t *testing.T) {
	guardrails := NewActionGuardrails("")
	notifier := &mockNotifier{approved: true}
	gw := NewApprovalGateway(GatewayConfig{
		Guardrails: guardrails,
		Enabled:    true,
		Notifier:   notifier,
	})

	// Cmd+Q is in the blocked hotkeys list.
	params, _ := json.Marshal(map[string][]uint16{
		"keys": {uint16(KeyCommand), uint16(KeyQ)},
	})
	approved, _, err := gw.CheckAndApprove(context.Background(), "hotkey", params, "test", nil)
	if err == nil {
		t.Fatal("expected error for blocked hotkey")
	}
	if approved {
		t.Error("blocked action should not be approved")
	}
	if notifier.callCount != 0 {
		t.Error("notifier should NOT be called for blocked actions")
	}
}

func TestCheckAndApprove_NoNotifier_FailClosed(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{
		Enabled:  true,
		Notifier: nil, // no notifier
	})

	params, _ := json.Marshal(map[string]int{"x": 100, "y": 200})
	approved, _, err := gw.CheckAndApprove(context.Background(), "click", params, "test", nil)
	if err == nil {
		t.Fatal("expected error when no notifier is configured")
	}
	if approved {
		t.Error("should fail-closed without notifier")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: AutoMode
// ──────────────────────────────────────────────────────────────

func TestCheckAndApprove_AutoMode(t *testing.T) {
	notifier := &mockNotifier{approved: true}
	gw := NewApprovalGateway(GatewayConfig{
		Enabled:  true,
		AutoMode: true,
		Notifier: notifier,
	})

	params, _ := json.Marshal(map[string]int{"x": 100, "y": 200})
	approved, _, err := gw.CheckAndApprove(context.Background(), "click", params, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("auto-mode should approve all non-blocked actions")
	}
	if notifier.callCount != 0 {
		t.Error("notifier should NOT be called in auto-mode")
	}
}

func TestAutoMode_Toggle(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})

	if gw.IsAutoMode() {
		t.Error("auto-mode should default to false (privacy-first)")
	}

	gw.SetAutoMode(true)
	if !gw.IsAutoMode() {
		t.Error("SetAutoMode(true) didn't take effect")
	}

	gw.SetAutoMode(false)
	if gw.IsAutoMode() {
		t.Error("SetAutoMode(false) didn't take effect")
	}
}

func TestAutoMode_StillBlocksDangerous(t *testing.T) {
	guardrails := NewActionGuardrails("")
	notifier := &mockNotifier{approved: true}
	gw := NewApprovalGateway(GatewayConfig{
		Guardrails: guardrails,
		Enabled:    true,
		AutoMode:   true,
		Notifier:   notifier,
	})

	// Even in auto-mode, Cmd+Q should be blocked by guardrails.
	params, _ := json.Marshal(map[string][]uint16{
		"keys": {uint16(KeyCommand), uint16(KeyQ)},
	})
	approved, _, err := gw.CheckAndApprove(context.Background(), "hotkey", params, "test", nil)
	if err == nil {
		t.Fatal("expected error for blocked hotkey even in auto-mode")
	}
	if approved {
		t.Error("auto-mode should NOT bypass guardrails blacklist")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Gateway disabled
// ──────────────────────────────────────────────────────────────

func TestCheckAndApprove_GatewayDisabled(t *testing.T) {
	notifier := &mockNotifier{approved: false}
	gw := NewApprovalGateway(GatewayConfig{
		Enabled:  false,
		Notifier: notifier,
	})

	params, _ := json.Marshal(map[string]int{"x": 100, "y": 200})
	approved, _, err := gw.CheckAndApprove(context.Background(), "click", params, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("disabled gateway should auto-approve all actions")
	}
	if notifier.callCount != 0 {
		t.Error("notifier should NOT be called when gateway is disabled")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Modified parameters
// ──────────────────────────────────────────────────────────────

func TestCheckAndApprove_ModifiedParams(t *testing.T) {
	modified, _ := json.Marshal(map[string]string{"input": "n"})
	notifier := &mockNotifier{
		approved:       true,
		modifiedParams: modified,
	}
	gw := NewApprovalGateway(GatewayConfig{
		Enabled:  true,
		Notifier: notifier,
	})

	params, _ := json.Marshal(map[string]string{"input": "y"})
	approved, newParams, err := gw.CheckAndApprove(context.Background(), "tui_respond", params, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("should be approved")
	}
	if newParams == nil {
		t.Fatal("expected modified params")
	}
	var result map[string]string
	json.Unmarshal(newParams, &result)
	if result["input"] != "n" {
		t.Errorf("modified input = %q, want %q", result["input"], "n")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Timeout / notifier error
// ──────────────────────────────────────────────────────────────

func TestCheckAndApprove_NotifierError(t *testing.T) {
	notifier := &mockNotifier{shouldErr: true}
	gw := NewApprovalGateway(GatewayConfig{
		Enabled:  true,
		Notifier: notifier,
		Timeout:  100 * time.Millisecond,
	})

	params, _ := json.Marshal(map[string]int{"x": 100, "y": 200})
	approved, _, err := gw.CheckAndApprove(context.Background(), "click", params, "test", nil)
	if err == nil {
		t.Fatal("expected error on notifier failure")
	}
	if approved {
		t.Error("should not be approved on notifier error")
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Description generation
// ──────────────────────────────────────────────────────────────

func TestBuildDescription(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})

	tests := []struct {
		action string
		params string
		want   string
	}{
		{"click", `{"x":100,"y":200}`, "点击坐标: (100, 200)"},
		{"click_element", `{"target":"确定"}`, `点击元素: "确定"`},
		{"type_text", `{"text":"hello"}`, `输入文本: "hello"`},
		{"tui_respond", `{"input":"y"}`, `回应终端提示: "y"`},
		{"tui_send_keys", `{"keys":[59,8]}`, "发送快捷键: [59 8]"},
		{"tui_run_command", `{"command":"git status"}`, `执行终端命令: "git status"`},
		{"tui_dismiss_dialog", `{}`, "关闭系统对话框/弹窗"},
		{"macos_save", `{}`, "保存 (⌘S)"},
		{"macos_copy", `{}`, "复制 (⌘C)"},
		{"unknown_action", `{}`, "执行操作: unknown_action"},
	}

	for _, tt := range tests {
		desc := gw.buildDescription(tt.action, json.RawMessage(tt.params))
		if desc != tt.want {
			t.Errorf("buildDescription(%q) = %q, want %q", tt.action, desc, tt.want)
		}
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: New TUI tools risk classification
// ──────────────────────────────────────────────────────────────

func TestClassifyRisk_TuiSendKeys_CtrlC_High(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})
	params, _ := json.Marshal(map[string][]uint16{
		"keys": {uint16(KeyControl), uint16(KeyC)},
	})
	risk := gw.ClassifyRisk("tui_send_keys", params)
	if risk != RiskHigh {
		t.Errorf("ClassifyRisk(tui_send_keys Ctrl+C) = %v, want RiskHigh", risk)
	}
}

func TestClassifyRisk_TuiSendKeys_Normal_Medium(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})
	params, _ := json.Marshal(map[string][]uint16{
		"keys": {uint16(KeyArrowDown)},
	})
	risk := gw.ClassifyRisk("tui_send_keys", params)
	if risk != RiskMedium {
		t.Errorf("ClassifyRisk(tui_send_keys ArrowDown) = %v, want RiskMedium", risk)
	}
}

func TestClassifyRisk_TuiRunCommand_Sudo_High(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})
	params, _ := json.Marshal(map[string]string{"command": "sudo apt install foo"})
	risk := gw.ClassifyRisk("tui_run_command", params)
	if risk != RiskHigh {
		t.Errorf("ClassifyRisk(tui_run_command with sudo) = %v, want RiskHigh", risk)
	}
}

func TestClassifyRisk_TuiRunCommand_Normal_Medium(t *testing.T) {
	gw := NewApprovalGateway(GatewayConfig{Enabled: true})
	params, _ := json.Marshal(map[string]string{"command": "git status"})
	risk := gw.ClassifyRisk("tui_run_command", params)
	if risk != RiskMedium {
		t.Errorf("ClassifyRisk(tui_run_command git status) = %v, want RiskMedium", risk)
	}
}

// ──────────────────────────────────────────────────────────────
// Tests: Per-action auto-approve whitelist
// ──────────────────────────────────────────────────────────────

func TestPerActionAutoApprove(t *testing.T) {
	notifier := &mockNotifier{approved: true}
	gw := NewApprovalGateway(GatewayConfig{
		Enabled:         true,
		Notifier:        notifier,
		AutoApproveList: []string{"macos_save"},
	})

	// macos_save is whitelisted — should not call notifier.
	approved, _, err := gw.CheckAndApprove(context.Background(), "macos_save", nil, "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Error("whitelisted action should be auto-approved")
	}
	if notifier.callCount != 0 {
		t.Error("notifier should NOT be called for whitelisted actions")
	}

	// click is NOT whitelisted — should call notifier.
	params, _ := json.Marshal(map[string]int{"x": 100, "y": 200})
	gw.CheckAndApprove(context.Background(), "click", params, "test", nil)
	if notifier.callCount != 1 {
		t.Errorf("notifier callCount = %d, want 1", notifier.callCount)
	}
}
