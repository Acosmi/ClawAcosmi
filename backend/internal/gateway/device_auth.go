package gateway

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math"
	"strings"
	"time"
)

// ---------- 设备认证工具 ----------
// 对齐 TS gateway/device-auth.ts + infra/device-identity.ts

// DeviceSignatureSkewMs 签名有效期窗口（10 分钟）。
const DeviceSignatureSkewMs = 10 * 60 * 1000

// ---------- base64url 工具 ----------

func base64UrlEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64UrlDecode(input string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(input)
}

// ---------- Ed25519 SPKI 前缀 ----------

// ed25519SPKIPrefix 是 Ed25519 SubjectPublicKeyInfo DER 编码的固定前缀（12 bytes）。
var ed25519SPKIPrefix = []byte{
	0x30, 0x2a, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65,
	0x70, 0x03, 0x21, 0x00,
}

// ---------- 公钥操作 ----------

// derivePublicKeyRaw 从 PEM 公钥中提取 32 字节原始 Ed25519 公钥。
func derivePublicKeyRaw(publicKeyPem string) ([]byte, error) {
	block, _ := pem.Decode([]byte(publicKeyPem))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse PKIX public key: %w", err)
	}
	edKey, ok := pub.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an Ed25519 public key")
	}
	return []byte(edKey), nil
}

// parsePublicKey 解析 PEM 或 base64url 格式的 Ed25519 公钥。
func parsePublicKey(publicKey string) (ed25519.PublicKey, error) {
	if strings.Contains(publicKey, "BEGIN") {
		raw, err := derivePublicKeyRaw(publicKey)
		if err != nil {
			return nil, err
		}
		return ed25519.PublicKey(raw), nil
	}
	// base64url raw key
	raw, err := base64UrlDecode(publicKey)
	if err != nil {
		return nil, fmt.Errorf("decode base64url public key: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid Ed25519 public key size: %d", len(raw))
	}
	return ed25519.PublicKey(raw), nil
}

