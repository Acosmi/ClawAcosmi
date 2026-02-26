package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ---------- TTL 缓存 ----------

func TestSessionStore_TTL_PureMemory(t *testing.T) {
	s := NewSessionStore("")
	// 纯内存模式下不应过期
	if s.isCacheStale() {
		t.Error("pure memory store should never be stale")
	}
}

func TestSessionStore_TTL_InitialLoad(t *testing.T) {
	tmp := t.TempDir()
	storePath := tmp

	// 写入初始 sessions.json
	filePath := filepath.Join(storePath, "sessions.json")
	data, _ := json.Marshal(map[string]*SessionEntry{
		"test-session": {SessionKey: "test-session", DisplayName: "Test"},
	})
	os.WriteFile(filePath, data, 0o600)

	s := NewSessionStore(storePath)

	// loadedAt 应该被设置
	if s.loadedAt == 0 {
		t.Error("loadedAt should be set after initial load")
	}

	// 应该能读到 session
	entry := s.LoadSessionEntry("test-session")
	if entry == nil {
		t.Fatal("expected entry not nil")
	}
	if entry.DisplayName != "Test" {
		t.Errorf("DisplayName = %q, want %q", entry.DisplayName, "Test")
	}
}

func TestSessionStore_TTL_NotStaleWithinTTL(t *testing.T) {
	tmp := t.TempDir()
	storePath := tmp
	filePath := filepath.Join(storePath, "sessions.json")
	data, _ := json.Marshal(map[string]*SessionEntry{})
	os.WriteFile(filePath, data, 0o600)

	s := NewSessionStore(storePath)
	// 刚加载完，不应过期
	s.mu.Lock()
	stale := s.isCacheStale()
	s.mu.Unlock()
	if stale {
		t.Error("should not be stale right after load")
	}
}

func TestSessionStore_TTL_StaleAfterTTL(t *testing.T) {
	tmp := t.TempDir()
	storePath := tmp
	filePath := filepath.Join(storePath, "sessions.json")
	data, _ := json.Marshal(map[string]*SessionEntry{})
	os.WriteFile(filePath, data, 0o600)

	s := NewSessionStore(storePath)

	// 人为设置 loadedAt 为 50s 前
	s.mu.Lock()
	s.loadedAt = time.Now().UnixMilli() - 50_000

	// 修改文件并显式设置 mtime 为将来（避免 macOS 1s mtime 精度问题）
	os.WriteFile(filePath, []byte(`{"new":"entry"}`), 0o600)
	futureTime := time.Now().Add(5 * time.Second)
	os.Chtimes(filePath, futureTime, futureTime)

	stale := s.isCacheStale()
	s.mu.Unlock()
	if !stale {
		t.Error("should be stale after TTL + mtime change")
	}
}

func TestSessionStore_TTL_NotStaleIfMtimeUnchanged(t *testing.T) {
	tmp := t.TempDir()
	storePath := tmp
	filePath := filepath.Join(storePath, "sessions.json")
	data, _ := json.Marshal(map[string]*SessionEntry{})
	os.WriteFile(filePath, data, 0o600)

	s := NewSessionStore(storePath)

	// 人为设置 loadedAt 为 50s 前（TTL 过期）
	// 但 mtime 未变，应该续期而非标记为 stale
	s.mu.Lock()
	s.loadedAt = time.Now().UnixMilli() - 50_000
	stale := s.isCacheStale()
	s.mu.Unlock()
	if stale {
		t.Error("should not be stale when TTL expired but mtime unchanged")
	}
}

// ---------- UpdateLastRoute ----------

func TestSessionStore_UpdateLastRoute_New(t *testing.T) {
	s := NewSessionStore("")
	s.UpdateLastRoute("sess-1", UpdateLastRouteParams{
		Channel: "telegram", To: "user-123", AccountId: "acc-1", ThreadId: "thread-42",
	})

	entry := s.LoadSessionEntry("sess-1")
	if entry == nil {
		t.Fatal("entry should be created")
	}
	if entry.DeliveryContext == nil || entry.DeliveryContext.Channel != "telegram" {
		t.Errorf("DeliveryContext.Channel want telegram")
	}
	if entry.LastTo != "user-123" {
		t.Errorf("LastTo = %q, want user-123", entry.LastTo)
	}
	if entry.LastAccountId != "acc-1" {
		t.Errorf("LastAccountId = %q, want acc-1", entry.LastAccountId)
	}
	if entry.LastThreadId != "thread-42" {
		t.Errorf("LastThreadId = %v, want thread-42", entry.LastThreadId)
	}
	if entry.LastChannel == nil || entry.LastChannel.Channel != "telegram" {
		t.Error("LastChannel should be set")
	}
	if entry.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set")
	}
}

