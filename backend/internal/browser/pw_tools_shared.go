// Package browser — shared utilities for Playwright tools.
// TS source: pw-tools-core.shared.ts (71L)
package browser

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// ---------- Arm ID generators ----------

var (
	nextUploadArmID   atomic.Int64
	nextDialogArmID   atomic.Int64
	nextDownloadArmID atomic.Int64
)

// BumpUploadArmID returns a monotonically increasing arm id for upload waiters.
func BumpUploadArmID() int64 {
	return nextUploadArmID.Add(1)
}

// BumpDialogArmID returns a monotonically increasing arm id for dialog waiters.
func BumpDialogArmID() int64 {
	return nextDialogArmID.Add(1)
}

// BumpDownloadArmID returns a monotonically increasing arm id for download waiters.
func BumpDownloadArmID() int64 {
	return nextDownloadArmID.Add(1)
}

// ---------- Ref helpers ----------

// RequireRef validates and normalises a ref string.
// It returns the normalised ref or an error if empty.
func RequireRef(value string) (string, error) {
	raw := strings.TrimSpace(value)
	roleRef := ""
	if raw != "" {
		roleRef = ParseRoleRef(raw)
	}
	ref := roleRef
	if ref == "" {
		if strings.HasPrefix(raw, "@") {
			ref = raw[1:]
		} else {
			ref = raw
		}
	}
	if ref == "" {
		return "", fmt.Errorf("ref is required")
	}
	return ref, nil
}

// NormalizeTimeoutMs clamps a timeout value to [500, 120000] ms.
func NormalizeTimeoutMs(timeoutMs, fallback int) int {
	v := timeoutMs
	if v <= 0 {
		v = fallback
	}
	if v < 500 {
		v = 500
	}
	if v > 120_000 {
		v = 120_000
	}
	return v
}

// ---------- AI-friendly error transforms ----------

// ToAIFriendlyError converts low-level CDP / Playwright errors into
// messages that an AI agent can understand and act upon.
func ToAIFriendlyError(err error, selector string) error {
	if err == nil {
		return nil
	}
	message := err.Error()

	if strings.Contains(message, "strict mode violation") {
		count := "multiple"
		if idx := strings.Index(message, "resolved to "); idx >= 0 {
			sub := message[idx+12:]
			if spIdx := strings.Index(sub, " "); spIdx > 0 {
				count = sub[:spIdx]
			}
		}
		return fmt.Errorf(
			"Selector %q matched %s elements. "+
				"Run a new snapshot to get updated refs, or use a different ref.",
			selector, count,
		)
	}

	if (strings.Contains(message, "Timeout") || strings.Contains(message, "waiting for")) &&
		(strings.Contains(message, "to be visible") || strings.Contains(message, "not visible")) {
		return fmt.Errorf(
			"Element %q not found or not visible. "+
				"Run a new snapshot to see current page elements.",
			selector,
		)
	}

	if strings.Contains(message, "intercepts pointer events") ||
		strings.Contains(message, "not visible") ||
		strings.Contains(message, "not receive pointer events") {
		return fmt.Errorf(
			"Element %q is not interactable (hidden or covered). "+
				"Try scrolling it into view, closing overlays, or re-snapshotting.",
			selector,
		)
	}

	return err
}
