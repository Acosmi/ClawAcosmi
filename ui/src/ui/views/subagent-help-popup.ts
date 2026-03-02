import { html, nothing } from "lit";
import type { SubagentHelpRequest } from "../controllers/subagent-help.ts";

// ---------- Subagent Help Popup ----------
// Phase 4: 三级指挥体系 — 子智能体求助弹窗

export interface SubagentHelpPopupProps {
  queue: SubagentHelpRequest[];
  onRespond: (id: string, response: string) => void;
}

export function renderSubagentHelpPopup(props: SubagentHelpPopupProps) {
  if (!props.queue.length) return nothing;

  const req = props.queue[0]; // 处理队首
  const elapsed = Math.floor((Date.now() - req.ts) / 1000);
  const mins = Math.floor(elapsed / 60);
  const secs = elapsed % 60;

  return html`
    <div class="escalation-overlay" @click=${(e: Event) => {
        if ((e.target as HTMLElement).classList.contains("escalation-overlay")) {
          // 点击背景不关闭（需明确回复）
        }
      }}>
      <div class="escalation-popup" style="max-width: 520px;">
        <div class="escalation-popup__header">
          <span class="escalation-popup__icon">🆘</span>
          <h3>子智能体求助</h3>
          <span style="margin-left:auto; font-size:0.8em; opacity:0.6">${mins}:${secs.toString().padStart(2, "0")} ago</span>
        </div>

        <div class="escalation-popup__body">
          ${req.label ? html`
            <div style="font-size:0.8em; opacity:0.6; margin-bottom:8px;">
              来自: <strong>${req.label}</strong>
            </div>
          ` : nothing}

          <div class="escalation-popup__field">
            <strong>问题</strong>
            <p style="white-space:pre-wrap; font-size:0.95em;">${req.question}</p>
          </div>

          ${req.context ? html`
            <div class="escalation-popup__field">
              <strong>上下文</strong>
              <p style="white-space:pre-wrap; max-height:100px; overflow-y:auto; font-size:0.85em; opacity:0.8;">${req.context}</p>
            </div>
          ` : nothing}

          ${req.options && req.options.length > 0 ? html`
            <div class="escalation-popup__field">
              <strong>建议选项</strong>
              <div style="display:flex; flex-direction:column; gap:4px; margin-top:4px;">
                ${req.options.map((opt, i) => html`
                  <button
                    class="btn btn--outline"
                    style="text-align:left; font-size:0.85em; padding:6px 10px;"
                    @click=${() => props.onRespond(req.id, opt)}
                  >
                    ${i + 1}. ${opt}
                  </button>
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
            @click=${() => {
              const response = window.prompt("请输入回复：");
              if (response !== null && response.trim()) {
                props.onRespond(req.id, response.trim());
              }
            }}
          >
            📝 输入回复
          </button>
        </div>

        ${props.queue.length > 1 ? html`
          <div style="text-align:center; font-size:0.8em; opacity:0.5; padding-top:8px;">
            还有 ${props.queue.length - 1} 个求助待回复
          </div>
        ` : nothing}
      </div>
    </div>
  `;
}
