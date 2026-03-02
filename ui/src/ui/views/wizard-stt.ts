/**
 * STT Configuration Wizard — Speech-to-Text Provider Setup
 * Phase C: frontend configuration wizard
 *
 * RPC: stt.config.get / stt.config.set / stt.test / stt.models
 */
import { html, nothing, render } from "lit";
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

// ─── Local guide portal (rendered into document.body to escape stacking contexts) ───
let sttGuidePortal: HTMLDivElement | null = null;

function openSTTGuide(state: AppViewState): void {
  if (sttGuidePortal) return;
  sttGuidePortal = document.createElement("div");
  document.body.appendChild(sttGuidePortal);
  renderSTTGuideContent(state);
}

function closeSTTGuide(): void {
  if (sttGuidePortal) {
    render(nothing, sttGuidePortal);
    sttGuidePortal.remove();
    sttGuidePortal = null;
  }
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
    if (["openai", "groq", "azure", "qwen"].includes(gw.selectedProvider)) {
      if (gw.apiKey && !gw.apiKey.startsWith("••")) params.apiKey = gw.apiKey;
      if (gw.model) params.model = gw.model;
      if (gw.baseUrl) params.baseUrl = gw.baseUrl;
    }
    if (gw.selectedProvider === "ollama") {
      if (gw.baseUrl) params.baseUrl = gw.baseUrl;
      if (gw.model) params.model = gw.model;
    }
    if (gw.selectedProvider === "local-whisper") {
      if (gw.binaryPath) params.binaryPath = gw.binaryPath;
      if (gw.modelPath) params.modelPath = gw.modelPath;
    }
    if (gw.language) params.language = gw.language;
    await client.request("stt.config.set", params);
    const verify = await client.request<Record<string, unknown>>("stt.config.get", {});
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
      if (gw.selectedProvider !== "local-whisper") {
        void loadModels(state, gw.selectedProvider);
      }
      return;
    case "credentials":
      if (gw.selectedProvider === "local-whisper") {
        // local-whisper 跳过 model step，直接保存测试
        void saveSTTConfig(state).then((ok) => {
          if (ok) {
            const g = getWizard(state);
            g.step = "test";
            setWizard(state, g);
            void testSTTConnection(state);
          }
        });
        return;
      }
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
    case "test": gw.step = gw.selectedProvider === "local-whisper" ? "credentials" : "model"; break;
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

function renderCredsStep(state: AppViewState, gw: STTWizardState) {
  const isCloud = ["openai", "groq", "azure", "qwen"].includes(gw.selectedProvider);
  const isOllama = gw.selectedProvider === "ollama";
  const isLocal = gw.selectedProvider === "local-whisper";
  return html`
    <div class="wizard-form">
      ${isCloud ? html`
        <label>${t("stt.wizard.apiKey")}
          <input type="password" .value=${gw.apiKey}
            @input=${(e: InputEvent) => { gw.apiKey = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
            placeholder=${gw.selectedProvider === "qwen" ? "sk-..." : "sk-..."} />
        </label>
        ${gw.selectedProvider !== "openai" ? html`
          <label>${t("stt.wizard.baseUrl")}
            <input type="text" .value=${gw.baseUrl}
              @input=${(e: InputEvent) => { gw.baseUrl = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
              placeholder=${gw.selectedProvider === "qwen" ? "https://dashscope.aliyuncs.com/compatible-mode/v1" : "https://api.groq.com/openai/v1"} />
          </label>` : nothing}
      ` : nothing}
      ${isOllama ? html`
        <div style="padding:8px 0;color:var(--text-secondary);font-size:13px;">
          ${t("stt.wizard.ollamaHint")}
        </div>
        <label>${t("stt.wizard.baseUrl")}
          <input type="text" .value=${gw.baseUrl}
            @input=${(e: InputEvent) => { gw.baseUrl = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
            placeholder="http://localhost:11434/v1" />
        </label>
      ` : nothing}
      ${isLocal ? html`
        <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px;">
          <span style="font-size:13px;color:var(--text-secondary);">${t("stt.wizard.localNeedInstall")}</span>
          <button class="pill" style="cursor:pointer;font-size:11px;padding:2px 10px;border:1px solid var(--border);border-radius:12px;background:var(--bg-secondary);color:var(--text-primary);"
            @click=${() => { openSTTGuide(state); }}>
            ${t("stt.wizard.localGuideBtn")}
          </button>
        </div>
        <label>${t("stt.wizard.binaryPath")}
          <input type="text" .value=${gw.binaryPath}
            @input=${(e: InputEvent) => { gw.binaryPath = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
            placeholder="/usr/local/bin/whisper-cpp" />
        </label>
        <label>${t("stt.wizard.modelPath")}
          <input type="text" .value=${gw.modelPath}
            @input=${(e: InputEvent) => { gw.modelPath = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
            placeholder="/path/to/ggml-base.bin" />
        </label>
      ` : nothing}
      <label>${t("stt.wizard.language")}
        <input type="text" .value=${gw.language}
          @input=${(e: InputEvent) => { gw.language = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
          placeholder="zh" />
      </label>
    </div>
    <div class="wizard-actions">
      <button class="btn-secondary" @click=${() => prevStep(state)}>${t("wizard.channel.back")}</button>
      <button class="btn-primary" @click=${() => nextStep(state)}>
        ${isLocal ? t("stt.wizard.saveAndTest") : t("wizard.continue")}
      </button>
    </div>
  `;
}

function renderModelStep(state: AppViewState, gw: STTWizardState) {
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

// ─── Local Whisper Guide Portal (rendered into document.body) ───

function renderSTTGuideContent(state: AppViewState): void {
  if (!sttGuidePortal) return;
  render(html`
    <div style="position:fixed;inset:0;z-index:99999;display:flex;align-items:center;justify-content:center;"
      @click=${(e: Event) => { if (e.target === e.currentTarget) closeSTTGuide(); }}>
        <div style="background:#fff;border-radius:12px;max-width:560px;width:90%;max-height:80vh;display:flex;flex-direction:column;box-shadow:0 12px 40px rgba(0,0,0,0.25);">
          <div style="display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);">
            <h3 style="margin:0;font-size:16px;">${t("stt.wizard.localGuideTitle")}</h3>
            <button style="background:none;border:none;font-size:20px;cursor:pointer;color:var(--text-secondary);padding:0 4px;"
              @click=${() => closeSTTGuide()}>&times;</button>
          </div>
          <div style="padding:20px;overflow-y:auto;font-size:13px;line-height:1.8;color:var(--text-primary);">
            <h4 style="margin:0 0 8px;">${t("stt.wizard.guideWhat")}</h4>
            <p style="margin:0 0 16px;color:var(--text-secondary);">${t("stt.wizard.guideWhatBody")}</p>

            <h4 style="margin:0 0 8px;">${t("stt.wizard.guideInstall")}</h4>
            <pre style="background:var(--bg-secondary);padding:10px 12px;border-radius:6px;overflow-x:auto;margin:0 0 6px;font-size:12px;">brew install whisper-cpp</pre>
            <p style="margin:0 0 16px;color:var(--text-secondary);">${t("stt.wizard.guideInstallAlt")}</p>

            <h4 style="margin:0 0 8px;">${t("stt.wizard.guideModel")}</h4>
            <p style="margin:0 0 8px;color:var(--text-secondary);">${t("stt.wizard.guideModelBody")}</p>
            <table style="width:100%;border-collapse:collapse;font-size:12px;margin:0 0 16px;">
              <tr style="border-bottom:1px solid var(--border);">
                <th style="text-align:left;padding:4px 8px;">Model</th>
                <th style="text-align:right;padding:4px 8px;">Size</th>
                <th style="text-align:right;padding:4px 8px;">RAM</th>
                <th style="text-align:left;padding:4px 8px;">CJK</th>
              </tr>
              <tr style="border-bottom:1px solid var(--border);">
                <td style="padding:4px 8px;">tiny</td><td style="text-align:right;padding:4px 8px;">75 MB</td>
                <td style="text-align:right;padding:4px 8px;">~125 MB</td><td style="padding:4px 8px;">Poor</td>
              </tr>
              <tr style="border-bottom:1px solid var(--border);">
                <td style="padding:4px 8px;">base</td><td style="text-align:right;padding:4px 8px;">142 MB</td>
                <td style="text-align:right;padding:4px 8px;">~210 MB</td><td style="padding:4px 8px;">Fair</td>
              </tr>
              <tr style="border-bottom:1px solid var(--border);">
                <td style="padding:4px 8px;">small</td><td style="text-align:right;padding:4px 8px;">466 MB</td>
                <td style="text-align:right;padding:4px 8px;">~600 MB</td><td style="padding:4px 8px;">Good</td>
              </tr>
              <tr style="border-bottom:1px solid var(--border);">
                <td style="padding:4px 8px;">medium</td><td style="text-align:right;padding:4px 8px;">1.5 GB</td>
                <td style="text-align:right;padding:4px 8px;">~1.7 GB</td><td style="padding:4px 8px;">Great</td>
              </tr>
              <tr>
                <td style="padding:4px 8px;">large-v3</td><td style="text-align:right;padding:4px 8px;">3.1 GB</td>
                <td style="text-align:right;padding:4px 8px;">~3.3 GB</td><td style="padding:4px 8px;">Best</td>
              </tr>
            </table>

            <h4 style="margin:0 0 8px;">${t("stt.wizard.guidePaths")}</h4>
            <p style="margin:0 0 4px;color:var(--text-secondary);">${t("stt.wizard.guidePathBinary")}</p>
            <pre style="background:var(--bg-secondary);padding:6px 12px;border-radius:6px;overflow-x:auto;margin:0 0 8px;font-size:12px;">which whisper-cpp</pre>
            <p style="margin:0 0 4px;color:var(--text-secondary);">${t("stt.wizard.guidePathModel")}</p>
            <pre style="background:var(--bg-secondary);padding:6px 12px;border-radius:6px;overflow-x:auto;margin:0;font-size:12px;">~/.local/share/whisper-cpp/ggml-base.bin</pre>
          </div>
          <div style="padding:12px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;">
            <button class="btn-primary" @click=${() => closeSTTGuide()}>${t("wizard.close")}</button>
          </div>
        </div>
    </div>
  `, sttGuidePortal);
}
