package input

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// approvalSeq is a monotonic counter for generating unique approval IDs.
var approvalSeq atomic.Int64

// newApprovalID generates a unique, sortable approval ID.
func newApprovalID() string {
	seq := approvalSeq.Add(1)
	return fmt.Sprintf("ap-%d-%04d", time.Now().UnixMilli(), seq)
}

// ──────────────────────────────────────────────────────────────
// Risk levels
// ──────────────────────────────────────────────────────────────

// ActionRiskLevel classifies the risk of an input action.
type ActionRiskLevel int

const (
	// RiskLow — read-only operations, auto-approved.
	RiskLow ActionRiskLevel = iota
	// RiskMedium — write operations, require human confirmation.
	RiskMedium
	// RiskHigh — destructive / privileged operations, require
	// double confirmation with a screenshot attached.
	RiskHigh
)

func (r ActionRiskLevel) String() string {
	switch r {
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	default:
		return "unknown"
	}
}

// ──────────────────────────────────────────────────────────────
// Approval request / response types
// ──────────────────────────────────────────────────────────────

// ApprovalRequest describes an action awaiting human confirmation.
type ApprovalRequest struct {
	ID          string          `json:"id"`
	ActionType  string          `json:"action_type"`
	Description string          `json:"description"`
	RiskLevel   ActionRiskLevel `json:"risk_level"`
	Context     *ScreenContext  `json:"context,omitempty"`
	Params      json.RawMessage `json:"params"`
	Timestamp   float64         `json:"timestamp"`
}

// ScreenContext provides visual context for the approval reviewer.
type ScreenContext struct {
	SceneDescription string `json:"scene_description,omitempty"`
	ScreenshotB64    string `json:"screenshot_b64,omitempty"`
}

// ApprovalResponse carries the human reviewer's decision.
type ApprovalResponse struct {
	Approved       bool            `json:"approved"`
	Reason         string          `json:"reason,omitempty"`
	ModifiedParams json.RawMessage `json:"modified_params,omitempty"`
	ApprovedBy     string          `json:"approved_by,omitempty"`
}

// ApprovalNotifier is the interface that an external system (OpenClaw)
// implements to relay approval requests to a human and return results.
type ApprovalNotifier interface {
	// RequestApproval blocks until the human responds or the context
	// is cancelled / times out.
	RequestApproval(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error)
}

// ──────────────────────────────────────────────────────────────
// Risk classification rules
// ──────────────────────────────────────────────────────────────

// actionRiskRules is the static risk classification table.
// Actions not listed here default to RiskMedium.
var actionRiskRules = map[string]ActionRiskLevel{
	// Read-only — auto-approved
	"capture_screen":   RiskLow,
	"describe_scene":   RiskLow,
	"locate_element":   RiskLow,
	"read_text":        RiskLow,
	"detect_dialog":    RiskLow,
	"watch_for_change": RiskLow,
	"tui_read_prompt":  RiskLow,
	"tui_read_diff":    RiskLow,
	"tui_wait_prompt":  RiskLow,
	"scroll":           RiskLow,

	// Write — require confirmation
	"click":                   RiskMedium,
	"click_element":           RiskMedium,
	"type_text":               RiskMedium, // may be upgraded dynamically
	"press_key":               RiskMedium, // may be upgraded dynamically
	"tui_respond":             RiskMedium,
	"tui_send_keys":           RiskMedium, // may be upgraded dynamically (Ctrl+C → High)
	"tui_run_command":         RiskMedium, // may be upgraded dynamically (sudo → High)
	"tui_dismiss_dialog":      RiskMedium,
	"macos_switch_app":        RiskMedium,
	"macos_spotlight":         RiskMedium,
	"macos_save":              RiskMedium,
	"macos_copy":              RiskLow,
	"macos_paste":             RiskMedium,
	"macos_cut":               RiskMedium,
	"macos_select_all":        RiskLow,
	"macos_undo":              RiskMedium,
	"macos_redo":              RiskMedium,
	"macos_find":              RiskLow,
	"macos_close_window":      RiskMedium,
	"macos_minimize":          RiskLow,
	"macos_hide":              RiskLow,
	"macos_fullscreen":        RiskLow,
	"macos_screenshot":        RiskLow,
	"macos_screenshot_region": RiskLow,
	"macos_new_tab":           RiskMedium,
	"macos_close_tab":         RiskMedium,
	"macos_back":              RiskLow,
	"macos_forward":           RiskLow,
	"open_url":                RiskMedium,

	// Shell — always high risk
	"run_shell": RiskHigh,
}

