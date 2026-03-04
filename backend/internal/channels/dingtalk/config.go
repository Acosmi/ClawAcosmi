package dingtalk

// config.go — 钉钉频道配置解析 + 校验

import (
	"fmt"

	"github.com/Acosmi/ClawAcosmi/internal/channels"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// ResolvedDingTalkAccount 已解析的钉钉账号信息
type ResolvedDingTalkAccount struct {
	AccountID string
	Config    *types.DingTalkAccountConfig
}

// ResolveDingTalkAccount 从配置中解析目标钉钉账号。
func ResolveDingTalkAccount(cfg *types.OpenAcosmiConfig, accountID string) *ResolvedDingTalkAccount {
	if cfg == nil || cfg.Channels.DingTalk == nil {
		return nil
	}

	dtCfg := cfg.Channels.DingTalk
	if accountID == "" {
		accountID = channels.DefaultAccountID
	}

	var acct *types.DingTalkAccountConfig
	if accountID != channels.DefaultAccountID && dtCfg.Accounts != nil {
		acct = dtCfg.Accounts[accountID]
	}
	if acct == nil {
		defaultAcct := dtCfg.DingTalkAccountConfig
		acct = &defaultAcct
	}

	return &ResolvedDingTalkAccount{
		AccountID: accountID,
		Config:    acct,
	}
}

// ValidateDingTalkConfig 校验钉钉账号配置必填字段。
func ValidateDingTalkConfig(acct *types.DingTalkAccountConfig) error {
	if acct == nil {
		return fmt.Errorf("dingtalk account config is nil")
	}
	if acct.AppKey == "" {
		return fmt.Errorf("dingtalk appKey is required")
	}
	if acct.AppSecret == "" {
		return fmt.Errorf("dingtalk appSecret is required")
	}
	return nil
}
