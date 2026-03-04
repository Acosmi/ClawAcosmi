// authchoice/apply_oauth.go — OAuth 类型 apply（Chutes OAuth）
// 对应 TS 文件: src/commands/auth-choice.apply.oauth.ts
package authchoice

import (
	"fmt"
	"os"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ApplyAuthChoiceOAuth 处理 OAuth 类型的 AuthChoice apply。
// 当前仅处理 Chutes OAuth 流程。
// 对应 TS: applyAuthChoiceOAuth()
func ApplyAuthChoiceOAuth(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	if params.AuthChoice != types.AuthChoiceChutes {
		return nil, nil
	}

	nextConfig := params.Config
	isRemote := IsRemoteEnvironment()

	// 解析 OAuth 参数（从环境变量或交互获取）
	redirectURI := "http://127.0.0.1:1456/oauth-callback"
	if env := strings.TrimSpace(os.Getenv("CHUTES_OAUTH_REDIRECT_URI")); env != "" {
		redirectURI = env
	}

	scopes := "openid profile chutes:invoke"
	if env := strings.TrimSpace(os.Getenv("CHUTES_OAUTH_SCOPES")); env != "" {
		scopes = env
	}

	clientID := strings.TrimSpace(os.Getenv("CHUTES_CLIENT_ID"))
	if clientID == "" {
		// 交互获取 Client ID
		raw, err := params.Prompter.Text(TextPromptOptions{
			Message:     "Enter Chutes OAuth client id",
			Placeholder: "cid_xxx",
			Validate: func(value string) string {
				if strings.TrimSpace(value) != "" {
					return ""
				}
				return "Required"
			},
		})
		if err != nil {
			return nil, err
		}
		clientID = strings.TrimSpace(raw)
	}

	clientSecret := strings.TrimSpace(os.Getenv("CHUTES_CLIENT_SECRET"))

	// 显示 OAuth 提示信息
	var noteMsg string
	if isRemote {
		noteMsg = strings.Join([]string{
			"You are running in a remote/VPS environment.",
			"A URL will be shown for you to open in your LOCAL browser.",
			"After signing in, paste the redirect URL back here.",
			"",
			fmt.Sprintf("Redirect URI: %s", redirectURI),
		}, "\n")
	} else {
		noteMsg = strings.Join([]string{
			"Browser will open for Chutes authentication.",
			"If the callback doesn't auto-complete, paste the redirect URL.",
			"",
			fmt.Sprintf("Redirect URI: %s", redirectURI),
		}, "\n")
	}
	_ = params.Prompter.Note(noteMsg, "Chutes OAuth")

	// 启动 OAuth 流程
	spin := params.Prompter.Progress("Starting OAuth flow…")
	scopeList := strings.Fields(scopes)

	handlers := CreateVpsAwareOAuthHandlers(VpsAwareOAuthHandlersParams{
		IsRemote:            isRemote,
		Prompter:            params.Prompter,
		Runtime:             params.Runtime,
		Spin:                spin,
		OpenURL:             OpenURL,
		LocalBrowserMessage: "Complete sign-in in browser…",
	})

	creds, err := LoginChutes(LoginChutesParams{
		App: ChutesApp{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURI:  redirectURI,
			Scopes:       scopeList,
		},
		Manual:     isRemote,
		OnAuth:     handlers.OnAuth,
		OnPrompt:   handlers.OnPrompt,
		OnProgress: func(msg string) { spin.Update(msg) },
	})
	if err != nil {
		spin.Stop("Chutes OAuth failed")
		params.Runtime.Error(fmt.Sprint(err))
		_ = params.Prompter.Note(
			strings.Join([]string{
				"Trouble with OAuth?",
				"Verify CHUTES_CLIENT_ID (and CHUTES_CLIENT_SECRET if required).",
				fmt.Sprintf("Verify the OAuth app redirect URI includes: %s", redirectURI),
				"Chutes docs: https://chutes.ai/docs/sign-in-with-chutes/overview",
			}, "\n"),
			"OAuth help",
		)
		return &ApplyAuthChoiceResult{Config: nextConfig}, nil
	}

	spin.Stop("Chutes OAuth complete")

	// 保存 OAuth 凭据并更新配置
	profileID, wErr := WriteOAuthCredentials("chutes", creds, params.AgentDir)
	if wErr != nil {
		return nil, wErr
	}
	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
		ProfileID: profileID,
		Provider:  "chutes",
		Mode:      "oauth",
	})

	return &ApplyAuthChoiceResult{Config: nextConfig}, nil
}