// highRiskHotkeys are key combos that always escalate to RiskHigh.
var highRiskHotkeys = [][]Key{
	{KeyControl, KeyC}, // Ctrl+C — terminal interrupt
	{KeyControl, KeyD}, // Ctrl+D — EOF
	{KeyControl, KeyZ}, // Ctrl+Z — suspend
}

// ──────────────────────────────────────────────────────────────
// ApprovalGateway
// ──────────────────────────────────────────────────────────────

// ApprovalGateway wraps the existing ActionGuardrails and adds a
// human-in-the-loop confirmation step for write operations.
//
// Two independent toggles:
//   - enabled: master switch for the gateway itself
//   - autoMode: when true, all non-blocked actions are auto-approved
//     (useful for trusted/supervised sessions). Defaults to false
//     (privacy-first — every write op requires human confirmation).
type ApprovalGateway struct {
	mu          sync.RWMutex
	guardrails  *ActionGuardrails
	notifier    ApprovalNotifier
	autoApprove map[string]bool // per-action whitelist
	timeout     time.Duration
	enabled     bool // master switch
	autoMode    bool // full-auto mode (default: false = privacy-first)
}

// GatewayConfig holds the configuration for creating an ApprovalGateway.
type GatewayConfig struct {
	Guardrails      *ActionGuardrails
	Notifier        ApprovalNotifier
	Timeout         time.Duration
	Enabled         bool
	AutoMode        bool     // when true, skip human approval (default: false)
	AutoApproveList []string // per-action names that bypass gateway
}

// NewApprovalGateway creates a gateway with the given configuration.
func NewApprovalGateway(cfg GatewayConfig) *ApprovalGateway {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}

	aa := make(map[string]bool, len(cfg.AutoApproveList))
	for _, a := range cfg.AutoApproveList {
		aa[a] = true
	}

	return &ApprovalGateway{
		guardrails:  cfg.Guardrails,
		notifier:    cfg.Notifier,
		autoApprove: aa,
		timeout:     cfg.Timeout,
		enabled:     cfg.Enabled,
		autoMode:    cfg.AutoMode,
	}
}

// ──────────────────────────────────────────────────────────────
// Public API
// ──────────────────────────────────────────────────────────────

