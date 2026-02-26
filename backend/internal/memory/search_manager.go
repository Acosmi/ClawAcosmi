package memory

import (
	"context"
	"fmt"
	"log/slog"
)

// SearchManager is the top-level entry point that delegates to either
// the builtin Manager or the QMDManager based on configuration.
type SearchManager struct {
	backend MemoryBackend
	builtin *Manager
	qmd     *QMDManager
	logger  *slog.Logger
}

// SearchManagerConfig holds configuration for creating a SearchManager.
type SearchManagerConfig struct {
	BackendConfig *ResolvedMemoryBackendConfig
	AgentID       string
	WorkspaceDir  string
	DBPath        string
	Logger        *slog.Logger
}

// NewSearchManager creates a SearchManager, initialising the appropriate backend.
func NewSearchManager(ctx context.Context, cfg SearchManagerConfig) (*SearchManager, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	backend := BackendBuiltin
	if cfg.BackendConfig != nil {
		backend = cfg.BackendConfig.Backend
	}

	sm := &SearchManager{
		backend: backend,
		logger:  cfg.Logger,
	}

	switch backend {
	case BackendQMD:
		if cfg.BackendConfig.QMD == nil {
			return nil, fmt.Errorf("memory: qmd backend requires qmd config")
		}
		sm.qmd = NewQMDManager(QMDManagerConfig{
			Config: cfg.BackendConfig.QMD,
			Logger: cfg.Logger,
		})
	default:
		mgr, err := NewManager(ctx, ManagerConfig{
			AgentID:      cfg.AgentID,
			WorkspaceDir: cfg.WorkspaceDir,
			DBPath:       cfg.DBPath,
			Config:       cfg.BackendConfig,
			Logger:       cfg.Logger,
		})
		if err != nil {
			return nil, err
		}
		sm.builtin = mgr
	}

	return sm, nil
}

// Search delegates to the active backend.
func (sm *SearchManager) Search(ctx context.Context, query string, opts *SearchOptions) ([]MemorySearchResult, error) {
	switch sm.backend {
	case BackendQMD:
		return sm.qmd.Search(ctx, query, opts)
	default:
		return sm.builtin.Search(ctx, query, opts)
	}
}

// ReadFile delegates to the builtin backend (QMD doesn't support file reads).
func (sm *SearchManager) ReadFile(ctx context.Context, params ReadFileParams) (*ReadFileResult, error) {
	if sm.builtin != nil {
		return sm.builtin.ReadFile(ctx, params)
	}
	return nil, fmt.Errorf("memory: ReadFile not supported for %s backend", sm.backend)
}

// Status delegates to the active backend.
func (sm *SearchManager) Status() MemoryProviderStatus {
	switch sm.backend {
	case BackendQMD:
		return sm.qmd.Status()
	default:
		return sm.builtin.Status()
	}
}

// Sync delegates to the active backend.
func (sm *SearchManager) Sync(ctx context.Context, opts *SyncOptions) error {
	switch sm.backend {
	case BackendQMD:
		return sm.qmd.Sync(ctx, opts)
	default:
		return sm.builtin.Sync(ctx, opts)
	}
}

// ProbeEmbeddingAvailability delegates to the builtin backend.
func (sm *SearchManager) ProbeEmbeddingAvailability(ctx context.Context) (*MemoryEmbeddingProbeResult, error) {
	if sm.builtin != nil {
		return sm.builtin.ProbeEmbeddingAvailability(ctx)
	}
	return &MemoryEmbeddingProbeResult{OK: true}, nil
}

// ProbeVectorAvailability delegates to the builtin backend.
func (sm *SearchManager) ProbeVectorAvailability(ctx context.Context) (bool, error) {
	if sm.builtin != nil {
		return sm.builtin.ProbeVectorAvailability(ctx)
	}
	return false, nil
}

// Close shuts down the active backend.
func (sm *SearchManager) Close() error {
	switch sm.backend {
	case BackendQMD:
		return sm.qmd.Close()
	default:
		if sm.builtin != nil {
			return sm.builtin.Close()
		}
		return nil
	}
}
