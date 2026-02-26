/**
 * Tool policy utilities — inlined from src/agents/tool-policy.ts
 * to decouple the frontend from the TS backend source tree.
 *
 * Only the UI-used subset is included:
 *   - normalizeToolName()
 *   - expandToolGroups()
 *   - resolveToolProfilePolicy()
 */

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const TOOL_NAME_ALIASES: Record<string, string> = {
  bash: "exec",
  "apply-patch": "apply_patch",
};

const TOOL_GROUPS: Record<string, string[]> = {
  "group:memory": ["memory_search", "memory_get"],
  "group:web": ["web_search", "web_fetch"],
  "group:fs": ["read", "write", "edit", "apply_patch"],
  "group:runtime": ["exec", "process"],
  "group:sessions": [
    "sessions_list",
    "sessions_history",
    "sessions_send",
    "sessions_spawn",
    "session_status",
  ],
  "group:ui": ["browser", "canvas"],
  "group:automation": ["cron", "gateway"],
  "group:messaging": ["message"],
  "group:nodes": ["nodes"],
  "group:openacosmi": [
    "browser",
    "canvas",
    "nodes",
    "cron",
    "message",
    "gateway",
    "agents_list",
    "sessions_list",
    "sessions_history",
    "sessions_send",
    "sessions_spawn",
    "session_status",
    "memory_search",
    "memory_get",
    "web_search",
    "web_fetch",
    "image",
  ],
};

type ToolProfileId = "minimal" | "coding" | "messaging" | "full";

type ToolProfilePolicy = {
  allow?: string[];
  deny?: string[];
};

const TOOL_PROFILES: Record<ToolProfileId, ToolProfilePolicy> = {
  minimal: {
    allow: ["session_status"],
  },
  coding: {
    allow: ["group:fs", "group:runtime", "group:sessions", "group:memory", "image"],
  },
  messaging: {
    allow: [
      "group:messaging",
      "sessions_list",
      "sessions_history",
      "sessions_send",
      "session_status",
    ],
  },
  full: {},
};

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

export function normalizeToolName(name: string): string {
  const normalized = name.trim().toLowerCase();
  return TOOL_NAME_ALIASES[normalized] ?? normalized;
}

function normalizeToolList(list?: string[]): string[] {
  if (!list) {
    return [];
  }
  return list.map(normalizeToolName).filter(Boolean);
}

export function expandToolGroups(list?: string[]): string[] {
  const normalized = normalizeToolList(list);
  const expanded: string[] = [];
  for (const value of normalized) {
    const group = TOOL_GROUPS[value];
    if (group) {
      expanded.push(...group);
      continue;
    }
    expanded.push(value);
  }
  return Array.from(new Set(expanded));
}

export function resolveToolProfilePolicy(profile?: string): ToolProfilePolicy | undefined {
  if (!profile) {
    return undefined;
  }
  const resolved = TOOL_PROFILES[profile as ToolProfileId];
  if (!resolved) {
    return undefined;
  }
  if (!resolved.allow && !resolved.deny) {
    return undefined;
  }
  return {
    allow: resolved.allow ? [...resolved.allow] : undefined,
    deny: resolved.deny ? [...resolved.deny] : undefined,
  };
}
