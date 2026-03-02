package media

// ============================================================================
// media/input_files_pdf.go — PDF 页面渲染逻辑
//
// 从 input_files.go 拆分：PDF 外部工具渲染 + 嵌入图提取。
// TS 对照: input-files.ts L227-253 (canvas rendering fallback)
// ============================================================================

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	pdfcpuApi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpuModel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// pdfCommandTimeout 外部 PDF 渲染命令超时时间。
const pdfCommandTimeout = 30 * time.Second

// renderPDFPages 尝试将 PDF 页面渲染为 PNG 图片。
// 按优先级尝试多种外部工具：
//  1. pdftocairo（Poppler 工具链，高质量矢量渲染）
//  2. qlmanage（macOS Quick Look，系统内置）
//  3. pdfcpu 嵌入图提取（纯 Go，覆盖扫描版 PDF）
//
// TS 端使用 pdfjs-dist + @napi-rs/canvas 原生渲染。Go 端无内嵌 PDF 渲染器，
// 采用外部工具是合理的架构差异（与 image_ops.go 中 sips 调用模式一致）。
func renderPDFPages(pdfData []byte, maxPages int, maxPixels int) [][]byte {
	// 策略 1: pdftocairo
	if images := renderViaPdftocairo(pdfData, maxPages, maxPixels); len(images) > 0 {
		return images
	}

	// 策略 2: macOS qlmanage
	if runtime.GOOS == "darwin" {
		if images := renderViaQlmanage(pdfData, maxPages); len(images) > 0 {
			return images
		}
	}

	// 策略 3: pdfcpu 嵌入图提取
	if images := extractEmbeddedImages(pdfData, maxPages); len(images) > 0 {
		return images
	}

	// 全部不可用 → 优雅降级（仅文本，与 TS canvas 不可用时行为一致）
	// TS 对照: input-files.ts L231-233 catch 分支
	return nil
}

// renderViaPdftocairo 使用 pdftocairo 渲染 PDF 页面为 PNG。
func renderViaPdftocairo(pdfData []byte, maxPages int, maxPixels int) [][]byte {
	if _, err := exec.LookPath("pdftocairo"); err != nil {
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "pdf-render-cairo-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.pdf")
	if err := os.WriteFile(inputPath, pdfData, 0600); err != nil {
		return nil
	}

	// 计算缩放比例：假设 A4 页面 612x792pt（72dpi），计算 DPI 使像素不超限
	// 默认像素预算 4M → sqrt(4000000 / (612*792)) ≈ 2.87x → ~206 DPI
	dpi := 150 // 默认 DPI
	if maxPixels > 0 {
		a4Pixels := 612.0 * 792.0 // 标准 A4 at 72dpi
		scale := math.Sqrt(float64(maxPixels) / a4Pixels)
		calculatedDPI := int(72.0 * scale)
		if calculatedDPI < 72 {
			calculatedDPI = 72
		}
		if calculatedDPI > 300 {
			calculatedDPI = 300
		}
		dpi = calculatedDPI
	}

	outputPrefix := filepath.Join(tmpDir, "page")
	args := []string{
		"-png",
		"-r", fmt.Sprintf("%d", dpi),
		"-f", "1",
		"-l", fmt.Sprintf("%d", maxPages),
		inputPath,
		outputPrefix,
	}

	ctx, cancel := context.WithTimeout(context.Background(), pdfCommandTimeout)
	defer cancel()
	if err := exec.CommandContext(ctx, "pdftocairo", args...).Run(); err != nil {
		return nil
	}

	return collectPNGFiles(tmpDir, maxPages)
}

// renderViaQlmanage 使用 macOS Quick Look 渲染 PDF 缩略图。
func renderViaQlmanage(pdfData []byte, maxPages int) [][]byte {
	if _, err := exec.LookPath("qlmanage"); err != nil {
		return nil
	}

	tmpDir, err := os.MkdirTemp("", "pdf-render-ql-*")
	if err != nil {
		return nil
	}
	defer os.RemoveAll(tmpDir)

	inputPath := filepath.Join(tmpDir, "input.pdf")
	if err := os.WriteFile(inputPath, pdfData, 0600); err != nil {
		return nil
	}

	outDir := filepath.Join(tmpDir, "out")
	if err := os.MkdirAll(outDir, 0700); err != nil {
		return nil
	}

	// qlmanage -t 生成缩略图（PNG 格式），-s 设置最大尺寸
	args := []string{"-t", "-s", "2000", "-o", outDir, inputPath}
	qlCtx, qlCancel := context.WithTimeout(context.Background(), pdfCommandTimeout)
	defer qlCancel()
	if err := exec.CommandContext(qlCtx, "qlmanage", args...).Run(); err != nil {
		return nil
	}

	// qlmanage 仅生成第一页缩略图
	entries, _ := os.ReadDir(outDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".png") {
			data, err := os.ReadFile(filepath.Join(outDir, name))
			if err == nil && len(data) > 0 {
				return [][]byte{data}
			}
		}
	}
	return nil
}

// extractEmbeddedImages 使用 pdfcpu ExtractImagesRaw 提取 PDF 嵌入的图片资源。
// 适用于扫描版 PDF，其每页内容本身就是一张图片。
func extractEmbeddedImages(pdfData []byte, maxPages int) [][]byte {
	reader := bytes.NewReader(pdfData)

	// 使用 pdfcpu ExtractImagesRaw 提取图片
	conf := pdfcpuModel.NewDefaultConfiguration()
	selectedPages := make([]string, 0, maxPages)
	for i := 1; i <= maxPages; i++ {
		selectedPages = append(selectedPages, fmt.Sprintf("%d", i))
	}

	pageImages, err := pdfcpuApi.ExtractImagesRaw(reader, selectedPages, conf)
	if err != nil {
		return nil
	}

	var images [][]byte
	for _, pageMap := range pageImages {
		for _, img := range pageMap {
			// model.Image 实现了 io.Reader，直接读取图片数据
			data, err := io.ReadAll(&img)
			if err != nil || len(data) == 0 {
				continue
			}

			// 将非 PNG 图像转为 PNG 以保持接口一致
			ext := strings.ToLower(img.FileType)
			if ext == "png" {
				images = append(images, data)
			} else if ext == "jpg" || ext == "jpeg" {
				if pngData, err := convertToPNG(data); err == nil {
					images = append(images, pngData)
				} else {
					images = append(images, data) // fallback: 原始格式
				}
			} else {
				images = append(images, data)
			}

			if len(images) >= maxPages {
				break
			}
		}
		if len(images) >= maxPages {
			break
		}
	}
	return images
}

// collectPNGFiles 从目录中收集 PNG 文件（按文件名排序）。
func collectPNGFiles(dir string, maxFiles int) [][]byte {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var pngNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
			pngNames = append(pngNames, entry.Name())
		}
	}
	sort.Strings(pngNames)

	var images [][]byte
	for _, name := range pngNames {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || len(data) == 0 {
			continue
		}
		images = append(images, data)
		if len(images) >= maxFiles {
			break
		}
	}
	return images
}

// convertToPNG 将 JPEG 图像转换为 PNG 格式。
func convertToPNG(jpegData []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("PNG 编码失败: %w", err)
	}
	return buf.Bytes(), nil
}
