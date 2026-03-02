package nodehost

// config.go — Node Host 配置文件管理
// 对应 TS: src/node-host/config.ts (73L)
//
// 管理 ~/.openacosmi/node.json 的读写，包含 nodeId / displayName / gateway 连接信息。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/openacosmi/claw-acismi/internal/config"
)

// ---------- 类型定义 ----------

// GatewayConfig 远程 gateway 连接配置。
type GatewayConfig struct {
	Host           string `json:"host,omitempty"`
	Port           int    `json:"port,omitempty"`
	TLS            bool   `json:"tls,omitempty"`
	TLSFingerprint string `json:"tlsFingerprint,omitempty"`
}

// Config Node Host 配置。
type Config struct {
	Version     int            `json:"version"`
	NodeID      string         `json:"nodeId"`
	Token       string         `json:"token,omitempty"`
	DisplayName string         `json:"displayName,omitempty"`
	Gateway     *GatewayConfig `json:"gateway,omitempty"`
}

const nodeHostFile = "node.json"

// ---------- 公开 API ----------

// ResolveConfigPath 解析 node host 配置文件路径。
func ResolveConfigPath() string {
	return filepath.Join(config.ResolveStateDir(), nodeHostFile)
}

// LoadConfig 加载 node host 配置。文件不存在或解析失败返回 nil。
func LoadConfig() *Config {
	data, err := os.ReadFile(ResolveConfigPath())
	if err != nil {
		return nil
	}
	var parsed Config
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil
	}
	return normalizeConfig(&parsed)
}

// SaveConfig 持久化 node host 配置（权限 0600）。
func SaveConfig(cfg *Config) error {
	filePath := ResolveConfigPath()
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("mkdir node-host config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal node-host config: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(filePath, data, 0o600); err != nil {
		return fmt.Errorf("write node-host config: %w", err)
	}
	// best-effort chmod
	_ = os.Chmod(filePath, 0o600)
	return nil
}

// EnsureConfig 加载配置，不存在则创建默认值并保存。
func EnsureConfig() (*Config, error) {
	existing := LoadConfig()
	normalized := normalizeConfig(existing)
	if err := SaveConfig(normalized); err != nil {
		return normalized, err
	}
	return normalized, nil
}

// ---------- 内部辅助 ----------

func normalizeConfig(cfg *Config) *Config {
	base := &Config{
		Version: 1,
	}
	if cfg != nil {
		base.Token = cfg.Token
		base.DisplayName = cfg.DisplayName
		base.Gateway = cfg.Gateway
		if cfg.Version == 1 && cfg.NodeID != "" {
			base.NodeID = trimString(cfg.NodeID)
		}
	}
	if base.NodeID == "" {
		base.NodeID = uuid.NewString()
	}
	return base
}

func trimString(s string) string {
	// 简单 trim 封装，避免 import strings 仅为一次调用
	result := ""
	start, end := 0, len(s)-1
	for start <= end && s[start] == ' ' {
		start++
	}
	for end >= start && s[end] == ' ' {
		end--
	}
	if start <= end {
		result = s[start : end+1]
	}
	return result
}
