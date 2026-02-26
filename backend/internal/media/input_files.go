package media

import (
	"bytes"
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

	"github.com/anthropic/open-acosmi/internal/security"
	pdfcpuApi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpuModel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	// 注册 JPEG/PNG/GIF 解码器（用于 PDF 嵌入图转换）
	_ "image/gif"
	_ "image/jpeg"
)

// TS 对照: media/input-files.ts (357L)
// 处理输入文件（图像、文本、PDF）的内容提取。

// ---------- PDF 限制常量 ----------
// TS 对照: input-files.ts L106-108

const (
	// DefaultPDFMaxPages PDF 最大渲染页数。
	DefaultPDFMaxPages = 4
	// DefaultPDFMaxPixels 每页最大像素数（4M 像素）。
	DefaultPDFMaxPixels = 4_000_000
	// DefaultPDFMinTextChars 文本量低于此阈值时才尝试渲染图片。
	DefaultPDFMinTextChars = 200
)

// PDFLimits PDF 处理限制。
// TS 对照: input-files.ts L47-51 InputPdfLimits
type PDFLimits struct {
	MaxPages     int
	MaxPixels    int
	MinTextChars int
}

// DefaultPDFLimits 返回默认 PDF 限制。
func DefaultPDFLimits() PDFLimits {
	return PDFLimits{
		MaxPages:     DefaultPDFMaxPages,
		MaxPixels:    DefaultPDFMaxPixels,
		MinTextChars: DefaultPDFMinTextChars,
	}
}

// ---------- 类型 ----------

// InputFileLimits 输入文件内容提取限制。
type InputFileLimits struct {
	MaxImageBytes int64
	MaxFileBytes  int64
	MaxTextChars  int64
	PDF           PDFLimits
}

// DefaultInputFileLimits 默认限制。
func DefaultInputFileLimits() InputFileLimits {
	return InputFileLimits{
		MaxImageBytes: MaxImageBytes,
		MaxFileBytes:  MaxDocumentBytes,
		MaxTextChars:  100_000,
		PDF:           DefaultPDFLimits(),
	}
}

// ExtractedContent 提取的文件内容。
type ExtractedContent struct {
	TextContent  string
	ImageBuffers [][]byte
	ContentType  string
	FileName     string
}

// ---------- 公开函数 ----------

// ExtractImageContentFromSource 从 URL 或本地路径提取图像内容。
// TS 对照: input-files.ts L45-90
func ExtractImageContentFromSource(source string, limits InputFileLimits) (*ExtractedContent, error) {
	if limits.MaxImageBytes <= 0 {
		limits.MaxImageBytes = MaxImageBytes
	}

	if looksLikeURL(source) {
		result, err := FetchRemoteMedia(FetchMediaOptions{
			URL:      source,
			MaxBytes: limits.MaxImageBytes,
		})
		if err != nil {
			return nil, fmt.Errorf("获取远程图像失败: %w", err)
		}
		return &ExtractedContent{
			ImageBuffers: [][]byte{result.Buffer},
			ContentType:  result.ContentType,
			FileName:     result.FileName,
		}, nil
	}

	// 本地文件
	info, err := os.Stat(source)
	if err != nil {
		return nil, fmt.Errorf("访问图像文件失败: %w", err)
	}
	if info.Size() > limits.MaxImageBytes {
		return nil, fmt.Errorf("图像文件 %d 字节超过 %d 限制", info.Size(), limits.MaxImageBytes)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return nil, fmt.Errorf("读取图像文件失败: %w", err)
	}
	mime := DetectMime(DetectMimeOpts{Buffer: data, FilePath: source})
	return &ExtractedContent{
		ImageBuffers: [][]byte{data},
		ContentType:  mime,
	}, nil
}

// ExtractFileContentFromSource 从 URL 或本地路径提取文本文件内容。
// TS 对照: input-files.ts L92-150
func ExtractFileContentFromSource(source string, limits InputFileLimits) (*ExtractedContent, error) {
	if limits.MaxFileBytes <= 0 {
		limits.MaxFileBytes = MaxDocumentBytes
	}
	if limits.MaxTextChars <= 0 {
		limits.MaxTextChars = 100_000
	}
	if limits.PDF.MaxPages <= 0 {
		limits.PDF = DefaultPDFLimits()
	}

	var data []byte
	var contentType string

	if looksLikeURL(source) {
		result, err := FetchRemoteMedia(FetchMediaOptions{
			URL:      source,
			MaxBytes: limits.MaxFileBytes,
		})
		if err != nil {
			return nil, fmt.Errorf("获取远程文件失败: %w", err)
		}
		data = result.Buffer
		contentType = result.ContentType
	} else {
		info, err := os.Stat(source)
		if err != nil {
			return nil, fmt.Errorf("访问文件失败: %w", err)
		}
		if info.Size() > limits.MaxFileBytes {
			return nil, fmt.Errorf("文件 %d 字节超过 %d 限制", info.Size(), limits.MaxFileBytes)
		}
		data, err = os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: %w", err)
		}
		contentType = DetectMime(DetectMimeOpts{Buffer: data, FilePath: source})
	}

	// PDF 处理
	if strings.HasPrefix(contentType, "application/pdf") {
		return extractPDFContent(data, limits)
	}

	// 文本内容
	text := string(data)
	if int64(len([]rune(text))) > limits.MaxTextChars {
		runes := []rune(text)
		text = string(runes[:limits.MaxTextChars])
	}

	return &ExtractedContent{
		TextContent: text,
		ContentType: contentType,
	}, nil
}

