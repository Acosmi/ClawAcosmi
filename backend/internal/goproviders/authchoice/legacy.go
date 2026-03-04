// authchoice/legacy.go — 遗留认证选择兼容
// 对应 TS 文件: src/commands/auth-choice-legacy.ts
package authchoice

import "github.com/Acosmi/ClawAcosmi/internal/goproviders/types"

// AuthChoiceLegacyAliasesForCLI CLI 中仍然接受的遗留别名列表。
// 对应 TS: AUTH_CHOICE_LEGACY_ALIASES_FOR_CLI
var AuthChoiceLegacyAliasesForCLI = []types.AuthChoice{
	types.AuthChoiceSetupToken,
	types.AuthChoiceOAuth,
	types.AuthChoiceClaudeCLI,
	types.AuthChoiceCodexCLI,
	types.AuthChoiceMinimaxCloud,
	types.AuthChoiceMinimax,
}

// NormalizeLegacyOnboardAuthChoice 将遗留认证选择标准化为当前使用的值。
// 对应 TS: normalizeLegacyOnboardAuthChoice()
func NormalizeLegacyOnboardAuthChoice(authChoice *types.AuthChoice) *types.AuthChoice {
	if authChoice == nil {
		return nil
	}
	switch *authChoice {
	case types.AuthChoiceOAuth, types.AuthChoiceClaudeCLI:
		result := types.AuthChoiceSetupToken
		return &result
	case types.AuthChoiceCodexCLI:
		result := types.AuthChoiceOpenAICodex
		return &result
	default:
		return authChoice
	}
}

// IsDeprecatedAuthChoice 检查给定的认证选择是否已废弃。
// 对应 TS: isDeprecatedAuthChoice()
func IsDeprecatedAuthChoice(authChoice *types.AuthChoice) bool {
	if authChoice == nil {
		return false
	}
	return *authChoice == types.AuthChoiceClaudeCLI || *authChoice == types.AuthChoiceCodexCLI
}
