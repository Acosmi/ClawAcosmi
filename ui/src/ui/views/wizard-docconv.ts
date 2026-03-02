/**
 * DocConv Configuration Wizard — Document Conversion Setup
 * Phase D: frontend configuration wizard
 *
 * RPC: docconv.config.get / docconv.config.set / docconv.test / docconv.formats
 */
import { html, nothing, render } from "lit";
import { t } from "../i18n.ts";
import type { AppViewState } from "../app-view-state.ts";

// ─── Types ───

type DocConvWizardState = {
  step: string;
  loading: boolean;
  error: string | null;
  providers: Array<{ id: string; label: string; hint?: string }>;
  presets: Array<{ name: string; label: string; command: string; transport: string; hint?: string }>;
  selectedProvider: string;
  mcpServerName: string;
  mcpTransport: string;
  mcpCommand: string;
  mcpUrl: string;
  pandocPath: string;
  selectedPreset: string;
  testResult: { success: boolean; error?: string; formats?: string[] } | null;
  configured: boolean;
};

function getWizard(state: AppViewState): DocConvWizardState {
  return (state.docConvWizard ?? {
    step: "provider", loading: false, error: null, providers: [], presets: [],
    selectedProvider: "", mcpServerName: "", mcpTransport: "stdio",
    mcpCommand: "", mcpUrl: "", pandocPath: "", selectedPreset: "",
    testResult: null, configured: false,
  }) as unknown as DocConvWizardState;
}

function setWizard(state: AppViewState, gw: DocConvWizardState): void {
  state.docConvWizard = { ...gw } as unknown as Record<string, unknown>;
  state.requestUpdate();
}

// ─── Pandoc guide portal (rendered into document.body to escape stacking contexts) ───
let docconvGuidePortal: HTMLDivElement | null = null;

function openDocConvGuide(state: AppViewState): void {
  if (docconvGuidePortal) return;
  docconvGuidePortal = document.createElement("div");
  document.body.appendChild(docconvGuidePortal);
  renderDocConvGuideContent(state);
}

function closeDocConvGuide(): void {
  if (docconvGuidePortal) {
    render(nothing, docconvGuidePortal);
    docconvGuidePortal.remove();
    docconvGuidePortal = null;
  }
}

// ─── Controller ───

export async function loadDocConvConfig(state: AppViewState): Promise<void> {
  const gw = getWizard(state);
  gw.loading = true;
  setWizard(state, gw);

  try {
    const client = state.client;
    if (!client) throw new Error("not connected");
    const data = await client.request<Record<string, unknown>>("docconv.config.get", {});
    gw.providers = (data.providers as DocConvWizardState["providers"]) ?? [];
    gw.presets = (data.mcpPresets as DocConvWizardState["presets"]) ?? [];
    gw.configured = (data.configured as boolean) ?? false;
    if (gw.configured) {
      gw.selectedProvider = String(data.provider ?? "");
      gw.mcpServerName = String(data.mcpServerName ?? "");
      gw.mcpTransport = String(data.mcpTransport ?? "stdio");
      gw.mcpCommand = String(data.mcpCommand ?? "");
      gw.mcpUrl = String(data.mcpUrl ?? "");
      gw.pandocPath = String(data.pandocPath ?? "");
    }
    gw.loading = false;
    gw.error = null;
  } catch (err) {
    gw.loading = false;
    gw.error = String(err);
  }
  setWizard(state, gw);
}

