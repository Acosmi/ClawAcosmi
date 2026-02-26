//go:build pty_enabled

// bash/pty_spawn.go — PTY 真实实现（需 creack/pty 依赖）。
// 当使用 go build -tags pty_enabled 时编译。
// TS 参考：src/agents/shell-utils.ts spawnPTY
package bash

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

// CanSpawnPTY 返回 true — PTY 可用。
func CanSpawnPTY() bool { return true }

// SpawnPTY 使用 creack/pty 创建 PTY 进程。
func SpawnPTY(ctx context.Context, opts SpawnPTYOpts) (*PTYHandle, error) {
	if len(opts.Argv) == 0 {
		return nil, fmt.Errorf("argv is empty")
	}

	cmd := exec.CommandContext(ctx, opts.Argv[0], opts.Argv[1:]...)
	cmd.Dir = opts.Workdir

	// 构建环境变量
	env := os.Environ()
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	// 设置进程组
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// PTY 窗口大小
	rows, cols := opts.Rows, opts.Cols
	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}
	winSize := &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}

	// 启动 PTY
	ptmx, err := pty.StartWithSize(cmd, winSize)
	if err != nil {
		return nil, fmt.Errorf("pty start: %w", err)
	}

	return &PTYHandle{
		PID: cmd.Process.Pid,
		PTY: ptmx,
		Resize: func(r, c int) error {
			return pty.Setsize(ptmx, &pty.Winsize{
				Rows: uint16(r),
				Cols: uint16(c),
			})
		},
		Wait: func() error {
			return cmd.Wait()
		},
	}, nil
}
