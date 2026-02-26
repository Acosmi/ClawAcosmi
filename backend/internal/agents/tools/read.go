// tools/read.go — 文件读写工具包装。
// TS 参考：src/agents/pi-tools.read.ts (302L)
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---------- 参数标准化 ----------

// paramAlias 参数别名映射（Claude → pi-coding-agent）。
// TS 参考: pi-tools.read.ts L14-25
var paramAliases = map[string]string{
	"file_path":   "path",
	"filePath":    "path",
	"file":        "path",
	"content":     "text",
	"fileContent": "text",
	"file_text":   "text",
}

// NormalizeToolParams 标准化工具参数名称（别名映射）。
// TS 参考: pi-tools.read.ts normalizeToolParams
func NormalizeToolParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}
	normalized := shallowCopy(params)
	for alias, canonical := range paramAliases {
		if val, ok := normalized[alias]; ok && alias != canonical {
			if _, exists := normalized[canonical]; !exists {
				normalized[canonical] = val
			}
			delete(normalized, alias)
		}
	}
	return normalized
}

// PatchToolSchemaForClaudeCompatibility 补丁 schema 以兼容 Claude 工具调用。
// Claude 有时使用 file_path 而非 path。
// TS 参考: pi-tools.read.ts patchToolSchemaForClaudeCompatibility
func PatchToolSchemaForClaudeCompatibility(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}
	props, ok := params["properties"].(map[string]any)
	if !ok {
		return params
	}

	// 如果有 path 属性但没有 file_path，添加 file_path 别名
	if _, hasPath := props["path"]; hasPath {
		if _, hasAlias := props["file_path"]; !hasAlias {
			patched := shallowCopy(params)
			patchedProps := shallowCopy(props)
			patchedProps["file_path"] = map[string]any{
				"type":        "string",
				"description": "Alias for 'path' parameter",
			}
			patched["properties"] = patchedProps
			return patched
		}
	}
	return params
}

// ---------- 沙箱路径守卫 ----------

// SandboxPathGuard 沙箱路径守卫。
type SandboxPathGuard struct {
	Root    string
	Enabled bool
}

// NewSandboxPathGuard 创建沙箱路径守卫。
func NewSandboxPathGuard(root string, enabled bool) *SandboxPathGuard {
	if root == "" {
		root = "/"
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	return &SandboxPathGuard{
		Root:    absRoot,
		Enabled: enabled,
	}
}

// Validate 验证路径是否在沙箱范围内。
func (g *SandboxPathGuard) Validate(path string) error {
	if !g.Enabled {
		return nil
	}
	if path == "" {
		return fmt.Errorf("path is required")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// 路径必须在沙箱根目录下
	if !strings.HasPrefix(absPath, g.Root) {
		return fmt.Errorf("path %q is outside sandbox root %q", path, g.Root)
	}

	return nil
}

// ResolvePath 解析并验证路径。
func (g *SandboxPathGuard) ResolvePath(path string) (string, error) {
	if err := g.Validate(path); err != nil {
		return "", err
	}
	return filepath.Abs(path)
}

// ---------- Read 工具 ----------

// ReadToolOptions Read 工具选项。
type ReadToolOptions struct {
	Sandbox     bool
	SandboxRoot string
	MaxFileSize int64
}

// CreateReadTool 创建文件读取工具。
// TS 参考: pi-tools.read.ts createOpenAcosmiReadTool
func CreateReadTool(opts ReadToolOptions) *AgentTool {
	guard := NewSandboxPathGuard(opts.SandboxRoot, opts.Sandbox)
	maxSize := opts.MaxFileSize
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024 // 10MB default
	}

	return &AgentTool{
		Name:        "read",
		Label:       "Read File",
		Description: "Read the contents of a file. Supports text files and returns base64 for binary files.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Absolute or relative path to the file to read",
				},
				"start_line": map[string]any{
					"type":        "number",
					"description": "Optional. First line to read (1-indexed)",
				},
				"end_line": map[string]any{
					"type":        "number",
					"description": "Optional. Last line to read (1-indexed, inclusive)",
				},
			},
			"required": []any{"path"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			normalized := NormalizeToolParams(args)
			path, err := ReadStringParam(normalized, "path", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}

			resolvedPath, err := guard.ResolvePath(path)
			if err != nil {
				return nil, err
			}

			info, err := os.Stat(resolvedPath)
			if err != nil {
				return nil, fmt.Errorf("file not found: %s", path)
			}
			if info.IsDir() {
				return nil, fmt.Errorf("path is a directory: %s", path)
			}
			if info.Size() > maxSize {
				return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxSize)
			}

			data, err := os.ReadFile(resolvedPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file: %w", err)
			}

			// 检查是否为二进制文件
			if isBinary(data) {
				// 返回图片结果
				mime := detectMimeFromBytes(data)
				if strings.HasPrefix(mime, "image/") {
					return ImageResultFromFile("Read File", resolvedPath, "", nil)
				}
				return JsonResult(map[string]any{
					"path":   path,
					"binary": true,
					"size":   len(data),
				}), nil
			}

			text := string(data)

			// 行范围处理
			startLine, hasStart, _ := ReadNumberParam(normalized, "start_line", &NumberParamOptions{Integer: true})
			endLine, hasEnd, _ := ReadNumberParam(normalized, "end_line", &NumberParamOptions{Integer: true})

			if hasStart || hasEnd {
				lines := strings.Split(text, "\n")
				start := 0
				end := len(lines)
				if hasStart && int(startLine) > 0 {
					start = int(startLine) - 1
				}
				if hasEnd && int(endLine) > 0 && int(endLine) <= len(lines) {
					end = int(endLine)
				}
				if start < 0 {
					start = 0
				}
				if end > len(lines) {
					end = len(lines)
				}
				if start >= end {
					text = ""
				} else {
					text = strings.Join(lines[start:end], "\n")
				}
			}

			return JsonResult(map[string]any{
				"path":    path,
				"content": text,
			}), nil
		},
	}
}

// isBinary 简单判断是否为二进制数据。
func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	limit := 512
	if len(data) < limit {
		limit = len(data)
	}
	nullCount := 0
	for _, b := range data[:limit] {
		if b == 0 {
			nullCount++
		}
	}
	return nullCount > 0
}
