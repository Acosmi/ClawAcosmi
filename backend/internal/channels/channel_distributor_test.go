package channels

import (
	"testing"
)

// mockVFS implements ChannelVFSWriter for testing.
type mockVFS struct {
	entries map[string]string // key="ns/cat/id" → l0
}

func newMockVFS() *mockVFS {
	return &mockVFS{entries: make(map[string]string)}
}

func (m *mockVFS) WriteSystemEntry(namespace, category, id, l0, _, _ string, _ map[string]interface{}) error {
	m.entries[namespace+"/"+category+"/"+id] = l0
	return nil
}

func (m *mockVFS) SystemEntryExists(namespace, category, id string) bool {
	_, ok := m.entries[namespace+"/"+category+"/"+id]
	return ok
}

func TestDistributeChannels(t *testing.T) {
	vfs := newMockVFS()
	result, err := DistributeChannels(vfs)
	if err != nil {
		t.Fatalf("DistributeChannels: %v", err)
	}
	if result.Indexed != len(chatChannelOrder) {
		t.Errorf("expected indexed=%d, got %d", len(chatChannelOrder), result.Indexed)
	}
	// Verify specific channel was written
	if _, ok := vfs.entries["plugins/channels/feishu"]; !ok {
		t.Error("expected feishu channel to be distributed")
	}
	if _, ok := vfs.entries["plugins/channels/telegram"]; !ok {
		t.Error("expected telegram channel to be distributed")
	}
}

func TestGenerateChannelL0(t *testing.T) {
	meta := chatChannelMeta[ChannelFeishu]
	l0 := generateChannelL0(meta)
	if l0 == "" {
		t.Fatal("expected non-empty L0")
	}
	if !containsStr(l0, "Feishu") {
		t.Errorf("L0 should contain Feishu, got: %s", l0)
	}
	if !containsStr(l0, "channel") {
		t.Errorf("L0 should contain type tag, got: %s", l0)
	}
}

func TestGenerateChannelL1(t *testing.T) {
	meta := chatChannelMeta[ChannelTelegram]
	l1 := generateChannelL1(meta)
	if l1 == "" {
		t.Fatal("expected non-empty L1")
	}
	if !containsStr(l1, "telegram") {
		t.Errorf("L1 should contain channel ID, got: %s", l1)
	}
}

func TestCollectDistributedChannelIDs(t *testing.T) {
	ids := CollectDistributedChannelIDs()
	if len(ids) != len(chatChannelOrder) {
		t.Errorf("expected %d IDs, got %d", len(chatChannelOrder), len(ids))
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
