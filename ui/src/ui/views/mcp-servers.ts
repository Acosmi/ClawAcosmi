// views/mcp-servers.ts — MCP 本地服务器管理页面
// 列表、状态、启停、卸载、工具列表

import { html, nothing } from "lit";
import type { TemplateResult } from "lit";
import type { AppViewState } from "../app-view-state.ts";
import { t } from "../i18n.ts";
import {
  startMcpServer,
  stopMcpServer,
  uninstallMcpServer,
  loadMcpServers,
  loadMcpServerTools,
} from "../controllers/mcp-servers.ts";
import type { McpServerStatus, McpToolEntry } from "../controllers/mcp-servers.ts";

export type McpSubTab = "servers" | "tools";

export function renderMcpServers(state: AppViewState): TemplateResult {
  const activeSubTab = (state.mcpSubTab || "servers") as McpSubTab;

  const setSubTab = (tab: McpSubTab) => {
    state.mcpSubTab = tab;
    state.requestUpdate();
  };

  return html`
    <section class="card">
      <div class="row" style="justify-content: space-between;">
        <div>
          <div class="card-title">${t("mcp.title")}</div>
          <div class="card-sub">${t("nav.sub.mcp")}</div>
        </div>
        <button
          class="btn"
          ?disabled=${state.mcpServersLoading}
          @click=${() => {
            void loadMcpServers(state);
            void loadMcpServerTools(state);
          }}
        >
          ${state.mcpServersLoading ? t("common.loading") : t("common.refresh")}
        </button>
      </div>

      <div class="agent-tabs" style="margin-top: 16px;">
        <button
          class="agent-tab ${activeSubTab === "servers" ? "active" : ""}"
          @click=${() => setSubTab("servers")}
        >
          ${t("mcp.tab.servers")}
        </button>
        <button
          class="agent-tab ${activeSubTab === "tools" ? "active" : ""}"
          @click=${() => setSubTab("tools")}
        >
          ${t("mcp.tab.tools")}
        </button>
      </div>

      <div style="margin-top: 16px;">
        ${activeSubTab === "servers"
          ? renderServersTab(state)
          : renderToolsTab(state)}
      </div>
    </section>
  `;
}

// ---------- Servers 子 tab ----------

function stateChip(s: string): TemplateResult {
  const colors: Record<string, { bg: string; color: string }> = {
    ready: { bg: "rgba(48,209,88,0.12)", color: "#30d158" },
    starting: { bg: "rgba(255,159,10,0.12)", color: "#ff9f0a" },
    degraded: { bg: "rgba(255,59,48,0.12)", color: "#ff3b30" },
    stopped: { bg: "rgba(128,128,128,0.1)", color: "#999" },
    init: { bg: "rgba(128,128,128,0.1)", color: "#999" },
  };
  const c = colors[s] || colors.stopped;
  return html`
    <span style="
      display: inline-flex;
      align-items: center;
      gap: 5px;
      font-size: 11px;
      padding: 2px 8px;
      border-radius: 10px;
      background: ${c.bg};
      color: ${c.color};
      font-weight: 500;
    ">
      <span style="width:6px;height:6px;border-radius:50%;background:${c.color};"></span>
      ${t(`mcp.state.${s}`)}
    </span>
  `;
}

function renderServersTab(state: AppViewState): TemplateResult {
  const servers: McpServerStatus[] = state.mcpServersList || [];

  if (state.mcpServersLoading && servers.length === 0) {
    return html`<div class="muted">${t("common.loading")}</div>`;
  }

  if (servers.length === 0) {
    return html`
      <div style="text-align: center; padding: 40px 0;">
        <div style="font-size: 14px; opacity: 0.6; margin-bottom: 12px;">
          ${t("mcp.empty")}
        </div>
        <div style="font-size: 12px; opacity: 0.4;">
          ${t("mcp.empty.hint")}
        </div>
      </div>
    `;
  }

  return html`
    ${state.mcpServersError ? html`
      <div class="callout danger" style="margin-bottom: 12px;">
        ${state.mcpServersError}
        <button class="btn btn-sm" style="margin-left:8px;font-size:11px;" @click=${() => { state.mcpServersError = null; state.requestUpdate(); }}>
          ${t("common.dismiss")}
        </button>
      </div>
    ` : nothing}

    <div style="display: flex; flex-direction: column; gap: 12px;">
      ${servers.map((s) => renderServerCard(s, state))}
    </div>
  `;
}

