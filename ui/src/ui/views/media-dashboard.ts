// views/media-dashboard.ts — 媒体运营仪表盘视图
// 三面板布局: 热点趋势 | 内容草稿 | 发布状态

import { html, nothing } from "lit";
import type { TemplateResult } from "lit";
import type { AppViewState } from "../app-view-state.ts";
import { t } from "../i18n.ts";
import {
  loadTrendingTopics,
  loadDraftsList,
  loadPublishHistory,
  deleteDraft,
  type TrendingTopic,
  type DraftEntry,
  type PublishRecord,
} from "../controllers/media-dashboard.ts";

export function renderMediaDashboard(state: AppViewState): TemplateResult {
  return html`
    <div style="display:flex;flex-direction:column;gap:20px;padding:0 4px;">
      ${renderProgressBanner(state)}
      ${renderTrendingPanel(state)}
      ${renderDraftsPanel(state)}
      ${renderPublishPanel(state)}
    </div>
  `;
}

// ---------- 进度横幅 ----------

function renderProgressBanner(state: AppViewState): TemplateResult | typeof nothing {
  const progress = state.agentProgress;
  if (!progress) return nothing;
  const percent = progress.percent;
  const phase = progress.phase;
  const elapsed = Math.round((Date.now() - progress.ts) / 1000);
  const stale = elapsed > 120; // >2min 视为过期
  if (stale) return nothing;
  return html`
    <div class="card" style="border-left:3px solid var(--accent);background:var(--bg-secondary);">
      <div class="card-body" style="padding:10px 14px;">
        <div style="display:flex;align-items:center;gap:10px;">
          <span style="font-size:14px;">&#9881;</span>
          <div style="flex:1;min-width:0;">
            <div style="font-size:13px;font-weight:500;">${progress.summary}</div>
            ${phase ? html`<div style="font-size:11px;color:var(--text-muted);margin-top:2px;">${phase}</div>` : nothing}
          </div>
          ${percent != null && percent > 0 ? html`<span style="font-size:12px;font-weight:600;color:var(--accent);white-space:nowrap;">${percent}%</span>` : nothing}
        </div>
        ${percent != null && percent > 0 ? html`
          <div style="margin-top:6px;height:4px;border-radius:2px;background:var(--bg-tertiary);overflow:hidden;">
            <div style="height:100%;width:${Math.min(percent, 100)}%;background:var(--accent);border-radius:2px;transition:width 0.3s ease;"></div>
          </div>
        ` : nothing}
      </div>
    </div>
  `;
}

// ---------- 热点趋势面板 ----------

function renderTrendingPanel(state: AppViewState): TemplateResult {
  const topics = state.mediaTrendingTopics || [];
  const sources = state.mediaTrendingSources || [];
  const loading = state.mediaTrendingLoading || false;
  const selectedSource = state.mediaTrendingSelectedSource || "";

  return html`
    <div class="card">
      <div class="card-header" style="display:flex;justify-content:space-between;align-items:center;">
        <h3 style="margin:0;font-size:15px;font-weight:600;">${t("media.trending.title")}</h3>
        <div style="display:flex;gap:8px;align-items:center;">
          ${sources.length > 0
            ? html`
                <select
                  class="input"
                  style="font-size:12px;padding:4px 8px;"
                  .value=${selectedSource}
                  @change=${(e: Event) => {
                    const val = (e.target as HTMLSelectElement).value;
                    state.mediaTrendingSelectedSource = val;
                    (state as any).requestUpdate?.();
                  }}
                >
                  <option value="">${t("media.trending.allSources")}</option>
                  ${sources.map((s: string) => html`<option value=${s}>${s}</option>`)}
                </select>
              `
            : nothing}
          <button
            class="btn btn-sm"
            ?disabled=${loading}
            @click=${() => {
              loadTrendingTopics(state, selectedSource || undefined);
              (state as any).requestUpdate?.();
            }}
          >
            ${loading ? t("media.trending.fetching") : t("media.trending.fetch")}
          </button>
        </div>
      </div>
      <div class="card-body" style="max-height:400px;overflow-y:auto;">
        ${topics.length === 0
          ? html`<p class="empty-hint">${t("media.trending.empty")}</p>`
          : html`
              <div style="display:flex;flex-direction:column;gap:4px;">
                ${topics.map((topic: TrendingTopic) => renderTrendingItem(topic))}
              </div>
            `}
      </div>
    </div>
  `;
}

function renderTrendingItem(topic: TrendingTopic): TemplateResult {
  const heatStr = formatHeatScore(topic.heat_score);
  return html`
    <div
      class="list-item"
      style="display:flex;justify-content:space-between;align-items:center;padding:8px 12px;border-radius:6px;"
    >
      <div style="flex:1;min-width:0;">
        ${topic.url
          ? html`<a
              href=${topic.url}
              target="_blank"
              rel="noopener"
              style="color:var(--text-primary);text-decoration:none;font-size:13px;"
              >${topic.title}</a
            >`
          : html`<span style="font-size:13px;">${topic.title}</span>`}
        <span class="pill" style="margin-left:8px;font-size:10px;">${topic.source}</span>
      </div>
      <span style="font-size:12px;color:var(--text-muted);white-space:nowrap;">
        ${heatStr}
      </span>
    </div>
  `;
}