async function saveDocConvConfig(state: AppViewState): Promise<boolean> {
  const gw = getWizard(state);
  gw.loading = true;
  setWizard(state, gw);
  try {
    const client = state.client;
    if (!client) throw new Error("not connected");
    const params: Record<string, string> = { provider: gw.selectedProvider };
    if (gw.selectedProvider === "mcp") {
      params.mcpServerName = gw.mcpServerName;
      params.mcpTransport = gw.mcpTransport;
      if (gw.mcpTransport === "stdio") params.mcpCommand = gw.mcpCommand;
      if (gw.mcpTransport === "sse") params.mcpUrl = gw.mcpUrl;
    }
    if (gw.selectedProvider === "builtin") {
      params.pandocPath = gw.pandocPath || "pandoc";
    }
    await client.request("docconv.config.set", params);
    const verify = await client.request<Record<string, unknown>>("docconv.config.get", {});
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

async function testDocConvConnection(state: AppViewState): Promise<void> {
  const gw = getWizard(state);
  gw.loading = true;
  gw.testResult = null;
  setWizard(state, gw);
  try {
    const client = state.client;
    if (!client) throw new Error("not connected");
    gw.testResult = await client.request<{ success: boolean; error?: string; formats?: string[] }>("docconv.test", {});
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
        void saveDocConvConfig(state).then(() => { const g = getWizard(state); g.step = "done"; setWizard(state, g); });
        return;
      }
      gw.step = "config";
      break;
    case "config":
      void saveDocConvConfig(state).then((ok) => {
        if (ok) {
          const g = getWizard(state);
          g.step = "test";
          setWizard(state, g);
          void testDocConvConnection(state);
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
    case "config": gw.step = "provider"; break;
    case "test": gw.step = "config"; break;
    default: return;
  }
  setWizard(state, gw);
}

// ─── Render ───

export function renderDocConvWizard(state: AppViewState) {
  if (!state.docConvWizard) return nothing;
  const gw = getWizard(state);
  const steps = ["provider", "config", "test", "done"];
  const icons = ["📄", "⚙️", "✅", "🎉"];
  const labels = [
    t("docconv.wizard.chooseMode"), t("docconv.wizard.configService"),
    t("docconv.wizard.testConnection"), t("docconv.wizard.done"),
  ];
  const idx = steps.indexOf(gw.step);

  return html`
    <div class="docconv-wizard">
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
        ${gw.step === "config" ? renderConfigStep(state, gw) : nothing}
        ${gw.step === "test" ? renderTestStep(state, gw) : nothing}
        ${gw.step === "done" ? html`
          <div class="wizard-done"><p>🎉 ${t("docconv.wizard.configComplete")}</p></div>
          <div class="wizard-actions">
            <button class="btn-primary" @click=${() => { state.docConvWizard = undefined; state.requestUpdate(); }}>${t("wizard.close")}</button>
          </div>
        ` : nothing}
      </div>
    </div>
  `;
}

function renderProviderStep(state: AppViewState, gw: DocConvWizardState) {
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

function renderConfigStep(state: AppViewState, gw: DocConvWizardState) {
  return html`
    <div class="wizard-form">
      ${gw.selectedProvider === "mcp" ? html`
        ${gw.presets.length > 0 ? html`
          <label>${t("docconv.wizard.mcpPreset")}</label>
          <div class="wizard-options compact">
            ${gw.presets.map((p) => html`
              <button class="wizard-option ${gw.selectedPreset === p.name ? "wizard-option--selected" : ""}"
                @click=${() => {
        gw.selectedPreset = p.name;
        gw.mcpServerName = p.name;
        gw.mcpCommand = p.command;
        gw.mcpTransport = p.transport;
        setWizard(state, gw);
      }}>
                <span class="option-label">${p.label}</span>
                ${p.hint ? html`<span class="option-hint">${p.hint}</span>` : nothing}
              </button>`)}
          </div>
          <div style="border-top:1px solid var(--border);margin:12px 0;"></div>
        ` : nothing}
        <label>${t("docconv.wizard.mcpServerName")}
          <input type="text" .value=${gw.mcpServerName}
            @input=${(e: InputEvent) => { gw.mcpServerName = (e.target as HTMLInputElement).value; gw.selectedPreset = ""; setWizard(state, gw); }}
            placeholder="my-docconv-server" />
        </label>
        <label>${t("docconv.wizard.mcpTransport")}
          <div style="display:flex;gap:12px;margin-top:4px;">
            <label style="display:flex;align-items:center;gap:4px;font-weight:normal;cursor:pointer;">
              <input type="radio" name="mcp-transport" .checked=${gw.mcpTransport === "stdio"}
                @change=${() => { gw.mcpTransport = "stdio"; gw.selectedPreset = ""; setWizard(state, gw); }} /> stdio
            </label>
            <label style="display:flex;align-items:center;gap:4px;font-weight:normal;cursor:pointer;">
              <input type="radio" name="mcp-transport" .checked=${gw.mcpTransport === "sse"}
                @change=${() => { gw.mcpTransport = "sse"; gw.selectedPreset = ""; setWizard(state, gw); }} /> sse
            </label>
          </div>
        </label>
        ${gw.mcpTransport === "stdio" ? html`
          <label>${t("docconv.wizard.mcpCommand")}
            <input type="text" .value=${gw.mcpCommand}
              @input=${(e: InputEvent) => { gw.mcpCommand = (e.target as HTMLInputElement).value; gw.selectedPreset = ""; setWizard(state, gw); }}
              placeholder="npx -y mcp-pandoc" />
          </label>
        ` : html`
          <label>${t("docconv.wizard.mcpUrl")}
            <input type="text" .value=${gw.mcpUrl}
              @input=${(e: InputEvent) => { gw.mcpUrl = (e.target as HTMLInputElement).value; gw.selectedPreset = ""; setWizard(state, gw); }}
              placeholder="http://localhost:8080/sse" />
          </label>
        `}
      ` : nothing}
      ${gw.selectedProvider === "builtin" ? html`
        <div style="display:flex;align-items:center;gap:8px;margin-bottom:8px;">
          <span style="font-size:13px;color:var(--text-secondary);">${t("docconv.wizard.pandocNeedInstall")}</span>
          <button class="pill" style="cursor:pointer;font-size:11px;padding:2px 10px;border:1px solid var(--border);border-radius:12px;background:var(--bg-secondary);color:var(--text-primary);"
            @click=${() => { openDocConvGuide(state); }}>
            ${t("docconv.wizard.pandocGuideBtn")}
          </button>
        </div>
        <label>${t("docconv.wizard.pandocPath")}
          <input type="text" .value=${gw.pandocPath}
            @input=${(e: InputEvent) => { gw.pandocPath = (e.target as HTMLInputElement).value; setWizard(state, gw); }}
            placeholder="pandoc" />
        </label>
      ` : nothing}
    </div>
    <div class="wizard-actions">
      <button class="btn-secondary" @click=${() => prevStep(state)}>${t("wizard.channel.back")}</button>
      <button class="btn-primary" @click=${() => nextStep(state)}>${t("stt.wizard.saveAndTest")}</button>
    </div>
  `;
}

function renderTestStep(state: AppViewState, gw: DocConvWizardState) {
  return html`
    <div class="wizard-test-result">
      ${gw.loading ? html`<p>🔄 ${t("docconv.wizard.testing")}</p>` : nothing}
      ${gw.testResult?.success ? html`
        <p class="test-success">✅ ${t("docconv.wizard.testSuccess")}</p>
        ${gw.testResult.formats ? html`<p>${t("docconv.wizard.supportedFormats")}: ${gw.testResult.formats.join(", ")}</p>` : nothing}
      ` : nothing}
      ${gw.testResult && !gw.testResult.success ? html`<p class="test-error">❌ ${gw.testResult.error || t("docconv.wizard.testFailed")}</p>` : nothing}
    </div>
    <div class="wizard-actions">
      <button class="btn-secondary" @click=${() => prevStep(state)}>${t("wizard.channel.back")}</button>
      <button class="btn-primary" @click=${() => nextStep(state)}>${t("wizard.continue")}</button>
    </div>
  `;
}

// ─── Pandoc Guide Portal (rendered into document.body) ───

function renderDocConvGuideContent(state: AppViewState): void {
  if (!docconvGuidePortal) return;
  render(html`
    <div style="position:fixed;inset:0;z-index:99999;display:flex;align-items:center;justify-content:center;"
      @click=${(e: Event) => { if (e.target === e.currentTarget) closeDocConvGuide(); }}>
        <div style="background:#fff;border-radius:12px;max-width:520px;width:90%;max-height:80vh;display:flex;flex-direction:column;box-shadow:0 12px 40px rgba(0,0,0,0.25);">
          <div style="display:flex;align-items:center;justify-content:space-between;padding:16px 20px;border-bottom:1px solid var(--border);">
            <h3 style="margin:0;font-size:16px;">${t("docconv.wizard.pandocGuideTitle")}</h3>
            <button style="background:none;border:none;font-size:20px;cursor:pointer;color:var(--text-secondary);padding:0 4px;"
              @click=${() => closeDocConvGuide()}>&times;</button>
          </div>
          <div style="padding:20px;overflow-y:auto;font-size:13px;line-height:1.8;color:var(--text-primary);">
            <h4 style="margin:0 0 8px;">${t("docconv.wizard.pandocWhat")}</h4>
            <p style="margin:0 0 16px;color:var(--text-secondary);">${t("docconv.wizard.pandocWhatBody")}</p>

            <h4 style="margin:0 0 8px;">${t("docconv.wizard.pandocInstall")}</h4>
            <p style="margin:0 0 6px;color:var(--text-secondary);">macOS:</p>
            <pre style="background:var(--bg-secondary);padding:8px 12px;border-radius:6px;overflow-x:auto;margin:0 0 8px;font-size:12px;">brew install pandoc</pre>
            <p style="margin:0 0 6px;color:var(--text-secondary);">Ubuntu / Debian:</p>
            <pre style="background:var(--bg-secondary);padding:8px 12px;border-radius:6px;overflow-x:auto;margin:0 0 8px;font-size:12px;">sudo apt install pandoc</pre>
            <p style="margin:0 0 6px;color:var(--text-secondary);">Windows:</p>
            <pre style="background:var(--bg-secondary);padding:8px 12px;border-radius:6px;overflow-x:auto;margin:0 0 16px;font-size:12px;">winget install --id JohnMacFarlane.Pandoc</pre>

            <h4 style="margin:0 0 8px;">${t("docconv.wizard.pandocVerify")}</h4>
            <pre style="background:var(--bg-secondary);padding:8px 12px;border-radius:6px;overflow-x:auto;margin:0 0 8px;font-size:12px;">pandoc --version</pre>
            <p style="margin:0 0 16px;color:var(--text-secondary);">${t("docconv.wizard.pandocVerifyBody")}</p>

            <h4 style="margin:0 0 8px;">${t("docconv.wizard.pandocFormats")}</h4>
            <p style="margin:0;color:var(--text-secondary);">${t("docconv.wizard.pandocFormatsBody")}</p>
          </div>
          <div style="padding:12px 20px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;">
            <button class="btn-primary" @click=${() => closeDocConvGuide()}>${t("wizard.close")}</button>
          </div>
        </div>
    </div>
  `, docconvGuidePortal);
}
