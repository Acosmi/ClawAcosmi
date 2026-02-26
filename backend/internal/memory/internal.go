package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// MemoryFileEntry represents a discovered memory file.
type MemoryFileEntry struct {
	Path    string // relative path (forward slashes)
	AbsPath string
	MtimeMs int64
	Size    int64
	Hash    string
}

// MemoryChunk represents a chunk of text from a memory file.
type MemoryChunk struct {
	StartLine int
	EndLine   int
	Text      string
	Hash      string
}

// EnsureDir creates the directory (and parents) if it doesn't exist.
func EnsureDir(dir string) string {
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

// NormalizeRelPath normalises a relative path by stripping leading ./ and
// converting backslashes to forward slashes.
func NormalizeRelPath(value string) string {
	trimmed := strings.TrimLeft(strings.TrimSpace(value), "./")
	return strings.ReplaceAll(trimmed, "\\", "/")
}

// NormalizeExtraMemoryPaths resolves extra memory paths relative to the
// workspace directory, deduplicating the result.
func NormalizeExtraMemoryPaths(workspaceDir string, extraPaths []string) []string {
	if len(extraPaths) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var result []string
	for _, raw := range extraPaths {
		v := strings.TrimSpace(raw)
		if v == "" {
			continue
		}
		var resolved string
		if filepath.IsAbs(v) {
			resolved = filepath.Clean(v)
		} else {
			resolved = filepath.Clean(filepath.Join(workspaceDir, v))
		}
		if _, ok := seen[resolved]; ok {
			continue
		}
		seen[resolved] = struct{}{}
		result = append(result, resolved)
	}
	return result
}

// IsMemoryPath returns true if the relative path points to a memory file.
func IsMemoryPath(relPath string) bool {
	n := NormalizeRelPath(relPath)
	if n == "" {
		return false
	}
	if n == "MEMORY.md" || n == "memory.md" {
		return true
	}
	return strings.HasPrefix(n, "memory/")
}

// walkDir recursively collects .md files under dir (skipping symlinks).
func walkDir(dir string, files *[]string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		full := filepath.Join(dir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if entry.IsDir() {
			if err := walkDir(full, files); err != nil {
				continue
			}
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".md") {
			*files = append(*files, full)
		}
	}
	return nil
}

// ListMemoryFiles returns all Markdown files in the workspace memory
// directory and any extra paths.
func ListMemoryFiles(workspaceDir string, extraPaths []string) ([]string, error) {
	var result []string

	addFile := func(absPath string) {
		info, err := os.Lstat(absPath)
		if err != nil {
			return
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return
		}
		if !strings.HasSuffix(absPath, ".md") {
			return
		}
		result = append(result, absPath)
	}

	addFile(filepath.Join(workspaceDir, "MEMORY.md"))
	addFile(filepath.Join(workspaceDir, "memory.md"))

	memoryDir := filepath.Join(workspaceDir, "memory")
	if info, err := os.Lstat(memoryDir); err == nil {
		if info.Mode()&os.ModeSymlink == 0 && info.IsDir() {
			_ = walkDir(memoryDir, &result)
		}
	}

	normalized := NormalizeExtraMemoryPaths(workspaceDir, extraPaths)
	for _, inputPath := range normalized {
		info, err := os.Lstat(inputPath)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			_ = walkDir(inputPath, &result)
			continue
		}
		if info.Mode().IsRegular() && strings.HasSuffix(inputPath, ".md") {
			result = append(result, inputPath)
		}
	}

	if len(result) <= 1 {
		return result, nil
	}

	// Deduplicate by real path.
	seen := make(map[string]struct{})
	var deduped []string
	for _, entry := range result {
		key := entry
		if real, err := filepath.EvalSymlinks(entry); err == nil {
			key = real
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, entry)
	}
	return deduped, nil
}

// HashText returns the SHA-256 hex digest of a string.
func HashText(value string) string {
	h := sha256.Sum256([]byte(value))
	return hex.EncodeToString(h[:])
}

