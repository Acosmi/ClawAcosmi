// views/media-manage.ts — 媒体运营管理页面（独立侧栏入口）
// 子 tab 导航 + 按 tab 分发到各面板渲染函数

import { html, nothing } from "lit";
import type { TemplateResult } from "lit";
import type { AppViewState } from "../app-view-state.ts";
import { t } from "../i18n.ts";
import {
  renderConfigPanel,
  renderPatrolPanel,
  renderProgressBanner,
  renderHeartbeatPanel,
  renderTrendingPanel,
  renderDraftsPanel,
  renderPublishPanel,
  renderDraftDetailModal,
  renderPublishDetailModal,
  renderDraftEditModal,
} from "./media-dashboard.ts";
import { toggleMediaTool, toggleMediaSource, updateMediaConfig } from "../controllers/media-dashboard.ts";

export type MediaSubTab = "overview" | "llm" | "sources" | "tools" | "drafts" | "publish" | "patrol" | "strategy";

const SUB_TABS: { id: MediaSubTab; labelKey: string }[] = [
  { id: "overview", labelKey: "media.subtab.overview" },
  { id: "llm", labelKey: "media.subtab.llm" },
  { id: "sources", labelKey: "media.subtab.sources" },
  { id: "tools", labelKey: "media.subtab.tools" },
  { id: "strategy", labelKey: "media.subtab.strategy" },
  { id: "drafts", labelKey: "media.subtab.drafts" },
  { id: "publish", labelKey: "media.subtab.publish" },
  { id: "patrol", labelKey: "media.subtab.patrol" },
];

export function renderMediaManage(state: AppViewState): TemplateResult {
  const activeSubTab = (state.mediaManageSubTab || "overview") as MediaSubTab;

  const setSubTab = (tab: MediaSubTab) => {
    state.mediaManageSubTab = tab;
    (state as any).requestUpdate?.();
  };

  return html`
    <section class="card">
      <div class="row" style="justify-content: space-between;">
        <div>
          <div class="card-title">${t("media.manage.title")}</div>
          <div class="card-sub">${t("nav.sub.media")}</div>
        </div>
      </div>

      <div class="agent-tabs" style="margin-top: 16px;">
        ${SUB_TABS.map(
          (tab) => html`
            <button
              class="agent-tab ${activeSubTab === tab.id ? "active" : ""}"
              @click=${() => setSubTab(tab.id)}
            >
              ${t(tab.labelKey)}
            </button>
          `,
        )}
      </div>

      <div style="margin-top: 16px;">
        ${dispatchSubTab(activeSubTab, state)}
      </div>
    </section>

    ${renderDraftDetailModal(state)}
    ${renderPublishDetailModal(state)}
    ${renderDraftEditModal(state)}
  `;
}

function dispatchSubTab(tab: MediaSubTab, state: AppViewState): TemplateResult | typeof nothing {
  switch (tab) {
    case "overview":
      return renderOverviewTab(state);
    case "llm":
      return renderConfigPanel(state);
    case "sources":
      return renderSourcesTab(state);
    case "tools":
      return renderToolsTab(state);
    case "drafts":
      return renderDraftsPanel(state);
    case "publish":
      return renderPublishPanel(state);
    case "patrol":
      return renderPatrolTab(state);
    case "strategy":
      return renderStrategyTab(state);
    default:
      return nothing;
  }
}

// ---------- Overview 子 tab ----------

function renderOverviewTab(state: AppViewState): TemplateResult {
  const config = state.mediaConfig;
  const isConfigured = config?.status === "configured";
  const toolCount = config?.tools?.length ?? 0;
  const sourceCount = config?.trending_sources?.length ?? 0;
  const draftCount = state.mediaDrafts?.length ?? 0;

  return html`
    <div style="display:flex;flex-direction:column;gap:16px;">
      ${renderProgressBanner(state)}
      ${renderHeartbeatPanel(state.mediaHeartbeat)}

      <div class="media-stat-grid">
        <div class="media-stat-card">
          <div class="media-stat-value">${toolCount}</div>
          <div class="media-stat-label">${t("media.subtab.tools")}</div>
        </div>
        <div class="media-stat-card">
          <div class="media-stat-value">${sourceCount}</div>
          <div class="media-stat-label">${t("media.subtab.sources")}</div>
        </div>
        <div class="media-stat-card">
          <div class="media-stat-value">${draftCount}</div>
          <div class="media-stat-label">${t("media.subtab.drafts")}</div>
        </div>
        <div class="media-stat-card">
          <div class="${isConfigured ? "media-stat-value" : "media-stat-value media-stat-value--warn"}">
            ${isConfigured ? "LLM" : "—"}
          </div>
          <div class="media-stat-label">${t("media.subtab.llm")}</div>
        </div>
      </div>

      ${!isConfigured ? html`
        <div class="callout" style="display:flex;align-items:center;gap:12px;">
          <span>${t("media.overview.notConfigured")}</span>
          <button
            class="btn primary"
            @click=${() => {
              state.mediaManageSubTab = "llm";
              (state as any).requestUpdate?.();
            }}
          >
            ${t("media.overview.configureNow")}
          </button>
        </div>
      ` : nothing}
    </div>
  `;
}

