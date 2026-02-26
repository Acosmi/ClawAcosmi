// bash/apply_patch.go — 补丁应用逻辑。
// TS 参考：src/agents/apply-patch.ts (504L)
//
// 解析 *** Begin Patch / *** End Patch 格式的补丁文本，
// 支持 Add/Delete/Update 文件操作，自动创建目录。
package bash

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ---------- 常量 ----------

const (
	BeginPatchMarker         = "*** Begin Patch"
	EndPatchMarker           = "*** End Patch"
	AddFileMarker            = "*** Add File: "
	DeleteFileMarker         = "*** Delete File: "
	UpdateFileMarker         = "*** Update File: "
	MoveToMarker             = "*** Move to: "
	EOFMarker                = "*** End of File"
	ChangeContextMarker      = "@@ "
	EmptyChangeContextMarker = "@@"
)

var unicodeSpacesRe = regexp.MustCompile(`[\x{00A0}\x{2000}-\x{200A}\x{202F}\x{205F}\x{3000}]`)

// ---------- 类型 ----------

// HunkKind 区分补丁操作类型。
type HunkKind string

const (
	HunkAdd    HunkKind = "add"
	HunkDelete HunkKind = "delete"
	HunkUpdate HunkKind = "update"
)

// AddFileHunk 添加文件操作。
type AddFileHunk struct {
	Path     string
	Contents string
}

// DeleteFileHunk 删除文件操作。
type DeleteFileHunk struct {
	Path string
}

// UpdateFileChunk 更新文件中的一个分块。
type UpdateFileChunk struct {
	ChangeContext string
	OldLines      []string
	NewLines      []string
	IsEndOfFile   bool
}

// UpdateFileHunk 更新文件操作。
type UpdateFileHunk struct {
	Path     string
	MovePath string
	Chunks   []UpdateFileChunk
}

// Hunk 统一的补丁操作。
type Hunk struct {
	Kind   HunkKind
	Add    *AddFileHunk
	Delete *DeleteFileHunk
	Update *UpdateFileHunk
}

// ApplyPatchSummary 补丁应用摘要。
type ApplyPatchSummary struct {
	Added    []string `json:"added"`
	Modified []string `json:"modified"`
	Deleted  []string `json:"deleted"`
}

// ApplyPatchResult 补丁应用结果。
type ApplyPatchResult struct {
	Summary ApplyPatchSummary `json:"summary"`
	Text    string            `json:"text"`
}

// ApplyPatchOptions 补丁选项。
type ApplyPatchOptions struct {
	Cwd         string
	SandboxRoot string
}

// ---------- 核心函数 ----------

// ApplyPatch 应用补丁文本。
// TS 参考: apply-patch.ts L113-174
func ApplyPatch(input string, opts ApplyPatchOptions) (*ApplyPatchResult, error) {
	parsed, err := parsePatchText(input)
	if err != nil {
		return nil, err
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("no files were modified")
	}

	summary := ApplyPatchSummary{
		Added:    make([]string, 0),
		Modified: make([]string, 0),
		Deleted:  make([]string, 0),
	}
	seen := map[string]map[string]bool{
		"added":    {},
		"modified": {},
		"deleted":  {},
	}

	for _, hunk := range parsed {
		switch hunk.Kind {
		case HunkAdd:
			target := resolvePatchPath(hunk.Add.Path, opts)
			if err := ensureDir(target.resolved); err != nil {
				return nil, fmt.Errorf("ensure dir for %s: %w", target.resolved, err)
			}
			if err := os.WriteFile(target.resolved, []byte(hunk.Add.Contents), 0644); err != nil {
				return nil, fmt.Errorf("write file %s: %w", target.resolved, err)
			}
			recordPatchSummary(&summary, seen, "added", target.display)

		case HunkDelete:
			target := resolvePatchPath(hunk.Delete.Path, opts)
			if err := os.Remove(target.resolved); err != nil {
				return nil, fmt.Errorf("delete file %s: %w", target.resolved, err)
			}
			recordPatchSummary(&summary, seen, "deleted", target.display)

		case HunkUpdate:
			target := resolvePatchPath(hunk.Update.Path, opts)
			applied, err := ApplyUpdateHunk(target.resolved, hunk.Update.Chunks)
			if err != nil {
				return nil, fmt.Errorf("apply update to %s: %w", target.resolved, err)
			}

			if hunk.Update.MovePath != "" {
				moveTarget := resolvePatchPath(hunk.Update.MovePath, opts)
				if err := ensureDir(moveTarget.resolved); err != nil {
					return nil, fmt.Errorf("ensure dir for %s: %w", moveTarget.resolved, err)
				}
				if err := os.WriteFile(moveTarget.resolved, []byte(applied), 0644); err != nil {
					return nil, fmt.Errorf("write moved file %s: %w", moveTarget.resolved, err)
				}
				if err := os.Remove(target.resolved); err != nil {
					return nil, fmt.Errorf("remove original %s: %w", target.resolved, err)
				}
				recordPatchSummary(&summary, seen, "modified", moveTarget.display)
			} else {
				if err := os.WriteFile(target.resolved, []byte(applied), 0644); err != nil {
					return nil, fmt.Errorf("write updated file %s: %w", target.resolved, err)
				}
				recordPatchSummary(&summary, seen, "modified", target.display)
			}
		}
	}

	return &ApplyPatchResult{
		Summary: summary,
		Text:    formatPatchSummary(summary),
	}, nil
}

