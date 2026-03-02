/**
 * Setup Wizard — Full-screen overlay view
 *
 * Apple-style 3-step onboarding: Provider → API Key → Model
 * Communicates with backend via wizard.start / wizard.next / wizard.cancel RPC
 */
import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { AppViewState } from "../app-view-state.ts";

// ─── Types ───

export type WizardStepOption = {
  value: string;
  label: string;
  hint?: string;
};

export type WizardStep = {
  id: string;
  type: "note" | "select" | "text" | "confirm";
  message: string;
  title?: string;
  options?: WizardStepOption[];
  initialValue?: unknown;
  placeholder?: string;
  sensitive?: boolean;
};

export type WizardState = {
  sessionId: string | null;
  step: WizardStep | null;
  status: "idle" | "running" | "done" | "cancelled" | "error";
  loading: boolean;
  error: string | null;
  selectedValue: unknown;
  totalSteps: number;
  currentStepIndex: number;
};

export const WIZARD_INITIAL_STATE: WizardState = {
  sessionId: null,
  step: null,
  status: "idle",
  loading: false,
  error: null,
  selectedValue: null,
  totalSteps: 3,
  currentStepIndex: 0,
};

// ─── Provider icons ───

const PROVIDER_ICONS: Record<string, string> = {
  anthropic: "🟣",
  openai: "🟢",
  deepseek: "🔵",
  google: "🔴",
  openrouter: "🟠",
  groq: "⚡",
  ollama: "🦙",
};

function getProviderIcon(value: string): string {
  const lower = value.toLowerCase();
  for (const [key, icon] of Object.entries(PROVIDER_ICONS)) {
    if (lower.includes(key)) return icon;
  }
  return "🤖";
}

// ─── Step icons per index ───
const STEP_ICONS = ["🏢", "🔑", "🧠"];
const STEP_TITLES_FALLBACK_KEYS = ["wizard.chooseProvider", "wizard.apiKey", "wizard.selectModel"];

// ─── Controller functions ───

export async function startWizard(state: AppViewState, mode?: string): Promise<void> {
  const client = state.client;
  if (!client) return;

  state.wizardState = {
    ...WIZARD_INITIAL_STATE,
    loading: true,
    status: "running",
  };
  state.wizardOpen = true;

  try {
    const params: Record<string, unknown> = {};
    if (mode) params.mode = mode;

    const result = await client.request<{
      sessionId: string;
      step?: WizardStep;
      done?: boolean;
    }>("wizard.start", params);

    state.wizardState = {
      ...state.wizardState,
      sessionId: result.sessionId,
      step: result.step ?? null,
      loading: false,
      status: result.done ? "done" : "running",
      currentStepIndex: 0,
    };
  } catch (err) {
    state.wizardState = {
      ...state.wizardState,
      loading: false,
      error: err instanceof Error ? err.message : String(err),
      status: "error",
    };
  }
}

export async function startOpenCoderWizard(state: AppViewState): Promise<void> {
  return startWizard(state, "open-coder");
}

export async function wizardNext(
  state: AppViewState,
  answer: unknown,
): Promise<void> {
  const client = state.client;
  const ws = state.wizardState;
  if (!client || !ws.sessionId || !ws.step) return;

  state.wizardState = { ...ws, loading: true, error: null };

  try {
    const result = await client.request<{
      step?: WizardStep;
      done?: boolean;
      status?: string;
    }>("wizard.next", {
      sessionId: ws.sessionId,
      answer: { stepId: ws.step.id, value: answer },
    });

    if (result.done) {
      state.wizardState = {
        ...state.wizardState,
        step: null,
        loading: false,
        status: "done",
        currentStepIndex: ws.totalSteps,
      };
    } else {
      state.wizardState = {
        ...state.wizardState,
        step: result.step ?? null,
        loading: false,
        status: "running",
        selectedValue: null,
        currentStepIndex: Math.min(ws.currentStepIndex + 1, ws.totalSteps - 1),
      };
    }
  } catch (err) {
    state.wizardState = {
      ...state.wizardState,
      loading: false,
      error: err instanceof Error ? err.message : String(err),
      status: "error",
    };
  }
}

