// types/oauth.go — OAuth 凭证类型定义
// 对应外部包 @mariozechner/pi-ai 中的 OAuthCredentials 类型，
// 在 Go 中本地化定义以消除外部依赖。
package types

// OAuthCredentials 表示可刷新的 OAuth 凭证信息。
// 对应 TS 类型: { refresh: string; access: string; expires: number }
type OAuthCredentials struct {
	// Refresh 刷新令牌
	Refresh string `json:"refresh"`
	// Access 访问令牌
	Access string `json:"access"`
	// Expires 过期时间戳（毫秒级 Unix 时间）
	Expires int64 `json:"expires"`
}