// ---------- 补丁解析 ----------

func parsePatchText(input string) ([]Hunk, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("invalid patch: input is empty")
	}

	lines := strings.Split(trimmed, "\n")
	// 规范化 \r\n
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, "\r")
	}

	validated, err := checkPatchBoundariesLenient(lines)
	if err != nil {
		return nil, err
	}

	var hunks []Hunk
	lastIdx := len(validated) - 1
	remaining := validated[1:lastIdx]
	lineNumber := 2

	for len(remaining) > 0 {
		hunk, consumed, err := parseOneHunk(remaining, lineNumber)
		if err != nil {
			return nil, err
		}
		hunks = append(hunks, hunk)
		lineNumber += consumed
		remaining = remaining[consumed:]
	}

	return hunks, nil
}

func checkPatchBoundariesLenient(lines []string) ([]string, error) {
	err := checkPatchBoundariesStrict(lines)
	if err == nil {
		return lines, nil
	}

	if len(lines) < 4 {
		return nil, err
	}
	first := lines[0]
	last := lines[len(lines)-1]
	if (first == "<<EOF" || first == "<<'EOF'" || first == `<<"EOF"`) && strings.HasSuffix(last, "EOF") {
		inner := lines[1 : len(lines)-1]
		if checkPatchBoundariesStrict(inner) == nil {
			return inner, nil
		}
	}
	return nil, err
}

func checkPatchBoundariesStrict(lines []string) error {
	if len(lines) == 0 {
		return fmt.Errorf("invalid patch: empty")
	}
	firstLine := strings.TrimSpace(lines[0])
	lastLine := strings.TrimSpace(lines[len(lines)-1])

	if firstLine == BeginPatchMarker && lastLine == EndPatchMarker {
		return nil
	}
	if firstLine != BeginPatchMarker {
		return fmt.Errorf("the first line of the patch must be '*** Begin Patch'")
	}
	return fmt.Errorf("the last line of the patch must be '*** End Patch'")
}

func parseOneHunk(lines []string, lineNumber int) (Hunk, int, error) {
	if len(lines) == 0 {
		return Hunk{}, 0, fmt.Errorf("invalid patch hunk at line %d: empty hunk", lineNumber)
	}
	firstLine := strings.TrimSpace(lines[0])

	// Add File
	if strings.HasPrefix(firstLine, AddFileMarker) {
		targetPath := firstLine[len(AddFileMarker):]
		var contents strings.Builder
		consumed := 1
		for _, line := range lines[1:] {
			if strings.HasPrefix(line, "+") {
				contents.WriteString(line[1:])
				contents.WriteString("\n")
				consumed++
			} else {
				break
			}
		}
		return Hunk{
			Kind: HunkAdd,
			Add:  &AddFileHunk{Path: targetPath, Contents: contents.String()},
		}, consumed, nil
	}

	// Delete File
	if strings.HasPrefix(firstLine, DeleteFileMarker) {
		targetPath := firstLine[len(DeleteFileMarker):]
		return Hunk{
			Kind:   HunkDelete,
			Delete: &DeleteFileHunk{Path: targetPath},
		}, 1, nil
	}

	// Update File
	if strings.HasPrefix(firstLine, UpdateFileMarker) {
		targetPath := firstLine[len(UpdateFileMarker):]
		remaining := lines[1:]
		consumed := 1
		var movePath string

		if len(remaining) > 0 {
			candidate := strings.TrimSpace(remaining[0])
			if strings.HasPrefix(candidate, MoveToMarker) {
				movePath = candidate[len(MoveToMarker):]
				remaining = remaining[1:]
				consumed++
			}
		}

		var chunks []UpdateFileChunk
		for len(remaining) > 0 {
			if strings.TrimSpace(remaining[0]) == "" {
				remaining = remaining[1:]
				consumed++
				continue
			}
			if strings.HasPrefix(remaining[0], "***") {
				break
			}
			chunk, chunkLines, err := parseUpdateFileChunk(remaining, lineNumber+consumed, len(chunks) == 0)
			if err != nil {
				return Hunk{}, 0, err
			}
			chunks = append(chunks, chunk)
			remaining = remaining[chunkLines:]
			consumed += chunkLines
		}

		if len(chunks) == 0 {
			return Hunk{}, 0, fmt.Errorf("invalid patch hunk at line %d: update file hunk for path '%s' is empty", lineNumber, targetPath)
		}

		return Hunk{
			Kind: HunkUpdate,
			Update: &UpdateFileHunk{
				Path:     targetPath,
				MovePath: movePath,
				Chunks:   chunks,
			},
		}, consumed, nil
	}

	return Hunk{}, 0, fmt.Errorf("invalid patch hunk at line %d: '%s' is not a valid hunk header", lineNumber, lines[0])
}

