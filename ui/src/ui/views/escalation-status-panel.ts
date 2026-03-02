import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { EscalationState } from "../controllers/escalation.ts";

export interface EscalationStatusPanelProps {
    state: EscalationState;
    selectedTtl: number;
    onRevoke: () => void;
    loading: boolean;
}

function formatCountdown(expiresAt: string): string {
    if (!expiresAt) return "—";
    const remaining = new Date(expiresAt).getTime() - Date.now();
    if (remaining <= 0) return "expired";
    const totalSeconds = Math.floor(remaining / 1000);
    const m = Math.floor(totalSeconds / 60);
    const s = totalSeconds % 60;
    if (m >= 60) {
        const h = Math.floor(m / 60);
        return `${h}h ${m % 60}m`;
    }
    return `${m}m ${String(s).padStart(2, "0")}s`;
}

function ttlProgress(expiresAt: string, grantedAt: string): number {
    if (!expiresAt || !grantedAt) return 0;
    const total = new Date(expiresAt).getTime() - new Date(grantedAt).getTime();
    if (total <= 0) return 0;
    const remaining = new Date(expiresAt).getTime() - Date.now();
    return Math.max(0, Math.min(1, remaining / total));
}

const LEVEL_LABELS: Record<string, string> = {
    deny: "L0 — Read Only",
    allowlist: "L1 — Allowlist",
    sandboxed: "L2 — Sandboxed Full",
    full: "L3 — Bare Machine Full",
};

export function renderEscalationStatusPanel(props: EscalationStatusPanelProps) {
    const { state } = props;
    const hasPending = state.popupVisible && state.request !== null;
    const hasActive = state.activeGrant !== null;

    if (!hasPending && !hasActive) return nothing;

    return html`
    <div class="escalation-status-panel">
      <h3 class="escalation-status-panel__title">
        <span>🔐</span>
        ${t("security.escalation.statusTitle")}
      </h3>

      ${hasPending && state.request ? html`
        <div class="escalation-status-item escalation-status-item--pending">
          <div class="escalation-status-item__icon">⏳</div>
          <div class="escalation-status-item__content">
            <div class="escalation-status-item__label">${t("security.escalation.pendingRequest")}</div>
            <div class="escalation-status-item__detail">
              ${t("security.escalation.requestedLevel")}: <strong>${LEVEL_LABELS[state.request.requestedLevel] ?? state.request.requestedLevel}</strong>
            </div>
            <div class="escalation-status-item__detail">
              ${t("security.escalation.reason")}: ${state.request.reason || "—"}
            </div>
            <div class="escalation-status-item__detail escalation-status-item__detail--muted">
              ${t("security.escalation.awaitingApproval")}
            </div>
          </div>
        </div>
      ` : nothing}

      ${hasActive && state.activeGrant ? (() => {
        const grant = state.activeGrant!;
        const countdown = formatCountdown(grant.expiresAt);
        const progress = ttlProgress(grant.expiresAt, grant.grantedAt);
        const pct = Math.round(progress * 100);
        return html`
          <div class="escalation-status-item escalation-status-item--active">
            <div class="escalation-status-item__icon">⚡</div>
            <div class="escalation-status-item__content">
              <div class="escalation-status-item__label">${t("security.escalation.activeGrant")}</div>
              <div class="escalation-status-item__detail">
                ${t("security.escalation.requestedLevel")}: <strong>${LEVEL_LABELS[grant.level] ?? grant.level}</strong>
              </div>
              ${grant.expiresAt ? html`
                <div class="escalation-status-ttl">
                  <span class="escalation-status-ttl__text">${t("security.escalation.expiresIn")}: ${countdown}</span>
                  <div class="escalation-status-ttl__bar">
                    <div class="escalation-status-ttl__fill" style="width: ${pct}%"></div>
                  </div>
                </div>
              ` : nothing}
              <button
                class="escalation-btn escalation-btn--revoke"
                ?disabled=${props.loading}
                @click=${() => props.onRevoke()}
              >${t("security.escalation.revoke")}</button>
            </div>
          </div>
        `;
      })() : nothing}
    </div>
  `;
}
