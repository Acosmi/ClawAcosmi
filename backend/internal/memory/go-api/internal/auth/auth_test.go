// Package auth — unit tests for JWT token management and password hashing.
package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/uhms/go-api/internal/config"
)

// setupTestConfig initializes config for testing.
func setupTestConfig() {
	cfg := &config.Config{
		JWTSecretKey:         "test-secret-key-for-jwt-testing-32bytes",
		AccessTokenExpireMin: 30,
	}
	config.SetForTest(cfg)
}

// TestHashPassword verifies bcrypt hashing produces valid output.
func TestHashPassword(t *testing.T) {
	password := "my-secure-password-123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty hash")
	}
	if hash == password {
		t.Fatal("Hash should not equal plaintext password")
	}
}

// TestVerifyPasswordCorrect verifies correct password matches its hash.
func TestVerifyPasswordCorrect(t *testing.T) {
	password := "correct-password"
	hash, _ := HashPassword(password)

	if !VerifyPassword(password, hash) {
		t.Fatal("VerifyPassword should return true for correct password")
	}
}

// TestVerifyPasswordIncorrect verifies incorrect password is rejected.
func TestVerifyPasswordIncorrect(t *testing.T) {
	hash, _ := HashPassword("correct-password")

	if VerifyPassword("wrong-password", hash) {
		t.Fatal("VerifyPassword should return false for wrong password")
	}
}

// TestCreateAccessToken verifies token creation and structure.
func TestCreateAccessToken(t *testing.T) {
	setupTestConfig()

	token, err := CreateAccessToken("user-123", "admin")
	if err != nil {
		t.Fatalf("CreateAccessToken failed: %v", err)
	}
	if token == "" {
		t.Fatal("Token should not be empty")
	}

	// JWT has 3 parts separated by dots
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT should have 3 parts, got %d", len(parts))
	}
}

// TestVerifyTokenValid verifies a valid token can be decoded.
func TestVerifyTokenValid(t *testing.T) {
	setupTestConfig()

	token, _ := CreateAccessToken("user-456", "user")
	data, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if data.UserID != "user-456" {
		t.Errorf("Expected UserID 'user-456', got '%s'", data.UserID)
	}
	if data.Role != "user" {
		t.Errorf("Expected Role 'user', got '%s'", data.Role)
	}
}

// TestVerifyTokenExpired verifies expired tokens are rejected.
func TestVerifyTokenExpired(t *testing.T) {
	setupTestConfig()

	// Create token that expires in 1 nanosecond
	token, _ := CreateAccessToken("user-789", "user", 1*time.Nanosecond)
	time.Sleep(10 * time.Millisecond) // Wait for expiration

	_, err := VerifyToken(token)
	if err == nil {
		t.Fatal("VerifyToken should fail for expired token")
	}
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}
}

// TestVerifyTokenInvalid verifies garbage token is rejected.
func TestVerifyTokenInvalid(t *testing.T) {
	setupTestConfig()

	_, err := VerifyToken("not.a.valid.jwt")
	if err == nil {
		t.Fatal("VerifyToken should fail for invalid token")
	}
}

// TestVerifyTokenDefaultRole verifies default role fallback.
func TestVerifyTokenDefaultRole(t *testing.T) {
	setupTestConfig()

	// Create token with empty role — should default to "user"
	token, _ := CreateAccessToken("user-default", "")
	data, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if data.Role != "user" {
		t.Errorf("Expected default Role 'user', got '%s'", data.Role)
	}
}
