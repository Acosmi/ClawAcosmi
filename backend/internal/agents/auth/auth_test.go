package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "auth.json")

	store := NewAuthStore(storePath)

	// 空 store 读取
	data, err := store.Load()
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if data.Version != AuthStoreVersion {
		t.Errorf("Version = %d, want %d", data.Version, AuthStoreVersion)
	}

	// 添加 profile 并保存
	data.Profiles["anthropic-main"] = &AuthProfileCredential{
		Type:     CredentialAPIKey,
		Provider: "anthropic",
		Key:      "sk-test-123",
		Email:    "test@example.com",
	}
	if err := store.Save(data); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 重新加载
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if loaded.Profiles["anthropic-main"] == nil {
		t.Fatal("profile not found after reload")
	}
	if loaded.Profiles["anthropic-main"].Key != "sk-test-123" {
		t.Errorf("Key mismatch: %q", loaded.Profiles["anthropic-main"].Key)
	}

	// 文件权限
	info, _ := os.Stat(storePath)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o, want 600", info.Mode().Perm())
	}
}

func TestAuthStoreUpdate(t *testing.T) {
	dir := t.TempDir()
	store := NewAuthStore(filepath.Join(dir, "auth.json"))

	result, err := store.Update(func(s *AuthProfileStore) bool {
		s.Profiles["test"] = &AuthProfileCredential{
			Type:     CredentialToken,
			Provider: "openai",
			Token:    "tok-abc",
		}
		return true
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if result.Profiles["test"].Token != "tok-abc" {
		t.Error("profile not updated")
	}
}

func TestIsProfileInCooldown(t *testing.T) {
	store := &AuthProfileStore{
		UsageStats: map[string]*ProfileUsageStats{},
	}

	// Not in cooldown
	if IsProfileInCooldown(store, "test") {
		t.Error("should not be in cooldown")
	}

	// In cooldown
	future := time.Now().Add(5 * time.Minute).UnixMilli()
	store.UsageStats["test"] = &ProfileUsageStats{CooldownUntil: &future}
	if !IsProfileInCooldown(store, "test") {
		t.Error("should be in cooldown")
	}

	// Expired cooldown
	past := time.Now().Add(-5 * time.Minute).UnixMilli()
	store.UsageStats["expired"] = &ProfileUsageStats{CooldownUntil: &past}
	if IsProfileInCooldown(store, "expired") {
		t.Error("expired cooldown should not count")
	}
}

func TestMarkProfileUsed(t *testing.T) {
	store := &AuthProfileStore{
		UsageStats: map[string]*ProfileUsageStats{
			"test": {ErrorCount: intPtr(5)},
		},
	}

	MarkProfileUsed(store, "test")
	stats := store.UsageStats["test"]
	if stats.LastUsed == nil {
		t.Error("LastUsed should be set")
	}
	if stats.ErrorCount == nil || *stats.ErrorCount != 0 {
		t.Error("ErrorCount should be reset to 0")
	}
	if stats.CooldownUntil != nil {
		t.Error("CooldownUntil should be cleared")
	}
}

func TestMarkProfileFailure(t *testing.T) {
	store := &AuthProfileStore{
		UsageStats: map[string]*ProfileUsageStats{},
	}

	MarkProfileFailure(store, "test", FailureRateLimit)
	stats := store.UsageStats["test"]
	if stats.ErrorCount == nil || *stats.ErrorCount != 1 {
		t.Error("ErrorCount should be 1")
	}
	if stats.CooldownUntil == nil {
		t.Error("CooldownUntil should be set for rate_limit")
	}
	if stats.FailureCounts[FailureRateLimit] != 1 {
		t.Error("FailureCounts should track rate_limit")
	}

	// Billing failure
	MarkProfileFailure(store, "billing-test", FailureBilling)
	billingStats := store.UsageStats["billing-test"]
	if billingStats.DisabledUntil == nil {
		t.Error("should be disabled for billing failure")
	}
	if billingStats.DisabledReason != FailureBilling {
		t.Errorf("DisabledReason = %q, want billing", billingStats.DisabledReason)
	}
}

func TestCalculateCooldownMs(t *testing.T) {
	tests := []struct {
		errorCount int
		wantMin    int64
		wantMax    int64
	}{
		{0, 60_000, 60_000},        // 1min
		{1, 60_000, 60_000},        // 1min
		{2, 300_000, 300_000},      // 5min
		{3, 1_500_000, 1_500_000},  // 25min
		{10, 3_600_000, 3_600_000}, // capped at 1hr
	}
	for _, tt := range tests {
		got := CalculateCooldownMs(tt.errorCount)
		if got < tt.wantMin || got > tt.wantMax {
			t.Errorf("CalculateCooldownMs(%d) = %d, want [%d, %d]", tt.errorCount, got, tt.wantMin, tt.wantMax)
		}
	}
}

func TestClearProfileCooldown(t *testing.T) {
	future := time.Now().Add(1 * time.Hour).UnixMilli()
	store := &AuthProfileStore{
		UsageStats: map[string]*ProfileUsageStats{
			"test": {
				CooldownUntil:  &future,
				DisabledUntil:  &future,
				DisabledReason: FailureBilling,
				ErrorCount:     intPtr(5),
			},
		},
	}

	ClearProfileCooldown(store, "test")
	stats := store.UsageStats["test"]
	if stats.CooldownUntil != nil {
		t.Error("CooldownUntil should be nil")
	}
	if stats.DisabledUntil != nil {
		t.Error("DisabledUntil should be nil")
	}
	if stats.ErrorCount == nil || *stats.ErrorCount != 0 {
		t.Error("ErrorCount should be 0")
	}
}

func intPtr(v int) *int { return &v }
