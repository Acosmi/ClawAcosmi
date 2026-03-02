package media

// docconv_builtin.go — 内置文档转换 fallback（Phase D 新增）
// 简单文本/代码文件直读 + pandoc CLI 调用
// 所有操作标注为需沙箱处理（实际沙箱集成在 Phase 后续）

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// BuiltinDocConverter 内置文档转换器
type BuiltinDocConverter struct {
	pandocPath string
}

// NewBuiltinDocConverter 创建内置文档转换器
func NewBuiltinDocConverter(cfg *types.DocConvConfig) *BuiltinDocConverter {
	pandocPath := cfg.PandocPath
	if pandocPath == "" {
		pandocPath = "pandoc" // 默认在 PATH 中查找
	}
	return &BuiltinDocConverter{
		pandocPath: pandocPath,
	}
}

// Name 返回 Provider 名称
func (c *BuiltinDocConverter) Name() string {
	return "builtin"
}

// SupportedFormats 返回内置支持的格式
func (c *BuiltinDocConverter) SupportedFormats() []string {
	formats := []string{".txt", ".md", ".csv", ".json", ".xml", ".yaml", ".yml"}
	// 代码文件
	formats = append(formats, ".py", ".go", ".rs", ".js", ".ts", ".java", ".c", ".cpp", ".h")
	// pandoc 支持
	if c.hasPandoc() {
		formats = append(formats, ".docx", ".html", ".htm", ".pdf")
	}
	return formats
}

// Convert 转换文档为 Markdown
func (c *BuiltinDocConverter) Convert(ctx context.Context, data []byte, mimeType, fileName string) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("docconv/builtin: empty document data")
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	category := FormatCategory(fileName)

	switch category {
	case "text":
		// 文本文件：直接作为代码块返回（标注需沙箱）
		// TODO: 实际通过沙箱读取，当前为占位
		return fmt.Sprintf("```\n%s\n```", string(data)), nil

	case "code":
		// 代码文件：语法高亮代码块（标注需沙箱）
		lang := extToLanguage(ext)
		return fmt.Sprintf("```%s\n%s\n```", lang, string(data)), nil

	case "web":
		// HTML：通过 pandoc 转换（标注需沙箱）
		if c.hasPandoc() {
			return c.pandocConvert(ctx, data, "html", fileName)
		}
		// fallback: 简单标签剥离
		return stripHTMLTags(string(data)), nil

	case "office", "pdf":
		// Office/PDF：必须通过 pandoc
		if !c.hasPandoc() {
			return "", fmt.Errorf("docconv/builtin: pandoc required for %s files", ext)
		}
		inputFormat := extToPandocFormat(ext)
		return c.pandocConvert(ctx, data, inputFormat, fileName)

	default:
		return "", fmt.Errorf("docconv/builtin: unsupported format: %s", ext)
	}
}

// TestConnection 测试 pandoc 可用性
func (c *BuiltinDocConverter) TestConnection(ctx context.Context) error {
	if !c.hasPandoc() {
		return fmt.Errorf("docconv/builtin: pandoc not found at: %s", c.pandocPath)
	}
	// 验证版本
	cmd := exec.CommandContext(ctx, c.pandocPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("docconv/builtin: pandoc not working: %w", err)
	}
	slog.Info("docconv/builtin: pandoc available", "version", strings.SplitN(string(output), "\n", 2)[0])
	return nil
}

// hasPandoc 检查 pandoc 是否可用
func (c *BuiltinDocConverter) hasPandoc() bool {
	_, err := exec.LookPath(c.pandocPath)
	return err == nil
}

// pandocConvert 通过 pandoc CLI 转换
func (c *BuiltinDocConverter) pandocConvert(ctx context.Context, data []byte, inputFormat, fileName string) (string, error) {
	tmpDir := os.TempDir()
	ext := filepath.Ext(fileName)
	if ext == "" {
		ext = ".bin"
	}
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("pandoc_input_%d%s", os.Getpid(), ext))
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return "", fmt.Errorf("docconv/builtin: write temp: %w", err)
	}
	defer os.Remove(tmpFile)

	args := []string{"-f", inputFormat, "-t", "markdown", tmpFile}
	cmd := exec.CommandContext(ctx, c.pandocPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docconv/builtin: pandoc failed: %w, output: %s",
			err, truncateString(string(output), 500))
	}

	slog.Info("docconv/builtin: pandoc conversion complete",
		"input", fileName,
		"format", inputFormat,
		"output_len", len(output),
	)
	return string(output), nil
}

// extToLanguage 文件扩展名 → 语法高亮语言标识
func extToLanguage(ext string) string {
	m := map[string]string{
		".py": "python", ".go": "go", ".rs": "rust",
		".js": "javascript", ".ts": "typescript", ".java": "java",
		".c": "c", ".cpp": "cpp", ".h": "c", ".hpp": "cpp",
		".rb": "ruby", ".php": "php", ".swift": "swift",
		".kt": "kotlin", ".sh": "bash", ".sql": "sql", ".css": "css",
	}
	if lang, ok := m[ext]; ok {
		return lang
	}
	return ""
}

// extToPandocFormat 文件扩展名 → pandoc 输入格式
func extToPandocFormat(ext string) string {
	m := map[string]string{
		".docx": "docx", ".html": "html", ".htm": "html",
		".md": "markdown", ".txt": "plain", ".pdf": "pdf",
	}
	if fmt, ok := m[ext]; ok {
		return fmt
	}
	return "plain"
}

// stripHTMLTags 简单 HTML 标签剥离（fallback）
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}
