package memory

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// Manager is the main memory indexing and search manager.
// It manages the SQLite database, embedding provider, and file index.
// The TS version (manager.ts) is 2411 lines; this Go port splits into
// manager.go (core) + search.go + config.go + sync.go.
type Manager struct {
	mu sync.RWMutex

	agentID      string
	workspaceDir string
	sessionsDir  string // 会话文件目录（用于 session 索引）
	dbPath       string
	db           *sql.DB
	provider     *EmbeddingProvider
	config       *ResolvedMemoryBackendConfig

	ftsAvailable bool
	ftsTable     string
	vectorTable  string
	cacheTable   string

	watcher *FileWatcher
	logger  *slog.Logger
	closed  bool
}

// ManagerConfig holds the parameters for creating a new Manager.
type ManagerConfig struct {
	AgentID      string
	WorkspaceDir string
	SessionsDir  string // optional; session files directory for indexing
	DBPath       string // optional; defaults to workspace/.memory/index.db
	Config       *ResolvedMemoryBackendConfig
	Logger       *slog.Logger
}

// NewManager creates a new memory Manager, initialising the SQLite database
// and schema.
func NewManager(ctx context.Context, cfg ManagerConfig) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	dbPath := cfg.DBPath
	if dbPath == "" {
		dbDir := filepath.Join(cfg.WorkspaceDir, ".memory")
		EnsureDir(dbDir)
		dbPath = filepath.Join(dbDir, "index.db")
	} else {
		EnsureDir(filepath.Dir(dbPath))
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("memory: open db: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		cfg.Logger.Warn("memory: failed to set WAL mode", "err", err)
	}

	ftsTable := "chunks_fts"
	cacheTable := "embedding_cache"
	vectorTable := "chunks_vec"

	ftsEnabled := true // always attempt FTS5
	schemaResult, err := EnsureMemoryIndexSchema(SchemaParams{
		DB:                  db,
		EmbeddingCacheTable: cacheTable,
		FTSTable:            ftsTable,
		FTSEnabled:          ftsEnabled,
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("memory: schema: %w", err)
	}

	m := &Manager{
		agentID:      cfg.AgentID,
		workspaceDir: cfg.WorkspaceDir,
		sessionsDir:  cfg.SessionsDir,
		dbPath:       dbPath,
		db:           db,
		config:       cfg.Config,
		ftsAvailable: schemaResult.FTSAvailable,
		ftsTable:     ftsTable,
		vectorTable:  vectorTable,
		cacheTable:   cacheTable,
		logger:       cfg.Logger,
	}

	return m, nil
}

// Search performs a hybrid vector+keyword search.
// TS 对照: manager.ts → search() (hybrid vector+keyword merge)
func (m *Manager) Search(ctx context.Context, query string, opts *SearchOptions) ([]MemorySearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, fmt.Errorf("memory: manager closed")
	}

	if opts == nil {
		opts = &SearchOptions{MaxResults: 6}
	}
	if opts.MaxResults <= 0 {
		opts.MaxResults = 6
	}

	m.logger.Debug("memory search", "query", query, "maxResults", opts.MaxResults)
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}

	snippetMax := 700
	limit := opts.MaxResults * 3 // over-fetch for merge

	// ---------- 1. 向量搜索（可选 — 需要 provider） ----------
	var vectorResults []HybridVectorResult
	if m.provider != nil && m.provider.EmbedQuery != nil {
		queryVec, err := m.provider.EmbedQuery(ctx, query)
		if err != nil {
			m.logger.Warn("memory: embed query failed, falling back to keyword-only", "err", err)
		} else if len(queryVec) > 0 {
			providerModel := m.provider.ID + ":" + m.provider.Model
			ensureVec := func(_ context.Context, dims int) (bool, error) {
				return EnsureVecTable(m.db, m.vectorTable, dims, m.logger)
			}
			svr, err := SearchVector(ctx, SearchVectorParams{
				DB:                m.db,
				VectorTable:       m.vectorTable,
				ProviderModel:     providerModel,
				QueryVec:          queryVec,
				Limit:             limit,
				SnippetMaxChars:   snippetMax,
				EnsureVectorReady: ensureVec,
			})
			if err != nil {
				m.logger.Warn("memory: vector search failed", "err", err)
			}
			for _, r := range svr {
				vectorResults = append(vectorResults, HybridVectorResult{
					ID: r.ID, Path: r.Path,
					StartLine: r.StartLine, EndLine: r.EndLine,
					Source: r.Source, Snippet: r.Snippet,
					VectorScore: r.Score,
				})
			}
		}
	}

	// ---------- 2. 关键词搜索（FTS5） ----------
	var keywordResults []HybridKeywordResult
	if m.ftsAvailable {
		providerModel := ""
		if m.provider != nil {
			providerModel = m.provider.ID + ":" + m.provider.Model
		}
		skr, err := SearchKeyword(ctx, SearchKeywordParams{
			DB:              m.db,
			FTSTable:        m.ftsTable,
			ProviderModel:   providerModel,
			Query:           query,
			Limit:           limit,
			SnippetMaxChars: snippetMax,
		})
		if err != nil {
			m.logger.Warn("memory: keyword search failed", "err", err)
		}
		for _, r := range skr {
			keywordResults = append(keywordResults, HybridKeywordResult{
				ID: r.ID, Path: r.Path,
				StartLine: r.StartLine, EndLine: r.EndLine,
				Source: r.Source, Snippet: r.Snippet,
				TextScore: r.TextScore,
			})
		}
	}

	// ---------- 3. 合并 ----------
	merged := MergeHybridResults(vectorResults, keywordResults, 0.7, 0.3)

	// ---------- 4. 过滤 + 截断 ----------
	var results []MemorySearchResult
	for _, mr := range merged {
		if opts.MinScore > 0 && mr.Score < opts.MinScore {
			continue
		}
		results = append(results, MemorySearchResult{
			Path:      mr.Path,
			StartLine: mr.StartLine,
			EndLine:   mr.EndLine,
			Score:     mr.Score,
			Snippet:   mr.Snippet,
			Source:    MemorySource(mr.Source),
		})
		if len(results) >= opts.MaxResults {
			break
		}
	}

	return results, nil
}

