package gateway

// remote_approval.go — P4 远程审批通知器
// 行业对照: ServiceNow Mobile Approval / Slack Approval Workflow
//
// 当智能体请求提权时，通过外部消息平台（飞书/钉钉/企业微信）发送互动卡片，
// 用户在手机端审批/拒绝后通过 HTTP Webhook 回调通知 Gateway。
//
// 设计：
//   - Provider 接口：每个消息平台一个实现
//   - RemoteApprovalNotifier：持有多个 Provider，扇出通知
//   - 配置持久化到 ~/.openacosmi/remote-approval-config.json

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ---------- Provider 接口 ----------

// RemoteApprovalProvider 远程审批消息 Provider 接口。
// 每个外部消息平台（飞书/钉钉/企业微信）实现此接口。
type RemoteApprovalProvider interface {
	// Name 返回 Provider 名称（如 "feishu", "dingtalk", "wecom"）。
	Name() string
	// SendApprovalRequest 发送审批请求卡片到外部平台。
	SendApprovalRequest(ctx context.Context, req ApprovalCardRequest) error
	// ValidateConfig 验证当前 Provider 配置是否有效。
	ValidateConfig() error
}

// ApprovalCardRequest 审批卡片请求参数。
type ApprovalCardRequest struct {
	EscalationID   string    `json:"escalationId"`
	RequestedLevel string    `json:"requestedLevel"`
	Reason         string    `json:"reason"`
	RunID          string    `json:"runId,omitempty"`
	SessionID      string    `json:"sessionId,omitempty"`
	TTLMinutes     int       `json:"ttlMinutes"`
	CallbackURL    string    `json:"callbackUrl"`
	RequestedAt    time.Time `json:"requestedAt"`
	// OriginatorChatID 发起操作的群聊 ID（如飞书 chat_id），用于审批卡片群发。
	OriginatorChatID string `json:"originatorChatId,omitempty"`
	// OriginatorUserID 发起远程操作的用户 ID（如飞书 open_id），用于审批卡片私聊。
	OriginatorUserID string `json:"originatorUserId,omitempty"`
}

// ApprovalCallbackPayload 外部平台回调载荷。
type ApprovalCallbackPayload struct {
	EscalationID string `json:"escalationId"`
	Approved     bool   `json:"approved"`
	TTLMinutes   int    `json:"ttlMinutes,omitempty"`
	Provider     string `json:"provider"`
	ApproverID   string `json:"approverId,omitempty"`
	ApproverName string `json:"approverName,omitempty"`
}

// ---------- 远程审批配置 ----------

// RemoteApprovalConfig 远程审批全局配置。
type RemoteApprovalConfig struct {
	Enabled     bool                    `json:"enabled"`
	CallbackURL string                  `json:"callbackUrl"` // 公网可达的回调地址
	Feishu      *FeishuProviderConfig   `json:"feishu,omitempty"`
	DingTalk    *DingTalkProviderConfig `json:"dingtalk,omitempty"`
	WeCom       *WeComProviderConfig    `json:"wecom,omitempty"`
}

// FeishuProviderConfig 飞书 Provider 配置。
type FeishuProviderConfig struct {
	Enabled   bool   `json:"enabled"`
	AppID     string `json:"appId"`
	AppSecret string `json:"appSecret"`
	ChatID    string `json:"chatId,omitempty"` // 目标群聊 ID
	UserID    string `json:"userId,omitempty"` // 目标用户 open_id
}

// DingTalkProviderConfig 钉钉 Provider 配置。
type DingTalkProviderConfig struct {
	Enabled       bool   `json:"enabled"`
	AppKey        string `json:"appKey"`
	AppSecret     string `json:"appSecret"`
	RobotCode     string `json:"robotCode,omitempty"`
	WebhookURL    string `json:"webhookUrl,omitempty"` // 自定义机器人 Webhook
	WebhookSecret string `json:"webhookSecret,omitempty"`
	ApiSecret     string `json:"apiSecret,omitempty"` // Phase 8: 互动卡片回调验签
}

// WeComProviderConfig 企业微信 Provider 配置。
type WeComProviderConfig struct {
	Enabled        bool   `json:"enabled"`
	CorpID         string `json:"corpId"`
	AgentID        int    `json:"agentId"`
	Secret         string `json:"secret"`
	ToUser         string `json:"toUser,omitempty"`         // 接收用户 ID（| 分隔）
	ToParty        string `json:"toParty,omitempty"`        // 接收部门 ID
	Token          string `json:"token,omitempty"`          // Phase 8: 回调签名 Token
	EncodingAESKey string `json:"encodingAESKey,omitempty"` // Phase 8: AES 解密密钥
}

const remoteApprovalConfigFile = "remote-approval-config.json"

// ---------- 远程审批通知管理器 ----------