// CheckAndApprove is the main entry point.  It:
//  1. Runs the static guardrails (blacklist check → hard block).
//  2. Classifies the risk level of the action.
//  3. For RiskLow or auto-approved actions, returns immediately.
//  4. For RiskMedium/High, sends an ApprovalRequest to the notifier
//     and blocks until the human responds or the timeout fires.
//
// Returns (approved, modifiedParams, error).
// modifiedParams may be non-nil if the human edited the parameters.
func (gw *ApprovalGateway) CheckAndApprove(
	ctx context.Context,
	action string,
	params json.RawMessage,
	source string,
	screenCtx *ScreenContext,
) (bool, json.RawMessage, error) {

	// ── Step 1: static guardrails (hard block) ──
	if gw.guardrails != nil {
		allowed, reason := gw.guardrails.CheckAction(action, params, source)
		if !allowed {
			return false, nil, fmt.Errorf("blocked by guardrails: %s", reason)
		}
	}

	// ── Step 2: classify risk ──
	risk := gw.classifyRisk(action, params)

	// ── Step 3: auto-approve? ──
	// Priority: disabled gateway → auto | low risk → auto | autoMode → auto | per-action whitelist → auto
	if !gw.enabled || risk == RiskLow || gw.isAutoMode() || gw.isAutoApproved(action) {
		reason := "auto-approved"
		if !gw.enabled {
			reason = "gateway-disabled"
		} else if gw.isAutoMode() {
			reason = "auto-mode"
		}
		gw.auditApproval(action, source, risk, true, reason, "")
		return true, nil, nil
	}

	// ── Step 4: request human approval ──
	if gw.notifier == nil {
		// No notifier configured — fail-closed for safety.
		gw.auditApproval(action, source, risk, false, "no notifier configured", "")
		return false, nil, fmt.Errorf("approval required but no notifier configured for action %q (risk=%s)", action, risk)
	}

	reqCtx, cancel := context.WithTimeout(ctx, gw.timeout)
	defer cancel()

	req := ApprovalRequest{
		ID:          newApprovalID(),
		ActionType:  action,
		Description: gw.buildDescription(action, params),
		RiskLevel:   risk,
		Context:     screenCtx,
		Params:      params,
		Timestamp:   float64(time.Now().UnixMilli()) / 1000.0,
	}

	// Attach screenshot for high-risk actions.
	if risk == RiskHigh && screenCtx != nil && screenCtx.ScreenshotB64 == "" {
		log.Printf("[ApprovalGateway] WARNING: High-risk action %q without screenshot context", action)
	}

	log.Printf("[ApprovalGateway] Requesting approval for %q (risk=%s, id=%s)", action, risk, req.ID)

	resp, err := gw.notifier.RequestApproval(reqCtx, req)
	if err != nil {
		gw.auditApproval(action, source, risk, false, fmt.Sprintf("notifier error: %v", err), "")
		return false, nil, fmt.Errorf("approval request failed: %w", err)
	}

	gw.auditApproval(action, source, risk, resp.Approved, resp.Reason, resp.ApprovedBy)

	if !resp.Approved {
		return false, nil, nil
	}

	// The human may have modified the parameters.
	if len(resp.ModifiedParams) > 0 {
		return true, resp.ModifiedParams, nil
	}
	return true, nil, nil
}

// SetEnabled toggles the gateway on/off at runtime.
func (gw *ApprovalGateway) SetEnabled(on bool) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.enabled = on
}

// IsEnabled returns whether the gateway is active.
func (gw *ApprovalGateway) IsEnabled() bool {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.enabled
}

// SetAutoMode toggles full-auto mode.
// When autoMode is true, all actions that pass the static guardrails
// blacklist are auto-approved without human confirmation.
// Default is false (privacy-first).
func (gw *ApprovalGateway) SetAutoMode(on bool) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.autoMode = on
	if on {
		log.Println("[ApprovalGateway] ⚠️  AutoMode ENABLED — all actions will be auto-approved")
	} else {
		log.Println("[ApprovalGateway] AutoMode disabled — privacy-first mode active")
	}
}

// IsAutoMode returns whether auto mode is active.
func (gw *ApprovalGateway) IsAutoMode() bool {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.autoMode
}

// ClassifyRisk exposes risk classification for external callers (e.g. API responses).
func (gw *ApprovalGateway) ClassifyRisk(action string, params json.RawMessage) ActionRiskLevel {
	return gw.classifyRisk(action, params)
}

// ──────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────

// classifyRisk determines the risk level, with dynamic escalation.
func (gw *ApprovalGateway) classifyRisk(action string, params json.RawMessage) ActionRiskLevel {
	base, ok := actionRiskRules[action]
	if !ok {
		base = RiskMedium // unknown actions default to medium
	}

	// Dynamic escalation for specific parameter patterns.
	switch action {
	case "type_text", "type":
		var p struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(params, &p) == nil {
			lower := strings.ToLower(p.Text)
			for _, kw := range sensitiveKeywords {
				if strings.Contains(lower, kw) {
					return RiskHigh
				}
			}
		}

	case "tui_run_command":
		// Escalate commands containing sensitive keywords.
		var p struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(params, &p) == nil {
			lower := strings.ToLower(p.Command)
			for _, kw := range sensitiveKeywords {
				if strings.Contains(lower, kw) {
					return RiskHigh
				}
			}
		}

	case "press_key", "tui_send_keys":
		var p struct {
			Keys []uint16 `json:"keys"`
		}
		if json.Unmarshal(params, &p) == nil {
			for _, hk := range highRiskHotkeys {
				if matchKeys(p.Keys, hk) {
					return RiskHigh
				}
			}
		}

	case "hotkey":
		var p struct {
			Keys []uint16 `json:"keys"`
		}
		if json.Unmarshal(params, &p) == nil {
			// Check high-risk hotkeys.
			for _, hk := range highRiskHotkeys {
				if matchKeys(p.Keys, hk) {
					return RiskHigh
				}
			}
			// Check blocked hotkeys from guardrails (always blocked, not just high-risk).
			for _, bh := range blockedHotkeys {
				if matchKeys(p.Keys, bh.keys) {
					return RiskHigh
				}
			}
		}
	}

	return base
}

