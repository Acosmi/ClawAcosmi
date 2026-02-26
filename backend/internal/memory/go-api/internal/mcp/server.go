// Package mcp — Model Context Protocol server using the official MCP Go SDK.
// Exposes memory tools for Cursor, Claude Desktop, and other MCP-compatible clients.
// Supports both stdio and SSE transports.
//
// Implementation is split across files:
//   - server.go          — server creation and transport runner (this file)
//   - tools.go           — tool registration
//   - tools_memory.go    — memory tool handler implementations
//   - tools_tree.go      — MemTree tool handler implementations
//   - tools_core_memory.go — Core Memory tool handler implementations
package mcp

import (
	"context"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/services"
)

// ============================================================================
// MCP Server
// ============================================================================

// Server wraps the official MCP SDK server with UHMS service dependencies.
type Server struct {
	inner       *sdkmcp.Server
	db          *gorm.DB
	manager     *services.MemoryManager
	graphStore  *services.GraphStoreService
	treeManager *services.TreeManager
}

// NewServer creates a new MCP server with all UHMS tools and resources registered.
func NewServer(db *gorm.DB, mm *services.MemoryManager, gs *services.GraphStoreService) *Server {
	inner := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "uhms-memory",
		Version: "2.0.0-go",
	}, nil)

	s := &Server{
		inner:       inner,
		db:          db,
		manager:     mm,
		graphStore:  gs,
		treeManager: services.GetTreeManager(),
	}

	s.registerTools()
	s.registerResources()

	return s
}

// Inner returns the underlying SDK server for direct transport integration.
func (s *Server) Inner() *sdkmcp.Server {
	return s.inner
}

// Run runs the MCP server over the given transport until the context is cancelled
// or the peer disconnects.
func (s *Server) Run(ctx context.Context, transport sdkmcp.Transport) error {
	return s.inner.Run(ctx, transport)
}
