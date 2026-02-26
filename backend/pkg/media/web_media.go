// web_media.go — 共享 Web 媒体加载工具。
//
// 从 WhatsApp media.go 提取的通用媒体加载逻辑，
// 支持本地文件和 HTTP URL，供各频道包共享使用。
//
// TS 对照: src/web/media.ts loadWebMedia()
package media

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// DefaultMaxMediaBytes 默认最大媒体尺寸（64MB）
const DefaultMaxMediaBytes = 64 * 1024 * 1024

// WebMedia 加载后的媒体数据
type WebMedia struct {
	Buffer      []byte
	ContentType string
	Kind        string // "image"|"video"|"audio"|"document"|"sticker"
	Filename    string
}

// LoadWebMedia 加载媒体（支持本地文件和 HTTP URL）。
// maxBytes <= 0 使用 DefaultMaxMediaBytes。
func LoadWebMedia(source string, maxBytes int64) (*WebMedia, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil, fmt.Errorf("empty media source")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxMediaBytes
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return loadWebMediaRemote(trimmed, maxBytes)
	}
	return loadWebMediaLocal(trimmed, maxBytes)
}

// loadWebMediaLocal 从本地文件加载媒体
func loadWebMediaLocal(filePath string, maxBytes int64) (*WebMedia, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read local media: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("media file too large (%d bytes, max %d)", len(data), maxBytes)
	}
	contentType := DetectContentType(filePath, data)
	kind := ResolveMediaKind(contentType)
	return &WebMedia{
		Buffer:      data,
		ContentType: contentType,
		Kind:        kind,
		Filename:    filepath.Base(filePath),
	}, nil
}

// loadWebMediaRemote 从 HTTP URL 加载媒体
func loadWebMediaRemote(url string, maxBytes int64) (*WebMedia, error) {
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote media returned HTTP %d from %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read remote media body: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("remote media too large (%d bytes, max %d)", len(data), maxBytes)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	// 去掉参数部分（如 "image/jpeg; charset=utf-8"）
	if idx := strings.Index(contentType, ";"); idx > 0 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	// 推断文件名
	fileName := "upload"
	if urlPath := extractURLPath(url); urlPath != "" {
		base := path.Base(urlPath)
		if base != "" && base != "." && base != "/" {
			fileName = base
		}
	}

	kind := ResolveMediaKind(contentType)
	return &WebMedia{
		Buffer:      data,
		ContentType: contentType,
		Kind:        kind,
		Filename:    fileName,
	}, nil
}

// DetectContentType 检测文件内容类型。
// 优先使用扩展名，后备使用 HTTP 内容检测。
func DetectContentType(filePath string, data []byte) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" {
		mimeType := mime.TypeByExtension(ext)
		if mimeType != "" {
			return mimeType
		}
	}
	return http.DetectContentType(data)
}

// ResolveMediaKind 根据 MIME 类型判断媒体种类。
func ResolveMediaKind(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.HasPrefix(ct, "image/webp"):
		return "sticker"
	case strings.HasPrefix(ct, "image/"):
		return "image"
	case strings.HasPrefix(ct, "video/"):
		return "video"
	case strings.HasPrefix(ct, "audio/"):
		return "audio"
	default:
		return "document"
	}
}

// extractURLPath 从 URL 中提取路径部分。
func extractURLPath(rawURL string) string {
	idx := strings.Index(rawURL, "://")
	if idx < 0 {
		return ""
	}
	rest := rawURL[idx+3:]
	slashIdx := strings.Index(rest, "/")
	if slashIdx < 0 {
		return ""
	}
	pathPart := rest[slashIdx:]
	if qIdx := strings.Index(pathPart, "?"); qIdx > 0 {
		pathPart = pathPart[:qIdx]
	}
	return pathPart
}
