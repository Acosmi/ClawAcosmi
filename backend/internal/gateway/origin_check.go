package gateway

import (
	"net"
	"net/url"
	"strings"
)

// isLocalAddr 判断 remoteAddr（host:port 格式）是否为本地地址。
func isLocalAddr(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return isLoopbackHost(host)
}

// OriginCheckResult 浏览器 Origin 检查结果。
type OriginCheckResult struct {
	OK     bool
	Reason string
}

// CheckBrowserOrigin 验证浏览器 Origin 头是否被允许。
// 对齐 TS gateway/origin-check.ts checkBrowserOrigin。
func CheckBrowserOrigin(requestHost, origin string, allowedOrigins []string) OriginCheckResult {
	parsedOrigin := parseOriginURL(origin)
	if parsedOrigin == nil {
		return OriginCheckResult{OK: false, Reason: "origin missing or invalid"}
	}

	// 1. 白名单检查
	allowlist := normalizeAllowlist(allowedOrigins)
	for _, allowed := range allowlist {
		if allowed == parsedOrigin.origin {
			return OriginCheckResult{OK: true}
		}
	}

	// 2. requestHost 匹配
	normRequestHost := normalizeHostHeader(requestHost)
	if normRequestHost != "" && parsedOrigin.host == normRequestHost {
		return OriginCheckResult{OK: true}
	}

	// 3. 双端 loopback 放行
	requestHostname := resolveHostName(normRequestHost)
	if isLoopbackHost(parsedOrigin.hostname) && isLoopbackHost(requestHostname) {
		return OriginCheckResult{OK: true}
	}

	return OriginCheckResult{OK: false, Reason: "origin not allowed"}
}

// --- 辅助函数 ---

type parsedOrigin struct {
	origin   string // scheme://host
	host     string // host:port
	hostname string // host (no port)
}

func normalizeHostHeader(hostHeader string) string {
	return strings.ToLower(strings.TrimSpace(hostHeader))
}

func resolveHostName(hostHeader string) string {
	host := normalizeHostHeader(hostHeader)
	if host == "" {
		return ""
	}
	// IPv6 bracket notation: [::1]:8080
	if strings.HasPrefix(host, "[") {
		end := strings.Index(host, "]")
		if end != -1 {
			return host[1:end]
		}
	}
	// host:port → host
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		return host[:idx]
	}
	return host
}

func parseOriginURL(originRaw string) *parsedOrigin {
	trimmed := strings.TrimSpace(originRaw)
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return nil
	}
	return &parsedOrigin{
		origin:   strings.ToLower(u.Scheme + "://" + u.Host),
		host:     strings.ToLower(u.Host),
		hostname: strings.ToLower(u.Hostname()),
	}
}

func isLoopbackHost(hostname string) bool {
	if hostname == "" {
		return false
	}
	if hostname == "localhost" {
		return true
	}
	if hostname == "::1" {
		return true
	}
	if hostname == "127.0.0.1" || strings.HasPrefix(hostname, "127.") {
		return true
	}
	return false
}

func normalizeAllowlist(origins []string) []string {
	out := make([]string, 0, len(origins))
	for _, o := range origins {
		v := strings.ToLower(strings.TrimSpace(o))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
