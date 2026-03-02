/**
 * Image Understanding Configuration Wizard — Phase E
 * Fallback vision API for non-multimodal models (e.g. DeepSeek)
 *
 * RPC: image.config.get / image.config.set / image.test / image.models / image.ollama.models
 * Pattern: follows wizard-stt.ts exactly
 */
import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { AppViewState } from "../app-view-state.ts";

// ─── Types ───

type ImageWizardState = {
  step: string;
  loading: boolean;
  error: string | null;
  providers: Array<{ id: string; label: string; hint?: string }>;
  selectedProvider: string;
  apiKey: string;
  baseUrl: string;
  model: string;
  prompt: string;
  maxTokens: number;
  availableModels: string[];
  ollamaModels: string[];
  ollamaOnline: boolean;
  testResult: { success: boolean; error?: string } | null;
  configured: boolean;
};

function getWizard(state: AppViewState): ImageWizardState {
  return (state.imageWizard ?? {
    step: "provider", loading: false, error: null, providers: [],
    selectedProvider: "", apiKey: "", baseUrl: "", model: "",
    prompt: "", maxTokens: 1024, availableModels: [], ollamaModels: [],
    ollamaOnline: false, testResult: null, configured: false,
  }) as unknown as ImageWizardState;
}

function setWizard(state: AppViewState, gw: ImageWizardState): void {
  state.imageWizard = { ...gw } as unknown as Record<string, unknown>;
  state.requestUpdate();
}

// ─── Controller ───