// RemoteApprovalNotifier 远程审批通知管理器。
// 持有多个 Provider，提权请求时扇出通知到所有已启用的平台。
type RemoteApprovalNotifier struct {
	mu          sync.RWMutex
	config      RemoteApprovalConfig
	providers   []RemoteApprovalProvider
	logger      *slog.Logger
	broadcaster *Broadcaster // 发送失败时广播通知前端
}

// NewRemoteApprovalNotifier 创建远程审批通知管理器。
// 自动加载配置并初始化已启用的 Provider。
func NewRemoteApprovalNotifier(broadcaster *Broadcaster) *RemoteApprovalNotifier {
	n := &RemoteApprovalNotifier{
		logger:      slog.Default().With("component", "remote-approval"),
		broadcaster: broadcaster,
	}
	// 加载配置
	if err := n.loadConfig(); err != nil {
		n.logger.Warn("加载远程审批配置失败，使用默认配置", "error", err)
	}
	n.rebuildProviders()
	return n
}

// InjectChannelFeishuConfig 从频道配置自动补充飞书审批凭据。
// 当 remote-approval-config.json 未配置飞书时，复用频道插件的 AppID/AppSecret。
// ChatID/UserID 由运行时 ApprovalCardRequest 动态传入，无需静态配置。
// 由 server.go 在频道插件初始化后调用。
func (n *RemoteApprovalNotifier) InjectChannelFeishuConfig(appID, appSecret string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 如果已有专用飞书审批配置且已启用，不覆盖
	if n.config.Feishu != nil && n.config.Feishu.Enabled && n.config.Feishu.AppID != "" {
		n.logger.Debug("飞书审批已有专用配置，跳过频道凭据注入")
		return
	}

	n.logger.Info("从飞书频道配置自动注入审批凭据",
		"appId", appID[:min(4, len(appID))]+"***",
	)

	n.config.Feishu = &FeishuProviderConfig{
		Enabled:   true,
		AppID:     appID,
		AppSecret: appSecret,
	}
	n.config.Enabled = true
	n.rebuildProviders()
}

// NotifyAll 向所有已启用的 Provider 发送审批通知。
// 不阻塞调用方——异步发送，收集错误日志。
func (n *RemoteApprovalNotifier) NotifyAll(req ApprovalCardRequest) {
	n.mu.RLock()
	if !n.config.Enabled || len(n.providers) == 0 {
		n.mu.RUnlock()
		return
	}
	providers := make([]RemoteApprovalProvider, len(n.providers))
	copy(providers, n.providers)
	n.mu.RUnlock()

	// 填充回调 URL
	if req.CallbackURL == "" {
		n.mu.RLock()
		req.CallbackURL = n.config.CallbackURL
		n.mu.RUnlock()
	}

	for _, p := range providers {
		go func(prov RemoteApprovalProvider) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := prov.SendApprovalRequest(ctx, req); err != nil {
				n.logger.Error("远程审批通知发送失败",
					"provider", prov.Name(),
					"escalation_id", req.EscalationID,
					"error", err,
				)
				// 广播发送失败事件，让前端知道远程审批卡片未送达
				if n.broadcaster != nil {
					n.broadcaster.Broadcast("remote.approval.sendFailed", map[string]interface{}{
						"provider":     prov.Name(),
						"escalationId": req.EscalationID,
						"error":        err.Error(),
					}, nil)
				}
			} else {
				n.logger.Info("远程审批通知已发送",
					"provider", prov.Name(),
					"escalation_id", req.EscalationID,
				)
			}
		}(p)
	}
}

// ---------- Phase 8: 审批结果通知 ----------

// ApprovalResultNotification 审批结果通知参数。
type ApprovalResultNotification struct {
	EscalationID   string `json:"escalationId"`
	Approved       bool   `json:"approved"`
	Reason         string `json:"reason"`         // 拒绝原因（超时/手动拒绝）
	RequestedLevel string `json:"requestedLevel"` // 请求的级别
	TTLMinutes     int    `json:"ttlMinutes"`     // 批准时的授权时长
}

// ResultNotifier 可选接口——Provider 实现后可推送审批结果卡片。
type ResultNotifier interface {
	SendResultNotification(ctx context.Context, result ApprovalResultNotification) error
}

// NotifyResult 审批结束后向所有已启用的平台推送结果卡片。
// 使用 type assertion 检测 Provider 是否支持结果通知。
func (n *RemoteApprovalNotifier) NotifyResult(result ApprovalResultNotification) {
	n.mu.RLock()
	if !n.config.Enabled || len(n.providers) == 0 {
		n.mu.RUnlock()
		return
	}
	providers := make([]RemoteApprovalProvider, len(n.providers))
	copy(providers, n.providers)
	n.mu.RUnlock()

	for _, p := range providers {
		rn, ok := p.(ResultNotifier)
		if !ok {
			continue
		}
		go func(notifier ResultNotifier, name string) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := notifier.SendResultNotification(ctx, result); err != nil {
				n.logger.Error("审批结果通知发送失败",
					"provider", name,
					"escalation_id", result.EscalationID,
					"error", err,
				)
			} else {
				n.logger.Info("审批结果通知已发送",
					"provider", name,
					"escalation_id", result.EscalationID,
					"approved", result.Approved,
				)
			}
		}(rn, p.Name())
	}
}

