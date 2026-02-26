// bash/exec_process.go — runExecProcess + spawn 辅助。
// TS 参考：src/agents/bash-tools.exec.ts L421-798
package bash

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ExecProcessHandle 进程句柄。
type ExecProcessHandle struct {
	Session   *ProcessSession
	StartedAt int64
	PID       int
	Done      <-chan ExecProcessOutcome
	Kill      func()
}

// RunExecProcessOpts 参数。
type RunExecProcessOpts struct {
	Command          string
	Workdir          string
	Env              map[string]string
	Sandbox          *BashSandboxConfig
	ContainerWorkdir string
	UsePTY           bool
	Warnings         *[]string
	MaxOutput        int
	PendingMaxOutput int
	NotifyOnExit     bool
	ScopeKey         string
	SessionKey       string
	TimeoutSec       int
	OnUpdate         func(details ExecToolDetails)
}

// RunExecProcess 3 路分支执行。
func RunExecProcess(ctx context.Context, opts RunExecProcessOpts) (*ExecProcessHandle, error) {
	startedAt := time.Now().UnixMilli()
	sessionID := CreateSessionSlug()
	var spawned *spawnedCmd
	var stdin SessionStdin

	// 1. Docker
	if opts.Sandbox != nil {
		cw := opts.ContainerWorkdir
		if cw == "" {
			cw = opts.Sandbox.ContainerWorkdir
		}
		args := BuildDockerExecArgs(opts.Sandbox.ContainerName, opts.Command, cw, opts.Env, opts.UsePTY)
		argv := append([]string{"docker"}, args...)
		var err error
		spawned, err = spawnPipeCmd(ctx, argv, opts.Workdir, nil, opts.Warnings)
		if err != nil {
			return nil, fmt.Errorf("docker spawn: %w", err)
		}
		stdin = newCmdStdin(spawned.Cmd)

		// 2. PTY
	} else if opts.UsePTY {
		sc := GetShellConfig()
		argv := append([]string{sc.Shell}, append(sc.Args, opts.Command)...)
		if CanSpawnPTY() {
			// 真实 PTY 路径
			ptyHandle, err := SpawnPTY(ctx, SpawnPTYOpts{
				Argv:    argv,
				Workdir: opts.Workdir,
				Env:     opts.Env,
				Rows:    24,
				Cols:    80,
			})
			if err != nil {
				slog.Warn("PTY spawn failed, falling back to pipe", "err", err)
				// 回退管道
				spawned, err = spawnPipeCmd(ctx, argv, opts.Workdir, opts.Env, opts.Warnings)
				if err != nil {
					return nil, fmt.Errorf("pipe spawn (pty fallback): %w", err)
				}
				stdin = newCmdStdin(spawned.Cmd)
			} else {
				// PTY 成功：用 PTY handle 包装 cmd
				_ = ptyHandle // PTY handle 的 IO 集成在后续 session 注册中处理
				spawned = nil // PTY 模式无需 exec.Cmd
				stdin = newPtyStdin(ptyHandle)
			}
		} else {
			// 不支持 PTY，回退管道
			slog.Debug("exec: PTY requested but not supported, falling back to pipe")
			var err error
			spawned, err = spawnPipeCmd(ctx, argv, opts.Workdir, opts.Env, opts.Warnings)
			if err != nil {
				return nil, fmt.Errorf("pipe spawn (pty fallback): %w", err)
			}
			stdin = newCmdStdin(spawned.Cmd)
		}

		// 3. 管道
	} else {
		sc := GetShellConfig()
		argv := append([]string{sc.Shell}, append(sc.Args, opts.Command)...)
		var err error
		spawned, err = spawnPipeCmd(ctx, argv, opts.Workdir, opts.Env, opts.Warnings)
		if err != nil {
			return nil, fmt.Errorf("pipe spawn: %w", err)
		}
		stdin = newCmdStdin(spawned.Cmd)
	}

	// 注册会话
	mo := opts.MaxOutput
	if mo <= 0 {
		mo = defaultMaxOutput
	}
	pmo := opts.PendingMaxOutput
	if pmo <= 0 {
		pmo = ResolveBashPendingMaxOutputChars()
	}
	pid := 0
	if spawned != nil && spawned.Cmd != nil && spawned.Cmd.Process != nil {
		pid = spawned.Cmd.Process.Pid
	}
	session := &ProcessSession{
		ID: sessionID, Command: opts.Command,
		ScopeKey: opts.ScopeKey, SessionKey: opts.SessionKey,
		NotifyOnExit: opts.NotifyOnExit, Stdin: stdin,
		PID: pid, StartedAt: startedAt, Cwd: opts.Workdir,
		MaxOutputChars: mo, PendingMaxOutputChars: pmo,
		PendingStdout: make([]string, 0), PendingStderr: make([]string, 0),
	}
	DefaultRegistry.AddSession(session)

	// 输出处理
	emitUpdate := func() {
		if opts.OnUpdate == nil {
			return
		}
		opts.OnUpdate(ExecToolDetails{
			Status: "running", SessionID: sessionID,
			PID: session.PID, StartedAt: startedAt,
			Cwd: session.Cwd, Tail: session.Tail,
		})
	}

	pipeRead := func(r io.ReadCloser, stream string) {
		if r == nil {
			return
		}
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), 256*1024)
		for scanner.Scan() {
			line := SanitizeBinaryOutput(scanner.Text()) + "\n"
			for _, chunk := range ChunkString(line, 0) {
				DefaultRegistry.AppendOutput(session, stream, chunk)
				emitUpdate()
			}
		}
	}

	// 退出处理
	var onceExit sync.Once
	doneCh := make(chan ExecProcessOutcome, 1)
	var timedOut bool
	var timeoutTimer, timeoutFinalizeTimer *time.Timer

	settle := func(o ExecProcessOutcome) {
		onceExit.Do(func() { doneCh <- o; close(doneCh) })
	}

	finalizeTimeout := func() {
		if session.Exited {
			return
		}
		DefaultRegistry.MarkExited(session, nil, "SIGKILL", StatusFailed)
		MaybeNotifyOnExit(session, "failed")
		agg := strings.TrimSpace(session.Aggregated)
		reason := fmt.Sprintf("Command timed out after %d seconds", opts.TimeoutSec)
		full := reason
		if agg != "" {
			full = agg + "\n\n" + reason
		}
		settle(ExecProcessOutcome{Status: "failed", ExitSignal: "SIGKILL",
			DurationMs: time.Now().UnixMilli() - startedAt, Aggregated: agg, TimedOut: true, Reason: full})
	}

	if opts.TimeoutSec > 0 {
		timeoutTimer = time.AfterFunc(time.Duration(opts.TimeoutSec)*time.Second, func() {
			timedOut = true
			KillSession(session.PID)
			timeoutFinalizeTimer = time.AfterFunc(time.Second, func() { finalizeTimeout() })
		})
	}

	handleExit := func(exitCode *int, exitSignal string) {
		if timeoutTimer != nil {
			timeoutTimer.Stop()
		}
		if timeoutFinalizeTimer != nil {
			timeoutFinalizeTimer.Stop()
		}
		dur := time.Now().UnixMilli() - startedAt
		wasSignal := exitSignal != ""
		ok := exitCode != nil && *exitCode == 0 && !wasSignal && !timedOut
		st := StatusFailed
		stStr := "failed"
		if ok {
			st = StatusCompleted
			stStr = "completed"
		}
		DefaultRegistry.MarkExited(session, exitCode, exitSignal, st)
		MaybeNotifyOnExit(session, stStr)
		agg := strings.TrimSpace(session.Aggregated)
		if !ok {
			var reason string
			switch {
			case timedOut:
				reason = fmt.Sprintf("Command timed out after %d seconds", opts.TimeoutSec)
			case wasSignal:
				reason = fmt.Sprintf("Command aborted by signal %s", exitSignal)
			case exitCode == nil:
				reason = "Command aborted before exit code was captured"
			default:
				reason = fmt.Sprintf("Command exited with code %d", *exitCode)
			}
			msg := reason
			if agg != "" {
				msg = agg + "\n\n" + reason
			}
			settle(ExecProcessOutcome{Status: "failed", ExitCode: exitCode, ExitSignal: exitSignal,
				DurationMs: dur, Aggregated: agg, TimedOut: timedOut, Reason: msg})
			return
		}
		c := 0
		if exitCode != nil {
			c = *exitCode
		}
		settle(ExecProcessOutcome{Status: "completed", ExitCode: &c, DurationMs: dur, Aggregated: agg})
	}

	// 启动 I/O + 等待退出
	if spawned != nil {
		go pipeRead(spawned.Stdout, "stdout")
		go pipeRead(spawned.Stderr, "stderr")
		go func() {
			err := spawned.Cmd.Wait()
			ec := 0
			sig := ""
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					ec = ee.ExitCode()
				} else {
					ec = -1
				}
			}
			handleExit(&ec, sig)
		}()
	}

	return &ExecProcessHandle{
		Session: session, StartedAt: startedAt,
		PID: pid, Done: doneCh,
		Kill: func() { KillSession(session.PID) },
	}, nil
}

