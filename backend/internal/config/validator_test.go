package config

import (
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

func TestValidateHexColor(t *testing.T) {
	v := getValidator()

	type testStruct struct {
		Color string `validate:"hexcolor"`
	}

	tests := []struct {
		value string
		valid bool
	}{
		{"#FF4500", true},
		{"FF4500", true},
		{"#ff4500", true},
		{"#GG4500", false},
		{"#FF450", false},
		{"#FF45001", false},
		{"", false},
	}

	for _, tt := range tests {
		s := testStruct{Color: tt.value}
		err := v.Struct(s)
		if tt.valid && err != nil {
			t.Errorf("expected %q to be valid, got error: %v", tt.value, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("expected %q to be invalid, got no error", tt.value)
		}
	}
}

func TestValidateSafeExecutable(t *testing.T) {
	v := getValidator()

	type testStruct struct {
		Cmd string `validate:"safe_executable"`
	}

	tests := []struct {
		value string
		valid bool
	}{
		{"node", true},
		{"/usr/bin/python3", true},
		{"my-tool", true},
		{"my_tool.sh", true},
		{"rm -rf /", false},
		{"$(whoami)", false},
		{"; echo pwned", false},
		{"", true}, // 空值允许（由 required 标签控制）
	}

	for _, tt := range tests {
		s := testStruct{Cmd: tt.value}
		err := v.Struct(s)
		if tt.valid && err != nil {
			t.Errorf("expected %q to be valid, got error: %v", tt.value, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("expected %q to be invalid, got no error", tt.value)
		}
	}
}

func TestValidateDurationString(t *testing.T) {
	v := getValidator()

	type testStruct struct {
		Dur string `validate:"duration_string"`
	}

	tests := []struct {
		value string
		valid bool
	}{
		{"500ms", true},
		{"30s", true},
		{"5m", true},
		{"1h", true},
		{"0ms", true},
		{"", true}, // 空值允许
		{"abc", false},
		{"5x", false},
		{"ms", false}, // 无数字
	}

	for _, tt := range tests {
		s := testStruct{Dur: tt.value}
		err := v.Struct(s)
		if tt.valid && err != nil {
			t.Errorf("expected %q to be valid, got error: %v", tt.value, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("expected %q to be invalid, got no error", tt.value)
		}
	}
}

func TestValidateAllowAlsoAllowMutex(t *testing.T) {
	// Both set → error
	err := ValidateAllowAlsoAllowMutex([]string{"a"}, []string{"b"})
	if err == nil {
		t.Error("expected error when both allow and alsoAllow are set")
	}

	// Only allow → ok
	err = ValidateAllowAlsoAllowMutex([]string{"a"}, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Only alsoAllow → ok
	err = ValidateAllowAlsoAllowMutex(nil, []string{"b"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Neither → ok
	err = ValidateAllowAlsoAllowMutex(nil, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateOpenPolicyAllowFrom(t *testing.T) {
	// open + has wildcard → ok
	err := ValidateOpenPolicyAllowFrom("open", []interface{}{"user1", "*"}, "test.allowFrom")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// open + no wildcard → error
	err = ValidateOpenPolicyAllowFrom("open", []interface{}{"user1"}, "test.allowFrom")
	if err == nil {
		t.Error("expected error for open policy without wildcard in allowFrom")
	}

	// non-open → ok regardless
	err = ValidateOpenPolicyAllowFrom("allowlist", []interface{}{"user1"}, "test.allowFrom")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNormalizeAllowFrom(t *testing.T) {
	result := NormalizeAllowFrom([]interface{}{"user1", 12345, " user2 ", ""})
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(result), result)
	}
	if result[0] != "user1" {
		t.Errorf("expected 'user1', got %q", result[0])
	}
	if result[1] != "12345" {
		t.Errorf("expected '12345', got %q", result[1])
	}
	if result[2] != "user2" {
		t.Errorf("expected 'user2', got %q", result[2])
	}

	// empty input
	result = NormalizeAllowFrom(nil)
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
}

func TestValidateConfig(t *testing.T) {
	// Smoke test: validate an empty struct should pass
	type SimpleConfig struct {
		Name string `validate:"omitempty,min=1"`
		Port *int   `validate:"omitempty,gt=0"`
	}

	errs := ValidateConfig(SimpleConfig{})
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty config, got %v", errs)
	}

	port := -1
	errs = ValidateConfig(SimpleConfig{Port: &port})
	if len(errs) == 0 {
		t.Error("expected errors for negative port")
	}
}

func TestFormatValidationMessage(t *testing.T) {
	// Just ensure getValidator doesn't panic on second call (sync.Once)
	v1 := getValidator()
	v2 := getValidator()
	if v1 != v2 {
		t.Error("expected same validator instance")
	}
}

// ----- H7-2: Deep Constraints Tests -----

func TestDeepConstraints_InvalidEnum(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Logging: &types.LoggingConfig{Level: "invalid_level"},
	}
	errs := ValidateOpenAcosmiConfig(cfg)
	found := false
	for _, e := range errs {
		if e.Field == "logging.level" && e.Tag == "enum" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected enum error for invalid logging.level")
	}
}

func TestDeepConstraints_InvalidPort(t *testing.T) {
	badPort := 99999
	cfg := &types.OpenAcosmiConfig{
		Gateway: &types.GatewayConfig{Port: &badPort},
	}
	errs := ValidateOpenAcosmiConfig(cfg)
	found := false
	for _, e := range errs {
		if e.Field == "gateway.port" && e.Tag == "range" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected range error for gateway.port=99999")
	}
}

func TestDeepConstraints_ValidConfig(t *testing.T) {
	goodPort := 18789
	cfg := &types.OpenAcosmiConfig{
		Logging: &types.LoggingConfig{Level: "info"},
		Gateway: &types.GatewayConfig{Port: &goodPort, Mode: "local", Bind: "auto"},
		Update:  &types.OpenAcosmiUpdateConfig{Channel: "stable"},
		Session: &types.SessionConfig{Scope: "per-sender", DmScope: "main"},
	}
	errs := ValidateOpenAcosmiConfig(cfg)
	for _, e := range errs {
		if e.Tag == "enum" || e.Tag == "range" {
			t.Errorf("unexpected deep constraint error: %v", e)
		}
	}
}
