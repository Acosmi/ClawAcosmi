// ---------- 结果签收门控控制器 ----------
// Phase 3: 三级指挥体系 — 结果签收事件解析 + 队列管理
// 复用 plan-confirmation.ts 的模式。

export type ResultReviewRequest = {
  id: string;
  originalTask: string;
  contractId: string;
  result: string;
  artifacts?: {
    files_modified?: string[];
    files_created?: string[];
    commands_run?: string[];
  } | null;
  reviewSummary?: string;
  createdAtMs: number;
  expiresAtMs: number;
};

export type ResultReviewDecision = {
  action: "approve" | "reject";
  feedback?: string;
};

export type ResultReviewResolved = {
  id: string;
  decision: ResultReviewDecision;
  ts?: number | null;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * 解析 "result.approve.requested" WebSocket 事件 payload。
 */
export function parseResultReviewRequested(payload: unknown): ResultReviewRequest | null {
  if (!isRecord(payload)) return null;

  const id = typeof payload.id === "string" ? payload.id.trim() : "";
  const originalTask = typeof payload.originalTask === "string" ? payload.originalTask : "";
  const contractId = typeof payload.contractId === "string" ? payload.contractId : "";
  const result = typeof payload.result === "string" ? payload.result : "";
  if (!id) return null;

  const createdAtMs = typeof payload.createdAtMs === "number" ? payload.createdAtMs : Date.now();
  const expiresAtMs = typeof payload.expiresAtMs === "number" ? payload.expiresAtMs : Date.now() + 180_000;

  let artifacts: ResultReviewRequest["artifacts"] = null;
  if (isRecord(payload.artifacts)) {
    const a = payload.artifacts;
    artifacts = {
      files_modified: Array.isArray(a.files_modified) ? a.files_modified.filter((s): s is string => typeof s === "string") : undefined,
      files_created: Array.isArray(a.files_created) ? a.files_created.filter((s): s is string => typeof s === "string") : undefined,
      commands_run: Array.isArray(a.commands_run) ? a.commands_run.filter((s): s is string => typeof s === "string") : undefined,
    };
  }

  const reviewSummary = typeof payload.reviewSummary === "string" ? payload.reviewSummary : undefined;

  return { id, originalTask, contractId, result, artifacts, reviewSummary, createdAtMs, expiresAtMs };
}

/**
 * 解析 "result.approve.resolved" WebSocket 事件 payload。
 */
export function parseResultReviewResolved(payload: unknown): ResultReviewResolved | null {
  if (!isRecord(payload)) return null;
  const id = typeof payload.id === "string" ? payload.id.trim() : "";
  if (!id) return null;

  const rawDecision = isRecord(payload.decision) ? payload.decision : {};
  const action = typeof rawDecision.action === "string" ? rawDecision.action : "approve";

  return {
    id,
    decision: {
      action: action as "approve" | "reject",
      feedback: typeof rawDecision.feedback === "string" ? rawDecision.feedback : undefined,
    },
    ts: typeof payload.ts === "number" ? payload.ts : null,
  };
}

function pruneQueue(queue: ResultReviewRequest[]): ResultReviewRequest[] {
  const now = Date.now();
  return queue.filter((entry) => entry.expiresAtMs > now);
}

export function addResultReview(
  queue: ResultReviewRequest[],
  entry: ResultReviewRequest,
): ResultReviewRequest[] {
  const next = pruneQueue(queue).filter((item) => item.id !== entry.id);
  next.push(entry);
  return next;
}

export function removeResultReview(
  queue: ResultReviewRequest[],
  id: string,
): ResultReviewRequest[] {
  return pruneQueue(queue).filter((entry) => entry.id !== id);
}
