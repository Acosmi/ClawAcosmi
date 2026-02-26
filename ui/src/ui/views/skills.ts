import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import type { SkillMessageMap } from "../controllers/skills.ts";
import type { SkillStatusEntry, SkillStatusReport } from "../types.ts";
import { clampText } from "../format.ts";

type SkillGroup = {
  id: string;
  label: string;
  skills: SkillStatusEntry[];
};

function getSkillSourceGroups(): Array<{ id: string; label: string; sources: string[] }> {
  return [
    { id: "workspace", label: t("agents.skillGroup.workspace"), sources: ["openacosmi-workspace"] },
    { id: "built-in", label: t("agents.skillGroup.builtIn"), sources: ["openacosmi-bundled"] },
    { id: "installed", label: t("agents.skillGroup.installed"), sources: ["openacosmi-managed"] },
    { id: "extra", label: t("agents.skillGroup.extra"), sources: ["openacosmi-extra"] },
  ];
}

function groupSkills(skills: SkillStatusEntry[]): SkillGroup[] {
  const SKILL_SOURCE_GROUPS = getSkillSourceGroups();
  const groups = new Map<string, SkillGroup>();
  for (const def of SKILL_SOURCE_GROUPS) {
    groups.set(def.id, { id: def.id, label: def.label, skills: [] });
  }
  const builtInGroup = SKILL_SOURCE_GROUPS.find((group) => group.id === "built-in");
  const other: SkillGroup = { id: "other", label: t("agents.skillGroup.other"), skills: [] };
  for (const skill of skills) {
    const match = skill.bundled
      ? builtInGroup
      : SKILL_SOURCE_GROUPS.find((group) => group.sources.includes(skill.source));
    if (match) {
      groups.get(match.id)?.skills.push(skill);
    } else {
      other.skills.push(skill);
    }
  }
  const ordered = SKILL_SOURCE_GROUPS.map((group) => groups.get(group.id)).filter(
    (group): group is SkillGroup => Boolean(group && group.skills.length > 0),
  );
  if (other.skills.length > 0) {
    ordered.push(other);
  }
  return ordered;
}

export type SkillsProps = {
  loading: boolean;
  report: SkillStatusReport | null;
  error: string | null;
  filter: string;
  edits: Record<string, string>;
  busyKey: string | null;
  messages: SkillMessageMap;
  distributeLoading: boolean;
  distributeResult: string | null;
  onFilterChange: (next: string) => void;
  onRefresh: () => void;
  onToggle: (skillKey: string, enabled: boolean) => void;
  onEdit: (skillKey: string, value: string) => void;
  onSaveKey: (skillKey: string) => void;
  onInstall: (skillKey: string, name: string, installId: string) => void;
  onDistribute: () => void;
};

export function renderSkills(props: SkillsProps) {
  const skills = props.report?.skills ?? [];
  const filter = props.filter.trim().toLowerCase();
  const filtered = filter
    ? skills.filter((skill) =>
      [skill.name, skill.description, skill.source].join(" ").toLowerCase().includes(filter),
    )
    : skills;
  const groups = groupSkills(filtered);

  return html`
    <section class="card">
      <div class="row" style="justify-content: space-between;">
        <div>
          <div class="card-title">${t("skills.title")}</div>
          <div class="card-sub">${t("skills.sub")}</div>
        </div>
        <div class="row" style="gap: 8px;">
          <button class="btn" ?disabled=${props.loading} @click=${props.onRefresh}>
            ${props.loading ? t("common.loading") : t("common.refresh")}
          </button>
          <button
            class="btn primary"
            ?disabled=${props.distributeLoading || props.loading}
            @click=${props.onDistribute}
          >
            ${props.distributeLoading ? "分级中..." : "一键 VFS 分级"}
          </button>
        </div>
      </div>

      <div class="filters" style="margin-top: 14px;">
        <label class="field" style="flex: 1;">
          <span>${t("agents.skills.filter")}</span>
          <input
            .value=${props.filter}
            @input=${(e: Event) => props.onFilterChange((e.target as HTMLInputElement).value)}
            placeholder=${t("agents.skills.searchPlaceholder")}
          />
        </label>
        <div class="muted">${t("agents.skills.shown", { count: filtered.length })}</div>
      </div>

      ${props.error
      ? html`<div class="callout danger" style="margin-top: 12px;">${props.error}</div>`
      : nothing
    }

      ${props.distributeResult
      ? html`<div class="callout" style="margin-top: 12px; background: var(--success-bg, #e6f9f0); color: var(--success-color, #0a7f5a);">${props.distributeResult}</div>`
      : nothing
    }

      ${filtered.length === 0
      ? html`
              <div class="muted" style="margin-top: 16px">${t("agents.skills.noSkills")}</div>
            `
      : html`
            <div class="agent-skills-groups" style="margin-top: 16px;">
              ${groups.map((group) => {
        const collapsedByDefault = group.id === "workspace" || group.id === "built-in";
        return html`
                  <details class="agent-skills-group" ?open=${!collapsedByDefault}>
                    <summary class="agent-skills-header">
                      <span>${group.label}</span>
                      <span class="muted">${group.skills.length}</span>
                    </summary>
                    <div class="list skills-grid">
                      ${group.skills.map((skill) => renderSkill(skill, props))}
                    </div>
                  </details>
                `;
      })}
            </div>
          `
    }
    </section>
  `;
}

