package feishu

// config.go — 飞书频道配置解析 + 校验

import (
	"fmt"

	"github.com/openacosmi/claw-acismi/internal/channels"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

const (
	// DomainFeishu 飞书国内版
	DomainFeishu = "feishu"
	// DomainLark 飞书国际版
	DomainLark = "lark"
)

// ResolvedFeishuAccount 已解析的飞书账号信息
type ResolvedFeishuAccount struct {
	AccountID string
	Config    *types.FeishuAccountConfig
}

// ResolveFeishuAccount 从配置中解析目标飞书账号。
func ResolveFeishuAccount(cfg *types.OpenAcosmiConfig, accountID string) *ResolvedFeishuAccount {
	if cfg == nil || cfg.Channels.Feishu == nil {
		return nil
	}

	feishuCfg := cfg.Channels.Feishu
	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	var acct *types.FeishuAccountConfig
	if accountID != channels.DefaultAccountID && feishuCfg.Accounts != nil {
		acct = feishuCfg.Accounts[accountID]
	}
	if acct == nil {
		defaultAcct := feishuCfg.FeishuAccountConfig
		acct = &defaultAcct
	}

	return &ResolvedFeishuAccount{
		AccountID: accountID,
		Config:    acct,
	}
}

// ValidateFeishuConfig 校验飞书账号配置必填字段。
func ValidateFeishuConfig(acct *types.FeishuAccountConfig) error {
	if acct == nil {
		return fmt.Errorf("feishu account config is nil")
	}
	if acct.AppID == "" {
		return fmt.Errorf("feishu appId is required")
	}
	if acct.AppSecret == "" {
		return fmt.Errorf("feishu appSecret is required")
	}
	return nil
}

// IsLarkDomain 判断是否为国际版（Lark）域名
func IsLarkDomain(domain string) bool {
	return domain == DomainLark
}
