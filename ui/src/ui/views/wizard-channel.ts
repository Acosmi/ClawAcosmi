/**
 * Channel Setup Wizard — Overlay for configuring Chinese channels
 *
 * Reuses the existing wizard CSS classes (wizard-overlay, wizard-card, etc.)
 * 3-step flow: Platform → Credentials → Confirm
 *
 * Triggered from the channels settings page.
 */
import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { AppViewState } from "../app-view-state.ts";

// ─── Types ───

interface ChannelWizardState {
    open: boolean;
    step: number; // 0=select, 1=credentials, 2=confirm
    selectedPlatform: string | null;
    fields: Record<string, string>;
    saving: boolean;
    error: string | null;
    done: boolean;
    skippedPlatformSelect: boolean; // true when opened via channel shortcut (skip step 0)
}

export const CHANNEL_WIZARD_INITIAL: ChannelWizardState = {
    open: false,
    step: 0,
    selectedPlatform: null,
    fields: {},
    saving: false,
    error: null,
    done: false,
    skippedPlatformSelect: false,
};

// ─── Platform definitions ───

const CHANNEL_PLATFORMS = [
    {
        id: "feishu",
        icon: "🐦",
        label: "channels.feishu.name",
        hint: "channels.feishu.description",
        fields: [
            { key: "appId", label: "wizard.channel.appId", placeholder: "cli_xxxxx", sensitive: false, required: true },
            { key: "appSecret", label: "wizard.channel.appSecret", placeholder: "", sensitive: true, required: true },
            { key: "verifyToken", label: "wizard.channel.verifyToken", placeholder: "", sensitive: true, required: false },
            { key: "encryptKey", label: "wizard.channel.encryptKey", placeholder: "", sensitive: true, required: false },
        ],
    },
    {
        id: "dingtalk",
        icon: "💬",
        label: "channels.dingtalk.name",
        hint: "channels.dingtalk.description",
        fields: [
            { key: "appKey", label: "wizard.channel.appKey", placeholder: "dingXXXXX", sensitive: false, required: true },
            { key: "appSecret", label: "wizard.channel.appSecret", placeholder: "", sensitive: true, required: true },
            { key: "robotCode", label: "wizard.channel.robotCode", placeholder: "", sensitive: false, required: false },
        ],
    },
    {
        id: "wecom",
        icon: "💼",
        label: "channels.wecom.name",
        hint: "channels.wecom.description",
        fields: [
            { key: "corpId", label: "wizard.channel.corpId", placeholder: "ww_xxxxx", sensitive: false, required: true },
            { key: "corpSecret", label: "wizard.channel.corpSecret", placeholder: "", sensitive: true, required: true },
            { key: "agentId", label: "wizard.channel.agentId", placeholder: "1000002", sensitive: false, required: true },
            { key: "token", label: "wizard.channel.callbackToken", placeholder: "", sensitive: true, required: false },
            { key: "encodingAESKey", label: "wizard.channel.encodingAESKey", placeholder: "", sensitive: true, required: false },
        ],
    },
];

// ─── Controller ───

function getWizardState(state: AppViewState): ChannelWizardState {
    return (state as any).channelWizardState ?? CHANNEL_WIZARD_INITIAL;
}

function setWizardState(state: AppViewState, ws: Partial<ChannelWizardState>) {
    (state as any).channelWizardState = { ...getWizardState(state), ...ws };
    // Trigger Lit re-render since channelWizardState is not a @state() property
    if (typeof (state as any).requestUpdate === "function") {
        (state as any).requestUpdate();
    }
}

export function openChannelWizard(state: AppViewState) {
    setWizardState(state, { ...CHANNEL_WIZARD_INITIAL, open: true });
}

export function closeChannelWizard(state: AppViewState) {
    setWizardState(state, { open: false });
}

function selectPlatform(state: AppViewState, platformId: string) {
    setWizardState(state, { selectedPlatform: platformId, fields: {} });
}

function updateField(state: AppViewState, key: string, value: string) {
    const ws = getWizardState(state);
    setWizardState(state, { fields: { ...ws.fields, [key]: value } });
}

