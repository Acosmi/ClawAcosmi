import { html, nothing } from "lit";
import type { CommandRule, RuleTestResult } from "../controllers/rules.ts";
import { t } from "../i18n.ts";
import { icons } from "../icons.ts";

// ---------- 规则管理视图 ----------

export interface RulesViewProps {
    loading: boolean;
    error: string | null;
    rules: CommandRule[];
    presetCount: number;
    userCount: number;
    // 添加表单
    addFormOpen: boolean;
    addPattern: string;
    addAction: "allow" | "ask" | "deny";
    addDescription: string;
    // 测试
    testCommand: string;
    testResult: RuleTestResult | null;
    testLoading: boolean;
    // 回调
    onRefresh: () => void;
    onAddFormToggle: () => void;
    onAddPatternChange: (v: string) => void;
    onAddActionChange: (v: "allow" | "ask" | "deny") => void;
    onAddDescriptionChange: (v: string) => void;
    onAddSubmit: () => void;
    onRemove: (id: string) => void;
    onTestCommandChange: (v: string) => void;
    onTestSubmit: () => void;
}

const ACTION_COLORS: Record<string, string> = {
    allow: "var(--color-ok, #22c55e)",
    ask: "var(--color-warn, #f59e0b)",
    deny: "var(--color-danger, #ef4444)",
};

const ACTION_ICONS: Record<string, string> = {
    allow: "✅",
    ask: "❓",
    deny: "🚫",
};

// ---------- 规则卡片 ----------

function renderRuleItem(rule: CommandRule, props: RulesViewProps) {
    const actionColor = ACTION_COLORS[rule.action] ?? "var(--text-secondary)";
    const actionIcon = ACTION_ICONS[rule.action] ?? "";

    return html`
    <div class="rule-item ${rule.isPreset ? "rule-item--preset" : "rule-item--user"}">
      <div class="rule-item__header">
        <span class="rule-item__icon">${actionIcon}</span>
        <code class="rule-item__pattern">${rule.pattern}</code>
        <span class="rule-item__action" style="color: ${actionColor}">
          ${rule.action.toUpperCase()}
        </span>
        ${rule.isPreset
            ? html`<span class="rule-item__badge rule-item__badge--preset">${t("security.rules.preset")}</span>`
            : html`<span class="rule-item__badge rule-item__badge--user">${t("security.rules.custom")}</span>`
        }
      </div>
      ${rule.description
            ? html`<div class="rule-item__desc">${rule.description}</div>`
            : nothing}
      <div class="rule-item__footer">
        ${!rule.isPreset
            ? html`<button
                class="btn btn--sm btn--danger"
                ?disabled=${props.loading}
                @click=${() => props.onRemove(rule.id)}
              >${t("security.rules.remove")}</button>`
            : html`<span class="rule-item__locked">🔒 ${t("security.rules.locked")}</span>`
        }
      </div>
    </div>
  `;
}

// ---------- 添加规则表单 ----------

function renderAddForm(props: RulesViewProps) {
    if (!props.addFormOpen) return nothing;
    const isValid = props.addPattern.trim().length > 0;

    return html`
    <div class="rules-add-form">
      <div class="rules-add-form__row">
        <label>${t("security.rules.pattern")}</label>
        <input
          type="text"
          class="rules-input"
          placeholder="rm -rf *, sudo *, *curl*|*sh*"
          .value=${props.addPattern}
          @input=${(e: Event) => props.onAddPatternChange((e.target as HTMLInputElement).value)}
        />
      </div>
      <div class="rules-add-form__row">
        <label>${t("security.rules.action")}</label>
        <div class="rules-action-group">
          ${(["deny", "ask", "allow"] as const).map(action => html`
            <button
              class="btn btn--sm ${props.addAction === action ? "btn--active" : ""}"
              style="--btn-active-color: ${ACTION_COLORS[action]}"
              @click=${() => props.onAddActionChange(action)}
            >
              ${ACTION_ICONS[action]} ${action.toUpperCase()}
            </button>
          `)}
        </div>
      </div>
      <div class="rules-add-form__row">
        <label>${t("security.rules.description")}</label>
        <input
          type="text"
          class="rules-input"
          placeholder=${t("security.rules.descPlaceholder")}
          .value=${props.addDescription}
          @input=${(e: Event) => props.onAddDescriptionChange((e.target as HTMLInputElement).value)}
        />
      </div>
      <div class="rules-add-form__actions">
        <button class="btn" @click=${props.onAddFormToggle}>${t("common.cancel")}</button>
        <button
          class="btn btn--primary"
          ?disabled=${!isValid || props.loading}
          @click=${props.onAddSubmit}
        >${t("security.rules.addRule")}</button>
      </div>
    </div>
  `;
}

