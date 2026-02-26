package i18n

// i18n_browser_zh.go — 浏览器自动化中文语言包
// 镜像: i18n_browser_en.go

func init() {
	RegisterBundle(LangZhCN, map[string]string{
		// ── 驱动管理 ──
		"browser.driver.init":              "浏览器驱动初始化: {driver}",
		"browser.driver.cdp.ready":         "CDP 浏览器驱动就绪",
		"browser.driver.playwright.ready":  "Playwright 浏览器驱动就绪",
		"browser.driver.playwright.failed": "Playwright 驱动初始化失败: {error}",

		// ── AI 浏览循环 ──
		"browser.ai.loop.start":     "AI 浏览循环已启动 — 目标: {goal}，最大步数: {maxSteps}",
		"browser.ai.loop.done":      "AI 浏览循环在 {steps} 步内完成",
		"browser.ai.loop.max_steps": "AI 浏览循环已达到最大步数，目标未完成",
		"browser.ai.observe.failed": "AI 观察步骤 {step} 失败: {error}",
		"browser.ai.plan.failed":    "AI 规划步骤 {step} 失败: {error}",
		"browser.ai.act.failed":     "AI 执行步骤 {step} 失败 (操作={action}): {error}",

		// ── Playwright 原生 ──
		"browser.playwright.launch":  "正在启动 Playwright {browser} 浏览器",
		"browser.playwright.connect": "Playwright 正在连接 CDP 端点: {url}",
		"browser.playwright.close":   "Playwright 浏览器连接已关闭",
	})
}
