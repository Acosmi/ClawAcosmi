// ---------- 方案确认门控控制器 ----------
// Phase 1: 三级指挥体系 — 方案确认事件解析 + 队列管理
// 复用 coder-confirmation.ts 的模式。

export type PlanConfirmRequest = {
  id: string;
  taskBrief: string;
  planSteps: string[];
  estimatedScope?: { path: string; type: string }[];
  intentTier: string;
  createdAtMs: number;
  expiresAtMs: number;
};

export type PlanConfirmDecision = {
  action: "approve" | "reject" | "edit";
  editedPlan?: string;
  feedback?: string;
};

export type PlanConfirmResolved = {
  id: string;
  decision: PlanConfirmDecision;
  ts?: number | null;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * 解析 "plan.confirm.requested" WebSocket 事件 payload。
 */
export function parsePlanConfirmRequested(payload: unknown): PlanConfirmRequest | null {
  if (!isRecord(payload)) return null;

  const id = typeof payload.id === "string" ? payload.id.trim() : "";
  const taskBrief = typeof payload.taskBrief === "string" ? payload.taskBrief : "";
  const intentTier = typeof payload.intentTier === "string" ? payload.intentTier : "";
  if (!id) return null;

  const createdAtMs = typeof payload.createdAtMs === "number" ? payload.createdAtMs : Date.now();
  const expiresAtMs = typeof payload.expiresAtMs === "number" ? payload.expiresAtMs : Date.now() + 300_000;

  let planSteps: string[] = [];
  if (Array.isArray(payload.planSteps)) {
    planSteps = payload.planSteps.filter((s): s is string => typeof s === "string");
  }

  let estimatedScope: { path: string; type: string }[] | undefined;
  if (Array.isArray(payload.estimatedScope)) {
    estimatedScope = payload.estimatedScope
      .filter(isRecord)
      .map((s) => ({
        path: typeof s.path === "string" ? s.path : "",
        type: typeof s.type === "string" ? s.type : "",
      }))
      .filter((s) => s.path);
  }

  return { id, taskBrief, planSteps, estimatedScope, intentTier, createdAtMs, expiresAtMs };
}

/**
 * 解析 "plan.confirm.resolved" WebSocket 事件 payload。
 */
export function parsePlanConfirmResolved(payload: unknown): PlanConfirmResolved | null {
  if (!isRecord(payload)) return null;
  const id = typeof payload.id === "string" ? payload.id.trim() : "";
  if (!id) return null;

  const rawDecision = isRecord(payload.decision) ? payload.decision : {};
  const action = typeof rawDecision.action === "string" ? rawDecision.action : "reject";

  return {
    id,
    decision: {
      action: action as "approve" | "reject" | "edit",
      editedPlan: typeof rawDecision.editedPlan === "string" ? rawDecision.editedPlan : undefined,
      feedback: typeof rawDecision.feedback === "string" ? rawDecision.feedback : undefined,
    },
    ts: typeof payload.ts === "number" ? payload.ts : null,
  };
}

function pruneQueue(queue: PlanConfirmRequest[]): PlanConfirmRequest[] {
  const now = Date.now();
  return queue.filter((entry) => entry.expiresAtMs > now);
}

export function addPlanConfirm(
  queue: PlanConfirmRequest[],
  entry: PlanConfirmRequest,
): PlanConfirmRequest[] {
  const next = pruneQueue(queue).filter((item) => item.id !== entry.id);
  next.push(entry);
  return next;
}

export function removePlanConfirm(
  queue: PlanConfirmRequest[],
  id: string,
): PlanConfirmRequest[] {
  return pruneQueue(queue).filter((entry) => entry.id !== id);
}
