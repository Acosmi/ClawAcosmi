// Package media 提供媒体文件处理工具。
//
// TS 对照: media/constants.ts (44L) + media/ 目录
//
// 主要功能:
//   - MIME 类型检测与扩展名映射
//   - 媒体大小限制常量
//   - 媒体类型分类 (image/audio/video/document)
//   - 媒体获取、存储、解析
package media

import "strings"

// ---------- 大小限制常量 ----------

const (
	// MaxImageBytes 图像最大字节数 (6MB)。
	// TS 对照: constants.ts L1
	MaxImageBytes = 6 * 1024 * 1024

	// MaxAudioBytes 音频最大字节数 (16MB)。
	// TS 对照: constants.ts L2
	MaxAudioBytes = 16 * 1024 * 1024

	// MaxVideoBytes 视频最大字节数 (16MB)。
	// TS 对照: constants.ts L3
	MaxVideoBytes = 16 * 1024 * 1024

	// MaxDocumentBytes 文档最大字节数 (100MB)。
	// TS 对照: constants.ts L4
	MaxDocumentBytes = 100 * 1024 * 1024
)

// ---------- 媒体类型 ----------

// MediaKind 媒体种类。
// TS 对照: constants.ts L6
type MediaKind string

const (
	KindImage    MediaKind = "image"
	KindAudio    MediaKind = "audio"
	KindVideo    MediaKind = "video"
	KindDocument MediaKind = "document"
	KindUnknown  MediaKind = "unknown"
)

// MediaKindFromMime 根据 MIME 类型判断媒体种类。
// TS 对照: constants.ts L8-28
func MediaKindFromMime(mime string) MediaKind {
	if mime == "" {
		return KindUnknown
	}
	m := strings.ToLower(mime)
	if strings.HasPrefix(m, "image/") {
		return KindImage
	}
	if strings.HasPrefix(m, "audio/") {
		return KindAudio
	}
	if strings.HasPrefix(m, "video/") {
		return KindVideo
	}
	if m == "application/pdf" {
		return KindDocument
	}
	if strings.HasPrefix(m, "application/") {
		return KindDocument
	}
	return KindUnknown
}

// MaxBytesForKind 返回指定媒体种类的最大字节数。
// TS 对照: constants.ts L30-43
func MaxBytesForKind(kind MediaKind) int64 {
	switch kind {
	case KindImage:
		return MaxImageBytes
	case KindAudio:
		return MaxAudioBytes
	case KindVideo:
		return MaxVideoBytes
	case KindDocument:
		return MaxDocumentBytes
	default:
		return MaxDocumentBytes
	}
}