function nextStep(state: AppViewState) {
    const ws = getWizardState(state);
    setWizardState(state, { step: ws.step + 1 });
}

function prevStep(state: AppViewState) {
    const ws = getWizardState(state);
    if (ws.step > 0) setWizardState(state, { step: ws.step - 1 });
}

async function saveChannelConfig(state: AppViewState) {
    const ws = getWizardState(state);
    if (!ws.selectedPlatform) return;
    setWizardState(state, { saving: true, error: null });

    try {
        const client = (state as any).client;
        if (!client) {
            throw new Error("Gateway not connected");
        }
        await client.request("channels.save", {
            channelId: ws.selectedPlatform,
            config: ws.fields,
        });
        setWizardState(state, { saving: false, done: true, step: 3 });
    } catch (err: any) {
        setWizardState(state, { saving: false, error: err?.message ?? String(err) });
    }
}

// ─── Render ───

export function renderChannelWizard(state: AppViewState) {
    const ws = getWizardState(state);
    if (!ws.open) return nothing;

    return html`
    <div class="wizard-overlay" @click=${(e: Event) => {
            if (e.target === e.currentTarget) {
                closeChannelWizard(state);
            }
        }}>
      <div class="wizard-card">
        <button class="wizard-close-btn" @click=${() => closeChannelWizard(state)} title="${t("wizard.close")}">✕</button>
        ${renderChannelStepDots(ws)}
        ${ws.saving ? renderChannelLoading()
            : ws.done ? renderChannelDone(state)
                : ws.error ? renderChannelError(state, ws)
                    : ws.step === 0 ? renderPlatformSelect(state, ws)
                        : ws.step === 1 ? renderCredentials(state, ws)
                            : ws.step === 2 ? renderConfirm(state, ws)
                                : renderChannelLoading()}
      </div>
    </div>
  `;
}

function renderChannelStepDots(ws: ChannelWizardState) {
    if (ws.done) return nothing;
    const totalSteps = 3;
    return html`
    <div class="wizard-steps">
      ${Array.from({ length: totalSteps }, (_, i) => {
        const cls = i === ws.step
            ? "wizard-step-dot wizard-step-dot--active"
            : i < ws.step
                ? "wizard-step-dot wizard-step-dot--done"
                : "wizard-step-dot";
        return html`<div class="${cls}"></div>`;
    })}
    </div>
  `;
}

function renderChannelLoading() {
    return html`
    <div class="wizard-loading">
      <div class="wizard-spinner"></div>
      <div class="wizard-loading-text">${t("wizard.channel.saving")}</div>
    </div>
  `;
}

function renderChannelDone(state: AppViewState) {
    return html`
    <div class="wizard-done">
      <div class="wizard-done-check">✓</div>
      <div class="wizard-done-title">${t("wizard.channel.configComplete")}</div>
      <div class="wizard-done-sub">${t("wizard.channel.configCompleteSub")}</div>
      <div class="wizard-actions" style="justify-content: center;">
        <button class="wizard-btn wizard-btn--primary" @click=${() => closeChannelWizard(state)}>
          ${t("wizard.getStarted")}
        </button>
      </div>
    </div>
  `;
}

function renderChannelError(state: AppViewState, ws: ChannelWizardState) {
    return html`
    <div class="wizard-icon">⚠️</div>
    <div class="wizard-title">${t("wizard.setupError")}</div>
    <div class="wizard-error">${ws.error ?? t("wizard.unexpectedError")}</div>
    <div class="wizard-actions">
      <button class="wizard-btn wizard-btn--secondary" @click=${() => closeChannelWizard(state)}>
        ${t("wizard.close")}
      </button>
      <button class="wizard-btn wizard-btn--primary" @click=${() => {
            setWizardState(state, { error: null, step: 1 });
        }}>
        ${t("wizard.retry")}
      </button>
    </div>
  `;
}

// ─── Step 0: Platform Select ───

const STEP_ICONS_CHANNEL = ["📡", "🔑", "✅"];

