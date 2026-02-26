// Package schemas — memory request/response schemas.
// Covers memory CRUD, search, and graph data structures.
package schemas

import (
	"time"

	"github.com/google/uuid"
)

// --- Memory CRUD ---

// MemoryCreateRequest is the request for creating a new memory.
type MemoryCreateRequest struct {
	Content         string         `json:"content" binding:"required,min=1"`
	UserID          string         `json:"user_id" binding:"required"`
	MemoryType      string         `json:"memory_type" binding:"omitempty,oneof=observation reflection dialogue"`
	Category        string         `json:"category,omitempty"`
	ImportanceScore *float64       `json:"importance_score,omitempty" binding:"omitempty,gte=0,lte=1"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	Pinned          bool           `json:"pinned,omitempty"`
	EventTime       *time.Time     `json:"event_time,omitempty"`
}

// MemoryUpdateRequest is the request for updating a memory.
type MemoryUpdateRequest struct {
	Content         *string        `json:"content,omitempty"`
	MemoryType      *string        `json:"memory_type,omitempty" binding:"omitempty,oneof=observation reflection dialogue"`
	Category        *string        `json:"category,omitempty"`
	ImportanceScore *float64       `json:"importance_score,omitempty" binding:"omitempty,gte=0,lte=1"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// MemoryResponse is the response schema for a single memory.
type MemoryResponse struct {
	ID              uuid.UUID      `json:"id"`
	Content         string         `json:"content"`
	UserID          string         `json:"user_id"`
	MemoryType      string         `json:"memory_type"`
	Category        string         `json:"category"`
	ImportanceScore float64        `json:"importance_score"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       *time.Time     `json:"updated_at,omitempty"`
	AccessCount     int            `json:"access_count"`
	DecayFactor     float64        `json:"decay_factor"`
	ArchivedAt      *time.Time     `json:"archived_at,omitempty"`
	RetentionPolicy string         `json:"retention_policy"`
	EventTime       *time.Time     `json:"event_time,omitempty"`
	IngestedAt      *time.Time     `json:"ingested_at,omitempty"`
}

// MemoryListResponse wraps a list of memories with pagination.
type MemoryListResponse struct {
	Memories []MemoryResponse `json:"memories"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
}

// --- Memory Search ---

// MemorySearchRequest is the request for semantic search over memories.
type MemorySearchRequest struct {
	Query    string  `json:"query" binding:"required,min=1"`
	UserID   string  `json:"user_id" binding:"required"`
	TopK     int     `json:"top_k" binding:"omitempty,gte=1,lte=100"`
	MinScore float64 `json:"min_score" binding:"omitempty,gte=0,lte=1"`
}

// MemorySearchResult represents a single search result with relevance score.
type MemorySearchResult struct {
	Memory MemoryResponse `json:"memory"`
	Score  float64        `json:"score"`
}

// MemorySearchResponse wraps search results.
type MemorySearchResponse struct {
	Results []MemorySearchResult `json:"results"`
	Query   string               `json:"query"`
	Total   int                  `json:"total"`
}

// --- Graph Entities & Relations ---

// EntityResponse is the API response for a knowledge graph entity.
type EntityResponse struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	EntityType  string         `json:"entity_type"`
	Description *string        `json:"description,omitempty"`
	UserID      string         `json:"user_id"`
	CommunityID *int           `json:"community_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	EventTime   *time.Time     `json:"event_time,omitempty"`
	ValidFrom   *time.Time     `json:"valid_from,omitempty"`
	ValidUntil  *time.Time     `json:"valid_until,omitempty"`
	IngestedAt  *time.Time     `json:"ingested_at,omitempty"`
}

// RelationResponse is the API response for a knowledge graph relation.
type RelationResponse struct {
	ID           uuid.UUID  `json:"id"`
	SourceID     uuid.UUID  `json:"source_id"`
	TargetID     uuid.UUID  `json:"target_id"`
	RelationType string     `json:"relation_type"`
	Weight       float64    `json:"weight"`
	CreatedAt    time.Time  `json:"created_at"`
	ValidFrom    *time.Time `json:"valid_from,omitempty"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
	IngestedAt   *time.Time `json:"ingested_at,omitempty"`
}

// GraphResponse wraps a set of entities and relations.
type GraphResponse struct {
	Entities  []EntityResponse   `json:"entities"`
	Relations []RelationResponse `json:"relations"`
}

// --- Common ---

// MessageResponse is a simple message response.
type MessageResponse struct {
	Message string `json:"message"`
}

// ErrorResponse is a standard error response.
type ErrorResponse struct {
	Detail string `json:"detail"`
}

// PaginationParams holds common pagination query parameters.
type PaginationParams struct {
	Page     int `form:"page" binding:"omitempty,gte=1"`
	PageSize int `form:"page_size" binding:"omitempty,gte=1,lte=100"`
}

// DefaultPagination returns pagination params with defaults applied.
func (p PaginationParams) DefaultPagination() (page, pageSize int) {
	page = p.Page
	if page < 1 {
		page = 1
	}
	pageSize = p.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize
}
