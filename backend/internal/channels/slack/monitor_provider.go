package slack

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	slackapi "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// Slack 监控入口 — 继承自 src/slack/monitor/provider.ts (380L)
// Phase 9 实现：Socket Mode (slack-go/slack) + HTTP Events API。

// MonitorSlackProvider 启动 Slack 监控提供者。
func MonitorSlackProvider(ctx context.Context, cfg *types.OpenAcosmiConfig, opts MonitorSlackOpts) error {
	monCtx := NewSlackMonitorContext(cfg, opts.AccountID, nil)
	monCtx.Mode = opts.Mode
	if monCtx.Mode == "" {
		monCtx.Mode = MonitorModeSocket
	}

	// auth.test 获取 bot 信息
	monCtx.PerformAuthTest()

	log.Printf("[slack:%s] monitor starting (mode=%s bot=%s team=%s)",
		monCtx.AccountID, monCtx.Mode, monCtx.BotUserID, monCtx.TeamID)

	switch monCtx.Mode {
	case MonitorModeSocket:
		return monitorSlackSocket(ctx, monCtx, opts)
	case MonitorModeHTTP:
		return monitorSlackHTTP(ctx, monCtx, opts)
	default:
		return fmt.Errorf("slack: unsupported monitor mode %q", monCtx.Mode)
	}
}

// ── Socket Mode (slack-go/slack/socketmode) ──

// monitorSlackSocket 使用 slack-go 的 socketmode 包实现 Socket Mode。
// 全托管 WebSocket 连接：自动重连、心跳、envelope ack。
func monitorSlackSocket(ctx context.Context, monCtx *SlackMonitorContext, opts MonitorSlackOpts) error {
	if opts.AppToken == "" {
		return fmt.Errorf("slack socket mode requires app token")
	}

	// 创建 slack API 客户端（需要同时提供 bot token 和 app-level token）
	api := slackapi.New(
		monCtx.Account.BotToken,
		slackapi.OptionAppLevelToken(opts.AppToken),
	)

	// 创建 Socket Mode 客户端
	smClient := socketmode.New(api)

	log.Printf("[slack:%s] socket mode: starting managed WebSocket connection", monCtx.AccountID)

	// 事件处理 goroutine
	go func() {
		for evt := range smClient.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				handleSocketModeEventsAPI(ctx, monCtx, smClient, evt)

			case socketmode.EventTypeSlashCommand:
				handleSocketModeSlashCommand(ctx, monCtx, smClient, evt)

			case socketmode.EventTypeInteractive:
				// 交互事件（按钮、菜单等）— 先 ack
				if evt.Request != nil {
					smClient.Ack(*evt.Request)
				}
				log.Printf("[slack:%s] socket: interactive event (not yet handled)", monCtx.AccountID)

			case socketmode.EventTypeConnected:
				log.Printf("[slack:%s] socket: connected to Slack", monCtx.AccountID)

			case socketmode.EventTypeDisconnect:
				log.Printf("[slack:%s] socket: disconnected, will reconnect...", monCtx.AccountID)

			case socketmode.EventTypeHello:
				log.Printf("[slack:%s] socket: hello received", monCtx.AccountID)

			case socketmode.EventTypeIncomingError:
				log.Printf("[slack:%s] socket: incoming error: %v", monCtx.AccountID, evt.Data)

			case socketmode.EventTypeConnectionError:
				log.Printf("[slack:%s] socket: connection error: %v", monCtx.AccountID, evt.Data)
			}
		}
	}()

	// RunContext 阻塞直到 ctx 取消，自动管理 WebSocket 连接和重连
	return smClient.RunContext(ctx)
}

// handleSocketModeEventsAPI 处理 Socket Mode 的 Events API 事件。
func handleSocketModeEventsAPI(ctx context.Context, monCtx *SlackMonitorContext, smClient *socketmode.Client, evt socketmode.Event) {
	// 必须先 ack
	if evt.Request != nil {
		smClient.Ack(*evt.Request)
	}

	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		return
	}

	// 事件过滤
	if monCtx.ShouldDropMismatchedEvent(eventsAPIEvent.APIAppID, eventsAPIEvent.TeamID) {
		return
	}

	// 解析内部事件并分发
	innerEvent := eventsAPIEvent.InnerEvent
	eventRaw, err := json.Marshal(innerEvent.Data)
	if err != nil {
		return
	}

	go dispatchSlackEvent(monCtx, eventRaw)
}

// handleSocketModeSlashCommand 处理 Socket Mode 的斜杠命令。
func handleSocketModeSlashCommand(ctx context.Context, monCtx *SlackMonitorContext, smClient *socketmode.Client, evt socketmode.Event) {
	// 先 ack
	if evt.Request != nil {
		smClient.Ack(*evt.Request)
	}

	cmd, ok := evt.Data.(slackapi.SlashCommand)
	if !ok {
		return
	}

	payload := map[string]string{
		"command":      cmd.Command,
		"text":         cmd.Text,
		"user_id":      cmd.UserID,
		"channel_id":   cmd.ChannelID,
		"response_url": cmd.ResponseURL,
		"trigger_id":   cmd.TriggerID,
	}

	go func() {
		if err := HandleSlackSlashCommand(ctx, monCtx, payload); err != nil {
			log.Printf("[slack:%s] socket: slash command error: %v", monCtx.AccountID, err)
		}
	}()
}

// slackEventHeader 事件头部（仅用于解析 type）
type slackEventHeader struct {
	Type string `json:"type"`
}

// ── HTTP Mode ──

