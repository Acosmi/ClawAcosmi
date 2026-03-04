//go:build darwin

package imessage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/autoreply"
	"github.com/Acosmi/ClawAcosmi/internal/channels"
	"github.com/Acosmi/ClawAcosmi/pkg/markdown"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// iMessage 入站监控 — 继承自 src/imessage/monitor/monitor-provider.ts (750L)
// + src/imessage/monitor/deliver.ts (70L) + src/imessage/monitor/runtime.ts (19L)

// SentMessageCache 最近发送消息缓存，用于回声检测
// key: scope:text, value: timestamp, TTL 5秒
type SentMessageCache struct {
	mu    sync.Mutex
	cache map[string]time.Time
	ttl   time.Duration
}

// NewSentMessageCache 创建发送消息缓存
func NewSentMessageCache() *SentMessageCache {
	return &SentMessageCache{
		cache: make(map[string]time.Time),
		ttl:   5 * time.Second,
	}
}

// Remember 记录发送的消息
func (c *SentMessageCache) Remember(scope, text string) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return
	}
	key := scope + ":" + trimmed
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = time.Now()
	c.cleanup()
}

// Has 检查是否最近发送过该消息
func (c *SentMessageCache) Has(scope, text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	key := scope + ":" + trimmed
	c.mu.Lock()
	defer c.mu.Unlock()
	ts, ok := c.cache[key]
	if !ok {
		return false
	}
	if time.Since(ts) > c.ttl {
		delete(c.cache, key)
		return false
	}
	return true
}

// cleanup 清理过期缓存条目
func (c *SentMessageCache) cleanup() {
	now := time.Now()
	for key, ts := range c.cache {
		if now.Sub(ts) > c.ttl {
			delete(c.cache, key)
		}
	}
}

// detectRemoteHostFromCliPath 从 SSH 包装脚本中检测远程主机
// 匹配如 `exec ssh -T openacosmi@192.168.64.3 /opt/homebrew/bin/imsg "$@"` 的模式
func detectRemoteHostFromCliPath(cliPath string) string {
	expanded := cliPath
	if strings.HasPrefix(expanded, "~") {
		home := os.Getenv("HOME")
		expanded = strings.Replace(expanded, "~", home, 1)
	}

	content, err := os.ReadFile(expanded)
	if err != nil {
		return ""
	}

	// 匹配 user@host 模式
	userHostRe := regexp.MustCompile(`\bssh\b[^\n]*?\s+([a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+)`)
	if matches := userHostRe.FindStringSubmatch(string(content)); len(matches) > 1 {
		return matches[1]
	}

	// 回退：匹配 host-only（ssh -T mac-mini imsg）
	hostOnlyRe := regexp.MustCompile(`\bssh\b[^\n]*?\s+([a-zA-Z][a-zA-Z0-9._-]*)\s+\S*\bimsg\b`)
	if matches := hostOnlyRe.FindStringSubmatch(string(content)); len(matches) > 1 {
		return matches[1]
	}

	return ""
}

// normalizeAllowList 规范化允许列表（W3-D2: 委托给 channels.InterfaceSliceToStringSlice + DeduplicateAllowlist）
func normalizeAllowList(list []interface{}) []string {
	return channels.DeduplicateAllowlist(channels.InterfaceSliceToStringSlice(list))
}

