package signal

import (
	"context"
	"fmt"
	"time"
)

// Signal 健康探测 — 继承自 src/signal/probe.ts (58L)

// SignalProbe 探测结果
type SignalProbe struct {
	OK      bool   `json:"ok"`
	Status  string `json:"status"`
	Error   string `json:"error,omitempty"`
	Version string `json:"version,omitempty"`
	Elapsed int64  `json:"elapsed"` // 毫秒
}

// ProbeSignal 探测 signal-cli daemon 健康状态并获取版本号
func ProbeSignal(ctx context.Context, baseURL string, account string) SignalProbe {
	start := time.Now()

	// 第一步：连通性检查
	checkCtx, checkCancel := context.WithTimeout(ctx, 5*time.Second)
	defer checkCancel()

	if err := SignalCheck(checkCtx, baseURL); err != nil {
		elapsed := time.Since(start).Milliseconds()
		return SignalProbe{
			OK:      false,
			Status:  "unreachable",
			Error:   fmt.Sprintf("check failed: %s", err),
			Elapsed: elapsed,
		}
	}

	// 第二步：获取版本号
	versionCtx, versionCancel := context.WithTimeout(ctx, 10*time.Second)
	defer versionCancel()

	version, err := SignalVersion(versionCtx, baseURL, account)
	elapsed := time.Since(start).Milliseconds()
	if err != nil {
		return SignalProbe{
			OK:      true,
			Status:  "connected",
			Error:   fmt.Sprintf("version query failed: %s", err),
			Elapsed: elapsed,
		}
	}

	return SignalProbe{
		OK:      true,
		Status:  "ok",
		Version: version,
		Elapsed: elapsed,
	}
}
