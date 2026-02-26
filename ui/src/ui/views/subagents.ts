/**
 * subagents.ts — 子智能体管理视图
 *
 * 提供视觉智能体（Argus Screen Observer）和编程智能体（oa-coder）的
 * 前端管理界面：开关、频率、VLA 模型选择、状态显示。
 */
import { html, type TemplateResult } from "lit";
import type { OpenAcosmiApp } from "../app.ts";
import { t } from "../i18n.ts";

// ---------- 类型 ----------

interface SubAgentState {
    screenObserver: {
        enabled: boolean;
        intervalMs: number;
        vlaModel: string;
        status: "running" | "paused" | "error" | "stopped";
        frameCount: number;
        lastFrameAgo: string;
    };
    coder: {
        enabled: boolean;
        status: "connected" | "disconnected" | "error";
    };
}

// ---------- 默认状态 ----------

const defaultState: SubAgentState = {
    screenObserver: {
        enabled: false,
        intervalMs: 1000,
        vlaModel: "none",
        status: "stopped",
        frameCount: 0,
        lastFrameAgo: "-",
    },
    coder: {
        enabled: false,
        status: "disconnected",
    },
};

// 频率选项
const intervalOptions = [
    { label: "0.2s (5fps)", value: 200 },
    { label: "0.5s (2fps)", value: 500 },
    { label: "1s (1fps)", value: 1000 },
    { label: "2s", value: 2000 },
    { label: "5s", value: 5000 },
];

// VLA 模型选项
const vlaModelOptions = [
    { label: t("subagents.model.none"), value: "none" },
    { label: "Anthropic Vision", value: "anthropic" },
    { label: "ShowUI-2B", value: "showui-2b" },
    { label: "OpenCUA-7B", value: "opencua-7b" },
];

// ---------- 渲染 ----------

export function renderSubAgents(app: OpenAcosmiApp): TemplateResult {
    const state = (app as any)._subAgentState ?? defaultState;

    return html`
    <div class="view-container subagents-view">
      <div class="view-header">
        <h2>${t("subagents.title")}</h2>
        <p class="view-subtitle">${t("subagents.sub")}</p>
      </div>

      <div class="subagents-grid">
        ${renderScreenObserver(app, state.screenObserver)}
        ${renderCoder(app, state.coder)}
      </div>
    </div>
  `;
}

function renderScreenObserver(
    app: OpenAcosmiApp,
    obs: SubAgentState["screenObserver"],
): TemplateResult {
    const statusColor =
        obs.status === "running"
            ? "var(--color-success)"
            : obs.status === "error"
                ? "var(--color-error)"
                : "var(--color-muted)";

    return html`
    <div class="subagent-card">
      <div class="subagent-card__header">
        <span class="subagent-card__indicator" style="background:${statusColor}"></span>
        <h3>${t("subagents.vision.title")}</h3>
        <span class="subagent-card__status">${t(`subagents.status.${obs.status}`)}</span>
      </div>

      <div class="subagent-card__body">
        <!-- 开关 -->
        <div class="subagent-row">
          <label>${t("subagents.enable")}</label>
          <label class="toggle">
            <input
              type="checkbox"
              .checked=${obs.enabled}
              @change=${(e: Event) => {
            const checked = (e.target as HTMLInputElement).checked;
            sendSubAgentCtl(app, "argus-screen", "set_enabled", checked);
        }}
            />
            <span class="toggle__slider"></span>
          </label>
        </div>

        <!-- 频率 -->
        <div class="subagent-row">
          <label>${t("subagents.interval")}</label>
          <select
            @change=${(e: Event) => {
            const val = parseInt((e.target as HTMLSelectElement).value, 10);
            sendSubAgentCtl(app, "argus-screen", "set_interval_ms", val);
        }}
          >
            ${intervalOptions.map(
            (opt) => html`
                <option value=${opt.value} ?selected=${opt.value === obs.intervalMs}>
                  ${opt.label}
                </option>
              `,
        )}
          </select>
        </div>

        <!-- VLA 模型 -->
        <div class="subagent-row">
          <label>${t("subagents.vlaModel")}</label>
          <select
            @change=${(e: Event) => {
            const val = (e.target as HTMLSelectElement).value;
            sendSubAgentCtl(app, "argus-screen", "set_vla_model", val);
        }}
          >
            ${vlaModelOptions.map(
            (opt) => html`
                <option value=${opt.value} ?selected=${opt.value === obs.vlaModel}>
                  ${opt.label}
                </option>
              `,
        )}
          </select>
        </div>

        <!-- 统计 -->
        <div class="subagent-row subagent-row--stats">
          <span>${t("subagents.frames")}: ${obs.frameCount}</span>
          <span>${t("subagents.lastFrame")}: ${obs.lastFrameAgo}</span>
        </div>
      </div>
    </div>
  `;
}

function renderCoder(
    app: OpenAcosmiApp,
    coder: SubAgentState["coder"],
): TemplateResult {
    const statusColor =
        coder.status === "connected"
            ? "var(--color-success)"
            : coder.status === "error"
                ? "var(--color-error)"
                : "var(--color-muted)";

    return html`
    <div class="subagent-card">
      <div class="subagent-card__header">
        <span class="subagent-card__indicator" style="background:${statusColor}"></span>
        <h3>${t("subagents.coder.title")}</h3>
        <span class="subagent-card__status">${t(`subagents.status.${coder.status}`)}</span>
      </div>

      <div class="subagent-card__body">
        <div class="subagent-row">
          <label>${t("subagents.enable")}</label>
          <label class="toggle">
            <input
              type="checkbox"
              .checked=${coder.enabled}
              @change=${(e: Event) => {
            const checked = (e.target as HTMLInputElement).checked;
            sendSubAgentCtl(app, "oa-coder", "set_enabled", checked);
        }}
            />
            <span class="toggle__slider"></span>
          </label>
        </div>
      </div>
    </div>
  `;
}

// ---------- WS 控制命令 ----------

function sendSubAgentCtl(
    app: OpenAcosmiApp,
    agentId: string,
    action: string,
    value: unknown,
): void {
    const gw = (app as any).gateway;
    if (gw?.send) {
        gw.send(
            JSON.stringify({
                type: "subagent_ctl",
                payload: { agent_id: agentId, action, value },
            }),
        );
    }
}

// ---------- 加载 ----------

export function loadSubAgents(app: OpenAcosmiApp): void {
    // 初始状态（后续通过 WS 事件实时更新）
    if (!(app as any)._subAgentState) {
        (app as any)._subAgentState = { ...defaultState };
    }
}
