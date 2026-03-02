package gateway

// wizard_helpers.go — Onboarding 共用辅助函数
// TS 对照: src/commands/onboard-helpers.ts (477L)
//
// 提供 gateway 可达性探测、Control UI URL 生成、token 生成等功能。

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// ---------- Token 辅助 ----------

// RandomToken 生成 24 字节 hex 随机 token（与 TS crypto.randomBytes(24).toString("hex") 等价）。
func RandomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		// 极端情况：crypto/rand 失败时 fallback 空字符串（不应发生）
		return ""
	}
	return hex.EncodeToString(b)
}

// NormalizeGatewayTokenInput 规范化用户输入的 gateway token。
// 空白/空值返回空字符串。
func NormalizeGatewayTokenInput(input string) string {
	return strings.TrimSpace(input)
}

// ---------- Control UI Links ----------

// ControlUiLinks HTTP 和 WS URL。
type ControlUiLinks struct {
	HttpURL string
	WsURL   string
}

// ResolveControlUiLinksParams 参数。
type ResolveControlUiLinksParams struct {
	Port           int
	Bind           string // "auto"|"lan"|"loopback"|"custom"|"tailnet"
	CustomBindHost string
	BasePath       string
}

// ResolveControlUiLinks 根据 bind mode 和 port 生成 HTTP/WS URL。
// 对应 TS resolveControlUiLinks (onboard-helpers.ts L437-466)。
func ResolveControlUiLinks(params ResolveControlUiLinksParams) ControlUiLinks {
	port := params.Port
	bind := params.Bind
	if bind == "" {
		bind = "loopback"
	}
	customBindHost := strings.TrimSpace(params.CustomBindHost)

	host := "127.0.0.1"
	switch bind {
	case "custom":
		if customBindHost != "" && IsValidIPv4(customBindHost) {
			host = customBindHost
		}
	case "tailnet":
		// Go 端暂无 pickPrimaryTailnetIPv4 等价实现，fallback loopback
		host = "127.0.0.1"
	case "lan":
		if lanIP := pickPrimaryLanIPv4(); lanIP != "" {
			host = lanIP
		}
	}

	basePath := normalizeControlUiBasePath(params.BasePath)
	uiPath := "/"
	wsPath := ""
	if basePath != "" {
		uiPath = basePath + "/"
		wsPath = basePath
	}

	return ControlUiLinks{
		HttpURL: fmt.Sprintf("http://%s:%d%s", host, port, uiPath),
		WsURL:   fmt.Sprintf("ws://%s:%d%s", host, port, wsPath),
	}
}

// normalizeControlUiBasePath 规范化 basePath（去尾斜杠，确保前导斜杠）。
func normalizeControlUiBasePath(basePath string) string {
	if basePath == "" {
		return ""
	}
	bp := strings.TrimRight(basePath, "/")
	if !strings.HasPrefix(bp, "/") {
		bp = "/" + bp
	}
	if bp == "/" {
		return ""
	}
	return bp
}

// pickPrimaryLanIPv4 获取主要的 LAN IPv4 地址。
func pickPrimaryLanIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok {
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			// 优先返回私有地址
			if ip[0] == 10 ||
				(ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31) ||
				(ip[0] == 192 && ip[1] == 168) {
				return ip.String()
			}
		}
	}
	return ""
}

// ---------- Gateway 可达性探测 ----------

// ProbeParams gateway 可达性探测参数。
type ProbeParams struct {
	URL       string
	Token     string
	Password  string
	TimeoutMs int // 默认 1500
}

// ProbeResult 探测结果。
type ProbeResult struct {
	OK     bool
	Detail string
}