func parseUpdateFileChunk(lines []string, lineNumber int, allowMissingContext bool) (UpdateFileChunk, int, error) {
	if len(lines) == 0 {
		return UpdateFileChunk{}, 0, fmt.Errorf("invalid patch hunk at line %d: update hunk does not contain any lines", lineNumber)
	}

	var changeContext string
	startIndex := 0

	if lines[0] == EmptyChangeContextMarker {
		startIndex = 1
	} else if strings.HasPrefix(lines[0], ChangeContextMarker) {
		changeContext = lines[0][len(ChangeContextMarker):]
		startIndex = 1
	} else if !allowMissingContext {
		return UpdateFileChunk{}, 0, fmt.Errorf("invalid patch hunk at line %d: expected update hunk to start with @@ context marker, got: '%s'", lineNumber, lines[0])
	}

	if startIndex >= len(lines) {
		return UpdateFileChunk{}, 0, fmt.Errorf("invalid patch hunk at line %d: update hunk does not contain any lines", lineNumber+1)
	}

	chunk := UpdateFileChunk{
		ChangeContext: changeContext,
		OldLines:      make([]string, 0),
		NewLines:      make([]string, 0),
	}

	parsedLines := 0
	for _, line := range lines[startIndex:] {
		if line == EOFMarker {
			if parsedLines == 0 {
				return UpdateFileChunk{}, 0, fmt.Errorf("invalid patch hunk at line %d: update hunk does not contain any lines", lineNumber+1)
			}
			chunk.IsEndOfFile = true
			parsedLines++
			break
		}

		if len(line) == 0 {
			chunk.OldLines = append(chunk.OldLines, "")
			chunk.NewLines = append(chunk.NewLines, "")
			parsedLines++
			continue
		}

		marker := line[0]
		switch marker {
		case ' ':
			content := line[1:]
			chunk.OldLines = append(chunk.OldLines, content)
			chunk.NewLines = append(chunk.NewLines, content)
			parsedLines++
		case '+':
			chunk.NewLines = append(chunk.NewLines, line[1:])
			parsedLines++
		case '-':
			chunk.OldLines = append(chunk.OldLines, line[1:])
			parsedLines++
		default:
			if parsedLines == 0 {
				return UpdateFileChunk{}, 0, fmt.Errorf("invalid patch hunk at line %d: unexpected line found in update hunk: '%s'. Every line should start with ' ', '+', or '-'", lineNumber+1, line)
			}
			goto done
		}
	}
done:

	return chunk, parsedLines + startIndex, nil
}

// ---------- 路径处理 ----------

type resolvedPath struct {
	resolved string
	display  string
}

func resolvePatchPath(filePath string, opts ApplyPatchOptions) resolvedPath {
	resolved := resolvePathFromCwd(filePath, opts.Cwd)
	return resolvedPath{
		resolved: resolved,
		display:  toDisplayPath(resolved, opts.Cwd),
	}
}

func normalizeUnicodeSpaces(value string) string {
	return unicodeSpacesRe.ReplaceAllString(value, " ")
}

func expandPath(filePath string) string {
	normalized := normalizeUnicodeSpaces(filePath)
	if normalized == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(normalized, "~/") {
		home, _ := os.UserHomeDir()
		return home + normalized[1:]
	}
	return normalized
}

func resolvePathFromCwd(filePath, cwd string) string {
	expanded := expandPath(filePath)
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded)
	}
	return filepath.Join(cwd, expanded)
}

func toDisplayPath(resolved, cwd string) string {
	rel, err := filepath.Rel(cwd, resolved)
	if err != nil || rel == "" {
		return filepath.Base(resolved)
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return resolved
	}
	return rel
}

func ensureDir(filePath string) error {
	parent := filepath.Dir(filePath)
	if parent == "" || parent == "." {
		return nil
	}
	return os.MkdirAll(parent, 0755)
}

func recordPatchSummary(summary *ApplyPatchSummary, seen map[string]map[string]bool, bucket, value string) {
	if seen[bucket][value] {
		return
	}
	seen[bucket][value] = true
	switch bucket {
	case "added":
		summary.Added = append(summary.Added, value)
	case "modified":
		summary.Modified = append(summary.Modified, value)
	case "deleted":
		summary.Deleted = append(summary.Deleted, value)
	}
}

func formatPatchSummary(summary ApplyPatchSummary) string {
	lines := []string{"Success. Updated the following files:"}
	for _, f := range summary.Added {
		lines = append(lines, "A "+f)
	}
	for _, f := range summary.Modified {
		lines = append(lines, "M "+f)
	}
	for _, f := range summary.Deleted {
		lines = append(lines, "D "+f)
	}
	return strings.Join(lines, "\n")
}

// ---------- 工具定义 ----------

// ApplyPatchToolSchema 返回 apply_patch 工具 JSON schema。
func ApplyPatchToolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{
				"type":        "string",
				"description": "Patch content using the *** Begin Patch/End Patch format.",
			},
		},
		"required": []string{"input"},
	}
}
