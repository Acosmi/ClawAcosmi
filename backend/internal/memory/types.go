// Package memory provides the memory system for indexing, embedding, and
// searching agent workspace files and session transcripts.
package memory

import "context"

// MemorySource distinguishes the origin of indexed content.
type MemorySource string

const (
	SourceMemory   MemorySource = "memory"
	SourceSessions MemorySource = "sessions"
)

// MemorySearchResult represents a single search hit.
type MemorySearchResult struct {
	Path      string       `json:"path"`
	StartLine int          `json:"startLine"`
	EndLine   int          `json:"endLine"`
	Score     float64      `json:"score"`
	Snippet   string       `json:"snippet"`
	Source    MemorySource `json:"source"`
	Citation  string       `json:"citation,omitempty"`
}

// MemoryEmbeddingProbeResult reports whether the embedding backend is alive.
type MemoryEmbeddingProbeResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// MemorySyncProgressUpdate is emitted during an index sync.
type MemorySyncProgressUpdate struct {
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
	Label     string `json:"label,omitempty"`
}

// MemoryProviderStatus describes the current state of the memory backend.
type MemoryProviderStatus struct {
	Backend           string         `json:"backend"` // "builtin" | "qmd"
	Provider          string         `json:"provider"`
	Model             string         `json:"model,omitempty"`
	RequestedProvider string         `json:"requestedProvider,omitempty"`
	Files             *int           `json:"files,omitempty"`
	Chunks            *int           `json:"chunks,omitempty"`
	Dirty             *bool          `json:"dirty,omitempty"`
	WorkspaceDir      string         `json:"workspaceDir,omitempty"`
	DBPath            string         `json:"dbPath,omitempty"`
	ExtraPaths        []string       `json:"extraPaths,omitempty"`
	Sources           []MemorySource `json:"sources,omitempty"`

	SourceCounts []SourceCount  `json:"sourceCounts,omitempty"`
	Cache        *CacheStatus   `json:"cache,omitempty"`
	FTS          *FTSStatus     `json:"fts,omitempty"`
	Fallback     *FallbackInfo  `json:"fallback,omitempty"`
	Vector       *VectorStatus  `json:"vector,omitempty"`
	Batch        *BatchStatus   `json:"batch,omitempty"`
	Custom       map[string]any `json:"custom,omitempty"`
}

// SourceCount breaks down file/chunk counts by source.
type SourceCount struct {
	Source MemorySource `json:"source"`
	Files  int          `json:"files"`
	Chunks int          `json:"chunks"`
}

// CacheStatus reports the search result cache state.
type CacheStatus struct {
	Enabled    bool `json:"enabled"`
	Entries    *int `json:"entries,omitempty"`
	MaxEntries *int `json:"maxEntries,omitempty"`
}

// FTSStatus reports FTS5 full-text search availability.
type FTSStatus struct {
	Enabled   bool   `json:"enabled"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
}

// FallbackInfo records a provider fallback event.
type FallbackInfo struct {
	From   string `json:"from"`
	Reason string `json:"reason,omitempty"`
}

// VectorStatus reports sqlite-vec vector search availability.
type VectorStatus struct {
	Enabled       bool   `json:"enabled"`
	Available     *bool  `json:"available,omitempty"`
	ExtensionPath string `json:"extensionPath,omitempty"`
	LoadError     string `json:"loadError,omitempty"`
	Dims          *int   `json:"dims,omitempty"`
}

// BatchStatus reports batch embedding state.
type BatchStatus struct {
	Enabled        bool   `json:"enabled"`
	Failures       int    `json:"failures"`
	Limit          int    `json:"limit"`
	Wait           bool   `json:"wait"`
	Concurrency    int    `json:"concurrency"`
	PollIntervalMs int    `json:"pollIntervalMs"`
	TimeoutMs      int    `json:"timeoutMs"`
	LastError      string `json:"lastError,omitempty"`
	LastProvider   string `json:"lastProvider,omitempty"`
}

// SyncOptions controls how a sync operation runs.
type SyncOptions struct {
	Reason   string
	Force    bool
	Progress func(update MemorySyncProgressUpdate)
}

// SearchOptions controls a search invocation.
type SearchOptions struct {
	MaxResults int
	MinScore   float64
	SessionKey string
}

// ReadFileParams specifies which region of a file to read.
type ReadFileParams struct {
	RelPath string
	From    int
	Lines   int
}

// ReadFileResult contains the content returned by ReadFile.
type ReadFileResult struct {
	Text string `json:"text"`
	Path string `json:"path"`
}

// MemorySearchManager is the primary interface for memory operations.
type MemorySearchManager interface {
	Search(ctx context.Context, query string, opts *SearchOptions) ([]MemorySearchResult, error)
	ReadFile(ctx context.Context, params ReadFileParams) (*ReadFileResult, error)
	Status() MemoryProviderStatus
	Sync(ctx context.Context, opts *SyncOptions) error
	ProbeEmbeddingAvailability(ctx context.Context) (*MemoryEmbeddingProbeResult, error)
	ProbeVectorAvailability(ctx context.Context) (bool, error)
	Close() error
}
