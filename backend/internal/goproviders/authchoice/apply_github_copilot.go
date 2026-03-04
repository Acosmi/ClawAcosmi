// authchoice/apply_github_copilot.go — GitHub Copilot 认证 apply
// 对应 TS 文件: src/commands/auth-choice.apply.github-copilot.ts
package authchoice

import (
	"os"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// ApplyAuthChoiceGitHubCopilot GitHub Copilot 认证 apply。
// 显示说明信息、检查 TTY、执行设备登录、配置 Profile 和默认模型。
// 对应 TS: applyAuthChoiceGitHubCopilot()
func ApplyAuthChoiceGitHubCopilot(params ApplyAuthChoiceParams) (*ApplyAuthChoiceResult, error) {
	if params.AuthChoice != types.AuthChoiceGitHubCopilot {
		return nil, nil
	}

	nextConfig := params.Config

	// 显示说明信息
	noteMsg := strings.Join([]string{
		"This will open a GitHub device login to authorize Copilot.",
		"Requires an active GitHub Copilot subscription.",
	}, "\n")
	_ = params.Prompter.Note(noteMsg, "GitHub Copilot")

	// 检查是否为交互式 TTY
	stat, _ := os.Stdin.Stat()
	isTTY := (stat.Mode() & os.ModeCharDevice) != 0
	if !isTTY {
		_ = params.Prompter.Note(
			"GitHub Copilot login requires an interactive TTY.",
			"GitHub Copilot",
		)
		return &ApplyAuthChoiceResult{Config: nextConfig}, nil
	}

	// 执行 GitHub Copilot 登录（stub）
	err := GitHubCopilotLoginCommand(params.Runtime)
	if err != nil {
		_ = params.Prompter.Note(
			"GitHub Copilot login failed: "+err.Error(),
			"GitHub Copilot",
		)
		return &ApplyAuthChoiceResult{Config: nextConfig}, nil
	}

	// 更新配置
	nextConfig = ApplyAuthProfileConfig(nextConfig, ApplyProfileConfigParams{
		ProfileID: "github-copilot:github",
		Provider:  "github-copilot",
		Mode:      "token",
	})

	// 设置默认模型
	if params.SetDefaultModel {
		model := "github-copilot/gpt-4o"
		nextConfig = ApplyPrimaryModel(nextConfig, model)
		_ = params.Prompter.Note("Default model set to "+model, "Model configured")
	}

	return &ApplyAuthChoiceResult{Config: nextConfig}, nil
}