// ---------- 命令测试 ----------

function renderTestSection(props: RulesViewProps) {
    return html`
    <div class="rules-test">
      <h3 class="rules-test__title">${t("security.rules.testTitle")}</h3>
      <div class="rules-test__row">
        <input
          type="text"
          class="rules-input rules-test__input"
          placeholder=${t("security.rules.testPlaceholder")}
          .value=${props.testCommand}
          @input=${(e: Event) => props.onTestCommandChange((e.target as HTMLInputElement).value)}
          @keydown=${(e: KeyboardEvent) => {
            if (e.key === "Enter") props.onTestSubmit();
        }}
        />
        <button
          class="btn btn--sm"
          ?disabled=${props.testLoading || !props.testCommand.trim()}
          @click=${props.onTestSubmit}
        >${props.testLoading ? "…" : t("security.rules.testBtn")}</button>
      </div>
      ${props.testResult ? renderTestResult(props.testResult) : nothing}
    </div>
  `;
}

function renderTestResult(result: RuleTestResult) {
    if (!result.matched) {
        return html`
      <div class="rules-test-result rules-test-result--none">
        <span>✅</span> ${t("security.rules.testNoMatch")}
      </div>
    `;
    }
    const actionColor = ACTION_COLORS[result.action ?? "deny"] ?? "";
    const actionIcon = ACTION_ICONS[result.action ?? "deny"] ?? "";
    return html`
    <div class="rules-test-result" style="border-left-color: ${actionColor}">
      <div class="rules-test-result__header">
        <span>${actionIcon}</span>
        <strong style="color: ${actionColor}">${result.action?.toUpperCase()}</strong>
      </div>
      <div class="rules-test-result__detail">${result.reason}</div>
      ${result.matchedRule
            ? html`<div class="rules-test-result__rule">
            ${t("security.rules.matchedBy")}:
            <code>${result.matchedRule.pattern}</code>
            ${result.matchedRule.isPreset
                    ? html`<span class="rule-item__badge rule-item__badge--preset">${t("security.rules.preset")}</span>`
                    : html`<span class="rule-item__badge rule-item__badge--user">${t("security.rules.custom")}</span>`
                }
          </div>`
            : nothing}
    </div>
  `;
}

// ---------- 主视图 ----------

export function renderRules(props: RulesViewProps) {
    const presetRules = props.rules.filter(r => r.isPreset);
    const userRules = props.rules.filter(r => !r.isPreset);

    return html`
    <div class="rules-page">
      <div class="rules-section">
        <div class="rules-section__header">
          <h2 class="rules-section__title">
            <span class="rules-section__title-icon">${icons.settings}</span>
            ${t("security.rules.title")}
          </h2>
          <div class="rules-section__actions">
            <button
              class="btn btn--sm"
              ?disabled=${props.loading}
              @click=${props.onRefresh}
            >${props.loading ? t("security.loading") : t("security.refresh")}</button>
            <button
              class="btn btn--sm btn--primary"
              ?disabled=${props.loading}
              @click=${props.onAddFormToggle}
            >${props.addFormOpen ? t("common.cancel") : t("security.rules.addNew")}</button>
          </div>
        </div>
        <p class="rules-section__desc">${t("security.rules.desc")}</p>

        ${props.error
            ? html`<div class="rules-error">${props.error}</div>`
            : nothing}

        ${renderAddForm(props)}

        <!-- 用户自定义规则 -->
        ${userRules.length > 0
            ? html`
            <h3 class="rules-group-title">${t("security.rules.userRules")} (${userRules.length})</h3>
            <div class="rules-list">
              ${userRules.map(rule => renderRuleItem(rule, props))}
            </div>
          `
            : nothing}

        <!-- 预设规则 -->
        <h3 class="rules-group-title">${t("security.rules.presetRules")} (${presetRules.length})</h3>
        <div class="rules-list">
          ${presetRules.map(rule => renderRuleItem(rule, props))}
        </div>
      </div>

      <div class="rules-section">
        ${renderTestSection(props)}
      </div>
    </div>
  `;
}
