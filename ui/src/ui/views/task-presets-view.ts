import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import { icons } from "../icons.ts";

// ---------- P5: 任务预设权限视图 ----------

export interface TaskPreset {
    id: string;
    name: string;
    pattern: string;
    level: string;
    autoApprove: boolean;
    maxTTL: number;
    description: string;
}

export interface TaskPresetMatchResult {
    matched: boolean;
    matchedBy: string;
    preset: TaskPreset | null;
}

export interface TaskPresetsViewProps {
    loading: boolean;
    error: string | null;
    presets: TaskPreset[];
    // 添加表单
    addFormOpen: boolean;
    addName: string;
    addPattern: string;
    addLevel: string;
    addAutoApprove: boolean;
    addMaxTTL: number;
    addDescription: string;
    // 测试
    testTaskName: string;
    testResult: TaskPresetMatchResult | null;
    testLoading: boolean;
    // 回调
    onRefresh: () => void;
    onAddFormToggle: () => void;
    onAddNameChange: (v: string) => void;
    onAddPatternChange: (v: string) => void;
    onAddLevelChange: (v: string) => void;
    onAddAutoApproveChange: (v: boolean) => void;
    onAddMaxTTLChange: (v: number) => void;
    onAddDescriptionChange: (v: string) => void;
    onAddSubmit: () => void;
    onRemove: (id: string) => void;
    onTestTaskNameChange: (v: string) => void;
    onTestSubmit: () => void;
}

const LEVEL_COLORS: Record<string, string> = {
    sandbox: "var(--color-ok, #22c55e)",
    allowlist: "var(--color-warn, #f59e0b)",
    full: "var(--color-danger, #ef4444)",
};

const LEVEL_ICONS: Record<string, string> = {
    sandbox: "🔒",
    allowlist: "📋",
    full: "⚡",
};

// ---------- 预设卡片 ----------

function renderPresetItem(preset: TaskPreset, props: TaskPresetsViewProps) {
    const levelColor = LEVEL_COLORS[preset.level] ?? "var(--text-secondary)";
    const levelIcon = LEVEL_ICONS[preset.level] ?? "";

    return html`
    <div class="tp-item">
      <div class="tp-item__header">
        <span class="tp-item__icon">${levelIcon}</span>
        <strong class="tp-item__name">${preset.name}</strong>
        <code class="tp-item__pattern">${preset.pattern}</code>
        <span class="tp-item__level" style="color: ${levelColor}">
          ${preset.level.toUpperCase()}
        </span>
        ${preset.autoApprove
            ? html`<span class="tp-item__badge tp-item__badge--auto">⚡ ${t("security.taskPresets.autoApprove")}</span>`
            : nothing}
      </div>
      ${preset.description
            ? html`<div class="tp-item__desc">${preset.description}</div>`
            : nothing}
      <div class="tp-item__footer">
        <span class="tp-item__ttl">${t("security.taskPresets.maxTtl")}: ${preset.maxTTL}min</span>
        <button class="btn btn--sm btn--danger"
          ?disabled=${props.loading}
          @click=${() => props.onRemove(preset.id)}
        >${t("security.taskPresets.remove")}</button>
      </div>
    </div>
  `;
}

// ---------- 添加预设表单 ----------

function renderAddForm(props: TaskPresetsViewProps) {
    if (!props.addFormOpen) return nothing;
    const isValid = props.addName.trim().length > 0 && props.addPattern.trim().length > 0;

    return html`
    <div class="tp-add-form">
      <div class="tp-add-form__row">
        <label>${t("security.taskPresets.name")}</label>
        <input type="text" class="tp-input" placeholder="Deploy Tasks"
          .value=${props.addName}
          @input=${(e: Event) => props.onAddNameChange((e.target as HTMLInputElement).value)}
        />
      </div>
      <div class="tp-add-form__row">
        <label>${t("security.taskPresets.pattern")}</label>
        <input type="text" class="tp-input" placeholder="deploy-*, ci-*-build"
          .value=${props.addPattern}
          @input=${(e: Event) => props.onAddPatternChange((e.target as HTMLInputElement).value)}
        />
        <span class="tp-field__hint">${t("security.taskPresets.patternHint")}</span>
      </div>
      <div class="tp-add-form__row">
        <label>${t("security.taskPresets.level")}</label>
        <div class="tp-level-group">
          ${(["sandbox", "allowlist", "full"] as const).map(level => html`
            <button
              class="btn btn--sm ${props.addLevel === level ? 'btn--active' : ''}"
              style="--btn-active-color: ${LEVEL_COLORS[level]}"
              @click=${() => props.onAddLevelChange(level)}
            >
              ${LEVEL_ICONS[level]} ${level.toUpperCase()}
            </button>
          `)}
        </div>
      </div>
      <div class="tp-add-form__row tp-add-form__row--inline">
        <label class="tp-toggle">
          <input type="checkbox"
            .checked=${props.addAutoApprove}
            @change=${(e: Event) => props.onAddAutoApproveChange((e.target as HTMLInputElement).checked)}
          />
          <span>${t("security.taskPresets.autoApprove")}</span>
        </label>
        <div class="tp-ttl-field">
          <label>${t("security.taskPresets.maxTtl")}</label>
          <input type="number" class="tp-input tp-input--sm" min="1" max="1440"
            .value=${String(props.addMaxTTL)}
            @input=${(e: Event) => props.onAddMaxTTLChange(parseInt((e.target as HTMLInputElement).value) || 30)}
          />
        </div>
      </div>
      <div class="tp-add-form__row">
        <label>${t("security.taskPresets.description")}</label>
        <input type="text" class="tp-input" placeholder="Optional description"
          .value=${props.addDescription}
          @input=${(e: Event) => props.onAddDescriptionChange((e.target as HTMLInputElement).value)}
        />
      </div>
      <div class="tp-add-form__actions">
        <button class="btn" @click=${props.onAddFormToggle}>${t("common.cancel")}</button>
        <button class="btn btn--primary"
          ?disabled=${!isValid || props.loading}
          @click=${props.onAddSubmit}
        >${t("security.taskPresets.addNew")}</button>
      </div>
    </div>
  `;
}