// MonitorIMessageProvider 启动 iMessage 入站监控
func MonitorIMessageProvider(ctx context.Context, opts MonitorIMessageOpts, logInfo func(string), logError func(string)) error {
	if logInfo == nil {
		logInfo = func(string) {}
	}
	if logError == nil {
		logError = func(string) {}
	}

	// 解析配置
	cfg := opts.Config
	if cfg == nil {
		return fmt.Errorf("config is required for iMessage monitor")
	}

	account := ResolveIMessageAccount(cfg, opts.AccountID)
	imsgCfg := account.Config

	cliPath := opts.CliPath
	if cliPath == "" {
		cliPath = imsgCfg.CliPath
	}
	if cliPath == "" {
		cliPath = "imsg"
	}
	dbPath := opts.DbPath
	if dbPath == "" {
		dbPath = imsgCfg.DbPath
	}

	probeTimeoutMs := DefaultProbeTimeoutMs
	if imsgCfg.ProbeTimeoutMs != nil {
		probeTimeoutMs = *imsgCfg.ProbeTimeoutMs
	}

	includeAttachments := false
	if opts.IncludeAttachments != nil {
		includeAttachments = *opts.IncludeAttachments
	} else if imsgCfg.IncludeAttachments != nil {
		includeAttachments = *imsgCfg.IncludeAttachments
	}

	// 检测远程主机
	remoteHost := imsgCfg.RemoteHost
	if remoteHost == "" && cliPath != "imsg" {
		remoteHost = detectRemoteHostFromCliPath(cliPath)
		if remoteHost != "" {
			logInfo(fmt.Sprintf("imessage: detected remoteHost=%s from cliPath", remoteHost))
		}
	}

	// 等待传输层就绪（探测循环）
	logInfo(fmt.Sprintf("imessage: waiting for imsg rpc (cliPath=%s, accountId=%s)",
		cliPath, account.AccountID))

	if err := waitForProbeReady(ctx, cliPath, dbPath, probeTimeoutMs, cfg, logInfo); err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// 创建 RPC 客户端
	sentCache := NewSentMessageCache()
	groupHistories := NewGroupHistories()
	deps := opts.Deps
	_ = remoteHost // 用于入站消息 MediaRemoteHost

	// 解析 allow 列表
	allowFrom := normalizeAllowList(opts.AllowFrom)
	if len(allowFrom) == 0 {
		allowFrom = normalizeAllowList(imsgCfg.AllowFrom)
	}
	groupAllowFrom := normalizeAllowList(opts.GroupAllowFrom)
	if len(groupAllowFrom) == 0 {
		groupAllowFrom = normalizeAllowList(imsgCfg.GroupAllowFrom)
		if len(groupAllowFrom) == 0 && len(imsgCfg.AllowFrom) > 0 {
			groupAllowFrom = normalizeAllowList(imsgCfg.AllowFrom)
		}
	}

	mediaMaxBytes := 16 * 1024 * 1024
	if opts.MediaMaxMB != nil {
		mediaMaxBytes = *opts.MediaMaxMB * 1024 * 1024
	} else if imsgCfg.MediaMaxMB != nil {
		mediaMaxBytes = *imsgCfg.MediaMaxMB * 1024 * 1024
	}

	textLimit := autoreply.ResolveTextChunkLimit(nil, "", autoreply.DefaultChunkLimit)

	// 入站防抖
	configuredDebounce := 0
	if cfg.Messages != nil && cfg.Messages.Inbound != nil && cfg.Messages.Inbound.DebounceMs != nil {
		configuredDebounce = *cfg.Messages.Inbound.DebounceMs
	}
	inboundDebounceMs := autoreply.ResolveInboundDebounceMs(configuredDebounce, 0)

	client, err := CreateIMessageRpcClient(ctx, IMessageRpcClientOptions{
		CliPath:  cliPath,
		DbPath:   dbPath,
		LogInfo:  logInfo,
		LogError: logError,
		OnNotification: func(msg IMessageRpcNotification) {
			if msg.Method == "message" {
				// 解析消息用于防抖
				var msgWrapper struct {
					Message *IMessagePayload `json:"message"`
				}
				if err := json.Unmarshal(msg.Params, &msgWrapper); err != nil || msgWrapper.Message == nil {
					return
				}
				// 入站消息通过防抖处理
				pipelineMsg := msgWrapper.Message
				if inboundDebounceMs > 0 {
					// 将消息入队防抖器（简化版：直接处理）
					go func() {
						if err := HandleInboundMessageFull(InboundPipelineParams{
							Ctx:                ctx,
							Message:            pipelineMsg,
							Account:            account,
							Config:             cfg,
							SentCache:          sentCache,
							GroupHistories:     groupHistories,
							Client:             nil, // 在 goroutine 中 client 尚未赋值
							Deps:               deps,
							AllowFrom:          allowFrom,
							GroupAllowFrom:     groupAllowFrom,
							IncludeAttachments: includeAttachments,
							MediaMaxBytes:      mediaMaxBytes,
							TextLimit:          textLimit,
							RemoteHost:         remoteHost,
							RequireMentionOpt:  opts.RequireMention,
							LogInfo:            logInfo,
							LogError:           logError,
						}); err != nil {
							logError(fmt.Sprintf("imessage: handler failed: %s", err))
						}
					}()
				} else {
					go func() {
						if err := HandleInboundMessageFull(InboundPipelineParams{
							Ctx:                ctx,
							Message:            pipelineMsg,
							Account:            account,
							Config:             cfg,
							SentCache:          sentCache,
							GroupHistories:     groupHistories,
							Client:             nil,
							Deps:               deps,
							AllowFrom:          allowFrom,
							GroupAllowFrom:     groupAllowFrom,
							IncludeAttachments: includeAttachments,
							MediaMaxBytes:      mediaMaxBytes,
							TextLimit:          textLimit,
							RemoteHost:         remoteHost,
							RequireMentionOpt:  opts.RequireMention,
							LogInfo:            logInfo,
							LogError:           logError,
						}); err != nil {
							logError(fmt.Sprintf("imessage: handler failed: %s", err))
						}
					}()
				}
			} else if msg.Method == "error" {
				logError(fmt.Sprintf("imessage: watch error: %s", string(msg.Params)))
			}
		},
	})
	if err != nil {
		return fmt.Errorf("imessage: create rpc client: %w", err)
	}
	defer client.Stop()

	// 订阅消息流
	subscribeParams := map[string]interface{}{
		"attachments": includeAttachments,
	}
	subResult, err := client.Request(ctx, "watch.subscribe", subscribeParams, DefaultProbeTimeoutMs)
	if err != nil {
		return fmt.Errorf("imessage: watch.subscribe failed: %w", err)
	}

	// H1: 保存 subscriptionId 用于取消时发送 unsubscribe
	var subscriptionID int
	if subResult != nil {
		var subResp struct {
			Subscription int `json:"subscription"`
		}
		if json.Unmarshal(subResult, &subResp) == nil && subResp.Subscription != 0 {
			subscriptionID = subResp.Subscription
		}
	}

	logInfo(fmt.Sprintf("imessage: monitor started, watching (accountId=%s)", account.AccountID))

	// 等待连接关闭或 ctx 取消
	select {
	case <-ctx.Done():
		// H1: 取消时先发送 watch.unsubscribe，再 Stop
		if subscriptionID != 0 {
			unsubCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, _ = client.Request(unsubCtx, "watch.unsubscribe", map[string]interface{}{
				"subscription": subscriptionID,
			}, 2000)
			cancel()
		}
		return ctx.Err()
	case <-client.closed:
		return fmt.Errorf("imessage: rpc connection closed unexpectedly")
	}
}

