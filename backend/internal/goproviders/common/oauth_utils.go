// common/oauth_utils.go — OAuth 工具函数
// 对应 TS 文件: src/plugin-sdk/oauth-utils.ts
package common

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
)

// ToFormURLEncoded 将键值对编码为 application/x-www-form-urlencoded 格式字符串。
// 对应 TS: toFormUrlEncoded()
func ToFormURLEncoded(data map[string]string) string {
	values := url.Values{}
	for k, v := range data {
		values.Set(k, v)
	}
	return values.Encode()
}

// PKCEVerifierChallenge PKCE 验证器和挑战码对。
type PKCEVerifierChallenge struct {
	// Verifier 随机生成的验证器
	Verifier string
	// Challenge 验证器的 SHA-256 哈希值（base64url 编码）
	Challenge string
}

// GeneratePKCEVerifierChallenge 生成 PKCE 验证器和挑战码。
// 对应 TS: generatePkceVerifierChallenge()
func GeneratePKCEVerifierChallenge() (PKCEVerifierChallenge, error) {
	// 生成 32 字节随机数据
	randomData := make([]byte, 32)
	if _, err := rand.Read(randomData); err != nil {
		return PKCEVerifierChallenge{}, err
	}

	// base64url 编码（无填充）作为 verifier
	verifier := base64.RawURLEncoding.EncodeToString(randomData)

	// SHA-256 哈希后 base64url 编码作为 challenge
	hash := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(hash[:])

	return PKCEVerifierChallenge{
		Verifier:  verifier,
		Challenge: challenge,
	}, nil
}
