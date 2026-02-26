package crypto

import (
	"bytes"
	"crypto/rand"
	"testing"
)

// TestRustAES_EncryptDecrypt verifies basic encrypt/decrypt round-trip.
func TestRustAES_EncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	plaintext := []byte("Hello, Argus! This is a secret message for testing AES-GCM.")

	ciphertext, nonce, err := RustAESEncrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if len(nonce) != 12 {
		t.Fatalf("nonce length = %d, want 12", len(nonce))
	}
	if len(ciphertext) == 0 {
		t.Fatal("ciphertext is empty")
	}

	decrypted, err := RustAESDecrypt(key, nonce, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decrypted != plaintext\n  got:  %q\n  want: %q", decrypted, plaintext)
	}
	t.Logf("Plaintext: %q (%d bytes)", plaintext, len(plaintext))
	t.Logf("Ciphertext: %d bytes (includes 16-byte GCM tag)", len(ciphertext))
}

// TestRustAES_WrongKey verifies decryption fails with wrong key.
func TestRustAES_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	plaintext := []byte("secret data")
	ciphertext, nonce, err := RustAESEncrypt(key1, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = RustAESDecrypt(key2, nonce, ciphertext)
	if err == nil {
		t.Error("expected decryption to fail with wrong key")
	} else {
		t.Logf("Correctly failed: %v", err)
	}
}

// TestRustAES_InvalidKeyLength verifies key validation.
func TestRustAES_InvalidKeyLength(t *testing.T) {
	shortKey := make([]byte, 16)
	_, _, err := RustAESEncrypt(shortKey, []byte("test"))
	if err == nil {
		t.Error("expected error for 16-byte key")
	}
}

// TestRustAES_LargeData verifies encryption of larger data blocks.
func TestRustAES_LargeData(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	// 1 MB plaintext
	plaintext := make([]byte, 1024*1024)
	rand.Read(plaintext)

	ciphertext, nonce, err := RustAESEncrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt large data failed: %v", err)
	}

	decrypted, err := RustAESDecrypt(key, nonce, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt large data failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("large data round-trip failed")
	}
	t.Logf("Encrypted/decrypted 1MB successfully")
}

// BenchmarkRustAESEncrypt_1KB benchmarks AES-256-GCM encryption of 1KB.
func BenchmarkRustAESEncrypt_1KB(b *testing.B) {
	key := make([]byte, 32)
	rand.Read(key)
	plaintext := make([]byte, 1024)
	rand.Read(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = RustAESEncrypt(key, plaintext)
	}
}
