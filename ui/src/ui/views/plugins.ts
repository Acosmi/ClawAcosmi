import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { PluginInfo, ToolItem, BrowserToolConfig } from "../types.ts";

export type PluginsProps = {
  panel: "plugins" | "tools";
  loading: boolean;
  plugins: PluginInfo[];
  error: string | null;
  editValues: Record<string, Record<string, string>>;
  saving: string | null;
  toolsLoading: boolean;
  tools: ToolItem[];
  toolsError: string | null;
  browserConfig: BrowserToolConfig | null;
  browserLoading: boolean;
  browserSaving: boolean;
  browserError: string | null;
  browserEdits: Record<string, string | boolean>;
  onEditChange: (pluginId: string, key: string, value: string) => void;
  onSave: (pluginId: string) => void;
  onGoToChannels: () => void;
  onPanelChange: (panel: "plugins" | "tools") => void;
  onBrowserEditChange: (key: string, value: string | boolean) => void;
  onBrowserSave: () => void;
};

export function renderPlugins(props: PluginsProps) {
  return html`
    <section class="card">
      <div class="row" style="justify-content: space-between; align-items: flex-start;">
        <div>
          <div class="card-title">${t("nav.tab.plugins")}</div>
          <div class="card-sub">${t("nav.sub.plugins")}</div>
        </div>
      </div>

      <!-- Sub-tab bar -->
      <div style="display: flex; gap: 0; margin-top: 16px; border-bottom: 1px solid var(--color-border, rgba(128,128,128,0.15));">
        ${renderSubTab(t("plugins.tab.plugins"), "plugins", props.panel, props.onPanelChange)}
        ${renderSubTab(t("plugins.tab.tools"), "tools", props.panel, props.onPanelChange)}
      </div>

      ${props.panel === "plugins" ? renderPluginsPanel(props) : renderToolsPanel(props)}
    </section>
  `;
}

function renderSubTab(
  label: string,
  value: "plugins" | "tools",
  current: "plugins" | "tools",
  onChange: (v: "plugins" | "tools") => void,
) {
  const active = current === value;
  return html`
    <button
      style="
        padding: 8px 16px;
        font-size: 13px;
        font-weight: ${active ? "600" : "400"};
        background: none;
        border: none;
        border-bottom: 2px solid ${active ? "var(--color-accent, #f97316)" : "transparent"};
        color: ${active ? "var(--color-text)" : "var(--color-muted)"};
        opacity: ${active ? "1" : "0.55"};
        cursor: pointer;
        transition: all 0.15s ease;
      "
      @click=${() => onChange(value)}
    >
      ${label}
    </button>
  `;
}

// ---------- Plugins Panel ----------

function renderPluginsPanel(props: PluginsProps) {
  const channelPlugins = props.plugins.filter((p) => p.category === "channel");
  const searchPlugins = props.plugins.filter((p) => p.category === "search");

  return html`
    ${props.error
      ? html`<div class="callout danger" style="margin-top: 12px;">${props.error}</div>`
      : nothing}

    ${props.loading
      ? html`<div class="muted" style="margin-top: 16px;">${t("common.loading")}</div>`
      : nothing}

    ${!props.loading && channelPlugins.length > 0
      ? html`
        <div style="margin-top: 24px;">
          <div class="section-label" style="font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; opacity: 0.5; margin-bottom: 12px;">
            ${t("plugins.title.channels")}
          </div>
          <div class="plugins-grid" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 12px;">
            ${channelPlugins.map((p) => renderChannelCard(p, props))}
          </div>
        </div>
      `
      : nothing}

    ${!props.loading && searchPlugins.length > 0
      ? html`
        <div style="margin-top: 24px;">
          <div class="section-label" style="font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; opacity: 0.5; margin-bottom: 12px;">
            ${t("plugins.title.search")}
          </div>
          <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 16px;">
            ${searchPlugins.map((p) => renderSearchCard(p, props))}
          </div>
        </div>
      `
      : nothing}
  `;
}

function statusBadge(plugin: PluginInfo) {
  if (plugin.running) {
    return html`<span class="chip chip-ok" style="font-size: 11px;">${t("plugins.status.running")}</span>`;
  }
  if (plugin.configured) {
    return html`<span class="chip" style="font-size: 11px; background: var(--color-accent, #3b82f6); color: #fff;">${t("plugins.status.configured")}</span>`;
  }
  return html`<span class="chip" style="font-size: 11px; opacity: 0.5;">${t("plugins.status.notConfigured")}</span>`;
}

