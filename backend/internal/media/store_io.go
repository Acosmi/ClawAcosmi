package media

// ============================================================================
// media/store_io.go — 媒体存储 I/O 操作
//
// 从 store.go 拆分：远程下载 + 本地复制 + Buffer 保存。
// TS 对照: store.ts downloadToFile / SaveMediaBuffer
// ============================================================================

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// validateBaseID 校验 baseID 不包含路径分隔符，防止路径穿越。
func validateBaseID(baseID string) error {
	if strings.ContainsAny(baseID, "/\\.") {
		return fmt.Errorf("invalid media ID: contains path separators")
	}
	return nil
}

// downloadAndSave 下载远程媒体并保存。
// headers 可为 nil，用于支持鉴权资源拉取。
// TS 对照: store.ts downloadToFile 支持 headers 透传。
func downloadAndSave(rawURL, dir, baseID string, headers map[string]string) (*SavedMedia, error) {
	if err := validateBaseID(baseID); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建下载请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载媒体失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("下载媒体 HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, MediaMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取媒体数据失败: %w", err)
	}
	if int64(len(data)) > MediaMaxBytes {
		return nil, fmt.Errorf("媒体超过 %d 字节限制", MediaMaxBytes)
	}

	parsedURL, _ := url.Parse(rawURL)
	mime := DetectMime(DetectMimeOpts{
		Buffer:     data,
		HeaderMime: resp.Header.Get("Content-Type"),
		FilePath:   parsedURL.Path,
	})
	ext := ExtensionForMime(mime)
	if ext == "" && parsedURL != nil {
		ext = filepath.Ext(parsedURL.Path)
	}
	id := baseID
	if ext != "" {
		id = baseID + ext
	}
	dest := filepath.Join(dir, id)
	if err := os.WriteFile(dest, data, 0600); err != nil {
		return nil, fmt.Errorf("保存媒体失败: %w", err)
	}
	return &SavedMedia{
		ID:          id,
		Path:        dest,
		Size:        int64(len(data)),
		ContentType: mime,
	}, nil
}

// copyLocalFile 从本地路径复制媒体文件。
func copyLocalFile(source, dir, baseID string) (*SavedMedia, error) {
	if err := validateBaseID(baseID); err != nil {
		return nil, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("媒体文件不存在: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("媒体路径不是普通文件")
	}
	if info.Size() > MediaMaxBytes {
		return nil, fmt.Errorf("媒体超过 %d 字节限制", MediaMaxBytes)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("读取媒体文件失败: %w", err)
	}
	mime := DetectMime(DetectMimeOpts{Buffer: data, FilePath: source})
	ext := ExtensionForMime(mime)
	if ext == "" {
		ext = filepath.Ext(source)
	}
	id := baseID
	if ext != "" {
		id = baseID + ext
	}
	dest := filepath.Join(dir, id)
	if err := os.WriteFile(dest, data, 0600); err != nil {
		return nil, fmt.Errorf("保存媒体失败: %w", err)
	}
	return &SavedMedia{
		ID:          id,
		Path:        dest,
		Size:        info.Size(),
		ContentType: mime,
	}, nil
}

// SaveMediaBuffer 从内存 buffer 保存媒体。
// TS 对照: store.ts L211-242
func SaveMediaBuffer(buffer []byte, contentType string, subdir string, maxBytes int64, originalFilename string) (*SavedMedia, error) {
	if maxBytes <= 0 {
		maxBytes = MediaMaxBytes
	}
	if subdir == "" {
		subdir = "inbound"
	}
	if int64(len(buffer)) > maxBytes {
		return nil, fmt.Errorf("媒体超过 %dMB 限制", maxBytes/(1024*1024))
	}
	baseDir := resolveMediaDir()
	dir := filepath.Join(baseDir, subdir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("创建媒体目录失败: %w", err)
	}
	uuid := randomUUID()

	// 提取 header MIME 的扩展名
	headerMime := ""
	if idx := strings.IndexByte(contentType, ';'); idx >= 0 {
		headerMime = strings.TrimSpace(contentType[:idx])
	} else {
		headerMime = strings.TrimSpace(contentType)
	}
	headerExt := ExtensionForMime(headerMime)
	mime := DetectMime(DetectMimeOpts{Buffer: buffer, HeaderMime: contentType})
	ext := headerExt
	if ext == "" {
		ext = ExtensionForMime(mime)
	}

	var id string
	if originalFilename != "" {
		base := strings.TrimSuffix(filepath.Base(originalFilename), filepath.Ext(originalFilename))
		sanitized := sanitizeFilename(base)
		if sanitized != "" {
			id = fmt.Sprintf("%s---%s%s", sanitized, uuid, ext)
		} else {
			id = uuid + ext
		}
	} else {
		if ext != "" {
			id = uuid + ext
		} else {
			id = uuid
		}
	}

	dest := filepath.Join(dir, id)
	if err := os.WriteFile(dest, buffer, 0600); err != nil {
		return nil, fmt.Errorf("保存媒体失败: %w", err)
	}
	return &SavedMedia{
		ID:          id,
		Path:        dest,
		Size:        int64(len(buffer)),
		ContentType: mime,
	}, nil
}
