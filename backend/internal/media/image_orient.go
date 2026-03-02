package media

// ============================================================================
// media/image_orient.go — EXIF 方向规范化
//
// 从 image_ops.go 拆分：方向旋转/翻转逻辑独立模块。
// TS 对照: image-ops.ts L276-302
// ============================================================================

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
)

// NormalizeExifOrientation 规范化图像的 EXIF 方向。
// 将图像像素旋转到正确方向。失败时返回原始 buffer。
// TS 对照: image-ops.ts L276-302
func NormalizeExifOrientation(buffer []byte) []byte {
	orientation := ReadJpegExifOrientation(buffer)
	if orientation <= 1 || orientation > 8 {
		return buffer
	}

	// macOS 路径：使用 sips 执行旋转
	if prefersSips() {
		result, err := sipsNormalizeOrientation(buffer)
		if err == nil {
			return result
		}
	}

	// 跨平台 fallback：使用 Go image 变换
	result, err := goNormalizeOrientation(buffer, orientation)
	if err != nil {
		return buffer // 失败时返回原始
	}
	return result
}

// sipsNormalizeOrientation 使用 macOS sips 自动修正 EXIF 方向。
// sips -r 0 会根据 EXIF 方向自动旋转像素并重置 orientation tag。
func sipsNormalizeOrientation(buffer []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "sips-orient-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.jpg")
	if err := os.WriteFile(inPath, buffer, 0600); err != nil {
		return nil, err
	}
	// sips --rotate 0 会触发 EXIF 方向自动修正
	if err := exec.Command("sips", "-r", "0", inPath).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(inPath)
}

// goNormalizeOrientation 使用 Go image 变换修正 EXIF 方向。
// EXIF orientation 值含义：
//
//	1 = 正常
//	2 = 水平翻转
//	3 = 旋转 180°
//	4 = 垂直翻转
//	5 = 转置（水平翻转 + 顺时针 270°）
//	6 = 顺时针旋转 90°
//	7 = 转位（水平翻转 + 顺时针 90°）
//	8 = 顺时针旋转 270°
func goNormalizeOrientation(buffer []byte, orientation int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(buffer))
	if err != nil {
		return nil, fmt.Errorf("解码图像失败: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	var result *image.RGBA
	switch orientation {
	case 2: // 水平翻转
		result = image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				result.Set(w-1-x, y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	case 3: // 旋转 180°
		result = image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				result.Set(w-1-x, h-1-y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	case 4: // 垂直翻转
		result = image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				result.Set(x, h-1-y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	case 5: // 转置（水平翻转 + 顺时针 270°）
		result = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				result.Set(y, x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	case 6: // 顺时针旋转 90°
		result = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				result.Set(h-1-y, x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	case 7: // 转位（水平翻转 + 顺时针 90°）
		result = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				result.Set(h-1-y, w-1-x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	case 8: // 顺时针旋转 270°
		result = image.NewRGBA(image.Rect(0, 0, h, w))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				result.Set(y, w-1-x, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
	default:
		return buffer, nil
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, result, &jpeg.Options{Quality: 95}); err != nil {
		return nil, fmt.Errorf("JPEG 编码失败: %w", err)
	}
	return buf.Bytes(), nil
}
