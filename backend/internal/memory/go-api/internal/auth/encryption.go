package auth

// Encryption provides Fernet-compatible symmetric encryption.
// Mirrors Python core/encryption.py: load_or_create_key, encrypt/decrypt.

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fernet/fernet-go"
)

const (
	encryptionKeyEnv = "ENCRYPTION_KEY"
	secretFileName   = ".secret_key"
)

var (
	fernetKey  *fernet.Key
	fernetOnce sync.Once
)

// initFernet loads or creates the Fernet encryption key (singleton).
func initFernet() {
	fernetOnce.Do(func() {
		keyStr := loadOrCreateKey()
		k, err := fernet.DecodeKey(keyStr)
		if err != nil {
			slog.Error("Failed to decode Fernet key", "error", err)
			panic(fmt.Sprintf("encryption initialization failed: %v", err))
		}
		fernetKey = k
		slog.Info("Encryption module initialized")
	})
}

// loadOrCreateKey mirrors the Python strategy:
// 1. Check ENCRYPTION_KEY env var
// 2. Check local .secret_key file
// 3. Generate new key and persist to .secret_key
func loadOrCreateKey() string {
	// 1. Try environment variable
	if key := os.Getenv(encryptionKeyEnv); key != "" {
		return key
	}

	// 2. Determine secret file path (relative to executable or working directory)
	secretPath := filepath.Join(".", secretFileName)

	// Try to read from local file
	if data, err := os.ReadFile(secretPath); err == nil {
		key := strings.TrimSpace(string(data))
		if key != "" {
			slog.Info("Loaded encryption key from local file", "path", secretPath)
			return key
		}
	}

	// 3. Generate new key and persist
	slog.Warn("No encryption key found, generating new key",
		"env_var", encryptionKeyEnv, "file", secretPath)

	newKey := generateFernetKey()

	if err := os.WriteFile(secretPath, []byte(newKey), 0600); err != nil {
		slog.Error("Failed to save encryption key to file", "error", err)
	} else {
		slog.Info("New encryption key saved", "path", secretPath)
	}

	return newKey
}

// generateFernetKey generates a new Fernet-compatible key.
func generateFernetKey() string {
	var key fernet.Key
	if _, err := rand.Read(key[:]); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return key.Encode()
}

// EncryptValue encrypts a plain-text string using Fernet.
// Returns empty string for empty input.
func EncryptValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	initFernet()

	tok, err := fernet.EncryptAndSign([]byte(value), fernetKey)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}
	return string(tok), nil
}

// DecryptValue decrypts a Fernet-encrypted token.
// Returns empty string on invalid token (fail-safe, matches Python behavior).
func DecryptValue(token string) string {
	if token == "" {
		return ""
	}
	initFernet()

	msg := fernet.VerifyAndDecrypt([]byte(token), 0, []*fernet.Key{fernetKey})
	if msg == nil {
		slog.Warn("Decryption failed: invalid token (key mismatch or corrupted data)")
		return ""
	}
	return string(msg)
}

// IsEncrypted checks if a value looks like a Fernet-encrypted token.
func IsEncrypted(value string) bool {
	if len(value) < 50 {
		return false
	}
	// Fernet tokens are base64url-encoded and start with a version byte
	_, err := base64.URLEncoding.DecodeString(value)
	return err == nil && strings.HasPrefix(value, "gAAAAA")
}
