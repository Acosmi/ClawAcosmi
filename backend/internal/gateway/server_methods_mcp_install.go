package gateway

// server_methods_mcp_install.go — Gateway RPC handlers for MCP server management.
//
// RPC methods:
//   - mcp.server.register   Register a new server (from CLI install)
//   - mcp.server.uninstall  Remove a server
//   - mcp.server.list       List all servers with status
//   - mcp.server.status     Get status of a specific server
//   - mcp.server.start      Start a specific server
//   - mcp.server.stop       Stop a specific server
//   - mcp.server.update     Re-register (update) a server
//   - mcp.server.tools      List all tools from all running servers

import (
	"context"
	"fmt"

	"github.com/Acosmi/ClawAcosmi/pkg/mcpinstall"
)

// registerMcpInstallHandlers registers all MCP server management RPC methods.
func registerMcpInstallHandlers(registry *MethodRegistry, manager *mcpinstall.McpLocalManager) {
	if manager == nil {
		return
	}

	registry.Register("mcp.server.register", func(ctx *MethodHandlerContext) {
		handleMcpServerRegister(ctx, manager)
	})
	registry.Register("mcp.server.uninstall", func(ctx *MethodHandlerContext) {
		handleMcpServerUninstall(ctx, manager)
	})
	registry.Register("mcp.server.list", func(ctx *MethodHandlerContext) {
		handleMcpServerList(ctx, manager)
	})
	registry.Register("mcp.server.status", func(ctx *MethodHandlerContext) {
		handleMcpServerStatus(ctx, manager)
	})
	registry.Register("mcp.server.start", func(ctx *MethodHandlerContext) {
		handleMcpServerStart(ctx, manager)
	})
	registry.Register("mcp.server.stop", func(ctx *MethodHandlerContext) {
		handleMcpServerStop(ctx, manager)
	})
	registry.Register("mcp.server.update", func(ctx *MethodHandlerContext) {
		handleMcpServerRegister(ctx, manager) // update = re-register
	})
	registry.Register("mcp.server.tools", func(ctx *MethodHandlerContext) {
		handleMcpServerTools(ctx, manager)
	})
}

// ---------- Handlers ----------

func handleMcpServerRegister(ctx *MethodHandlerContext, mgr *mcpinstall.McpLocalManager) {
	name, _ := ctx.Params["name"].(string)
	if name == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInvalidParams, "name is required"))
		return
	}

	binaryPath, _ := ctx.Params["binary_path"].(string)
	transportStr, _ := ctx.Params["transport"].(string)
	transport := mcpinstall.TransportStdio
	if transportStr != "" {
		transport = mcpinstall.TransportMode(transportStr)
	}

	var command *string
	if cmdStr, ok := ctx.Params["command"].(string); ok && cmdStr != "" {
		command = &cmdStr
	}

	var args []string
	if argsRaw, ok := ctx.Params["args"].([]interface{}); ok {
		for _, a := range argsRaw {
			if s, ok := a.(string); ok {
				args = append(args, s)
			}
		}
	}

	env := make(map[string]string)
	if envRaw, ok := ctx.Params["env"].(map[string]interface{}); ok {
		for k, v := range envRaw {
			if s, ok := v.(string); ok {
				env[k] = s
			}
		}
	}

	server := mcpinstall.InstalledMcpServer{
		Name:       name,
		BinaryPath: binaryPath,
		Transport:  transport,
		Command:    command,
		Args:       args,
		Env:        env,
	}

	if err := mgr.RegisterServer(context.Background(), server, true); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, fmt.Sprintf("register failed: %v", err)))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"ok":   true,
		"name": name,
	}, nil)
}

func handleMcpServerUninstall(ctx *MethodHandlerContext, mgr *mcpinstall.McpLocalManager) {
	name, _ := ctx.Params["name"].(string)
	if name == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInvalidParams, "name is required"))
		return
	}

	if err := mgr.UnregisterServer(name); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, fmt.Sprintf("uninstall failed: %v", err)))
		return
	}

	ctx.Respond(true, map[string]interface{}{"ok": true, "name": name}, nil)
}

func handleMcpServerList(ctx *MethodHandlerContext, mgr *mcpinstall.McpLocalManager) {
	servers := mgr.ListServers()
	ctx.Respond(true, map[string]interface{}{
		"servers": servers,
		"count":   len(servers),
	}, nil)
}

func handleMcpServerStatus(ctx *MethodHandlerContext, mgr *mcpinstall.McpLocalManager) {
	name, _ := ctx.Params["name"].(string)
	if name == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInvalidParams, "name is required"))
		return
	}

	status, err := mgr.GetServerStatus(name)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeNotFound, fmt.Sprintf("server not found: %v", err)))
		return
	}

	ctx.Respond(true, status, nil)
}

func handleMcpServerStart(ctx *MethodHandlerContext, mgr *mcpinstall.McpLocalManager) {
	name, _ := ctx.Params["name"].(string)
	if name == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInvalidParams, "name is required"))
		return
	}

	if err := mgr.StartServer(context.Background(), name); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, fmt.Sprintf("start failed: %v", err)))
		return
	}

	ctx.Respond(true, map[string]interface{}{"ok": true, "name": name, "state": "ready"}, nil)
}

func handleMcpServerStop(ctx *MethodHandlerContext, mgr *mcpinstall.McpLocalManager) {
	name, _ := ctx.Params["name"].(string)
	if name == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInvalidParams, "name is required"))
		return
	}

	if err := mgr.StopServer(name); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, fmt.Sprintf("stop failed: %v", err)))
		return
	}

	ctx.Respond(true, map[string]interface{}{"ok": true, "name": name, "state": "stopped"}, nil)
}

func handleMcpServerTools(ctx *MethodHandlerContext, mgr *mcpinstall.McpLocalManager) {
	tools := mgr.AllTools()
	ctx.Respond(true, map[string]interface{}{
		"tools": tools,
		"count": len(tools),
	}, nil)
}
