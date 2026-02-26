// Package schemas provides request/response structs for API validation.
// Mirrors Python schemas/docs.py — Documentation CMS endpoint schemas.
package schemas

import "time"

// --- API Parameter ---

// ApiParameter defines an API parameter specification.
type ApiParameter struct {
	Name        string `json:"name" binding:"required,min=1,max=100"`
	Type        string `json:"type" binding:"required,min=1,max=50"`
	Required    bool   `json:"required"`
	Description string `json:"description" binding:"max=500"`
}

// --- Code Examples ---

// CodeExamples holds code samples in multiple languages.
type CodeExamples struct {
	Curl       string `json:"curl"`
	Python     string `json:"python"`
	Javascript string `json:"javascript"`
}

// --- Doc Endpoint CRUD ---

// DocEndpointCreate is the request schema for creating/syncing a doc endpoint.
type DocEndpointCreate struct {
	ID              string         `json:"id" binding:"required,min=1,max=100"`
	Category        string         `json:"category" binding:"required,min=1,max=50"`
	Name            string         `json:"name" binding:"required,min=1,max=200"`
	Method          string         `json:"method" binding:"required,oneof=GET POST PUT DELETE PATCH CONCEPT"`
	Path            string         `json:"path" binding:"required,min=1,max=500"`
	Description     string         `json:"description" binding:"max=50000"`
	PricingInfo     *string        `json:"pricing_info,omitempty" binding:"omitempty,max=100"`
	IsMock          bool           `json:"is_mock"`
	Parameters      []ApiParameter `json:"parameters,omitempty"`
	RequestBody     map[string]any `json:"request_body,omitempty"`
	ResponseExample any            `json:"response_example,omitempty"`
	CodeExamples    *CodeExamples  `json:"code_examples,omitempty"`
	SortOrder       int            `json:"sort_order" binding:"omitempty,gte=0"`
}

// DocEndpointUpdate is the request schema for partial updates (PATCH).
type DocEndpointUpdate struct {
	Name            *string        `json:"name,omitempty" binding:"omitempty,max=200"`
	Description     *string        `json:"description,omitempty" binding:"omitempty,max=50000"`
	PricingInfo     *string        `json:"pricing_info,omitempty" binding:"omitempty,max=100"`
	IsMock          *bool          `json:"is_mock,omitempty"`
	Parameters      []ApiParameter `json:"parameters,omitempty"`
	RequestBody     map[string]any `json:"request_body,omitempty"`
	ResponseExample any            `json:"response_example,omitempty"`
	CodeExamples    *CodeExamples  `json:"code_examples,omitempty"`
	SortOrder       *int           `json:"sort_order,omitempty" binding:"omitempty,gte=0"`
}

// DocEndpointResponse is the full response schema for a doc endpoint.
type DocEndpointResponse struct {
	ID              string            `json:"id"`
	Category        string            `json:"category"`
	Name            string            `json:"name"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	Description     string            `json:"description"`
	PricingInfo     *string           `json:"pricing_info,omitempty"`
	IsMock          bool              `json:"is_mock"`
	Parameters      []map[string]any  `json:"parameters,omitempty"`
	RequestBody     map[string]any    `json:"request_body,omitempty"`
	ResponseExample any               `json:"response_example,omitempty"`
	CodeExamples    map[string]string `json:"code_examples,omitempty"`
	SortOrder       int               `json:"sort_order"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       *time.Time        `json:"updated_at,omitempty"`
}

// --- Grouped Responses ---

// DocCategoryGroup groups endpoints by category.
type DocCategoryGroup struct {
	Category  string                `json:"category"`
	Endpoints []DocEndpointResponse `json:"endpoints"`
}

// DocsListResponse is the response for admin docs listing.
type DocsListResponse struct {
	Groups []DocCategoryGroup `json:"groups"`
	Total  int                `json:"total"`
}

// --- Sync ---

// DocsSyncRequest is the request for bulk sync.
type DocsSyncRequest struct {
	Endpoints []DocEndpointCreate `json:"endpoints" binding:"required,dive"`
}

// DocsSyncResponse is the response for bulk sync operations.
type DocsSyncResponse struct {
	Created int    `json:"created"`
	Updated int    `json:"updated"`
	Message string `json:"message"`
}

// --- Public API Docs ---

// PublicDocEndpoint is the public-facing endpoint format.
type PublicDocEndpoint struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	DisplayLabel    *string           `json:"displayLabel,omitempty"`
	Category        string            `json:"category"`
	IsPublic        bool              `json:"isPublic"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	Description     string            `json:"description"`
	Parameters      []map[string]any  `json:"parameters"`
	RequestBody     map[string]any    `json:"requestBody,omitempty"`
	ResponseExample any               `json:"responseExample,omitempty"`
	CodeExamples    map[string]string `json:"codeExamples"`
}

// PublicDocsResponse wraps public docs keyed by endpoint ID.
type PublicDocsResponse struct {
	Endpoints map[string]PublicDocEndpoint `json:"endpoints"`
}