// ---------- Tools 子 tab ----------

const TOOL_ICONS: Record<string, string> = {
  trending_topics: "📊",
  content_compose: "✍️",
  media_publish: "🚀",
  social_interact: "💬",
  web_search: "🔍",
  report_progress: "📢",
};

const TOOL_LABELS: Record<string, string> = {
  trending_topics: "热点发现",
  content_compose: "内容创作",
  media_publish: "多平台发布",
  social_interact: "社交互动",
  web_search: "网页搜索",
  report_progress: "进度汇报",
};

// 可切换的工具（media_publish / social_interact）
const TOGGLEABLE_TOOLS = new Set(["media_publish", "social_interact"]);

function renderToolsTab(state: AppViewState): TemplateResult {
  const config = state.mediaConfig;
  if (!config) {
    return html`<div class="muted">${t("common.loading")}</div>`;
  }

  const allTools = config.tools || [];
  const mediaTools = allTools.filter((ti: any) => ti.scope !== "shared");
  const sharedTools = allTools.filter((ti: any) => ti.scope === "shared");

  const renderToolCard = (tool: any, toggleable: boolean) => {
    const name = tool.name;
    const enabled = tool.enabled;
    return html`
      <div class="media-tool-card ${enabled ? "" : "media-tool-card--disabled"}">
        <span class="media-tool-icon">${TOOL_ICONS[name] || "🔧"}</span>
        <div style="flex:1;min-width:0;">
          <div style="display:flex;align-items:center;gap:8px;">
            <span class="media-tool-name">${TOOL_LABELS[name] || name}</span>
            ${enabled
              ? html`<span class="chip chip-ok" style="font-size:10px;">已启用</span>`
              : html`<span class="chip chip-muted" style="font-size:10px;">未启用</span>`}
          </div>
          <div class="media-tool-desc">
            ${tool.description || TOOL_LABELS[name] || name}
          </div>
          ${toggleable ? html`
            <label class="media-tool-toggle">
              <input
                type="checkbox"
                .checked=${enabled}
                @change=${(e: Event) => {
                  const checked = (e.target as HTMLInputElement).checked;
                  void toggleMediaTool(state, name, checked);
                }}
              />
              ${enabled ? "启用中" : "已关闭"}
            </label>
          ` : nothing}
        </div>
      </div>
    `;
  };

  return html`
    <div style="display:flex;flex-direction:column;gap:16px;">
      <div class="media-section-title">媒体专属工具</div>
      <div class="media-tool-grid">
        ${mediaTools.map((tool: any) => renderToolCard(tool, TOGGLEABLE_TOOLS.has(tool.name)))}
      </div>
      ${sharedTools.length > 0 ? html`
        <div class="media-section-title media-section-title--spaced">共享工具（运行时自动获得）</div>
        <div class="media-tool-grid">
          ${sharedTools.map((tool: any) => renderToolCard(tool, false))}
        </div>
      ` : nothing}
    </div>
  `;
}

// ---------- Sources 子 tab ----------

const ALL_SOURCES = ["weibo", "baidu", "zhihu"] as const;
const SOURCE_LABELS: Record<string, string> = {
  weibo: "微博热搜",
  baidu: "百度热搜",
  zhihu: "知乎热榜",
};