function renderChannelCard(plugin: PluginInfo, props: PluginsProps) {
  return html`
    <div class="card" style="padding: 14px 16px; display: flex; flex-direction: column; gap: 8px;">
      <div class="row" style="justify-content: space-between; align-items: center;">
        <div style="font-weight: 600; font-size: 14px;">${plugin.name}</div>
        ${statusBadge(plugin)}
      </div>
      <div style="font-size: 12px; opacity: 0.6; line-height: 1.4;">${plugin.description}</div>
      <div style="margin-top: 4px;">
        <button
          class="btn btn-sm"
          style="font-size: 12px;"
          @click=${() => props.onGoToChannels()}
        >
          ${t("plugins.btn.goToChannels")}
        </button>
      </div>
    </div>
  `;
}

function renderSearchCard(plugin: PluginInfo, props: PluginsProps) {
  const editVals = props.editValues[plugin.id] ?? {};
  const isSaving = props.saving === plugin.id;
  const fields = plugin.configFields ?? [];

  const getVal = (key: string) => {
    if (key in editVals) return editVals[key];
    return plugin.configValues?.[key] ?? "";
  };

  return html`
    <div class="card" style="padding: 16px; display: flex; flex-direction: column; gap: 12px;">
      <div class="row" style="justify-content: space-between; align-items: center;">
        <div style="font-weight: 600; font-size: 15px;">${plugin.name}</div>
        ${statusBadge(plugin)}
      </div>
      <div style="font-size: 12px; opacity: 0.6; line-height: 1.4;">${plugin.description}</div>

      <div style="display: flex; flex-direction: column; gap: 10px;">
        ${fields.filter((f) => f.type !== "boolean").map(
    (field) => html`
            <div style="display: flex; flex-direction: column; gap: 4px;">
              <label style="font-size: 12px; font-weight: 500; opacity: 0.7;">${field.label}</label>
              <input
                class="input"
                type=${field.sensitive ? "password" : "text"}
                placeholder=${field.placeholder ?? ""}
                .value=${getVal(field.key)}
                @input=${(e: Event) => {
        props.onEditChange(plugin.id, field.key, (e.target as HTMLInputElement).value);
      }}
                style="font-size: 13px;"
              />
            </div>
          `
  )}

        <div class="row" style="justify-content: space-between; align-items: center; gap: 12px;">
          <label class="row" style="gap: 8px; cursor: pointer; font-size: 13px;">
            <input
              type="checkbox"
              ?checked=${getVal("enabled") === "true" || plugin.enabled}
              @change=${(e: Event) => {
      const checked = (e.target as HTMLInputElement).checked;
      props.onEditChange(plugin.id, "enabled", checked ? "true" : "false");
    }}
            />
            ${t("plugins.label.enabled")}
          </label>
          <button
            class="btn btn-primary btn-sm"
            ?disabled=${isSaving}
            @click=${() => props.onSave(plugin.id)}
          >
            ${isSaving ? t("plugins.btn.saving") : t("plugins.btn.save")}
          </button>
        </div>
      </div>
    </div>
  `;
}

// ---------- Tools Panel ----------

const CATEGORY_ORDER = ["file", "exec", "web", "system", "session", "ai", "memory"];

const CATEGORY_ICONS: Record<string, string> = {
  file: "📁",
  exec: "⚡",
  web: "🌐",
  system: "⚙️",
  session: "💬",
  ai: "🤖",
  memory: "🧠",
};

