import { html, nothing } from "lit";
import { t } from "../i18n.ts";
import { icons } from "../icons.ts";

// ---------- P4: 远程审批配置视图 ----------

export interface RemoteApprovalViewProps {
    loading: boolean;
    error: string | null;
    enabled: boolean;
    callbackUrl: string;
    enabledProviders: string[];
    // 飞书
    feishuEnabled: boolean;
    feishuAppId: string;
    feishuAppSecret: string;
    feishuChatId: string;
    // 钉钉
    dingtalkEnabled: boolean;
    dingtalkWebhookUrl: string;
    dingtalkWebhookSecret: string;
    // 企业微信
    wecomEnabled: boolean;
    wecomCorpId: string;
    wecomAgentId: string;
    wecomSecret: string;
    wecomToUser: string;
    wecomToParty: string;
    // 测试状态
    testLoading: boolean;
    testResult: string | null;
    testError: string | null;
    // 保存状态
    saving: boolean;
    saved: boolean;
    // 回调
    onEnabledChange: (v: boolean) => void;
    onCallbackUrlChange: (v: string) => void;
    onFeishuEnabledChange: (v: boolean) => void;
    onFeishuAppIdChange: (v: string) => void;
    onFeishuAppSecretChange: (v: string) => void;
    onFeishuChatIdChange: (v: string) => void;
    onDingtalkEnabledChange: (v: boolean) => void;
    onDingtalkWebhookUrlChange: (v: string) => void;
    onDingtalkWebhookSecretChange: (v: string) => void;
    onWecomEnabledChange: (v: boolean) => void;
    onWecomCorpIdChange: (v: string) => void;
    onWecomAgentIdChange: (v: string) => void;
    onWecomSecretChange: (v: string) => void;
    onWecomToUserChange: (v: string) => void;
    onWecomToPartyChange: (v: string) => void;
    onTest: (platform: string) => void;
    onSave: () => void;
    onRefresh: () => void;
}

// ---------- 平台配置卡片 ----------

function renderFeishuCard(props: RemoteApprovalViewProps) {
    return html`
    <div class="ra-platform-card ${props.feishuEnabled ? 'ra-platform-card--active' : ''}">
      <div class="ra-platform-card__header">
        <label class="ra-toggle">
          <input type="checkbox"
            .checked=${props.feishuEnabled}
            @change=${(e: Event) => props.onFeishuEnabledChange((e.target as HTMLInputElement).checked)}
          />
          <span class="ra-toggle__label">🪶 ${t("security.remoteApproval.feishu")}</span>
        </label>
        ${props.feishuEnabled ? html`
          <button class="btn btn--sm"
            ?disabled=${props.testLoading}
            @click=${() => props.onTest("feishu")}
          >${props.testLoading ? t("security.remoteApproval.testing") : t("security.remoteApproval.testConnection")}</button>
        ` : nothing}
      </div>
      ${props.feishuEnabled ? html`
        <div class="ra-platform-card__body">
          <div class="ra-field">
            <label>${t("security.remoteApproval.appId")}</label>
            <input type="text" class="ra-input" .value=${props.feishuAppId}
              @input=${(e: Event) => props.onFeishuAppIdChange((e.target as HTMLInputElement).value)} />
          </div>
          <div class="ra-field">
            <label>${t("security.remoteApproval.appSecret")}</label>
            <input type="password" class="ra-input" .value=${props.feishuAppSecret}
              @input=${(e: Event) => props.onFeishuAppSecretChange((e.target as HTMLInputElement).value)} />
          </div>
          <div class="ra-field">
            <label>${t("security.remoteApproval.chatId")}</label>
            <input type="text" class="ra-input" .value=${props.feishuChatId}
              @input=${(e: Event) => props.onFeishuChatIdChange((e.target as HTMLInputElement).value)} />
          </div>
        </div>
      ` : nothing}
    </div>
  `;
}

function renderDingtalkCard(props: RemoteApprovalViewProps) {
    return html`
    <div class="ra-platform-card ${props.dingtalkEnabled ? 'ra-platform-card--active' : ''}">
      <div class="ra-platform-card__header">
        <label class="ra-toggle">
          <input type="checkbox"
            .checked=${props.dingtalkEnabled}
            @change=${(e: Event) => props.onDingtalkEnabledChange((e.target as HTMLInputElement).checked)}
          />
          <span class="ra-toggle__label">📌 ${t("security.remoteApproval.dingtalk")}</span>
        </label>
        ${props.dingtalkEnabled ? html`
          <button class="btn btn--sm"
            ?disabled=${props.testLoading}
            @click=${() => props.onTest("dingtalk")}
          >${props.testLoading ? t("security.remoteApproval.testing") : t("security.remoteApproval.testConnection")}</button>
        ` : nothing}
      </div>
      ${props.dingtalkEnabled ? html`
        <div class="ra-platform-card__body">
          <div class="ra-field">
            <label>${t("security.remoteApproval.webhookUrl")}</label>
            <input type="text" class="ra-input" .value=${props.dingtalkWebhookUrl}
              @input=${(e: Event) => props.onDingtalkWebhookUrlChange((e.target as HTMLInputElement).value)} />
          </div>
          <div class="ra-field">
            <label>${t("security.remoteApproval.webhookSecret")}</label>
            <input type="password" class="ra-input" .value=${props.dingtalkWebhookSecret}
              @input=${(e: Event) => props.onDingtalkWebhookSecretChange((e.target as HTMLInputElement).value)} />
          </div>
        </div>
      ` : nothing}
    </div>
  `;
}

