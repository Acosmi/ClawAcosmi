package media

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

// TS 对照: media/mime.ts (191L)

// ---------- MIME ↔ 扩展名映射 ----------

// extByMIME 常见 MIME 到首选文件扩展名。
// TS 对照: mime.ts L6-35 EXT_BY_MIME
var extByMIME = map[string]string{
	"image/heic":                    ".heic",
	"image/heif":                    ".heif",
	"image/jpeg":                    ".jpg",
	"image/png":                     ".png",
	"image/webp":                    ".webp",
	"image/gif":                     ".gif",
	"audio/ogg":                     ".ogg",
	"audio/mpeg":                    ".mp3",
	"audio/x-m4a":                   ".m4a",
	"audio/mp4":                     ".m4a",
	"video/mp4":                     ".mp4",
	"video/quicktime":               ".mov",
	"application/pdf":               ".pdf",
	"application/json":              ".json",
	"application/zip":               ".zip",
	"application/gzip":              ".gz",
	"application/x-tar":             ".tar",
	"application/x-7z-compressed":   ".7z",
	"application/vnd.rar":           ".rar",
	"application/msword":            ".doc",
	"application/vnd.ms-excel":      ".xls",
	"application/vnd.ms-powerpoint": ".ppt",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   ".docx",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         ".xlsx",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
	"text/csv":      ".csv",
	"text/plain":    ".txt",
	"text/markdown": ".md",
}

// mimeByExt 扩展名到 MIME 反向映射（含别名）。
// TS 对照: mime.ts L37-41 MIME_BY_EXT
var mimeByExt map[string]string

func init() {
	mimeByExt = make(map[string]string, len(extByMIME)+1)
	for mime, ext := range extByMIME {
		mimeByExt[ext] = mime
	}
	// 额外别名
	mimeByExt[".jpeg"] = "image/jpeg"
}

// audioFileExtensions 音频文件扩展名集合。
// TS 对照: mime.ts L43-53
var audioFileExtensions = map[string]bool{
	".aac":  true,
	".caf":  true,
	".flac": true,
	".m4a":  true,
	".mp3":  true,
	".oga":  true,
	".ogg":  true,
	".opus": true,
	".wav":  true,
}

// ---------- 公开函数 ----------

// GetFileExtension 获取文件路径的扩展名（小写），支持 URL。
// TS 对照: mime.ts L75-89
func GetFileExtension(filePath string) string {
	if filePath == "" {
		return ""
	}
	// 尝试解析为 URL
	if strings.HasPrefix(filePath, "http://") || strings.HasPrefix(filePath, "https://") {
		if u, err := url.Parse(filePath); err == nil {
			ext := strings.ToLower(filepath.Ext(u.Path))
			if ext != "" {
				return ext
			}
		}
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	return ext
}

// IsAudioFileName 判断文件名是否为音频文件。
// TS 对照: mime.ts L91-97
func IsAudioFileName(fileName string) bool {
	ext := GetFileExtension(fileName)
	if ext == "" {
		return false
	}
	return audioFileExtensions[ext]
}

// normalizeHeaderMime 规范化 HTTP Content-Type 头中的 MIME 类型。
// TS 对照: mime.ts L55-61
func normalizeHeaderMime(mime string) string {
	if mime == "" {
		return ""
	}
	parts := strings.SplitN(mime, ";", 2)
	cleaned := strings.TrimSpace(strings.ToLower(parts[0]))
	return cleaned
}

// isGenericMime 判断是否为通用容器 MIME。
// TS 对照: mime.ts L107-113
func isGenericMime(mime string) bool {
	if mime == "" {
		return true
	}
	m := strings.ToLower(mime)
	return m == "application/octet-stream" || m == "application/zip"
}

// SniffMime 从 buffer 前 512 字节嗅探 MIME 类型。
// Go 使用 net/http.DetectContentType 替代 npm file-type。
// TS 对照: mime.ts L63-73 sniffMime
func SniffMime(buffer []byte) string {
	if len(buffer) == 0 {
		return ""
	}
	ct := http.DetectContentType(buffer)
	// DetectContentType 可能返回带参的类型如 "text/html; charset=utf-8"
	return normalizeHeaderMime(ct)
}

// DetectMimeOpts MIME 检测参数。
type DetectMimeOpts struct {
	Buffer     []byte
	HeaderMime string
	FilePath   string
}

// DetectMime 综合检测 MIME 类型（优先级：sniff > 扩展名 > header）。
// 不让通用容器类型覆盖更精确的扩展名映射（如 XLSX vs ZIP）。
// TS 对照: mime.ts L99-145 detectMime
func DetectMime(opts DetectMimeOpts) string {
	ext := GetFileExtension(opts.FilePath)
	extMime := ""
	if ext != "" {
		extMime = mimeByExt[ext]
	}

	headerMime := normalizeHeaderMime(opts.HeaderMime)
	sniffed := SniffMime(opts.Buffer)

	// 优先 sniffed，但不让通用容器类型覆盖更精确的扩展名映射
	if sniffed != "" && (!isGenericMime(sniffed) || extMime == "") {
		return sniffed
	}
	if extMime != "" {
		return extMime
	}
	if headerMime != "" && !isGenericMime(headerMime) {
		return headerMime
	}
	if sniffed != "" {
		return sniffed
	}
	if headerMime != "" {
		return headerMime
	}
	return ""
}

// ExtensionForMime 获取 MIME 类型对应的首选扩展名。
// TS 对照: mime.ts L147-152
func ExtensionForMime(mime string) string {
	if mime == "" {
		return ""
	}
	return extByMIME[strings.ToLower(mime)]
}

// IsGifMedia 判断媒体是否为 GIF。
// TS 对照: mime.ts L154-163
func IsGifMedia(contentType, fileName string) bool {
	if strings.ToLower(contentType) == "image/gif" {
		return true
	}
	return GetFileExtension(fileName) == ".gif"
}

// ImageMimeFromFormat 从图像格式名获取 MIME 类型。
// TS 对照: mime.ts L165-186
func ImageMimeFromFormat(format string) string {
	if format == "" {
		return ""
	}
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "heic":
		return "image/heic"
	case "heif":
		return "image/heif"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	default:
		return ""
	}
}

// KindFromMime 从 MIME 类型获取媒体种类（MediaKindFromMime 的别名）。
// TS 对照: mime.ts L188-190
func KindFromMime(mime string) MediaKind {
	return MediaKindFromMime(mime)
}
