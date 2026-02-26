package pairing

// 配对存储测试 — 对齐 src/pairing/pairing-store.test.ts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withTempStateDir 创建临时状态目录用于测试。
func withTempStateDir(t *testing.T) (cleanup func()) {
	t.Helper()
	dir := t.TempDir()
	previous := os.Getenv("OPENACOSMI_STATE_DIR")
	os.Setenv("OPENACOSMI_STATE_DIR", dir)
	return func() {
		if previous == "" {
			os.Unsetenv("OPENACOSMI_STATE_DIR")
		} else {
			os.Setenv("OPENACOSMI_STATE_DIR", previous)
		}
	}
}

func TestUpsertReusesCode(t *testing.T) {
	// 对齐 TS: "reuses pending code and reports created=false"
	cleanup := withTempStateDir(t)
	defer cleanup()

	code1, created1, err := UpsertChannelPairingRequest("discord", "u1", nil)
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if !created1 {
		t.Error("expected created=true for first upsert")
	}
	if code1 == "" {
		t.Error("expected non-empty code")
	}

	code2, created2, err := UpsertChannelPairingRequest("discord", "u1", nil)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if created2 {
		t.Error("expected created=false for second upsert")
	}
	if code2 != code1 {
		t.Errorf("expected same code %q, got %q", code1, code2)
	}

	list, err := ListChannelPairingRequests("discord")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 request, got %d", len(list))
	}
	if list[0].Code != code1 {
		t.Errorf("expected code %q, got %q", code1, list[0].Code)
	}
}

func TestExpiresAfterTTL(t *testing.T) {
	// 对齐 TS: "expires pending requests after TTL"
	cleanup := withTempStateDir(t)
	defer cleanup()

	code, created, err := UpsertChannelPairingRequest("signal", "+15550001111", nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !created || code == "" {
		t.Error("expected created=true with non-empty code")
	}

	// 手动过期（修改文件中的时间戳为 2 小时前）
	stateDir := os.Getenv("OPENACOSMI_STATE_DIR")
	credDir := filepath.Join(stateDir, "credentials")
	filePath := filepath.Join(credDir, "signal-pairing.json")
	raw, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var store pairingStore
	if err := json.Unmarshal(raw, &store); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	expiredAt := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339Nano)
	for i := range store.Requests {
		store.Requests[i].CreatedAt = expiredAt
		store.Requests[i].LastSeenAt = expiredAt
	}
	data, _ := json.MarshalIndent(store, "", "  ")
	if err := os.WriteFile(filePath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	list, err := ListChannelPairingRequests("signal")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 requests after expiry, got %d", len(list))
	}

	// 重新创建应成功
	_, created2, err := UpsertChannelPairingRequest("signal", "+15550001111", nil)
	if err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	if !created2 {
		t.Error("expected created=true after expired")
	}
}

func TestCapsAtMaxPending(t *testing.T) {
	// 对齐 TS: "caps pending requests at the default limit"
	cleanup := withTempStateDir(t)
	defer cleanup()

	ids := []string{"+15550000001", "+15550000002", "+15550000003"}
	for _, id := range ids {
		_, created, err := UpsertChannelPairingRequest("whatsapp", id, nil)
		if err != nil {
			t.Fatalf("upsert %s: %v", id, err)
		}
		if !created {
			t.Errorf("expected created=true for %s", id)
		}
	}

	// 第 4 个应被拒绝
	code, created, err := UpsertChannelPairingRequest("whatsapp", "+15550000004", nil)
	if err != nil {
		t.Fatalf("upsert blocked: %v", err)
	}
	if created {
		t.Error("expected created=false when at capacity")
	}
	if code != "" {
		t.Error("expected empty code when blocked")
	}

	list, err := ListChannelPairingRequests("whatsapp")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 requests, got %d", len(list))
	}
	listIDs := make(map[string]bool)
	for _, r := range list {
		listIDs[r.ID] = true
	}
	for _, id := range ids {
		if !listIDs[id] {
			t.Errorf("expected %s in list", id)
		}
	}
	if listIDs["+15550000004"] {
		t.Error("blocked ID should not be in list")
	}
}

func TestApproveCode(t *testing.T) {
	cleanup := withTempStateDir(t)
	defer cleanup()

	code, _, err := UpsertChannelPairingRequest("telegram", "123", nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	id, entry, err := ApproveChannelPairingCode("telegram", code)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if id != "123" {
		t.Errorf("expected id=123, got %s", id)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	// 再次列出应为空
	list, _ := ListChannelPairingRequests("telegram")
	if len(list) != 0 {
		t.Errorf("expected 0 requests after approve, got %d", len(list))
	}

	// 白名单应包含该 ID
	allow, err := ReadChannelAllowFromStore("telegram")
	if err != nil {
		t.Fatalf("read allow: %v", err)
	}
	found := false
	for _, a := range allow {
		if a == "123" {
			found = true
		}
	}
	if !found {
		t.Error("expected 123 in allow list after approval")
	}
}

func TestAllowFromAddRemove(t *testing.T) {
	cleanup := withTempStateDir(t)
	defer cleanup()

	// 添加
	changed, list, err := AddChannelAllowFromStoreEntry("discord", "user1")
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if !changed {
		t.Error("expected changed=true")
	}
	if len(list) != 1 || list[0] != "user1" {
		t.Errorf("unexpected list: %v", list)
	}

	// 重复添加
	changed2, _, err := AddChannelAllowFromStoreEntry("discord", "user1")
	if err != nil {
		t.Fatalf("add dup: %v", err)
	}
	if changed2 {
		t.Error("expected changed=false for duplicate")
	}

	// 移除
	changed3, list3, err := RemoveChannelAllowFromStoreEntry("discord", "user1")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !changed3 {
		t.Error("expected changed=true for remove")
	}
	if len(list3) != 0 {
		t.Errorf("expected empty list, got %v", list3)
	}

	// 移除不存在的
	changed4, _, err := RemoveChannelAllowFromStoreEntry("discord", "nonexistent")
	if err != nil {
		t.Fatalf("remove nonexistent: %v", err)
	}
	if changed4 {
		t.Error("expected changed=false for nonexistent")
	}
}

func TestSafeChannelKey(t *testing.T) {
	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"discord", "discord", false},
		{"Discord", "discord", false},
		{" signal ", "signal", false},
		{"my:channel", "my_channel", false},
		{"", "", true},
		{" ", "", true},
	}
	for _, tc := range cases {
		got, err := safeChannelKey(tc.input)
		if tc.wantErr {
			if err == nil {
				t.Errorf("safeChannelKey(%q) expected error", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("safeChannelKey(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("safeChannelKey(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