function renderWecomCard(props: RemoteApprovalViewProps) {
    return html`
    <div class="ra-platform-card ${props.wecomEnabled ? 'ra-platform-card--active' : ''}">
      <div class="ra-platform-card__header">
        <label class="ra-toggle">
          <input type="checkbox"
            .checked=${props.wecomEnabled}
            @change=${(e: Event) => props.onWecomEnabledChange((e.target as HTMLInputElement).checked)}
          />
          <span class="ra-toggle__label">💬 ${t("security.remoteApproval.wecom")}</span>
        </label>
        ${props.wecomEnabled ? html`
          <button class="btn btn--sm"
            ?disabled=${props.testLoading}
            @click=${() => props.onTest("wecom")}
          >${props.testLoading ? t("security.remoteApproval.testing") : t("security.remoteApproval.testConnection")}</button>
        ` : nothing}
      </div>
      ${props.wecomEnabled ? html`
        <div class="ra-platform-card__body">
          <div class="ra-field">
            <label>${t("security.remoteApproval.corpId")}</label>
            <input type="text" class="ra-input" .value=${props.wecomCorpId}
              @input=${(e: Event) => props.onWecomCorpIdChange((e.target as HTMLInputElement).value)} />
          </div>
          <div class="ra-field">
            <label>${t("security.remoteApproval.agentId")}</label>
            <input type="text" class="ra-input" .value=${props.wecomAgentId}
              @input=${(e: Event) => props.onWecomAgentIdChange((e.target as HTMLInputElement).value)} />
          </div>
          <div class="ra-field">
            <label>${t("security.remoteApproval.secret")}</label>
            <input type="password" class="ra-input" .value=${props.wecomSecret}
              @input=${(e: Event) => props.onWecomSecretChange((e.target as HTMLInputElement).value)} />
          </div>
          <div class="ra-field">
            <label>${t("security.remoteApproval.toUser")}</label>
            <input type="text" class="ra-input" placeholder="@all"
              .value=${props.wecomToUser}
              @input=${(e: Event) => props.onWecomToUserChange((e.target as HTMLInputElement).value)} />
          </div>
          <div class="ra-field">
            <label>${t("security.remoteApproval.toParty")}</label>
            <input type="text" class="ra-input"
              .value=${props.wecomToParty}
              @input=${(e: Event) => props.onWecomToPartyChange((e.target as HTMLInputElement).value)} />
          </div>
        </div>
      ` : nothing}
    </div>
  `;
}

// ---------- 主视图 ----------

export function renderRemoteApproval(props: RemoteApprovalViewProps) {
    return html`
    <div class="ra-page">
      <div class="ra-section">
        <div class="ra-section__header">
          <h2 class="ra-section__title">
            <span class="ra-section__title-icon">${icons.settings}</span>
            ${t("security.remoteApproval.title")}
          </h2>
          <div class="ra-section__actions">
            <button class="btn btn--sm" ?disabled=${props.loading} @click=${props.onRefresh}>
              ${props.loading ? t("security.loading") : t("security.refresh")}
            </button>
          </div>
        </div>
        <p class="ra-section__desc">${t("security.remoteApproval.desc")}</p>

        ${props.error
            ? html`<div class="ra-error">${props.error}</div>`
            : nothing}

        <!-- 全局开关 -->
        <div class="ra-global-toggle">
          <label class="ra-toggle">
            <input type="checkbox"
              .checked=${props.enabled}
              @change=${(e: Event) => props.onEnabledChange((e.target as HTMLInputElement).checked)}
            />
            <span class="ra-toggle__label">${t("security.remoteApproval.enabled")}</span>
          </label>
          ${props.enabledProviders.length > 0
            ? html`<span class="ra-badge">${t("security.remoteApproval.enabledProviders")}: ${props.enabledProviders.join(", ")}</span>`
            : html`<span class="ra-badge ra-badge--muted">${t("security.remoteApproval.noProviders")}</span>`}
        </div>

        ${props.enabled ? html`
          <!-- 回调 URL -->
          <div class="ra-field ra-callback-field">
            <label>${t("security.remoteApproval.callbackUrl")}</label>
            <input type="text" class="ra-input"
              placeholder="https://your-domain.com/api/gateway/callback"
              .value=${props.callbackUrl}
              @input=${(e: Event) => props.onCallbackUrlChange((e.target as HTMLInputElement).value)}
            />
            <span class="ra-field__hint">${t("security.remoteApproval.callbackUrlHint")}</span>
          </div>

          <!-- 平台卡片 -->
          <div class="ra-platforms">
            ${renderFeishuCard(props)}
            ${renderDingtalkCard(props)}
            ${renderWecomCard(props)}
          </div>

          <!-- 测试结果 -->
          ${props.testResult
                ? html`<div class="ra-test-result ra-test-result--ok">${props.testResult}</div>`
                : nothing}
          ${props.testError
                ? html`<div class="ra-test-result ra-test-result--error">${t("security.remoteApproval.testFailed")}: ${props.testError}</div>`
                : nothing}

          <!-- 保存按钮 -->
          <div class="ra-save-bar">
            <button class="btn btn--primary"
              ?disabled=${props.saving}
              @click=${props.onSave}
            >${props.saving ? t("security.remoteApproval.saving") : t("security.remoteApproval.save")}</button>
            ${props.saved ? html`<span class="ra-saved-indicator">✓ ${t("security.remoteApproval.saved")}</span>` : nothing}
          </div>
        ` : nothing}
      </div>
    </div>
  `;
}
