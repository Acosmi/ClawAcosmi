package daemon

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ResolveGatewayProgramArguments 解析 gateway 服务的启动参数
// Go 版简化：使用 os.Executable() 获取当前二进制位置，不需要 bun/node 运行时检测
// 对应 TS: program-args.ts resolveGatewayProgramArguments
func ResolveGatewayProgramArguments(port int) (GatewayProgramArgs, error) {
	execPath, err := os.Executable()
	if err != nil {
		return GatewayProgramArgs{}, fmt.Errorf("无法解析可执行文件路径: %w", err)
	}

	gatewayArgs := []string{execPath, "gateway", "--port", strconv.Itoa(port)}
	return GatewayProgramArgs{
		ProgramArguments: gatewayArgs,
	}, nil
}

// ResolveNodeProgramArguments 解析 node 服务的启动参数
// 对应 TS: program-args.ts resolveNodeProgramArguments
func ResolveNodeProgramArguments(params NodeProgramParams) (GatewayProgramArgs, error) {
	execPath, err := os.Executable()
	if err != nil {
		return GatewayProgramArgs{}, fmt.Errorf("无法解析可执行文件路径: %w", err)
	}

	args := []string{execPath, "node", "run", "--host", params.Host, "--port", strconv.Itoa(params.Port)}
	if params.TLS || params.TLSFingerprint != "" {
		args = append(args, "--tls")
	}
	if params.TLSFingerprint != "" {
		args = append(args, "--tls-fingerprint", params.TLSFingerprint)
	}
	if params.NodeID != "" {
		args = append(args, "--node-id", params.NodeID)
	}
	if params.DisplayName != "" {
		args = append(args, "--display-name", params.DisplayName)
	}

	return GatewayProgramArgs{
		ProgramArguments: args,
	}, nil
}

// NodeProgramParams 是 ResolveNodeProgramArguments 的参数
type NodeProgramParams struct {
	Host           string
	Port           int
	TLS            bool
	TLSFingerprint string
	NodeID         string
	DisplayName    string
}

// WithNodeServiceEnv 为环境变量注入 Node 服务特有的标识
// 对应 TS: node-service.ts withNodeServiceEnv
func WithNodeServiceEnv(env map[string]string) map[string]string {
	result := make(map[string]string, len(env)+7)
	for k, v := range env {
		result[k] = v
	}
	result["OPENACOSMI_LAUNCHD_LABEL"] = ResolveNodeLaunchAgentLabel()
	result["OPENACOSMI_SYSTEMD_UNIT"] = ResolveNodeSystemdServiceName()
	result["OPENACOSMI_WINDOWS_TASK_NAME"] = ResolveNodeWindowsTaskName()
	result["OPENACOSMI_TASK_SCRIPT_NAME"] = NodeWindowsTaskScriptName
	result["OPENACOSMI_LOG_PREFIX"] = "node"
	result["OPENACOSMI_SERVICE_MARKER"] = NodeServiceMarker
	result["OPENACOSMI_SERVICE_KIND"] = NodeServiceKind
	return result
}

// HasGatewaySubcommand 检查参数列表中是否包含 "gateway" 子命令
// 对应 TS: service-audit.ts hasGatewaySubcommand
func HasGatewaySubcommand(programArguments []string) bool {
	for _, arg := range programArguments {
		if arg == "gateway" {
			return true
		}
	}
	return false
}

// IsNodeRuntime 检查是否为 Node 运行时路径
// 对应 TS: service-audit.ts isNodeRuntime
func IsNodeRuntime(execPath string) bool {
	base := strings.ToLower(baseNameFromPath(execPath))
	return base == "node" || base == "node.exe"
}

// IsBunRuntime 检查是否为 Bun 运行时路径
// 对应 TS: service-audit.ts isBunRuntime
func IsBunRuntime(execPath string) bool {
	base := strings.ToLower(baseNameFromPath(execPath))
	return base == "bun" || base == "bun.exe"
}

// baseNameFromPath 从路径中提取文件名
func baseNameFromPath(p string) string {
	// 处理 / 和 \ 两种分隔符
	idx := strings.LastIndexAny(p, "/\\")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}
