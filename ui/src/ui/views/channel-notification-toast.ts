// channel-notification-toast.ts — 跨会话频道消息通知 Toast
// 当飞书等远程频道收到消息时，在当前聊天视图顶部弹出通知。
// 5 秒后自动消失，点击"查看"跳转到对应会话。

import { html, nothing } from "lit";
import { icons } from "../icons.ts";

const CHANNEL_TOAST_AUTO_DISMISS_MS = 5000;

let _dismissTimer: ReturnType<typeof setTimeout> | null = null;

function resetState(): void {
  if (_dismissTimer) {
    clearTimeout(_dismissTimer);
    _dismissTimer = null;
  }
}

export interface ChannelNotification {
  sessionKey: string;
  channel: string;
  text: string;
  from: string;
  label: string;
  ts: number;
}

/** 渲染频道消息通知 Toast */
export function renderChannelNotificationToast(
  notification: ChannelNotification | null,
  currentSessionKey: string,
  onView: (sessionKey: string) => void,
  onDismiss: () => void,
) {
  if (!notification) {
    resetState();
    return nothing;
  }

  // 当前会话的消息不弹通知
  if (notification.sessionKey === currentSessionKey) {
    resetState();
    return nothing;
  }

  // 启动自动消失计时器
  if (!_dismissTimer) {
    _dismissTimer = setTimeout(() => {
      _dismissTimer = null;
      onDismiss();
    }, CHANNEL_TOAST_AUTO_DISMISS_MS);
  }

  const handleView = () => {
    resetState();
    onView(notification.sessionKey);
  };

  const handleDismiss = () => {
    resetState();
    onDismiss();
  };

  const preview = notification.text.length > 60
    ? notification.text.slice(0, 60) + "..."
    : notification.text;

  return html`
    <div class="error-toast-container" role="status" aria-live="polite">
      <div class="error-toast" style="border-left: 3px solid var(--accent, #3b82f6);">
        <span class="error-toast__icon">💬</span>
        <div class="error-toast__body">
          <div class="error-toast__title">[${notification.label}] ${notification.from}</div>
          <div class="error-toast__message">${preview}</div>
        </div>
        <div class="error-toast__actions">
          <button
            class="error-toast__expand"
            type="button"
            @click=${handleView}
          >
            查看
          </button>
          <button
            class="error-toast__dismiss"
            type="button"
            aria-label="关闭"
            @click=${handleDismiss}
          >
            ${icons.x}
          </button>
        </div>
      </div>
    </div>
  `;
}
