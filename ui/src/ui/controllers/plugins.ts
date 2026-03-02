import type { PluginInfo, PluginsListResult, ToolItem, ToolsListResult, BrowserToolConfig } from "../types.ts";
import type { GatewayBrowserClient } from "../gateway.ts";

type PluginsState = {
  client: GatewayBrowserClient | null;
  connected: boolean;
  pluginsLoading: boolean;
  pluginsList: PluginInfo[];
  pluginsError: string | null;
  pluginsEditValues: Record<string, Record<string, string>>;
  pluginsSaving: string | null;
  toolsLoading: boolean;
  toolsList: ToolItem[];
  toolsError: string | null;
  browserToolConfig: BrowserToolConfig | null;
  browserToolLoading: boolean;
  browserToolSaving: boolean;
  browserToolError: string | null;
  browserToolEdits: Record<string, string | boolean>;
};

export async function loadPlugins(state: PluginsState): Promise<void> {
  if (!state.client || !state.connected) {
    return;
  }
  if (state.pluginsLoading) {
    return;
  }
  state.pluginsLoading = true;
  state.pluginsError = null;
  try {
    const res = await state.client.request<PluginsListResult>("plugins.list", {});
    state.pluginsList = res.plugins ?? [];
  } catch (err) {
    state.pluginsError = String(err);
  } finally {
    state.pluginsLoading = false;
  }
}

export async function savePluginConfig(
  state: PluginsState,
  pluginId: string,
  config: Record<string, string>,
): Promise<void> {
  if (!state.client || !state.connected) {
    return;
  }
  state.pluginsSaving = pluginId;
  try {
    await state.client.request("plugins.config.set", { pluginId, config });
    await loadPlugins(state);
  } catch (err) {
    state.pluginsError = String(err);
  } finally {
    state.pluginsSaving = null;
  }
}

export async function loadTools(state: PluginsState): Promise<void> {
  if (!state.client || !state.connected) {
    return;
  }
  if (state.toolsLoading) {
    return;
  }
  state.toolsLoading = true;
  state.toolsError = null;
  try {
    const res = await state.client.request<ToolsListResult>("tools.list", {});
    state.toolsList = res.tools ?? [];
  } catch (err) {
    state.toolsError = String(err);
  } finally {
    state.toolsLoading = false;
  }
}

export async function loadBrowserToolConfig(state: PluginsState): Promise<void> {
  if (!state.client || !state.connected) {
    return;
  }
  state.browserToolLoading = true;
  state.browserToolError = null;
  try {
    const res = await state.client.request<BrowserToolConfig>("tools.browser.get", {});
    state.browserToolConfig = res;
    state.browserToolEdits = {};
  } catch (err) {
    state.browserToolError = String(err);
  } finally {
    state.browserToolLoading = false;
  }
}

export async function saveBrowserToolConfig(state: PluginsState): Promise<void> {
  if (!state.client || !state.connected) {
    return;
  }
  state.browserToolSaving = true;
  state.browserToolError = null;
  try {
    // Merge current config with edits
    const cfg = state.browserToolConfig ?? { enabled: true, cdpUrl: "", evaluateEnabled: true, headless: false, configured: false };
    const params: Record<string, unknown> = {};

    const edits = state.browserToolEdits;
    params.enabled = "enabled" in edits ? edits.enabled : cfg.enabled;
    params.cdpUrl = "cdpUrl" in edits ? edits.cdpUrl : cfg.cdpUrl;
    params.evaluateEnabled = "evaluateEnabled" in edits ? edits.evaluateEnabled : cfg.evaluateEnabled;
    params.headless = "headless" in edits ? edits.headless : cfg.headless;

    await state.client.request("tools.browser.set", params);
    await loadBrowserToolConfig(state);
  } catch (err) {
    state.browserToolError = String(err);
  } finally {
    state.browserToolSaving = false;
  }
}
