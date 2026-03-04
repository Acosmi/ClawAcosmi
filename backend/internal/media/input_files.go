package media

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/security"
	pdfcpuApi "github.com/pdfcpu/pdfcpu/pkg/api"
	pdfcpuModel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// TS 对照: media/input-files.ts (357L)
// 处理输入文件（图像、文本、PDF）的内容提取。
//
// 拆分说明：
//   - PDF 页面渲染（pdftocairo/qlmanage/pdfcpu） → input_files_pdf.go

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
