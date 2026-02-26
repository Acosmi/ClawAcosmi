// docker.go — Docker CLI 交互。
//
// TS 对照: agents/sandbox/docker.ts (352L)
//
// 通过 os/exec 与 Docker CLI 交互，包括镜像检查、
// 容器创建、启动、状态查询和标签读取。
package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ---------- Docker 命令执行 ----------

// ExecDocker 执行 Docker CLI 命令。
// TS 对照: docker.ts execDocker()
func ExecDocker(args []string, opts *ExecDockerOpts) (*ExecDockerResult, error) {
	timeout := DefaultExecTimeout
	if opts != nil && opts.TimeoutSec > 0 {
		timeout = opts.TimeoutSec
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			if opts != nil && opts.AllowFailure {
				return &ExecDockerResult{Code: -1, Stderr: err.Error()}, nil
			}
			return nil, fmt.Errorf("failed to run docker: %w", err)
		}
	}

	result := &ExecDockerResult{
		Code:   exitCode,
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if exitCode != 0 && (opts == nil || !opts.AllowFailure) {
		return result, fmt.Errorf("docker command failed (exit %d): %s", exitCode, strings.TrimSpace(stderr.String()))
	}

	return result, nil
}

// ---------- 镜像操作 ----------

// DockerImageExists 检查 Docker 镜像是否存在。
// TS 对照: docker.ts dockerImageExists()
func DockerImageExists(image string) (bool, error) {
	result, err := ExecDocker([]string{"image", "inspect", image}, &ExecDockerOpts{AllowFailure: true})
	if err != nil {
		return false, err
	}
	if result.Code == 0 {
		return true, nil
	}
	stderr := strings.TrimSpace(result.Stderr)
	if strings.Contains(stderr, "No such image") {
		return false, nil
	}
	return false, fmt.Errorf("failed to inspect sandbox image: %s", stderr)
}

// EnsureSandboxImage 确保沙箱镜像存在，不存在则拉取。
// TS 对照: docker.ts ensureSandboxImage()
func EnsureSandboxImage(image string) error {
	exists, err := DockerImageExists(image)
	if err != nil {
		return fmt.Errorf("checking sandbox image: %w", err)
	}
	if exists {
		return nil
	}

	_, err = ExecDocker([]string{"pull", image}, &ExecDockerOpts{
		TimeoutSec: 300, // 拉取镜像超时 5 分钟
	})
	if err != nil {
		return fmt.Errorf("pulling sandbox image %s: %w", image, err)
	}
	return nil
}

// ---------- 容器状态 ----------

// DockerContainerState 查询容器状态。
// TS 对照: docker.ts dockerContainerState()
func DockerContainerState(name string) ContainerState {
	result, err := ExecDocker(
		[]string{"inspect", "-f", "{{.State.Status}}", name},
		&ExecDockerOpts{AllowFailure: true},
	)
	if err != nil || result.Code != 0 {
		return ContainerState{Exists: false}
	}

	status := strings.TrimSpace(result.Stdout)
	return ContainerState{
		Exists:  true,
		Running: status == "running",
		Status:  status,
	}
}

// ---------- 容器创建 ----------

// BuildCreateArgs 构建 docker create 参数列表。
// TS 对照: docker.ts buildCreateArgs()
func BuildCreateArgs(cfg SandboxConfig, containerName, sessionKey, configHash string) []string {
	args := []string{"create", "--name", containerName}

	// 工作目录
	workdir := cfg.Docker.Workdir
	if workdir == "" {
		workdir = DefaultWorkdir
	}
	args = append(args, "-w", workdir)

	// 网络
	if cfg.Docker.Network != "" {
		args = append(args, "--network", cfg.Docker.Network)
	} else {
		args = append(args, "--network", "none")
	}

	// 用户
	if cfg.Docker.User != "" {
		args = append(args, "--user", cfg.Docker.User)
	}

	// 只读根文件系统
	if cfg.Docker.ReadOnlyRoot {
		args = append(args, "--read-only")
	}

	// tmpfs
	for _, t := range cfg.Docker.Tmpfs {
		args = append(args, "--tmpfs", t)
	}

	// capabilities
	args = append(args, "--cap-drop", "ALL")
	for _, cap := range cfg.Docker.Capabilities {
		args = append(args, "--cap-add", cap)
	}

	// 资源限制
	if cfg.Docker.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", cfg.Docker.MemoryMB))
	}
	if cfg.Docker.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", cfg.Docker.CPUs))
	}

	// 安全策略
	if cfg.Docker.SeccompPolicy != "" {
		args = append(args, "--security-opt", fmt.Sprintf("seccomp=%s", cfg.Docker.SeccompPolicy))
	}
	if cfg.Docker.ApparmorProfile != "" {
		args = append(args, "--security-opt", fmt.Sprintf("apparmor=%s", cfg.Docker.ApparmorProfile))
	}

	// 环境变量
	for k, v := range cfg.Docker.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// 标签
	args = append(args, "--label", "openacosmi.sandbox=true")
	args = append(args, "--label", fmt.Sprintf("openacosmi.session-key=%s", sessionKey))
	if configHash != "" {
		args = append(args, "--label", fmt.Sprintf("openacosmi.config-hash=%s", configHash))
	}

	// 镜像
	image := cfg.Docker.Image
	if image == "" {
		image = DefaultImage
	}
	args = append(args, image)

	// 保持容器运行
	args = append(args, "sleep", "infinity")

	return args
}

// CreateAndStartContainer 创建并启动沙箱容器。
// TS 对照: docker.ts createAndStartContainer()
func CreateAndStartContainer(cfg SandboxConfig, containerName, sessionKey string) error {
	configHash := NormalizeAndHashConfig(cfg)

	args := BuildCreateArgs(cfg, containerName, sessionKey, configHash)
	_, err := ExecDocker(args, nil)
	if err != nil {
		return fmt.Errorf("creating container %s: %w", containerName, err)
	}

	_, err = ExecDocker([]string{"start", containerName}, nil)
	if err != nil {
		return fmt.Errorf("starting container %s: %w", containerName, err)
	}

	return nil
}

// ---------- 容器标签 ----------

// ReadContainerLabel 读取容器标签。
// TS 对照: docker.ts readContainerLabel()
func ReadContainerLabel(containerName, label string) (string, error) {
	result, err := ExecDocker(
		[]string{"inspect", "-f", fmt.Sprintf("{{index .Config.Labels %q}}", label), containerName},
		&ExecDockerOpts{AllowFailure: true},
	)
	if err != nil || result.Code != 0 {
		return "", fmt.Errorf("reading label %q from container %s", label, containerName)
	}
	return strings.TrimSpace(result.Stdout), nil
}

// RemoveContainer 强制删除容器。
// TS 对照: docker.ts（内联于 manage/prune 中）
func RemoveContainer(containerName string) error {
	_, err := ExecDocker([]string{"rm", "-f", containerName}, &ExecDockerOpts{AllowFailure: true})
	return err
}

// ReadContainerImage 读取容器的镜像名称。
func ReadContainerImage(containerName string) (string, error) {
	result, err := ExecDocker(
		[]string{"inspect", "-f", "{{.Config.Image}}", containerName},
		&ExecDockerOpts{AllowFailure: true},
	)
	if err != nil || result.Code != 0 {
		return "", fmt.Errorf("reading image from container %s", containerName)
	}
	return strings.TrimSpace(result.Stdout), nil
}
