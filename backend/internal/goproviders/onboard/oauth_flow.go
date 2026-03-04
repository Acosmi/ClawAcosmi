// onboard/oauth_flow.go — OAuth VPS 感知处理器
// 对应 TS 文件: src/commands/oauth-flow.ts
// 创建 VPS 感知的 OAuth 认证和提示处理器。
package onboard

import "fmt"

// OAuthPrompt OAuth 提示信息。
type OAuthPrompt struct {
	Message     string
	Placeholder string
}

// WizardPrompter 向导交互提示器接口。
type WizardPrompter interface {
	// Text 文本输入提示
	Text(message string, validate func(string) string) (string, error)
}

// RuntimeEnv 运行时环境接口。
type RuntimeEnv interface {
	Log(msg string)
}

// ProgressSpinner 进度旋转器接口。
type ProgressSpinner interface {
	Update(message string)
	Stop(message string)
}

// VpsAwareOAuthHandlersParams VPS 感知 OAuth 处理器参数。
type VpsAwareOAuthHandlersParams struct {
	IsRemote            bool
	Prompter            WizardPrompter
	Runtime             RuntimeEnv
	Spin                ProgressSpinner
	OpenURL             func(string) error
	LocalBrowserMessage string
	ManualPromptMessage string
}

// VpsAwareOAuthHandlers VPS 感知 OAuth 处理器结果。
type VpsAwareOAuthHandlers struct {
	OnAuth   func(url string) error
	OnPrompt func(prompt OAuthPrompt) (string, error)
}

// validateRequiredInput 验证必填输入。
func validateRequiredInput(value string) string {
	if len(value) == 0 {
		return "Required"
	}
	return ""
}

// CreateVpsAwareOAuthHandlers 创建 VPS 感知 OAuth 处理器。
// 对应 TS: createVpsAwareOAuthHandlers()
func CreateVpsAwareOAuthHandlers(params VpsAwareOAuthHandlersParams) VpsAwareOAuthHandlers {
	manualPromptMessage := params.ManualPromptMessage
	if manualPromptMessage == "" {
		manualPromptMessage = "Paste the redirect URL"
	}
	var manualCodeResult *string

	return VpsAwareOAuthHandlers{
		OnAuth: func(url string) error {
			if params.IsRemote {
				params.Spin.Stop("OAuth URL ready")
				params.Runtime.Log(fmt.Sprintf("\nOpen this URL in your LOCAL browser:\n\n%s\n", url))
				code, err := params.Prompter.Text(manualPromptMessage, validateRequiredInput)
				if err != nil {
					return err
				}
				manualCodeResult = &code
				return nil
			}

			params.Spin.Update(params.LocalBrowserMessage)
			if params.OpenURL != nil {
				if err := params.OpenURL(url); err != nil {
					return err
				}
			}
			params.Runtime.Log(fmt.Sprintf("Open: %s", url))
			return nil
		},
		OnPrompt: func(prompt OAuthPrompt) (string, error) {
			if manualCodeResult != nil {
				return *manualCodeResult, nil
			}
			code, err := params.Prompter.Text(prompt.Message, validateRequiredInput)
			if err != nil {
				return "", err
			}
			return code, nil
		},
	}
}
