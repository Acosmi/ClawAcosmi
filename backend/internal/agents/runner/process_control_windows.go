//go:build windows

package runner

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureCommandForProcessTracking(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{}
}

func trackedProcessID(pid int) (int, error) {
	return pid, nil
}

func terminateTrackedProcess(id int) error {
	proc, err := os.FindProcess(id)
	if err != nil {
		return err
	}
	err = proc.Kill()
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func forceKillTrackedProcess(id int) error {
	return terminateTrackedProcess(id)
}
