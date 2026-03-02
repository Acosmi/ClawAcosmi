import { html, nothing } from "lit";
import type { BackupEntry, MaintenanceState, ResetTarget } from "../controllers/system-maintenance.ts";
import { t } from "../i18n.ts";

// ---------- Props ----------

export interface MaintenanceViewProps {
  state: MaintenanceState;
  loading: boolean;
  onLoadBackups: () => void;
  onRestoreBackup: (index: number) => void;
  onPreviewReset: () => void;
  onExecuteReset: () => void;
  onConfirmOpen: (action: "restore" | "reset", index?: number) => void;
  onConfirmCancel: () => void;
  onConfirmTextChange: (text: string) => void;
  onConfirmSubmit: () => void;
}

// ---------- Entry Point ----------

export function renderMaintenance(props: MaintenanceViewProps) {
  return html`
    <div class="tab-view">
      <header class="tab-header">
        <h2>${t("maintenance.title")}</h2>
        <p class="tab-subtitle">${t("maintenance.subtitle")}</p>
      </header>

      <section class="security-section">
        <h3 class="security-section__title">${t("maintenance.backup.title")}</h3>
        <p class="security-section__desc">${t("maintenance.backup.desc")}</p>
        ${renderBackupSection(props)}
      </section>

      <section class="security-section" style="margin-top: 2rem;">
        <h3 class="security-section__title">${t("maintenance.reset.title")}</h3>
        <p class="security-section__desc">${t("maintenance.reset.desc")}</p>
        ${renderResetSection(props)}
      </section>

      ${renderConfirmDialog(props)}
    </div>
  `;
}

// ---------- Backup Section ----------

function renderBackupSection(props: MaintenanceViewProps) {
  const { state } = props;

  return html`
    <div style="margin-top: 1rem;">
      <button
        class="btn"
        ?disabled=${state.backupsLoading}
        @click=${() => props.onLoadBackups()}
      >
        ${state.backupsLoading ? t("maintenance.loading") : t("maintenance.backup.refresh")}
      </button>

      ${state.backupsError
      ? html`<div class="alert alert--error" style="margin-top: 0.5rem;">${state.backupsError}</div>`
      : nothing}

      ${state.restoreResult
      ? html`<div class="alert alert--success" style="margin-top: 0.5rem;">${state.restoreResult}</div>`
      : nothing}

      ${state.backups.length > 0
      ? html`
          <table class="table" style="margin-top: 1rem; width: 100%;">
            <thead>
              <tr>
                <th>#</th>
                <th>${t("maintenance.backup.date")}</th>
                <th>${t("maintenance.backup.size")}</th>
                <th>${t("maintenance.backup.valid")}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              ${state.backups.map((b: BackupEntry) => html`
                <tr>
                  <td>${b.index}</td>
                  <td>${formatDate(b.modTime)}</td>
                  <td>${formatSize(b.size)}</td>
                  <td>${b.valid ? "✓" : "✗"}</td>
                  <td>
                    <button
                      class="btn btn--sm"
                      ?disabled=${!b.valid || state.restoring}
                      @click=${() => props.onConfirmOpen("restore", b.index)}
                    >
                      ${t("maintenance.backup.restore")}
                    </button>
                  </td>
                </tr>
              `)}
            </tbody>
          </table>
        `
      : state.backupsLoading
        ? nothing
        : html`<p style="margin-top: 1rem; color: var(--text-secondary);">${t("maintenance.backup.empty")}</p>`}
    </div>
  `;
}

// ---------- Reset Section ----------

