package signal

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/anthropic/open-acosmi/internal/autoreply"
	"github.com/anthropic/open-acosmi/internal/autoreply/reply"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// Signal 监控入口 — 继承自 src/signal/monitor.ts (401L)

// MonitorSignalOpts 监控配置
type MonitorSignalOpts struct {
	AccountID   string
	Deps        *SignalMonitorDeps
	OnEvent     func(event SignalSSEvent)
	OnDaemonLog func(level SignalLogLevel, line string)
	OnStarted   func(baseURL string, version string)
	OnError     func(err error)
	LogInfo     func(msg string)
	LogError    func(msg string)
}

// MonitorSignalProvider 启动 Signal 监控：
// 1. 解析账户配置
// 2. 可选启动 daemon
// 3. 探测健康
// 4. 启动 SSE 事件循环
func MonitorSignalProvider(ctx context.Context, cfg *types.OpenAcosmiConfig, opts MonitorSignalOpts) error {
	account := ResolveSignalAccount(cfg, opts.AccountID)
	if !account.Enabled {
		return fmt.Errorf("signal account %q is disabled", opts.AccountID)
	}

	logInfo := opts.LogInfo
	if logInfo == nil {
		logInfo = func(string) {}
	}
	logError := opts.LogError
	if logError == nil {
		logError = func(string) {}
	}

	baseURL := account.BaseURL
	signalAccount := account.Config.Account

	// 检查是否需要自动启动 daemon
	autoStart := false
	if account.Config.AutoStart != nil {
		autoStart = *account.Config.AutoStart
	}

	var daemonHandle *SignalDaemonHandle
	if autoStart {
		cliPath := account.Config.CliPath
		if cliPath == "" {
			cliPath = "signal-cli"
		}
		host := strings.TrimSpace(account.Config.HttpHost)
		if host == "" {
			host = "127.0.0.1"
		}
		port := 8080
		if account.Config.HttpPort != nil {
			port = *account.Config.HttpPort
		}

		logInfo(fmt.Sprintf("signal: spawning daemon (cli=%s, account=%s, http=%s:%d)",
			cliPath, signalAccount, host, port))

		// 对齐 TS: 传递 daemon 配置标志
		receiveMode := account.Config.ReceiveMode
		ignoreAttachments := false
		if account.Config.IgnoreAttachments != nil {
			ignoreAttachments = *account.Config.IgnoreAttachments
		}
		ignoreStories := false
		if account.Config.IgnoreStories != nil {
			ignoreStories = *account.Config.IgnoreStories
		}
		sendReadReceipts := false
		if account.Config.SendReadReceipts != nil {
			sendReadReceipts = *account.Config.SendReadReceipts
		}

		handle, err := SpawnSignalDaemon(ctx, SignalDaemonOpts{
			CliPath:           cliPath,
			Account:           signalAccount,
			HttpHost:          host,
			HttpPort:          port,
			ReceiveMode:       receiveMode,
			IgnoreAttachments: ignoreAttachments,
			IgnoreStories:     ignoreStories,
			SendReadReceipts:  sendReadReceipts,
		}, func(level SignalLogLevel, line string) {
			if opts.OnDaemonLog != nil {
				opts.OnDaemonLog(level, line)
			}
		})
		if err != nil {
			return fmt.Errorf("spawn daemon: %w", err)
		}
		daemonHandle = handle
		defer daemonHandle.Stop()

		// 对齐 TS: 默认 30s，clamp 在 [1s, 120s]
		timeoutMs := 30000
		if account.Config.StartupTimeoutMs != nil {
			timeoutMs = *account.Config.StartupTimeoutMs
		}
		if timeoutMs < 1000 {
			timeoutMs = 1000
		}
		if timeoutMs > 120000 {
			timeoutMs = 120000
		}
		if err := waitForSignalDaemonReady(ctx, baseURL, time.Duration(timeoutMs)*time.Millisecond, logInfo); err != nil {
			return fmt.Errorf("daemon startup: %w", err)
		}
	}

	// 探测
	probe := ProbeSignal(ctx, baseURL, signalAccount)
	if !probe.OK {
		return fmt.Errorf("signal probe failed: %s", probe.Error)
	}
	logInfo(fmt.Sprintf("signal: connected (version=%s, elapsed=%dms)", probe.Version, probe.Elapsed))

	if opts.OnStarted != nil {
		opts.OnStarted(baseURL, probe.Version)
	}

	// 对齐 TS: readReceiptsViaDaemon = autoStart && sendReadReceipts
	readReceiptsViaDaemon := autoStart && func() bool {
		if account.Config.SendReadReceipts != nil {
			return *account.Config.SendReadReceipts
		}
		return false
	}()

	// 对齐 TS: 群组历史配置
	historyLimit := reply.DefaultGroupHistoryLimit
	if account.Config.HistoryLimit != nil && *account.Config.HistoryLimit >= 0 {
		historyLimit = *account.Config.HistoryLimit
	} else if cfg != nil && cfg.Messages != nil && cfg.Messages.GroupChat != nil && cfg.Messages.GroupChat.HistoryLimit != nil {
		historyLimit = *cfg.Messages.GroupChat.HistoryLimit
	}
	groupHistories := reply.NewHistoryMap()

	// 创建事件处理器
	handler := CreateSignalEventHandler(SignalEventHandlerDeps{
		Ctx:                   ctx,
		CFG:                   cfg,
		BaseURL:               baseURL,
		Account:               signalAccount,
		AccountID:             opts.AccountID,
		Deps:                  opts.Deps,
		GroupHistories:        groupHistories,
		HistoryLimit:          historyLimit,
		ReadReceiptsViaDaemon: readReceiptsViaDaemon,
		LogInfo:               logInfo,
		LogError:              logError,
		OnError:               opts.OnError,
	})

	// 启动 SSE 事件循环
	return RunSignalSseLoop(ctx, SSEReconnectConfig{
		BaseURL: baseURL,
		Account: signalAccount,
		Handler: func(event SignalSSEvent) {
			handler(event)
			if opts.OnEvent != nil {
				opts.OnEvent(event)
			}
		},
		LogInfo:  logInfo,
		LogError: logError,
	})
}