// BuildFileEntry builds a MemoryFileEntry by reading stats and hashing content.
func BuildFileEntry(absPath, workspaceDir string) (*MemoryFileEntry, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(workspaceDir, absPath)
	return &MemoryFileEntry{
		Path:    strings.ReplaceAll(rel, "\\", "/"),
		AbsPath: absPath,
		MtimeMs: info.ModTime().UnixMilli(),
		Size:    info.Size(),
		Hash:    HashText(string(content)),
	}, nil
}

// ChunkingConfig controls the markdown chunking algorithm.
type ChunkingConfig struct {
	Tokens  int
	Overlap int
}

// ChunkMarkdown splits content into overlapping chunks.
// RUST_CANDIDATE: P2 — hot-path during full reindex.
func ChunkMarkdown(content string, cfg ChunkingConfig) []MemoryChunk {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil
	}
	maxChars := max(32, cfg.Tokens*4)
	overlapChars := max(0, cfg.Overlap*4)

	type lineEntry struct {
		line   string
		lineNo int
	}

	var chunks []MemoryChunk
	var current []lineEntry
	currentChars := 0

	flush := func() {
		if len(current) == 0 {
			return
		}
		first := current[0]
		last := current[len(current)-1]
		var sb strings.Builder
		for i, e := range current {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(e.line)
		}
		text := sb.String()
		chunks = append(chunks, MemoryChunk{
			StartLine: first.lineNo,
			EndLine:   last.lineNo,
			Text:      text,
			Hash:      HashText(text),
		})
	}

	carryOverlap := func() {
		if overlapChars <= 0 || len(current) == 0 {
			current = current[:0]
			currentChars = 0
			return
		}
		acc := 0
		kept := 0
		for i := len(current) - 1; i >= 0; i-- {
			acc += len(current[i].line) + 1
			kept++
			if acc >= overlapChars {
				break
			}
		}
		start := len(current) - kept
		newCurrent := make([]lineEntry, kept)
		copy(newCurrent, current[start:])
		current = newCurrent
		currentChars = 0
		for _, e := range current {
			currentChars += len(e.line) + 1
		}
	}

	for i, line := range lines {
		lineNo := i + 1
		var segments []string
		if len(line) == 0 {
			segments = append(segments, "")
		} else {
			for start := 0; start < len(line); start += maxChars {
				end := start + maxChars
				if end > len(line) {
					end = len(line)
				}
				segments = append(segments, line[start:end])
			}
		}
		for _, seg := range segments {
			lineSize := len(seg) + 1
			if currentChars+lineSize > maxChars && len(current) > 0 {
				flush()
				carryOverlap()
			}
			current = append(current, lineEntry{line: seg, lineNo: lineNo})
			currentChars += lineSize
		}
	}
	flush()
	return chunks
}

// ParseEmbedding deserialises a JSON-encoded embedding vector.
func ParseEmbedding(raw string) []float64 {
	var vec []float64
	if err := json.Unmarshal([]byte(raw), &vec); err != nil {
		return nil
	}
	return vec
}

// embeddingToJSON serialises a float64 embedding vector to a JSON string.
func embeddingToJSON(vec []float64) string {
	data, err := json.Marshal(vec)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// CosineSimilarity computes the cosine similarity of two vectors.
// RUST_CANDIDATE: P1 — called in inner search loop.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, normA, normB float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// RunWithConcurrency runs task functions with a concurrency limit.
// The first error aborts remaining tasks.
func RunWithConcurrency[T any](ctx context.Context, tasks []func() (T, error), limit int) ([]T, error) {
	if len(tasks) == 0 {
		return nil, nil
	}
	if limit < 1 {
		limit = 1
	}
	if limit > len(tasks) {
		limit = len(tasks)
	}

	results := make([]T, len(tasks))
	var firstErr atomic.Value
	var next atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < limit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if firstErr.Load() != nil {
					return
				}
				select {
				case <-ctx.Done():
					firstErr.CompareAndSwap(nil, ctx.Err())
					return
				default:
				}
				idx := int(next.Add(1) - 1)
				if idx >= len(tasks) {
					return
				}
				result, err := tasks[idx]()
				if err != nil {
					firstErr.CompareAndSwap(nil, err)
					return
				}
				results[idx] = result
			}
		}()
	}

	wg.Wait()
	if v := firstErr.Load(); v != nil {
		return nil, v.(error)
	}
	return results, nil
}
