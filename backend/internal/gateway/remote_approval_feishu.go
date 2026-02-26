package gateway

// remote_approval_feishu.go — P4 飞书远程审批 Provider
// 通过飞书开放平台 API 发送互动卡片消息。
//
// 实现方式：直接 HTTP API 调用（标准库），不引入 larksuite/oapi-sdk-go
// 原因：减少外部依赖，飞书 API 结构稳定，HTTP 调用足够简单。
//
// API 参考：
//   - 获取 tenant_access_token: POST https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal/
//   - 发送消息: POST https://open.feishu.cn/open-apis/im/v1/messages

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	feishuTokenURL   = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal/"
	feishuMessageURL = "https://open.feishu.cn/open-apis/im/v1/messages"
)

// feishuProvider 飞书远程审批 Provider。
type feishuProvider struct {
	config *FeishuProviderConfig
	client *http.Client
}

// newFeishuProvider 创建飞书 Provider。
func newFeishuProvider(cfg *FeishuProviderConfig) *feishuProvider {
	return &feishuProvider{
		config: cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *feishuProvider) Name() string { return "feishu" }

func (p *feishuProvider) ValidateConfig() error {
	if p.config.AppID == "" {
		return fmt.Errorf("飞书 App ID 不能为空")
	}
	if p.config.AppSecret == "" {
		return fmt.Errorf("飞书 App Secret 不能为空")
	}
	// 接收方校验移至 SendApprovalRequest，因为 OriginatorUserID 是运行时动态填入的
	return nil
}

func (p *feishuProvider) SendApprovalRequest(ctx context.Context, req ApprovalCardRequest) error {
	if err := p.ValidateConfig(); err != nil {
		return err
	}

	// 1. 获取 tenant_access_token
	token, err := p.getTenantAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("获取飞书 access token 失败: %w", err)
	}

	// 2. 构建互动卡片
	card := p.buildApprovalCard(req)

	// 3. 群发：收集所有目标（去重）
	type target struct {
		idType string
		id     string
	}
	seen := make(map[string]bool)
	var targets []target
	addTarget := func(idType, id string) {
		if id == "" {
			return
		}
		key := idType + ":" + id
		if seen[key] {
			return
		}
		seen[key] = true
		targets = append(targets, target{idType, id})
	}

	// 静态配置的群聊 / 用户
	addTarget("chat_id", p.config.ChatID)
	addTarget("open_id", p.config.UserID)
	// 运行时动态获取的发起群聊（飞书消息事件的 chat_id）
	addTarget("chat_id", req.OriginatorChatID)
	// 运行时动态获取的发起用户（飞书消息事件的 open_id）
	addTarget("open_id", req.OriginatorUserID)

	if len(targets) == 0 {
		return fmt.Errorf("飞书消息接收方为空: 需要配置 chatId/userId 或由飞书消息事件自动填充")
	}

	// 4. 逐一发送，收集错误
	var firstErr error
	for _, t := range targets {
		if err := p.sendMessageTo(ctx, token, card, t.idType, t.id); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// getTenantAccessToken 获取飞书 tenant_access_token。
func (p *feishuProvider) getTenantAccessToken(ctx context.Context) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"app_id":     p.config.AppID,
		"app_secret": p.config.AppSecret,
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", feishuTokenURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("飞书 token API 错误: code=%d, msg=%s", result.Code, result.Msg)
	}
	return result.TenantAccessToken, nil
}

// buildApprovalCard 构建飞书互动卡片 JSON。
func (p *feishuProvider) buildApprovalCard(req ApprovalCardRequest) map[string]interface{} {
	levelLabel := map[string]string{
		"full":      "🔴 L2 — 完全权限 / Full Access",
		"allowlist": "🟡 L1 — 受限执行 / Allowlist",
		"sandbox":   "🟡 L1 — 沙盒执行 / Sandbox",
	}[req.RequestedLevel]
	if levelLabel == "" {
		levelLabel = req.RequestedLevel
	}

	card := map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"header": map[string]interface{}{
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": "🔐 权限提升审批 / Permission Escalation",
			},
			"template": "red",
		},
		"elements": []interface{}{
			map[string]interface{}{
				"tag": "div",
				"fields": []interface{}{
					map[string]interface{}{
						"is_short": true,
						"text": map[string]interface{}{
							"tag":     "lark_md",
							"content": fmt.Sprintf("**请求级别**\n%s", levelLabel),
						},
					},
					map[string]interface{}{
						"is_short": true,
						"text": map[string]interface{}{
							"tag":     "lark_md",
							"content": fmt.Sprintf("**授权时长**\n%d 分钟", req.TTLMinutes),
						},
					},
				},
			},
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": fmt.Sprintf("**原因**: %s", req.Reason),
				},
			},
			map[string]interface{}{
				"tag": "hr",
			},
			map[string]interface{}{
				"tag": "action",
				"actions": []interface{}{
					map[string]interface{}{
						"tag": "button",
						"text": map[string]interface{}{
							"tag":     "plain_text",
							"content": "✅ 批准 / Approve",
						},
						"type": "primary",
						"value": map[string]interface{}{
							"action": "approve",
							"id":     req.EscalationID,
							"ttl":    req.TTLMinutes,
						},
					},
					map[string]interface{}{
						"tag": "button",
						"text": map[string]interface{}{
							"tag":     "plain_text",
							"content": "❌ 拒绝 / Deny",
						},
						"type": "danger",
						"value": map[string]interface{}{
							"action": "deny",
							"id":     req.EscalationID,
						},
					},
				},
			},
			map[string]interface{}{
				"tag": "note",
				"elements": []interface{}{
					map[string]interface{}{
						"tag":     "plain_text",
						"content": fmt.Sprintf("ID: %s | %s", req.EscalationID, req.RequestedAt.Format(time.RFC3339)),
					},
				},
			},
		},
	}
	return card
}

