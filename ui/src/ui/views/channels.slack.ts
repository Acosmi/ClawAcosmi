import { html, nothing } from "lit";
import type { SlackStatus } from "../types.ts";
import type { ChannelsProps } from "./channels.types.ts";
import { formatRelativeTimestamp } from "../format.ts";
import { t } from "../i18n.ts";

export function renderSlackCard(params: {
  props: ChannelsProps;
  slack?: SlackStatus | null;
  accountCountLabel: unknown;
}) {
  const { props, slack, accountCountLabel } = params;

  return html`
    ${accountCountLabel}

    <div class="status-list" style="margin-top: 8px;">
      <div>
        <span class="label">${t("channels.configured")}</span>
        <span>${slack?.configured ? t("channels.yes") : t("channels.no")}</span>
      </div>
      <div>
        <span class="label">${t("channels.running")}</span>
        <span>${slack?.running ? t("channels.yes") : t("channels.no")}</span>
      </div>
      <div>
        <span class="label">${t("channels.lastStart")}</span>
        <span>${slack?.lastStartAt ? formatRelativeTimestamp(slack.lastStartAt) : "n/a"}</span>
      </div>
      <div>
        <span class="label">${t("channels.lastProbe")}</span>
        <span>${slack?.lastProbeAt ? formatRelativeTimestamp(slack.lastProbeAt) : "n/a"}</span>
      </div>
    </div>

    ${slack?.lastError
      ? html`<div class="callout danger" style="margin-top: 12px;">
          ${slack.lastError}
        </div>`
      : nothing
    }

    ${slack?.probe
      ? html`<div class="callout" style="margin-top: 12px;">
          Probe ${slack.probe.ok ? "ok" : "failed"} ·
          ${slack.probe.status ?? ""} ${slack.probe.error ?? ""}
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
