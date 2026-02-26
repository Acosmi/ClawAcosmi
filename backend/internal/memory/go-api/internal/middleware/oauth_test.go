// Package middleware — OAuth 中间件单元测试。
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const testJWTSecret = "test-secret-key-for-unit-tests"

// newTestJWT 生成测试用 JWT token。
func newTestJWT(t *testing.T, subject string, scopes []string, expiresAt time.Time) string {
	t.Helper()
	claims := &UHMSClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Scopes: scopes,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("Failed to sign JWT: %v", err)
	}
	return signed
}

// --- validateJWT ---

func TestValidateJWT_ValidToken(t *testing.T) {
	token := newTestJWT(t, "user-123", []string{"mcp:read"}, time.Now().Add(1*time.Hour))
	claims, err := validateJWT(token, testJWTSecret)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if claims.Subject != "user-123" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user-123")
	}
	if len(claims.Scopes) != 1 || claims.Scopes[0] != "mcp:read" {
		t.Errorf("Scopes = %v, want [mcp:read]", claims.Scopes)
	}
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	token := newTestJWT(t, "user-123", nil, time.Now().Add(-1*time.Hour))
	_, err := validateJWT(token, testJWTSecret)
	if err == nil {
		t.Fatal("Expected error for expired token, got nil")
	}
}

func TestValidateJWT_WrongSecret(t *testing.T) {
	token := newTestJWT(t, "user-123", nil, time.Now().Add(1*time.Hour))
	_, err := validateJWT(token, "wrong-secret")
	if err == nil {
		t.Fatal("Expected error for wrong secret, got nil")
	}
}

func TestValidateJWT_MissingSubject(t *testing.T) {
	token := newTestJWT(t, "", nil, time.Now().Add(1*time.Hour))
	_, err := validateJWT(token, testJWTSecret)
	if err == nil {
		t.Fatal("Expected error for missing subject, got nil")
	}
}

// --- isAPIKey ---

func TestIsAPIKey(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{"uhms_sk_abc123", true},
		{"mcp_sk_xyz789", true},
		{"sk-uhms-test", true},
		{"eyJhbGciOiJIUzI1NiJ9.xxx", false}, // JWT-like
		{"random-string", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isAPIKey(tt.token)
		if got != tt.want {
			t.Errorf("isAPIKey(%q) = %v, want %v", tt.token, got, tt.want)
		}
	}
}

// Note: extractBearerToken is no longer a standalone function — SDK's
// auth.RequireBearerToken handles bearer extraction internally.

// --- MCPOAuth middleware integration ---

func TestMCPOAuth_NoAuthHeader(t *testing.T) {
	// Request without Authorization header should get 401.
	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	r.Use(MCPOAuth(MCPOAuthConfig{JWTSecret: testJWTSecret}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})
	c.Request = httptest.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, c.Request)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestMCPOAuth_ValidJWT(t *testing.T) {
	token := newTestJWT(t, "tenant-abc", []string{"mcp:read", "mcp:write"}, time.Now().Add(1*time.Hour))

	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(MCPOAuth(MCPOAuthConfig{JWTSecret: testJWTSecret}))
	r.GET("/test", func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		method, _ := c.Get("auth_method")
		c.JSON(200, gin.H{"user_id": uid, "auth_method": method})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestMCPOAuth_APIKeyPassesThrough(t *testing.T) {
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(MCPOAuth(MCPOAuthConfig{JWTSecret: testJWTSecret}))
	r.GET("/test", func(c *gin.Context) {
		method, _ := c.Get("auth_method")
		c.JSON(200, gin.H{"auth_method": method})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer uhms_sk_test123")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d. Body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestMCPOAuth_DevAuthBypass(t *testing.T) {
	// If user_id already set (dev auth bypass), middleware should pass through.
	w := httptest.NewRecorder()
	_, r := gin.CreateTestContext(w)
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "dev-user")
		c.Next()
	})
	r.Use(MCPOAuth(MCPOAuthConfig{JWTSecret: testJWTSecret}))
	r.GET("/test", func(c *gin.Context) {
		uid, _ := c.Get("user_id")
		c.JSON(200, gin.H{"user_id": uid})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	// No Authorization header needed.
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}
}
