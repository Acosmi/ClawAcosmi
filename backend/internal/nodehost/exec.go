package nodehost

// exec.go — 命令执行 + 可执行文件查找
// 对应 TS: runner.ts L400-534 (runCommand + resolveExecutable + handleSystemWhich)

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// RunCommand 执行命令并捕获输出，支持超时和输出截断。
func RunCommand(argv []string, cwd string, env map[string]string, timeoutMs int) *RunResult {
	if len(argv) == 0 {
		return &RunResult{Error: "empty command"}
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if timeoutMs > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		envSlice := make([]string, 0, len(env))
		for k, v := range env {
			envSlice = append(envSlice, k+"="+v)
		}
		cmd.Env = envSlice
	}

	var stdoutBuf, stderrBuf capBuffer
	stdoutBuf.cap = OutputCap
	stderrBuf.cap = OutputCap
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	timedOut := ctx.Err() == context.DeadlineExceeded
	result := &RunResult{
		TimedOut:  timedOut,
		Stdout:    stdoutBuf.String(),
		Stderr:    stderrBuf.String(),
		Truncated: stdoutBuf.truncated || stderrBuf.truncated,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			result.ExitCode = &code
			result.Success = code == 0 && !timedOut
		} else {
			result.Error = err.Error()
			result.Success = false
		}
	} else {
		zero := 0
		result.ExitCode = &zero
		result.Success = !timedOut
	}
	return result
}

// capBuffer 实现带上限的 io.Writer。
type capBuffer struct {
	buf       bytes.Buffer
	cap       int
	total     int
	truncated bool
}

func (b *capBuffer) Write(p []byte) (int, error) {
	if b.total >= b.cap {
		b.truncated = true
		return len(p), nil
	}
	remaining := b.cap - b.total
	toWrite := p
	if len(p) > remaining {
		toWrite = p[:remaining]
		b.truncated = true
	}
	n, err := b.buf.Write(toWrite)
	b.total += len(p) // 统计实际接收的总量
	return n + (len(p) - len(toWrite)), err
}

func (b *capBuffer) String() string {
	return b.buf.String()
}

// ResolveExecutable 在 PATH 中查找可执行文件，返回完整路径。
func ResolveExecutable(bin string, env map[string]string) string {
	if strings.Contains(bin, "/") || strings.Contains(bin, `\`) {
		return ""
	}

	extensions := []string{""}
	if runtime.GOOS == "windows" {
		pathext := os.Getenv("PATHEXT")
		if pathext == "" {
			pathext = ".EXE;.CMD;.BAT;.COM"
		}
		extensions = make([]string, 0)
		for _, ext := range strings.Split(pathext, ";") {
			extensions = append(extensions, strings.ToLower(ext))
		}
	}

	for _, dir := range resolveEnvPath(env) {
		for _, ext := range extensions {
			candidate := filepath.Join(dir, bin+ext)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}

// HandleSystemWhich 查找多个可执行文件的位置。
func HandleSystemWhich(bins []string, env map[string]string) map[string]string {
	found := make(map[string]string)
	for _, bin := range bins {
		b := strings.TrimSpace(bin)
		if b == "" {
			continue
		}
		if p := ResolveExecutable(b, env); p != "" {
			found[b] = p
		}
	}
	return found
}

func resolveEnvPath(env map[string]string) []string {
	raw := ""
	if env != nil {
		if v, ok := env["PATH"]; ok {
			raw = v
		} else if v, ok := env["Path"]; ok {
			raw = v
		}
	}
	if raw == "" {
		raw = os.Getenv("PATH")
		if raw == "" {
			raw = os.Getenv("Path")
			if raw == "" {
				raw = DefaultNodePath
			}
		}
	}
	parts := strings.Split(raw, string(filepath.ListSeparator))
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
