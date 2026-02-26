import { html, nothing } from "lit";
import type { GoogleChatStatus } from "../types.ts";
import type { ChannelsProps } from "./channels.types.ts";
import { formatRelativeTimestamp } from "../format.ts";
import { t } from "../i18n.ts";

export function renderGoogleChatCard(params: {
  props: ChannelsProps;
  googleChat?: GoogleChatStatus | null;
  accountCountLabel: unknown;
}) {
  const { props, googleChat, accountCountLabel } = params;

  return html`
    ${accountCountLabel}

    <div class="status-list" style="margin-top: 8px;">
      <div>
        <span class="label">${t("channels.configured")}</span>
        <span>${googleChat ? (googleChat.configured ? t("channels.yes") : t("channels.no")) : "n/a"}</span>
      </div>
      <div>
        <span class="label">${t("channels.running")}</span>
        <span>${googleChat ? (googleChat.running ? t("channels.yes") : t("channels.no")) : "n/a"}</span>
      </div>
      <div>
        <span class="label">Credential</span>
        <span>${googleChat?.credentialSource ?? "n/a"}</span>
      </div>
      <div>
        <span class="label">Audience</span>
        <span>
          ${googleChat?.audienceType
      ? `${googleChat.audienceType}${googleChat.audience ? ` · ${googleChat.audience}` : ""}`
      : "n/a"
    }
        </span>
      </div>
      <div>
        <span class="label">${t("channels.lastStart")}</span>
        <span>${googleChat?.lastStartAt ? formatRelativeTimestamp(googleChat.lastStartAt) : "n/a"}</span>
      </div>
      <div>
        <span class="label">${t("channels.lastProbe")}</span>
        <span>${googleChat?.lastProbeAt ? formatRelativeTimestamp(googleChat.lastProbeAt) : "n/a"}</span>
      </div>
    </div>

    ${googleChat?.lastError
      ? html`<div class="callout danger" style="margin-top: 12px;">
          ${googleChat.lastError}
        </div>`
      : nothing
    }

    ${googleChat?.probe
      ? html`<div class="callout" style="margin-top: 12px;">
          Probe ${googleChat.probe.ok ? "ok" : "failed"} ·
          ${googleChat.probe.status ?? ""} ${googleChat.probe.error ?? ""}
        </div>`
      : nothing
    }

    <div class="row" style="margin-top: 12px;">
      <button class="btn" @click=${() => props.onRefresh(true)}>
        Probe
      </button>
    </div>
  `;
}
