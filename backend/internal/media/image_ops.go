package media

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	xdraw "golang.org/x/image/draw"

	// 注册 JPEG/PNG/GIF 解码器
	_ "image/gif"
)

// TS 对照: media/image-ops.ts (474L)
// RUST_CANDIDATE: P2 — 图像编解码密集运算

// ---------- 类型 ----------

// ImageMetadata 图像元数据。
// TS 对照: image-ops.ts L8-11
type ImageMetadata struct {
	Width  int
	Height int
}

// ---------- 平台检测 ----------

// prefersSips 判断是否优先使用 macOS sips 而非 Go 内置处理。
// TS 对照: image-ops.ts L17-22
func prefersSips() bool {
	return runtime.GOOS == "darwin"
}

// ---------- EXIF 方向读取 ----------

// ReadJpegExifOrientation 从 JPEG buffer 读取 EXIF 方向值 (1-8)。
// 返回 0 表示未找到或非 JPEG。
// TS 对照: image-ops.ts L30-125
func ReadJpegExifOrientation(buffer []byte) int {
	if len(buffer) < 12 {
		return 0
	}
	// JPEG SOI 标记
	if buffer[0] != 0xFF || buffer[1] != 0xD8 {
		return 0
	}

	pos := 2
	for pos+4 < len(buffer) {
		if buffer[pos] != 0xFF {
			break
		}
		marker := buffer[pos+1]
		// APP1 (EXIF)
		if marker == 0xE1 {
			return readExifOrientation(buffer, pos)
		}
		// 跳过其他段
		if pos+4 > len(buffer) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(buffer[pos+2 : pos+4]))
		pos += 2 + segLen
	}
	return 0
}

// readExifOrientation 从 APP1 段中解析 EXIF 方向。
func readExifOrientation(buffer []byte, app1Start int) int {
	if app1Start+4 > len(buffer) {
		return 0
	}
	segLen := int(binary.BigEndian.Uint16(buffer[app1Start+2 : app1Start+4]))
	segEnd := app1Start + 2 + segLen
	if segEnd > len(buffer) {
		segEnd = len(buffer)
	}
	exifStart := app1Start + 4
	// 检查 "Exif\0\0"
	if exifStart+6 > segEnd {
		return 0
	}
	if string(buffer[exifStart:exifStart+4]) != "Exif" {
		return 0
	}

	tiffStart := exifStart + 6
	if tiffStart+8 > segEnd {
		return 0
	}

	// 字节序
	var bigEndian bool
	if buffer[tiffStart] == 'M' && buffer[tiffStart+1] == 'M' {
		bigEndian = true
	} else if buffer[tiffStart] == 'I' && buffer[tiffStart+1] == 'I' {
		bigEndian = false
	} else {
		return 0
	}

	readU16 := func(offset int) uint16 {
		if offset+2 > segEnd {
			return 0
		}
		if bigEndian {
			return binary.BigEndian.Uint16(buffer[offset : offset+2])
		}
		return binary.LittleEndian.Uint16(buffer[offset : offset+2])
	}

	// IFD0 偏移
	ifdOffsetRaw := tiffStart + 4
	if ifdOffsetRaw+4 > segEnd {
		return 0
	}
	var ifdOffset uint32
	if bigEndian {
		ifdOffset = binary.BigEndian.Uint32(buffer[ifdOffsetRaw : ifdOffsetRaw+4])
	} else {
		ifdOffset = binary.LittleEndian.Uint32(buffer[ifdOffsetRaw : ifdOffsetRaw+4])
	}

	ifdPos := tiffStart + int(ifdOffset)
	if ifdPos+2 > segEnd {
		return 0
	}
	numEntries := int(readU16(ifdPos))
	ifdPos += 2

	for i := 0; i < numEntries; i++ {
		entryPos := ifdPos + i*12
		if entryPos+12 > segEnd {
			break
		}
		tag := readU16(entryPos)
		if tag == 0x0112 { // Orientation
			value := readU16(entryPos + 8)
			if value >= 1 && value <= 8 {
				return int(value)
			}
		}
	}
	return 0
}

// ---------- 图像元数据 ----------

// GetImageMetadata 获取图像的宽高元数据。
// TS 对照: image-ops.ts L208-228
func GetImageMetadata(buffer []byte) (*ImageMetadata, error) {
	if prefersSips() {
		meta, err := sipsMetadataFromBuffer(buffer)
		if err == nil && meta != nil {
			return meta, nil
		}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(buffer))
	if err != nil {
		return nil, fmt.Errorf("解码图像配置失败: %w", err)
	}
	return &ImageMetadata{Width: cfg.Width, Height: cfg.Height}, nil
}

// sipsMetadataFromBuffer 使用 macOS sips 获取图像元数据。
// TS 对照: image-ops.ts L136-163
func sipsMetadataFromBuffer(buffer []byte) (*ImageMetadata, error) {
	tmpFile, err := os.CreateTemp("", "sips-meta-*.jpg")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(buffer); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	out, err := exec.Command("sips", "-g", "pixelWidth", "-g", "pixelHeight", tmpFile.Name()).Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(out), "\n")
	var width, height int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pixelWidth:") {
			fmt.Sscanf(line, "pixelWidth: %d", &width)
		}
		if strings.HasPrefix(line, "pixelHeight:") {
			fmt.Sscanf(line, "pixelHeight: %d", &height)
		}
	}
	if width > 0 && height > 0 {
		return &ImageMetadata{Width: width, Height: height}, nil
	}
	return nil, fmt.Errorf("sips 未返回有效尺寸")
}

