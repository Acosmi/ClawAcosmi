package gateway

// channel_pairing_bridge.go — 渠道配对存储桥接
// 将各渠道的 UpsertPairingRequest / ReadAllowFromStore 回调桥接到
// device_pairing.go 的已有配对状态管理实现。

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// channelPairingMu 保护渠道配对数据的并发访问。
var channelPairingMu sync.Mutex

// channelPairingEntry 渠道配对条目。
type channelPairingEntry struct {
	Channel   string            `json:"channel"`
	PeerID    string            `json:"peerId"`
	Code      string            `json:"code"`
	Meta      map[string]string `json:"meta,omitempty"`
	CreatedAt int64             `json:"createdAt"`
	Approved  bool              `json:"approved"`
}

// channelPairingStore 渠道配对存储。
type channelPairingStore struct {
	Entries []channelPairingEntry `json:"entries"`
}

// resolveChannelPairingPath 返回渠道配对存储文件路径。
func resolveChannelPairingPath(storePath string) string {
	return filepath.Join(storePath, "pairing", "channel-pairing.json")
}

// loadChannelPairingStore 加载渠道配对存储。
func loadChannelPairingStore(storePath string) (*channelPairingStore, error) {
	filePath := resolveChannelPairingPath(storePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &channelPairingStore{}, nil
		}
		return nil, err
	}
	var store channelPairingStore
	if err := json.Unmarshal(data, &store); err != nil {
		slog.Warn("channel_pairing: parse failed, starting fresh", "error", err)
		return &channelPairingStore{}, nil
	}
	return &store, nil
}

// saveChannelPairingStore 保存渠道配对存储。
func saveChannelPairingStore(storePath string, store *channelPairingStore) error {
	filePath := resolveChannelPairingPath(storePath)
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, filePath)
}

// generatePairingCode 生成 6 字符的配对码。
func generatePairingCode() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return strings.ToUpper(hex.EncodeToString(b))
}

// UpsertChannelPairingRequest 创建或更新渠道配对请求。
// 如果 channel+peerID 已存在则返回现有 code，否则创建新条目。
func UpsertChannelPairingRequest(storePath, channel, peerID string, meta map[string]string) (code string, created bool, err error) {
	channelPairingMu.Lock()
	defer channelPairingMu.Unlock()

	store, err := loadChannelPairingStore(storePath)
	if err != nil {
		return "", false, fmt.Errorf("load pairing store: %w", err)
	}

	// 查找已存在的条目
	normalizedChannel := strings.ToLower(strings.TrimSpace(channel))
	normalizedPeerID := strings.TrimSpace(peerID)
	for _, entry := range store.Entries {
		if strings.ToLower(entry.Channel) == normalizedChannel && entry.PeerID == normalizedPeerID {
			return entry.Code, false, nil
		}
	}

	// 创建新条目
	newCode := generatePairingCode()
	store.Entries = append(store.Entries, channelPairingEntry{
		Channel:   normalizedChannel,
		PeerID:    normalizedPeerID,
		Code:      newCode,
		Meta:      meta,
		CreatedAt: time.Now().UnixMilli(),
	})

	if err := saveChannelPairingStore(storePath, store); err != nil {
		return "", false, fmt.Errorf("save pairing store: %w", err)
	}

	return newCode, true, nil
}

// ReadChannelPairingAllowlist 读取渠道配对允许列表。
// 返回指定渠道中已被批准的 peer ID 列表。
func ReadChannelPairingAllowlist(storePath, channel string) ([]string, error) {
	channelPairingMu.Lock()
	defer channelPairingMu.Unlock()

	store, err := loadChannelPairingStore(storePath)
	if err != nil {
		return nil, err
	}

	normalizedChannel := strings.ToLower(strings.TrimSpace(channel))
	var allowed []string
	for _, entry := range store.Entries {
		if strings.ToLower(entry.Channel) == normalizedChannel && entry.Approved {
			allowed = append(allowed, entry.PeerID)
		}
	}
	return allowed, nil
}
