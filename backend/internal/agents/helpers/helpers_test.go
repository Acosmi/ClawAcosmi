package helpers

import "testing"

func TestIsContextOverflowError(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"", false},
		{"context length exceeded", true},
		{"maximum context length", true},
		{"request_too_large", true},
		{"prompt is too long", true},
		{"413 too large", true},
		{"context window limit exceeded", false}, // ← TS: not exact match, only likely
		{"token count exceeds limit", false},     // ← TS: not exact match
		{"random error", false},
	}
	for _, tt := range tests {
		if got := IsContextOverflowError(tt.msg); got != tt.want {
			t.Errorf("IsContextOverflowError(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}

func TestIsRateLimitErrorMessage(t *testing.T) {
	if !IsRateLimitErrorMessage("rate limit exceeded") {
		t.Error("should match rate limit")
	}
	if !IsRateLimitErrorMessage("too many requests") {
		t.Error("should match too many requests")
	}
	if IsRateLimitErrorMessage("random error") {
		t.Error("should not match random")
	}
}

func TestIsBillingErrorMessage(t *testing.T) {
	if !IsBillingErrorMessage("payment required") {
		t.Error("should match payment required")
	}
	if !IsBillingErrorMessage("billing: please upgrade your plan") {
		t.Error("should match billing + plan")
	}
	if IsBillingErrorMessage("billing error occurred") {
		t.Error("billing without upgrade/credits/payment/plan should NOT match per TS")
	}
}

func TestClassifyFailoverReason(t *testing.T) {
	if ClassifyFailoverReason("rate limit exceeded") != "rate_limit" {
		t.Error("rate limit")
	}
	if ClassifyFailoverReason("payment required") != "billing" {
		t.Error("billing")
	}
	if ClassifyFailoverReason("unauthorized") != "auth" {
		t.Error("auth")
	}
	if ClassifyFailoverReason("timeout expired") != "timeout" {
		t.Error("timeout")
	}
	if ClassifyFailoverReason("random") != "" {
		t.Error("should be empty for unknown")
	}
}

func TestSanitizeUserFacingText(t *testing.T) {
	result := SanitizeUserFacingText("Hello <final> World </final>!")
	if result != "Hello  World !" {
		t.Errorf("sanitize = %q", result)
	}
}

func TestNormalizeTextForComparison(t *testing.T) {
	a := "  Hello   World  "
	b := "hello world"
	if NormalizeTextForComparison(a) != b {
		t.Errorf("normalize = %q", NormalizeTextForComparison(a))
	}
}

func TestIsMessagingToolDuplicate(t *testing.T) {
	if !IsMessagingToolDuplicate("Hello  World", "hello world") {
		t.Error("should be duplicate")
	}
	if IsMessagingToolDuplicate("Hello", "World") {
		t.Error("should not be duplicate")
	}
}

func TestResolveBootstrapMaxChars(t *testing.T) {
	if ResolveBootstrapMaxChars(0) != DefaultBootstrapMaxChars {
		t.Error("zero → default")
	}
	if ResolveBootstrapMaxChars(1000) != 1000 {
		t.Error("explicit")
	}
}
