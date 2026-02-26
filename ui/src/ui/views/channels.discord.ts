import { html, nothing } from "lit";
import type { DiscordStatus } from "../types.ts";
import type { ChannelsProps } from "./channels.types.ts";
import { formatRelativeTimestamp } from "../format.ts";
import { t } from "../i18n.ts";

export function renderDiscordCard(params: {
  props: ChannelsProps;
  discord?: DiscordStatus | null;
  accountCountLabel: unknown;
}) {
  const { props, discord, accountCountLabel } = params;

  return html`
    ${accountCountLabel}

    <div class="status-list" style="margin-top: 8px;">
      <div>
        <span class="label">${t("channels.configured")}</span>
        <span>${discord?.configured ? t("channels.yes") : t("channels.no")}</span>
      </div>
      <div>
        <span class="label">${t("channels.running")}</span>
        <span>${discord?.running ? t("channels.yes") : t("channels.no")}</span>
      </div>
      <div>
        <span class="label">${t("channels.lastStart")}</span>
        <span>${discord?.lastStartAt ? formatRelativeTimestamp(discord.lastStartAt) : "n/a"}</span>
      </div>
      <div>
        <span class="label">${t("channels.lastProbe")}</span>
        <span>${discord?.lastProbeAt ? formatRelativeTimestamp(discord.lastProbeAt) : "n/a"}</span>
      </div>
    </div>

    ${discord?.lastError
      ? html`<div class="callout danger" style="margin-top: 12px;">
          ${discord.lastError}
        </div>`
      : nothing
    }

    ${discord?.probe
      ? html`<div class="callout" style="margin-top: 12px;">
          Probe ${discord.probe.ok ? "ok" : "failed"} ·
          ${discord.probe.status ?? ""} ${discord.probe.error ?? ""}
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
