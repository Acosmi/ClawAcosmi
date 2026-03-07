//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

func configureCommandForProcessTracking(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func trackedProcessID(pid int) (int, error) {
	return syscall.Getpgid(pid)
}

func terminateTrackedProcess(id int) error {
	return syscall.Kill(-id, syscall.SIGTERM)
}

func forceKillTrackedProcess(id int) error {
	return syscall.Kill(-id, syscall.SIGKILL)
}
