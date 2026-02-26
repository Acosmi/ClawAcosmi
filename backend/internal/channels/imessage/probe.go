//go:build darwin

package imessage

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// iMessage CLI 可用性探测 — 继承自 src/imessage/probe.ts (107L)

// IMessageProbe 探测结果
type IMessageProbe struct {
	OK    bool
	Error string
	Fatal bool
}

// IMessageProbeOptions 探测选项
type IMessageProbeOptions struct {
	CliPath string
	DbPath  string
	Config  *types.OpenAcosmiConfig // G4: 支持 config 级 probeTimeoutMs 回退
}

// rpcSupportResult RPC 支持检测结果
type rpcSupportResult struct {
	supported bool
	err       string
	fatal     bool
}

// rpcSupportCache 全局缓存：cliPath → rpcSupportResult
var (
	rpcSupportCache   = make(map[string]*rpcSupportResult)
	rpcSupportCacheMu sync.RWMutex
)

// probeRpcSupport 检测 imsg CLI 是否支持 rpc 子命令
func probeRpcSupport(ctx context.Context, cliPath string, timeoutMs int) *rpcSupportResult {
	rpcSupportCacheMu.RLock()
	cached, ok := rpcSupportCache[cliPath]
	rpcSupportCacheMu.RUnlock()
	if ok {
		return cached
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	if timeout <= 0 {
		timeout = time.Duration(DefaultProbeTimeoutMs) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cliPath, "rpc", "--help")
	output, err := cmd.CombinedOutput()
	combined := strings.TrimSpace(string(output))
	normalized := strings.ToLower(combined)

	if err != nil {
		// 检查是否是 "unknown command" 错误
		if strings.Contains(normalized, "unknown command") && strings.Contains(normalized, "rpc") {
			result := &rpcSupportResult{
				supported: false,
				fatal:     true,
				err:       `imsg CLI does not support the "rpc" subcommand (update imsg)`,
			}
			rpcSupportCacheMu.Lock()
			rpcSupportCache[cliPath] = result
			rpcSupportCacheMu.Unlock()
			return result
		}
		return &rpcSupportResult{
			supported: false,
			err:       fmt.Sprintf("imsg rpc --help failed: %s", combined),
		}
	}

	// exit code 0 → 支持
	result := &rpcSupportResult{supported: true}
	rpcSupportCacheMu.Lock()
	rpcSupportCache[cliPath] = result
	rpcSupportCacheMu.Unlock()
	return result
}

// DetectBinaryExists 检测二进制文件是否在 PATH 中
func DetectBinaryExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ProbeIMessage 探测 iMessage RPC 可达性
func ProbeIMessage(ctx context.Context, timeoutMs int, opts IMessageProbeOptions) IMessageProbe {
	cliPath := strings.TrimSpace(opts.CliPath)
	if cliPath == "" {
		cliPath = "imsg"
	}
	dbPath := strings.TrimSpace(opts.DbPath)

	effectiveTimeout := timeoutMs
	if effectiveTimeout <= 0 {
		// G4: 先检查 config.channels.imessage.probeTimeoutMs
		if opts.Config != nil && opts.Config.Channels != nil &&
			opts.Config.Channels.IMessage != nil &&
			opts.Config.Channels.IMessage.ProbeTimeoutMs != nil {
			effectiveTimeout = *opts.Config.Channels.IMessage.ProbeTimeoutMs
		}
	}
	if effectiveTimeout <= 0 {
		effectiveTimeout = DefaultProbeTimeoutMs
	}

	// 检测 CLI 是否存在
	if !DetectBinaryExists(cliPath) {
		return IMessageProbe{OK: false, Error: fmt.Sprintf("imsg not found (%s)", cliPath)}
	}

	// 检测 RPC 子命令支持
	rpcSupport := probeRpcSupport(ctx, cliPath, effectiveTimeout)
	if !rpcSupport.supported {
		errMsg := rpcSupport.err
		if errMsg == "" {
			errMsg = "imsg rpc unavailable"
		}
		return IMessageProbe{OK: false, Error: errMsg, Fatal: rpcSupport.fatal}
	}

	// 尝试 RPC 连接并发送 chats.list
	client, err := CreateIMessageRpcClient(ctx, IMessageRpcClientOptions{
		CliPath: cliPath,
		DbPath:  dbPath,
	})
	if err != nil {
		return IMessageProbe{OK: false, Error: fmt.Sprintf("imsg rpc start: %s", err)}
	}
	defer client.Stop()

	_, err = client.Request(ctx, "chats.list", map[string]interface{}{"limit": 1}, effectiveTimeout)
	if err != nil {
		return IMessageProbe{OK: false, Error: fmt.Sprintf("%s", err)}
	}
	return IMessageProbe{OK: true}
}
