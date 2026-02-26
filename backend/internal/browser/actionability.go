// actionability.go — Playwright-style element actionability checks.
//
// Before performing interactions (click, fill, hover, type), we verify that
// the target element is in a state where the action will succeed. This
// eliminates the most common class of browser automation failures.
//
// The 5-point actionability check (inspired by Playwright):
//  1. Attached — element is connected to the DOM
//  2. Visible  — element has non-zero bounding box and is not hidden
//  3. Stable   — element is not animating (bounding rect unchanged over 100ms)
//  4. Receives Events — element is not obscured by overlays (hit-test)
//  5. Enabled  — element is not disabled
//
// References:
//   - Playwright: https://playwright.dev/docs/actionability
//   - browser-use: CDP actionability pre-checks
//   - Rod: auto-waiting with configurable timeout
package browser

import (
	"encoding/json"
	"fmt"
	"time"
)

// ActionabilityResult holds the result of an actionability check.
type ActionabilityResult struct {
	Attached       bool    `json:"attached"`
	Visible        bool    `json:"visible"`
	Enabled        bool    `json:"enabled"`
	ReceivesEvents bool    `json:"receivesEvents"`
	RectX          float64 `json:"rectX"`
	RectY          float64 `json:"rectY"`
	RectW          float64 `json:"rectW"`
	RectH          float64 `json:"rectH"`
}

// actionabilityCheckJS is the JavaScript executed via Runtime.callFunctionOn
// to perform all checks in a single CDP round-trip.
const actionabilityCheckJS = `function() {
  var rect = this.getBoundingClientRect();
  var style = window.getComputedStyle(this);
  var cx = rect.x + rect.width / 2;
  var cy = rect.y + rect.height / 2;
  var hitEl = document.elementFromPoint(cx, cy);
  return JSON.stringify({
    attached: this.isConnected,
    visible: rect.width > 0 && rect.height > 0 &&
             style.visibility !== 'hidden' &&
             style.display !== 'none' &&
             parseFloat(style.opacity) > 0,
    enabled: !this.disabled,
    receivesEvents: hitEl === this || this.contains(hitEl) ||
                    (hitEl && hitEl.closest && hitEl.closest('[data-ref="' + (this.dataset.ref || '') + '"]') === this),
    rectX: rect.x,
    rectY: rect.y,
    rectW: rect.width,
    rectH: rect.height
  });
}`

// EnsureActionable waits until the element passes all actionability checks
// or the timeout expires. Returns the center coordinates on success.
//
// This should be called before Click, Fill, Hover, Type, and similar interactions.
func EnsureActionable(send CdpSendFn, objectID string, timeoutMs int) (centerX, centerY float64, err error) {
	if timeoutMs <= 0 {
		timeoutMs = 8000 // 8s default, matching Playwright
	}

	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	var lastResult ActionabilityResult
	var lastErr error

	for time.Now().Before(deadline) {
		result, checkErr := checkActionability(send, objectID)
		if checkErr != nil {
			lastErr = checkErr
			time.Sleep(100 * time.Millisecond)
			continue
		}
		lastResult = result
		lastErr = nil

		// Check all conditions.
		if !result.Attached {
			lastErr = fmt.Errorf("element is detached from the DOM")
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if !result.Visible {
			lastErr = fmt.Errorf("element is not visible")
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if !result.Enabled {
			lastErr = fmt.Errorf("element is disabled")
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if !result.ReceivesEvents {
			lastErr = fmt.Errorf("element is obscured by another element")
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// Stability check: verify rect doesn't change over a short interval.
		time.Sleep(50 * time.Millisecond)
		result2, err2 := checkActionability(send, objectID)
		if err2 != nil {
			lastErr = err2
			continue
		}
		if !isRectStable(result, result2) {
			lastErr = fmt.Errorf("element is not stable (animating)")
			time.Sleep(100 * time.Millisecond)
			continue
		}

		// All checks pass — return center coordinates.
		cx := result2.RectX + result2.RectW/2
		cy := result2.RectY + result2.RectH/2
		return cx, cy, nil
	}

	if lastErr != nil {
		return 0, 0, fmt.Errorf("actionability timeout: %w (last state: attached=%v visible=%v enabled=%v receivesEvents=%v)",
			lastErr, lastResult.Attached, lastResult.Visible, lastResult.Enabled, lastResult.ReceivesEvents)
	}
	return 0, 0, fmt.Errorf("actionability timeout")
}

// checkActionability performs one round of actionability checks.
func checkActionability(send CdpSendFn, objectID string) (ActionabilityResult, error) {
	raw, err := send("Runtime.callFunctionOn", map[string]any{
		"objectId":            objectID,
		"functionDeclaration": actionabilityCheckJS,
		"returnByValue":       true,
	})
	if err != nil {
		return ActionabilityResult{}, err
	}

	var resp struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return ActionabilityResult{}, fmt.Errorf("actionability unmarshal: %w", err)
	}

	var result ActionabilityResult
	// Value is a JSON string (from JSON.stringify in the JS).
	var s string
	if err := json.Unmarshal(resp.Result.Value, &s); err != nil {
		// Try direct parse.
		if err2 := json.Unmarshal(resp.Result.Value, &result); err2 != nil {
			return ActionabilityResult{}, fmt.Errorf("actionability parse: %w", err2)
		}
		return result, nil
	}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return ActionabilityResult{}, fmt.Errorf("actionability parse string: %w", err)
	}
	return result, nil
}

// isRectStable returns true if the bounding rect hasn't changed significantly
// between two measurements (within 2px tolerance for sub-pixel rendering).
func isRectStable(a, b ActionabilityResult) bool {
	const tol = 2.0
	return abs64(a.RectX-b.RectX) < tol &&
		abs64(a.RectY-b.RectY) < tol &&
		abs64(a.RectW-b.RectW) < tol &&
		abs64(a.RectH-b.RectH) < tol
}

func abs64(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
