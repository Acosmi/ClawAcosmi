// Package middleware — MCP OAuth 2.1 鉴权中间件。
// 使用 MCP Go SDK 的 auth.RequireBearerToken 标准 API 实现。
// 支持双轨验证: API Key (uhms_sk_/mcp_sk_/sk-uhms- 前缀) + JWT Bearer token。
package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// MCPOAuthConfig holds configuration for the MCP OAuth middleware.
type MCPOAuthConfig struct {
	JWTSecret   string // HMAC 签名密钥
	ResourceURL string // 本 MCP Server 的 public URL
	AuthServer  string // Authorization Server issuer URL
}

// MCPOAuth 返回一个 Gin 中间件，使用 SDK auth.RequireBearerToken 校验 MCP 请求。
//
// 支持两种 token 格式:
//   - API Key: uhms_sk_*、mcp_sk_*、sk-uhms-* 前缀
//   - JWT: 标准 JWT token（需 jwtSecret 签名校验）
//
// 校验通过后设置 context:
//   - "user_id": 用户/租户 ID
//   - "token_scopes": 权限范围
//   - "auth_method": "api_key" 或 "jwt"
func MCPOAuth(cfg MCPOAuthConfig) gin.HandlerFunc {
	// Build SDK TokenVerifier with dual-track support.
	verifier := buildTokenVerifier(cfg.JWTSecret)

	// Construct SDK middleware — handles Bearer extraction, WWW-Authenticate, etc.
	sdkMiddleware := auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
		ResourceMetadataURL: "/.well-known/oauth-protected-resource",
		Scopes:              nil, // No mandatory scopes for now
	})

	return func(c *gin.Context) {
		// Dev auth bypass: if user_id already set upstream, skip.
		if _, exists := c.Get("user_id"); exists {
			c.Next()
			return
		}

		// Wrap Gin into standard http.Handler for SDK middleware.
		var passed bool
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			passed = true

			// Extract TokenInfo from context (set by SDK middleware).
			tokenInfo := auth.TokenInfoFromContext(r.Context())
			if tokenInfo != nil {
				c.Set("user_id", tokenInfo.UserID)
				c.Set("token_scopes", tokenInfo.Scopes)

				// Determine auth method from Extra field.
				if method, ok := tokenInfo.Extra["auth_method"].(string); ok {
					c.Set("auth_method", method)
				}
			}

			// Update Gin request context with enriched context.
			c.Request = r
		})

		handler := sdkMiddleware(inner)
		handler.ServeHTTP(c.Writer, c.Request)

		if !passed {
			c.Abort()
			return
		}
		c.Next()
	}
}

// buildTokenVerifier 构建 SDK 兼容的 TokenVerifier。
// 支持 API Key 和 JWT 双轨验证。
func buildTokenVerifier(jwtSecret string) auth.TokenVerifier {
	return func(ctx context.Context, token string, req *http.Request) (*auth.TokenInfo, error) {
		// Track 1: API Key
		if isAPIKey(token) {
			// API Key 格式通过，实际余额/权限校验由 Billing 中间件处理。
			// 这里只做格式确认，设置基本 TokenInfo。
			return &auth.TokenInfo{
				UserID:     "", // Will be set by billing middleware downstream
				Scopes:     []string{"mcp:read", "mcp:write"},
				Expiration: time.Now().Add(24 * time.Hour), // API Key 不过期，设远期值
				Extra:      map[string]any{"auth_method": "api_key"},
			}, nil
		}

		// Track 2: JWT
		if jwtSecret == "" {
			return nil, fmt.Errorf("%w: JWT authentication not configured", auth.ErrInvalidToken)
		}

		claims, err := validateJWT(token, jwtSecret)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", auth.ErrInvalidToken, err)
		}

		expiration := time.Now().Add(1 * time.Hour) // default
		if claims.ExpiresAt != nil {
			expiration = claims.ExpiresAt.Time
		}

		return &auth.TokenInfo{
			UserID:     claims.Subject,
			Scopes:     claims.Scopes,
			Expiration: expiration,
			Extra:      map[string]any{"auth_method": "jwt"},
		}, nil
	}
}

// --- Internal helpers ---

// isAPIKey 判断 token 是否为 API Key 格式。
func isAPIKey(token string) bool {
	return strings.HasPrefix(token, "uhms_sk_") ||
		strings.HasPrefix(token, "mcp_sk_") ||
		strings.HasPrefix(token, "sk-uhms-")
}

// UHMSClaims 是 UHMS JWT 的自定义 claims。
type UHMSClaims struct {
	jwt.RegisteredClaims
	Scopes []string `json:"scopes,omitempty"`
}

// validateJWT 校验 JWT token 并返回 claims。
func validateJWT(tokenStr, secret string) (*UHMSClaims, error) {
	claims := &UHMSClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.ExpiresAt != nil && claims.ExpiresAt.Time.Before(time.Now()) {
		return nil, errors.New("token expired")
	}
	if claims.Subject == "" {
		return nil, errors.New("token missing subject (user_id)")
	}
	return claims, nil
}

// ProtectedResourceMetadata 返回符合 RFC 9728 的元数据 handler。
// 使用 SDK 的 auth.ProtectedResourceMetadataHandler 标准实现。
func ProtectedResourceMetadata(resource, authServer string) gin.HandlerFunc {
	meta := &oauthex.ProtectedResourceMetadata{
		Resource:               resource,
		AuthorizationServers:   []string{authServer},
		ScopesSupported:        []string{"mcp:read", "mcp:write", "mcp:admin"},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "UHMS Memory MCP Server",
	}
	handler := auth.ProtectedResourceMetadataHandler(meta)
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}