function renderToolsPanel(props: PluginsProps) {
  const tools = props.tools;

  // Group by category
  const grouped = new Map<string, ToolItem[]>();
  for (const tool of tools) {
    const cat = tool.category || "other";
    if (!grouped.has(cat)) grouped.set(cat, []);
    grouped.get(cat)!.push(tool);
  }

  // Sort categories by defined order
  const sortedCategories = [...grouped.keys()].sort((a, b) => {
    const ia = CATEGORY_ORDER.indexOf(a);
    const ib = CATEGORY_ORDER.indexOf(b);
    return (ia === -1 ? 999 : ia) - (ib === -1 ? 999 : ib);
  });

  return html`
    <!-- Browser Automation Configurable Card -->
    ${renderBrowserToolCard(props)}

    ${props.toolsError
      ? html`<div class="callout danger" style="margin-top: 12px;">${props.toolsError}</div>`
      : nothing}

    ${props.toolsLoading
      ? html`<div class="muted" style="margin-top: 16px;">${t("common.loading")}</div>`
      : nothing}

    ${!props.toolsLoading && tools.length > 0
      ? html`
        <div style="margin-top: 16px; display: flex; align-items: center; gap: 8px;">
          <span style="font-size: 12px; opacity: 0.5;">
            ${t("tools.count").replace("{count}", String(tools.length))}
          </span>
        </div>

        ${sortedCategories.map((cat) => html`
          <div style="margin-top: 20px;">
            <div style="font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; opacity: 0.5; margin-bottom: 12px; display: flex; align-items: center; gap: 6px;">
              <span>${CATEGORY_ICONS[cat] ?? "🔧"}</span>
              <span>${t(`tools.category.${cat}`) || cat}</span>
            </div>
            <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 12px;">
              ${grouped.get(cat)!.map((tool) => renderToolCard(tool))}
            </div>
          </div>
        `)}
      `
      : nothing}

    ${!props.toolsLoading && tools.length === 0 && !props.toolsError
      ? html`<div class="muted" style="margin-top: 16px;">${t("tools.empty")}</div>`
      : nothing}
  `;
}

// ---------- Browser Automation Card ----------

function browserStatusBadge(cfg: BrowserToolConfig | null) {
  if (!cfg) {
    return html`<span class="chip" style="font-size: 11px; opacity: 0.5;">${t("plugins.status.notConfigured")}</span>`;
  }
  if (cfg.configured) {
    return html`<span class="chip chip-ok" style="font-size: 11px;">${t("plugins.status.configured")}</span>`;
  }
  return html`<span class="chip" style="font-size: 11px; opacity: 0.5;">${t("plugins.status.notConfigured")}</span>`;
}