// waitForSignalDaemonReady 等待 daemon 按就绪
func waitForSignalDaemonReady(ctx context.Context, baseURL string, timeout time.Duration, logInfo func(string)) error {
	deadline := time.Now().Add(timeout)
	interval := 500 * time.Millisecond

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for signal daemon after %s", timeout)
		}

		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := SignalCheck(checkCtx, baseURL)
		cancel()

		if err == nil {
			logInfo("signal: daemon ready")
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// SignalReplyPayload 回复载荷
type SignalReplyPayload struct {
	Text      string   `json:"text,omitempty"`
	MediaPath string   `json:"mediaPath,omitempty"`
	MediaURL  string   `json:"mediaUrl,omitempty"`
	MediaURLs []string `json:"mediaUrls,omitempty"`
}

// DeliverSignalReplies 投递 Signal 回复消息（含分块 + 表格转换）
// 对齐 TS monitor.ts deliverReplies: chunkMode + textLimit 文本分块
func DeliverSignalReplies(ctx context.Context, params DeliverSignalRepliesParams) error {
	for _, r := range params.Replies {
		text := r.Text
		mediaURLs := r.MediaURLs
		if r.MediaURL != "" && len(mediaURLs) == 0 {
			mediaURLs = []string{r.MediaURL}
		}
		if text == "" && len(mediaURLs) == 0 && r.MediaPath == "" {
			continue
		}

		// 本地附件消息
		if r.MediaPath != "" {
			if err := SendMessageWithAttachment(ctx, params.Target, text, r.MediaPath, params.SendOpts); err != nil {
				return fmt.Errorf("signal deliver attachment: %w", err)
			}
			continue
		}

		// 远程 URL 附件（对齐 TS: 首个 URL 携带 caption，后续不携带）
		if len(mediaURLs) > 0 {
			first := true
			for _, u := range mediaURLs {
				caption := ""
				if first {
					caption = text
					first = false
				}
				opts := params.SendOpts
				opts.MediaURL = u
				if _, err := SendMessageSignal(ctx, params.Target, caption, opts); err != nil {
					return fmt.Errorf("signal deliver media reply: %w", err)
				}
			}
			continue
		}

		// 纯文本（对齐 TS: 使用 chunkTextWithMode 分块）
		textLimit := params.TextLimit
		if textLimit <= 0 {
			textLimit = autoreply.DefaultChunkLimit
		}
		chunks := autoreply.ChunkTextWithMode(text, textLimit, params.ChunkMode)
		for _, chunk := range chunks {
			if _, err := SendMessageSignal(ctx, params.Target, chunk, params.SendOpts); err != nil {
				return fmt.Errorf("signal deliver reply: %w", err)
			}
		}
	}
	return nil
}

// DeliverSignalRepliesParams 回复投递参数
type DeliverSignalRepliesParams struct {
	Replies   []SignalReplyPayload
	Target    string
	SendOpts  SignalSendOpts
	TextLimit int
	ChunkMode autoreply.ChunkMode
}