// ReadFile reads content from a file in the workspace directory.
func (m *Manager) ReadFile(_ context.Context, params ReadFileParams) (*ReadFileResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.closed {
		return nil, fmt.Errorf("memory: manager closed")
	}

	relPath := NormalizeRelPath(params.RelPath)
	absPath := filepath.Join(m.workspaceDir, relPath)

	// Security: ensure the path is within the workspace.
	abs, err := filepath.Abs(absPath)
	if err != nil {
		return nil, fmt.Errorf("memory: invalid path: %w", err)
	}
	wsAbs, _ := filepath.Abs(m.workspaceDir)
	if !strings.HasPrefix(abs, wsAbs) {
		return nil, fmt.Errorf("memory: path outside workspace")
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("memory: read file: %w", err)
	}

	text := string(data)
	if params.From > 0 || params.Lines > 0 {
		lines := strings.Split(text, "\n")
		start := params.From
		if start < 0 {
			start = 0
		}
		if start >= len(lines) {
			text = ""
		} else {
			end := len(lines)
			if params.Lines > 0 && start+params.Lines < end {
				end = start + params.Lines
			}
			text = strings.Join(lines[start:end], "\n")
		}
	}

	return &ReadFileResult{Text: text, Path: relPath}, nil
}

// Status returns the current status of the memory manager.
func (m *Manager) Status() MemoryProviderStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := MemoryProviderStatus{
		Backend:      string(BackendBuiltin),
		WorkspaceDir: m.workspaceDir,
		DBPath:       m.dbPath,
	}

	if m.provider != nil {
		status.Provider = m.provider.ID
		status.Model = m.provider.Model
	}

	fts := &FTSStatus{
		Enabled:   true,
		Available: m.ftsAvailable,
	}
	status.FTS = fts

	return status
}