function renderBrowserToolCard(props: PluginsProps) {
  const cfg = props.browserConfig;
  const edits = props.browserEdits;

  const getBool = (key: string, fallback: boolean): boolean => {
    if (key in edits) return edits[key] as boolean;
    if (!cfg) return fallback;
    return (cfg as Record<string, unknown>)[key] as boolean ?? fallback;
  };
  const getString = (key: string, fallback: string): string => {
    if (key in edits) return edits[key] as string;
    if (!cfg) return fallback;
    return (cfg as Record<string, unknown>)[key] as string ?? fallback;
  };

  return html`
    <div style="margin-top: 20px;">
      <div style="font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; opacity: 0.5; margin-bottom: 12px; display: flex; align-items: center; gap: 6px;">
        <span>🌐</span>
        <span>${t("tools.browser.title")}</span>
      </div>

      <div class="card" style="padding: 18px 20px; display: flex; flex-direction: column; gap: 14px;">
        <div class="row" style="justify-content: space-between; align-items: center;">
          <div style="display: flex; align-items: center; gap: 10px;">
            <code style="
              font-size: 13px;
              font-weight: 600;
              color: var(--color-accent, #f97316);
              background: rgba(249, 115, 22, 0.1);
              padding: 2px 6px;
              border-radius: 4px;
            ">browser</code>
            <span style="font-weight: 600; font-size: 15px;">${t("tools.browser.title")}</span>
          </div>
          ${browserStatusBadge(cfg)}
        </div>

        <div style="font-size: 12px; opacity: 0.55; line-height: 1.5;">
          ${t("tools.browser.desc")}
        </div>

        ${props.browserError
      ? html`<div class="callout danger" style="font-size: 12px;">${props.browserError}</div>`
      : nothing}

        ${props.browserLoading
      ? html`<div class="muted" style="font-size: 12px;">${t("common.loading")}</div>`
      : html`
            <div style="display: flex; flex-direction: column; gap: 12px; border-top: 1px solid var(--color-border, rgba(128,128,128,0.15)); padding-top: 14px;">

              <!-- enabled toggle -->
              <label class="row" style="gap: 10px; cursor: pointer; font-size: 13px;">
                <input
                  type="checkbox"
                  ?checked=${getBool("enabled", true)}
                  @change=${(e: Event) => {
          props.onBrowserEditChange("enabled", (e.target as HTMLInputElement).checked);
        }}
                />
                <div>
                  <div style="font-weight: 500;">${t("tools.browser.enabled")}</div>
                  <div style="font-size: 11px; opacity: 0.5; margin-top: 2px;">${t("tools.browser.enabled.desc")}</div>
                </div>
              </label>

              <!-- cdpUrl input -->
              <div style="display: flex; flex-direction: column; gap: 4px;">
                <label style="font-size: 12px; font-weight: 500; opacity: 0.7;">${t("tools.browser.cdpUrl")}</label>
                <input
                  class="input"
                  type="text"
                  placeholder="ws://127.0.0.1:9222"
                  .value=${getString("cdpUrl", "")}
                  @input=${(e: Event) => {
          props.onBrowserEditChange("cdpUrl", (e.target as HTMLInputElement).value);
        }}
                  style="font-size: 13px; font-family: monospace;"
                />
                <div style="font-size: 11px; opacity: 0.45; margin-top: 1px;">${t("tools.browser.cdpUrl.desc")}</div>
              </div>

              <!-- evaluateEnabled toggle -->
              <label class="row" style="gap: 10px; cursor: pointer; font-size: 13px;">
                <input
                  type="checkbox"
                  ?checked=${getBool("evaluateEnabled", true)}
                  @change=${(e: Event) => {
          props.onBrowserEditChange("evaluateEnabled", (e.target as HTMLInputElement).checked);
        }}
                />
                <div>
                  <div style="font-weight: 500;">${t("tools.browser.evaluateEnabled")}</div>
                  <div style="font-size: 11px; opacity: 0.5; margin-top: 2px;">${t("tools.browser.evaluateEnabled.desc")}</div>
                </div>
              </label>

              <!-- headless toggle -->
              <label class="row" style="gap: 10px; cursor: pointer; font-size: 13px;">
                <input
                  type="checkbox"
                  ?checked=${getBool("headless", false)}
                  @change=${(e: Event) => {
          props.onBrowserEditChange("headless", (e.target as HTMLInputElement).checked);
        }}
                />
                <div>
                  <div style="font-weight: 500;">${t("tools.browser.headless")}</div>
                  <div style="font-size: 11px; opacity: 0.5; margin-top: 2px;">${t("tools.browser.headless.desc")}</div>
                </div>
              </label>

              <!-- Save button -->
              <div style="display: flex; justify-content: flex-end; margin-top: 4px;">
                <button
                  class="btn btn-primary btn-sm"
                  ?disabled=${props.browserSaving}
                  @click=${() => props.onBrowserSave()}
                >
                  ${props.browserSaving ? t("plugins.btn.saving") : t("plugins.btn.save")}
                </button>
              </div>
            </div>
          `}
      </div>
    </div>
  `;
}

function renderToolCard(tool: ToolItem) {
  const labelKey = `tools.item.${tool.name}.label`;
  const descKey = `tools.item.${tool.name}.desc`;
  const localLabel = t(labelKey);
  const localDesc = t(descKey);
  // t() returns the key itself when missing; use backend value as fallback
  const label = localLabel !== labelKey ? localLabel : tool.label;
  const desc = localDesc !== descKey ? localDesc : tool.description;

  return html`
    <div class="card" style="
      padding: 14px 16px;
      display: flex;
      flex-direction: column;
      gap: 6px;
      transition: transform 0.1s ease, box-shadow 0.1s ease;
    ">
      <div class="row" style="justify-content: space-between; align-items: center;">
        <div style="display: flex; align-items: center; gap: 8px;">
          <code style="
            font-size: 13px;
            font-weight: 600;
            color: var(--color-accent, #f97316);
            background: rgba(249, 115, 22, 0.1);
            padding: 2px 6px;
            border-radius: 4px;
          ">${tool.name}</code>
        </div>
        ${tool.builtin
      ? html`<span class="chip chip-ok" style="font-size: 10px;">${t("tools.badge.builtin")}</span>`
      : nothing}
      </div>
      <div style="font-size: 13px; font-weight: 500; opacity: 0.85;">${label}</div>
      <div style="font-size: 12px; opacity: 0.55; line-height: 1.5;">${desc}</div>
    </div>
  `;
}
