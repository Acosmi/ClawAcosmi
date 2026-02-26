//go:build linux

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// systemdService 实现 Linux systemd 服务管理
type systemdService struct{}

func (s *systemdService) Label() string         { return "systemd" }
func (s *systemdService) LoadedText() string    { return "enabled" }
func (s *systemdService) NotLoadedText() string { return "disabled" }

// Install 安装 systemd user service
// 对应 TS: systemd.ts installSystemdService
func (s *systemdService) Install(args GatewayServiceInstallArgs) error {
	unitPath := resolveSystemdUserUnitPath(args.Env)

	// 确保目录存在
	dir := filepath.Dir(unitPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建 systemd user 目录失败: %w", err)
	}

	unitContent := buildSystemdUnit(args)
	if err := os.WriteFile(unitPath, []byte(unitContent), 0o644); err != nil {
		return fmt.Errorf("写入 systemd unit 文件失败: %w", err)
	}

	// systemctl --user daemon-reload
	if err := runSystemctl("daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload 失败: %w", err)
	}

	// systemctl --user enable --now <service>
	serviceName := resolveInstallSystemdServiceName(args.Env)
	if err := runSystemctl("enable", "--now", serviceName+".service"); err != nil {
		return fmt.Errorf("systemctl enable 失败: %w", err)
	}

	return nil
}

// Uninstall 卸载 systemd user service
// 对应 TS: systemd.ts uninstallSystemdService
func (s *systemdService) Uninstall(env map[string]string) error {
	serviceName := resolveInstallSystemdServiceName(env)
	unitPath := resolveSystemdUserUnitPath(env)

	// systemctl --user disable --now
	_ = runSystemctl("disable", "--now", serviceName+".service")

	// 删除 unit 文件
	_ = os.Remove(unitPath)

	// daemon-reload
	_ = runSystemctl("daemon-reload")

	return nil
}

// Stop 停止 systemd service
// 对应 TS: systemd.ts stopSystemdService
func (s *systemdService) Stop(env map[string]string) error {
	serviceName := resolveInstallSystemdServiceName(env)
	return runSystemctl("stop", serviceName+".service")
}

// Restart 重启 systemd service
// 对应 TS: systemd.ts restartSystemdService
func (s *systemdService) Restart(env map[string]string) error {
	serviceName := resolveInstallSystemdServiceName(env)
	return runSystemctl("restart", serviceName+".service")
}

// IsLoaded 检查 systemd service 是否已启用
// 对应 TS: systemd.ts isSystemdServiceEnabled
func (s *systemdService) IsLoaded(env map[string]string) (bool, error) {
	serviceName := resolveInstallSystemdServiceName(env)
	cmd := exec.Command("systemctl", "--user", "is-enabled", serviceName+".service")
	err := cmd.Run()
	return err == nil, nil
}

// ReadCommand 读取 systemd service 的 ExecStart 配置
// 对应 TS: systemd.ts readSystemdServiceExecStart
func (s *systemdService) ReadCommand(env map[string]string) (*GatewayServiceCommand, error) {
	unitPath := resolveSystemdUserUnitPath(env)
	data, err := os.ReadFile(unitPath)
	if err != nil {
		return nil, nil
	}
	content := string(data)

	var execStart string
	var workingDir string
	envVars := make(map[string]string)

	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])

		switch key {
		case "ExecStart":
			execStart = value
		case "WorkingDirectory":
			workingDir = value
		case "Environment":
			// 格式: "KEY=VALUE" 或 KEY=VALUE，使用 parseSystemdEnvAssignment 正确处理转义
			if k, v, ok := parseSystemdEnvAssignment(value); ok {
				envVars[k] = v
			}
		}
	}

	if execStart == "" {
		return nil, nil
	}

	// 使用 parseSystemdExecStart 正确处理带引号的参数
	args := parseSystemdExecStart(execStart)
	result := &GatewayServiceCommand{
		ProgramArguments: args,
		SourcePath:       unitPath,
	}
	if workingDir != "" {
		result.WorkingDirectory = workingDir
	}
	if len(envVars) > 0 {
		result.Environment = envVars
	}
	return result, nil
}

