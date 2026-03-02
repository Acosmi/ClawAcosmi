// ---------- 子智能体求助控制器 ----------
// Phase 4: 三级指挥体系 — 子智能体求助事件解析 + 队列管理
// 复用 result-review.ts 的模式。

export type SubagentHelpRequest = {
  id: string;
  contractId: string;
  question: string;
  context?: string;
  options?: string[];
  label?: string;
  ts: number;
};

export type SubagentHelpResolved = {
  id: string;
  response: string;
  ts?: number | null;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * 解析 "subagent.help.requested" WebSocket 事件 payload。
 */
export function parseSubagentHelpRequested(payload: unknown): SubagentHelpRequest | null {
  if (!isRecord(payload)) return null;

  const id = typeof payload.id === "string" ? payload.id.trim() : "";
  if (!id) return null;

  const contractId = typeof payload.contractId === "string" ? payload.contractId : "";
  const question = typeof payload.question === "string" ? payload.question : "";
  const context = typeof payload.context === "string" ? payload.context : undefined;
  const label = typeof payload.label === "string" ? payload.label : undefined;
  const ts = typeof payload.ts === "number" ? payload.ts : Date.now();

  let options: string[] | undefined;
  if (Array.isArray(payload.options)) {
    options = payload.options.filter((s): s is string => typeof s === "string");
    if (options.length === 0) options = undefined;
  }

  return { id, contractId, question, context, options, label, ts };
}

/**
 * 解析 "subagent.help.resolved" WebSocket 事件 payload。
 */
export function parseSubagentHelpResolved(payload: unknown): SubagentHelpResolved | null {
  if (!isRecord(payload)) return null;
  const id = typeof payload.id === "string" ? payload.id.trim() : "";
  if (!id) return null;

  const response = typeof payload.response === "string" ? payload.response : "";
  return {
    id,
    response,
    ts: typeof payload.ts === "number" ? payload.ts : null,
  };
}

/**
 * 添加求助请求到队列（去重 + 按时间排序）。
 */
export function addSubagentHelp(
  queue: SubagentHelpRequest[],
  entry: SubagentHelpRequest,
): SubagentHelpRequest[] {
  const next = queue.filter((item) => item.id !== entry.id);
  next.push(entry);
  return next;
}

/**
 * 从队列中移除已解决的求助请求。
 */
export function removeSubagentHelp(
  queue: SubagentHelpRequest[],
  id: string,
): SubagentHelpRequest[] {
  return queue.filter((entry) => entry.id !== id);
}
