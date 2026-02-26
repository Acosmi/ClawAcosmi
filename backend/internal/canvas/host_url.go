package canvas

// 画布主机 URL 解析 — 对应 src/infra/canvas-host-url.ts (82L)
//
// 从 canvasPort + 多种 host 来源生成完整的 canvas host URL。

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// CanvasHostURLParams 画布主机 URL 参数。
// TS 对照: canvas-host-url.ts L3-10
type CanvasHostURLParams struct {
	CanvasPort     int
	HostOverride   string
	RequestHost    string
	ForwardedProto string // 首个值
	LocalAddress   string
	Scheme         string // "http" 或 "https"
}

// ResolveCanvasHostURL 解析画布主机 URL。
// TS 对照: canvas-host-url.ts L61-81
func ResolveCanvasHostURL(params CanvasHostURLParams) string {
	if params.CanvasPort <= 0 {
		return ""
	}

	scheme := params.Scheme
	if scheme == "" {
		if strings.TrimSpace(params.ForwardedProto) == "https" {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}

	override := normalizeHost(params.HostOverride, true)
	requestHost := normalizeHost(parseHostHeader(params.RequestHost), override != "")
	localAddress := normalizeHost(params.LocalAddress, override != "" || requestHost != "")

	host := override
	if host == "" {
		host = requestHost
	}
	if host == "" {
		host = localAddress
	}
	if host == "" {
		return ""
	}

	formatted := host
	if strings.Contains(host, ":") {
		formatted = "[" + host + "]"
	}
	return fmt.Sprintf("%s://%s:%d", scheme, formatted, params.CanvasPort)
}

// isLoopbackHost 判断是否为回环地址。
// TS 对照: canvas-host-url.ts L12-27
func isLoopbackHost(value string) bool {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if normalized == "" {
		return false
	}
	if normalized == "localhost" || normalized == "::1" || normalized == "0.0.0.0" || normalized == "::" {
		return true
	}
	return strings.HasPrefix(normalized, "127.")
}

// normalizeHost 规范化主机地址。
// TS 对照: canvas-host-url.ts L29-41
func normalizeHost(value string, rejectLoopback bool) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if rejectLoopback && isLoopbackHost(trimmed) {
		return ""
	}
	return trimmed
}

// parseHostHeader 从 Host 头解析主机名。
// TS 对照: canvas-host-url.ts L43-52
func parseHostHeader(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	u, err := url.Parse("http://" + trimmed)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	if host == "" {
		// 尝试直接解析（可能是纯 IP）
		h, _, splitErr := net.SplitHostPort(trimmed)
		if splitErr == nil {
			return h
		}
		return trimmed
	}
	return host
}
