package i18n

// i18n_tailscale_en.go — TailScale + mDNS English language pack
// Mirrors: i18n_tailscale_zh.go

func init() {
	RegisterBundle(LangEnUS, map[string]string{
		// ── Tailscale binary discovery ──
		"tailscale.binary.found":     "Tailscale binary found: {path}",
		"tailscale.binary.not_found": "Tailscale binary not found; using default PATH lookup",
		"tailscale.binary.check":     "Checking Tailscale binary: {path}",

		// ── Tailscale status ──
		"tailscale.status.connected":   "Tailscale connected: {hostname}",
		"tailscale.status.ip_fallback": "Tailscale DNS unavailable; using IP fallback: {ip}",
		"tailscale.status.error":       "Tailscale status query failed: {error}",
		"tailscale.status.not_running": "Tailscale is not running or not installed",

		// ── Tailscale whois ──
		"tailscale.whois.resolved":  "Tailscale identity resolved: {login}",
		"tailscale.whois.cache_hit": "Tailscale whois cache hit: {ip}",
		"tailscale.whois.not_found": "No Tailscale identity for: {ip}",

		// ── Tailscale sudo fallback ──
		"tailscale.sudo.retry":   "Permission denied; retrying with sudo: {command}",
		"tailscale.sudo.success": "Sudo retry succeeded",
		"tailscale.sudo.failed":  "Sudo retry also failed; using original error",

		// ── Tailscale serve/funnel ──
		"tailscale.serve.enabled":   "Tailscale Serve enabled on port {port}",
		"tailscale.serve.disabled":  "Tailscale Serve disabled",
		"tailscale.funnel.enabled":  "Tailscale Funnel enabled on port {port}",
		"tailscale.funnel.disabled": "Tailscale Funnel disabled",
		"tailscale.expose.url":      "Tailscale {mode} active: https://{hostname}{path}",

		// ── mDNS / Bonjour ──
		"bonjour.register.success": "mDNS service registered: {name} on port {port}",
		"bonjour.register.failed":  "mDNS service registration failed: {error}",
		"bonjour.shutdown":         "mDNS services stopped",
		"bonjour.disabled.env":     "mDNS disabled by environment variable (OPENACOSMI_DISABLE_BONJOUR)",
		"bonjour.watchdog.repair":  "mDNS watchdog: service not announced, attempting re-registration",
		"bonjour.watchdog.ok":      "mDNS watchdog: all services healthy",

		// ── Discovery ──
		"discovery.started":              "Gateway discovery service started",
		"discovery.tailnet.dns_injected": "Tailnet DNS injected into mDNS TXT: {dns}",
		"discovery.tailnet.dns_failed":   "Failed to get Tailnet DNS for mDNS: {error}",
		"discovery.zeroconf.created":     "Zeroconf registrar created for mDNS",
	})
}
