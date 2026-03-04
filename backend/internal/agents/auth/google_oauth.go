package auth

// google_oauth.go — Google Gemini OAuth 常量
//
// ClientID 来源：google-gemini/gemini-cli（Apache-2.0 开源项目）
// 适用于桌面/CLI PKCE OAuth 流程（loopback redirect，无需 client_secret）。
// 该 ClientID 授权访问 Google Generative Language API（Code Assist 免费配额）。
//
// 参考：https://github.com/google-gemini/gemini-cli

// Google OAuth 常量。
const (
	// GeminiOAuthClientID Google 官方 gemini-cli 的 OAuth Client ID（Apache-2.0）。
	// 可用于 PKCE 授权流程，无需 client_secret。
	GeminiOAuthClientID = "681255809395-oo8ft2oprdrnp9e3aqf6av3hmdib135j.apps.googleusercontent.com"

	// GeminiOAuthClientSecret — Google Desktop OAuth 即使是公共客户端也需要 client_secret 进行 token exchange。
	// 来源: gemini-cli（Apache-2.0 开源），Google 明确说明桌面应用的 secret 不视为机密。
	GeminiOAuthClientSecret = "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"

	// GeminiOAuthScope — gemini-cli 实际使用的 scope 是 cloud-platform（而非 generative-language）。
	// generative-language scope 未在该 ClientID 的 GCP 项目 OAuth 同意屏幕中注册，
	// 请求它会导致 403 restricted_client / Unregistered scope 错误。
	GeminiOAuthScope = "https://www.googleapis.com/auth/cloud-platform"
)
