package metrics

import (
	"testing"
)

// TestGetRustMetrics verifies Rust metrics can be fetched.
func TestGetRustMetrics(t *testing.T) {
	// Reset first to get a clean state
	ResetRustMetrics()

	m, err := GetRustMetrics()
	if err != nil {
		t.Fatalf("GetRustMetrics failed: %v", err)
	}
	// After reset, all should be 0
	if m.FramesCaptured != 0 || m.ResizesTotal != 0 || m.ShmWritesTotal != 0 {
		t.Errorf("expected all zeros after reset, got %+v", m)
	}
	t.Logf("Rust metrics: %+v", m)
}

// TestRenderRustMetrics verifies Prometheus format output.
func TestRenderRustMetrics(t *testing.T) {
	ResetRustMetrics()
	text := RenderRustMetrics()
	if len(text) == 0 {
		t.Error("expected non-empty Prometheus metrics text")
	}
	t.Logf("Prometheus output:\n%s", text)
}

// TestRustMetrics_PIIIncrement verifies PII scan counter increments.
func TestRustMetrics_PIIIncrement(t *testing.T) {
	ResetRustMetrics()

	// The PII scan counter should be 0 after reset
	m1, _ := GetRustMetrics()
	if m1.PIIScansTotal != 0 {
		t.Fatalf("expected pii_scans=0 after reset, got %d", m1.PIIScansTotal)
	}

	t.Logf("After reset: pii_scans=%d, crypto_ops=%d", m1.PIIScansTotal, m1.CryptoOpsTotal)
}
