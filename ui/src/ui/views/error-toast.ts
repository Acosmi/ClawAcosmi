// error-toast.ts — 错误 Toast 组件 (S2-2: GW-UI-D1)
// 当 chat 请求失败时，显示浮动可关闭的错误提示。
// 3 秒后自动消失，可点击展开查看完整详情。

import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import { icons } from "../icons.ts";

const ERROR_TOAST_AUTO_DISMISS_MS = 5000;

let _expanded = false;
let _dismissTimer: ReturnType<typeof setTimeout> | null = null;

/** 重置 toast 内部状态 */
function resetState(): void {
    _expanded = false;
    if (_dismissTimer) {
        clearTimeout(_dismissTimer);
        _dismissTimer = null;
    }
}

/** 渲染错误 Toast */
export function renderErrorToast(
    error: string | null,
    onDismiss: () => void,
    requestUpdate?: () => void,
) {
    if (!error) {
        resetState();
        return nothing;
    }

    // 启动自动消失计时器（仅第一次渲染时启动）
    if (!_dismissTimer) {
        _dismissTimer = setTimeout(() => {
            _dismissTimer = null;
            onDismiss();
        }, ERROR_TOAST_AUTO_DISMISS_MS);
    }

    const handleDismiss = () => {
        resetState();
        onDismiss();
    };

    const handleExpand = () => {
        _expanded = !_expanded;
        // 展开时取消自动关闭
        if (_expanded && _dismissTimer) {
            clearTimeout(_dismissTimer);
            _dismissTimer = null;
        }
        requestUpdate?.();
    };

    // 判断是否需要展开按钮（错误消息超过 80 字符）
    const isLong = error.length > 80;

    return html`
    <div class="error-toast-container" role="alert" aria-live="assertive">
      <div class="error-toast">
        <span class="error-toast__icon">⚠️</span>
        <div class="error-toast__body">
          <div class="error-toast__title">${t("chat.error.title")}</div>
          <div class="error-toast__message ${_expanded ? "error-toast__message--expanded" : ""}">
            ${error}
          </div>
        </div>
        <div class="error-toast__actions">
          ${isLong
            ? html`
              <button
                class="error-toast__expand"
                type="button"
                @click=${handleExpand}
              >
                ${_expanded ? t("chat.error.collapse") : t("chat.error.details")}
              </button>
            `
            : nothing}
          <button
            class="error-toast__dismiss"
            type="button"
            aria-label="${t("chat.error.dismiss")}"
            @click=${handleDismiss}
          >
            ${icons.x}
          </button>
        </div>
      </div>
    </div>
  `;
}
