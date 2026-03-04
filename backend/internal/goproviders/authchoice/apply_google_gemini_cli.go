// authchoice/apply_google_gemini_cli.go — Google Gemini CLI 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.google-gemini-cli.ts
// Google Gemini CLI OAuth 流程：先显示风险警告，确认后委托 plugin provider。
package authchoice

import (
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ApplyAuthChoiceGoogleGeminiCli Google Gemini CLI 认证 apply。
// 显示风险警告，用户确认后委托 ApplyAuthChoicePluginProvider。
// 对应 TS: applyAuthChoiceGoogleGeminiCli()
func ApplyAuthChoiceGoogleGeminiCli(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	if params.AuthChoice != types.AuthChoiceGoogleGeminiCLI {
		return nil, nil
	}

	// 显示风险警告
	cautionMsg := strings.Join([]string{
		"This is an unofficial integration and is not endorsed by Google.",
		"Some users have reported account restrictions or suspensions after using third-party Gemini CLI and Antigravity OAuth clients.",
		"Proceed only if you understand and accept this risk.",
	}, "\n")
	_ = params.Prompter.Note(cautionMsg, "Google Gemini CLI caution")

	// 确认继续
	proceed, err := params.Prompter.Confirm(ConfirmPromptOptions{
		Message:      "Continue with Google Gemini CLI OAuth?",
		InitialValue: false,
	})
	if err != nil {
		return nil, err
	}

	if !proceed {
		_ = params.Prompter.Note("Skipped Google Gemini CLI OAuth setup.", "Setup skipped")
		return &ApplyAuthChoiceResult{Config: params.Config}, nil
	}

	// 委托 plugin provider
	return ApplyAuthChoicePluginProvider(params, PluginProviderAuthChoiceOptions{
		AuthChoice: "google-gemini-cli",
		PluginID:   "google-gemini-cli-auth",
		ProviderID: "google-gemini-cli",
		MethodID:   "oauth",
		Label:      "Google Gemini CLI",
	})
}
