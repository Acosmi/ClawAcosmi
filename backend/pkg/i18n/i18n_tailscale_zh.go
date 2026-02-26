package i18n

// i18n_tailscale_zh.go — TailScale + mDNS 中文语言包
// 镜像: i18n_tailscale_en.go

func init() {
	RegisterBundle(LangZhCN, map[string]string{
		// ── Tailscale 二进制发现 ──
		"tailscale.binary.found":     "Tailscale 二进制已找到: {path}",
		"tailscale.binary.not_found": "未找到 Tailscale 二进制，使用默认 PATH 查找",
		"tailscale.binary.check":     "正在检查 Tailscale 二进制: {path}",

		// ── Tailscale 状态 ──
		"tailscale.status.connected":   "Tailscale 已连接: {hostname}",
		"tailscale.status.ip_fallback": "Tailscale DNS 不可用，使用 IP 回退: {ip}",
		"tailscale.status.error":       "Tailscale 状态查询失败: {error}",
		"tailscale.status.not_running": "Tailscale 未运行或未安装",

		// ── Tailscale whois ──
		"tailscale.whois.resolved":  "Tailscale 身份已识别: {login}",
		"tailscale.whois.cache_hit": "Tailscale whois 缓存命中: {ip}",
		"tailscale.whois.not_found": "未找到 Tailscale 身份: {ip}",

		// ── Tailscale sudo 降级 ──
		"tailscale.sudo.retry":   "权限不足，正在使用 sudo 重试: {command}",
		"tailscale.sudo.success": "sudo 重试成功",
		"tailscale.sudo.failed":  "sudo 重试也失败，使用原始错误",

		// ── Tailscale serve/funnel ──
		"tailscale.serve.enabled":   "Tailscale Serve 已在端口 {port} 启用",
		"tailscale.serve.disabled":  "Tailscale Serve 已停用",
		"tailscale.funnel.enabled":  "Tailscale Funnel 已在端口 {port} 启用",
		"tailscale.funnel.disabled": "Tailscale Funnel 已停用",
		"tailscale.expose.url":      "Tailscale {mode} 已激活: https://{hostname}{path}",

		// ── mDNS / Bonjour ──
		"bonjour.register.success": "mDNS 服务已注册: {name}，端口 {port}",
		"bonjour.register.failed":  "mDNS 服务注册失败: {error}",
		"bonjour.shutdown":         "mDNS 服务已停止",
		"bonjour.disabled.env":     "mDNS 已被环境变量禁用 (OPENACOSMI_DISABLE_BONJOUR)",
		"bonjour.watchdog.repair":  "mDNS 看门狗：服务未公告，正在尝试重新注册",
		"bonjour.watchdog.ok":      "mDNS 看门狗：所有服务正常",

		// ── 发现服务 ──
		"discovery.started":              "Gateway 发现服务已启动",
		"discovery.tailnet.dns_injected": "Tailnet DNS 已注入 mDNS TXT 记录: {dns}",
		"discovery.tailnet.dns_failed":   "获取 Tailnet DNS 用于 mDNS 失败: {error}",
		"discovery.zeroconf.created":     "已创建 zeroconf 注册器用于 mDNS",
	})
}
