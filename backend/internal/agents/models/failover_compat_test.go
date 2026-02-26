package models

import (
	"errors"
	"testing"
)

func TestShouldFailover_FailoverError(t *testing.T) {
	fe := &FailoverError{Message: "test", Reason: FailoverRateLimit}
	if !ShouldFailover(fe) {
		t.Error("FailoverError should trigger failover")
	}
}

func TestClassifyFailoverReason(t *testing.T) {
	tests := []struct {
		msg  string
		want FailoverReason
	}{
		{"rate limit exceeded", FailoverRateLimit},
		{"payment required", FailoverBilling},
		{"unauthorized", FailoverAuth},
		{"request timeout", FailoverTimeout},
		{"overloaded_error", FailoverRateLimit}, // TS: overloaded → rate_limit
		{"random unknown error", ""},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := ClassifyFailoverReason(tt.msg)
			if got != tt.want {
				t.Errorf("ClassifyFailoverReason(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

func TestCoerceToFailoverError(t *testing.T) {
	err := errors.New("rate limit exceeded")
	fe := CoerceToFailoverError(err, "anthropic", "claude-3", "")
	if fe == nil {
		t.Fatal("should coerce to FailoverError")
	}
	if fe.Reason != FailoverRateLimit {
		t.Errorf("reason = %q, want rate_limit", fe.Reason)
	}
	if fe.Status != 429 {
		t.Errorf("status = %d, want 429", fe.Status)
	}

	// Unknown error
	fe = CoerceToFailoverError(errors.New("random"), "a", "b", "")
	if fe != nil {
		t.Error("unknown error should return nil")
	}
}

func TestNormalizeModelCompat_Zai(t *testing.T) {
	result := NormalizeModelCompat("zai", "", nil)
	if result == nil || result.SupportsDeveloperRole == nil || *result.SupportsDeveloperRole {
		t.Error("Zai should disable developer role")
	}

	// Non-Zai
	result = NormalizeModelCompat("anthropic", "", nil)
	if result != nil {
		t.Error("non-Zai should return nil for nil input")
	}
}
