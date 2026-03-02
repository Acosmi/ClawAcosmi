/**
 * Media Configuration Panel — STT / DocConv / Image Understanding
 *
 * Embeds wizard-stt.ts, wizard-docconv.ts, and wizard-image.ts as expandable cards.
 * Entry: memory page "media" sub-tab + chat header capsule button.
 */
import { html, nothing } from "lit";
import type { AppViewState } from "../app-view-state.ts";
import { t } from "../i18n.ts";
import { loadSTTConfig, renderSTTWizard } from "./wizard-stt.ts";
import { loadDocConvConfig, renderDocConvWizard } from "./wizard-docconv.ts";
import { loadImageConfig, renderImageWizard } from "./wizard-image.ts";

// ─── Local expand state (simple module-level flags) ───

let sttExpanded = false;
let docconvExpanded = false;
let imageExpanded = false;

// ─── Public API ───

export async function loadMediaConfig(state: AppViewState): Promise<void> {
  await Promise.all([loadSTTConfig(state), loadDocConvConfig(state), loadImageConfig(state)]);
}

export function renderMediaConfig(state: AppViewState) {
  const sttConfigured = !!(state.sttWizard as Record<string, unknown> | undefined)?.configured;
  const docconvConfigured = !!(state.docConvWizard as Record<string, unknown> | undefined)?.configured;
  const imageConfigured = !!(state.imageWizard as Record<string, unknown> | undefined)?.configured;

  return html`
    <div style="display:flex;flex-direction:column;gap:16px;">

      <div style="display:flex;justify-content:flex-end;margin-bottom:4px;">
        <button class="btn btn--sm btn--ghost" @click=${() => { void loadMediaConfig(state); }}>
          ↻ ${t("media.refreshStatus")}
        </button>
      </div>

      <!-- STT Card -->
      <div class="card" style="padding:16px;display:flex;flex-direction:column;gap:10px;">
        <div style="display:flex;align-items:center;justify-content:space-between;">
          <div style="display:flex;align-items:center;gap:8px;">
            <span style="font-size:18px;">🎙</span>
            <span style="font-weight:600;">${t("media.stt.title")}</span>
            <span class="pill ${sttConfigured ? "success" : "warning"}" style="font-size:11px;">
              ${sttConfigured ? t("media.stt.configured") : t("media.stt.notConfigured")}
            </span>
          </div>
          <button class="btn btn--sm"
            @click=${() => {
              sttExpanded = !sttExpanded;
              if (sttExpanded) loadSTTConfig(state);
              state.requestUpdate();
            }}
          >
            ${sttExpanded ? t("media.collapse") : (sttConfigured ? t("media.reconfigure") : t("media.configure"))}
          </button>
        </div>
        <div style="color:var(--text-secondary);font-size:13px;">${t("media.stt.desc")}</div>
        ${sttExpanded ? renderSTTWizard(state) : nothing}
      </div>

      <!-- DocConv Card -->
      <div class="card" style="padding:16px;display:flex;flex-direction:column;gap:10px;">
        <div style="display:flex;align-items:center;justify-content:space-between;">
          <div style="display:flex;align-items:center;gap:8px;">
            <span style="font-size:18px;">📄</span>
            <span style="font-weight:600;">${t("media.docconv.title")}</span>
            <span class="pill ${docconvConfigured ? "success" : "warning"}" style="font-size:11px;">
              ${docconvConfigured ? t("media.docconv.configured") : t("media.docconv.notConfigured")}
            </span>
          </div>
          <button class="btn btn--sm"
            @click=${() => {
              docconvExpanded = !docconvExpanded;
              if (docconvExpanded) loadDocConvConfig(state);
              state.requestUpdate();
            }}
          >
            ${docconvExpanded ? t("media.collapse") : (docconvConfigured ? t("media.reconfigure") : t("media.configure"))}
          </button>
        </div>
        <div style="color:var(--text-secondary);font-size:13px;">${t("media.docconv.desc")}</div>
        ${docconvExpanded ? renderDocConvWizard(state) : nothing}
      </div>

      <!-- Image Understanding Card -->
      <div class="card" style="padding:16px;display:flex;flex-direction:column;gap:10px;">
        <div style="display:flex;align-items:center;justify-content:space-between;">
          <div style="display:flex;align-items:center;gap:8px;">
            <span style="font-size:18px;">🖼</span>
            <span style="font-weight:600;">${t("media.image.title")}</span>
            <span class="pill ${imageConfigured ? "success" : "warning"}" style="font-size:11px;">
              ${imageConfigured ? t("media.image.configured") : t("media.image.notConfigured")}
            </span>
          </div>
          <button class="btn btn--sm"
            @click=${() => {
              imageExpanded = !imageExpanded;
              if (imageExpanded) loadImageConfig(state);
              state.requestUpdate();
            }}
          >
            ${imageExpanded ? t("media.collapse") : (imageConfigured ? t("media.reconfigure") : t("media.configure"))}
          </button>
        </div>
        <div style="color:var(--text-secondary);font-size:13px;">${t("media.image.desc")}</div>
        ${imageExpanded ? renderImageWizard(state) : nothing}
      </div>

    </div>
  `;
}
