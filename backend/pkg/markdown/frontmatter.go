// frontmatter.go — YAML/行式 frontmatter 解析。
//
// TS 对照: markdown/frontmatter.ts (158L)
//
// 解析 Markdown 开头的 --- 分隔的 frontmatter 块。
// 支持两种格式:
//   - YAML 格式（使用 gopkg.in/yaml.v3）
//   - 行式 key: value 格式（含多行值支持）
//
// 两种格式结果合并，YAML 优先，行式中的 JSON 值（{...}, [...]）覆盖。
package markdown

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParsedFrontmatter frontmatter 解析结果。
// TS 对照: frontmatter.ts ParsedFrontmatter = Record<string, string>
type ParsedFrontmatter map[string]string

// lineFrontmatterKeyPattern 匹配 key: value 行。
var lineFrontmatterKeyPattern = regexp.MustCompile(`^([\w-]+):\s*(.*)$`)

// stripQuotes 去除首尾引号。
// TS 对照: frontmatter.ts stripQuotes()
func stripQuotes(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

// coerceFrontmatterValue 将任意值强制转为字符串。
// TS 对照: frontmatter.ts coerceFrontmatterValue()
func coerceFrontmatterValue(value interface{}) (string, bool) {
	if value == nil {
		return "", false
	}
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	case bool:
		return fmt.Sprintf("%v", v), true
	case int:
		return fmt.Sprintf("%d", v), true
	case float64:
		// 整数值去掉小数点
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v)), true
		}
		return fmt.Sprintf("%g", v), true
	default:
		// 复杂类型 JSON 序列化
		b, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
}

// parseYamlFrontmatter 用 YAML 解析器解析 frontmatter 块。
// TS 对照: frontmatter.ts parseYamlFrontmatter()
func parseYamlFrontmatter(block string) (ParsedFrontmatter, bool) {
	var parsed interface{}
	if err := yaml.Unmarshal([]byte(block), &parsed); err != nil {
		return nil, false
	}

	m, ok := parsed.(map[string]interface{})
	if !ok || m == nil {
		return nil, false
	}

	result := make(ParsedFrontmatter)
	for rawKey, value := range m {
		key := strings.TrimSpace(rawKey)
		if key == "" {
			continue
		}
		coerced, valid := coerceFrontmatterValue(value)
		if !valid {
			continue
		}
		result[key] = coerced
	}
	return result, true
}

// extractMultiLineValue 提取多行值（缩进续行）。
// TS 对照: frontmatter.ts extractMultiLineValue()
func extractMultiLineValue(lines []string, startIndex int) (string, int) {
	if startIndex >= len(lines) {
		return "", 1
	}
	startLine := lines[startIndex]
	m := lineFrontmatterKeyPattern.FindStringSubmatch(startLine)
	if m == nil {
		return "", 1
	}

	inlineValue := strings.TrimSpace(m[2])
	if inlineValue != "" {
		return inlineValue, 1
	}

	var valueLines []string
	i := startIndex + 1
	for i < len(lines) {
		line := lines[i]
		if len(line) > 0 && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			break
		}
		valueLines = append(valueLines, line)
		i++
	}

	combined := strings.TrimSpace(strings.Join(valueLines, "\n"))
	return combined, i - startIndex
}

// parseLineFrontmatter 用行式解析器解析 frontmatter。
// TS 对照: frontmatter.ts parseLineFrontmatter()
func parseLineFrontmatter(block string) ParsedFrontmatter {
	result := make(ParsedFrontmatter)
	lines := strings.Split(block, "\n")
	i := 0

	for i < len(lines) {
		line := lines[i]
		m := lineFrontmatterKeyPattern.FindStringSubmatch(line)
		if m == nil {
			i++
			continue
		}

		key := m[1]
		inlineValue := strings.TrimSpace(m[2])

		if key == "" {
			i++
			continue
		}

		// 检查多行值
		if inlineValue == "" && i+1 < len(lines) {
			nextLine := lines[i+1]
			if strings.HasPrefix(nextLine, " ") || strings.HasPrefix(nextLine, "\t") {
				value, consumed := extractMultiLineValue(lines, i)
				if value != "" {
					result[key] = value
				}
				i += consumed
				continue
			}
		}

		value := stripQuotes(inlineValue)
		if value != "" {
			result[key] = value
		}
		i++
	}

	return result
}

// ParseFrontmatterBlock 解析 Markdown 内容的 frontmatter 块。
//
// 查找 --- 分隔的 frontmatter，同时用 YAML 和行式解析，然后合并。
// YAML 结果优先，行式中以 { 或 [ 开头的值覆盖 YAML 结果。
//
// TS 对照: frontmatter.ts parseFrontmatterBlock()
func ParseFrontmatterBlock(content string) ParsedFrontmatter {
	// 规范化换行
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	if !strings.HasPrefix(normalized, "---") {
		return make(ParsedFrontmatter)
	}

	endIndex := strings.Index(normalized[3:], "\n---")
	if endIndex < 0 {
		return make(ParsedFrontmatter)
	}

	// 提取 frontmatter 块（跳过开头 "---\n" 的 4 个字符）
	block := normalized[4 : 3+endIndex]

	lineParsed := parseLineFrontmatter(block)
	yamlParsed, yamlOK := parseYamlFrontmatter(block)

	if !yamlOK {
		return lineParsed
	}

	// 合并: YAML 为基础，行式中的 JSON 值覆盖
	merged := make(ParsedFrontmatter)
	for k, v := range yamlParsed {
		merged[k] = v
	}
	for k, v := range lineParsed {
		if strings.HasPrefix(v, "{") || strings.HasPrefix(v, "[") {
			merged[k] = v
		}
	}

	return merged
}
