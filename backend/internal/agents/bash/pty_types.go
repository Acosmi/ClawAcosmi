// bash/pty_types.go — PTY 相关共享类型。
// 被 pty_spawn.go 和 pty_spawn_stub.go 共同使用。
package bash

import (
	"io"
)

// SpawnPTYOpts PTY spawn 选项。
type SpawnPTYOpts struct {
	Argv    []string
	Workdir string
	Env     map[string]string
	Rows    int
	Cols    int
}

// PTYHandle PTY 进程句柄。
type PTYHandle struct {
	PID    int
	PTY    io.ReadWriteCloser
	Resize func(rows, cols int) error
	Wait   func() error
}