// broadcastCard 群发飞书卡片到所有已配置的目标（群聊+用户）。
func (p *feishuProvider) broadcastCard(ctx context.Context, token string, card map[string]interface{}) error {
	type target struct {
		idType string
		id     string
	}
	seen := make(map[string]bool)
	var targets []target
	addTarget := func(idType, id string) {
		if id == "" {
			return
		}
		key := idType + ":" + id
		if seen[key] {
			return
		}
		seen[key] = true
		targets = append(targets, target{idType, id})
	}

	addTarget("chat_id", p.config.ChatID)
	addTarget("open_id", p.config.UserID)

	if len(targets) == 0 {
		return fmt.Errorf("飞书消息接收方为空: 需要配置 chatId 或 userId")
	}

	var firstErr error
	for _, t := range targets {
		if err := p.sendMessageTo(ctx, token, card, t.idType, t.id); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// sendMessageTo 发送飞书消息到指定接收方。
func (p *feishuProvider) sendMessageTo(ctx context.Context, token string, card map[string]interface{}, receiveIDType, receiveID string) error {
	if receiveID == "" {
		return fmt.Errorf("飞书消息接收方为空: 需要配置 chatId、userId 或由系统自动填充 originatorUserId")
	}

	cardJSON, err := json.Marshal(card)
	if err != nil {
		return err
	}

	msgBody := map[string]interface{}{
		"receive_id": receiveID,
		"msg_type":   "interactive",
		"content":    string(cardJSON),
	}

	body, err := json.Marshal(msgBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s?receive_id_type=%s", feishuMessageURL, receiveIDType)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json; charset=utf-8")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("飞书消息 API 响应解析失败: %s", string(respBody))
	}
	if result.Code != 0 {
		return fmt.Errorf("飞书消息发送失败: code=%d, msg=%s", result.Code, result.Msg)
	}
	return nil
}

// ---------- Phase 8: 审批结果通知 ----------

// SendResultNotification 实现 ResultNotifier 接口，推送审批结果卡片。
func (p *feishuProvider) SendResultNotification(ctx context.Context, result ApprovalResultNotification) error {
	if err := p.ValidateConfig(); err != nil {
		return err
	}

	token, err := p.getTenantAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("获取飞书 access token 失败: %w", err)
	}

	card := p.buildResultCard(result)
	return p.broadcastCard(ctx, token, card)
}

// buildResultCard 构建审批结果互动卡片。
func (p *feishuProvider) buildResultCard(result ApprovalResultNotification) map[string]interface{} {
	var headerTitle, headerTemplate, bodyText string

	if result.Approved {
		headerTitle = "✅ 权限已生效 / Permission Granted"
		headerTemplate = "green"
		bodyText = fmt.Sprintf("权限提升请求已批准。\n\n"+
			"**授权级别**: %s\n"+
			"**有效时长**: %d 分钟\n\n"+
			"权限到期后将自动降级。",
			result.RequestedLevel, result.TTLMinutes)
	} else {
		headerTitle = "❌ 权限请求已拒绝 / Permission Denied"
		headerTemplate = "red"
		reason := result.Reason
		if reason == "" {
			reason = "管理员拒绝 / Denied by administrator"
		}
		bodyText = fmt.Sprintf("权限提升请求未通过。\n\n"+
			"**拒绝原因**: %s\n\n"+
			"相关任务已暂停执行。如需继续，请重新发起权限申请。",
			reason)
	}

	return map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"header": map[string]interface{}{
			"title": map[string]interface{}{
				"tag":     "plain_text",
				"content": headerTitle,
			},
			"template": headerTemplate,
		},
		"elements": []interface{}{
			map[string]interface{}{
				"tag": "div",
				"text": map[string]interface{}{
					"tag":     "lark_md",
					"content": bodyText,
				},
			},
			map[string]interface{}{
				"tag": "note",
				"elements": []interface{}{
					map[string]interface{}{
						"tag":     "plain_text",
						"content": fmt.Sprintf("ID: %s", result.EscalationID),
					},
				},
			},
		},
	}
}