// ProbeGatewayReachable 探测 gateway 是否可达（HTTP GET /health）。
// 对应 TS probeGatewayReachable (onboard-helpers.ts L360-382)。
func ProbeGatewayReachable(params ProbeParams) ProbeResult {
	url := strings.TrimSpace(params.URL)
	timeoutMs := params.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 1500
	}

	// 将 ws:// URL 转换为 http:// 进行 health check
	healthURL := url
	healthURL = strings.Replace(healthURL, "ws://", "http://", 1)
	healthURL = strings.Replace(healthURL, "wss://", "https://", 1)
	// 去掉路径，追加 /health
	if idx := strings.LastIndex(healthURL, ":"); idx > 5 {
		// 找端口后的路径
		rest := healthURL[idx:]
		if slashIdx := strings.Index(rest, "/"); slashIdx > 0 {
			healthURL = healthURL[:idx] + rest[:slashIdx]
		}
	}
	if !strings.HasSuffix(healthURL, "/health") {
		healthURL += "/health"
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return ProbeResult{OK: false, Detail: err.Error()}
	}

	if params.Token != "" {
		req.Header.Set("Authorization", "Bearer "+params.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ProbeResult{OK: false, Detail: summarizeError(err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return ProbeResult{OK: true}
	}
	return ProbeResult{OK: false, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

// WaitParams gateway 可达性轮询参数。
type WaitParams struct {
	URL            string
	Token          string
	Password       string
	DeadlineMs     int // 默认 15000
	ProbeTimeoutMs int // 默认 1500
	PollMs         int // 默认 400
}

// WaitForGatewayReachable 轮询等待 gateway 可达。
// 对应 TS waitForGatewayReachable (onboard-helpers.ts L384-416)。
func WaitForGatewayReachable(params WaitParams) ProbeResult {
	deadlineMs := params.DeadlineMs
	if deadlineMs <= 0 {
		deadlineMs = 15000
	}
	pollMs := params.PollMs
	if pollMs <= 0 {
		pollMs = 400
	}
	probeTimeoutMs := params.ProbeTimeoutMs
	if probeTimeoutMs <= 0 {
		probeTimeoutMs = 1500
	}

	deadline := time.Now().Add(time.Duration(deadlineMs) * time.Millisecond)
	var lastDetail string

	for time.Now().Before(deadline) {
		probe := ProbeGatewayReachable(ProbeParams{
			URL:       params.URL,
			Token:     params.Token,
			Password:  params.Password,
			TimeoutMs: probeTimeoutMs,
		})
		if probe.OK {
			return probe
		}
		lastDetail = probe.Detail
		time.Sleep(time.Duration(pollMs) * time.Millisecond)
	}

	return ProbeResult{OK: false, Detail: lastDetail}
}

// ---------- Browser 辅助 ----------

// BrowserOpenSupport 浏览器打开支持状态。
type BrowserOpenSupport struct {
	OK      bool
	Reason  string
	Command string
}

// DetectBrowserOpenSupport 检测当前环境是否支持打开浏览器。
func DetectBrowserOpenSupport() BrowserOpenSupport {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("open"); err == nil {
			return BrowserOpenSupport{OK: true, Command: "open"}
		}
		return BrowserOpenSupport{OK: false, Reason: "missing-open"}
	case "linux":
		if _, err := exec.LookPath("xdg-open"); err == nil {
			return BrowserOpenSupport{OK: true, Command: "xdg-open"}
		}
		return BrowserOpenSupport{OK: false, Reason: "missing-xdg-open"}
	case "windows":
		return BrowserOpenSupport{OK: true, Command: "cmd"}
	default:
		return BrowserOpenSupport{OK: false, Reason: "unsupported-platform"}
	}
}

// OpenURL 在默认浏览器中打开 URL。
func OpenURL(url string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	default:
		return false
	}
	if err := cmd.Start(); err != nil {
		return false
	}
	// 不等待子进程退出
	go func() { _ = cmd.Wait() }()
	return true
}

// FormatControlUiSshHint 生成 SSH 端口转发提示。
func FormatControlUiSshHint(port int, basePath, token string) string {
	bp := normalizeControlUiBasePath(basePath)
	uiPath := "/"
	if bp != "" {
		uiPath = bp + "/"
	}
	localURL := fmt.Sprintf("http://localhost:%d%s", port, uiPath)
	lines := []string{
		"No GUI detected. Open from your computer:",
		fmt.Sprintf("ssh -N -L %d:127.0.0.1:%d <user@host>", port, port),
		"Then open:",
		localURL,
	}
	if token != "" {
		lines = append(lines, fmt.Sprintf("%s#token=%s", localURL, token))
	}
	lines = append(lines,
		"Docs:",
		"docs/skills/gateway/remote/SKILL.md",
		"docs/skills/web/control-ui/SKILL.md",
	)
	return strings.Join(lines, "\n")
}

// ---------- 内部辅助 ----------

func summarizeError(err error) string {
	if err == nil {
		return "unknown error"
	}
	msg := err.Error()
	lines := strings.Split(msg, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			if len(trimmed) > 120 {
				return trimmed[:119] + "…"
			}
			return trimmed
		}
	}
	return msg
}