function renderPlatformSelect(state: AppViewState, ws: ChannelWizardState) {
    return html`
    <div class="wizard-icon">${STEP_ICONS_CHANNEL[0]}</div>
    <div class="wizard-title">${t("wizard.channel.choosePlatform")}</div>
    <div class="wizard-subtitle">${t("wizard.channel.choosePlatformSub")}</div>
    <div class="wizard-options">
      ${CHANNEL_PLATFORMS.map((p) => {
        const isSelected = ws.selectedPlatform === p.id;
        return html`
          <button
            class="wizard-option ${isSelected ? "wizard-option--selected" : ""}"
            @click=${() => selectPlatform(state, p.id)}
          >
            <div class="wizard-option__icon">${p.icon}</div>
            <div class="wizard-option__text">
              <div class="wizard-option__label">${t(p.label)}</div>
              <div class="wizard-option__hint">${t(p.hint)}</div>
            </div>
          </button>
        `;
    })}
    </div>
    <div class="wizard-actions">
      <button class="wizard-btn wizard-btn--danger" @click=${() => closeChannelWizard(state)}>
        ${t("wizard.cancel")}
      </button>
      <button
        class="wizard-btn wizard-btn--primary"
        ?disabled=${!ws.selectedPlatform}
        @click=${() => { if (ws.selectedPlatform) nextStep(state); }}
      >
        ${t("wizard.continue")}
      </button>
    </div>
  `;
}

// ─── Step 1: Credentials ───

function renderCredentials(state: AppViewState, ws: ChannelWizardState) {
    const platform = CHANNEL_PLATFORMS.find((p) => p.id === ws.selectedPlatform);
    if (!platform) return nothing;

    return html`
    <div class="wizard-icon">${STEP_ICONS_CHANNEL[1]}</div>
    <div class="wizard-title">${t("wizard.channel.enterCredentials")}</div>
    <div class="wizard-subtitle">${t(platform.label)}</div>
    ${platform.fields.map((f) => html`
      <div class="wizard-input-group">
        <label class="wizard-input-label">${t(f.label)}${f.required === false ? html` <span class="wizard-optional-tag">(${t("wizard.channel.optional")})</span>` : nothing}</label>
        <input
          class="wizard-input"
          type="${f.sensitive ? "password" : "text"}"
          placeholder="${f.placeholder}"
          .value=${ws.fields[f.key] ?? ""}
          @input=${(e: Event) => updateField(state, f.key, (e.target as HTMLInputElement).value)}
        />
      </div>
    `)}
    <div class="wizard-actions">
      <button class="wizard-btn wizard-btn--secondary" @click=${() => {
            if (ws.skippedPlatformSelect) { closeChannelWizard(state); } else { prevStep(state); }
        }}>
        ${ws.skippedPlatformSelect ? t("wizard.close") : t("wizard.channel.back")}
      </button>
      <button
        class="wizard-btn wizard-btn--primary"
        ?disabled=${!platform.fields.filter((f) => f.required !== false).every((f) => (ws.fields[f.key] ?? "").trim())}
        @click=${() => nextStep(state)}
      >
        ${t("wizard.continue")}
      </button>
    </div>
  `;
}

// ─── Step 2: Confirm ───

function renderConfirm(state: AppViewState, ws: ChannelWizardState) {
    const platform = CHANNEL_PLATFORMS.find((p) => p.id === ws.selectedPlatform);
    if (!platform) return nothing;

    return html`
    <div class="wizard-icon">${STEP_ICONS_CHANNEL[2]}</div>
    <div class="wizard-title">${t("wizard.channel.confirmTitle")}</div>
    <div class="wizard-note">
      <strong>${t(platform.label)}</strong><br/>
      ${platform.fields.map((f) => html`
        <div style="margin: 4px 0; font-size: 13px; color: var(--text-secondary, #888);">
          ${t(f.label)}: ${f.sensitive ? "••••••" : ws.fields[f.key] ?? ""}
        </div>
      `)}
    </div>
    <div class="wizard-actions">
      <button class="wizard-btn wizard-btn--secondary" @click=${() => prevStep(state)}>
        ${t("wizard.channel.back")}
      </button>
      <button class="wizard-btn wizard-btn--primary" @click=${() => void saveChannelConfig(state)}>
        ${t("wizard.channel.save")}
      </button>
    </div>
  `;
}
