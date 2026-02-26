// Package services — 配置值加解密辅助函数。
// 使用 nexus-crypto (AES-256-GCM) 透明加密敏感配置。
package services

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/uhms/go-api/internal/ffi"
)

// encPrefix 标识已加密的配置值。
const encPrefix = "enc:"

// getEncryptionKey 从环境变量读取 32 字节加密密钥（hex 编码）。
func getEncryptionKey() ([]byte, error) {
	raw := os.Getenv("CONFIG_ENCRYPTION_KEY")
	if raw == "" {
		return nil, fmt.Errorf("CONFIG_ENCRYPTION_KEY not set")
	}
	key, err := hex.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid CONFIG_ENCRYPTION_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("CONFIG_ENCRYPTION_KEY must be 32 bytes, got %d", len(key))
	}
	return key, nil
}

// EncryptConfigValue 加密配置值，返回 "enc:<base64>" 格式。
func EncryptConfigValue(plaintext string) (string, error) {
	key, err := getEncryptionKey()
	if err != nil {
		return plaintext, err // 无密钥时返回原文
	}
	ct, err := ffi.AES256GCMEncrypt(key, []byte(plaintext))
	if err != nil {
		return "", fmt.Errorf("encrypt config: %w", err)
	}
	return encPrefix + base64.StdEncoding.EncodeToString(ct), nil
}

// DecryptConfigValue 解密 "enc:<base64>" 格式的值。非加密值原样返回。
func DecryptConfigValue(value string) (string, error) {
	if !strings.HasPrefix(value, encPrefix) {
		return value, nil // 未加密，原样返回
	}
	key, err := getEncryptionKey()
	if err != nil {
		return value, err
	}
	data, err := base64.StdEncoding.DecodeString(value[len(encPrefix):])
	if err != nil {
		return "", fmt.Errorf("decode config: %w", err)
	}
	pt, err := ffi.AES256GCMDecrypt(key, data)
	if err != nil {
		return "", fmt.Errorf("decrypt config: %w", err)
	}
	return string(pt), nil
}

// IsEncrypted 判断值是否已加密。
func IsEncrypted(value string) bool {
	return strings.HasPrefix(value, encPrefix)
}
