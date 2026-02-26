package whatsapp

import (
	"fmt"
	"path/filepath"
	"strings"

	pkgmedia "github.com/anthropic/open-acosmi/pkg/media"
)

// WhatsApp 媒体加载 — 继承自 src/web/media.ts (336L)
// 核心 LoadWebMedia 逻辑已提取到 pkg/media，
// 本文件保留 WhatsApp 特有的优化管线（HEIC/PNG）。

// WebMedia 类型别名 — 指向共享包
type WebMedia = pkgmedia.WebMedia

// MaxMediaSizeBytes 最大媒体尺寸（64MB，WhatsApp 限制）
const MaxMediaSizeBytes = 64 * 1024 * 1024

// LoadWebMedia 加载媒体（委托给共享 pkg/media）
func LoadWebMedia(source string) (*WebMedia, error) {
	return pkgmedia.LoadWebMedia(source, MaxMediaSizeBytes)
}

// ── WA-D: 媒体优化管线 ──

// MaxImageDimension WhatsApp 图片最大尺寸（像素）
const MaxImageDimension = 4096

// MediaOptimizer 媒体优化器接口
// 允许注入不同的优化实现（如 CGo HEIC 解码、外部 pngquant）
type MediaOptimizer interface {
	// OptimizeImage 优化图片
	// 返回优化后的数据和内容类型，或原始数据（如果不需要优化）
	OptimizeImage(data []byte, contentType string) ([]byte, string, error)
}

// OptimizeWebMedia WA-D: 媒体优化入口
// 根据内容类型自动选择优化策略：
// - HEIC → JPEG（需要 MediaOptimizer 注入）
// - PNG → 优化 PNG（pngquant）
// - 其他 → 原样返回
func OptimizeWebMedia(media *WebMedia, optimizer MediaOptimizer) *WebMedia {
	if media == nil {
		return nil
	}

	// 仅对图片类型优化
	if media.Kind != "image" && media.Kind != "sticker" {
		return media
	}

	ct := strings.ToLower(media.ContentType)

	// HEIC → JPEG 转换（通过 DI 优化器）
	if isHEICContentType(ct) {
		if optimizer == nil {
			// 无优化器，原样返回（HEIC 需要 CGo 支持）
			return media
		}
		optimized, newCT, err := optimizer.OptimizeImage(media.Buffer, media.ContentType)
		if err != nil {
			// 优化失败，原样返回
			return media
		}
		return &WebMedia{
			Buffer:      optimized,
			ContentType: newCT,
			Kind:        pkgmedia.ResolveMediaKind(newCT),
			Filename:    replaceExt(media.Filename, ".jpg"),
		}
	}

	// PNG 优化
	if ct == "image/png" {
		optimized, err := optimizePNG(media.Buffer)
		if err != nil || len(optimized) >= len(media.Buffer) {
			// 优化失败或没有变小，原样返回
			return media
		}
		return &WebMedia{
			Buffer:      optimized,
			ContentType: media.ContentType,
			Kind:        media.Kind,
			Filename:    media.Filename,
		}
	}

	// JPEG 尺寸检查（通过优化器）
	if (ct == "image/jpeg" || ct == "image/jpg") && optimizer != nil {
		optimized, newCT, err := optimizer.OptimizeImage(media.Buffer, media.ContentType)
		if err != nil {
			return media
		}
		return &WebMedia{
			Buffer:      optimized,
			ContentType: newCT,
			Kind:        media.Kind,
			Filename:    media.Filename,
		}
	}

	return media
}

// ClampImageDimensions 检查图片尺寸是否在 WhatsApp 限制内
// TS 对照: media.ts clampResolution()
// 返回 true 如果需要缩放
func ClampImageDimensions(width, height int) (newWidth, newHeight int, needsResize bool) {
	if width <= MaxImageDimension && height <= MaxImageDimension {
		return width, height, false
	}

	// 等比缩放
	if width >= height {
		newWidth = MaxImageDimension
		newHeight = int(float64(height) * float64(MaxImageDimension) / float64(width))
	} else {
		newHeight = MaxImageDimension
		newWidth = int(float64(width) * float64(MaxImageDimension) / float64(height))
	}

	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	return newWidth, newHeight, true
}

// isHEICContentType 判断是否为 HEIC 内容类型
func isHEICContentType(ct string) bool {
	lower := strings.ToLower(ct)
	return lower == "image/heic" || lower == "image/heif"
}

// optimizePNG PNG 压缩优化
// 当前实现为桩 — 实际优化需要 pngquant CLI 或纯 Go PNG 优化库
func optimizePNG(data []byte) ([]byte, error) {
	// Phase 6 Gateway: 集成 pngquant CLI 调用
	// exec.Command("pngquant", "--quality=65-80", "--speed=4", "-")
	return nil, fmt.Errorf("PNG optimization not yet available (requires pngquant)")
}

// replaceExt 替换文件扩展名
func replaceExt(filename, newExt string) string {
	if filename == "" {
		return ""
	}
	ext := filepath.Ext(filename)
	if ext == "" {
		return filename + newExt
	}
	return filename[:len(filename)-len(ext)] + newExt
}
