import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { TaskKanbanItem, TaskKanbanState } from "../controllers/task-kanban.ts";
import { groupByColumn } from "../controllers/task-kanban.ts";

// ---------- Task Kanban View ----------

export type TaskKanbanProps = {
  kanbanState: TaskKanbanState;
  onPrune: () => void;
};

export function renderTaskKanban(props: TaskKanbanProps) {
  const { queued, running, done } = groupByColumn(props.kanbanState);
  const total = queued.length + running.length + done.length;

  return html`
    <section class="card">
      <div class="row" style="justify-content: space-between;">
        <div>
          <div class="card-title">${t("tasks.title")}</div>
          <div class="card-sub">${t("tasks.subtitle")}</div>
        </div>
        <div class="row" style="gap: 8px;">
          ${done.length > 0
            ? html`<button class="btn btn-sm" @click=${props.onPrune}>
                ${t("tasks.prune")}
              </button>`
            : nothing}
        </div>
      </div>

      ${total === 0
        ? html`<div class="muted" style="margin-top: 16px;">${t("tasks.empty")}</div>`
        : html`
          <div class="task-kanban" style="margin-top: 16px; display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 12px;">
            ${renderColumn(t("tasks.col.queued"), queued, "queued")}
            ${renderColumn(t("tasks.col.running"), running, "running")}
            ${renderColumn(t("tasks.col.done"), done, "done")}
          </div>
        `}
    </section>
  `;
}

function renderColumn(title: string, items: TaskKanbanItem[], column: string) {
  return html`
    <div class="task-column" data-column=${column}>
      <div class="card-sub" style="margin-bottom: 8px; font-weight: 600;">
        ${title}
        <span class="badge" style="margin-left: 4px;">${items.length}</span>
      </div>
      ${items.length === 0
        ? html`<div class="muted" style="font-size: 12px; padding: 8px 0;">—</div>`
        : items.map((task) => renderTaskCard(task))}
    </div>
  `;
}

function renderTaskCard(task: TaskKanbanItem) {
  const statusClass = task.status === "failed" ? "danger"
    : task.status === "completed" ? "success"
    : task.status === "queued" ? "muted"
    : "info";

  const elapsed = formatElapsed(task);

  return html`
    <div class="task-card" style="
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 8px 10px;
      margin-bottom: 6px;
      font-size: 13px;
      background: var(--bg-secondary, var(--bg));
    ">
      <div class="row" style="justify-content: space-between; align-items: center; gap: 6px;">
        <span class="badge badge-${statusClass}" style="font-size: 11px;">
          ${statusLabel(task.status)}
        </span>
        ${task.async ? html`<span class="badge" style="font-size: 10px;">async</span>` : nothing}
      </div>

      ${task.text
        ? html`<div style="margin-top: 4px; word-break: break-word; color: var(--text);">
            ${truncate(task.text, 80)}
          </div>`
        : nothing}

      ${task.toolName
        ? html`<div style="margin-top: 4px; font-size: 12px; color: var(--text-muted);">
            <code>${task.toolName}</code>
            ${task.phase ? html` · ${task.phase}` : nothing}
            ${task.isError ? html` · <span style="color: var(--danger);">error</span>` : nothing}
          </div>`
        : nothing}

      ${task.summary
        ? html`<div style="margin-top: 4px; font-size: 12px; color: var(--text-muted);">
            ${truncate(task.summary, 120)}
          </div>`
        : nothing}

      ${task.error
        ? html`<div style="margin-top: 4px; font-size: 12px; color: var(--danger);">
            ${truncate(task.error, 120)}
          </div>`
        : nothing}

      ${elapsed
        ? html`<div style="margin-top: 4px; font-size: 11px; color: var(--text-muted);">${elapsed}</div>`
        : nothing}
    </div>
  `;
}

function statusLabel(status: string): string {
  switch (status) {
    case "queued": return "⏳";
    case "started": return "▶";
    case "progress": return "⚙";
    case "completed": return "✓";
    case "failed": return "✗";
    default: return status;
  }
}

function formatElapsed(task: TaskKanbanItem): string {
  if (task.status === "completed" || task.status === "failed") {
    if (task.startedAt && task.completedAt) {
      const ms = task.completedAt - task.startedAt;
      return formatDuration(ms);
    }
    return "";
  }
  if (task.startedAt) {
    const ms = Date.now() - task.startedAt;
    return `${formatDuration(ms)}…`;
  }
  return "";
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const secs = Math.floor(ms / 1000);
  if (secs < 60) return `${secs}s`;
  const mins = Math.floor(secs / 60);
  const remainSecs = secs % 60;
  return `${mins}m${remainSecs > 0 ? ` ${remainSecs}s` : ""}`;
}

function truncate(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text;
  return text.slice(0, maxLen) + "…";
}
