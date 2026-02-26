package argus

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindAppBundleBinary_NoBundle(t *testing.T) {
	// 在没有 .app bundle 的环境下应返回空字符串
	result := FindAppBundleBinary()
	// 不做断言（CI 可能有也可能没有），仅验证不 panic
	t.Logf("FindAppBundleBinary() = %q", result)
}

func TestEnsureCodeSigned_NonExistentFile(t *testing.T) {
	err := EnsureCodeSigned("/nonexistent/argus-sensory-xyz-12345")
	if runtime.GOOS == "darwin" {
		// macOS 上对不存在的文件应该不 panic，可能返回错误
		t.Logf("EnsureCodeSigned on nonexistent file: err=%v", err)
	} else {
		// 非 macOS 应该是 no-op
		if err != nil {
			t.Errorf("expected nil error on non-darwin, got %v", err)
		}
	}
}

func TestEnsureCodeSigned_InsideAppBundle(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}

	// 创建假 .app 结构
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "Test.app", "Contents", "MacOS")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(appDir, "test-binary")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	// .app bundle 内的二进制应被跳过（不尝试签名）
	err := EnsureCodeSigned(binPath)
	if err != nil {
		t.Errorf("expected nil for .app bundle binary, got %v", err)
	}
}

func TestIsInsideAppBundle(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-only test")
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"/Applications/Argus.app/Contents/MacOS/argus-sensory", true},
		{"/Users/test/Argus.app/Contents/MacOS/sensory-server", true},
		{"/usr/local/bin/argus-sensory", false},
		{"/tmp/argus-sensory", false},
		{"argus-sensory", false},
	}

	for _, tc := range tests {
		got := isInsideAppBundle(tc.path)
		if got != tc.expected {
			t.Errorf("isInsideAppBundle(%q): expected %v, got %v", tc.path, tc.expected, got)
		}
	}
}
