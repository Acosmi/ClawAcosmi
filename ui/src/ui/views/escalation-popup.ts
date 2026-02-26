import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { EscalationRequest, ActiveGrant } from "../controllers/escalation.ts";

// ---------- Escalation popup view ----------

export interface EscalationPopupProps {
    visible: boolean;
    request: EscalationRequest | null;
    activeGrant: ActiveGrant | null;
    loading: boolean;
    selectedTtl: number;
    onApprove: (ttlMinutes: number) => void;
    onDeny: () => void;
    onRevoke: () => void;
    onTtlChange: (ttl: number) => void;
    onClose: () => void;
}

const TTL_OPTIONS = [
    { value: 15, label: "15 min" },
    { value: 30, label: "30 min" },
    { value: 60, label: "60 min" },
    { value: 120, label: "2 hours" },
];

const LEVEL_LABELS: Record<string, string> = {
    allowlist: "L1 — Allowlist",
    full: "L2 — Full Access",
};

const LEVEL_COLORS: Record<string, string> = {
    allowlist: "var(--color-warn, #f59e0b)",
    full: "var(--color-danger, #ef4444)",
};

// ---------- Request popup ----------

function renderRequestPopup(props: EscalationPopupProps) {
    const req = props.request!;
    const levelLabel = LEVEL_LABELS[req.requestedLevel] ?? req.requestedLevel;
    const levelColor = LEVEL_COLORS[req.requestedLevel] ?? "var(--text-secondary)";

    return html`
    <div class="escalation-overlay" @click=${(e: Event) => {
            if ((e.target as HTMLElement).classList.contains("escalation-overlay")) {
                props.onClose();
            }
        }}>
      <div class="escalation-popup">
        <div class="escalation-popup__header">
          <span class="escalation-popup__icon">🔐</span>
          <h3>${t("security.escalation.popupTitle")}</h3>
        </div>

        <div class="escalation-popup__body">
          <div class="escalation-popup__level" style="border-left: 3px solid ${levelColor}">
            <strong>${t("security.escalation.requestedLevel")}</strong>
            <span style="color: ${levelColor}">${levelLabel}</span>
          </div>

          <div class="escalation-popup__field">
            <strong>${t("security.escalation.reason")}</strong>
            <p>${req.reason || t("security.escalation.noReason")}</p>
          </div>

          ${req.runId ? html`
            <div class="escalation-popup__field">
              <strong>${t("security.escalation.runId")}</strong>
              <code>${req.runId}</code>
            </div>
          ` : nothing}

          <div class="escalation-popup__ttl">
            <strong>${t("security.escalation.ttlLabel")}</strong>
            <div class="escalation-popup__ttl-options">
              ${TTL_OPTIONS.map(opt => html`
                <button
                  class="escalation-ttl-btn ${props.selectedTtl === opt.value ? "escalation-ttl-btn--active" : ""}"
                  @click=${() => props.onTtlChange(opt.value)}
                >${opt.label}</button>
              `)}
            </div>
          </div>
        </div>

        <div class="escalation-popup__actions">
          <button
            class="escalation-btn escalation-btn--deny"
            ?disabled=${props.loading}
            @click=${() => props.onDeny()}
          >${t("security.escalation.deny")}</button>
          <button
            class="escalation-btn escalation-btn--approve"
            ?disabled=${props.loading}
            @click=${() => props.onApprove(props.selectedTtl)}
          >${props.loading ? t("common.loading") : t("security.escalation.approve")}</button>
        </div>
      </div>
    </div>
  `;
}

// ---------- Active grant banner ----------

export function renderActiveGrantBanner(props: EscalationPopupProps) {
    const grant = props.activeGrant;
    if (!grant) return nothing;

    const levelLabel = LEVEL_LABELS[grant.level] ?? grant.level;
    const levelColor = LEVEL_COLORS[grant.level] ?? "var(--text-secondary)";
    const expiresAt = grant.expiresAt ? new Date(grant.expiresAt).toLocaleTimeString() : "—";

    return html`
    <div class="escalation-banner" style="border-left: 3px solid ${levelColor}">
      <div class="escalation-banner__info">
        <span class="escalation-banner__icon">⚡</span>
        <span>${t("security.escalation.activeGrant")}: <strong style="color: ${levelColor}">${levelLabel}</strong></span>
        <span class="escalation-banner__expires">${t("security.escalation.expiresAt")}: ${expiresAt}</span>
      </div>
      <button
        class="escalation-btn escalation-btn--revoke"
        ?disabled=${props.loading}
        @click=${() => props.onRevoke()}
      >${t("security.escalation.revoke")}</button>
    </div>
  `;
}

// ---------- Entry point ----------

export function renderEscalationPopup(props: EscalationPopupProps) {
    if (props.visible && props.request) {
        return renderRequestPopup(props);
    }
    return nothing;
}
