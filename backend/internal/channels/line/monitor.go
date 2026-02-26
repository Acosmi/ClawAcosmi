package line

// TS 对照: src/line/monitor.ts (385L)
// LINE monitor 主入口 — HTTP webhook 注册、bot 初始化、processMessage 绑定

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// ---------- 运行时状态 ----------

// LineRuntimeState LINE 帐号运行时状态。
type LineRuntimeState struct {
	Running        bool       `json:"running"`
	LastStartAt    *time.Time `json:"lastStartAt,omitempty"`
	LastStopAt     *time.Time `json:"lastStopAt,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	LastInboundAt  *time.Time `json:"lastInboundAt,omitempty"`
	LastOutboundAt *time.Time `json:"lastOutboundAt,omitempty"`
}

var (
	runtimeStateMu sync.RWMutex
	runtimeStates  = make(map[string]*LineRuntimeState)
)

func recordRuntimeState(accountID string, update func(s *LineRuntimeState)) {
	key := "line:" + accountID
	runtimeStateMu.Lock()
	defer runtimeStateMu.Unlock()
	s := runtimeStates[key]
	if s == nil {
		s = &LineRuntimeState{}
		runtimeStates[key] = s
	}
	update(s)
}

// GetLineRuntimeState 返回帐号运行时状态（线程安全）。
func GetLineRuntimeState(accountID string) *LineRuntimeState {
	runtimeStateMu.RLock()
	defer runtimeStateMu.RUnlock()
	s := runtimeStates["line:"+accountID]
	if s == nil {
		return nil
	}
	copy := *s
	return &copy
}

// ---------- HTTPRegistry — 轻量 HTTP 路由注册接口 ----------

// HTTPHandler HTTP 处理器函数类型。
type HTTPHandler func(w http.ResponseWriter, r *http.Request)

// HTTPRegistry HTTP 路由注册接口（由上层 server 实现注入）。
type HTTPRegistry interface {
	// Register 注册路由，返回注销函数。
	Register(path string, handler HTTPHandler) (unregister func())
}

// ---------- LoadingKeepalive ----------

// startLoadingKeepalive 每隔 intervalMs 发送一次加载动画请求。
// 返回 stop 函数。TS: startLineLoadingKeepalive()
func startLoadingKeepalive(
	ctx context.Context,
	client *Client,
	userID string,
	intervalMs time.Duration,
	loadingSeconds int,
) func() {
	if intervalMs <= 0 {
		intervalMs = 18 * time.Second
	}
	if loadingSeconds <= 0 {
		loadingSeconds = 20
	}

	stopCh := make(chan struct{})

	trigger := func() {
		if client == nil {
			return
		}
		_ = client.ShowLoadingAnimation(ctx, userID, loadingSeconds)
	}

	trigger()
	go func() {
		ticker := time.NewTicker(intervalMs)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				trigger()
			case <-stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	once := sync.Once{}
	return func() {
		once.Do(func() { close(stopCh) })
	}
}

// ---------- MonitorLineOptions ----------

// MonitorLineOptions LINE monitor 选项。
// TS: MonitorLineProviderOptions
type MonitorLineOptions struct {
	ChannelAccessToken string
	ChannelSecret      string
	AccountID          string
	Config             LineConfig
	// ProcessMessage 注入的消息分发回调（由 autoreply 层提供）。
	ProcessMessage func(ctx context.Context, inbound LineInboundContext) error
	// PairingStore 可选，默认 noopPairingStore。
	PairingStore PairingStore
	// HTTPRegistry 可选，若为 nil 则跳过 HTTP 路由注册。
	HTTPRegistry HTTPRegistry
	// WebhookPath 可选自定义 webhook 路径，默认 /line/webhook。
	WebhookPath string
	// MediaMaxBytes 媒体下载上限，默认 10MB。
	MediaMaxBytes int64
	// LogVerbose 可选 verbose 日志。
	LogVerbose func(string)
	// OnError 错误回调。
	OnError func(error)
	// AbortCtx 取消信号。
	AbortCtx context.Context
}

// LineMonitor LINE monitor 实例。
// TS: LineProviderMonitor
type LineMonitor struct {
	Account       ResolvedLineAccount
	HandleWebhook func(ctx context.Context, body *WebhookBody) error
	Stop          func()
}

// MonitorLineProvider 初始化 LINE provider 并注册 webhook。
// TS: monitorLineProvider()
func MonitorLineProvider(opts MonitorLineOptions) (*LineMonitor, error) {
	accountID := opts.AccountID
	if accountID == "" {
		accountID = "default"
	}

	// 解析帐号
	account, err := ResolveLineAccount(&opts.Config, accountID)
	if err != nil {
		return nil, fmt.Errorf("line: resolve account: %w", err)
	}
	// 注入 resolved config
	account.Config = ResolvedLineConfig{
		AllowFrom:      coalesceSlice(opts.Config.Accounts[accountID].AllowFrom, opts.Config.AllowFrom),
		GroupAllowFrom: coalesceSlice(opts.Config.Accounts[accountID].GroupAllowFrom, opts.Config.GroupAllowFrom),
		DMPolicy:       coalesceStr(opts.Config.Accounts[accountID].DMPolicy, opts.Config.DMPolicy),
		GroupPolicy:    coalesceStr(opts.Config.Accounts[accountID].GroupPolicy, opts.Config.GroupPolicy),
		Groups:         mergeGroups(opts.Config.Accounts[accountID].Groups, opts.Config.Groups),
	}

	client := NewClient(account.ChannelAccessToken, account.ChannelSecret)

	logVerbose := opts.LogVerbose
	if logVerbose == nil {
		logVerbose = func(msg string) { log.Printf("[line] %s", msg) }
	}
	onError := opts.OnError
	if onError == nil {
		onError = func(err error) { log.Printf("[line] error: %v", err) }
	}

	// 记录启动状态
	now := time.Now()
	recordRuntimeState(accountID, func(s *LineRuntimeState) {
		s.Running = true
		s.LastStartAt = &now
	})

	hctx := &LineHandlerContext{
		Account:        *account,
		Config:         opts.Config,
		MediaMaxBytes:  opts.MediaMaxBytes,
		PairingStore:   opts.PairingStore,
		Client:         client,
		ProcessMessage: opts.ProcessMessage,
		LogVerbose:     logVerbose,
		OnError:        onError,
	}

	// handleWebhook 函数
	handleWebhook := func(ctx context.Context, body *WebhookBody) error {
		if body == nil || len(body.Events) == 0 {
			return nil
		}
		logVerbose(fmt.Sprintf("line: received %d webhook events", len(body.Events)))
		HandleLineWebhookEvents(ctx, body.Events, hctx)
		return nil
	}

	// 注册 HTTP webhook
	var unregisterHTTP func()
	if opts.HTTPRegistry != nil {
		webhookPath := opts.WebhookPath
		if webhookPath == "" {
			webhookPath = "/line/webhook"
		}

		unregisterHTTP = opts.HTTPRegistry.Register(webhookPath, func(w http.ResponseWriter, r *http.Request) {
			// GET: webhook 验证
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
				return
			}

			// 只接受 POST
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "GET, POST")
				http.Error(w, `{"error":"Method Not Allowed"}`, http.StatusMethodNotAllowed)
				return
			}

			rawBody, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				http.Error(w, `{"error":"Bad Request"}`, http.StatusBadRequest)
				return
			}

			signature := r.Header.Get("X-Line-Signature")
			if signature == "" {
				logVerbose("line: webhook missing X-Line-Signature header")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"Missing X-Line-Signature header"}`))
				return
			}

			// 签名验证失败 → 401
			if !ValidateLineSignature(rawBody, signature, opts.ChannelSecret) {
				logVerbose("line: webhook signature validation failed")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"Invalid signature"}`))
				return
			}

			var body WebhookBody
			if err := json.Unmarshal(rawBody, &body); err != nil {
				http.Error(w, `{"error":"Invalid body"}`, http.StatusBadRequest)
				return
			}

			// 立即返回 200，异步处理
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))

			// 记录入站时间
			inboundNow := time.Now()
			recordRuntimeState(accountID, func(s *LineRuntimeState) {
				s.LastInboundAt = &inboundNow
			})

			// 异步处理事件
			if len(body.Events) > 0 {
				go func() {
					bgCtx := context.Background()
					if opts.AbortCtx != nil {
						bgCtx = opts.AbortCtx
					}
					if err := handleWebhook(bgCtx, &body); err != nil {
						onError(fmt.Errorf("line webhook handler failed: %w", err))
					}
				}()
			}
		})
		logVerbose(fmt.Sprintf("line: registered webhook handler at %s", webhookPath))
	}

	// 停止函数
	stopOnce := sync.Once{}
	stop := func() {
		stopOnce.Do(func() {
			logVerbose(fmt.Sprintf("line: stopping provider for account %s", accountID))
			if unregisterHTTP != nil {
				unregisterHTTP()
			}
			stopNow := time.Now()
			recordRuntimeState(accountID, func(s *LineRuntimeState) {
				s.Running = false
				s.LastStopAt = &stopNow
			})
		})
	}

	// 如果传入 AbortCtx，监听取消信号
	if opts.AbortCtx != nil {
		go func() {
			<-opts.AbortCtx.Done()
			stop()
		}()
	}

	return &LineMonitor{
		Account:       *account,
		HandleWebhook: handleWebhook,
		Stop:          stop,
	}, nil
}

// ---------- 辅助函数 ----------

func coalesceSlice(values ...[]string) []string {
	for _, v := range values {
		if len(v) > 0 {
			return v
		}
	}
	return nil
}

func coalesceStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func mergeGroups(primary, fallback map[string]LineGroupConfig) map[string]LineGroupConfig {
	if len(primary) > 0 {
		return primary
	}
	return fallback
}