// ---------- 图像缩放 ----------

// ResizeToJpeg 缩放图像到 JPEG 格式。
// TS 对照: image-ops.ts L304-347
func ResizeToJpeg(buffer []byte, maxSide int, quality int) ([]byte, error) {
	if quality <= 0 {
		quality = 85
	}

	// macOS sips 路径
	if prefersSips() {
		result, err := sipsResizeToJpeg(buffer, maxSide, quality)
		if err == nil {
			return result, nil
		}
	}

	// Go 标准库路径
	img, _, err := image.Decode(bytes.NewReader(buffer))
	if err != nil {
		return nil, fmt.Errorf("解码图像失败: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxSide && h <= maxSide {
		// 不需要缩放，直接编码为 JPEG
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("JPEG 编码失败: %w", err)
		}
		return buf.Bytes(), nil
	}

	// 计算缩放比例
	ratio := float64(maxSide) / math.Max(float64(w), float64(h))
	newW := int(math.Round(float64(w) * ratio))
	newH := int(math.Round(float64(h) * ratio))

	resized := resizeImage(img, newW, newH)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("JPEG 编码失败: %w", err)
	}
	return buf.Bytes(), nil
}

// sipsResizeToJpeg 使用 macOS sips 缩放图像到 JPEG。
// TS 对照: image-ops.ts L165-193
func sipsResizeToJpeg(buffer []byte, maxSide, quality int) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "sips-resize-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input.jpg")
	outPath := filepath.Join(tmpDir, "output.jpg")
	if err := os.WriteFile(inPath, buffer, 0600); err != nil {
		return nil, err
	}

	args := []string{
		"-s", "format", "jpeg",
		"-s", "formatOptions", fmt.Sprintf("%d", quality),
		"--resampleHeightWidthMax", fmt.Sprintf("%d", maxSide),
		inPath,
		"--out", outPath,
	}
	if err := exec.Command("sips", args...).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(outPath)
}

// ConvertHeicToJpeg 将 HEIC 图像转换为 JPEG。
// TS 对照: image-ops.ts L349-355
func ConvertHeicToJpeg(buffer []byte) ([]byte, error) {
	if !prefersSips() {
		return nil, fmt.Errorf("HEIC 转换仅在 macOS 上通过 sips 支持")
	}
	return sipsConvertToJpeg(buffer)
}

// sipsConvertToJpeg 使用 sips 转换为 JPEG。
// TS 对照: image-ops.ts L195-206
func sipsConvertToJpeg(buffer []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "sips-convert-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	inPath := filepath.Join(tmpDir, "input")
	outPath := filepath.Join(tmpDir, "output.jpg")
	if err := os.WriteFile(inPath, buffer, 0600); err != nil {
		return nil, err
	}
	if err := exec.Command("sips", "-s", "format", "jpeg", inPath, "--out", outPath).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(outPath)
}

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

// HasAlphaChannel 检查图像是否有 alpha 通道（透明度）。
// TS 对照: image-ops.ts L357-372
func HasAlphaChannel(buffer []byte) bool {
	img, _, err := image.Decode(bytes.NewReader(buffer))
	if err != nil {
		return false
	}
	switch img.(type) {
	case *image.NRGBA, *image.NRGBA64, *image.RGBA, *image.RGBA64:
		return true
	default:
		return false
	}
}

// ResizeToPng 缩放图像到 PNG 格式，保留 alpha 通道。
// TS 对照: image-ops.ts L374-398
func ResizeToPng(buffer []byte, maxSide int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(buffer))
	if err != nil {
		return nil, fmt.Errorf("解码图像失败: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxSide && h <= maxSide {
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, fmt.Errorf("PNG 编码失败: %w", err)
		}
		return buf.Bytes(), nil
	}

	ratio := float64(maxSide) / math.Max(float64(w), float64(h))
	newW := int(math.Round(float64(w) * ratio))
	newH := int(math.Round(float64(h) * ratio))

	resized := resizeImage(img, newW, newH)
	var buf bytes.Buffer
	if err := png.Encode(&buf, resized); err != nil {
		return nil, fmt.Errorf("PNG 编码失败: %w", err)
	}
	return buf.Bytes(), nil
}

// OptimizeToPngResult PNG 优化结果。
type OptimizeToPngResult struct {
	Buffer           []byte
	OptimizedSize    int
	ResizeSide       int
	CompressionLevel int
}

// OptimizeImageToPng 尝试多种分辨率使图像 PNG 大小不超过 maxBytes。
// TS 对照: image-ops.ts L400-457
func OptimizeImageToPng(buffer []byte, maxBytes int) (*OptimizeToPngResult, error) {
	// 尝试从大到小的分辨率
	sides := []int{2048, 1536, 1024, 768, 512, 384, 256}
	for _, side := range sides {
		result, err := ResizeToPng(buffer, side)
		if err != nil {
			continue
		}
		if len(result) <= maxBytes {
			return &OptimizeToPngResult{
				Buffer:        result,
				OptimizedSize: len(result),
				ResizeSide:    side,
			}, nil
		}
	}
	return nil, fmt.Errorf("无法将图像优化到 %d 字节以内", maxBytes)
}

// ---------- 内部辅助 ----------

// resizeImage 使用双三次插值（CatmullRom）缩放图像。
// 已从 BiLinear 升级为 CatmullRom（双三次），质量显著提升。
// RUST_CANDIDATE: P2 — Rust FFI 可选优化（SIMD 加速批量处理场景）
func resizeImage(src image.Image, newW, newH int) image.Image {
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Src, nil)
	return dst
}
