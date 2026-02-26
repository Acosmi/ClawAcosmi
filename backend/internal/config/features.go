package config

// features.go — 功能开关环境变量
//
// 对应 TS 的 OPENACOSMI_SKIP_* 系列环境变量。
// TS 参考: src/gateway/server/server.impl.ts (启动顺序第 4-7 步)
//
// 所有变量在进程启动时一次性读取，之后可供各子系统查询。

import "os"

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