// monitorSlackHTTP HTTP Events API 实现。
// 处理 Slack Events API 的 HTTP POST 请求。
func monitorSlackHTTP(ctx context.Context, monCtx *SlackMonitorContext, opts MonitorSlackOpts) error {
	webhookPath := NormalizeSlackWebhookPath(opts.WebhookPath)
	signingSecret := opts.SigningSecret

	log.Printf("[slack:%s] HTTP mode: registering webhook at %s", monCtx.AccountID, webhookPath)

	unregister := RegisterSlackHttpHandler(webhookPath, func(w http.ResponseWriter, r *http.Request) {
		handleSlackHTTPEvent(w, r, monCtx, signingSecret)
	}, func(msg string) {
		log.Printf("[slack:%s] %s", monCtx.AccountID, msg)
	}, monCtx.AccountID)

	defer unregister()

	log.Printf("[slack:%s] HTTP mode: listening at %s", monCtx.AccountID, webhookPath)

	<-ctx.Done()
	return ctx.Err()
}

// handleSlackHTTPEvent 处理单个 HTTP 事件请求。
func handleSlackHTTPEvent(w http.ResponseWriter, r *http.Request, monCtx *SlackMonitorContext, signingSecret string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 签名验证
	if signingSecret != "" {
		if !verifySlackSignature(r, body, signingSecret) {
			log.Printf("[slack:%s] HTTP: invalid signature", monCtx.AccountID)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	// 解析事件类型
	var envelope struct {
		Type      string          `json:"type"`
		Challenge string          `json:"challenge,omitempty"`
		Event     json.RawMessage `json:"event,omitempty"`
		APIAppID  string          `json:"api_app_id,omitempty"`
		TeamID    string          `json:"team_id,omitempty"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// url_verification challenge
	if envelope.Type == "url_verification" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"challenge": envelope.Challenge})
		return
	}

	// 事件过滤
	if monCtx.ShouldDropMismatchedEvent(envelope.APIAppID, envelope.TeamID) {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 分发事件
	if envelope.Type == "event_callback" && len(envelope.Event) > 0 {
		go dispatchSlackEvent(monCtx, envelope.Event)
	}

	w.WriteHeader(http.StatusOK)
}

// verifySlackSignature 验证 Slack 请求签名（HMAC-SHA256）。
func verifySlackSignature(r *http.Request, body []byte, signingSecret string) bool {
	timestamp := r.Header.Get("X-Slack-Request-Timestamp")
	signature := r.Header.Get("X-Slack-Signature")

	if timestamp == "" || signature == "" {
		return false
	}

	// 防重放: 检查时间戳是否在 5 分钟内
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if abs64(time.Now().Unix()-ts) > 300 {
		return false
	}

	// HMAC-SHA256
	baseString := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	mac := hmac.New(sha256.New, []byte(signingSecret))
	mac.Write([]byte(baseString))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// dispatchSlackEvent 分发已解析的 Slack 事件到对应处理器。
func dispatchSlackEvent(monCtx *SlackMonitorContext, eventRaw json.RawMessage) {
	var header slackEventHeader
	if err := json.Unmarshal(eventRaw, &header); err != nil {
		return
	}

	switch header.Type {
	case "message", "app_mention":
		dispatchSlackMessageLikeEvent(monCtx, header.Type, eventRaw)
	case "reaction_added":
		var ev SlackReactionEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackReactionAddedEvent(context.Background(), monCtx, ev)
		}
	case "reaction_removed":
		var ev SlackReactionEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackReactionRemovedEvent(context.Background(), monCtx, ev)
		}
	case "channel_rename":
		var ev SlackChannelEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackChannelRenameEvent(context.Background(), monCtx, ev)
		}
	case "channel_archive":
		var ev SlackChannelEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackChannelArchiveEvent(context.Background(), monCtx, ev)
		}
	case "channel_unarchive":
		var ev SlackChannelEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackChannelUnarchiveEvent(context.Background(), monCtx, ev)
		}
	case "channel_deleted":
		var ev SlackChannelEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackChannelDeletedEvent(context.Background(), monCtx, ev)
		}
	case "channel_created":
		var ev SlackChannelEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackChannelCreatedEvent(context.Background(), monCtx, ev)
		}
	case "channel_id_changed":
		var ev SlackChannelEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackChannelIDChangedEvent(context.Background(), monCtx, ev)
		}
	case "member_joined_channel":
		var ev SlackMemberChannelEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackMemberJoinedChannelEvent(context.Background(), monCtx, ev)
		}
	case "member_left_channel":
		var ev SlackMemberChannelEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackMemberLeftChannelEvent(context.Background(), monCtx, ev)
		}
	case "pin_added":
		var ev SlackPinEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackPinAddedEvent(context.Background(), monCtx, ev)
		}
	case "pin_removed":
		var ev SlackPinEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackPinRemovedEvent(context.Background(), monCtx, ev)
		}
	}
}

// dispatchSlackMessageLikeEvent 分发消息类事件。
func dispatchSlackMessageLikeEvent(monCtx *SlackMonitorContext, eventType string, eventRaw json.RawMessage) {
	if eventType == "app_mention" {
		var ev SlackAppMentionEvent
		if json.Unmarshal(eventRaw, &ev) == nil {
			HandleSlackAppMentionEvent(context.Background(), monCtx, ev)
		}
		return
	}

	var ev SlackMessageEvent
	if json.Unmarshal(eventRaw, &ev) == nil {
		HandleSlackMessageEvent(context.Background(), monCtx, ev)
	}
}

// ── 辅助函数 ──

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// InferSlackMode 从配置推断 Slack 监控模式。
func InferSlackMode(account ResolvedSlackAccount) MonitorSlackMode {
	if account.Config.Mode != "" {
		return MonitorSlackMode(strings.ToLower(account.Config.Mode))
	}
	if account.AppToken != "" {
		return MonitorModeSocket
	}
	return MonitorModeHTTP
}
