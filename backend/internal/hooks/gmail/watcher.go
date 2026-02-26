package gmail

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// --- Gmail Watcher Service ---
// 对应 TS: gmail-watcher.ts

var addressInUseRe = regexp.MustCompile(`(?i)address already in use|EADDRINUSE`)

// IsAddressInUseError 判断是否为地址占用错误
func IsAddressInUseError(line string) bool {
	return addressInUseRe.MatchString(line)
}

// GmailWatcher Gmail watcher 服务
type GmailWatcher struct {
	mu            sync.Mutex
	process       *exec.Cmd
	cancelRenew   context.CancelFunc
	shuttingDown  bool
	currentConfig *GmailHookRuntimeConfig
	logger        *slog.Logger
}

// NewGmailWatcher 创建 watcher 实例
func NewGmailWatcher(logger *slog.Logger) *GmailWatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &GmailWatcher{
		logger: logger.With("subsystem", "gmail-watcher"),
	}
}

// GmailWatcherStartResult watcher 启动结果
type GmailWatcherStartResult struct {
	Started bool   `json:"started"`
	Reason  string `json:"reason,omitempty"`
}

// Start 启动 Gmail watcher 服务
// 对应 TS: gmail-watcher.ts startGmailWatcher
func (w *GmailWatcher) Start(cfg *types.OpenAcosmiConfig) GmailWatcherStartResult {
	// 检查 skip 环境变量
	if os.Getenv("OPENACOSMI_SKIP_GMAIL_WATCHER") == "1" {
		return GmailWatcherStartResult{Started: false, Reason: "skipped via env"}
	}

	if cfg == nil || cfg.Hooks == nil || cfg.Hooks.Enabled == nil || !*cfg.Hooks.Enabled {
		return GmailWatcherStartResult{Started: false, Reason: "hooks not enabled"}
	}

	if cfg.Hooks.Gmail == nil || cfg.Hooks.Gmail.Account == "" {
		return GmailWatcherStartResult{Started: false, Reason: "no gmail account configured"}
	}

	if !HasBinary("gog") {
		return GmailWatcherStartResult{Started: false, Reason: "gog binary not found"}
	}

	runtimeCfg, err := ResolveGmailHookRuntimeConfig(cfg, GmailHookOverrides{})
	if err != nil {
		return GmailWatcherStartResult{Started: false, Reason: err.Error()}
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	w.currentConfig = runtimeCfg
	w.shuttingDown = false

	// Setup Tailscale if needed
	if runtimeCfg.Tailscale.Mode != "off" {
		endpoint, err := EnsureTailscaleEndpoint(
			runtimeCfg.Tailscale.Mode,
			runtimeCfg.Tailscale.Path,
			runtimeCfg.Serve.Port,
			runtimeCfg.Tailscale.Target,
			"",
		)
		if err != nil {
			w.logger.Error("tailscale setup failed", "error", err)
			return GmailWatcherStartResult{
				Started: false,
				Reason:  fmt.Sprintf("tailscale setup failed: %s", err),
			}
		}
		if endpoint != "" {
			w.logger.Info("tailscale configured", "mode", runtimeCfg.Tailscale.Mode, "port", runtimeCfg.Serve.Port)
		}
	}

	// Start Gmail watch (register with API)
	if err := StartGmailWatch(runtimeCfg.Account, runtimeCfg.Label, runtimeCfg.Topic, 120_000); err != nil {
		w.logger.Warn("gmail watch start failed, continuing with serve", "error", err)
	}

	// Spawn serve process
	w.spawnServe()

	// Setup renew interval
	renewMs := time.Duration(runtimeCfg.RenewEveryMinutes) * time.Minute
	ctx, cancel := context.WithCancel(context.Background())
	w.cancelRenew = cancel
	go w.renewLoop(ctx, renewMs)

	w.logger.Info("gmail watcher started",
		"account", runtimeCfg.Account,
		"renewMinutes", runtimeCfg.RenewEveryMinutes,
	)

	return GmailWatcherStartResult{Started: true}
}

func (w *GmailWatcher) spawnServe() {
	if w.currentConfig == nil {
		return
	}

	args := BuildGogWatchServeArgs(w.currentConfig)
	w.logger.Info("starting gog", "args", strings.Join(args, " "))

	cmd := exec.Command("gog", args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		w.logger.Error("failed to start gog serve", "error", err)
		return
	}

	w.process = cmd

	// Monitor exit in background
	go func() {
		err := cmd.Wait()
		w.mu.Lock()
		defer w.mu.Unlock()

		if w.shuttingDown {
			return
		}

		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}

		w.logger.Warn("gog exited, restarting in 5s", "exitCode", exitCode, "error", err)
		w.process = nil

		time.AfterFunc(5*time.Second, func() {
			w.mu.Lock()
			defer w.mu.Unlock()
			if w.shuttingDown || w.currentConfig == nil {
				return
			}
			w.spawnServe()
		})
	}()
}

func (w *GmailWatcher) renewLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.mu.Lock()
			cfg := w.currentConfig
			shuttingDown := w.shuttingDown
			w.mu.Unlock()

			if shuttingDown || cfg == nil {
				return
			}

			if err := StartGmailWatch(cfg.Account, cfg.Label, cfg.Topic, 120_000); err != nil {
				w.logger.Warn("gmail watch renewal failed", "error", err)
			}
		}
	}
}

// Stop 停止 Gmail watcher 服务
// 对应 TS: gmail-watcher.ts stopGmailWatcher
func (w *GmailWatcher) Stop() {
	w.mu.Lock()
	w.shuttingDown = true

	if w.cancelRenew != nil {
		w.cancelRenew()
		w.cancelRenew = nil
	}

	proc := w.process
	w.mu.Unlock()

	if proc != nil && proc.Process != nil {
		w.logger.Info("stopping gmail watcher")
		_ = proc.Process.Signal(os.Interrupt)

		// 等待 3 秒优雅退出
		done := make(chan struct{})
		go func() {
			_ = proc.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = proc.Process.Kill()
		}
	}

	w.mu.Lock()
	w.process = nil
	w.currentConfig = nil
	w.mu.Unlock()

	w.logger.Info("gmail watcher stopped")
}

// IsRunning 检查 watcher 是否运行中
func (w *GmailWatcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.process != nil && !w.shuttingDown
}
