package i18n

// i18n_browser_en.go — Browser automation English language pack
// Mirrors: i18n_browser_zh.go

func init() {
	RegisterBundle(LangEnUS, map[string]string{
		// ── Driver management ──
		"browser.driver.init":              "Browser driver initializing: {driver}",
		"browser.driver.cdp.ready":         "CDP browser driver ready",
		"browser.driver.playwright.ready":  "Playwright browser driver ready",
		"browser.driver.playwright.failed": "Playwright driver initialization failed: {error}",

		// ── AI browse loop ──
		"browser.ai.loop.start":     "AI browse loop started — goal: {goal}, max steps: {maxSteps}",
		"browser.ai.loop.done":      "AI browse loop completed in {steps} steps",
		"browser.ai.loop.max_steps": "AI browse loop reached maximum steps without completing goal",
		"browser.ai.observe.failed": "AI observe step {step} failed: {error}",
		"browser.ai.plan.failed":    "AI plan step {step} failed: {error}",
		"browser.ai.act.failed":     "AI act step {step} failed (action={action}): {error}",

		// ── Playwright native ──
		"browser.playwright.launch":  "Launching Playwright {browser} browser",
		"browser.playwright.connect": "Connecting Playwright to CDP endpoint: {url}",
		"browser.playwright.close":   "Playwright browser connection closed",
	})
}
