// bash/apply_patch_update.go — 文件更新 hunk 应用。
// TS 参考：src/agents/apply-patch-update.ts (200L)
//
// 提供 seekSequence / applyUpdateHunk 等核心逻辑，
// 支持精确匹配、trimEnd 匹配、trim 匹配和 Unicode 标点规范化匹配。
package bash

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

// ApplyUpdateHunk 读取文件内容并应用 update 补丁块。
// TS 参考: apply-patch-update.ts L10-29
func ApplyUpdateHunk(filePath string, chunks []UpdateFileChunk) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file to update %s: %w", filePath, err)
	}

	originalLines := strings.Split(string(content), "\n")
	if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
		originalLines = originalLines[:len(originalLines)-1]
	}

	replacements, err := computeReplacements(originalLines, filePath, chunks)
	if err != nil {
		return "", err
	}

	newLines := applyReplacements(originalLines, replacements)
	if len(newLines) == 0 || newLines[len(newLines)-1] != "" {
		newLines = append(newLines, "")
	}
	return strings.Join(newLines, "\n"), nil
}

// replacement 表示一个替换操作: [起始行, 删除行数, 新行列表]
type replacement struct {
	start    int
	oldLen   int
	newLines []string
}

// computeReplacements 计算所有替换位置。
// TS 参考: apply-patch-update.ts L31-81
func computeReplacements(originalLines []string, filePath string, chunks []UpdateFileChunk) ([]replacement, error) {
	var replacements []replacement
	lineIndex := 0

	for _, chunk := range chunks {
		if chunk.ChangeContext != "" {
			ctxIndex := seekSequence(originalLines, []string{chunk.ChangeContext}, lineIndex, false)
			if ctxIndex < 0 {
				return nil, fmt.Errorf("failed to find context '%s' in %s", chunk.ChangeContext, filePath)
			}
			lineIndex = ctxIndex + 1
		}

		if len(chunk.OldLines) == 0 {
			insertionIndex := len(originalLines)
			if len(originalLines) > 0 && originalLines[len(originalLines)-1] == "" {
				insertionIndex = len(originalLines) - 1
			}
			replacements = append(replacements, replacement{
				start:    insertionIndex,
				oldLen:   0,
				newLines: chunk.NewLines,
			})
			continue
		}

		pattern := chunk.OldLines
		newSlice := chunk.NewLines
		found := seekSequence(originalLines, pattern, lineIndex, chunk.IsEndOfFile)

		// 如果末尾有空行导致匹配失败，尝试去掉末尾空行
		if found < 0 && len(pattern) > 0 && pattern[len(pattern)-1] == "" {
			pattern = pattern[:len(pattern)-1]
			if len(newSlice) > 0 && newSlice[len(newSlice)-1] == "" {
				newSlice = newSlice[:len(newSlice)-1]
			}
			found = seekSequence(originalLines, pattern, lineIndex, chunk.IsEndOfFile)
		}

		if found < 0 {
			return nil, fmt.Errorf("failed to find expected lines in %s:\n%s", filePath, strings.Join(chunk.OldLines, "\n"))
		}

		replacements = append(replacements, replacement{
			start:    found,
			oldLen:   len(pattern),
			newLines: newSlice,
		})
		lineIndex = found + len(pattern)
	}

	sort.Slice(replacements, func(i, j int) bool {
		return replacements[i].start < replacements[j].start
	})
	return replacements, nil
}

// applyReplacements 按倒序应用替换。
// TS 参考: apply-patch-update.ts L83-99
func applyReplacements(lines []string, replacements []replacement) []string {
	result := make([]string, len(lines))
	copy(result, lines)

	// 倒序应用以避免索引偏移
	for i := len(replacements) - 1; i >= 0; i-- {
		r := replacements[i]
		start := r.start
		oldLen := r.oldLen

		// 删除旧行
		tail := make([]string, len(result[start+oldLen:]))
		copy(tail, result[start+oldLen:])

		// 构建新数组
		newResult := make([]string, 0, start+len(r.newLines)+len(tail))
		newResult = append(newResult, result[:start]...)
		newResult = append(newResult, r.newLines...)
		newResult = append(newResult, tail...)
		result = newResult
	}
	return result
}

// seekSequence 在 lines 中从 start 开始搜索 pattern。
// 使用逐步放宽匹配策略：精确 → trimEnd → trim → Unicode标点规范化。
// TS 参考: apply-patch-update.ts L101-142
func seekSequence(lines, pattern []string, start int, eof bool) int {
	if len(pattern) == 0 {
		return start
	}
	if len(pattern) > len(lines) {
		return -1
	}

	maxStart := len(lines) - len(pattern)
	searchStart := start
	if eof && len(lines) >= len(pattern) {
		searchStart = maxStart
	}
	if searchStart > maxStart {
		return -1
	}

	// Pass 1: 精确匹配
	for i := searchStart; i <= maxStart; i++ {
		if linesMatchWith(lines, pattern, i, identity) {
			return i
		}
	}
	// Pass 2: trimEnd 匹配
	for i := searchStart; i <= maxStart; i++ {
		if linesMatchWith(lines, pattern, i, trimEnd) {
			return i
		}
	}
	// Pass 3: trim 匹配
	for i := searchStart; i <= maxStart; i++ {
		if linesMatchWith(lines, pattern, i, trimBoth) {
			return i
		}
	}
	// Pass 4: Unicode 标点规范化匹配
	for i := searchStart; i <= maxStart; i++ {
		if linesMatchWith(lines, pattern, i, normalizeAndTrim) {
			return i
		}
	}

	return -1
}

type normalizer func(string) string

func identity(s string) string { return s }
func trimEnd(s string) string  { return strings.TrimRight(s, " \t") }
func trimBoth(s string) string { return strings.TrimSpace(s) }
func normalizeAndTrim(s string) string {
	return normalizePunctuation(strings.TrimSpace(s))
}

func linesMatchWith(lines, pattern []string, start int, normalize normalizer) bool {
	for idx := 0; idx < len(pattern); idx++ {
		if start+idx >= len(lines) {
			return false
		}
		if normalize(lines[start+idx]) != normalize(pattern[idx]) {
			return false
		}
	}
	return true
}

// normalizePunctuation 规范化 Unicode 标点符号。
// TS 参考: apply-patch-update.ts L158-199
func normalizePunctuation(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch r {
		// 各种 Unicode 破折号 → ASCII 连字符
		case '\u2010', '\u2011', '\u2012', '\u2013', '\u2014', '\u2015', '\u2212':
			b.WriteByte('-')
		// 各种 Unicode 单引号 → ASCII 单引号
		case '\u2018', '\u2019', '\u201A', '\u201B':
			b.WriteByte('\'')
		// 各种 Unicode 双引号 → ASCII 双引号
		case '\u201C', '\u201D', '\u201E', '\u201F':
			b.WriteByte('"')
		// 各种 Unicode 空白 → ASCII 空格
		default:
			if isUnicodeSpace(r) {
				b.WriteByte(' ')
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

func isUnicodeSpace(r rune) bool {
	switch r {
	case '\u00A0', '\u2002', '\u2003', '\u2004', '\u2005', '\u2006',
		'\u2007', '\u2008', '\u2009', '\u200A', '\u202F', '\u205F', '\u3000':
		return true
	}
	return unicode.IsSpace(r) && r != ' ' && r != '\t' && r != '\n' && r != '\r'
}
