package sandbox

// docker_task_executor.go — DockerRunner → TaskExecutor 适配器
// 将 DockerRunner 适配为 Worker 所需的 TaskExecutor 接口

import (
	"context"
	"fmt"
	"strings"
)

// DockerTaskExecutor 将 Docker 容器池执行适配为 TaskExecutor 接口。
type DockerTaskExecutor struct {
	pool *ContainerPool
}

// NewDockerTaskExecutor 创建 Docker 任务执行器。
func NewDockerTaskExecutor(pool *ContainerPool) *DockerTaskExecutor {
	return &DockerTaskExecutor{pool: pool}
}

// Execute 实现 TaskExecutor 接口。
func (e *DockerTaskExecutor) Execute(ctx context.Context, task *SandboxTask, progressFn ProgressFunc) (*ExecutionOutput, error) {
	if e.pool == nil {
		// 无容器池时 fallback 到临时容器
		runner := NewDockerRunner(DefaultDockerConfig())
		command := []string{"sh", "-c", task.Input}
		result, err := runner.Execute(ctx, "", command, "")
		if err != nil {
			return nil, fmt.Errorf("docker execute: %w", err)
		}
		output := result.Stdout
		if result.Stderr != "" {
			output += "\n[stderr] " + result.Stderr
		}
		return &ExecutionOutput{Output: output}, nil
	}

	// 使用容器池
	progressFn(10, "acquiring container from pool")
	container, err := e.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire container: %w", err)
	}
	defer e.pool.Release(container.ID)

	progressFn(30, "executing in container")
	runner := NewDockerRunner(DefaultDockerConfig())
	command := parseCommand(task.Input)
	result, err := runner.Execute(ctx, "", command, "")
	if err != nil {
		e.pool.Destroy(container.ID)
		return nil, fmt.Errorf("execute in container: %w", err)
	}

	progressFn(90, "execution complete")
	output := result.Stdout
	if result.Stderr != "" {
		output += "\n[stderr] " + result.Stderr
	}
	return &ExecutionOutput{Output: output}, nil
}

// parseCommand 将输入解析为命令数组。
func parseCommand(input string) []string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return []string{"echo", "empty input"}
	}
	return []string{"sh", "-c", trimmed}
}