export async function cancelWizard(state: AppViewState): Promise<void> {
  const client = state.client;
  const ws = state.wizardState;
  if (!client || !ws.sessionId) {
    state.wizardOpen = false;
    state.wizardState = { ...WIZARD_INITIAL_STATE };
    return;
  }

  try {
    await client.request("wizard.cancel", { sessionId: ws.sessionId });
  } catch {
    // ignore
  }
  state.wizardOpen = false;
  state.wizardState = { ...WIZARD_INITIAL_STATE };
}

export function closeWizard(state: AppViewState): void {
  const wasDone = state.wizardState.status === "done";
  state.wizardOpen = false;
  state.wizardState = { ...WIZARD_INITIAL_STATE };

  // wizard 完成后自动刷新 subagents 数据（配置变更后需要重新加载）
  if (wasDone) {
    // 延迟 300ms 确保后端配置写入 + 缓存清除已完成
    setTimeout(() => {
      import("../controllers/subagents.ts").then((m) => m.loadSubAgents(state as any));
    }, 300);
  }
}

// ─── Render ───

export function renderWizard(state: AppViewState) {
  if (!state.wizardOpen) return nothing;

  const ws = state.wizardState;

  return html`
    <div class="wizard-overlay" @click=${(e: Event) => {
      // Close on backdrop click only if done/error
      if (e.target === e.currentTarget && (ws.status === "done" || ws.status === "error")) {
        closeWizard(state);
      }
    }}>
      <div class="wizard-card">
        ${renderStepDots(ws)}
        ${ws.loading ? renderLoading()
      : ws.status === "done" ? renderDone(state)
        : ws.status === "error" ? renderError(state, ws)
          : ws.step ? renderStep(state, ws)
            : renderLoading()}
      </div>
    </div>
  `;
}

function renderStepDots(ws: WizardState) {
  if (ws.status === "done") return nothing;
  return html`
    <div class="wizard-steps">
      ${Array.from({ length: ws.totalSteps }, (_, i) => {
    const cls = i === ws.currentStepIndex
      ? "wizard-step-dot wizard-step-dot--active"
      : i < ws.currentStepIndex
        ? "wizard-step-dot wizard-step-dot--done"
        : "wizard-step-dot";
    return html`<div class="${cls}"></div>`;
  })}
    </div>
  `;
}

function renderLoading() {
  return html`
    <div class="wizard-loading">
      <div class="wizard-spinner"></div>
      <div class="wizard-loading-text">${t("wizard.preparing")}</div>
    </div>
  `;
}

function renderDone(state: AppViewState) {
  return html`
    <div class="wizard-done">
      <div class="wizard-done-check">✓</div>
      <div class="wizard-done-title">${t("wizard.setupComplete")}</div>
      <div class="wizard-done-sub">${t("wizard.setupCompleteSub")}</div>
      <div class="wizard-actions" style="justify-content: center;">
        <button class="wizard-btn wizard-btn--primary" @click=${() => closeWizard(state)}>
          ${t("wizard.getStarted")}
        </button>
      </div>
    </div>
  `;
}

function renderError(state: AppViewState, ws: WizardState) {
  return html`
    <div class="wizard-icon">⚠️</div>
    <div class="wizard-title">${t("wizard.setupError")}</div>
    <div class="wizard-error">${ws.error ?? t("wizard.unexpectedError")}</div>
    <div class="wizard-actions">
      <button class="wizard-btn wizard-btn--secondary" @click=${() => closeWizard(state)}>
        ${t("wizard.close")}
      </button>
      <button class="wizard-btn wizard-btn--primary" @click=${() => startWizard(state)}>
        ${t("wizard.retry")}
      </button>
    </div>
  `;
}

function renderStep(state: AppViewState, ws: WizardState) {
  const step = ws.step!;
  const icon = STEP_ICONS[ws.currentStepIndex] ?? "⚙️";
  const title = step.title ?? step.message ?? t(STEP_TITLES_FALLBACK_KEYS[ws.currentStepIndex] ?? "wizard.setup");

  return html`
    <div class="wizard-icon">${icon}</div>
    <div class="wizard-title">${title}</div>
    ${step.message && step.message !== title
      ? html`<div class="wizard-subtitle">${step.message}</div>`
      : nothing}
    ${step.type === "note" ? renderNoteStep(state, ws)
      : step.type === "select" ? renderSelectStep(state, ws)
        : step.type === "text" ? renderTextStep(state, ws)
          : step.type === "confirm" ? renderConfirmStep(state, ws)
            : renderNoteStep(state, ws)}
  `;
}