// ---------- 测试匹配 ----------

function renderTestSection(props: TaskPresetsViewProps) {
    return html`
    <div class="tp-test">
      <h3 class="tp-test__title">${t("security.taskPresets.testTitle")}</h3>
      <div class="tp-test__row">
        <input type="text" class="tp-input tp-test__input"
          placeholder=${t("security.taskPresets.testPlaceholder")}
          .value=${props.testTaskName}
          @input=${(e: Event) => props.onTestTaskNameChange((e.target as HTMLInputElement).value)}
          @keydown=${(e: KeyboardEvent) => { if (e.key === "Enter") props.onTestSubmit(); }}
        />
        <button class="btn btn--sm"
          ?disabled=${props.testLoading || !props.testTaskName.trim()}
          @click=${props.onTestSubmit}
        >${props.testLoading ? "…" : t("security.taskPresets.testBtn")}</button>
      </div>
      ${props.testResult ? renderTestResult(props.testResult) : nothing}
    </div>
  `;
}

function renderTestResult(result: TaskPresetMatchResult) {
    if (!result.matched) {
        return html`
      <div class="tp-test-result tp-test-result--none">
        <span>✅</span> ${t("security.taskPresets.noMatch")}
      </div>
    `;
    }
    const preset = result.preset;
    if (!preset) return nothing;
    const levelColor = LEVEL_COLORS[preset.level] ?? "";
    return html`
    <div class="tp-test-result tp-test-result--matched" style="border-left-color: ${levelColor}">
      <div class="tp-test-result__header">
        <span>${LEVEL_ICONS[preset.level] ?? ""}</span>
        <strong>${preset.name}</strong>
        <span style="color: ${levelColor}">${preset.level.toUpperCase()}</span>
      </div>
      <div class="tp-test-result__detail">
        ${t("security.rules.matchedBy")}: <code>${result.matchedBy}</code>
        ${preset.autoApprove
            ? html` · <span class="tp-item__badge tp-item__badge--auto">⚡ ${t("security.taskPresets.autoApprove")}</span>`
            : nothing}
      </div>
    </div>
  `;
}

// ---------- 主视图 ----------

export function renderTaskPresets(props: TaskPresetsViewProps) {
    return html`
    <div class="tp-page">
      <div class="tp-section">
        <div class="tp-section__header">
          <h2 class="tp-section__title">
            <span class="tp-section__title-icon">${icons.settings}</span>
            ${t("security.taskPresets.title")}
          </h2>
          <div class="tp-section__actions">
            <button class="btn btn--sm" ?disabled=${props.loading} @click=${props.onRefresh}>
              ${props.loading ? t("security.loading") : t("security.refresh")}
            </button>
            <button class="btn btn--sm btn--primary"
              ?disabled=${props.loading}
              @click=${props.onAddFormToggle}
            >${props.addFormOpen ? t("common.cancel") : t("security.taskPresets.addNew")}</button>
          </div>
        </div>
        <p class="tp-section__desc">${t("security.taskPresets.desc")}</p>

        ${props.error
            ? html`<div class="tp-error">${props.error}</div>`
            : nothing}

        ${renderAddForm(props)}

        <!-- 预设列表 -->
        ${props.presets.length > 0
            ? html`
              <div class="tp-list">
                ${props.presets.map(p => renderPresetItem(p, props))}
              </div>
            `
            : html`
              <div class="tp-empty">${t("security.taskPresets.noPresets")}</div>
            `}
      </div>

      <div class="tp-section">
        ${renderTestSection(props)}
      </div>
    </div>
  `;
}
