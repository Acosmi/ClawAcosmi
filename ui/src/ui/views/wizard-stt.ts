/**
 * STT Configuration Wizard — Speech-to-Text Provider Setup
 * Phase C: frontend configuration wizard
 *
 * RPC: stt.config.get / stt.config.set / stt.test / stt.models
 */
import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { AppViewState } from "../app-view-state.ts";

// ─── Types ───

type STTWizardState = {
  step: string;
  loading: boolean;
  error: string | null;
  providers: Array<{ id: string; label: string; hint?: string }>;
  selectedProvider: string;
  apiKey: string;
  baseUrl: string;
  model: string;
  language: string;
  binaryPath: string;
  modelPath: string;
  availableModels: string[];
  testResult: { success: boolean; error?: string } | null;
  configured: boolean;
};

function getWizard(state: AppViewState): STTWizardState {
  return (state.sttWizard ?? {
    step: "provider", loading: false, error: null, providers: [],
    selectedProvider: "", apiKey: "", baseUrl: "", model: "",
    language: "", binaryPath: "", modelPath: "", availableModels: [],
    testResult: null, configured: false,
  }) as unknown as STTWizardState;
}

function setWizard(state: AppViewState, gw: STTWizardState): void {
  state.sttWizard = { ...gw } as unknown as Record<string, unknown>;
  state.requestUpdate();
}

// ─── Controller ───

export async function loadSTTConfig(state: AppViewState): Promise<void> {
  const gw = getWizard(state);
  gw.loading = true;
  setWizard(state, gw);

  try {
    const client = state.client;
    if (!client) throw new Error("not connected");
    const data = await client.request<Record<string, unknown>>("stt.config.get", {});
    gw.providers = (data.providers as STTWizardState["providers"]) ?? [];
    gw.configured = (data.configured as boolean) ?? false;
    if (gw.configured) {
      gw.selectedProvider = String(data.provider ?? "");
      gw.model = String(data.model ?? "");
      gw.baseUrl = String(data.baseUrl ?? "");
      gw.language = String(data.language ?? "");
      gw.apiKey = (data.hasApiKey as boolean) ? "••••••••" : "";
    }
    gw.loading = false;
    gw.error = null;
  } catch (err) {
    gw.loading = false;
    gw.error = String(err);
  }
  setWizard(state, gw);
}

async function loadModels(state: AppViewState, provider: string): Promise<void> {
  const gw = getWizard(state);
  try {
    const client = state.client;
    if (!client) return;
    const data = await client.request<{ models?: string[] }>("stt.models", { provider });
    gw.availableModels = data.models ?? [];
    if (gw.availableModels.length > 0 && !gw.model) gw.model = gw.availableModels[0];
  } catch {
    gw.availableModels = [];
  }
  setWizard(state, gw);
}

async function saveSTTConfig(state: AppViewState): Promise<boolean> {
  const gw = getWizard(state);
  gw.loading = true;
  setWizard(state, gw);
  try {
    const client = state.client;
    if (!client) throw new Error("not connected");
    const params: Record<string, string> = { provider: gw.selectedProvider };
    if (["openai", "groq", "azure"].includes(gw.selectedProvider)) {
      if (gw.apiKey && !gw.apiKey.startsWith("••")) params.apiKey = gw.apiKey;
      if (gw.model) params.model = gw.model;
      if (gw.baseUrl) params.baseUrl = gw.baseUrl;
    }
    if (gw.selectedProvider === "local-whisper") {
      if (gw.binaryPath) params.binaryPath = gw.binaryPath;
      if (gw.modelPath) params.modelPath = gw.modelPath;
    }
    if (gw.language) params.language = gw.language;
    await client.request("stt.config.set", params);
    gw.loading = false;
    gw.configured = true;
    setWizard(state, gw);
    return true;
  } catch (err) {
    gw.loading = false;
    gw.error = String(err);
    setWizard(state, gw);
    return false;
  }
}

async function testSTTConnection(state: AppViewState): Promise<void> {
  const gw = getWizard(state);
  gw.loading = true;
  gw.testResult = null;
  setWizard(state, gw);
  try {
    const client = state.client;
    if (!client) throw new Error("not connected");
    const data = await client.request<{ success: boolean; error?: string }>("stt.test", {});
    gw.testResult = data;
    gw.loading = false;
  } catch (err) {
    gw.testResult = { success: false, error: String(err) };
    gw.loading = false;
  }
  setWizard(state, gw);
}

// ─── Navigation ───

function nextStep(state: AppViewState): void {
  const gw = getWizard(state);
  switch (gw.step) {
    case "provider":
      if (!gw.selectedProvider) {
        void saveSTTConfig(state).then(() => { const g = getWizard(state); g.step = "done"; setWizard(state, g); });
        return;
      }
      gw.step = "credentials";
      setWizard(state, gw);
      void loadModels(state, gw.selectedProvider);
      return;
    case "credentials":
      gw.step = "model";
      break;
    case "model":
      void saveSTTConfig(state).then((ok) => {
        if (ok) {
          const g = getWizard(state);
          g.step = "test";
          setWizard(state, g);
          void testSTTConnection(state);
        }
      });
      return;
    case "test":
      gw.step = "done";
      break;
  }
  setWizard(state, gw);
}

function prevStep(state: AppViewState): void {
  const gw = getWizard(state);
  switch (gw.step) {
    case "credentials": gw.step = "provider"; break;
    case "model": gw.step = "credentials"; break;
    case "test": gw.step = "model"; break;
    default: return;
  }
  setWizard(state, gw);
}

// ─── Render ───

