import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { GatewayBrowserClient } from "../gateway.ts";

// ---------- Types (mirrors backend EscalationAuditEntry) ----------

export type AuditEntry = {
    timestamp: string;
    event: string;
    requestId: string;
    requestedLevel?: string;
    reason?: string;
    runId?: string;
    sessionId?: string;
    ttlMinutes?: number;
};

export type EscalationAuditState = {
    loading: boolean;
    entries: AuditEntry[];
    error: string | null;
};

export function createAuditState(): EscalationAuditState {
    return { loading: false, entries: [], error: null };
}

// ---------- RPC ----------

export async function loadEscalationAudit(
    client: GatewayBrowserClient,
    limit = 50,
): Promise<{ entries: AuditEntry[]; total: number }> {
    return client.request<{ entries: AuditEntry[]; total: number }>(
        "security.escalation.audit",
        { limit },
    );
}

// ---------- View ----------

function eventLabel(event: string): string {
    const key = `security.escalation.event.${event}`;
    const translated = t(key);
    return translated !== key ? translated : event;
}

const EVENT_ICONS: Record<string, string> = {
    request: "📋",
    approve: "✅",
    deny: "❌",
    expire: "⏰",
    task_complete: "🏁",
    manual_revoke: "🔒",
};

const LEVEL_SHORT: Record<string, string> = {
    allowlist: "L1",
    sandboxed: "L2",
    full: "L3",
};

function formatTimestamp(ts: string): string {
    try {
        return new Date(ts).toLocaleString(undefined, {
            month: "short",
            day: "numeric",
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
        });
    } catch {
        return ts;
    }
}

export interface EscalationAuditProps {
    state: EscalationAuditState;
    onRefresh: () => void;
}

export function renderEscalationAuditLog(props: EscalationAuditProps) {
    const { state } = props;

    return html`
    <div class="escalation-audit">
      <div class="escalation-audit__header">
        <h3 class="escalation-audit__title">
          <span>📜</span>
          ${t("security.escalation.auditTitle")}
        </h3>
        <button
          class="btn btn--sm"
          ?disabled=${state.loading}
          @click=${() => props.onRefresh()}
        >${state.loading ? t("common.loading") : t("security.refresh")}</button>
      </div>

      ${state.error
        ? html`<div class="escalation-audit__error">${state.error}</div>`
        : nothing}

      ${state.entries.length === 0 && !state.loading
        ? html`<div class="escalation-audit__empty">${t("security.escalation.noAuditEntries")}</div>`
        : nothing}

      ${state.entries.length > 0 ? html`
        <div class="escalation-audit__table-wrap">
          <table class="escalation-audit__table">
            <thead>
              <tr>
                <th>${t("security.escalation.auditTime")}</th>
                <th>${t("security.escalation.auditEvent")}</th>
                <th>${t("security.escalation.requestedLevel")}</th>
                <th>${t("security.escalation.reason")}</th>
                <th>TTL</th>
              </tr>
            </thead>
            <tbody>
              ${state.entries.map(entry => html`
                <tr class="escalation-audit__row escalation-audit__row--${entry.event}">
                  <td class="escalation-audit__cell--time">${formatTimestamp(entry.timestamp)}</td>
                  <td>
                    <span class="escalation-audit__event-badge">
                      ${EVENT_ICONS[entry.event] ?? "•"}
                      ${eventLabel(entry.event)}
                    </span>
                  </td>
                  <td>${LEVEL_SHORT[entry.requestedLevel ?? ""] ?? entry.requestedLevel ?? "—"}</td>
                  <td class="escalation-audit__cell--reason">${entry.reason || "—"}</td>
                  <td>${entry.ttlMinutes ? `${entry.ttlMinutes}m` : "—"}</td>
                </tr>
              `)}
            </tbody>
          </table>
        </div>
      ` : nothing}
    </div>
  `;
}