// isAutoMode checks the auto-mode flag (thread-safe internal version).
func (gw *ApprovalGateway) isAutoMode() bool {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.autoMode
}

// isAutoApproved checks the per-action whitelist.
func (gw *ApprovalGateway) isAutoApproved(action string) bool {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.autoApprove[action]
}

// buildDescription creates a human-readable description for the approval prompt.
func (gw *ApprovalGateway) buildDescription(action string, params json.RawMessage) string {
	switch action {
	case "click", "click_element":
		var p struct {
			X      int    `json:"x"`
			Y      int    `json:"y"`
			Target string `json:"target"`
		}
		if json.Unmarshal(params, &p) == nil {
			if p.Target != "" {
				return fmt.Sprintf("点击元素: %q", p.Target)
			}
			return fmt.Sprintf("点击坐标: (%d, %d)", p.X, p.Y)
		}

	case "type_text", "type":
		var p struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(params, &p) == nil {
			display := p.Text
			if len(display) > 80 {
				display = display[:80] + "..."
			}
			return fmt.Sprintf("输入文本: %q", display)
		}

	case "press_key":
		var p struct {
			Keys string `json:"keys"`
		}
		if json.Unmarshal(params, &p) == nil {
			return fmt.Sprintf("按键: %s", p.Keys)
		}

	case "hotkey":
		var p struct {
			Keys []uint16 `json:"keys"`
		}
		if json.Unmarshal(params, &p) == nil {
			return fmt.Sprintf("快捷键: %v", p.Keys)
		}

	case "tui_respond":
		var p struct {
			Input string `json:"input"`
		}
		if json.Unmarshal(params, &p) == nil {
			return fmt.Sprintf("回应终端提示: %q", p.Input)
		}

	case "tui_send_keys":
		var p struct {
			Keys []uint16 `json:"keys"`
		}
		if json.Unmarshal(params, &p) == nil {
			return fmt.Sprintf("发送快捷键: %v", p.Keys)
		}

	case "tui_run_command":
		var p struct {
			Command string `json:"command"`
		}
		if json.Unmarshal(params, &p) == nil {
			display := p.Command
			if len(display) > 80 {
				display = display[:80] + "..."
			}
			return fmt.Sprintf("执行终端命令: %q", display)
		}

	case "tui_dismiss_dialog":
		return "关闭系统对话框/弹窗"

	case "macos_switch_app":
		return "切换应用 (⌘Tab)"
	case "macos_spotlight":
		return "打开 Spotlight (⌘Space)"
	case "macos_save":
		return "保存 (⌘S)"
	case "macos_copy":
		return "复制 (⌘C)"
	case "macos_paste":
		return "粘贴 (⌘V)"
	case "macos_undo":
		return "撤销 (⌘Z)"
	case "macos_close_window":
		return "关闭窗口 (⌘W)"
	}

	return fmt.Sprintf("执行操作: %s", action)
}

// auditApproval records the approval decision in the guardrails audit log.
func (gw *ApprovalGateway) auditApproval(action, source string, risk ActionRiskLevel, approved bool, reason, approvedBy string) {
	result := "approved"
	if !approved {
		result = "denied"
	}

	if gw.guardrails != nil {
		gw.guardrails.auditWithApproval(action, source, result, reason, risk, approvedBy)
	}

	level := "INFO"
	if !approved {
		level = "WARN"
	}
	log.Printf("[ApprovalGateway] %s action=%q source=%q risk=%s result=%s reason=%q",
		level, action, source, risk, result, reason)
}