// DeriveDeviceIdFromPublicKey 从公钥派生 deviceId（SHA256 hex）。
// 对齐 TS infra/device-identity.ts deriveDeviceIdFromPublicKey。
func DeriveDeviceIdFromPublicKey(publicKey string) (string, error) {
	var raw []byte
	var err error
	if strings.Contains(publicKey, "BEGIN") {
		raw, err = derivePublicKeyRaw(publicKey)
	} else {
		raw, err = base64UrlDecode(publicKey)
	}
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

// NormalizeDevicePublicKeyBase64Url 标准化公钥为 base64url 格式。
func NormalizeDevicePublicKeyBase64Url(publicKey string) (string, error) {
	if strings.Contains(publicKey, "BEGIN") {
		raw, err := derivePublicKeyRaw(publicKey)
		if err != nil {
			return "", err
		}
		return base64UrlEncode(raw), nil
	}
	raw, err := base64UrlDecode(publicKey)
	if err != nil {
		return "", err
	}
	return base64UrlEncode(raw), nil
}

// ---------- 签名载荷构建 ----------

// DeviceAuthPayloadParams 构建签名载荷的参数。
type DeviceAuthPayloadParams struct {
	DeviceID   string
	ClientID   string
	ClientMode string
	Role       string
	Scopes     []string
	SignedAtMs int64
	Token      string
	Nonce      string
	Version    string // "v1" or "v2"
}

// BuildDeviceAuthPayload 构建用于签名的载荷字符串。
// 对齐 TS gateway/device-auth.ts buildDeviceAuthPayload。
func BuildDeviceAuthPayload(params DeviceAuthPayloadParams) string {
	version := params.Version
	if version == "" {
		if params.Nonce != "" {
			version = "v2"
		} else {
			version = "v1"
		}
	}
	scopes := strings.Join(params.Scopes, ",")
	token := params.Token

	parts := []string{
		version,
		params.DeviceID,
		params.ClientID,
		params.ClientMode,
		params.Role,
		scopes,
		fmt.Sprintf("%d", params.SignedAtMs),
		token,
	}
	if version == "v2" {
		parts = append(parts, params.Nonce)
	}
	return strings.Join(parts, "|")
}

// ---------- 签名验证 ----------

// VerifyDeviceSignature 验证设备签名。
// 对齐 TS infra/device-identity.ts verifyDeviceSignature。
func VerifyDeviceSignature(publicKey string, payload string, signatureBase64Url string) bool {
	key, err := parsePublicKey(publicKey)
	if err != nil {
		return false
	}
	sig, err := base64UrlDecode(signatureBase64Url)
	if err != nil {
		// 回退到标准 base64
		sig, err = base64.StdEncoding.DecodeString(signatureBase64Url)
		if err != nil {
			return false
		}
	}
	return ed25519.Verify(key, []byte(payload), sig)
}

// ---------- 时间戳验证 ----------

// IsSignedAtValid 检查签名时间戳是否在有效窗口内。
func IsSignedAtValid(signedAtMs int64) bool {
	nowMs := time.Now().UnixMilli()
	diff := nowMs - signedAtMs
	if diff < 0 {
		diff = -diff
	}
	return diff <= DeviceSignatureSkewMs
}

// ---------- 完整设备认证 ----------

// DeviceAuthResult 设备认证结果。
type DeviceAuthResult struct {
	OK       bool
	Reason   string
	DeviceID string
}

// ValidateDeviceAuth 执行完整的设备认证：ID 派生验证 + 时间戳检查 + 签名验证。
// 对齐 TS message-handler.ts L401-659 的核心验证逻辑。
func ValidateDeviceAuth(device *ConnectDeviceAuth, clientID, clientMode, role string, scopes []string, token string, isLocal bool) DeviceAuthResult {
	if device == nil {
		return DeviceAuthResult{OK: true} // no device = skip
	}

	// 1. 派生 deviceId 验证
	derivedID, err := DeriveDeviceIdFromPublicKey(device.PublicKey)
	if err != nil {
		return DeviceAuthResult{OK: false, Reason: "invalid device public key"}
	}
	if derivedID != device.ID {
		return DeviceAuthResult{OK: false, Reason: "device ID does not match public key"}
	}

	// 2. 时间戳检查
	if !IsSignedAtValid(device.SignedAt) {
		return DeviceAuthResult{OK: false, Reason: fmt.Sprintf("device signature expired (skew=%dms, max=%dms)",
			int64(math.Abs(float64(time.Now().UnixMilli()-device.SignedAt))), int64(DeviceSignatureSkewMs))}
	}

	// 3. 构建 v2 payload（有 nonce）或 v1 payload（无 nonce）
	payload := BuildDeviceAuthPayload(DeviceAuthPayloadParams{
		DeviceID:   device.ID,
		ClientID:   clientID,
		ClientMode: clientMode,
		Role:       role,
		Scopes:     scopes,
		SignedAtMs: device.SignedAt,
		Token:      token,
		Nonce:      device.Nonce,
	})

	if VerifyDeviceSignature(device.PublicKey, payload, device.Signature) {
		return DeviceAuthResult{OK: true, DeviceID: derivedID}
	}

	// 4. 回退：本地连接且无 nonce 时尝试 v1
	if isLocal && device.Nonce == "" {
		payloadV1 := BuildDeviceAuthPayload(DeviceAuthPayloadParams{
			DeviceID:   device.ID,
			ClientID:   clientID,
			ClientMode: clientMode,
			Role:       role,
			Scopes:     scopes,
			SignedAtMs: device.SignedAt,
			Token:      token,
			Version:    "v1",
		})
		if VerifyDeviceSignature(device.PublicKey, payloadV1, device.Signature) {
			return DeviceAuthResult{OK: true, DeviceID: derivedID}
		}
	}

	return DeviceAuthResult{OK: false, Reason: "device signature verification failed"}
}
