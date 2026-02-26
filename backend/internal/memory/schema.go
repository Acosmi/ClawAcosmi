package memory

import (
	"database/sql"
	"fmt"
	"log/slog"
)

// SchemaParams controls which tables and features to set up.
type SchemaParams struct {
	DB                  *sql.DB
	EmbeddingCacheTable string
	FTSTable            string
	FTSEnabled          bool
}

// SchemaResult reports the outcome of schema initialisation.
type SchemaResult struct {
	FTSAvailable bool
	FTSError     string
}

// EnsureMemoryIndexSchema creates the required tables (meta, files, chunks,
// embedding_cache, and optionally FTS5) if they don't already exist.
func EnsureMemoryIndexSchema(p SchemaParams) (*SchemaResult, error) {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS files (
			path TEXT PRIMARY KEY,
			source TEXT NOT NULL DEFAULT 'memory',
			hash TEXT NOT NULL,
			mtime INTEGER NOT NULL,
			size INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			path TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'memory',
			start_line INTEGER NOT NULL,
			end_line INTEGER NOT NULL,
			hash TEXT NOT NULL,
			model TEXT NOT NULL,
			text TEXT NOT NULL,
			embedding TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			provider_key TEXT NOT NULL,
			hash TEXT NOT NULL,
			embedding TEXT NOT NULL,
			dims INTEGER,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (provider, model, provider_key, hash)
		)`, p.EmbeddingCacheTable),
		fmt.Sprintf(
			`CREATE INDEX IF NOT EXISTS idx_embedding_cache_updated_at ON %s(updated_at)`,
			p.EmbeddingCacheTable,
		),
	}

	for _, stmt := range stmts {
		if _, err := p.DB.Exec(stmt); err != nil {
			return nil, fmt.Errorf("memory schema: %w", err)
		}
	}

	result := &SchemaResult{}
	if p.FTSEnabled {
		ftsSQL := fmt.Sprintf(
			`CREATE VIRTUAL TABLE IF NOT EXISTS %s USING fts5(
				text,
				id UNINDEXED,
				path UNINDEXED,
				source UNINDEXED,
				model UNINDEXED,
				start_line UNINDEXED,
				end_line UNINDEXED
			)`, p.FTSTable,
		)
		if _, err := p.DB.Exec(ftsSQL); err != nil {
			result.FTSError = err.Error()
		} else {
			result.FTSAvailable = true
		}
	}

	// Ensure columns added in later schema migrations exist.
	ensureColumn(p.DB, "files", "source", "TEXT NOT NULL DEFAULT 'memory'")
	ensureColumn(p.DB, "chunks", "source", "TEXT NOT NULL DEFAULT 'memory'")

	idxStmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_chunks_path ON chunks(path)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_source ON chunks(source)`,
	}
	for _, stmt := range idxStmts {
		if _, err := p.DB.Exec(stmt); err != nil {
			return nil, fmt.Errorf("memory schema index: %w", err)
		}
	}

	return result, nil
}

// ensureColumn adds a column to a table if it doesn't already exist.
func ensureColumn(db *sql.DB, table, column, definition string) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid       int
			name      string
			colType   string
			notNull   int
			dfltValue sql.NullString
			pk        int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == column {
			return // column already exists
		}
	}

	if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		slog.Warn("memory schema: add column failed", "table", table, "column", column, "err", err)
	}
}
