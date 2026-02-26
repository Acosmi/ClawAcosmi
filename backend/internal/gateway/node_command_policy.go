package gateway

// node_command_policy.go — 节点命令策略
// 对应 TS: src/gateway/node-command-policy.ts (181L)
//
// 功能:
//   - 基于平台的默认命令白名单
//   - config overlay (allowCommands / denyCommands)
//   - 命令声明检查

import (
	"strings"
)

// ---------- 平台默认命令白名单 ----------

var canvasCommands = []string{
	"canvas.present", "canvas.hide", "canvas.navigate",
	"canvas.eval", "canvas.snapshot",
	"canvas.a2ui.push", "canvas.a2ui.pushJSONL", "canvas.a2ui.reset",
}

var cameraCommands = []string{"camera.list"}
var cameraDangerousCommands = []string{"camera.snap", "camera.clip"}
var screenDangerousCommands = []string{"screen.record"}
var locationCommands = []string{"location.get"}
var deviceCommands = []string{"device.info", "device.status"}
var contactsCommands = []string{"contacts.search"}
var contactsDangerousCommands = []string{"contacts.add"}
var calendarCommands = []string{"calendar.events"}
var calendarDangerousCommands = []string{"calendar.add"}
var remindersCommands = []string{"reminders.list"}
var remindersDangerousCommands = []string{"reminders.add"}
var photosCommands = []string{"photos.latest"}
var motionCommands = []string{"motion.activity", "motion.pedometer"}
var smsDangerousCommands = []string{"sms.send"}
var iosSystemCommands = []string{"system.notify"}
var systemCommands = []string{
	"system.run", "system.which", "system.notify",
	"system.execApprovals.get", "system.execApprovals.set",
	"browser.proxy",
}

// DefaultDangerousNodeCommands 高风险节点命令（需配置显式启用）。
var DefaultDangerousNodeCommands = flatten(
	cameraDangerousCommands,
	screenDangerousCommands,
	contactsDangerousCommands,
	calendarDangerousCommands,
	remindersDangerousCommands,
	smsDangerousCommands,
)

var platformDefaults = map[string][]string{
	"ios": flatten(
		canvasCommands, cameraCommands, locationCommands, deviceCommands,
		contactsCommands, calendarCommands, remindersCommands,
		photosCommands, motionCommands, iosSystemCommands,
	),
	"android": flatten(
		canvasCommands, cameraCommands, locationCommands, deviceCommands,
		contactsCommands, calendarCommands, remindersCommands,
		photosCommands, motionCommands,
	),
	"macos": flatten(
		canvasCommands, cameraCommands, locationCommands, deviceCommands,
		contactsCommands, calendarCommands, remindersCommands,
		photosCommands, motionCommands, systemCommands,
	),
	"linux":   append([]string{}, systemCommands...),
	"windows": append([]string{}, systemCommands...),
	"unknown": flatten(
		canvasCommands, cameraCommands, locationCommands, systemCommands,
	),
}

func flatten(lists ...[]string) []string {
	var out []string
	for _, list := range lists {
		out = append(out, list...)
	}
	return out
}

// ---------- 平台标识归一化 ----------

func normalizePlatformID(platform, deviceFamily string) string {
	raw := strings.TrimSpace(strings.ToLower(platform))
	switch {
	case strings.HasPrefix(raw, "ios"):
		return "ios"
	case strings.HasPrefix(raw, "android"):
		return "android"
	case strings.HasPrefix(raw, "mac"), strings.HasPrefix(raw, "darwin"):
		return "macos"
	case strings.HasPrefix(raw, "win"):
		return "windows"
	case strings.HasPrefix(raw, "linux"):
		return "linux"
	}
	family := strings.TrimSpace(strings.ToLower(deviceFamily))
	switch {
	case strings.Contains(family, "iphone"), strings.Contains(family, "ipad"), strings.Contains(family, "ios"):
		return "ios"
	case strings.Contains(family, "android"):
		return "android"
	case strings.Contains(family, "mac"):
		return "macos"
	case strings.Contains(family, "windows"):
		return "windows"
	case strings.Contains(family, "linux"):
		return "linux"
	}
	return "unknown"
}

// ---------- 白名单解析 ----------

// NodeCommandPolicyInput 节点命令策略输入。
type NodeCommandPolicyInput struct {
	Platform     string
	DeviceFamily string
	// 来自 config: gateway.nodes.allowCommands
	AllowCommands []string
	// 来自 config: gateway.nodes.denyCommands
	DenyCommands []string
}

// ResolveNodeCommandAllowlist 解析节点命令白名单。
func ResolveNodeCommandAllowlist(input NodeCommandPolicyInput) map[string]bool {
	platformID := normalizePlatformID(input.Platform, input.DeviceFamily)
	base, ok := platformDefaults[platformID]
	if !ok {
		base = platformDefaults["unknown"]
	}

	allow := make(map[string]bool)
	for _, cmd := range base {
		if t := strings.TrimSpace(cmd); t != "" {
			allow[t] = true
		}
	}
	for _, cmd := range input.AllowCommands {
		if t := strings.TrimSpace(cmd); t != "" {
			allow[t] = true
		}
	}
	for _, cmd := range input.DenyCommands {
		if t := strings.TrimSpace(cmd); t != "" {
			delete(allow, t)
		}
	}
	return allow
}

// ---------- 命令检查 ----------

// NodeCommandAllowedResult 命令执行许可结果。
type NodeCommandAllowedResult struct {
	OK     bool
	Reason string
}

// IsNodeCommandAllowed 检查节点命令是否被允许执行。
func IsNodeCommandAllowed(command string, declaredCommands []string, allowlist map[string]bool) NodeCommandAllowedResult {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return NodeCommandAllowedResult{OK: false, Reason: "command required"}
	}
	if !allowlist[cmd] {
		return NodeCommandAllowedResult{OK: false, Reason: "command not allowlisted"}
	}
	if len(declaredCommands) > 0 {
		found := false
		for _, dc := range declaredCommands {
			if dc == cmd {
				found = true
				break
			}
		}
		if !found {
			return NodeCommandAllowedResult{OK: false, Reason: "command not declared by node"}
		}
	} else {
		return NodeCommandAllowedResult{OK: false, Reason: "node did not declare commands"}
	}
	return NodeCommandAllowedResult{OK: true}
}
