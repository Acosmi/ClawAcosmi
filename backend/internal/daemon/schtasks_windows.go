//go:build windows

package daemon

import (
	"fmt"
	"os/exec"
	"strings"
)

// schtasksService 实现 Windows 计划任务管理
type schtasksService struct{}

func (s *schtasksService) Label() string         { return "Scheduled Task" }
func (s *schtasksService) LoadedText() string    { return "registered" }
func (s *schtasksService) NotLoadedText() string { return "missing" }

// Install 安装 Windows 计划任务
// 对应 TS: schtasks.ts installScheduledTask
func (s *schtasksService) Install(args GatewayServiceInstallArgs) error {
	taskName := resolveInstallWindowsTaskName(args.Env)
	programPath := ""
	programArgs := ""
	if len(args.ProgramArguments) > 0 {
		programPath = args.ProgramArguments[0]
		if len(args.ProgramArguments) > 1 {
			programArgs = strings.Join(args.ProgramArguments[1:], " ")
		}
	}

	schtasksArgs := []string{
		"/Create",
		"/TN", taskName,
		"/SC", "ONLOGON",
		"/TR", fmt.Sprintf(`"%s" %s`, programPath, programArgs),
		"/F",
	}

	cmd := exec.Command("schtasks", schtasksArgs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks /Create 失败: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// 立即运行
	runCmd := exec.Command("schtasks", "/Run", "/TN", taskName)
	_ = runCmd.Run()

	return nil
}

// Uninstall 卸载 Windows 计划任务
// 对应 TS: schtasks.ts uninstallScheduledTask
func (s *schtasksService) Uninstall(env map[string]string) error {
	taskName := resolveInstallWindowsTaskName(env)
	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	_ = cmd.Run()
	return nil
}

// Stop 停止 Windows 计划任务
// 对应 TS: schtasks.ts stopScheduledTask
func (s *schtasksService) Stop(env map[string]string) error {
	taskName := resolveInstallWindowsTaskName(env)
	cmd := exec.Command("schtasks", "/End", "/TN", taskName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks /End 失败: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Restart 重启 Windows 计划任务
// 对应 TS: schtasks.ts restartScheduledTask
func (s *schtasksService) Restart(env map[string]string) error {
	taskName := resolveInstallWindowsTaskName(env)

	// 先停止
	stopCmd := exec.Command("schtasks", "/End", "/TN", taskName)
	_ = stopCmd.Run()

	// 再运行
	runCmd := exec.Command("schtasks", "/Run", "/TN", taskName)
	if out, err := runCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks /Run 失败: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// IsLoaded 检查 Windows 计划任务是否已注册
// 对应 TS: schtasks.ts isScheduledTaskInstalled
func (s *schtasksService) IsLoaded(env map[string]string) (bool, error) {
	taskName := resolveInstallWindowsTaskName(env)
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName)
	err := cmd.Run()
	return err == nil, nil
}

// ReadCommand 读取计划任务的命令配置
// 对应 TS: schtasks.ts readScheduledTaskCommand
func (s *schtasksService) ReadCommand(env map[string]string) (*GatewayServiceCommand, error) {
	taskName := resolveInstallWindowsTaskName(env)
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName, "/FO", "LIST", "/V")
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	kv := ParseKeyValueOutput(string(out), ":")
	taskToRun, ok := kv["task to run"]
	if !ok || taskToRun == "" {
		return nil, nil
	}

	args := strings.Fields(taskToRun)
	return &GatewayServiceCommand{
		ProgramArguments: args,
	}, nil
}

// ReadRuntime 读取计划任务运行时状态
// 对应 TS: schtasks.ts readScheduledTaskRuntime
func (s *schtasksService) ReadRuntime(env map[string]string) (GatewayServiceRuntime, error) {
	taskName := resolveInstallWindowsTaskName(env)
	cmd := exec.Command("schtasks", "/Query", "/TN", taskName, "/FO", "LIST", "/V")
	out, err := cmd.Output()
	if err != nil {
		return GatewayServiceRuntime{
			Status:      "stopped",
			MissingUnit: true,
		}, nil
	}

	kv := ParseKeyValueOutput(string(out), ":")
	rt := GatewayServiceRuntime{}

	if status, ok := kv["status"]; ok {
		switch strings.ToLower(status) {
		case "running":
			rt.Status = "running"
		case "ready":
			rt.Status = "stopped"
		default:
			rt.Status = "unknown"
		}
		rt.State = status
	}
	if result, ok := kv["last result"]; ok {
		rt.LastRunResult = result
	}
	if runTime, ok := kv["last run time"]; ok {
		rt.LastRunTime = runTime
	}

	return rt, nil
}

// resolveInstallWindowsTaskName 解析安装时使用的 Windows 任务名
func resolveInstallWindowsTaskName(env map[string]string) string {
	if name, ok := env["OPENACOSMI_WINDOWS_TASK_NAME"]; ok && name != "" {
		return name
	}
	return ResolveGatewayWindowsTaskName(env["OPENACOSMI_PROFILE"])
}

// newSchtasksService 创建 Windows 计划任务服务实例
func newSchtasksService() GatewayService {
	return &schtasksService{}
}