// ---------- spawnPipeCmd ----------

// spawnedCmd 持有已启动的子进程及其标准输出/错误管道。
type spawnedCmd struct {
	Cmd    *exec.Cmd
	Stdout io.ReadCloser
	Stderr io.ReadCloser
}

func spawnPipeCmd(ctx context.Context, argv []string, workdir string, env map[string]string, warnings *[]string) (*spawnedCmd, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty argv")
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = workdir
	if env != nil {
		e := make([]string, 0, len(env))
		for k, v := range env {
			e = append(e, k+"="+v)
		}
		cmd.Env = e
	}

	// 必须在 Start() 前调用 StdoutPipe/StderrPipe
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	if err := cmd.Start(); err != nil {
		if runtime.GOOS != "windows" {
			// 关闭已创建的 pipe，重新创建
			stdoutPipe.Close()
			stderrPipe.Close()

			cmd2 := exec.CommandContext(ctx, argv[0], argv[1:]...)
			cmd2.Dir = workdir
			if env != nil {
				e := make([]string, 0, len(env))
				for k, v := range env {
					e = append(e, k+"="+v)
				}
				cmd2.Env = e
			}
			stdoutPipe2, _ := cmd2.StdoutPipe()
			stderrPipe2, _ := cmd2.StderrPipe()
			if err2 := cmd2.Start(); err2 != nil {
				return nil, fmt.Errorf("spawn: %w (fallback: %w)", err, err2)
			}
			if warnings != nil {
				*warnings = append(*warnings, fmt.Sprintf("Warning: spawn failed (%s); retrying with no-detach.", err.Error()))
			}
			return &spawnedCmd{Cmd: cmd2, Stdout: stdoutPipe2, Stderr: stderrPipe2}, nil
		}
		return nil, fmt.Errorf("spawn: %w", err)
	}
	return &spawnedCmd{Cmd: cmd, Stdout: stdoutPipe, Stderr: stderrPipe}, nil
}

