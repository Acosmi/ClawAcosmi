// Package algo — Cloud Algorithm API layer.
// Exposes pure computation endpoints (embed, classify, rank, reflect, graph-build)
// for remote clients (e.g., local-proxy) that keep data locally.
//
// Design principle: This package NEVER persists user data. It only processes
// inputs and returns computed results. All data persistence happens on the
// caller side (local-proxy or MCP server).
//
// Endpoints:
//
//	POST /api/v1/algo/embed      — Generate vector embeddings for texts
//	POST /api/v1/algo/classify   — NLP classification + importance scoring
//	POST /api/v1/algo/rank       — Semantic reranking of candidate documents
//	POST /api/v1/algo/reflect    — Generate reflections from memory summaries
//	POST /api/v1/algo/extract    — Extract entities and relations for knowledge graph
package algo

// ============================================================================
// Request / Response types for the Algorithm API
// ============================================================================

// --- Embed ---

// EmbedRequest is the request body for POST /algo/embed.
type EmbedRequest struct {
	Texts []string `json:"texts" binding:"required,min=1"`
}

// EmbedResponse is the response for POST /algo/embed.
type EmbedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Dimension  int         `json:"dimension"`
	Model      string      `json:"model,omitempty"`
}

// --- Classify ---

// ClassifyRequest is the request body for POST /algo/classify.
type ClassifyRequest struct {
	Content string `json:"content" binding:"required"`
}

// ClassifyResponse is the response for POST /algo/classify.
type ClassifyResponse struct {
	Category        string  `json:"category"`
	ImportanceScore float64 `json:"importance_score"`
	Reasoning       string  `json:"reasoning,omitempty"`
}

// --- Rank ---

// RankRequest is the request body for POST /algo/rank.
type RankRequest struct {
	Query     string   `json:"query" binding:"required"`
	Documents []string `json:"documents" binding:"required,min=1"`
	TopN      int      `json:"top_n,omitempty"` // defaults to len(documents)
}

// RankResult is a single ranked document.
type RankResult struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
}

// RankResponse is the response for POST /algo/rank.
type RankResponse struct {
	Results []RankResult `json:"results"`
}

// --- Reflect ---

// ReflectRequest is the request body for POST /algo/reflect.
type ReflectRequest struct {
	Memories          []string `json:"memories" binding:"required,min=1"`
	CoreMemoryContext string   `json:"core_memory_context,omitempty"`
}

// CoreMemoryEdit is a suggested edit to core memory.
type CoreMemoryEdit struct {
	Section string `json:"section"` // persona, preferences, instructions
	Content string `json:"content"`
	Mode    string `json:"mode"` // replace, append
}

// ReflectResponse is the response for POST /algo/reflect.
type ReflectResponse struct {
	Reflection      string           `json:"reflection"`
	CoreMemoryEdits []CoreMemoryEdit `json:"core_memory_edits,omitempty"`
}

// --- Extract (Entity/Relation Extraction for Knowledge Graph) ---

// ExtractRequest is the request body for POST /algo/extract.
type ExtractRequest struct {
	Content string `json:"content" binding:"required"`
}

// ExtractedEntity is an entity extracted by the LLM.
type ExtractedEntity struct {
	Name        string `json:"name"`
	EntityType  string `json:"entity_type"`
	Description string `json:"description,omitempty"`
}

// ExtractedRelation is a relation between two entities.
type ExtractedRelation struct {
	Source       string `json:"source"`
	Target       string `json:"target"`
	RelationType string `json:"relation_type"`
}

// ExtractResponse is the response for POST /algo/extract.
type ExtractResponse struct {
	Entities  []ExtractedEntity   `json:"entities"`
	Relations []ExtractedRelation `json:"relations"`
}

// --- Health ---

// HealthResponse reports the status of algorithm services.
type HealthResponse struct {
	Status         string `json:"status"` // "ok" or "degraded"
	Embedding      bool   `json:"embedding"`
	LLM            bool   `json:"llm"`
	Rerank         bool   `json:"rerank"`
	EmbedDimension int    `json:"embed_dimension,omitempty"`
}