function renderSourcesTab(state: AppViewState): TemplateResult {
  const config = state.mediaConfig;
  // 语义: enabled_sources_configured=false (nil) → 全部启用
  //        enabled_sources_configured=true, trending_sources=[] → 全部禁用
  //        enabled_sources_configured=true, trending_sources=[...] → 仅列出的启用
  const sourcesConfigured = config?.enabled_sources_configured === true;
  const registeredSources = (config?.trending_sources || []).map((si: any) => si.name);
  const allEnabled = !sourcesConfigured; // nil = 全部启用

  return html`
    <div style="display:flex;flex-direction:column;gap:16px;">
      <div class="card media-source-panel">
        <div class="media-source-title">热点来源开关</div>
        ${allEnabled ? html`
          <div class="media-source-status">当前为默认模式：全部来源已启用</div>
        ` : nothing}
        <div class="media-source-grid">
          ${ALL_SOURCES.map((name) => {
            const enabled = allEnabled || registeredSources.includes(name);
            return html`
              <label style="display:flex;align-items:center;gap:6px;cursor:pointer;font-size:13px;">
                <input
                  type="checkbox"
                  .checked=${enabled}
                  @change=${(e: Event) => {
                    const checked = (e.target as HTMLInputElement).checked;
                    void toggleMediaSource(state, name, checked);
                  }}
                />
                ${SOURCE_LABELS[name] || name}
              </label>
            `;
          })}
        </div>
        <div class="media-source-hint">
          取消勾选将禁用该来源的热点抓取（需重启子系统生效）。
          首次修改后将切换为显式配置模式。
        </div>
      </div>
      ${renderTrendingPanel(state)}
    </div>
  `;
}

// ---------- Strategy 子 tab ----------

function renderStrategyTab(state: AppViewState): TemplateResult {
  const config = state.mediaConfig;
  const strategy = config?.trending_strategy;
  const hotKeywords = strategy?.hotKeywords ?? [];
  const monitorInterval = strategy?.monitorIntervalMin ?? 30;
  const threshold = strategy?.trendingThreshold ?? 10000;
  const categories = strategy?.contentCategories ?? [];
  const autoDraft = strategy?.autoDraftEnabled ?? false;

  return html`
    <div style="display:flex;flex-direction:column;gap:16px;">
      <div class="card media-strategy-panel">
        <div class="media-section-title" style="margin-bottom:12px;">热点策略配置</div>

        <label class="field" style="margin-bottom:12px;">
          <span class="media-config-field-label">热度阈值（低于此值的话题将被跳过）</span>
          <input
            type="number"
            class="media-strategy-input"
            .value=${String(threshold)}
            @change=${(e: Event) => {
              const v = parseFloat((e.target as HTMLInputElement).value);
              if (!isNaN(v) && v >= 0) void updateMediaConfig(state, { trendingThreshold: v });
            }}
          />
        </label>

        <label class="field" style="margin-bottom:12px;">
          <span class="media-config-field-label">监控频率（分钟）</span>
          <input
            type="number"
            class="media-strategy-input"
            min="5"
            max="1440"
            .value=${String(monitorInterval)}
            @change=${(e: Event) => {
              const v = parseInt((e.target as HTMLInputElement).value, 10);
              if (!isNaN(v) && v >= 5) void updateMediaConfig(state, { monitorIntervalMin: v });
            }}
          />
        </label>

        <label class="field" style="margin-bottom:12px;">
          <span class="media-config-field-label">自定义关键词（逗号分隔）</span>
          <input
            type="text"
            class="media-strategy-input media-strategy-input--wide"
            .value=${hotKeywords.join(", ")}
            @change=${(e: Event) => {
              const v = (e.target as HTMLInputElement).value;
              const keywords = v.split(",").map(s => s.trim()).filter(Boolean);
              void updateMediaConfig(state, { hotKeywords: keywords });
            }}
            placeholder="例如: AI, 科技, 创业"
          />
        </label>

        <label class="field" style="margin-bottom:12px;">
          <span class="media-config-field-label">内容领域偏好（逗号分隔）</span>
          <input
            type="text"
            class="media-strategy-input media-strategy-input--wide"
            .value=${categories.join(", ")}
            @change=${(e: Event) => {
              const v = (e.target as HTMLInputElement).value;
              const cats = v.split(",").map(s => s.trim()).filter(Boolean);
              void updateMediaConfig(state, { contentCategories: cats });
            }}
            placeholder="例如: 科技, 教育, 商业"
          />
        </label>

        <label style="display:flex;align-items:center;gap:8px;cursor:pointer;font-size:13px;">
          <input
            type="checkbox"
            .checked=${autoDraft}
            @change=${(e: Event) => {
              const checked = (e.target as HTMLInputElement).checked;
              void updateMediaConfig(state, { autoDraftEnabled: checked });
            }}
          />
          自动生成草稿（发现匹配热点时自动创建内容草稿）
        </label>
      </div>
    </div>
  `;
}

// ---------- Patrol 子 tab ----------

function renderPatrolTab(state: AppViewState): TemplateResult {
  return html`
    <div style="display:flex;flex-direction:column;gap:16px;">
      ${renderPatrolPanel(state)}
      ${renderHeartbeatPanel(state.mediaHeartbeat)}
    </div>
  `;
}