// extractPDFContent 从 PDF 提取文本内容，文本不足时尝试渲染页面为图片。
// TS 对照: input-files.ts L197-254
//
// 策略与 TS 端一致：
//  1. 先提取文本
//  2. 若文本 >= minTextChars → 直接返回文本（无图片）
//  3. 若文本不足 → 尝试渲染页面为 PNG 图片
//  4. 渲染失败 → 仅返回文本（优雅降级）
func extractPDFContent(data []byte, limits InputFileLimits) (*ExtractedContent, error) {
	reader := bytes.NewReader(data)
	pdfLimits := limits.PDF
	if pdfLimits.MaxPages <= 0 {
		pdfLimits = DefaultPDFLimits()
	}

	// 使用 pdfcpu 读取 PDF 上下文获取页数
	pdfCtx, err := pdfcpuApi.ReadContext(reader, pdfcpuModel.NewDefaultConfiguration())
	if err != nil {
		return &ExtractedContent{
			TextContent: fmt.Sprintf("[PDF 解析失败: %v, %d 字节]", err, len(data)),
			ContentType: "application/pdf",
		}, nil
	}
	pageCount := pdfCtx.PageCount
	maxPages := pdfLimits.MaxPages
	if pageCount < maxPages {
		maxPages = pageCount
	}

	// 使用 ExtractContent 提取到临时目录
	tmpDir, err := os.MkdirTemp("", "pdf-extract-*")
	if err != nil {
		return &ExtractedContent{
			TextContent: fmt.Sprintf("[PDF 文件, %d 页, 临时目录创建失败]", pageCount),
			ContentType: "application/pdf",
		}, nil
	}
	defer os.RemoveAll(tmpDir)

	// 重置 reader
	reader.Seek(0, io.SeekStart)

	// 提取所有页面内容到临时目录
	conf := pdfcpuModel.NewDefaultConfiguration()
	_ = pdfcpuApi.ExtractContent(reader, tmpDir, "content", nil, conf)

	// 读取提取的文件内容
	var textBuilder strings.Builder
	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(tmpDir, entry.Name()))
		if err != nil {
			continue
		}
		text := strings.TrimSpace(string(content))
		if text != "" {
			if textBuilder.Len() > 0 {
				textBuilder.WriteString("\n\n")
			}
			textBuilder.WriteString(text)
		}
		// 提前截断检查
		if limits.MaxTextChars > 0 && textBuilder.Len() > int(limits.MaxTextChars) {
			break
		}
	}

	text := strings.TrimSpace(textBuilder.String())
	if text == "" {
		text = fmt.Sprintf("[PDF 文件, %d 页, 无可提取文本]", pageCount)
	}

	// 截断
	if limits.MaxTextChars > 0 && len([]rune(text)) > int(limits.MaxTextChars) {
		runes := []rune(text)
		text = string(runes[:limits.MaxTextChars])
	}

	// TS 对照: input-files.ts L223-225
	// 如果文本量充足 → 直接返回文本（与 TS 行为一致）
	if len(strings.TrimSpace(text)) >= pdfLimits.MinTextChars {
		return &ExtractedContent{
			TextContent: text,
			ContentType: "application/pdf",
		}, nil
	}

	// 文本量不足 → 尝试渲染 PDF 页面为 PNG 图片
	// TS 对照: input-files.ts L227-253 (canvas rendering fallback)
	images := renderPDFPages(data, maxPages, pdfLimits.MaxPixels)

	return &ExtractedContent{
		TextContent:  text,
		ImageBuffers: images,
		ContentType:  "application/pdf",
	}, nil
}

// ---------- PDF 页面渲染 ----------

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

	if err := exec.Command("pdftocairo", args...).Run(); err != nil {
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
	if err := exec.Command("qlmanage", args...).Run(); err != nil {
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

// ---------- SSRF 安全获取 ----------

// FetchRemoteFile 从远程 URL 获取文件内容（通用版）。
// 集成 SSRF 防护（P7B-3）。
// TS 对照: input-files.ts 内部引用 fetchWithSsrFGuard
func FetchRemoteFile(rawURL string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = MaxDocumentBytes
	}
	resp, err := security.SafeFetchURL(rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("获取远程文件失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("远程文件 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("读取远程文件失败: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("远程文件超过 %d 字节限制", maxBytes)
	}
	return data, nil
}