function renderNoteStep(state: AppViewState, ws: WizardState) {
  return html`
    <div class="wizard-note">
      ${ws.step?.message ?? ""}
    </div>
    <div class="wizard-actions">
      <button class="wizard-btn wizard-btn--danger" @click=${() => cancelWizard(state)}>
        ${t("wizard.cancel")}
      </button>
      <button class="wizard-btn wizard-btn--primary" @click=${() => wizardNext(state, true)}>
        ${t("wizard.continue")}
      </button>
    </div>
  `;
}

function renderSelectStep(state: AppViewState, ws: WizardState) {
  const options = ws.step?.options ?? [];
  const selected = ws.selectedValue ?? ws.step?.initialValue;

  return html`
    <div class="wizard-options">
      ${options.map((opt) => {
    const isSelected = selected === opt.value;
    const icon = getProviderIcon(opt.value);
    return html`
          <button
            class="wizard-option ${isSelected ? "wizard-option--selected" : ""}"
            @click=${() => {
        state.wizardState = { ...ws, selectedValue: opt.value };
      }}
          >
            <div class="wizard-option__icon">${icon}</div>
            <div class="wizard-option__text">
              <div class="wizard-option__label">${opt.label}</div>
              ${opt.hint ? html`<div class="wizard-option__hint">${opt.hint}</div>` : nothing}
            </div>
          </button>
        `;
  })}
    </div>
    <div class="wizard-actions">
      <button class="wizard-btn wizard-btn--danger" @click=${() => cancelWizard(state)}>
        ${t("wizard.cancel")}
      </button>
      <button
        class="wizard-btn wizard-btn--primary"
        ?disabled=${!selected}
        @click=${() => { if (selected) void wizardNext(state, selected); }}
      >
        ${t("wizard.continue")}
      </button>
    </div>
  `;
}

function renderTextStep(state: AppViewState, ws: WizardState) {
  const currentVal = (ws.selectedValue as string) ?? "";
  const isSensitive = ws.step?.sensitive ?? false;
  const placeholder = ws.step?.placeholder ?? "";
  const hasEnvHint = ws.step?.options?.some((o) => o.hint?.includes("environment"));

  return html`
    <div class="wizard-input-group">
      <label class="wizard-input-label">${ws.step?.title ?? t("wizard.value")}</label>
      <input
        class="wizard-input"
        type="${isSensitive ? "password" : "text"}"
        placeholder="${placeholder}"
        .value=${currentVal}
        @input=${(e: Event) => {
      const val = (e.target as HTMLInputElement).value;
      state.wizardState = { ...ws, selectedValue: val };
    }}
        @keydown=${(e: KeyboardEvent) => {
      if (e.key === "Enter" && currentVal.trim()) {
        void wizardNext(state, currentVal.trim());
      }
    }}
      />
      ${hasEnvHint ? html`
        <div class="wizard-env-badge">
          ${t("wizard.envDetected")}
        </div>
      ` : nothing}
      ${placeholder ? html`<div class="wizard-input-hint">${t("wizard.enterOrContinue")}</div>` : nothing}
    </div>
    <div class="wizard-actions">
      <button class="wizard-btn wizard-btn--danger" @click=${() => cancelWizard(state)}>
        ${t("wizard.cancel")}
      </button>
      <button
        class="wizard-btn wizard-btn--primary"
        ?disabled=${!currentVal.trim()}
        @click=${() => { if (currentVal.trim()) void wizardNext(state, currentVal.trim()); }}
      >
        ${t("wizard.continue")}
      </button>
    </div>
  `;
}

function renderConfirmStep(state: AppViewState, ws: WizardState) {
  return html`
    <div class="wizard-note">
      ${ws.step?.message ?? t("wizard.confirm")}
    </div>
    <div class="wizard-actions">
      <button class="wizard-btn wizard-btn--secondary" @click=${() => wizardNext(state, false)}>
        ${t("wizard.no")}
      </button>
      <button class="wizard-btn wizard-btn--primary" @click=${() => wizardNext(state, true)}>
        ${t("wizard.yes")}
      </button>
    </div>
  `;
}
