package input

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// ActionGuardrails enforces safety checks before input injection.
// Runs in the Go execution layer so dangerous actions are blocked
// before they ever reach CoreGraphics.
type ActionGuardrails struct {
	mu           sync.Mutex
	auditLog     *os.File
	auditEntries int
}

// AuditEntry records an action for security review.
type AuditEntry struct {
	Timestamp  float64 `json:"ts"`
	Action     string  `json:"action"`
	Source     string  `json:"source"`
	Result     string  `json:"result"` // "allowed" | "blocked" | "approved" | "denied"
	Reason     string  `json:"reason,omitempty"`
	ApprovalID string  `json:"approval_id,omitempty"`
	ApprovedBy string  `json:"approved_by,omitempty"`
	RiskLevel  int     `json:"risk_level,omitempty"`
}

// blockedHotkey describes a dangerous key combination.
type blockedHotkey struct {
	keys    []Key
	display string
}

// Dangerous hotkey combinations (modifier key codes + main key).
var blockedHotkeys = []blockedHotkey{
	{keys: []Key{KeyCommand, KeyQ}, display: "Cmd+Q (Quit application)"},
	{keys: []Key{KeyCommand, KeyShift, KeyDelete}, display: "Cmd+Shift+Delete (Empty Trash)"},
}

// Keywords in typed text that require extra caution.
var sensitiveKeywords = []string{
	"sudo", "rm ", "rm\t", "mkfs", "fdisk", "format",
	"shutdown", "reboot", "halt",
}

// NewActionGuardrails creates a new guardrails instance.
// If logPath is non-empty, audit entries are appended to that file.
func NewActionGuardrails(logPath string) *ActionGuardrails {
	g := &ActionGuardrails{}
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("[Guardrails] Warning: cannot open audit log %s: %v", logPath, err)
		} else {
			g.auditLog = f
		}
	}
	return g
}

// CheckAction validates whether an action is safe to execute.
// Returns (allowed bool, reason string).
func (g *ActionGuardrails) CheckAction(action string, params json.RawMessage, source string) (bool, string) {
	switch action {
	case "hotkey":
		var p struct {
			Keys []uint16 `json:"keys"`
		}
		if err := json.Unmarshal(params, &p); err == nil {
			if reason := g.checkBlockedHotkey(p.Keys); reason != "" {
				g.audit(action, source, "blocked", reason)
				return false, reason
			}
		}

	case "type":
		var p struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(params, &p); err == nil {
			if reason := g.checkSensitiveText(p.Text); reason != "" {
				g.audit(action, source, "blocked", reason)
				return false, reason
			}
		}
	}

	g.audit(action, source, "allowed", "")
	return true, ""
}

// checkBlockedHotkey returns a reason string if the key combo is dangerous.
func (g *ActionGuardrails) checkBlockedHotkey(keys []uint16) string {
	for _, bh := range blockedHotkeys {
		if matchKeys(keys, bh.keys) {
			return fmt.Sprintf("Blocked dangerous hotkey: %s", bh.display)
		}
	}
	return ""
}

// matchKeys checks if the provided keys match a blocked pattern (order-insensitive).
func matchKeys(have []uint16, want []Key) bool {
	if len(have) != len(want) {
		return false
	}
	seen := make(map[Key]bool, len(want))
	for _, k := range want {
		seen[k] = true
	}
	for _, k := range have {
		if !seen[Key(k)] {
			return false
		}
	}
	return true
}

// checkSensitiveText returns a reason if the text contains dangerous keywords.
func (g *ActionGuardrails) checkSensitiveText(text string) string {
	lower := strings.ToLower(text)
	for _, kw := range sensitiveKeywords {
		if strings.Contains(lower, kw) {
			return fmt.Sprintf("Text contains sensitive keyword '%s', action blocked", strings.TrimSpace(kw))
		}
	}
	return ""
}

// audit writes an entry to the audit log.
func (g *ActionGuardrails) audit(action, source, result, reason string) {
	g.writeAuditEntry(AuditEntry{
		Timestamp: float64(time.Now().UnixMilli()) / 1000.0,
		Action:    action,
		Source:    source,
		Result:    result,
		Reason:    reason,
	})
}

// auditWithApproval writes an audit entry with approval-specific metadata.
// Called by ApprovalGateway to record human approval decisions.
func (g *ActionGuardrails) auditWithApproval(action, source, result, reason string, riskLevel ActionRiskLevel, approvedBy string) {
	g.writeAuditEntry(AuditEntry{
		Timestamp:  float64(time.Now().UnixMilli()) / 1000.0,
		Action:     action,
		Source:     source,
		Result:     result,
		Reason:     reason,
		RiskLevel:  int(riskLevel),
		ApprovedBy: approvedBy,
	})
}

// writeAuditEntry is the shared writer for audit entries.
func (g *ActionGuardrails) writeAuditEntry(entry AuditEntry) {
	if entry.Result == "blocked" || entry.Result == "denied" {
		log.Printf("[Guardrails] %s %s from %s: %s", strings.ToUpper(entry.Result), entry.Action, entry.Source, entry.Reason)
	}

	if g.auditLog != nil {
		g.mu.Lock()
		defer g.mu.Unlock()
		data, _ := json.Marshal(entry)
		g.auditLog.Write(append(data, '\n'))
		g.auditEntries++
	}
}

// Close releases resources.
func (g *ActionGuardrails) Close() {
	if g.auditLog != nil {
		g.auditLog.Close()
	}
}
