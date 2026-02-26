package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// QMDManager manages a QMD subprocess for external index management.
// It wraps the qmd CLI tool and provides search/sync operations.
// Ported from qmd-manager.ts (908 lines).
type QMDManager struct {
	mu     sync.RWMutex
	config *ResolvedQmdConfig
	logger *slog.Logger
	closed bool
}

// QMDManagerConfig holds configuration for creating a QMDManager.
type QMDManagerConfig struct {
	Config *ResolvedQmdConfig
	Logger *slog.Logger
}

// NewQMDManager creates a new QMD index manager.
func NewQMDManager(cfg QMDManagerConfig) *QMDManager {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &QMDManager{
		config: cfg.Config,
		logger: cfg.Logger,
	}
}

// qmdSearchResult is the JSON output structure of `qmd search --json`.
type qmdSearchResult struct {
	Path      string  `json:"path"`
	StartLine int     `json:"startLine"`
	EndLine   int     `json:"endLine"`
	Score     float64 `json:"score"`
	Snippet   string  `json:"snippet"`
	Source    string  `json:"source"`
}

// Search queries the QMD index via subprocess.
// TS 对照: qmd-manager.ts → search()
func (q *QMDManager) Search(ctx context.Context, query string, opts *SearchOptions) ([]MemorySearchResult, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.closed {
		return nil, fmt.Errorf("qmd: manager closed")
	}

	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	maxResults := 6
	if opts != nil && opts.MaxResults > 0 {
		maxResults = opts.MaxResults
	}

	timeoutMs := q.config.Limits.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 4000
	}

	q.logger.Debug("qmd search", "query", query, "maxResults", maxResults)

	args := []string{"search", "--query", query, "--limit", strconv.Itoa(maxResults), "--json"}
	out, err := q.qmdExec(ctx, args, time.Duration(timeoutMs)*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("qmd search: %w", err)
	}

	var rawResults []qmdSearchResult
	if err := json.Unmarshal(out, &rawResults); err != nil {
		return nil, fmt.Errorf("qmd search: parse output: %w", err)
	}

	results := make([]MemorySearchResult, 0, len(rawResults))
	for _, r := range rawResults {
		if opts != nil && opts.MinScore > 0 && r.Score < opts.MinScore {
			continue
		}
		source := MemorySource(r.Source)
		if source == "" {
			source = SourceMemory
		}
		results = append(results, MemorySearchResult{
			Path:      r.Path,
			StartLine: r.StartLine,
			EndLine:   r.EndLine,
			Score:     r.Score,
			Snippet:   r.Snippet,
			Source:    source,
		})
	}
	return results, nil
}

// Sync triggers a QMD index update via subprocess.
// TS 对照: qmd-manager.ts → syncQmdIndex()
func (q *QMDManager) Sync(ctx context.Context, opts *SyncOptions) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return fmt.Errorf("qmd: manager closed")
	}

	reason := "manual"
	if opts != nil && opts.Reason != "" {
		reason = opts.Reason
	}
	q.logger.Info("qmd sync start", "reason", reason)

	updateTimeout := time.Duration(q.config.Update.UpdateTimeoutMs) * time.Millisecond
	if updateTimeout <= 0 {
		updateTimeout = 120 * time.Second
	}

	// Step 1: qmd update
	if _, err := q.qmdExec(ctx, []string{"update"}, updateTimeout); err != nil {
		return fmt.Errorf("qmd update: %w", err)
	}

	// Step 2: qmd embed
	embedTimeout := time.Duration(q.config.Update.EmbedTimeoutMs) * time.Millisecond
	if embedTimeout <= 0 {
		embedTimeout = 120 * time.Second
	}
	if _, err := q.qmdExec(ctx, []string{"embed"}, embedTimeout); err != nil {
		return fmt.Errorf("qmd embed: %w", err)
	}

	q.logger.Info("qmd sync complete", "reason", reason)
	return nil
}

// qmdExec executes a qmd subprocess with the given args and timeout.
func (q *QMDManager) qmdExec(ctx context.Context, args []string, timeout time.Duration) ([]byte, error) {
	binPath := q.config.Command
	if binPath == "" {
		binPath = "qmd"
	}

	// Check binary exists
	if _, err := exec.LookPath(binPath); err != nil {
		return nil, fmt.Errorf("qmd binary not found: %s (install qmd or set config.memory.qmd.command)", binPath)
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, binPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		q.logger.Warn("qmd exec failed", "args", args, "err", err, "output", string(out))
		return nil, fmt.Errorf("qmd exec %v: %w (output: %s)", args, err, string(out))
	}
	return out, nil
}

// Status returns QMD backend status.
func (q *QMDManager) Status() MemoryProviderStatus {
	q.mu.RLock()
	defer q.mu.RUnlock()

	return MemoryProviderStatus{
		Backend:  string(BackendQMD),
		Provider: "qmd",
	}
}

// Close stops the QMD manager.
func (q *QMDManager) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	return nil
}
