package daemon

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// AuditGatewayServiceConfig 审计 gateway 服务配置
// 检查 launchd/systemd 配置文件和运行时路径
// 对应 TS: service-audit.ts auditGatewayServiceConfig
func AuditGatewayServiceConfig(env map[string]string, command *GatewayServiceCommand, platform string) ServiceConfigAudit {
	if platform == "" {
		platform = runtime.GOOS
	}

	var issues []ServiceConfigIssue

	// 审计 gateway 子命令
	auditGatewayCommand(command, &issues)

	// 审计 PATH
	auditGatewayServicePath(command, &issues, env, platform)

	// 审计运行时
	auditGatewayRuntime(command, &issues)

	// 平台特定审计
	switch platform {
	case "linux":
		auditSystemdUnit(env, &issues)
	case "darwin":
		auditLaunchdPlist(env, &issues)
	}

	return ServiceConfigAudit{
		OK:     len(issues) == 0,
		Issues: issues,
	}
}

// auditGatewayCommand 检查服务命令是否包含 gateway 子命令
func auditGatewayCommand(command *GatewayServiceCommand, issues *[]ServiceConfigIssue) {
	if command == nil || len(command.ProgramArguments) == 0 {
		return
	}
	if !HasGatewaySubcommand(command.ProgramArguments) {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeGatewayCommandMissing,
			Message: "Service command does not include the gateway subcommand",
			Level:   "aggressive",
		})
	}
}

// auditGatewayServicePath 检查服务 PATH 是否最小化
func auditGatewayServicePath(command *GatewayServiceCommand, issues *[]ServiceConfigIssue, env map[string]string, platform string) {
	if platform == "windows" {
		return
	}
	if command == nil || command.Environment == nil {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeGatewayPathMissing,
			Message: "Gateway service PATH is not set; the daemon should use a minimal PATH.",
			Level:   "recommended",
		})
		return
	}
	servicePath, ok := command.Environment["PATH"]
	if !ok || servicePath == "" {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeGatewayPathMissing,
			Message: "Gateway service PATH is not set; the daemon should use a minimal PATH.",
			Level:   "recommended",
		})
		return
	}

	expected := GetMinimalServicePathPartsFromEnv(MinimalServicePathOptions{
		Platform: platform,
		Env:      env,
	})
	parts := splitPath(servicePath)
	normalizedParts := make(map[string]bool)
	for _, p := range parts {
		normalizedParts[normalizePathEntry(p, platform)] = true
	}

	var missing []string
	for _, e := range expected {
		if !normalizedParts[normalizePathEntry(e, platform)] {
			missing = append(missing, e)
		}
	}
	if len(missing) > 0 {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeGatewayPathMissingDirs,
			Message: "Gateway service PATH missing required dirs: " + strings.Join(missing, ", "),
			Level:   "recommended",
		})
	}

	// 检查版本管理器路径
	normalizedExpected := make(map[string]bool)
	for _, e := range expected {
		normalizedExpected[normalizePathEntry(e, platform)] = true
	}
	versionManagerPatterns := []string{
		"/.nvm/", "/.fnm/", "/.volta/", "/.asdf/", "/.n/",
		"/.nodenv/", "/.nodebrew/", "/nvs/", "/.local/share/pnpm/", "/pnpm/",
	}
	var nonMinimal []string
	for _, p := range parts {
		n := normalizePathEntry(p, platform)
		if normalizedExpected[n] {
			continue
		}
		for _, pattern := range versionManagerPatterns {
			if strings.Contains(n, pattern) || strings.HasSuffix(n, "/pnpm") {
				nonMinimal = append(nonMinimal, p)
				break
			}
		}
	}
	if len(nonMinimal) > 0 {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeGatewayPathNonMinimal,
			Message: "Gateway service PATH includes version managers or package managers; recommend a minimal PATH.",
			Detail:  strings.Join(nonMinimal, ", "),
			Level:   "recommended",
		})
	}
}