// waitForProbeReady 等待 imsg rpc 可达，轮询探测
func waitForProbeReady(ctx context.Context, cliPath, dbPath string, probeTimeoutMs int, cfg *types.OpenAcosmiConfig, logInfo func(string)) error {
	deadline := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("imessage: probe timeout (30s)")
		case <-ticker.C:
			probe := ProbeIMessage(ctx, probeTimeoutMs, IMessageProbeOptions{
				CliPath: cliPath,
				DbPath:  dbPath,
				Config:  cfg,
			})
			if probe.OK {
				logInfo("imessage: imsg rpc ready")
				return nil
			}
			if probe.Fatal {
				return fmt.Errorf("imessage: %s", probe.Error)
			}
		}
	}
}

// DeliverReplies 投递回复消息（含 IM-D: 分块 + 表格转换）
func DeliverReplies(ctx context.Context, params DeliverRepliesParams) error {
	// 解析表格转换模式
	tableMode := resolveIMessageTableMode(params.Config)

	for _, reply := range params.Replies {
		text := strings.TrimSpace(reply.Text)
		if text == "" {
			continue
		}

		// IM-D: Markdown 表格转换
		if tableMode != "" && tableMode != types.MarkdownTableOff {
			text = markdown.ConvertMarkdownTables(text, markdown.TableMode(tableMode))
		}

		// IM-D: 文本分块
		chunkLimit := params.TextLimit
		if chunkLimit <= 0 {
			chunkLimit = autoreply.DefaultChunkLimit
		}
		chunks := autoreply.ChunkTextWithMode(text, chunkLimit, autoreply.ChunkModeLength)
		if len(chunks) == 0 {
			chunks = []string{text}
		}

		for _, chunk := range chunks {
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				continue
			}

			// 记录回声检测
			if params.SentMessageCache != nil {
				scope := params.AccountID + ":" + params.Target
				params.SentMessageCache.Remember(scope, chunk)
			}

			// 发送消息
			_, err := SendMessageIMessage(ctx, params.Target, chunk, IMessageSendOpts{
				MaxBytes:  params.MaxBytes,
				Client:    params.Client,
				AccountID: params.AccountID,
			}, params.Config)
			if err != nil {
				return fmt.Errorf("deliver reply: %w", err)
			}
		}
	}
	return nil
}

// DeliverRepliesParams 回复投递参数
type DeliverRepliesParams struct {
	Replies          []ReplyPayload
	Target           string
	Client           *IMessageRpcClient
	AccountID        string
	Config           *types.OpenAcosmiConfig
	MaxBytes         int
	TextLimit        int
	SentMessageCache *SentMessageCache
}

// ReplyPayload 回复载荷
type ReplyPayload struct {
	Text      string   `json:"text,omitempty"`
	MediaUrl  string   `json:"mediaUrl,omitempty"`
	MediaUrls []string `json:"mediaUrls,omitempty"`
}