// ReadRuntime 读取 systemd service 运行时状态
// 对应 TS: systemd.ts readSystemdServiceRuntime
func (s *systemdService) ReadRuntime(env map[string]string) (GatewayServiceRuntime, error) {
	serviceName := resolveInstallSystemdServiceName(env)

	cmd := exec.Command("systemctl", "--user", "show", serviceName+".service",
		"--property=ActiveState,SubState,MainPID,ExecMainStatus,Result,ActiveEnterTimestamp")
	out, err := cmd.Output()
	if err != nil {
		unitPath := resolveSystemdUserUnitPath(env)
		if _, statErr := os.Stat(unitPath); statErr == nil {
			return GatewayServiceRuntime{
				Status: "stopped",
				Detail: "unit exists but service not active",
			}, nil
		}
		return GatewayServiceRuntime{
			Status:      "stopped",
			MissingUnit: true,
		}, nil
	}

	kv := ParseKeyValueOutput(string(out), "=")
	rt := GatewayServiceRuntime{}

	if state, ok := kv["activestate"]; ok {
		rt.State = state
		switch state {
		case "active":
			rt.Status = "running"
		case "inactive", "failed", "deactivating":
			rt.Status = "stopped"
		default:
			rt.Status = "unknown"
		}
	}
	if sub, ok := kv["substate"]; ok {
		rt.SubState = sub
	}
	if pid, ok := kv["mainpid"]; ok {
		if p, err := fmt.Sscanf(pid, "%d", &rt.PID); p > 0 && err == nil && rt.PID > 0 {
			// PID 有效
		} else {
			rt.PID = 0
		}
	}
	if status, ok := kv["execmainstatus"]; ok {
		if code, err := fmt.Sscanf(status, "%d", &rt.LastExitStatus); code > 0 && err == nil {
			// 退出码有效
		}
	}
	if result, ok := kv["result"]; ok {
		rt.LastRunResult = result
	}
	if ts, ok := kv["activeentertimestamp"]; ok {
		rt.LastRunTime = ts
	}

	return rt, nil
}

// resolveSystemdUserUnitPath 解析 systemd user unit 文件路径
// 对应 TS: systemd.ts resolveSystemdUserUnitPath
func resolveSystemdUserUnitPath(env map[string]string) string {
	home, err := ResolveHomeDir(env)
	if err != nil {
		home, _ = os.UserHomeDir()
	}
	serviceName := ResolveGatewaySystemdServiceName(env["OPENACOSMI_PROFILE"])
	return filepath.Join(home, ".config", "systemd", "user", serviceName+".service")
}

// ResolveSystemdUserUnitPath 导出版本
func ResolveSystemdUserUnitPath(env map[string]string) string {
	return resolveSystemdUserUnitPath(env)
}

// resolveInstallSystemdServiceName 解析安装时使用的 systemd 服务名
func resolveInstallSystemdServiceName(env map[string]string) string {
	if unit, ok := env["OPENACOSMI_SYSTEMD_UNIT"]; ok && unit != "" {
		// 移除 .service 后缀
		return strings.TrimSuffix(unit, ".service")
	}
	return ResolveGatewaySystemdServiceName(env["OPENACOSMI_PROFILE"])
}

// buildSystemdUnit 构建 systemd unit 文件内容
// 对应 TS: systemd-unit.ts buildSystemdUnit
func buildSystemdUnit(args GatewayServiceInstallArgs) string {
	return buildSystemdUnitContent(SystemdUnitArgs{
		Description:      args.Description,
		ProgramArguments: args.ProgramArguments,
		WorkingDirectory: args.WorkingDirectory,
		Environment:      args.Environment,
	})
}

// runSystemctl 执行 systemctl --user 命令
func runSystemctl(args ...string) error {
	cmdArgs := append([]string{"--user"}, args...)
	cmd := exec.Command("systemctl", cmdArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl %s: %s: %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// newSystemdService 创建 systemd 服务实例
func newSystemdService() GatewayService {
	return &systemdService{}
}
