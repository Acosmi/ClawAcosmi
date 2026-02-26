package runner

// ============================================================================
// 图片优化 — JPEG 压缩 + 多分辨率网格
// TS 对照: web/media.ts → optimizeImageToJpeg()
//
// 使用标准库 image/jpeg + image/png 解码，
// golang.org/x/image/draw 做高质量缩放。
// HEIC 支持标记为 RUST_CANDIDATE: P2。
// ============================================================================

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"log/slog"

	// 注册更多图片格式解码器
	_ "image/gif"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"golang.org/x/image/draw"
)

// optimizeSizes 缩放目标分辨率列表（长边像素）。
// TS 对照: web/media.ts → SIZES
var optimizeSizes = []int{2048, 1536, 1280, 1024, 800}

// optimizeQualities JPEG 压缩质量列表。
// TS 对照: web/media.ts → QUALITIES
var optimizeQualities = []int{80, 70, 60, 50, 40}

// OptimizeImageToJPEG 将图片数据优化为 JPEG 格式。
// 尝试 sizes × qualities 网格，找到第一个不超过 maxBytes 的组合。
// 如果所有组合都超出限制，返回最小的成功压缩结果。
// maxBytes <= 0 时直接返回原始数据。
// TS 对照: web/media.ts → optimizeImageToJpeg()
func OptimizeImageToJPEG(data []byte, maxBytes int) ([]byte, error) {
	if maxBytes <= 0 || len(data) <= maxBytes {
		return data, nil
	}

	// 解码图片
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		slog.Debug("image optimize: 解码失败，返回原始数据", "error", err)
		return data, nil // 无法解码则返回原始数据
	}

	slog.Debug("image optimize: 开始优化",
		"format", format,
		"originalSize", len(data),
		"maxBytes", maxBytes,
		"bounds", img.Bounds().Max)

	bounds := img.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	// 确定长边
	longSide := origW
	if origH > longSide {
		longSide = origH
	}

	var bestResult []byte

	for _, size := range optimizeSizes {
		// 跳过不需要缩放的尺寸
		if size >= longSide {
			// 不缩放，仅压缩
			for _, quality := range optimizeQualities {
				result, err := compressJPEG(img, quality)
				if err != nil {
					continue
				}
				if bestResult == nil || len(result) < len(bestResult) {
					bestResult = result
				}
				if len(result) <= maxBytes {
					slog.Debug("image optimize: 成功",
						"size", "original",
						"quality", quality,
						"resultSize", len(result))
					return result, nil
				}
			}
			continue
		}

		// 计算缩放比例
		var newW, newH int
		if origW >= origH {
			newW = size
			newH = int(float64(origH) * float64(size) / float64(origW))
		} else {
			newH = size
			newW = int(float64(origW) * float64(size) / float64(origH))
		}
		if newW < 1 {
			newW = 1
		}
		if newH < 1 {
			newH = 1
		}

		// 缩放图片
		resized := resizeImage(img, newW, newH)

		for _, quality := range optimizeQualities {
			result, err := compressJPEG(resized, quality)
			if err != nil {
				continue
			}
			if bestResult == nil || len(result) < len(bestResult) {
				bestResult = result
			}
			if len(result) <= maxBytes {
				slog.Debug("image optimize: 成功",
					"size", size,
					"quality", quality,
					"resultSize", len(result))
				return result, nil
			}
		}
	}

	// 所有组合都超出限制，返回最小结果
	if bestResult != nil {
		slog.Debug("image optimize: 全部超限，使用最小结果",
			"resultSize", len(bestResult),
			"maxBytes", maxBytes)
		return bestResult, nil
	}

	return data, nil
}

// resizeImage 使用 CatmullRom 插值缩放图片。
func resizeImage(src image.Image, newW, newH int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}

// compressJPEG 将图片编码为指定质量的 JPEG。
func compressJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodePNG 解码 PNG 图片。（导出函数供测试使用）
func DecodePNG(data []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(data))
}
