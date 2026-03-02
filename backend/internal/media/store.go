package media

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TS 对照: media/store.ts (243L)
//
// 拆分说明：
//   - I/O 操作（下载/复制/Buffer写入） → store_io.go

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