// auditGatewayRuntime 检查运行时是否使用 Bun 或版本管理器 Node
func auditGatewayRuntime(command *GatewayServiceCommand, issues *[]ServiceConfigIssue) {
	if command == nil || len(command.ProgramArguments) == 0 {
		return
	}
	execPath := command.ProgramArguments[0]

	if IsBunRuntime(execPath) {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeGatewayRuntimeBun,
			Message: "Gateway service uses Bun; Bun is incompatible with WhatsApp + Telegram channels.",
			Detail:  execPath,
			Level:   "recommended",
		})
		return
	}

	if !IsNodeRuntime(execPath) {
		return
	}

	if IsVersionManagedNodePath(execPath, runtime.GOOS) {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeGatewayRuntimeNodeVersionManager,
			Message: "Gateway service uses Node from a version manager; it can break after upgrades.",
			Detail:  execPath,
			Level:   "recommended",
		})
	}
}

// auditSystemdUnit 审计 systemd unit 文件
func auditSystemdUnit(env map[string]string, issues *[]ServiceConfigIssue) {
	home, err := ResolveHomeDir(env)
	if err != nil {
		return
	}
	serviceName := ResolveGatewaySystemdServiceName(env["OPENACOSMI_PROFILE"])
	unitPath := filepath.Join(home, ".config", "systemd", "user", serviceName+".service")

	data, err := os.ReadFile(unitPath)
	if err != nil {
		return
	}
	content := string(data)

	parsed := parseSystemdUnitForAudit(content)
	if !parsed.after["network-online.target"] {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeSystemdAfterNetworkOnline,
			Message: "Missing systemd After=network-online.target",
			Detail:  unitPath,
			Level:   "recommended",
		})
	}
	if !parsed.wants["network-online.target"] {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeSystemdWantsNetworkOnline,
			Message: "Missing systemd Wants=network-online.target",
			Detail:  unitPath,
			Level:   "recommended",
		})
	}
	if !isRestartSecPreferred(parsed.restartSec) {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeSystemdRestartSec,
			Message: "RestartSec does not match the recommended 5s",
			Detail:  unitPath,
			Level:   "recommended",
		})
	}
}

// auditLaunchdPlist 审计 launchd plist 文件
func auditLaunchdPlist(env map[string]string, issues *[]ServiceConfigIssue) {
	home, err := ResolveHomeDir(env)
	if err != nil {
		return
	}
	label := ResolveGatewayLaunchAgentLabel(env["OPENACOSMI_PROFILE"])
	plistPath := filepath.Join(home, "Library", "LaunchAgents", label+".plist")

	data, err := os.ReadFile(plistPath)
	if err != nil {
		return
	}
	content := string(data)

	runAtLoadRe := regexp.MustCompile(`(?i)<key>RunAtLoad</key>\s*<true\s*/>`)
	keepAliveRe := regexp.MustCompile(`(?i)<key>KeepAlive</key>\s*<true\s*/>`)

	if !runAtLoadRe.MatchString(content) {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeLaunchdRunAtLoad,
			Message: "LaunchAgent is missing RunAtLoad=true",
			Detail:  plistPath,
			Level:   "recommended",
		})
	}
	if !keepAliveRe.MatchString(content) {
		*issues = append(*issues, ServiceConfigIssue{
			Code:    AuditCodeLaunchdKeepAlive,
			Message: "LaunchAgent is missing KeepAlive=true",
			Detail:  plistPath,
			Level:   "recommended",
		})
	}
}

type systemdUnitParsed struct {
	after      map[string]bool
	wants      map[string]bool
	restartSec string
}

func parseSystemdUnitForAudit(content string) systemdUnitParsed {
	result := systemdUnitParsed{
		after: make(map[string]bool),
		wants: make(map[string]bool),
	}
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if value == "" {
			continue
		}
		switch key {
		case "After":
			for _, entry := range strings.Fields(value) {
				result.after[entry] = true
			}
		case "Wants":
			for _, entry := range strings.Fields(value) {
				result.wants[entry] = true
			}
		case "RestartSec":
			result.restartSec = value
		}
	}
	return result
}

func isRestartSecPreferred(value string) bool {
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return false
	}
	return math.Abs(parsed-5) < 0.01
}

func splitPath(p string) []string {
	parts := strings.Split(p, ":")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func normalizePathEntry(entry, platform string) string {
	normalized := filepath.Clean(entry)
	normalized = strings.ReplaceAll(normalized, "\\", "/")
	if platform == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}
