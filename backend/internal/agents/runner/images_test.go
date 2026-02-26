package runner

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectImageReferences_AbsolutePath(t *testing.T) {
	refs := DetectImageReferences("Look at this /home/user/photo.png please")
	if len(refs) == 0 {
		t.Fatal("expected at least 1 ref")
	}
	found := false
	for _, ref := range refs {
		if ref.Resolved == "/home/user/photo.png" && ref.Type == ImageRefPath {
			found = true
		}
	}
	if !found {
		t.Errorf("expected absolute path ref /home/user/photo.png, got %+v", refs)
	}
}

func TestDetectImageReferences_FileURL(t *testing.T) {
	refs := DetectImageReferences("Check file:///tmp/test/image.jpg here")
	found := false
	for _, ref := range refs {
		if ref.Resolved == "/tmp/test/image.jpg" && ref.Type == ImageRefPath {
			found = true
		}
	}
	if !found {
		t.Errorf("expected file URL ref, got %+v", refs)
	}
}

func TestDetectImageReferences_MediaAttached(t *testing.T) {
	refs := DetectImageReferences("[media attached: /var/data/screenshot.png (image/png) | 1024]")
	found := false
	for _, ref := range refs {
		if ref.Resolved == "/var/data/screenshot.png" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected media attached ref, got %+v", refs)
	}
}

func TestDetectImageReferences_Dedup(t *testing.T) {
	refs := DetectImageReferences("/home/user/photo.png and again /home/user/photo.png")
	count := 0
	for _, ref := range refs {
		if ref.Resolved == "/home/user/photo.png" {
			count++
		}
	}
	if count > 1 {
		t.Errorf("expected dedup, got %d refs for same path", count)
	}
}

func TestDetectImageReferences_NonImage(t *testing.T) {
	refs := DetectImageReferences("Look at /home/user/document.pdf please")
	for _, ref := range refs {
		if ref.Resolved == "/home/user/document.pdf" {
			t.Error("should not detect non-image files")
		}
	}
}

func TestModelSupportsImages(t *testing.T) {
	if !ModelSupportsImages([]string{"text", "image"}) {
		t.Error("should return true when image in list")
	}
	if ModelSupportsImages([]string{"text"}) {
		t.Error("should return false when image not in list")
	}
	if ModelSupportsImages(nil) {
		t.Error("should return false for nil")
	}
}

func TestLoadImageFromRef_NotFound(t *testing.T) {
	ref := DetectedImageRef{
		Raw:      "/nonexistent/image.png",
		Type:     ImageRefPath,
		Resolved: "/nonexistent/image.png",
	}
	result := LoadImageFromRef(ref, "/tmp", nil)
	if result != nil {
		t.Error("should return nil for nonexistent file")
	}
}

func TestLoadImageFromRef_Success(t *testing.T) {
	// 创建临时图片文件
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.png")
	imgData := []byte("fake-png-data")
	if err := os.WriteFile(imgPath, imgData, 0o644); err != nil {
		t.Fatal(err)
	}

	ref := DetectedImageRef{
		Raw:      imgPath,
		Type:     ImageRefPath,
		Resolved: imgPath,
	}
	result := LoadImageFromRef(ref, dir, nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != "image" {
		t.Errorf("expected type=image, got %s", result.Type)
	}
	decoded, err := base64.StdEncoding.DecodeString(result.Data)
	if err != nil {
		t.Fatalf("invalid base64: %v", err)
	}
	if string(decoded) != "fake-png-data" {
		t.Errorf("data mismatch: %s", string(decoded))
	}
}

func TestLoadImageFromRef_SandboxBlocked(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	imgPath := filepath.Join(outside, "escape.png")
	os.WriteFile(imgPath, []byte("data"), 0o644)

	ref := DetectedImageRef{
		Raw:      imgPath,
		Type:     ImageRefPath,
		Resolved: imgPath,
	}
	result := LoadImageFromRef(ref, dir, &LoadImageFromRefOptions{SandboxRoot: dir})
	if result != nil {
		t.Error("should block access outside sandbox")
	}
}

func TestLoadImageFromRef_SizeLimit(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "big.png")
	os.WriteFile(imgPath, make([]byte, 1024), 0o644)

	ref := DetectedImageRef{
		Raw:      imgPath,
		Type:     ImageRefPath,
		Resolved: imgPath,
	}
	result := LoadImageFromRef(ref, dir, &LoadImageFromRefOptions{MaxBytes: 100})
	if result != nil {
		t.Error("should reject files exceeding size limit")
	}
}

func TestDetectImagesFromHistory(t *testing.T) {
	messages := []HistoryMessage{
		{Role: "user", Text: "Look at /tmp/test.png"},
		{Role: "assistant", Text: "I see /tmp/other.png"},
		{Role: "user", Text: "Also /var/data/photo.jpg"},
	}
	refs := DetectImagesFromHistory(messages)

	// Should only detect from user messages, not assistant
	for _, ref := range refs {
		if ref.Resolved == "/tmp/other.png" {
			t.Error("should not detect from assistant messages")
		}
	}

	// Should have refs from user messages
	foundTest := false
	foundPhoto := false
	for _, ref := range refs {
		if ref.Resolved == "/tmp/test.png" {
			foundTest = true
			if ref.MessageIndex != 0 {
				t.Errorf("expected messageIndex=0, got %d", ref.MessageIndex)
			}
		}
		if ref.Resolved == "/var/data/photo.jpg" {
			foundPhoto = true
			if ref.MessageIndex != 2 {
				t.Errorf("expected messageIndex=2, got %d", ref.MessageIndex)
			}
		}
	}
	if !foundTest || !foundPhoto {
		t.Errorf("missing expected refs: test=%v photo=%v refs=%+v", foundTest, foundPhoto, refs)
	}
}
