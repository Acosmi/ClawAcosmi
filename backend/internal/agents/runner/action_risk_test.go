package runner

import "testing"

// ============================================================================
// ActionRisk 单元测试
// ============================================================================

func TestClassifyActionRisk_KnownTools(t *testing.T) {
	tests := []struct {
		tool     string
		expected ActionRiskLevel
	}{
		{"screenshot", RiskNone},
		{"capture_screen", RiskNone},
		{"get_elements", RiskNone},
		{"wait", RiskNone},
		{"scroll", RiskLow},
		{"hover", RiskLow},
		{"click", RiskMedium},
		{"type", RiskMedium},
		{"select", RiskMedium},
		{"navigate", RiskHigh},
		{"submit", RiskHigh},
		{"run_shell", RiskHigh},
	}
	for _, tc := range tests {
		got := ClassifyActionRisk(tc.tool)
		if got != tc.expected {
			t.Errorf("ClassifyActionRisk(%q) = %d, want %d", tc.tool, got, tc.expected)
		}
	}
}

func TestClassifyActionRisk_Unknown(t *testing.T) {
	got := ClassifyActionRisk("future_unknown_tool")
	if got != RiskMedium {
		t.Errorf("unknown tool should default to RiskMedium, got %d", got)
	}
}

func TestShouldRequireApproval_None(t *testing.T) {
	for _, risk := range []ActionRiskLevel{RiskNone, RiskLow, RiskMedium, RiskHigh} {
		if ShouldRequireApproval(risk, "none") {
			t.Errorf("mode=none should never require approval, risk=%d", risk)
		}
	}
}

func TestShouldRequireApproval_All(t *testing.T) {
	for _, risk := range []ActionRiskLevel{RiskNone, RiskLow, RiskMedium, RiskHigh} {
		if !ShouldRequireApproval(risk, "all") {
			t.Errorf("mode=all should always require approval, risk=%d", risk)
		}
	}
}

func TestShouldRequireApproval_MediumAndAbove(t *testing.T) {
	tests := []struct {
		risk     ActionRiskLevel
		expected bool
	}{
		{RiskNone, false},
		{RiskLow, false},
		{RiskMedium, true},
		{RiskHigh, true},
	}
	for _, tc := range tests {
		got := ShouldRequireApproval(tc.risk, "medium_and_above")
		if got != tc.expected {
			t.Errorf("medium_and_above: risk=%d, got %v, want %v", tc.risk, got, tc.expected)
		}
	}
}

func TestShouldRequireApproval_DefaultMode(t *testing.T) {
	// Empty string = default = medium_and_above
	if ShouldRequireApproval(RiskNone, "") {
		t.Error("default mode + RiskNone should not require approval")
	}
	if !ShouldRequireApproval(RiskHigh, "") {
		t.Error("default mode + RiskHigh should require approval")
	}
}

func TestRiskLevelString(t *testing.T) {
	tests := []struct {
		level    ActionRiskLevel
		expected string
	}{
		{RiskNone, "none"},
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
		{ActionRiskLevel(99), "unknown"},
	}
	for _, tc := range tests {
		got := RiskLevelString(tc.level)
		if got != tc.expected {
			t.Errorf("RiskLevelString(%d) = %q, want %q", tc.level, got, tc.expected)
		}
	}
}
