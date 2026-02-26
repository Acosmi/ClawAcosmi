// Package schemas — system configuration schemas.
// Mirrors Python schemas/system.py — Admin dashboard config view/update.
package schemas

// SystemConfigResponse is the response schema for admin system config viewing.
// API keys are masked for security (e.g., "sk-****abcd").
type SystemConfigResponse struct {
	// Embedding
	EmbeddingProvider  string  `json:"embedding_provider"`
	EmbeddingModel     string  `json:"embedding_model"`
	EmbeddingDimension int     `json:"embedding_dimension"`
	EmbeddingAPIKey    string  `json:"embedding_api_key"`
	EmbeddingBaseURL   *string `json:"embedding_base_url,omitempty"`

	// Rerank
	RerankEnabled  bool    `json:"rerank_enabled"`
	RerankProvider *string `json:"rerank_provider,omitempty"`
	RerankModel    *string `json:"rerank_model,omitempty"`
	RerankAPIKey   string  `json:"rerank_api_key"`

	// LLM
	LLMProvider string  `json:"llm_provider"`
	LLMModel    *string `json:"llm_model,omitempty"`
	LLMBaseURL  *string `json:"llm_base_url,omitempty"`
	LLMAPIKey   string  `json:"llm_api_key"`

	// System
	Version      string `json:"version"`
	Debug        bool   `json:"debug"`
	ConfigSource string `json:"config_source"` // "database" or "environment"
}

// SystemConfigUpdate is the request schema for updating system configuration.
// All fields are optional — only provided fields are updated.
// API keys containing "****" are ignored (treated as masked placeholder).
type SystemConfigUpdate struct {
	// Embedding
	EmbeddingProvider  *string `json:"embedding_provider,omitempty"`
	EmbeddingModel     *string `json:"embedding_model,omitempty"`
	EmbeddingDimension *int    `json:"embedding_dimension,omitempty" binding:"omitempty,gte=64,lte=4096"`
	EmbeddingAPIKey    *string `json:"embedding_api_key,omitempty"`
	EmbeddingBaseURL   *string `json:"embedding_base_url,omitempty"`

	// Rerank
	RerankEnabled  *bool   `json:"rerank_enabled,omitempty"`
	RerankProvider *string `json:"rerank_provider,omitempty"`
	RerankModel    *string `json:"rerank_model,omitempty"`
	RerankAPIKey   *string `json:"rerank_api_key,omitempty"`

	// LLM
	LLMProvider *string `json:"llm_provider,omitempty"`
	LLMModel    *string `json:"llm_model,omitempty"`
	LLMBaseURL  *string `json:"llm_base_url,omitempty"`
	LLMAPIKey   *string `json:"llm_api_key,omitempty"`
}

// SystemConfigUpdateResponse is the response after updating system config.
type SystemConfigUpdateResponse struct {
	Success       bool                 `json:"success"`
	Message       string               `json:"message"`
	UpdatedFields []string             `json:"updated_fields"`
	Config        SystemConfigResponse `json:"config"`
}