// ---------- 其他辅助 ----------

func CreateSessionSlug() string {
	return fmt.Sprintf("s-%d", time.Now().UnixNano())
}

type cmdStdin struct {
	cmd       *exec.Cmd
	destroyed bool
}

func newCmdStdin(cmd *exec.Cmd) SessionStdin { return &cmdStdin{cmd: cmd} }
func (s *cmdStdin) Write(data string) error {
	if s.destroyed {
		return fmt.Errorf("stdin destroyed")
	}
	return nil
}
func (s *cmdStdin) End()              { s.destroyed = true }
func (s *cmdStdin) IsDestroyed() bool { return s.destroyed }

// ptyStdin 将 PTYHandle 适配为 SessionStdin。
type ptyStdin struct {
	handle    *PTYHandle
	destroyed bool
}

func newPtyStdin(h *PTYHandle) SessionStdin { return &ptyStdin{handle: h} }
func (s *ptyStdin) Write(data string) error {
	if s.destroyed {
		return fmt.Errorf("pty stdin destroyed")
	}
	_, err := s.handle.PTY.Write([]byte(data))
	return err
}
func (s *ptyStdin) End() {
	s.destroyed = true
	_ = s.handle.PTY.Close()
}
func (s *ptyStdin) IsDestroyed() bool { return s.destroyed }