export async function loadImageConfig(state: AppViewState): Promise<void> {
  const gw = getWizard(state);
  gw.loading = true;
  setWizard(state, gw);

  try {
    const client = state.client;
    if (!client) throw new Error("not connected");
    const data = await client.request<Record<string, unknown>>("image.config.get", {});
    gw.providers = (data.providers as ImageWizardState["providers"]) ?? [];
    gw.configured = (data.configured as boolean) ?? false;
    gw.ollamaOnline = (data.ollamaOnline as boolean) ?? false;
    if (gw.configured) {
      gw.selectedProvider = String(data.provider ?? "");
      gw.model = String(data.model ?? "");
      gw.baseUrl = String(data.baseUrl ?? "");
      gw.prompt = String(data.prompt ?? "");
      gw.maxTokens = (data.maxTokens as number) || 1024;
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

    if (provider === "ollama") {
      // Ollama: probe local vision models
      const data = await client.request<{ online: boolean; models?: string[] }>("image.ollama.models", {});
      gw.ollamaOnline = data.online;
      gw.ollamaModels = data.models ?? [];
      gw.availableModels = gw.ollamaModels.length > 0 ? gw.ollamaModels : ["llava", "bakllava", "moondream"];
    } else {
      const data = await client.request<{ models?: string[] }>("image.models", { provider });
      gw.availableModels = data.models ?? [];
    }
    if (gw.availableModels.length > 0 && !gw.model) gw.model = gw.availableModels[0];
  } catch {
    gw.availableModels = [];
  }
  setWizard(state, gw);
}

async function saveImageConfig(state: AppViewState): Promise<boolean> {
  const gw = getWizard(state);
  gw.loading = true;
  setWizard(state, gw);
  try {
    const client = state.client;
    if (!client) throw new Error("not connected");
    const params: Record<string, unknown> = { provider: gw.selectedProvider };
    if (gw.selectedProvider !== "ollama") {
      if (gw.apiKey && !gw.apiKey.startsWith("••")) params.apiKey = gw.apiKey;
    }
    if (gw.model) params.model = gw.model;
    if (gw.baseUrl) params.baseUrl = gw.baseUrl;
    if (gw.prompt) params.prompt = gw.prompt;
    if (gw.maxTokens && gw.maxTokens !== 1024) params.maxTokens = gw.maxTokens;
    await client.request("image.config.set", params);
    const verify = await client.request<Record<string, unknown>>("image.config.get", {});
    gw.loading = false;
    gw.configured = !!(verify as Record<string, unknown>).configured;
    setWizard(state, gw);
    return true;
  } catch (err) {
    gw.loading = false;
    gw.error = String(err);
    setWizard(state, gw);
    return false;
  }
}

async function testImageConnection(state: AppViewState): Promise<void> {
  const gw = getWizard(state);
  gw.loading = true;
  gw.testResult = null;
  setWizard(state, gw);
  try {
    const client = state.client;
    if (!client) throw new Error("not connected");
    const data = await client.request<{ success: boolean; error?: string }>("image.test", {});
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
        void saveImageConfig(state).then(() => { const g = getWizard(state); g.step = "done"; setWizard(state, g); });
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
      void saveImageConfig(state).then((ok) => {
        if (ok) {
          const g = getWizard(state);
          g.step = "test";
          setWizard(state, g);
          void testImageConnection(state);
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

export function renderImageWizard(state: AppViewState) {
  if (!state.imageWizard) return nothing;
  const gw = getWizard(state);
  const steps = ["provider", "credentials", "model", "test", "done"];
  const stepIcons = ["\u{1F5BC}", "\u{1F511}", "\u{1F9E0}", "\u2705", "\u{1F389}"];
  const labels = [
    t("image.wizard.chooseProvider"), t("image.wizard.credentials"),
    t("image.wizard.selectModel"), t("image.wizard.testConnection"), t("image.wizard.done"),
  ];
  const idx = steps.indexOf(gw.step);

  return html`
    <div class="stt-wizard">
      <div class="wizard-header">
        <h2>${stepIcons[idx]} ${labels[idx]}</h2>
        <div class="wizard-progress">
          ${steps.map((_, i) => html`<span class="progress-dot ${i <= idx ? "active" : ""}">${i + 1}</span>`)}
        </div>
      </div>
      ${gw.error ? html`<div class="wizard-error">${gw.error}</div>` : nothing}
      ${gw.loading ? html`<div class="wizard-loading">${t("wizard.preparing")}</div>` : nothing}
      <div class="wizard-body">
        ${gw.step === "provider" ? renderProviderStep(state, gw) : nothing}
        ${gw.step === "credentials" ? renderCredsStep(state, gw) : nothing}
        ${gw.step === "model" ? renderModelStep(state, gw) : nothing}
        ${gw.step === "test" ? renderTestStep(state, gw) : nothing}
        ${gw.step === "done" ? html`
          <div class="wizard-done"><p>${t("image.wizard.configComplete")}</p></div>
          <div class="wizard-actions">
            <button class="btn-primary" @click=${() => { state.imageWizard = undefined; state.requestUpdate(); }}>${t("wizard.close")}</button>
          </div>
        ` : nothing}
      </div>
    </div>
  `;
}

function renderProviderStep(state: AppViewState, gw: ImageWizardState) {
  return html`
    <div class="wizard-options">
      ${gw.providers.map((p) => html`
        <button class="wizard-option ${gw.selectedProvider === p.id ? "wizard-option--selected" : ""}"
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

function renderCredsStep(state: AppViewState, gw: ImageWizardState) {
  const isOllama = gw.selectedProvider === "ollama";
  return html`
    <div class="wizard-form">
      ${!isOllama ? html`
        <label>${t("image.wizard.apiKey")}
          <input type="password" .value=${gw.apiKey}
            @input=${(e: InputEvent) => { gw.apiKey = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
            placeholder=${gw.selectedProvider === "qwen-vl" ? "sk-..." : "sk-..."} />
        </label>
      ` : nothing}
      ${isOllama ? html`
        <div style="padding:8px 0;color:var(--text-secondary);font-size:13px;">
          ${gw.ollamaOnline
            ? t("image.wizard.ollamaDetected")
            : t("image.wizard.ollamaNotDetected")}
        </div>
      ` : nothing}
      <label>${t("image.wizard.baseUrl")}
        <input type="text" .value=${gw.baseUrl}
          @input=${(e: InputEvent) => { gw.baseUrl = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
          placeholder=${isOllama ? "http://localhost:11434/v1" : ""} />
      </label>
    </div>
    <div class="wizard-actions">
      <button class="btn-secondary" @click=${() => prevStep(state)}>${t("wizard.channel.back")}</button>
      <button class="btn-primary" @click=${() => nextStep(state)}>${t("wizard.continue")}</button>
    </div>
  `;
}

function renderModelStep(state: AppViewState, gw: ImageWizardState) {
  return html`
    <div class="wizard-options">
      ${gw.availableModels.map((m) => html`
        <button class="wizard-option ${gw.model === m ? "wizard-option--selected" : ""}"
          @click=${() => { gw.model = m; setWizard(state, gw); }}>
          <span class="option-label">${m}</span>
        </button>`)}
    </div>
    <div class="wizard-actions">
      <button class="btn-secondary" @click=${() => prevStep(state)}>${t("wizard.channel.back")}</button>
      <button class="btn-primary" @click=${() => nextStep(state)} ?disabled=${!gw.model}>${t("image.wizard.saveAndTest")}</button>
    </div>
  `;
}

function renderTestStep(state: AppViewState, gw: ImageWizardState) {
  return html`
    <div class="wizard-test-result">
      ${gw.loading ? html`<p>${t("image.wizard.testing")}</p>` : nothing}
      ${gw.testResult?.success ? html`<p class="test-success">${t("image.wizard.testSuccess")}</p>` : nothing}
      ${gw.testResult && !gw.testResult.success ? html`<p class="test-error">${gw.testResult.error || t("image.wizard.testFailed")}</p>` : nothing}
    </div>
    <div class="wizard-actions">
      <button class="btn-secondary" @click=${() => prevStep(state)}>${t("wizard.channel.back")}</button>
      <button class="btn-primary" @click=${() => nextStep(state)}>${t("wizard.continue")}</button>
    </div>
  `;
}
