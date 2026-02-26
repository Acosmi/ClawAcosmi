// Package handler — MCP over HTTP handler.
// Exposes the UHMS MCP server via Streamable HTTP transport (SSE-based),
// allowing remote MCP clients (Claude Desktop, Cursor, etc.) to connect
// over HTTP instead of stdio.
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/uhms/go-api/internal/database"
	"github.com/uhms/go-api/internal/mcp"
	"github.com/uhms/go-api/internal/services"
)

// MCPHandler handles MCP-over-HTTP connections.
type MCPHandler struct {
	httpHandler *sdkmcp.StreamableHTTPHandler
}

// NewMCPHandler creates a new MCPHandler that wraps an MCP server
// with Streamable HTTP transport. Uses TenantDBRouter for per-request
// DB routing based on the authenticated user identity.
func NewMCPHandler(router *database.TenantDBRouter, mm *services.MemoryManager, gs *services.GraphStoreService) *MCPHandler {
	handler := sdkmcp.NewStreamableHTTPHandler(
		func(r *http.Request) *sdkmcp.Server {
			// Extract tenant_id from auth context (set by auth/billing middleware).
			// Falls back to X-Tenant-ID header only for backward compat.
			tenantID := ""
			if uid, ok := r.Context().Value("user_id").(string); ok && uid != "" {
				tenantID = uid
			} else if uid := r.Header.Get("X-Tenant-ID"); uid != "" {
				tenantID = uid
			}

			// Get per-tenant DB connection.
			db, _ := router.GetDB(r.Context(), tenantID)

			// Create MCP server with tenant-specific DB.
			mcpServer := mcp.NewServer(db, mm, gs)
			return mcpServer.Inner()
		},
		&sdkmcp.StreamableHTTPOptions{
			Stateless: true, // Each request is independent (API Key auth)
		},
	)
	return &MCPHandler{httpHandler: handler}
}

// RegisterRoutes registers MCP HTTP routes on the given router group.
//
// Endpoints:
//
//	POST /mcp — Main MCP message endpoint (Streamable HTTP)
//	GET  /mcp — SSE stream for server-initiated messages
//	DELETE /mcp — Close MCP session
func (h *MCPHandler) RegisterRoutes(rg *gin.RouterGroup) {
	// The SDK's StreamableHTTPHandler handles POST, GET, DELETE on the same path.
	// We wrap it to be compatible with Gin's routing.
	rg.POST("/mcp", h.handleMCP)
	rg.GET("/mcp", h.handleMCP)
	rg.DELETE("/mcp", h.handleMCP)
}

// handleMCP delegates to the SDK's StreamableHTTPHandler.
func (h *MCPHandler) handleMCP(c *gin.Context) {
	h.httpHandler.ServeHTTP(c.Writer, c.Request)
}
