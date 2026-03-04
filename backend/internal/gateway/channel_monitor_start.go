package gateway

// channel_monitor_start.go — Monitor 模式渠道启动编排
// 从配置读取 Discord/Telegram/Slack 的 token 和账号信息，构建 opts，启动 goroutine。

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/Acosmi/ClawAcosmi/internal/channels"
	"github.com/Acosmi/ClawAcosmi/internal/channels/discord"
	slackch "github.com/Acosmi/ClawAcosmi/internal/channels/slack"
	"github.com/Acosmi/ClawAcosmi/internal/channels/telegram"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// startMonitorChannels 启动 Monitor 模式渠道（Discord/Telegram/Slack）。
// 每个渠道在独立 goroutine 中运行，使用 ctx 控制生命周期。
func startMonitorChannels(ctx context.Context, dctx *ChannelDepsContext, cfg *types.OpenAcosmiConfig, mux *http.ServeMux) {
	if cfg == nil || cfg.Channels == nil {
		return
	}

	// Discord
	if cfg.Channels.Discord != nil {
		startDiscordMonitors(ctx, dctx, cfg)
	}

	// Telegram
	if cfg.Channels.Telegram != nil {
		startTelegramMonitors(ctx, dctx, cfg)
	}

	// Slack
	if cfg.Channels.Slack != nil {
		startSlackMonitors(ctx, dctx, cfg, mux)
	}
}

// startDiscordMonitors 启动 Discord 渠道监控（支持多账户）。
func startDiscordMonitors(ctx context.Context, dctx *ChannelDepsContext, cfg *types.OpenAcosmiConfig) {
	dc := cfg.Channels.Discord
	deps := BuildDiscordDeps(dctx)

	// 默认账户
	defaultToken := discord.ResolveDiscordToken(cfg)
	if defaultToken.Token != "" {
		accountID := channels.DefaultAccountID
		slog.Info("channel: starting Discord monitor", "account", accountID)
		go func() {
			err := discord.MonitorDiscordProvider(ctx, cfg, discord.MonitorDiscordOpts{
				Token:     defaultToken.Token,
				AccountID: string(accountID),
				Mode:      discord.MonitorDiscordModeGateway,
				Deps:      deps,
			})
			if err != nil {
				slog.Error("channel: Discord monitor exited", "account", accountID, "error", err)
			}
		}()
	} else {
		slog.Info("channel: Discord token not configured, skipping")
	}

	// 多账户
	for accountName, acct := range dc.Accounts {
		if acct == nil {
			continue
		}
		token := discord.ResolveDiscordToken(cfg, discord.WithAccountID(accountName))
		if token.Token == "" {
			continue
		}
		name := accountName // capture
		slog.Info("channel: starting Discord monitor", "account", name)
		go func() {
			err := discord.MonitorDiscordProvider(ctx, cfg, discord.MonitorDiscordOpts{
				Token:     token.Token,
				AccountID: name,
				Mode:      discord.MonitorDiscordModeGateway,
				Deps:      deps,
			})
			if err != nil {
				slog.Error("channel: Discord monitor exited", "account", name, "error", err)
			}
		}()
	}
}

// startTelegramMonitors 启动 Telegram 渠道监控（支持多账户）。
func startTelegramMonitors(ctx context.Context, dctx *ChannelDepsContext, cfg *types.OpenAcosmiConfig) {
	tc := cfg.Channels.Telegram
	deps := BuildTelegramDeps(dctx)

	// 默认账户
	defaultToken := telegram.ResolveTelegramToken(cfg, string(channels.DefaultAccountID))
	if defaultToken.Token != "" {
		accountID := channels.DefaultAccountID
		slog.Info("channel: starting Telegram monitor", "account", accountID)
		go func() {
			err := telegram.MonitorTelegramProvider(ctx, telegram.MonitorConfig{
				Token:     defaultToken.Token,
				AccountID: string(accountID),
				Config:    cfg,
				Deps:      deps,
			})
			if err != nil {
				slog.Error("channel: Telegram monitor exited", "account", accountID, "error", err)
			}
		}()
	} else {
		slog.Info("channel: Telegram token not configured, skipping")
	}

	// 多账户
	for accountName, acct := range tc.Accounts {
		if acct == nil {
			continue
		}
		token := telegram.ResolveTelegramToken(cfg, accountName)
		if token.Token == "" {
			continue
		}
		name := accountName
		slog.Info("channel: starting Telegram monitor", "account", name)
		go func() {
			err := telegram.MonitorTelegramProvider(ctx, telegram.MonitorConfig{
				Token:     token.Token,
				AccountID: name,
				Config:    cfg,
				Deps:      deps,
			})
			if err != nil {
				slog.Error("channel: Telegram monitor exited", "account", name, "error", err)
			}
		}()
	}
}

// startSlackMonitors 启动 Slack 渠道监控（支持多账户）。
func startSlackMonitors(ctx context.Context, dctx *ChannelDepsContext, cfg *types.OpenAcosmiConfig, mux *http.ServeMux) {
	sc := cfg.Channels.Slack
	deps := BuildSlackDeps(dctx)

	// 默认账户
	botToken := slackch.ResolveSlackBotToken(sc.BotToken)
	appToken := slackch.ResolveSlackAppToken(sc.AppToken)
	if botToken != "" {
		accountID := channels.DefaultAccountID
		account := slackch.ResolveSlackAccount(cfg, string(accountID))
		mode := slackch.InferSlackMode(account)

		opts := slackch.MonitorSlackOpts{
			AccountID:     string(accountID),
			Mode:          mode,
			BotToken:      botToken,
			AppToken:      appToken,
			SigningSecret: sc.SigningSecret,
			WebhookPath:   sc.WebhookPath,
		}

		slog.Info("channel: starting Slack monitor", "account", accountID, "mode", mode)
		go func() {
			err := slackch.MonitorSlackProvider(ctx, cfg, opts)
			if err != nil {
				slog.Error("channel: Slack monitor exited", "account", accountID, "error", err)
			}
		}()
	} else {
		slog.Info("channel: Slack bot token not configured, skipping")
	}

	// 多账户
	for accountName, acct := range sc.Accounts {
		if acct == nil {
			continue
		}
		bt := slackch.ResolveSlackBotToken(acct.BotToken)
		if bt == "" {
			continue
		}
		at := slackch.ResolveSlackAppToken(acct.AppToken)
		name := accountName
		account := slackch.ResolveSlackAccount(cfg, name)
		mode := slackch.InferSlackMode(account)

		slog.Info("channel: starting Slack monitor", "account", name, "mode", mode)
		go func() {
			err := slackch.MonitorSlackProvider(ctx, cfg, slackch.MonitorSlackOpts{
				AccountID:     name,
				Mode:          mode,
				BotToken:      bt,
				AppToken:      at,
				SigningSecret: acct.SigningSecret,
				WebhookPath:   acct.WebhookPath,
			})
			if err != nil {
				slog.Error("channel: Slack monitor exited", "account", name, "error", err)
			}
		}()
	}

	// 注意: Slack HTTP 模式的 webhook 路由在 MonitorSlackProvider 内部处理
	// （monitorSlackHTTP 自行启动 HTTP server），因此此处无需额外注册 mux 路由。
	_ = mux
	_ = deps
}
