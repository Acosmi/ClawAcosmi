import { html, nothing } from "lit";
import type { ResultReviewRequest } from "../controllers/result-review.ts";

// ---------- Result Review Popup ----------
// Phase 3: 三级指挥体系 — 结果签收弹窗

export interface ResultReviewPopupProps {
  queue: ResultReviewRequest[];
  onApprove: (id: string) => void;
  onReject: (id: string, feedback: string) => void;
}

export function renderResultReviewPopup(props: ResultReviewPopupProps) {
  if (!props.queue.length) return nothing;

  const req = props.queue[0]; // 处理队首
  const remaining = Math.max(0, Math.ceil((req.expiresAtMs - Date.now()) / 1000));
  const mins = Math.floor(remaining / 60);
  const secs = remaining % 60;

  const filesModified = req.artifacts?.files_modified ?? [];
  const filesCreated = req.artifacts?.files_created ?? [];
  const hasFiles = filesModified.length > 0 || filesCreated.length > 0;

  return html`
    <div class="escalation-overlay" @click=${(e: Event) => {
        if ((e.target as HTMLElement).classList.contains("escalation-overlay")) {
          // 点击背景不关闭（结果签收需明确决策）
        }
      }}>
      <div class="escalation-popup" style="max-width: 560px;">
        <div class="escalation-popup__header">
          <span class="escalation-popup__icon">📦</span>
          <h3>结果签收</h3>
          <span style="margin-left:auto; font-size:0.8em; opacity:0.6">${mins}:${secs.toString().padStart(2, "0")}</span>
        </div>

        <div class="escalation-popup__body">
          ${req.originalTask ? html`
            <div class="escalation-popup__field">
              <strong>原始任务</strong>
              <p style="white-space:pre-wrap; max-height:80px; overflow-y:auto; font-size:0.9em;">${req.originalTask}</p>
            </div>
          ` : nothing}

          ${req.reviewSummary ? html`
            <div class="escalation-popup__level" style="border-left: 3px solid var(--color-success, #22c55e)">
              <strong>质量审核</strong>
              <span style="font-size:0.9em;">${req.reviewSummary}</span>
            </div>
          ` : nothing}

          ${req.result ? html`
            <div class="escalation-popup__field">
              <strong>执行结果</strong>
              <pre style="white-space:pre-wrap; max-height:200px; overflow-y:auto; font-size:0.85em; background:var(--bg-secondary, #1a1a2e); padding:8px; border-radius:4px; margin:4px 0 0;">${req.result}</pre>
            </div>
          ` : nothing}

          ${hasFiles ? html`
            <div class="escalation-popup__field">
              <strong>文件变更</strong>
              <div style="font-size:0.85em; max-height:120px; overflow-y:auto;">
                ${filesModified.map((f) => html`
                  <div><span style="color:var(--color-warning, #f59e0b)">M</span> <code>${f}</code></div>
                `)}
                ${filesCreated.map((f) => html`
                  <div><span style="color:var(--color-success, #22c55e)">+</span> <code>${f}</code></div>
                `)}
              </div>
            </div>
          ` : nothing}

          ${req.contractId ? html`
            <div style="font-size:0.75em; opacity:0.4; margin-top:4px;">
              合约: ${req.contractId.substring(0, 8)}...
            </div>
          ` : nothing}
        </div>

        <div class="escalation-popup__actions">
          <button
            class="btn btn--primary"
            @click=${() => props.onApprove(req.id)}
          >
            ✅ 签收通过
          </button>
          <button
            class="btn btn--outline"
            @click=${() => {
              const feedback = window.prompt("请输入退回理由（可选）：") ?? "";
              props.onReject(req.id, feedback);
            }}
          >
            ↩️ 退回修改
          </button>
        </div>

        ${props.queue.length > 1 ? html`
          <div style="text-align:center; font-size:0.8em; opacity:0.5; padding-top:8px;">
            还有 ${props.queue.length - 1} 个结果待签收
          </div>
        ` : nothing}
      </div>
    </div>
  `;
}