// GetConfig 获取当前配置（脱敏）。
func (n *RemoteApprovalNotifier) GetConfig() RemoteApprovalConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.config
}

// GetConfigSanitized 获取脱敏后的配置（隐藏 secret 字段），用于前端展示。
func (n *RemoteApprovalNotifier) GetConfigSanitized() RemoteApprovalConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()

	cfg := n.config

	if cfg.Feishu != nil {
		copy := *cfg.Feishu
		if copy.AppSecret != "" {
			copy.AppSecret = "***"
		}
		cfg.Feishu = &copy
	}
	if cfg.DingTalk != nil {
		copy := *cfg.DingTalk
		if copy.AppSecret != "" {
			copy.AppSecret = "***"
		}
		if copy.WebhookSecret != "" {
			copy.WebhookSecret = "***"
		}
		cfg.DingTalk = &copy
	}
	if cfg.WeCom != nil {
		copy := *cfg.WeCom
		if copy.Secret != "" {
			copy.Secret = "***"
		}
		cfg.WeCom = &copy
	}
	return cfg
}

// UpdateConfig 更新配置，保存到磁盘并重建 Provider。
func (n *RemoteApprovalNotifier) UpdateConfig(cfg RemoteApprovalConfig) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 如果前端传回 "***"，保留原有 secret
	if cfg.Feishu != nil && n.config.Feishu != nil && cfg.Feishu.AppSecret == "***" {
		cfg.Feishu.AppSecret = n.config.Feishu.AppSecret
	}
	if cfg.DingTalk != nil && n.config.DingTalk != nil {
		if cfg.DingTalk.AppSecret == "***" {
			cfg.DingTalk.AppSecret = n.config.DingTalk.AppSecret
		}
		if cfg.DingTalk.WebhookSecret == "***" {
			cfg.DingTalk.WebhookSecret = n.config.DingTalk.WebhookSecret
		}
	}
	if cfg.WeCom != nil && n.config.WeCom != nil && cfg.WeCom.Secret == "***" {
		cfg.WeCom.Secret = n.config.WeCom.Secret
	}

	n.config = cfg
	if err := n.saveConfigLocked(); err != nil {
		return fmt.Errorf("保存远程审批配置失败: %w", err)
	}
	n.rebuildProviders()
	return nil
}

// TestProvider 测试指定 Provider 是否可正常发送消息。
func (n *RemoteApprovalNotifier) TestProvider(providerName string) error {
	n.mu.RLock()
	defer n.mu.RUnlock()

	for _, p := range n.providers {
		if p.Name() == providerName {
			if err := p.ValidateConfig(); err != nil {
				return fmt.Errorf("配置验证失败: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return p.SendApprovalRequest(ctx, ApprovalCardRequest{
				EscalationID:   "test_" + fmt.Sprintf("%d", time.Now().UnixMilli()),
				RequestedLevel: "full",
				Reason:         "测试远程审批连接 / Test remote approval connection",
				TTLMinutes:     30,
				CallbackURL:    n.config.CallbackURL,
				RequestedAt:    time.Now(),
			})
		}
	}
	return fmt.Errorf("provider %q 未启用或不存在", providerName)
}

// EnabledProviderNames 返回当前已启用的 Provider 名称列表。
func (n *RemoteApprovalNotifier) EnabledProviderNames() []string {
	n.mu.RLock()
	defer n.mu.RUnlock()

	names := make([]string, 0, len(n.providers))
	for _, p := range n.providers {
		names = append(names, p.Name())
	}
	return names
}

// ---------- 配置持久化 ----------

func (n *RemoteApprovalNotifier) configFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".openacosmi", remoteApprovalConfigFile)
}

func (n *RemoteApprovalNotifier) loadConfig() error {
	data, err := os.ReadFile(n.configFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 无配置文件，使用默认值
		}
		return err
	}
	return json.Unmarshal(data, &n.config)
}

func (n *RemoteApprovalNotifier) saveConfigLocked() error {
	filePath := n.configFilePath()
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(n.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0o600)
}

// ---------- Provider 构建 ----------

func (n *RemoteApprovalNotifier) rebuildProviders() {
	n.providers = nil

	if n.config.Feishu != nil && n.config.Feishu.Enabled {
		n.providers = append(n.providers, newFeishuProvider(n.config.Feishu))
	}
	if n.config.DingTalk != nil && n.config.DingTalk.Enabled {
		n.providers = append(n.providers, newDingTalkProvider(n.config.DingTalk))
	}
	if n.config.WeCom != nil && n.config.WeCom.Enabled {
		n.providers = append(n.providers, newWeComProvider(n.config.WeCom))
	}
}
