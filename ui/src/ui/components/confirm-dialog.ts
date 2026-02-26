/**
 * 自定义 confirm / prompt 对话框
 *
 * 用原生 <dialog> 元素替代 window.confirm / window.prompt，
 * 以支持 i18n 按钮文字和统一 UI 风格。
 */

import { t } from "../i18n.ts";

/* ── helpers ── */

function createDialog(): HTMLDialogElement {
  const dialog = document.createElement("dialog");
  dialog.classList.add("custom-dialog");
  return dialog;
}

/* ── showConfirmDialog ── */

export async function showConfirmDialog(message: string): Promise<boolean> {
  const dialog = createDialog();
  dialog.innerHTML = `
    <div class="custom-dialog__body">${escapeHtml(message)}</div>
    <div class="custom-dialog__actions">
      <button class="btn custom-dialog__cancel">${t("dialog.cancel")}</button>
      <button class="btn primary custom-dialog__ok">${t("dialog.confirm")}</button>
    </div>
  `;

  document.body.appendChild(dialog);
  dialog.showModal();

  return new Promise<boolean>((resolve) => {
    const cleanup = () => {
      dialog.close();
      dialog.remove();
    };
    dialog.querySelector(".custom-dialog__ok")!.addEventListener("click", () => {
      cleanup();
      resolve(true);
    });
    dialog.querySelector(".custom-dialog__cancel")!.addEventListener("click", () => {
      cleanup();
      resolve(false);
    });
    dialog.addEventListener("cancel", () => {
      cleanup();
      resolve(false);
    });
  });
}

/* ── showPromptDialog ── */

export async function showPromptDialog(
  message: string,
  defaultValue = "",
): Promise<string | null> {
  const dialog = createDialog();
  dialog.innerHTML = `
    <div class="custom-dialog__body">${escapeHtml(message)}</div>
    <input class="custom-dialog__input" type="text" value="${escapeAttr(defaultValue)}" />
    <div class="custom-dialog__actions">
      <button class="btn custom-dialog__cancel">${t("dialog.cancel")}</button>
      <button class="btn primary custom-dialog__ok">${t("dialog.ok")}</button>
    </div>
  `;

  document.body.appendChild(dialog);
  dialog.showModal();

  const input = dialog.querySelector<HTMLInputElement>(".custom-dialog__input")!;
  input.select();

  return new Promise<string | null>((resolve) => {
    const cleanup = () => {
      dialog.close();
      dialog.remove();
    };
    dialog.querySelector(".custom-dialog__ok")!.addEventListener("click", () => {
      const val = input.value;
      cleanup();
      resolve(val);
    });
    dialog.querySelector(".custom-dialog__cancel")!.addEventListener("click", () => {
      cleanup();
      resolve(null);
    });
    dialog.addEventListener("cancel", () => {
      cleanup();
      resolve(null);
    });
  });
}

/* ── util ── */

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/\n/g, "<br>");
}

function escapeAttr(s: string): string {
  return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;");
}
