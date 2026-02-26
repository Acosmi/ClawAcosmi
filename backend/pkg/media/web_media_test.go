package media

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWebMedia_EmptySource(t *testing.T) {
	_, err := LoadWebMedia("", 0)
	if err == nil {
		t.Fatal("expected error for empty source")
	}
}

func TestLoadWebMedia_WhitespaceSource(t *testing.T) {
	_, err := LoadWebMedia("   ", 0)
	if err == nil {
		t.Fatal("expected error for whitespace source")
	}
}

func TestLoadWebMedia_LocalFile(t *testing.T) {
	// 创建临时文件
	tmp, err := os.CreateTemp("", "media_test_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	content := []byte("hello media")
	if _, err := tmp.Write(content); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	m, err := LoadWebMedia(tmp.Name(), 0)
	if err != nil {
		t.Fatalf("LoadWebMedia failed: %v", err)
	}
	if string(m.Buffer) != "hello media" {
		t.Errorf("unexpected buffer: %q", string(m.Buffer))
	}
	if m.Filename != filepath.Base(tmp.Name()) {
		t.Errorf("unexpected filename: %q", m.Filename)
	}
}

func TestLoadWebMedia_LocalFileTooLarge(t *testing.T) {
	tmp, err := os.CreateTemp("", "media_test_*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())

	// 写入超过限制的数据
	data := make([]byte, 100)
	if _, err := tmp.Write(data); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	_, err = LoadWebMedia(tmp.Name(), 50) // 限制 50 字节
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
}

func TestLoadWebMedia_NonExistentFile(t *testing.T) {
	_, err := LoadWebMedia("/nonexistent/file.jpg", 0)
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestDetectContentType_ByExtension(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"image.jpg", "image/jpeg"},
		{"audio.mp3", "audio/mpeg"},
		{"video.mp4", "video/mp4"},
	}
	for _, tt := range tests {
		got := DetectContentType(tt.path, []byte{0xFF, 0xD8, 0xFF}) // JPEG magic bytes
		if got != tt.want {
			t.Errorf("DetectContentType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestDetectContentType_FallbackToMagic(t *testing.T) {
	// 无扩展名，应该使用 magic bytes
	got := DetectContentType("noext", []byte{0xFF, 0xD8, 0xFF, 0xE0})
	if got != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %q", got)
	}
}

func TestResolveMediaKind(t *testing.T) {
	tests := []struct {
		ct   string
		want string
	}{
		{"image/webp", "sticker"},
		{"image/jpeg", "image"},
		{"image/png", "image"},
		{"video/mp4", "video"},
		{"audio/mpeg", "audio"},
		{"application/pdf", "document"},
		{"text/plain", "document"},
	}
	for _, tt := range tests {
		got := ResolveMediaKind(tt.ct)
		if got != tt.want {
			t.Errorf("ResolveMediaKind(%q) = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestExtractURLPath(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/image.jpg", "/image.jpg"},
		{"https://example.com/path/to/file.png?w=100", "/path/to/file.png"},
		{"https://example.com", ""},
		{"noscheme", ""},
	}
	for _, tt := range tests {
		got := extractURLPath(tt.url)
		if got != tt.want {
			t.Errorf("extractURLPath(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
