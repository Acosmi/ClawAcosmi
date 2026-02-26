package media

// docconv.go — 文档转换（Document Conversion）Provider 接口（Phase D 新增）
// 支持 MCP 工具协议和内置方式，可独立配置和切换
// 所有文件处理通过沙箱（用户要求），不支持电子书格式

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// DocConverter 文档转换 Provider 接口
type DocConverter interface {
	// Convert 将文档转换为 Markdown
	// data: 文档二进制数据
	// mimeType: 文档 MIME 类型
	// fileName: 原始文件名（用于格式推断）
	Convert(ctx context.Context, data []byte, mimeType, fileName string) (string, error)

	// SupportedFormats 返回支持的文件扩展名列表
	SupportedFormats() []string

	// Name 返回 Provider 名称
	Name() string

	// TestConnection 测试连接/可用性
	TestConnection(ctx context.Context) error
}

// NewDocConverter 根据配置创建 DocConverter（工厂方法）
func NewDocConverter(cfg *types.DocConvConfig) (DocConverter, error) {
	if cfg == nil || cfg.Provider == "" {
		return nil, fmt.Errorf("docconv: provider not configured")
	}

	switch cfg.Provider {
	case "mcp":
		return NewMCPDocConverter(cfg), nil
	case "builtin":
		return NewBuiltinDocConverter(cfg), nil
	default:
		return nil, fmt.Errorf("docconv: unknown provider: %s", cfg.Provider)
	}
}

// IsSupportedFormat 判断文件扩展名是否为支持的文档格式
func IsSupportedFormat(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	_, ok := supportedFormats[ext]
	return ok
}

// FormatCategory 返回文件的格式类别
func FormatCategory(fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	if cat, ok := supportedFormats[ext]; ok {
		return cat
	}
	return ""
}

// supportedFormats 支持的文档格式 → 类别映射
// 不含电子书格式（用户要求）
var supportedFormats = map[string]string{
	// 文本（沙箱内处理）
	".txt":  "text",
	".md":   "text",
	".csv":  "text",
	".json": "text",
	".xml":  "text",
	".yaml": "text",
	".yml":  "text",
	// 代码（沙箱内处理）
	".py":    "code",
	".go":    "code",
	".rs":    "code",
	".js":    "code",
	".ts":    "code",
	".java":  "code",
	".c":     "code",
	".cpp":   "code",
	".h":     "code",
	".hpp":   "code",
	".rb":    "code",
	".php":   "code",
	".swift": "code",
	".kt":    "code",
	".sh":    "code",
	".sql":   "code",
	".css":   "code",
	".html":  "web",
	".htm":   "web",
	// Office（MCP convert_document）
	".docx": "office",
	".xlsx": "office",
	".pptx": "office",
	// PDF（MCP convert_document）
	".pdf": "pdf",
}

// AllSupportedExtensions 返回所有支持的扩展名（按类别分组）
func AllSupportedExtensions() map[string][]string {
	result := map[string][]string{}
	for ext, cat := range supportedFormats {
		result[cat] = append(result[cat], ext)
	}
	return result
}