function formatHeatScore(score: number): string {
  if (score >= 10000) {
    return (score / 10000).toFixed(1) + "万";
  }
  if (score >= 1000) {
    return (score / 1000).toFixed(1) + "k";
  }
  return String(Math.round(score));
}

// ---------- 内容草稿面板 ----------

function renderDraftsPanel(state: AppViewState): TemplateResult {
  const drafts = state.mediaDrafts || [];
  const loading = state.mediaDraftsLoading || false;
  const selectedPlatform = state.mediaDraftsSelectedPlatform || "";

  return html`
    <div class="card">
      <div class="card-header" style="display:flex;justify-content:space-between;align-items:center;">
        <h3 style="margin:0;font-size:15px;font-weight:600;">${t("media.drafts.title")}</h3>
        <div style="display:flex;gap:8px;align-items:center;">
          <select
            class="input"
            style="font-size:12px;padding:4px 8px;"
            .value=${selectedPlatform}
            @change=${(e: Event) => {
              const val = (e.target as HTMLSelectElement).value;
              state.mediaDraftsSelectedPlatform = val;
              loadDraftsList(state, val || undefined);
              (state as any).requestUpdate?.();
            }}
          >
            <option value="">${t("media.drafts.allPlatforms")}</option>
            <option value="xiaohongshu">小红书</option>
            <option value="wechat">微信</option>
            <option value="website">Website</option>
          </select>
        </div>
      </div>
      <div class="card-body" style="max-height:400px;overflow-y:auto;">
        ${loading
          ? html`<p class="empty-hint">Loading…</p>`
          : drafts.length === 0
            ? html`<p class="empty-hint">${t("media.drafts.empty")}</p>`
            : html`
                <div style="display:flex;flex-direction:column;gap:4px;">
                  ${drafts.map((d: DraftEntry) => renderDraftItem(state, d))}
                </div>
              `}
      </div>
    </div>
  `;
}

function renderDraftItem(state: AppViewState, draft: DraftEntry): TemplateResult {
  const statusColor =
    draft.status === "published"
      ? "var(--green)"
      : draft.status === "approved"
        ? "var(--blue)"
        : "var(--text-muted)";
  return html`
    <div
      class="list-item"
      style="display:flex;justify-content:space-between;align-items:center;padding:8px 12px;border-radius:6px;"
    >
      <div style="flex:1;min-width:0;">
        <span style="font-size:13px;font-weight:500;">${draft.title || "(untitled)"}</span>
        <span class="pill" style="margin-left:8px;font-size:10px;">${draft.platform}</span>
        <span style="margin-left:6px;font-size:11px;color:${statusColor};">${draft.status}</span>
      </div>
      <button
        class="btn btn-sm btn-danger"
        style="font-size:11px;"
        @click=${() => {
          deleteDraft(state, draft.id);
          (state as any).requestUpdate?.();
        }}
      >
        ${t("media.drafts.delete")}
      </button>
    </div>
  `;
}

// ---------- 发布状态面板 ----------

function renderPublishPanel(state: AppViewState): TemplateResult {
  const records = (state.mediaPublishRecords || []) as PublishRecord[];
  const loading = state.mediaPublishLoading || false;

  return html`
    <div class="card">
      <div class="card-header" style="display:flex;justify-content:space-between;align-items:center;">
        <h3 style="margin:0;font-size:15px;font-weight:600;">${t("media.publish.title")}</h3>
        <button
          class="btn btn-sm"
          ?disabled=${loading}
          @click=${() => loadPublishHistory(state)}
        >
          ${loading ? "..." : t("media.refreshStatus")}
        </button>
      </div>
      <div class="card-body">
        ${records.length === 0
          ? html`<p class="empty-hint">${t("media.publish.empty")}</p>`
          : html`
              <div style="display:flex;flex-direction:column;gap:8px;">
                ${records.map(
                  (r) => html`
                    <div
                      class="list-item"
                      style="display:flex;justify-content:space-between;align-items:center;padding:8px 12px;border-radius:6px;background:var(--bg-secondary);"
                    >
                      <div style="flex:1;min-width:0;">
                        <div style="font-weight:500;font-size:13px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">
                          ${r.title}
                        </div>
                        <div style="font-size:11px;color:var(--text-tertiary);margin-top:2px;">
                          ${r.platform} &middot; ${r.status}
                          ${r.published_at
                            ? ` &middot; ${new Date(r.published_at).toLocaleString()}`
                            : ""}
                        </div>
                      </div>
                      ${r.url
                        ? html`<a
                            href=${r.url}
                            target="_blank"
                            rel="noopener"
                            style="font-size:12px;color:var(--accent);text-decoration:none;margin-left:8px;"
                            >${t("media.publish.viewLink")}</a
                          >`
                        : nothing}
                    </div>
                  `,
                )}
              </div>
            `}
      </div>
    </div>
  `;
}
