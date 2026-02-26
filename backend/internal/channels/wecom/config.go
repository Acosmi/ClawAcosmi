package wecom

// config.go — 企业微信频道配置解析 + 校验

import (
	"fmt"

	"github.com/anthropic/open-acosmi/internal/channels"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// ResolvedWeComAccount 已解析的企业微信账号信息
type ResolvedWeComAccount struct {
	AccountID string
	Config    *types.WeComAccountConfig
}

// ResolveWeComAccount 从配置中解析目标企业微信账号。
func ResolveWeComAccount(cfg *types.OpenAcosmiConfig, accountID string) *ResolvedWeComAccount {
	if cfg == nil || cfg.Channels.WeCom == nil {
		return nil
	}

	wcCfg := cfg.Channels.WeCom
	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	var acct *types.WeComAccountConfig
	if accountID != channels.DefaultAccountID && wcCfg.Accounts != nil {
		acct = wcCfg.Accounts[accountID]
	}
	if acct == nil {
		defaultAcct := wcCfg.WeComAccountConfig
		acct = &defaultAcct
	}

	return &ResolvedWeComAccount{
		AccountID: accountID,
		Config:    acct,
	}
}

// ValidateWeComConfig 校验企业微信账号配置必填字段。
func ValidateWeComConfig(acct *types.WeComAccountConfig) error {
	if acct == nil {
		return fmt.Errorf("wecom account config is nil")
	}
	if acct.CorpID == "" {
		return fmt.Errorf("wecom corpId is required")
	}
	if acct.Secret == "" {
		return fmt.Errorf("wecom secret is required")
	}
	return nil
}