// Sync triggers a re-index of memory files.
// TS 对照: manager.ts → syncMemory() / syncIndex()
func (m *Manager) Sync(ctx context.Context, opts *SyncOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return fmt.Errorf("memory: manager closed")
	}
	if opts == nil {
		opts = &SyncOptions{Reason: "manual"}
	}

	m.logger.Info("memory sync start", "reason", opts.Reason, "force", opts.Force)

	// ---------- 1. 扫描内存文件 ----------
	files, err := ListMemoryFiles(m.workspaceDir, nil)
	if err != nil {
		return fmt.Errorf("memory: list files: %w", err)
	}
	if len(files) == 0 {
		m.logger.Debug("memory sync: no memory files found")
		return nil
	}

	// ---------- 2. 构建文件条目 + hash ----------
	var entries []*MemoryFileEntry
	for _, absPath := range files {
		entry, err := BuildFileEntry(absPath, m.workspaceDir)
		if err != nil {
			m.logger.Warn("memory sync: skip file", "path", absPath, "err", err)
			continue
		}
		entries = append(entries, entry)
	}

	reportProgress := func(completed, total int, label string) {
		if opts.Progress != nil {
			opts.Progress(MemorySyncProgressUpdate{
				Completed: completed, Total: total, Label: label,
			})
		}
	}

	// ---------- 3. 比较 DB 状态，筛选 dirty 文件 ----------
	var dirty []*MemoryFileEntry
	for _, entry := range entries {
		if opts.Force {
			dirty = append(dirty, entry)
			continue
		}
		// 查询 DB 中的 hash
		var storedHash string
		row := m.db.QueryRow("SELECT hash FROM files WHERE path = ?", entry.Path)
		if err := row.Scan(&storedHash); err != nil || storedHash != entry.Hash {
			dirty = append(dirty, entry)
		}
	}

	if len(dirty) == 0 {
		m.logger.Info("memory sync: all files up to date")
		return nil
	}

	m.logger.Info("memory sync: processing dirty files", "count", len(dirty), "total", len(entries))
	total := len(dirty)
	providerModel := ""
	if m.provider != nil {
		providerModel = m.provider.ID + ":" + m.provider.Model
	}

	chunkCfg := ChunkingConfig{Tokens: 512, Overlap: 50}

	for i, entry := range dirty {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		reportProgress(i, total, entry.Path)

		// ---------- 4. 读取 + 分块 ----------
		content, err := os.ReadFile(entry.AbsPath)
		if err != nil {
			m.logger.Warn("memory sync: read failed", "path", entry.AbsPath, "err", err)
			continue
		}
		chunks := ChunkMarkdown(string(content), chunkCfg)
		if len(chunks) == 0 {
			continue
		}

		// ---------- 5. 生成 embeddings（可选） ----------
		var embeddings [][]float64
		if m.provider != nil && m.provider.EmbedBatch != nil {
			texts := make([]string, len(chunks))
			for j, c := range chunks {
				texts[j] = c.Text
			}
			emb, err := m.provider.EmbedBatch(ctx, texts)
			if err != nil {
				m.logger.Warn("memory sync: embed batch failed", "path", entry.Path, "err", err)
			} else {
				embeddings = emb
			}
		}

		// ---------- 6. 事务写入 ----------
		tx, err := m.db.BeginTx(ctx, nil)
		if err != nil {
			m.logger.Warn("memory sync: tx begin failed", "err", err)
			continue
		}

		// 删除旧 chunks
		if _, err := tx.Exec("DELETE FROM chunks WHERE path = ?", entry.Path); err != nil {
			m.logger.Warn("memory sync: delete chunks failed", "path", entry.Path, "err", err)
		}
		if m.ftsAvailable {
			if _, err := tx.Exec(fmt.Sprintf("DELETE FROM %s WHERE path = ?", m.ftsTable), entry.Path); err != nil {
				m.logger.Warn("memory sync: fts delete failed", "path", entry.Path, "err", err)
			}
		}

		// 插入新 chunks
		for j, chunk := range chunks {
			chunkID := entry.Path + ":" + fmt.Sprintf("%d-%d", chunk.StartLine, chunk.EndLine)
			embJSON := "[]"
			if j < len(embeddings) && len(embeddings[j]) > 0 {
				embJSON = embeddingToJSON(embeddings[j])
			}
			model := providerModel
			if model == "" {
				model = "none"
			}

			_, err := tx.Exec(
				`INSERT OR REPLACE INTO chunks (id, path, source, start_line, end_line, hash, model, text, embedding, updated_at)
				 VALUES (?, ?, 'memory', ?, ?, ?, ?, ?, ?, ?)`,
				chunkID, entry.Path, chunk.StartLine, chunk.EndLine,
				chunk.Hash, model, chunk.Text, embJSON, entry.MtimeMs,
			)
			if err != nil {
				m.logger.Warn("memory sync: insert chunk failed", "id", chunkID, "err", err)
				continue
			}

			// FTS5 索引
			if m.ftsAvailable {
				if _, err := tx.Exec(
					fmt.Sprintf(
						"INSERT INTO %s (text, id, path, source, model, start_line, end_line) VALUES (?, ?, ?, 'memory', ?, ?, ?)",
						m.ftsTable,
					),
					chunk.Text, chunkID, entry.Path, model, chunk.StartLine, chunk.EndLine,
				); err != nil {
					m.logger.Warn("memory sync: fts insert failed", "id", chunkID, "err", err)
				}
			}
		}

		// Upsert files 表
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO files (path, source, hash, mtime, size) VALUES (?, 'memory', ?, ?, ?)`,
			entry.Path, entry.Hash, entry.MtimeMs, entry.Size,
		); err != nil {
			m.logger.Warn("memory sync: upsert files failed", "path", entry.Path, "err", err)
		}

		if err := tx.Commit(); err != nil {
			m.logger.Warn("memory sync: commit failed", "path", entry.Path, "err", err)
			continue
		}
	}

	reportProgress(total, total, "done")
	m.logger.Info("memory sync complete", "processed", len(dirty))

	// ---------- Session 文件同步 ----------
	// 对应 TS: sync-session-files.ts → syncSessionFiles()
	if m.sessionsDir != "" {
		if err := m.syncSessionFiles(ctx, opts); err != nil {
			m.logger.Warn("memory sync: session files failed", "err", err)
		}
	}

	return nil
}

// ProbeEmbeddingAvailability checks if the embedding provider is usable.
func (m *Manager) ProbeEmbeddingAvailability(_ context.Context) (*MemoryEmbeddingProbeResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.provider == nil {
		return &MemoryEmbeddingProbeResult{OK: false, Error: "no provider configured"}, nil
	}
	return &MemoryEmbeddingProbeResult{OK: true}, nil
}

// ProbeVectorAvailability checks if the sqlite-vec extension is usable.
func (m *Manager) ProbeVectorAvailability(_ context.Context) (bool, error) {
	if m.db == nil {
		return false, nil
	}
	ok, path, err := LoadSqliteVecExtension(m.db, "", m.logger)
	if err != nil {
		m.logger.Debug("sqlite-vec probe failed", "err", err)
		return false, nil
	}
	if ok {
		m.logger.Info("sqlite-vec available", "path", path)
	}
	return ok, nil
}

// StartWatch begins watching workspace memory paths for changes.
// On detecting changes, it calls Sync after a debounce period.
func (m *Manager) StartWatch(ctx context.Context, debounceMs int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.watcher != nil || m.closed {
		return nil
	}

	watchPaths := []string{
		filepath.Join(m.workspaceDir, "MEMORY.md"),
		filepath.Join(m.workspaceDir, "memory.md"),
		filepath.Join(m.workspaceDir, "memory"),
	}
	// 追加 session 文件目录到监听列表
	// 对应 TS: sync-session-files.ts 中 chokidar 监听
	if m.sessionsDir != "" {
		watchPaths = append(watchPaths, m.sessionsDir)
	}

	w, err := NewFileWatcher(watchPaths, debounceMs, func() {
		if syncErr := m.Sync(ctx, &SyncOptions{Reason: "watch"}); syncErr != nil {
			m.logger.Warn("memory sync failed (watch)", "err", syncErr)
		}
	}, m.logger)
	if err != nil {
		return fmt.Errorf("memory: start watcher: %w", err)
	}

	m.watcher = w
	w.Start(ctx)
	return nil
}

// StopWatch stops the file watcher if running.
func (m *Manager) StopWatch() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.watcher != nil {
		m.watcher.Stop()
		m.watcher = nil
	}
}

// Close shuts down the manager and closes the database.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	if m.watcher != nil {
		m.watcher.Stop()
		m.watcher = nil
	}
	if m.db != nil {
		return m.db.Close()
	}
	return nil
}
