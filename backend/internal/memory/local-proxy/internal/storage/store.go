// Package storage — local SQLite storage layer for UHMS memories.
// Provides CRUD operations on memories, core memory, and tree nodes
// using SQLite as the local persistence layer.
package storage

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ============================================================================
// Models — mirrors cloud UHMS models for local storage
// ============================================================================

// Memory represents a single memory entry stored locally.
type Memory struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	UserID          string    `gorm:"index;not null" json:"user_id"`
	Content         string    `gorm:"type:text;not null" json:"content"`
	MemoryType      string    `gorm:"size:32;default:episodic" json:"memory_type"`
	Category        string    `gorm:"size:32" json:"category"`
	ImportanceScore float64   `gorm:"default:0.5" json:"importance_score"`
	Embedding       []byte    `gorm:"type:blob" json:"-"` // stored as binary blob
	EmbedDimension  int       `json:"-"`
	Metadata        string    `gorm:"type:text" json:"metadata,omitempty"` // JSON string
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// Entity represents a knowledge graph entity.
type Entity struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	UserID      string    `gorm:"index;not null" json:"user_id"`
	Name        string    `gorm:"not null" json:"name"`
	EntityType  string    `gorm:"size:32" json:"entity_type"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// Relation represents a knowledge graph relation between two entities.
type Relation struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	UserID       string    `gorm:"index;not null" json:"user_id"`
	SourceID     string    `gorm:"size:36;not null" json:"source_id"`
	TargetID     string    `gorm:"size:36;not null" json:"target_id"`
	RelationType string    `gorm:"size:64;not null" json:"relation_type"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// CoreMemory stores the user's persistent core memory (persona, preferences, instructions).
type CoreMemory struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	UserID       string    `gorm:"uniqueIndex;not null" json:"user_id"`
	Persona      string    `gorm:"type:text" json:"persona"`
	Preferences  string    `gorm:"type:text" json:"preferences"`
	Instructions string    `gorm:"type:text" json:"instructions"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TreeNode represents a hierarchical memory tree node.
type TreeNode struct {
	ID            string    `gorm:"primaryKey;size:36" json:"id"`
	UserID        string    `gorm:"index;not null" json:"user_id"`
	ParentID      string    `gorm:"size:36;index" json:"parent_id,omitempty"`
	Content       string    `gorm:"type:text;not null" json:"content"`
	NodeType      string    `gorm:"size:32;default:leaf" json:"node_type"` // leaf, summary, root
	Category      string    `gorm:"size:32" json:"category"`
	Depth         int       `gorm:"default:0" json:"depth"`
	ChildrenCount int       `gorm:"default:0" json:"children_count"`
	IsLeaf        bool      `gorm:"default:true" json:"is_leaf"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// ============================================================================
// Store — SQLite database wrapper
// ============================================================================

// Store manages the local SQLite database.
type Store struct {
	db *gorm.DB
}

// NewStore creates and initializes a local SQLite database.
func NewStore(dbPath string) (*Store, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(dbPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("storage: cannot resolve home dir: %w", err)
		}
		dbPath = filepath.Join(home, dbPath[1:])
	}

	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("storage: cannot create dir %s: %w", dir, err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("storage: open sqlite: %w", err)
	}

	// Auto-migrate all tables
	if err := db.AutoMigrate(&Memory{}, &Entity{}, &Relation{}, &CoreMemory{}, &TreeNode{}); err != nil {
		return nil, fmt.Errorf("storage: migrate: %w", err)
	}

	slog.Info("Local SQLite initialized", "path", dbPath)
	return &Store{db: db}, nil
}

// DB returns the underlying gorm.DB for advanced queries.
func (s *Store) DB() *gorm.DB {
	return s.db
}

// Close closes the database connection.
func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
