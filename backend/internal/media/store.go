package media

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TS 对照: media/store.ts (243L)

const (
	// MediaMaxBytes 默认媒体最大字节数 (5MB)。
	// TS 对照: store.ts L13
	MediaMaxBytes = 5 * 1024 * 1024

	// defaultStoreTTL 存储默认 TTL (2 分钟)。
	defaultStoreTTL = 2 * time.Minute
)

// sanitizeRe 文件名净化正则。
var sanitizeRe = regexp.MustCompile(`[^\p{L}\p{N}._-]+`)

// underscoreCollapseRe 合并连续下划线。
var underscoreCollapseRe = regexp.MustCompile(`_+`)

// uuidInNameRe 检测嵌入的 UUID 模式。
// Pattern: {original}---{uuid}.{ext}
var uuidInNameRe = regexp.MustCompile(
	`^(.+)---[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`,
)

// ---------- 辅助函数 ----------

// sanitizeFilename 净化文件名，仅保留安全字符。
// TS 对照: store.ts L22-30
func sanitizeFilename(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	sanitized := sanitizeRe.ReplaceAllString(trimmed, "_")
	sanitized = underscoreCollapseRe.ReplaceAllString(sanitized, "_")
	sanitized = strings.Trim(sanitized, "_")
	if len(sanitized) > 60 {
		sanitized = sanitized[:60]
	}
	return sanitized
}

// randomUUID 生成 UUID v4。
func randomUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// looksLikeURL 判断字符串是否为 HTTP(S) URL。
func looksLikeURL(src string) bool {
	return strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
}

// ---------- 公开类型和函数 ----------

// SavedMedia 已保存的媒体信息。
// TS 对照: store.ts L163-168
type SavedMedia struct {
	ID          string
	Path        string
	Size        int64
	ContentType string
}

// resolveMediaDir 解析媒体存储目录。
// TS 对照: store.ts L12
func resolveMediaDir() string {
	configDir := os.Getenv("OPENACOSMI_CONFIG_DIR")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config", "openacosmi")
	}
	return filepath.Join(configDir, "media")
}

// GetMediaDir 返回媒体存储目录路径。
// TS 对照: store.ts L57-59
func GetMediaDir() string {
	return resolveMediaDir()
}

// EnsureMediaDir 确保媒体目录存在并返回路径。
// TS 对照: store.ts L61-65
func EnsureMediaDir() (string, error) {
	dir := resolveMediaDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("创建媒体目录失败: %w", err)
	}
	return dir, nil
}

// CleanOldMedia 清理过期的媒体文件。
// TS 对照: store.ts L67-83
func CleanOldMedia(ttl time.Duration) {
	if ttl == 0 {
		ttl = defaultStoreTTL
	}
	dir, err := EnsureMediaDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > ttl {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}

// ExtractOriginalFilename 从嵌入 UUID 的文件名中提取原始文件名。
// Pattern: {original}---{uuid}.{ext} → "{original}.{ext}"
// TS 对照: store.ts L37-55
func ExtractOriginalFilename(filePath string) string {
	basename := filepath.Base(filePath)
	if basename == "" || basename == "." {
		return "file.bin"
	}
	ext := filepath.Ext(basename)
	nameWithoutExt := strings.TrimSuffix(basename, ext)

	matches := uuidInNameRe.FindStringSubmatch(strings.ToLower(nameWithoutExt))
	if len(matches) >= 2 {
		// 使用原始大小写
		origMatches := uuidInNameRe.FindStringSubmatch(nameWithoutExt)
		if len(origMatches) >= 2 {
			return origMatches[1] + ext
		}
	}
	return basename
}

// SaveMediaSource 从本地路径或远程 URL 保存媒体。
// TS 对照: store.ts L170-209
func SaveMediaSource(source string) (*SavedMedia, error) {
	return SaveMediaSourceWithHeaders(source, nil)
}

// SaveMediaSourceWithHeaders 从本地路径或远程 URL 保存媒体，支持自定义 Headers。
// TS 对照: store.ts downloadToFile 支持 headers?: Record<string, string> 透传。
func SaveMediaSourceWithHeaders(source string, headers map[string]string) (*SavedMedia, error) {
	dir, err := EnsureMediaDir()
	if err != nil {
		return nil, err
	}
	CleanOldMedia(0)
	baseID := randomUUID()

	if looksLikeURL(source) {
		return downloadAndSave(source, dir, baseID, headers)
	}
	return copyLocalFile(source, dir, baseID)
}

// downloadAndSave 下载远程媒体并保存。
// headers 可为 nil，用于支持鉴权资源拉取。
// TS 对照: store.ts downloadToFile 支持 headers 透传。
func downloadAndSave(rawURL, dir, baseID string, headers map[string]string) (*SavedMedia, error) {
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
