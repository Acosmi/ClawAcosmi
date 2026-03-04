package config

// features.go — 功能开关环境变量
//
// 对应 TS 的 OPENACOSMI_SKIP_* 系列环境变量。
// TS 参考: src/gateway/server/server.impl.ts (启动顺序第 4-7 步)
//
// 所有变量在进程启动时一次性读取，之后可供各子系统查询。

import (
	"os"
	"strings"
)

// SkipCron 跳过 cron 调度器启动。
// 对应 TS OPENACOSMI_SKIP_CRON 环境变量。
var SkipCron = os.Getenv("OPENACOSMI_SKIP_CRON") != ""

// SkipChannels 跳过通道子系统启动（WhatsApp/Telegram/Discord 等）。
// 对应 TS OPENACOSMI_SKIP_CHANNELS 环境变量。
var SkipChannels = os.Getenv("OPENACOSMI_SKIP_CHANNELS") != ""

// SkipBrowserControl 跳过浏览器控制服务器启动。
// 对应 TS OPENACOSMI_SKIP_BROWSER_CONTROL_SERVER 环境变量。
var SkipBrowserControl = os.Getenv("OPENACOSMI_SKIP_BROWSER_CONTROL_SERVER") != ""

// SkipCanvasHost 跳过 Canvas 主机启动。
// 对应 TS OPENACOSMI_SKIP_CANVAS_HOST 环境变量。
var SkipCanvasHost = os.Getenv("OPENACOSMI_SKIP_CANVAS_HOST") != ""

// SkipProviders 跳过 provider 初始化（仅用于测试）。
// 对应 TS OPENACOSMI_SKIP_PROVIDERS 环境变量。
var SkipProviders = os.Getenv("OPENACOSMI_SKIP_PROVIDERS") != ""

// MultimodalChannelsSwitch 控制多模态渠道灰度开关。
// 语义：
//   - 空 / all / * : 全量启用（默认）
//   - none / off / false / disabled / 0 : 全量禁用（快速回滚）
//   - feishu,dingtalk,wecom : 仅启用指定渠道（灰度）
var MultimodalChannelsSwitch = os.Getenv("OPENACOSMI_MULTIMODAL_CHANNELS")

var multimodalAllowAll, multimodalAllowList = parseMultimodalChannelsSwitch(MultimodalChannelsSwitch)

// IsMultimodalChannelEnabled 返回指定渠道是否启用多模态分发。
// channel 建议使用：feishu / dingtalk / wecom。
func IsMultimodalChannelEnabled(channel string) bool {
	return isMultimodalChannelEnabled(channel, multimodalAllowAll, multimodalAllowList)
}

func isMultimodalChannelEnabled(channel string, allowAll bool, allowList map[string]bool) bool {
	ch := normalizeMultimodalChannelName(channel)
	if ch == "" {
		return false
	}
	if allowAll {
		return true
	}
	return allowList[ch]
}

func parseMultimodalChannelsSwitch(raw string) (bool, map[string]bool) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "", "all", "*":
		return true, map[string]bool{}
	case "none", "off", "false", "disabled", "0":
		return false, map[string]bool{}
	}

	replacer := strings.NewReplacer(";", ",", "|", ",")
	tokens := strings.Split(replacer.Replace(normalized), ",")
	allowList := make(map[string]bool)
	for _, token := range tokens {
		for _, part := range strings.Fields(token) {
			name := normalizeMultimodalChannelName(part)
			if name == "" {
				continue
			}
			allowList[name] = true
		}
	}

	// 非法值 fail-open：避免拼写错误导致全量关停。
	if len(allowList) == 0 {
		return true, map[string]bool{}
	}
	return false, allowList
}

func normalizeMultimodalChannelName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "" {
		return ""
	}
	name = strings.NewReplacer("_", "", "-", "", " ", "").Replace(name)
	switch name {
	case "feishu", "lark":
		return "feishu"
	case "dingtalk":
		return "dingtalk"
	case "wecom", "wechatwork", "workweixin":
		return "wecom"
	default:
		// 非已知渠道名按空处理，避免误拼写导致“只启用未知渠道”从而误关停已知渠道。
		return ""
	}
}
