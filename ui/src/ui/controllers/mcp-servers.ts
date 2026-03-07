// controllers/mcp-servers.ts — MCP 本地服务器管理控制器
// 通过 Gateway RPC 管理 MCP 服务器的生命周期。

import type { AppViewState } from "../app-view-state.ts";

// ---------- 类型 ----------

export interface McpServerInfo {
  name: string;
  source_url: string;
  source_kind: string;
  project_type: string;
  transport: string;
  binary_path: string;
  command?: string;
  args: string[];
  env: Record<string, string>;
  pinned_ref?: string;
  source_commit?: string;
  binary_sha256?: string;
  installed_at: string;
  updated_at?: string;
}

export interface McpServerStatus {
  server: McpServerInfo;
  state: "init" | "starting" | "ready" | "degraded" | "stopped";
  tools: number;
}

export interface McpToolEntry {
  server_name: string;
  tool: {
    name: string;
    title?: string;
    description: string;
  };
  prefixed_name: string;
}

// ---------- 数据加载 ----------

export async function loadMcpServers(state: AppViewState): Promise<void> {
  if (!state.client || !state.connected) return;
  state.mcpServersLoading = true;
  try {
    const res = await state.client.request<{
      servers: McpServerStatus[];
      count: number;
    }>("mcp.server.list");
    if (res) {
      state.mcpServersList = res.servers || [];
    }
  } catch {
    state.mcpServersList = [];
  } finally {
    state.mcpServersLoading = false;
  }
}

export async function loadMcpServerTools(state: AppViewState): Promise<void> {
  if (!state.client || !state.connected) return;
  state.mcpToolsLoading = true;
  try {
    const res = await state.client.request<{
      tools: McpToolEntry[];
      count: number;
    }>("mcp.server.tools");
    if (res) {
      state.mcpToolsList = res.tools || [];
    }
  } catch {
    state.mcpToolsList = [];
  } finally {
    state.mcpToolsLoading = false;
  }
}

// ---------- 操作 ----------

export async function startMcpServer(state: AppViewState, name: string): Promise<boolean> {
  if (!state.client || !state.connected) return false;
  state.mcpServersBusy = name;
  try {
    await state.client.request("mcp.server.start", { name });
    await loadMcpServers(state);
    return true;
  } catch (err) {
    state.mcpServersError = `启动失败: ${err}`;
    return false;
  } finally {
    state.mcpServersBusy = null;
  }
}

export async function stopMcpServer(state: AppViewState, name: string): Promise<boolean> {
  if (!state.client || !state.connected) return false;
  state.mcpServersBusy = name;
  try {
    await state.client.request("mcp.server.stop", { name });
    await loadMcpServers(state);
    return true;
  } catch (err) {
    state.mcpServersError = `停止失败: ${err}`;
    return false;
  } finally {
    state.mcpServersBusy = null;
  }
}

export async function uninstallMcpServer(state: AppViewState, name: string): Promise<boolean> {
  if (!state.client || !state.connected) return false;
  state.mcpServersBusy = name;
  try {
    await state.client.request("mcp.server.uninstall", { name });
    await loadMcpServers(state);
    return true;
  } catch (err) {
    state.mcpServersError = `卸载失败: ${err}`;
    return false;
  } finally {
    state.mcpServersBusy = null;
  }
}

export async function registerMcpServer(
  state: AppViewState,
  server: {
    name: string;
    binary_path?: string;
    transport?: string;
    command?: string;
    args?: string[];
    env?: Record<string, string>;
  },
): Promise<boolean> {
  if (!state.client || !state.connected) return false;
  try {
    await state.client.request("mcp.server.register", server);
    await loadMcpServers(state);
    return true;
  } catch (err) {
    state.mcpServersError = `注册失败: ${err}`;
    return false;
  }
}

export async function loadMcpDashboard(state: AppViewState): Promise<void> {
  await Promise.all([
    loadMcpServers(state),
    loadMcpServerTools(state),
  ]);
}
