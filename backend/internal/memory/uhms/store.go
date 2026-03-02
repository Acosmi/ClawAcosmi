package uhms

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
)

// Store manages the local SQLite metadata database for UHMS.
// Content is stored in VFS (file system); SQLite holds only metadata + FTS5 index.
type Store struct {
	db     *sql.DB
	dbPath string
}

// NewStore creates and initializes the UHMS SQLite metadata store.
func NewStore(dbPath string) (*Store, error) {
	dbPath = resolveDBPath(dbPath)

	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("uhms/store: create dir %s: %w", dir, err)
	}

	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("uhms/store: open sqlite %s: %w", dbPath, err)
	}

	// 设置连接池 (SQLite 单写, 适度读并发)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("uhms/store: ping sqlite: %w", err)
	}

	if err := ensureSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("uhms/store: schema: %w", err)
	}

	slog.Info("uhms/store: initialized", "path", dbPath)
	return &Store{db: db, dbPath: dbPath}, nil
}

// DB returns the underlying *sql.DB.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database connection.
func (s *Store) Close() error { return s.db.Close() }

// ============================================================================
// Schema
// ============================================================================

func ensureSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS memories (
			id              TEXT PRIMARY KEY,
			user_id         TEXT NOT NULL,
			content         TEXT NOT NULL DEFAULT '',
			memory_type     TEXT NOT NULL DEFAULT 'episodic',
			category        TEXT NOT NULL DEFAULT 'fact',
			importance_score REAL NOT NULL DEFAULT 0.5,
			decay_factor    REAL NOT NULL DEFAULT 1.0,
			retention_policy TEXT NOT NULL DEFAULT 'standard',
			access_count    INTEGER NOT NULL DEFAULT 0,
			last_accessed_at TEXT,
			archived_at     TEXT,
			event_time      TEXT,
			ingested_at     TEXT NOT NULL DEFAULT (datetime('now')),
			vfs_path        TEXT DEFAULT '',
			embedding_ref   TEXT DEFAULT '',
			metadata        TEXT DEFAULT '',
			created_at      TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at      TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mem_user ON memories(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mem_type ON memories(memory_type)`,
		`CREATE INDEX IF NOT EXISTS idx_mem_cat ON memories(category)`,
		`CREATE INDEX IF NOT EXISTS idx_mem_event ON memories(event_time)`,
		`CREATE INDEX IF NOT EXISTS idx_mem_ingest ON memories(ingested_at)`,

		`CREATE TABLE IF NOT EXISTS entities (
			id          TEXT PRIMARY KEY,
			user_id     TEXT NOT NULL,
			name        TEXT NOT NULL,
			entity_type TEXT DEFAULT '',
			description TEXT DEFAULT '',
			created_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ent_user ON entities(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_ent_name ON entities(name)`,

		`CREATE TABLE IF NOT EXISTS relations (
			id            TEXT PRIMARY KEY,
			user_id       TEXT NOT NULL,
			source_id     TEXT NOT NULL,
			target_id     TEXT NOT NULL,
			relation_type TEXT NOT NULL,
			weight        REAL NOT NULL DEFAULT 1.0,
			created_at    TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rel_user ON relations(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_rel_src ON relations(source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_rel_tgt ON relations(target_id)`,

		`CREATE TABLE IF NOT EXISTS core_memory (
			id           TEXT PRIMARY KEY,
			user_id      TEXT NOT NULL UNIQUE,
			persona      TEXT DEFAULT '',
			preferences  TEXT DEFAULT '',
			instructions TEXT DEFAULT '',
			updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS tree_nodes (
			id             TEXT PRIMARY KEY,
			user_id        TEXT NOT NULL,
			parent_id      TEXT DEFAULT '',
			content        TEXT NOT NULL,
			node_type      TEXT NOT NULL DEFAULT 'leaf',
			category       TEXT DEFAULT '',
			depth          INTEGER NOT NULL DEFAULT 0,
			children_count INTEGER NOT NULL DEFAULT 0,
			is_leaf        INTEGER NOT NULL DEFAULT 1,
			created_at     TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_tn_user ON tree_nodes(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_tn_parent ON tree_nodes(parent_id)`,

		`CREATE TABLE IF NOT EXISTS decay_profiles (
			id          TEXT PRIMARY KEY,
			user_id     TEXT NOT NULL,
			memory_type TEXT NOT NULL,
			half_life   REAL NOT NULL DEFAULT 168,
			min_decay   REAL NOT NULL DEFAULT 0.01,
			updated_at  TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(user_id, memory_type)
		)`,
	}

	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec schema: %w\nSQL: %s", err, stmt)
		}
	}

	// FTS5 虚拟表 (非致命)
	_, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
		content,
		content='memories',
		content_rowid='rowid'
	)`)
	if err != nil {
		slog.Warn("uhms/store: FTS5 creation failed (non-fatal)", "error", err)
	}

	return nil
}

// ============================================================================
// Memory CRUD
// ============================================================================

// CreateMemory inserts a new memory record.
func (s *Store) CreateMemory(m *Memory) error {
	now := time.Now().UTC().Format(time.RFC3339)
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	if m.IngestedAt.IsZero() {
		m.IngestedAt = time.Now().UTC()
	}

	_, err := s.db.Exec(`INSERT INTO memories
		(id, user_id, content, memory_type, category, importance_score, decay_factor,
		 retention_policy, access_count, event_time, ingested_at, vfs_path,
		 embedding_ref, metadata, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.UserID, m.Content, m.MemoryType, m.Category,
		m.ImportanceScore, m.DecayFactor, m.RetentionPolicy, m.AccessCount,
		formatTimePtr(m.EventTime), m.IngestedAt.Format(time.RFC3339),
		m.VFSPath, m.EmbeddingRef, m.Metadata,
		m.CreatedAt.Format(time.RFC3339), now,
	)
	if err != nil {
		return fmt.Errorf("uhms/store: create memory: %w", err)
	}

	// FTS5 sync
	s.db.Exec(`INSERT INTO memories_fts(rowid, content)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), ?)`, m.ID, m.Content)
	return nil
}

// GetMemory retrieves a memory by ID.
func (s *Store) GetMemory(id string) (*Memory, error) {
	row := s.db.QueryRow(`SELECT id, user_id, content, memory_type, category,
		importance_score, decay_factor, retention_policy, access_count,
		last_accessed_at, archived_at, event_time, ingested_at, vfs_path,
		embedding_ref, metadata, created_at, updated_at
		FROM memories WHERE id = ?`, id)
	m, err := scanMemory(row)
	if err != nil {
		return nil, fmt.Errorf("uhms/store: get memory %s: %w", id, err)
	}
	return m, nil
}

// UpdateMemory updates an existing memory record.
func (s *Store) UpdateMemory(m *Memory) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE memories SET
		content=?, memory_type=?, category=?, importance_score=?, decay_factor=?,
		retention_policy=?, access_count=?, last_accessed_at=?, archived_at=?,
		event_time=?, vfs_path=?, embedding_ref=?, metadata=?, updated_at=?
		WHERE id=?`,
		m.Content, m.MemoryType, m.Category, m.ImportanceScore, m.DecayFactor,
		m.RetentionPolicy, m.AccessCount, formatTimePtr(m.LastAccessedAt),
		formatTimePtr(m.ArchivedAt), formatTimePtr(m.EventTime),
		m.VFSPath, m.EmbeddingRef, m.Metadata, now, m.ID,
	)
	if err != nil {
		return fmt.Errorf("uhms/store: update memory %s: %w", m.ID, err)
	}

	// FTS5 sync (delete + re-insert)
	s.db.Exec(`INSERT INTO memories_fts(memories_fts, rowid, content) VALUES('delete',
		(SELECT rowid FROM memories WHERE id = ?), ?)`, m.ID, m.Content)
	s.db.Exec(`INSERT INTO memories_fts(rowid, content)
		VALUES ((SELECT rowid FROM memories WHERE id = ?), ?)`, m.ID, m.Content)
	return nil
}

// DeleteMemory removes a memory record by ID.
func (s *Store) DeleteMemory(id string) error {
	// FTS5 delete first (needs content before row is gone)
	s.db.Exec(`INSERT INTO memories_fts(memories_fts, rowid, content) VALUES('delete',
		(SELECT rowid FROM memories WHERE id = ?),
		(SELECT content FROM memories WHERE id = ?))`, id, id)

	_, err := s.db.Exec(`DELETE FROM memories WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("uhms/store: delete memory %s: %w", id, err)
	}
	return nil
}

// ListMemories returns memories for a user with optional filters.
func (s *Store) ListMemories(userID string, opts ListOptions) ([]Memory, error) {
	var clauses []string
	var args []interface{}

	if userID != "" {
		clauses = append(clauses, "user_id = ?")
		args = append(args, userID)
	}

	if opts.MemoryType != "" {
		clauses = append(clauses, "memory_type = ?")
		args = append(args, opts.MemoryType)
	}
	if opts.Category != "" {
		clauses = append(clauses, "category = ?")
		args = append(args, opts.Category)
	}
	if opts.MinImportance > 0 {
		clauses = append(clauses, "importance_score >= ?")
		args = append(args, opts.MinImportance)
	}
	if opts.MinDecayFactor > 0 {
		clauses = append(clauses, "decay_factor >= ?")
		args = append(args, opts.MinDecayFactor)
	}
	if !opts.IncludeArchived {
		clauses = append(clauses, "archived_at IS NULL")
	}

	whereClause := ""
	if len(clauses) > 0 {
		whereClause = " WHERE " + strings.Join(clauses, " AND ")
	}

	query := `SELECT id, user_id, content, memory_type, category,
		importance_score, decay_factor, retention_policy, access_count,
		last_accessed_at, archived_at, event_time, ingested_at, vfs_path,
		embedding_ref, metadata, created_at, updated_at
		FROM memories` + whereClause + ` ORDER BY created_at DESC`

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("uhms/store: list memories: %w", err)
	}
	defer rows.Close()

	return scanMemories(rows)
}

// escapeFTS5Query converts a raw query into a safe FTS5 MATCH expression.
// Each whitespace-delimited token is wrapped in double quotes; internal
// double-quotes are escaped as "". This prevents syntax errors when the
// query contains CJK characters, slashes, parentheses, or other FTS5
// operators.
func escapeFTS5Query(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	tokens := strings.Fields(raw)
	escaped := make([]string, 0, len(tokens))
	for _, t := range tokens {
		t = strings.ReplaceAll(t, `"`, `""`)
		escaped = append(escaped, `"`+t+`"`)
	}
	return strings.Join(escaped, " ")
}

// SearchByFTS5 performs full-text search using SQLite FTS5.
func (s *Store) SearchByFTS5(userID, query string, limit int) ([]SearchResult, error) {
	query = escapeFTS5Query(query)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.Query(`
		SELECT m.id, m.user_id, m.content, m.memory_type, m.category,
			m.importance_score, m.decay_factor, m.retention_policy, m.access_count,
			m.last_accessed_at, m.archived_at, m.event_time, m.ingested_at, m.vfs_path,
			m.embedding_ref, m.metadata, m.created_at, m.updated_at
		FROM memories m
		JOIN memories_fts fts ON m.rowid = fts.rowid
		WHERE memories_fts MATCH ? AND m.user_id = ?
		ORDER BY rank
		LIMIT ?
	`, query, userID, limit)

	if err != nil {
		slog.Debug("uhms/store: FTS5 search failed, falling back to LIKE", "error", err)
		return s.searchByLike(userID, query, limit)
	}
	defer rows.Close()

	memories, err := scanMemories(rows)
	if err != nil {
		return s.searchByLike(userID, query, limit)
	}

	results := make([]SearchResult, len(memories))
	for i, m := range memories {
		results[i] = SearchResult{Memory: m, Score: 1.0 - float64(i)*0.05, Source: "fts5"}
	}
	return results, nil
}

// IncrementAccess updates access count and last accessed time.
func (s *Store) IncrementAccess(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`UPDATE memories SET access_count = access_count + 1,
		last_accessed_at = ? WHERE id = ?`, now, id)
	return err
}

// CountMemories returns total memory count for a user.
func (s *Store) CountMemories(userID string) (int64, error) {
	var count int64
	var err error
	if userID != "" {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM memories WHERE user_id = ? AND archived_at IS NULL`, userID).Scan(&count)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM memories WHERE archived_at IS NULL`).Scan(&count)
	}
	return count, err
}

// AggregateStats holds aggregated statistics for the memory dashboard.
type AggregateStats struct {
	ByType      map[string]int64 `json:"byType"`
	ByCategory  map[string]int64 `json:"byCategory"`
	ByRetention map[string]int64 `json:"byRetention"`
	DecayHealth struct {
		Healthy   int64 `json:"healthy"`
		Fading    int64 `json:"fading"`
		Critical  int64 `json:"critical"`
		Permanent int64 `json:"permanent"`
	} `json:"decayHealth"`
	TotalAccess   int64   `json:"totalAccess"`
	AvgImportance float64 `json:"avgImportance"`
	OldestAt      int64   `json:"oldestAt"`
	NewestAt      int64   `json:"newestAt"`
}

// AggregateStats computes aggregated statistics for a user's memories.
// When userID is empty, statistics cover all users.
func (s *Store) AggregateStats(userID string) (*AggregateStats, error) {
	stats := &AggregateStats{
		ByType:      make(map[string]int64),
		ByCategory:  make(map[string]int64),
		ByRetention: make(map[string]int64),
	}

	// 构建 user_id 过滤条件
	userFilter := "archived_at IS NULL"
	var userArgs []interface{}
	if userID != "" {
		userFilter = "user_id = ? AND archived_at IS NULL"
		userArgs = []interface{}{userID}
	}

	// GROUP BY memory_type
	rows, err := s.db.Query(`SELECT memory_type, COUNT(*) FROM memories
		WHERE `+userFilter+` GROUP BY memory_type`, userArgs...)
	if err != nil {
		return nil, fmt.Errorf("uhms/store: aggregate by type: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var c int64
		if err := rows.Scan(&k, &c); err != nil {
			return nil, err
		}
		stats.ByType[k] = c
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("uhms/store: aggregate by type iter: %w", err)
	}

	// GROUP BY category
	catRows, err := s.db.Query(`SELECT category, COUNT(*) FROM memories
		WHERE `+userFilter+` GROUP BY category`, userArgs...)
	if err != nil {
		return nil, fmt.Errorf("uhms/store: aggregate by category: %w", err)
	}
	defer catRows.Close()
	for catRows.Next() {
		var k string
		var c int64
		if err := catRows.Scan(&k, &c); err != nil {
			return nil, err
		}
		stats.ByCategory[k] = c
	}
	if err := catRows.Err(); err != nil {
		return nil, fmt.Errorf("uhms/store: aggregate by category iter: %w", err)
	}

	// Aggregate: retention, decay health, totals, timestamps
	var retPermanent, retStandard, retOther int64
	err = s.db.QueryRow(`SELECT
		COALESCE(SUM(CASE WHEN retention_policy = 'permanent' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN retention_policy = 'standard' THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN retention_policy NOT IN ('permanent','standard') THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN retention_policy != 'permanent' AND decay_factor >= 0.5 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN retention_policy != 'permanent' AND decay_factor >= 0.1 AND decay_factor < 0.5 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN retention_policy != 'permanent' AND decay_factor < 0.1 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(access_count), 0),
		COALESCE(AVG(importance_score), 0),
		COALESCE(MIN(strftime('%s', created_at)), 0),
		COALESCE(MAX(strftime('%s', created_at)), 0)
		FROM memories WHERE `+userFilter, userArgs...).Scan(
		&retPermanent,
		&retStandard,
		&retOther,
		&stats.DecayHealth.Healthy,
		&stats.DecayHealth.Fading,
		&stats.DecayHealth.Critical,
		&stats.TotalAccess,
		&stats.AvgImportance,
		&stats.OldestAt,
		&stats.NewestAt,
	)
	if err != nil {
		return nil, fmt.Errorf("uhms/store: aggregate totals: %w", err)
	}
	stats.ByRetention["permanent"] = retPermanent
	stats.ByRetention["standard"] = retStandard
	stats.ByRetention["other"] = retOther
	stats.DecayHealth.Permanent = retPermanent

	return stats, nil
}

// ============================================================================
// Entity / Relation (Knowledge Graph)
// ============================================================================

// CreateEntity inserts an entity.
func (s *Store) CreateEntity(e *Entity) error {
	_, err := s.db.Exec(`INSERT INTO entities (id, user_id, name, entity_type, description, created_at)
		VALUES (?,?,?,?,?,?)`,
		e.ID, e.UserID, e.Name, e.EntityType, e.Description,
		e.CreatedAt.Format(time.RFC3339))
	return err
}

// CreateRelation inserts a relation.
func (s *Store) CreateRelation(r *Relation) error {
	_, err := s.db.Exec(`INSERT INTO relations (id, user_id, source_id, target_id, relation_type, weight, created_at)
		VALUES (?,?,?,?,?,?,?)`,
		r.ID, r.UserID, r.SourceID, r.TargetID, r.RelationType, r.Weight,
		r.CreatedAt.Format(time.RFC3339))
	return err
}

// FindEntitiesByName searches entities by name prefix.
func (s *Store) FindEntitiesByName(userID, namePrefix string, limit int) ([]Entity, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT id, user_id, name, entity_type, description, created_at
		FROM entities WHERE user_id = ? AND name LIKE ? LIMIT ?`,
		userID, namePrefix+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var e Entity
		var createdAt string
		if err := rows.Scan(&e.ID, &e.UserID, &e.Name, &e.EntityType, &e.Description, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

// GetRelationsForEntity returns all relations involving an entity.
func (s *Store) GetRelationsForEntity(userID, entityID string) ([]Relation, error) {
	rows, err := s.db.Query(`SELECT id, user_id, source_id, target_id, relation_type, weight, created_at
		FROM relations WHERE user_id = ? AND (source_id = ? OR target_id = ?)`,
		userID, entityID, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []Relation
	for rows.Next() {
		var r Relation
		var createdAt string
		if err := rows.Scan(&r.ID, &r.UserID, &r.SourceID, &r.TargetID, &r.RelationType, &r.Weight, &createdAt); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		relations = append(relations, r)
	}
	return relations, rows.Err()
}

// ============================================================================
// CoreMemory
// ============================================================================

// GetCoreMemory returns the user's core memory.
func (s *Store) GetCoreMemory(userID string) (*CoreMemory, error) {
	var cm CoreMemory
	var updatedAt string
	err := s.db.QueryRow(`SELECT id, user_id, persona, preferences, instructions, updated_at
		FROM core_memory WHERE user_id = ?`, userID).
		Scan(&cm.ID, &cm.UserID, &cm.Persona, &cm.Preferences, &cm.Instructions, &updatedAt)
	if err != nil {
		return nil, err
	}
	cm.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &cm, nil
}

// UpsertCoreMemory creates or updates the user's core memory.
func (s *Store) UpsertCoreMemory(cm *CoreMemory) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO core_memory (id, user_id, persona, preferences, instructions, updated_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(user_id) DO UPDATE SET persona=excluded.persona, preferences=excluded.preferences,
		instructions=excluded.instructions, updated_at=excluded.updated_at`,
		cm.ID, cm.UserID, cm.Persona, cm.Preferences, cm.Instructions, now)
	return err
}

// ============================================================================
// DecayProfile
// ============================================================================

// GetDecayProfile returns the decay profile for a (user, memoryType) pair.
func (s *Store) GetDecayProfile(userID string, memType MemoryType) (*DecayProfile, error) {
	var dp DecayProfile
	var updatedAt string
	err := s.db.QueryRow(`SELECT id, user_id, memory_type, half_life, min_decay, updated_at
		FROM decay_profiles WHERE user_id = ? AND memory_type = ?`, userID, memType).
		Scan(&dp.ID, &dp.UserID, &dp.MemoryType, &dp.HalfLife, &dp.MinDecay, &updatedAt)
	if err != nil {
		return nil, err
	}
	dp.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &dp, nil
}

// UpsertDecayProfile creates or updates a decay profile.
func (s *Store) UpsertDecayProfile(dp *DecayProfile) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO decay_profiles (id, user_id, memory_type, half_life, min_decay, updated_at)
		VALUES (?,?,?,?,?,?)
		ON CONFLICT(user_id, memory_type) DO UPDATE SET half_life=excluded.half_life,
		min_decay=excluded.min_decay, updated_at=excluded.updated_at`,
		dp.ID, dp.UserID, dp.MemoryType, dp.HalfLife, dp.MinDecay, now)
	return err
}

// BatchUpdateDecay applies multiplicative decay to matching memories.
func (s *Store) BatchUpdateDecay(userID string, memType MemoryType, factor float64, minDecay float64) (int64, error) {
	result, err := s.db.Exec(`UPDATE memories SET decay_factor = MAX(decay_factor * ?, ?)
		WHERE user_id = ? AND memory_type = ? AND retention_policy != ? AND decay_factor > ?`,
		factor, minDecay, userID, memType, RetentionPermanent, minDecay)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ============================================================================
// Scan Helpers
// ============================================================================

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanMemory(row rowScanner) (*Memory, error) {
	var m Memory
	var lastAccessed, archived, eventTime, ingestedAt, createdAt, updatedAt sql.NullString

	err := row.Scan(
		&m.ID, &m.UserID, &m.Content, &m.MemoryType, &m.Category,
		&m.ImportanceScore, &m.DecayFactor, &m.RetentionPolicy, &m.AccessCount,
		&lastAccessed, &archived, &eventTime, &ingestedAt,
		&m.VFSPath, &m.EmbeddingRef, &m.Metadata, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	m.LastAccessedAt = parseNullTime(lastAccessed)
	m.ArchivedAt = parseNullTime(archived)
	m.EventTime = parseNullTime(eventTime)
	if ingestedAt.Valid {
		m.IngestedAt, _ = time.Parse(time.RFC3339, ingestedAt.String)
	}
	if createdAt.Valid {
		m.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
	}
	if updatedAt.Valid {
		m.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
	}
	return &m, nil
}

func scanMemories(rows *sql.Rows) ([]Memory, error) {
	var memories []Memory
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, *m)
	}
	return memories, rows.Err()
}

func parseNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, ns.String)
	if err != nil {
		return nil
	}
	return &t
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// ============================================================================
// LIKE Fallback
// ============================================================================

func (s *Store) searchByLike(userID, query string, limit int) ([]SearchResult, error) {
	pattern := "%" + strings.ReplaceAll(query, " ", "%") + "%"
	rows, err := s.db.Query(`SELECT id, user_id, content, memory_type, category,
		importance_score, decay_factor, retention_policy, access_count,
		last_accessed_at, archived_at, event_time, ingested_at, vfs_path,
		embedding_ref, metadata, created_at, updated_at
		FROM memories WHERE user_id = ? AND content LIKE ?
		ORDER BY updated_at DESC LIMIT ?`, userID, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("uhms/store: LIKE search: %w", err)
	}
	defer rows.Close()

	memories, err := scanMemories(rows)
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, len(memories))
	for i, m := range memories {
		results[i] = SearchResult{Memory: m, Score: 0.5, Source: "like"}
	}
	return results, nil
}

// ---------- path helpers ----------

func resolveDBPath(dbPath string) string {
	if dbPath == "" {
		return defaultDBPath()
	}
	if strings.HasPrefix(dbPath, "~") {
		return expandHome(dbPath)
	}
	return dbPath
}
