import { html, nothing } from "lit";
import type { PlanConfirmRequest } from "../controllers/plan-confirmation.ts";

// ---------- Plan Confirmation Popup ----------
// Phase 1: 三级指挥体系 — 方案确认弹窗

export interface PlanConfirmPopupProps {
  queue: PlanConfirmRequest[];
  onApprove: (id: string) => void;
  onReject: (id: string) => void;
  onEdit: (id: string, editedPlan: string) => void;
}

const TIER_LABELS: Record<string, string> = {
  task_write: "创建/修改",
  task_delete: "删除操作",
  task_multimodal: "视觉/浏览器",
};

const TIER_COLORS: Record<string, string> = {
  task_write: "var(--color-primary, #3b82f6)",
  task_delete: "var(--color-danger, #ef4444)",
  task_multimodal: "var(--color-info, #8b5cf6)",
};

export function renderPlanConfirmPopup(props: PlanConfirmPopupProps) {
  if (!props.queue.length) return nothing;

  const req = props.queue[0]; // 处理队首
  const tierLabel = TIER_LABELS[req.intentTier] ?? req.intentTier;
  const tierColor = TIER_COLORS[req.intentTier] ?? "var(--text-secondary)";
  const remaining = Math.max(0, Math.ceil((req.expiresAtMs - Date.now()) / 1000));
  const mins = Math.floor(remaining / 60);
  const secs = remaining % 60;

  return html`
    <div class="escalation-overlay" @click=${(e: Event) => {
        if ((e.target as HTMLElement).classList.contains("escalation-overlay")) {
          props.onReject(req.id);
        }
      }}>
      <div class="escalation-popup" style="max-width: 520px;">
        <div class="escalation-popup__header">
          <span class="escalation-popup__icon">📋</span>
          <h3>方案确认</h3>
          <span style="margin-left:auto; font-size:0.8em; opacity:0.6">${mins}:${secs.toString().padStart(2, "0")}</span>
        </div>

        <div class="escalation-popup__body">
          <div class="escalation-popup__level" style="border-left: 3px solid ${tierColor}">
            <strong>意图类型</strong>
            <span style="color: ${tierColor}">${tierLabel}</span>
          </div>

          ${req.taskBrief ? html`
            <div class="escalation-popup__field">
              <strong>任务描述</strong>
              <p style="white-space:pre-wrap; max-height:120px; overflow-y:auto;">${req.taskBrief}</p>
            </div>
          ` : nothing}

          ${req.planSteps.length > 0 ? html`
            <div class="escalation-popup__field">
              <strong>执行方案 (${req.planSteps.length} 步)</strong>
              <ol style="margin:4px 0 0 16px; padding:0; font-size:0.9em; max-height:200px; overflow-y:auto;">
                ${req.planSteps.map((step) => html`<li style="margin-bottom:4px">${step}</li>`)}
              </ol>
            </div>
          ` : nothing}

          ${req.estimatedScope?.length ? html`
            <div class="escalation-popup__field">
              <strong>预估范围</strong>
              <div style="font-size:0.85em; opacity:0.8">
                ${req.estimatedScope.map((s) => html`
                  <div><code>${s.path}</code> <span style="opacity:0.5">(${s.type})</span></div>
                `)}
              </div>
            </div>
          ` : nothing}
        </div>

        <div class="escalation-popup__actions">
          <button
            class="btn btn--primary"
            @click=${() => props.onApprove(req.id)}
          >
            ✅ 批准执行
          </button>
          <button
            class="btn btn--outline"
            @click=${() => props.onReject(req.id)}
          >
            ❌ 拒绝
          </button>
        </div>

        ${props.queue.length > 1 ? html`
          <div style="text-align:center; font-size:0.8em; opacity:0.5; padding-top:8px;">
            还有 ${props.queue.length - 1} 个方案待确认
          </div>
        ` : nothing}
      </div>
    </div>
  `;
}