function renderResetSection(props: MaintenanceViewProps) {
  const { state } = props;

  return html`
    <div style="margin-top: 1rem;">
      <button
        class="btn"
        ?disabled=${state.resetPreviewLoading}
        @click=${() => props.onPreviewReset()}
      >
        ${state.resetPreviewLoading ? t("maintenance.loading") : t("maintenance.reset.preview")}
      </button>

      ${state.resetResult
      ? html`<div class="alert alert--success" style="margin-top: 0.5rem;">${state.resetResult}</div>`
      : nothing}

      ${state.resetTargets.length > 0
      ? html`
          <table class="table" style="margin-top: 1rem; width: 100%;">
            <thead>
              <tr>
                <th>${t("maintenance.reset.file")}</th>
                <th>${t("maintenance.reset.size")}</th>
                <th>${t("maintenance.reset.action")}</th>
                <th>${t("maintenance.reset.status")}</th>
              </tr>
            </thead>
            <tbody>
              ${state.resetTargets.map((target: ResetTarget) => html`
                <tr>
                  <td style="font-family: monospace; font-size: 0.85em;">${abbreviatePath(target.path)}</td>
                  <td>${target.exists ? formatSize(target.size) : "—"}</td>
                  <td><span class="badge badge--${target.action === "delete" ? "danger" : "warn"}">${target.action}</span></td>
                  <td>${target.exists ? t("maintenance.reset.exists") : t("maintenance.reset.missing")}</td>
                </tr>
              `)}
            </tbody>
          </table>
          <button
            class="btn danger"
            style="margin-top: 1rem;"
            ?disabled=${state.resetting || !state.resetTargets.some((rt: ResetTarget) => rt.exists)}
            @click=${() => props.onConfirmOpen("reset")}
          >
            ${state.resetting ? t("maintenance.loading") : t("maintenance.reset.execute")}
          </button>
        `
      : nothing}
    </div>
  `;
}

// ---------- Confirm Dialog ----------

function renderConfirmDialog(props: MaintenanceViewProps) {
  if (!props.state.confirmOpen) return nothing;

  const isValid = props.state.confirmText.trim().toUpperCase() === "CONFIRM";
  const title = props.state.confirmAction === "restore"
    ? t("maintenance.confirm.restoreTitle")
    : t("maintenance.confirm.resetTitle");
  const warning = props.state.confirmAction === "restore"
    ? t("maintenance.confirm.restoreWarning")
    : t("maintenance.confirm.resetWarning");

  return html`
    <div class="security-confirm-overlay" @click=${(e: Event) => {
      if ((e.target as HTMLElement).classList.contains("security-confirm-overlay")) {
        props.onConfirmCancel();
      }
    }}>
      <div class="security-confirm-dialog" role="alertdialog">
        <div class="security-confirm-header">
          <span class="security-confirm-icon">⚠️</span>
          <h3>${title}</h3>
        </div>
        <div class="security-confirm-body">
          <p>${warning}</p>
          <div class="security-confirm-input-group">
            <label for="maintenance-confirm-input">${t("maintenance.confirm.prompt")}</label>
            <input
              id="maintenance-confirm-input"
              type="text"
              class="security-confirm-input"
              placeholder="CONFIRM"
              .value=${props.state.confirmText}
              @input=${(e: Event) =>
                props.onConfirmTextChange((e.target as HTMLInputElement).value)}
              @keydown=${(e: KeyboardEvent) => {
                if (e.key === "Enter" && isValid) {
                  props.onConfirmSubmit();
                }
              }}
            />
          </div>
        </div>
        <div class="security-confirm-actions">
          <button class="btn" @click=${props.onConfirmCancel}>
            ${t("maintenance.confirm.cancel")}
          </button>
          <button
            class="btn danger"
            ?disabled=${!isValid || props.loading}
            @click=${props.onConfirmSubmit}
          >
            ${t("maintenance.confirm.submit")}
          </button>
        </div>
      </div>
    </div>
  `;
}

// ---------- Helpers ----------

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString();
  } catch {
    return iso;
  }
}

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

function abbreviatePath(path: string): string {
  const home = "~";
  const homeDir = path.match(/^(\/Users\/[^/]+|\/home\/[^/]+)/)?.[0];
  if (homeDir) return path.replace(homeDir, home);
  return path;
}
