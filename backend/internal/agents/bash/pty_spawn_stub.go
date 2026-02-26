//go:build !pty_enabled

// bash/pty_spawn_stub.go — PTY 占位实现（无 pty_enabled tag 时编译）。
// 当不支持 PTY 时，所有 PTY 操作回退到管道。
package bash

import (
	"context"
	"fmt"
)

// CanSpawnPTY 报告是否支持 PTY spawn。
// stub 版本始终返回 false。
func CanSpawnPTY() bool { return false }

// SpawnPTY 在 stub 构建中不可用，返回错误。
func SpawnPTY(_ context.Context, _ SpawnPTYOpts) (*PTYHandle, error) {
	return nil, fmt.Errorf("PTY not available: build without pty_enabled tag")
}