function renderSkill(skill: SkillStatusEntry, props: SkillsProps) {
  const busy = props.busyKey === skill.skillKey;
  const apiKey = props.edits[skill.skillKey] ?? "";
  const message = props.messages[skill.skillKey] ?? null;
  const canInstall = skill.install.length > 0 && skill.missing.bins.length > 0;
  const showBundledBadge = Boolean(skill.bundled && skill.source !== "openacosmi-bundled");
  const missing = [
    ...skill.missing.bins.map((b) => `bin:${b}`),
    ...skill.missing.env.map((e) => `env:${e}`),
    ...skill.missing.config.map((c) => `config:${c}`),
    ...skill.missing.os.map((o) => `os:${o}`),
  ];
  const reasons: string[] = [];
  if (skill.disabled) {
    reasons.push(t("agents.skill.disabled"));
  }
  if (skill.blockedByAllowlist) {
    reasons.push(t("agents.skill.blockedByAllowlist"));
  }
  return html`
    <div class="list-item">
      <div class="list-main">
        <div class="list-title">
          ${skill.emoji ? `${skill.emoji} ` : ""}${skill.name}
        </div>
        <div class="list-sub">${clampText(skill.description, 140)}</div>
        <div class="chip-row" style="margin-top: 6px;">
          <span class="chip">${skill.source}</span>
          ${showBundledBadge
      ? html`
                  <span class="chip">${t("skills.bundled")}</span>
                `
      : nothing
    }
          <span class="chip ${skill.eligible ? "chip-ok" : "chip-warn"}">
            ${skill.eligible ? t("agents.skill.eligible") : t("agents.skill.blocked")}
          </span>
          ${skill.disabled
      ? html`
                  <span class="chip chip-warn">${t("agents.skill.disabled")}</span>
                `
      : nothing
    }
          ${skill.distributed
      ? html`<span class="chip chip-ok">已VFS分级</span>`
      : html`<span class="chip chip-warn">未分级</span>`
    }
        </div>
        ${missing.length > 0
      ? html`
              <div class="muted" style="margin-top: 6px;">
                ${t("agents.skill.missing", { list: missing.join(", ") })}
              </div>
            `
      : nothing
    }
        ${reasons.length > 0
      ? html`
              <div class="muted" style="margin-top: 6px;">
                ${t("agents.skill.reason", { list: reasons.join(", ") })}
              </div>
            `
      : nothing
    }
      </div>
      <div class="list-meta">
        <div class="row" style="justify-content: flex-end; flex-wrap: wrap;">
          <button
            class="btn"
            ?disabled=${busy}
            @click=${() => props.onToggle(skill.skillKey, skill.disabled)}
          >
            ${skill.disabled ? t("skills.enable") : t("skills.disable")}
          </button>
          ${canInstall
      ? html`<button
                class="btn"
                ?disabled=${busy}
                @click=${() => props.onInstall(skill.skillKey, skill.name, skill.install[0].id)}
              >
                ${busy ? t("skills.installing") : skill.install[0].label}
              </button>`
      : nothing
    }
        </div>
        ${message
      ? html`<div
              class="muted"
              style="margin-top: 8px; color: ${message.kind === "error"
          ? "var(--danger-color, #d14343)"
          : "var(--success-color, #0a7f5a)"
        };"
            >
              ${message.message}
            </div>`
      : nothing
    }
        ${skill.primaryEnv
      ? html`
              <div class="field" style="margin-top: 10px;">
                <span>${t("skills.apiKey")}</span>
                <input
                  type="password"
                  .value=${apiKey}
                  @input=${(e: Event) =>
          props.onEdit(skill.skillKey, (e.target as HTMLInputElement).value)}
                />
              </div>
              <button
                class="btn primary"
                style="margin-top: 8px;"
                ?disabled=${busy}
                @click=${() => props.onSaveKey(skill.skillKey)}
              >
                ${t("skills.saveKey")}
              </button>
            `
      : nothing
    }
      </div>
    </div>
  `;
}