function renderServerCard(s: McpServerStatus, state: AppViewState): TemplateResult {
  const server = s.server;
  const busy = state.mcpServersBusy === server.name;
  const isRunning = s.state === "ready" || s.state === "starting";

  return html`
    <div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 10px;">
      <div class="row" style="justify-content: space-between; align-items: center;">
        <div style="display: flex; align-items: center; gap: 10px;">
          <code style="
            font-size: 13px;
            font-weight: 600;
            color: var(--color-accent, #f97316);
            background: rgba(249, 115, 22, 0.1);
            padding: 2px 6px;
            border-radius: 4px;
          ">${server.name}</code>
          ${stateChip(s.state)}
          ${s.tools > 0 ? html`
            <span style="font-size: 11px; opacity: 0.5;">
              ${s.tools} ${t("mcp.toolCount")}
            </span>
          ` : nothing}
        </div>
        <div style="display: flex; gap: 6px;">
          ${isRunning ? html`
            <button
              class="btn btn-sm"
              style="font-size: 12px;"
              ?disabled=${busy}
              @click=${() => void stopMcpServer(state, server.name)}
            >
              ${t("mcp.action.stop")}
            </button>
          ` : html`
            <button
              class="btn btn-sm"
              style="font-size: 12px;"
              ?disabled=${busy}
              @click=${() => void startMcpServer(state, server.name)}
            >
              ${t("mcp.action.start")}
            </button>
          `}
          <button
            class="btn btn-sm"
            style="font-size: 12px; color: var(--danger-color, #d14343);"
            ?disabled=${busy}
            @click=${() => {
              if (confirm(t("mcp.confirm.uninstall"))) {
                void uninstallMcpServer(state, server.name);
              }
            }}
          >
            ${t("mcp.action.uninstall")}
          </button>
        </div>
      </div>

      <div style="display: flex; flex-wrap: wrap; gap: 16px; font-size: 12px; opacity: 0.55;">
        ${server.transport ? html`
          <span>${t("mcp.field.transport")}: <strong>${server.transport}</strong></span>
        ` : nothing}
        ${server.project_type ? html`
          <span>${t("mcp.field.type")}: <strong>${server.project_type}</strong></span>
        ` : nothing}
        ${server.source_url ? html`
          <span title="${server.source_url}">${t("mcp.field.source")}: <strong>${truncateUrl(server.source_url)}</strong></span>
        ` : nothing}
        ${server.installed_at ? html`
          <span>${t("mcp.field.installedAt")}: <strong>${formatDate(server.installed_at)}</strong></span>
        ` : nothing}
      </div>

      ${server.binary_path ? html`
        <div style="font-size: 11px; opacity: 0.35; font-family: monospace; word-break: break-all;">
          ${server.binary_path}
        </div>
      ` : nothing}
    </div>
  `;
}

// ---------- Tools 子 tab ----------

function renderToolsTab(state: AppViewState): TemplateResult {
  const tools: McpToolEntry[] = state.mcpToolsList || [];

  if (state.mcpToolsLoading && tools.length === 0) {
    return html`<div class="muted">${t("common.loading")}</div>`;
  }

  if (tools.length === 0) {
    return html`
      <div style="text-align: center; padding: 40px 0;">
        <div style="font-size: 14px; opacity: 0.6;">
          ${t("mcp.tools.empty")}
        </div>
      </div>
    `;
  }

  // Group by server
  const grouped = new Map<string, McpToolEntry[]>();
  for (const tool of tools) {
    const key = tool.server_name;
    if (!grouped.has(key)) grouped.set(key, []);
    grouped.get(key)!.push(tool);
  }

  return html`
    <div style="font-size: 12px; opacity: 0.5; margin-bottom: 12px;">
      ${t("mcp.tools.count").replace("{count}", String(tools.length))}
    </div>
    ${[...grouped.entries()].map(([serverName, serverTools]) => html`
      <div style="margin-bottom: 20px;">
        <div style="font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; opacity: 0.5; margin-bottom: 10px;">
          ${serverName}
        </div>
        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 10px;">
          ${serverTools.map((entry) => html`
            <div class="card" style="padding: 12px 14px; display: flex; flex-direction: column; gap: 4px;">
              <div style="display: flex; align-items: center; gap: 8px;">
                <code style="
                  font-size: 12px;
                  font-weight: 600;
                  color: var(--color-accent, #f97316);
                  background: rgba(249, 115, 22, 0.1);
                  padding: 1px 5px;
                  border-radius: 3px;
                ">${entry.tool.name}</code>
              </div>
              <div style="font-size: 12px; opacity: 0.55; line-height: 1.4;">
                ${entry.tool.description || entry.tool.title || "—"}
              </div>
              <div style="font-size: 10px; opacity: 0.3; font-family: monospace;">
                ${entry.prefixed_name}
              </div>
            </div>
          `)}
        </div>
      </div>
    `)}
  `;
}

// ---------- Helpers ----------

function truncateUrl(url: string): string {
  if (url.length <= 50) return url;
  return url.slice(0, 47) + "...";
}

function formatDate(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleDateString();
  } catch {
    return iso;
  }
}
