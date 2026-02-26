import { html, nothing } from "lit";
import type { SecurityLevelInfo } from "../controllers/security.ts";
import { t } from "../i18n.ts";
import { icons } from "../icons.ts";

// ---------- 安全设置页面入口 ----------

export interface SecurityViewProps {
    loading: boolean;
    error: string | null;
    currentLevel: string;
    levels: SecurityLevelInfo[];
    confirmOpen: boolean;
    pendingLevel: string | null;
    confirmText: string;
    onLevelChange: (level: string) => void;
    onConfirmOpen: (level: string) => void;
    onConfirmCancel: () => void;
    onConfirmTextChange: (text: string) => void;
    onConfirmSubmit: () => void;
    onRefresh: () => void;
}

const RISK_COLORS: Record<string, string> = {
    low: "var(--color-ok, #22c55e)",
    medium: "var(--color-warn, #f59e0b)",
    high: "var(--color-danger, #ef4444)",
};

const RISK_ICONS: Record<string, string> = {
    low: "🛡️",
    medium: "⚡",
    high: "⚠️",
};

function renderLevelCard(
    level: SecurityLevelInfo,
    isCurrent: boolean,
    props: SecurityViewProps,
) {
    const riskColor = RISK_COLORS[level.risk] ?? "var(--text-secondary)";
    const riskIcon = RISK_ICONS[level.risk] ?? "";
    const isLoading = props.loading;

    return html`
    <button
      class="security-card ${isCurrent ? "security-card--active" : ""}"
      ?disabled=${isLoading}
      @click=${() => {
            if (isCurrent || isLoading) return;
            if (level.id === "full") {
                props.onConfirmOpen(level.id);
            } else {
                props.onLevelChange(level.id);
            }
        }}
      style="--risk-color: ${riskColor}"
    >
      <div class="security-card__header">
        <span class="security-card__icon">${riskIcon}</span>
        <span class="security-card__label">${level.label}</span>
        ${isCurrent
            ? html`<span class="security-card__badge">${t("security.active")}</span>`
            : nothing}
      </div>
      <div class="security-card__desc">${level.description}</div>
      <div class="security-card__risk">
        <span
          class="security-card__risk-dot"
          style="background: ${riskColor}"
        ></span>
        ${t(`security.risk.${level.risk}`)}
      </div>
    </button>
  `;
}

function renderConfirmDialog(props: SecurityViewProps) {
    if (!props.confirmOpen) return nothing;

    const isValid =
        props.confirmText.trim().toUpperCase() === "CONFIRM";

    return html`
    <div class="security-confirm-overlay" @click=${(e: Event) => {
            if ((e.target as HTMLElement).classList.contains("security-confirm-overlay")) {
                props.onConfirmCancel();
            }
        }}>
      <div class="security-confirm-dialog" role="alertdialog">
        <div class="security-confirm-header">
          <span class="security-confirm-icon">⚠️</span>
          <h3>${t("security.confirm.title")}</h3>
        </div>
        <div class="security-confirm-body">
          <p>${t("security.confirm.warning")}</p>
          <ul class="security-confirm-risks">
            <li>${t("security.confirm.risk1")}</li>
            <li>${t("security.confirm.risk2")}</li>
            <li>${t("security.confirm.risk3")}</li>
          </ul>
          <div class="security-confirm-input-group">
            <label for="security-confirm-input">${t("security.confirm.prompt")}</label>
            <input
              id="security-confirm-input"
              type="text"
              class="security-confirm-input"
              placeholder="CONFIRM"
              .value=${props.confirmText}
              @input=${(e: Event) =>
            props.onConfirmTextChange((e.target as HTMLInputElement).value)}
              @keydown=${(e: KeyboardEvent) => {
            if (e.key === "Enter" && isValid) {
                props.onConfirmSubmit();
            }
        }}
            />
          </div>
        </div>
        <div class="security-confirm-actions">
          <button
            class="btn"
            @click=${props.onConfirmCancel}
          >
            ${t("security.confirm.cancel")}
          </button>
          <button
            class="btn danger"
            ?disabled=${!isValid || props.loading}
            @click=${props.onConfirmSubmit}
          >
            ${t("security.confirm.submit")}
          </button>
        </div>
      </div>
    </div>
  `;
}

export function renderSecurity(props: SecurityViewProps) {
    return html`
    <div class="security-page">
      ${props.currentLevel === "full"
            ? html`
            <div class="security-warning-banner">
              <span class="security-warning-banner__icon">⚠️</span>
              <span>${t("security.fullWarning")}</span>
            </div>
          `
            : nothing}

      <div class="security-section">
        <div class="security-section__header">
          <h2 class="security-section__title">
            <span class="security-section__title-icon">${icons.settings}</span>
            ${t("security.levels.title")}
          </h2>
          <button
            class="btn btn--sm"
            ?disabled=${props.loading}
            @click=${props.onRefresh}
          >
            ${props.loading ? t("security.loading") : t("security.refresh")}
          </button>
        </div>
        <p class="security-section__desc">${t("security.levels.desc")}</p>

        ${props.error
            ? html`<div class="security-error">${props.error}</div>`
            : nothing}

        <div class="security-cards">
          ${props.levels.map((level) =>
                renderLevelCard(level, level.id === props.currentLevel, props),
            )}
        </div>
      </div>

      <div class="security-section">
        <h2 class="security-section__title">${t("security.info.title")}</h2>
        <div class="security-info-grid">
          <div class="security-info-card">
            <h4>${t("security.info.whatIs.title")}</h4>
            <p>${t("security.info.whatIs.body")}</p>
          </div>
          <div class="security-info-card">
            <h4>${t("security.info.recommendation.title")}</h4>
            <p>${t("security.info.recommendation.body")}</p>
          </div>
        </div>
      </div>

      ${renderConfirmDialog(props)}
    </div>
  `;
}