func TestSessionStore_UpdateLastRoute_Existing(t *testing.T) {
	s := NewSessionStore("")
	s.Save(&SessionEntry{
		SessionKey:  "sess-2",
		Channel:     "slack",
		DisplayName: "Old Name",
		UpdatedAt:   1000,
	})

	s.UpdateLastRoute("sess-2", UpdateLastRouteParams{Channel: "telegram"})

	entry := s.LoadSessionEntry("sess-2")
	// DeliveryContext 应更新为 telegram
	if entry.DeliveryContext == nil || entry.DeliveryContext.Channel != "telegram" {
		t.Errorf("DeliveryContext.Channel want telegram")
	}
	// DisplayName 不应被修改
	if entry.DisplayName != "Old Name" {
		t.Errorf("DisplayName = %q, want Old Name", entry.DisplayName)
	}
	// UpdatedAt 应该更新
	if entry.UpdatedAt <= 1000 {
		t.Error("UpdatedAt should be updated")
	}
}

func TestSessionStore_UpdateLastRoute_EmptyKey(t *testing.T) {
	s := NewSessionStore("")
	s.UpdateLastRoute("", UpdateLastRouteParams{Channel: "telegram", To: "user"})
	if s.Count() != 0 {
		t.Error("should not create entry for empty key")
	}
}

func TestSessionStore_UpdateLastRoute_PartialUpdate(t *testing.T) {
	s := NewSessionStore("")
	s.UpdateLastRoute("sess-3", UpdateLastRouteParams{Channel: "slack", To: "user-1", AccountId: "acc-1", ThreadId: "thread-1"})
	s.UpdateLastRoute("sess-3", UpdateLastRouteParams{To: "user-2"})

	entry := s.LoadSessionEntry("sess-3")
	// Channel 应保留（从 fallback）
	if entry.DeliveryContext == nil || entry.DeliveryContext.Channel != "slack" {
		t.Errorf("Channel should stay slack")
	}
	if entry.LastTo != "user-2" {
		t.Errorf("LastTo should update, got %q", entry.LastTo)
	}
	if entry.LastAccountId != "acc-1" {
		t.Errorf("LastAccountId should stay, got %q", entry.LastAccountId)
	}
}

// ---------- RecordSessionMeta ----------

func TestSessionStore_RecordSessionMeta_New(t *testing.T) {
	s := NewSessionStore("")
	s.RecordSessionMeta("sess-meta-1", InboundMeta{
		DisplayName:  "Alice",
		Subject:      "Test Subject",
		Channel:      "imessage",
		GroupChannel: "group-1",
	})

	entry := s.LoadSessionEntry("sess-meta-1")
	if entry == nil {
		t.Fatal("entry should be created")
	}
	if entry.DisplayName != "Alice" {
		t.Errorf("DisplayName = %q", entry.DisplayName)
	}
	if entry.Subject != "Test Subject" {
		t.Errorf("Subject = %q", entry.Subject)
	}
	if entry.Channel != "imessage" {
		t.Errorf("Channel = %q", entry.Channel)
	}
	if entry.GroupChannel != "group-1" {
		t.Errorf("GroupChannel = %q", entry.GroupChannel)
	}
	if entry.CreatedAt == 0 {
		t.Error("CreatedAt should be set for new entries")
	}
}

func TestSessionStore_RecordSessionMeta_MergeExisting(t *testing.T) {
	s := NewSessionStore("")
	s.Save(&SessionEntry{
		SessionKey:  "sess-meta-2",
		DisplayName: "Original",
		Subject:     "Original Subject",
		Channel:     "telegram",
	})

	// 部分更新
	s.RecordSessionMeta("sess-meta-2", InboundMeta{
		DisplayName: "Updated",
		// Subject 为空,不应覆盖
	})

	entry := s.LoadSessionEntry("sess-meta-2")
	if entry.DisplayName != "Updated" {
		t.Errorf("DisplayName should update, got %q", entry.DisplayName)
	}
	if entry.Subject != "Original Subject" {
		t.Errorf("Subject should stay, got %q", entry.Subject)
	}
}

func TestSessionStore_RecordSessionMeta_EmptyKey(t *testing.T) {
	s := NewSessionStore("")
	s.RecordSessionMeta("", InboundMeta{DisplayName: "test"})
	if s.Count() != 0 {
		t.Error("should not create entry for empty key")
	}
}
