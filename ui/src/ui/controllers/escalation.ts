/**
 * Escalation controller — P2: 智能体即时授权 + UI 弹窗 + 自动降权
 *
 * Manages escalation popup state and API calls for:
 *   - exec.approval.requested (escalation variant, id starts with "esc_")
 *   - exec.approval.resolved (escalation variant)
 *   - security.escalation.resolve / status / revoke
 */

import type { GatewayBrowserClient } from "../gateway.ts";

// ---------- Types ----------

export type EscalationRequest = {
    id: string;
    requestedLevel: string; // "allowlist" | "full"
    reason: string;
    runId?: string;
    sessionId?: string;
    requestedAt: number; // ms
    ttlMinutes: number;
};

export type ActiveGrant = {
    id: string;
    level: string;
    grantedAt: string;
    expiresAt: string;
    runId?: string;
    sessionId?: string;
};

export type EscalationStatus = {
    hasPending: boolean;
    pending?: EscalationRequest;
    hasActive: boolean;
    active?: ActiveGrant;
    baseLevel: string;
    activeLevel: string;
};

export type EscalationState = {
    popupVisible: boolean;
    request: EscalationRequest | null;
    activeGrant: ActiveGrant | null;
    loading: boolean;
    error: string | null;
};

// ---------- Parsing ----------

export function isEscalationEvent(payload: unknown): boolean {
    if (typeof payload !== "object" || payload === null) return false;
    const p = payload as Record<string, unknown>;
    const id = typeof p.id === "string" ? p.id : "";
    return id.startsWith("esc_");
}

export function parseEscalationRequested(payload: unknown): EscalationRequest | null {
    if (!isEscalationEvent(payload)) return null;
    const p = payload as Record<string, unknown>;
    return {
        id: p.id as string,
        requestedLevel: typeof p.requestedLevel === "string" ? p.requestedLevel : "full",
        reason: typeof p.reason === "string" ? p.reason : "",
        runId: typeof p.runId === "string" ? p.runId : undefined,
        sessionId: typeof p.sessionId === "string" ? p.sessionId : undefined,
        requestedAt: typeof p.requestedAt === "number" ? p.requestedAt : Date.now(),
        ttlMinutes: typeof p.ttlMinutes === "number" ? p.ttlMinutes : 30,
    };
}

export function parseEscalationResolved(payload: unknown): { id: string; approved: boolean; level: string; reason?: string } | null {
    if (typeof payload !== "object" || payload === null) return null;
    const p = payload as Record<string, unknown>;
    const id = typeof p.id === "string" ? p.id : "";
    if (!id.startsWith("esc_")) return null;
    return {
        id,
        approved: p.approved === true,
        level: typeof p.level === "string" ? p.level : "deny",
        reason: typeof p.reason === "string" ? p.reason : undefined,
    };
}

// ---------- State management ----------

export function createEscalationState(): EscalationState {
    return {
        popupVisible: false,
        request: null,
        activeGrant: null,
        loading: false,
        error: null,
    };
}

export function handleEscalationRequested(state: EscalationState, payload: unknown): EscalationState {
    const req = parseEscalationRequested(payload);
    if (!req) return state;
    return {
        ...state,
        popupVisible: true,
        request: req,
        error: null,
    };
}

export function handleEscalationResolved(state: EscalationState, payload: unknown): EscalationState {
    const resolved = parseEscalationResolved(payload);
    if (!resolved) return state;
    return {
        ...state,
        popupVisible: false,
        request: null,
        activeGrant: resolved.approved
            ? { id: resolved.id, level: resolved.level, grantedAt: new Date().toISOString(), expiresAt: "", runId: undefined, sessionId: undefined }
            : null,
        error: null,
    };
}

// ---------- API calls ----------

export async function resolveEscalation(
    client: GatewayBrowserClient,
    approve: boolean,
    ttlMinutes: number,
): Promise<void> {
    await client.request("security.escalation.resolve", { approve, ttlMinutes });
}

export async function revokeEscalation(client: GatewayBrowserClient): Promise<void> {
    await client.request("security.escalation.revoke", {});
}

export async function loadEscalationStatus(client: GatewayBrowserClient): Promise<EscalationStatus> {
    return client.request<EscalationStatus>("security.escalation.status", {});
}
