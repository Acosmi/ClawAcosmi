package auth

// auth.go provides API Key authentication, session/bearer token authentication,
// and resource access control. Mirrors Python core/auth.py.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/uhms/go-api/internal/config"
	"github.com/uhms/go-api/internal/database"
)

// HashAPIKey computes SHA-256 hash of an API key for secure storage and comparison.
func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// --- Session User Model ---

// SessionUser represents a user authenticated via session/bearer token.
type SessionUser struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

// --- Gin Context Keys ---

const (
	ContextKeyUser   = "current_user"
	ContextKeyAPIKey = "api_key"
	ContextKeyUserID = "user_id"
)

// --- Dual-Track Bearer Authentication Middleware ---

// GetCurrentUser extracts and validates the current user from Bearer token.
// Supports dual-track auth: dev bypass (Track A) and JWT verification (Track B).
// Must be called as a Gin handler; sets user in context.
func GetCurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Get()

		// Extract Bearer token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"detail": "未提供认证凭证",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		// Track A: Dev Bypass (双重守卫：ALLOW_DEV_AUTH_BYPASS + Debug 模式)
		// 仅在 Debug 模式下允许 dev bypass，生产环境始终走 JWT 验证
		if cfg.AllowDevAuthBypass && gin.Mode() == gin.DebugMode && strings.HasPrefix(token, "dev_") {
			parts := strings.Split(token, "_")
			if len(parts) < 3 {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"detail": "Invalid dev token format. Expected: dev_{user_id}_{role}",
				})
				return
			}

			role := parts[len(parts)-1]
			userID := strings.Join(parts[1:len(parts)-1], "_")

			if role != "user" && role != "admin" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"detail": fmt.Sprintf("Invalid role in dev token: %s. Must be 'user' or 'admin'", role),
				})
				return
			}

			user := &SessionUser{
				UserID: userID,
				Email:  fmt.Sprintf("%s@dev.local", userID),
				Name:   fmt.Sprintf("Dev User %s", userID),
				Role:   role,
			}

			c.Set(ContextKeyUser, user)
			c.Set(ContextKeyUserID, userID)
			slog.Warn("[AUTH] Dev bypass authentication — DEBUG MODE ONLY", "user_id", userID, "role", role)
			c.Next()
			return
		}

		// Track B: Production JWT Verification
		tokenData, err := VerifyToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"detail": fmt.Sprintf("Token 验证失败: %v", err),
			})
			return
		}

		user := &SessionUser{
			UserID: tokenData.UserID,
			Email:  fmt.Sprintf("%s@platform.local", tokenData.UserID),
			Name:   fmt.Sprintf("User %s", tokenData.UserID),
			Role:   tokenData.Role,
		}

		c.Set(ContextKeyUser, user)
		c.Set(ContextKeyUserID, tokenData.UserID)
		c.Next()
	}
}

// RequireAdmin is a middleware that requires the current user to be an admin.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get(ContextKeyUser)
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "未认证"})
			return
		}

		sessionUser, ok := user.(*SessionUser)
		if !ok || sessionUser.Role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"detail": "需要管理员权限",
			})
			return
		}

		c.Next()
	}
}

// --- API Key Authentication ---

// VerifyAPIKey is a middleware that validates the X-API-Key header.
// Queries the database for key existence, active status, and expiration.
func VerifyAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")

		// 1. Check presence
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"detail": "API key is required",
			})
			return
		}

		// 2. Validate format
		if !strings.HasPrefix(apiKey, "sk-uhms-") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"detail": "Invalid API key format",
			})
			return
		}

		// 3. Query database
		db, err := database.GetDB()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"detail": "Database unavailable",
			})
			return
		}

		// ApiKeyRecord is a lightweight struct for key validation.
		type ApiKeyRecord struct {
			ID             string     `gorm:"column:id"`
			KeyHash        string     `gorm:"column:key_hash"`
			Key            string     `gorm:"column:key"` // 兼容：旧明文字段
			UserID         string     `gorm:"column:user_id"`
			Role           string     `gorm:"column:role"`
			IsActive       bool       `gorm:"column:is_active"`
			ExpiresAt      *time.Time `gorm:"column:expires_at"`
			LastUsedAt     *time.Time `gorm:"column:last_used_at"`
			AllowedIPs     string     `gorm:"column:allowed_ips"`
			AllowedDomains string     `gorm:"column:allowed_domains"`
		}

		// 安全策略：优先用 SHA-256 哈希匹配，兼容旧明文 key 字段
		keyHash := HashAPIKey(apiKey)
		var record ApiKeyRecord
		result := db.Table("api_keys").Where("key_hash = ?", keyHash).First(&record)
		if result.Error != nil {
			// 兼容降级：尝试旧明文 key 匹配（迁移过渡期）
			result = db.Table("api_keys").Where("key = ? AND (key_hash IS NULL OR key_hash = '')", apiKey).First(&record)
			if result.Error != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
					"detail": "Invalid or revoked API key",
				})
				return
			}
			// 自动迁移：将旧明文 key 的哈希写入 key_hash 字段
			db.Table("api_keys").Where("id = ?", record.ID).Updates(map[string]any{
				"key_hash": keyHash,
			})
			slog.Info("[AUTH] Auto-migrated API key to hash storage", "key_id", record.ID)
		}

		// 4. Check active status
		if !record.IsActive {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"detail": "API key has been revoked",
			})
			return
		}

		// 5. Check expiration
		if record.ExpiresAt != nil && record.ExpiresAt.Before(time.Now()) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"detail": "API key has expired",
			})
			return
		}

		// 6. Set context
		c.Set(ContextKeyAPIKey, &record)
		c.Set(ContextKeyUserID, record.UserID)

		// 7. Update last_used_at
		now := time.Now()
		db.Table("api_keys").Where("id = ?", record.ID).Update("last_used_at", now)

		c.Next()
	}
}

// --- Network Origin Verification ---

// IsDomainAllowed checks if an origin matches any allowed domain pattern.
// Supports exact match and wildcard subdomains (*.example.com).
func IsDomainAllowed(origin string, allowedDomains []string) bool {
	if origin == "" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	host := parsed.Hostname()
	if host == "" {
		host = origin
	}

	for _, pattern := range allowedDomains {
		if strings.HasPrefix(pattern, "*.") {
			baseDomain := pattern[2:]
			if host == baseDomain || strings.HasSuffix(host, "."+baseDomain) {
				return true
			}
		} else if host == pattern {
			return true
		}
	}
	return false
}

// --- Resource Access Control ---

// VerifyResourceAccess checks if the authenticated entity has access to a target user's resources.
// Admin role can access any user's resources; regular users can only access their own.
func VerifyResourceAccess(targetUserID string, authUserID string, authRole string) error {
	if authRole == "admin" {
		return nil
	}
	if authUserID != targetUserID {
		return fmt.Errorf("无权访问用户 '%s' 的资源", targetUserID)
	}
	return nil
}

// ExtractCurrentUser extracts the SessionUser from the Gin context.
// Returns nil if no user is set.
func ExtractCurrentUser(c *gin.Context) *SessionUser {
	user, exists := c.Get(ContextKeyUser)
	if !exists {
		return nil
	}
	sessionUser, ok := user.(*SessionUser)
	if !ok {
		return nil
	}
	return sessionUser
}