export function renderSTTWizard(state: AppViewState) {
  if (!state.sttWizard) return nothing;
  const gw = getWizard(state);
  const steps = ["provider", "credentials", "model", "test", "done"];
  const icons = ["🎤", "🔑", "🧠", "✅", "🎉"];
  const labels = [
    t("stt.wizard.chooseProvider"), t("stt.wizard.credentials"),
    t("stt.wizard.selectModel"), t("stt.wizard.testConnection"), t("stt.wizard.done"),
  ];
  const idx = steps.indexOf(gw.step);

  return html`
    <div class="stt-wizard">
      <div class="wizard-header">
        <h2>${icons[idx]} ${labels[idx]}</h2>
        <div class="wizard-progress">
          ${steps.map((_, i) => html`<span class="progress-dot ${i <= idx ? "active" : ""}">${i + 1}</span>`)}
        </div>
      </div>
      ${gw.error ? html`<div class="wizard-error">⚠️ ${gw.error}</div>` : nothing}
      ${gw.loading ? html`<div class="wizard-loading">${t("wizard.preparing")}</div>` : nothing}
      <div class="wizard-body">
        ${gw.step === "provider" ? renderProviderStep(state, gw) : nothing}
        ${gw.step === "credentials" ? renderCredsStep(state, gw) : nothing}
        ${gw.step === "model" ? renderModelStep(state, gw) : nothing}
        ${gw.step === "test" ? renderTestStep(state, gw) : nothing}
        ${gw.step === "done" ? html`
          <div class="wizard-done"><p>🎉 ${t("stt.wizard.configComplete")}</p></div>
          <div class="wizard-actions">
            <button class="btn-primary" @click=${() => { state.sttWizard = undefined; state.requestUpdate(); }}>${t("wizard.close")}</button>
          </div>
        ` : nothing}
      </div>
    </div>
  `;
}

function renderProviderStep(state: AppViewState, gw: STTWizardState) {
  return html`
    <div class="wizard-options">
      ${gw.providers.map((p) => html`
        <button class="wizard-option ${gw.selectedProvider === p.id ? "selected" : ""}"
          @click=${() => { gw.selectedProvider = p.id; setWizard(state, gw); }}>
          <span class="option-label">${p.label}</span>
          ${p.hint ? html`<span class="option-hint">${p.hint}</span>` : nothing}
        </button>`)}
    </div>
    <div class="wizard-actions">
      <button class="btn-primary" @click=${() => nextStep(state)}>${t("wizard.continue")}</button>
    </div>
  `;
}

function renderCredsStep(state: AppViewState, gw: STTWizardState) {
  const isCloud = ["openai", "groq", "azure"].includes(gw.selectedProvider);
  const isLocal = gw.selectedProvider === "local-whisper";
  return html`
    <div class="wizard-form">
      ${isCloud ? html`
        <label>${t("stt.wizard.apiKey")}
          <input type="password" .value=${gw.apiKey}
            @input=${(e: InputEvent) => { gw.apiKey = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
            placeholder="sk-..." />
        </label>
        ${gw.selectedProvider !== "openai" ? html`
          <label>${t("stt.wizard.baseUrl")}
            <input type="text" .value=${gw.baseUrl}
              @input=${(e: InputEvent) => { gw.baseUrl = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
              placeholder="https://api.groq.com/openai/v1" />
          </label>` : nothing}
      ` : nothing}
      ${isLocal ? html`
        <label>${t("stt.wizard.binaryPath")}
          <input type="text" .value=${gw.binaryPath}
            @input=${(e: InputEvent) => { gw.binaryPath = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
            placeholder="/usr/local/bin/whisper" />
        </label>
        <label>${t("stt.wizard.modelPath")}
          <input type="text" .value=${gw.modelPath}
            @input=${(e: InputEvent) => { gw.modelPath = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
            placeholder="/path/to/ggml-base.bin" />
        </label>` : nothing}
      <label>${t("stt.wizard.language")}
        <input type="text" .value=${gw.language}
          @input=${(e: InputEvent) => { gw.language = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
          placeholder="zh" />
      </label>
    </div>
    <div class="wizard-actions">
      <button class="btn-secondary" @click=${() => prevStep(state)}>${t("wizard.channel.back")}</button>
      <button class="btn-primary" @click=${() => nextStep(state)}>${t("wizard.continue")}</button>
    </div>
  `;
}

function renderModelStep(state: AppViewState, gw: STTWizardState) {
  return html`
    <div class="wizard-options">
      ${gw.availableModels.map((m) => html`
        <button class="wizard-option ${gw.model === m ? "selected" : ""}"
          @click=${() => { gw.model = m; setWizard(state, gw); }}>
          <span class="option-label">${m}</span>
        </button>`)}
    </div>
    <div class="wizard-actions">
      <button class="btn-secondary" @click=${() => prevStep(state)}>${t("wizard.channel.back")}</button>
      <button class="btn-primary" @click=${() => nextStep(state)} ?disabled=${!gw.model}>${t("stt.wizard.saveAndTest")}</button>
    </div>
  `;
}

function renderTestStep(state: AppViewState, gw: STTWizardState) {
  return html`
    <div class="wizard-test-result">
      ${gw.loading ? html`<p>🔄 ${t("stt.wizard.testing")}</p>` : nothing}
      ${gw.testResult?.success ? html`<p class="test-success">✅ ${t("stt.wizard.testSuccess")}</p>` : nothing}
      ${gw.testResult && !gw.testResult.success ? html`<p class="test-error">❌ ${gw.testResult.error || t("stt.wizard.testFailed")}</p>` : nothing}
    </div>
    <div class="wizard-actions">
      <button class="btn-secondary" @click=${() => prevStep(state)}>${t("wizard.channel.back")}</button>
      <button class="btn-primary" @click=${() => nextStep(state)}>${t("wizard.continue")}</button>
    </div>
  `;
}
