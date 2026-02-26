package gateway

// TS 对照: src/infra/tailscale.ts (496L)
// Gateway Tailscale 暴露控制 — 多策略二进制发现 + serve/funnel + whois + sudo 降级。

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TailscaleMode Tailscale 暴露模式。
type TailscaleMode string

const (
	TailscaleModeOff    TailscaleMode = "off"
	TailscaleModeServe  TailscaleMode = "serve"
	TailscaleModeFunnel TailscaleMode = "funnel"
)

// TailscaleExposureParams Tailscale 暴露参数。
type TailscaleExposureParams struct {
	Mode              TailscaleMode
	ResetOnExit       bool
	Port              int
	ControlUIBasePath string
	Logger            *slog.Logger
}

// TailscaleExposure Tailscale 暴露控制实例。
type TailscaleExposure struct {
	params TailscaleExposureParams
}

// ---------- 多策略二进制发现 ----------
// TS 对照: tailscale.ts findTailscaleBinary (L29-104)

var (
	tailscaleBinaryOnce sync.Once
	tailscaleBinaryPath string
)

// FindTailscaleBinary 通过 4 种策略定位 tailscale 二进制文件。
// 结果会缓存，后续调用直接返回。
func FindTailscaleBinary() string {
	tailscaleBinaryOnce.Do(func() {
		tailscaleBinaryPath = doFindTailscaleBinary()
	})
	return tailscaleBinaryPath
}

// ResetTailscaleBinaryCache 重置缓存（仅用于测试）。
func ResetTailscaleBinaryCache() {
	tailscaleBinaryOnce = sync.Once{}
	tailscaleBinaryPath = ""
}

func doFindTailscaleBinary() string {
	// 策略 1: PATH 查找
	if p, err := exec.LookPath("tailscale"); err == nil {
		if checkTailscaleBinary(p) {
			return p
		}
	}

	// 策略 2: macOS 已知路径
	macAppPath := "/Applications/Tailscale.app/Contents/MacOS/Tailscale"
	if checkTailscaleBinary(macAppPath) {
		return macAppPath
	}

	// 策略 3: find 命令搜索 /Applications
	if bin := findViaMacOSSearch(); bin != "" {
		return bin
	}

	// 策略 4: locate 数据库
	if bin := findViaLocate(); bin != "" {
		return bin
	}

	// 兜底：返回默认名称（依赖 PATH）
	return "tailscale"
}

// checkTailscaleBinary 验证路径是否为可执行的 tailscale 二进制。
func checkTailscaleBinary(path string) bool {
	if path == "" {
		return false
	}
	// 绝对路径先检查文件是否存在
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	// 执行 --version 验证（3 秒超时）
	cmd := exec.Command(path, "--version")
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()
	select {
	case err := <-done:
		return err == nil
	case <-time.After(3 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return false
	}
}

// findViaMacOSSearch 通过 find 命令在 /Applications 下搜索 Tailscale。
func findViaMacOSSearch() string {
	cmd := exec.Command("find", "/Applications",
		"-maxdepth", "3",
		"-name", "Tailscale",
		"-path", "*/Tailscale.app/Contents/MacOS/Tailscale",
	)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	lines := strings.SplitN(strings.TrimSpace(string(output)), "\n", 2)
	if len(lines) > 0 && lines[0] != "" {
		if checkTailscaleBinary(lines[0]) {
			return lines[0]
		}
	}
	return ""
}

// findViaLocate 通过 locate 数据库搜索 Tailscale。
func findViaLocate() string {
	cmd := exec.Command("locate", "Tailscale.app")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.Contains(line, "/Tailscale.app/Contents/MacOS/Tailscale") {
			if checkTailscaleBinary(line) {
				return line
			}
		}
	}
	return ""
}

// ---------- Tailscale 状态查询 ----------
// TS 对照: tailscale.ts getTailnetHostname (L106-144)

// tailscaleStatusJSON tailscale status --json 的结构。
type tailscaleStatusJSON struct {
	Self *tailscaleSelf `json:"Self"`
}

type tailscaleSelf struct {
	DNSName      string   `json:"DNSName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
}

// parsePossiblyNoisyJSON 解析可能包含前后噪声的 JSON 输出。
// TS 对照: tailscale.ts parsePossiblyNoisyJsonObject (L10-18)
func parsePossiblyNoisyJSON(data []byte) ([]byte, error) {
	trimmed := strings.TrimSpace(string(data))
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		return []byte(trimmed[start : end+1]), nil
	}
	return data, nil
}

// getTailnetHostname 获取 Tailscale 网络主机名（DNS 优先，IP 回退）。
// TS 对照: tailscale.ts getTailnetHostname (L106-144)
func getTailnetHostname() (string, error) {
	bin := FindTailscaleBinary()
	candidates := []string{bin}
	// 如果不是绝对路径，加上 macOS 备选
	if bin != "/Applications/Tailscale.app/Contents/MacOS/Tailscale" {
		candidates = append(candidates, "/Applications/Tailscale.app/Contents/MacOS/Tailscale")
	}

	var lastErr error
	for _, candidate := range candidates {
		if filepath.IsAbs(candidate) {
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
		}
		cmd := exec.Command(candidate, "status", "--json")
		output, err := cmd.Output()
		if err != nil {
			lastErr = err
			continue
		}

		cleaned, _ := parsePossiblyNoisyJSON(output)
		var status tailscaleStatusJSON
		if jsonErr := json.Unmarshal(cleaned, &status); jsonErr != nil {
			lastErr = fmt.Errorf("tailscale status parse: %w", jsonErr)
			continue
		}

		if status.Self != nil {
			// DNS 名优先
			dns := strings.TrimSuffix(strings.TrimSpace(status.Self.DNSName), ".")
			if dns != "" {
				return dns, nil
			}
			// IP 回退
			if len(status.Self.TailscaleIPs) > 0 {
				return status.Self.TailscaleIPs[0], nil
			}
		}
		lastErr = fmt.Errorf("could not determine Tailscale DNS or IP")
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("could not determine Tailscale DNS or IP")
}

// ReadTailscaleStatusJSON 读取完整的 tailscale status JSON。
func ReadTailscaleStatusJSON() (map[string]interface{}, error) {
	bin := FindTailscaleBinary()
	cmd := exec.Command(bin, "status", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tailscale status: %w", err)
	}
	cleaned, _ := parsePossiblyNoisyJSON(output)
	var result map[string]interface{}
	if err := json.Unmarshal(cleaned, &result); err != nil {
		return nil, fmt.Errorf("tailscale status parse: %w", err)
	}
	return result, nil
}

// ---------- Whois 身份识别 ----------
// TS 对照: tailscale.ts readTailscaleWhoisIdentity (L464-495)

// TailscaleWhoisIdentity Tailscale whois 身份信息。
type TailscaleWhoisIdentity struct {
	Login string
	Name  string
}

type whoisCacheEntry struct {
	value     *TailscaleWhoisIdentity
	expiresAt time.Time
}

var (
	whoisCache   = make(map[string]whoisCacheEntry)
	whoisCacheMu sync.RWMutex
)

// ReadTailscaleWhoisIdentity 通过 tailscale whois 查询 IP 身份（带 TTL 缓存）。
// TS 对照: tailscale.ts readTailscaleWhoisIdentity (L464-495)
func ReadTailscaleWhoisIdentity(ip string, cacheTTL, errorTTL time.Duration) *TailscaleWhoisIdentity {
	normalized := strings.TrimSpace(ip)
	if normalized == "" {
		return nil
	}
	if cacheTTL == 0 {
		cacheTTL = 60 * time.Second
	}
	if errorTTL == 0 {
		errorTTL = 5 * time.Second
	}

	// 缓存命中
	whoisCacheMu.RLock()
	entry, ok := whoisCache[normalized]
	whoisCacheMu.RUnlock()
	if ok {
		if time.Now().Before(entry.expiresAt) {
			return entry.value
		}
		// 过期，删除
		whoisCacheMu.Lock()
		delete(whoisCache, normalized)
		whoisCacheMu.Unlock()
	}

	// 查询
	identity := doWhoisQuery(normalized)

	// 写缓存
	ttl := cacheTTL
	if identity == nil {
		ttl = errorTTL
	}
	whoisCacheMu.Lock()
	whoisCache[normalized] = whoisCacheEntry{
		value:     identity,
		expiresAt: time.Now().Add(ttl),
	}
	whoisCacheMu.Unlock()

	return identity
}

func doWhoisQuery(ip string) *TailscaleWhoisIdentity {
	bin := FindTailscaleBinary()
	cmd := exec.Command(bin, "whois", "--json", ip)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	cleaned, _ := parsePossiblyNoisyJSON(output)
	var payload map[string]interface{}
	if err := json.Unmarshal(cleaned, &payload); err != nil {
		return nil
	}

	return parseWhoisIdentity(payload)
}

// parseWhoisIdentity 从 whois JSON 中提取身份信息。
// TS 对照: tailscale.ts parseWhoisIdentity (L427-446)
func parseWhoisIdentity(payload map[string]interface{}) *TailscaleWhoisIdentity {
	// 尝试多种字段名
	userProfile := getNestedMap(payload, "UserProfile", "userProfile", "User")

	login := getStringFromMaps(userProfile, payload, "LoginName", "Login", "login")
	if login == "" {
		return nil
	}

	name := getStringFromMaps(userProfile, payload, "DisplayName", "Name", "displayName", "name")

	return &TailscaleWhoisIdentity{Login: login, Name: name}
}

func getNestedMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if nested, ok := v.(map[string]interface{}); ok {
				return nested
			}
		}
	}
	return nil
}

func getStringFromMaps(primary, fallback map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if primary != nil {
			if v, ok := primary[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
		if fallback != nil {
			if v, ok := fallback[k]; ok {
				if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
					return strings.TrimSpace(s)
				}
			}
		}
	}
	return ""
}

// ---------- Sudo 降级回退 ----------
// TS 对照: tailscale.ts execWithSudoFallback (L271-295)

// isPermissionDeniedError 判断错误是否为权限拒绝。
// TS 对照: tailscale.ts isPermissionDeniedError (L251-268)
func isPermissionDeniedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	patterns := []string{
		"permission denied",
		"access denied",
		"operation not permitted",
		"not permitted",
		"requires root",
		"must be run as root",
		"must be run with sudo",
		"requires sudo",
		"need sudo",
	}
	for _, p := range patterns {
		if strings.Contains(msg, p) {
			return true
		}
	}
	// 检查 exit error 中的 stderr（如果存在）
	if exitErr, ok := err.(*exec.ExitError); ok {
		stderr := strings.ToLower(string(exitErr.Stderr))
		for _, p := range patterns {
			if strings.Contains(stderr, p) {
				return true
			}
		}
	}
	return false
}

// execWithSudoFallback 执行命令，如遇权限拒绝则自动用 sudo -n 重试。
// TS 对照: tailscale.ts execWithSudoFallback (L271-295)
func execWithSudoFallback(bin string, args ...string) ([]byte, error) {
	cmd := exec.Command(bin, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return output, nil
	}

	// 非权限错误直接返回
	if !isPermissionDeniedError(err) {
		return output, fmt.Errorf("%s %v: %w (output: %s)", bin, args, err, string(output))
	}

	// 权限拒绝 → 用 sudo -n 重试
	sudoArgs := append([]string{"-n", bin}, args...)
	sudoCmd := exec.Command("sudo", sudoArgs...)
	sudoOutput, sudoErr := sudoCmd.CombinedOutput()
	if sudoErr == nil {
		return sudoOutput, nil
	}

	// sudo 也失败 → 返回原始错误
	return output, fmt.Errorf("%s %v: %w (output: %s)", bin, args, err, string(output))
}

// ---------- Serve / Funnel 控制 ----------
// TS 对照: tailscale.ts enableTailscaleServe/disableTailscaleServe/enableTailscaleFunnel/disableTailscaleFunnel

// runTailscaleCommand 执行 tailscale CLI 命令（支持 sudo 降级）。
func runTailscaleCommand(args ...string) error {
	bin := FindTailscaleBinary()
	_, err := execWithSudoFallback(bin, args...)
	return err
}

// EnableTailscaleServe 启用 Tailscale Serve。
func EnableTailscaleServe(port int) error {
	return runTailscaleCommand("serve", "--bg", "--yes", strconv.Itoa(port))
}

// DisableTailscaleServe 停用 Tailscale Serve。
func DisableTailscaleServe() error {
	return runTailscaleCommand("serve", "reset")
}

// EnableTailscaleFunnel 启用 Tailscale Funnel。
func EnableTailscaleFunnel(port int) error {
	return runTailscaleCommand("funnel", "--bg", "--yes", strconv.Itoa(port))
}

// DisableTailscaleFunnel 停用 Tailscale Funnel。
func DisableTailscaleFunnel() error {
	return runTailscaleCommand("funnel", "reset")
}

// ---------- 启动/停止 ----------

// StartGatewayTailscaleExposure 启动 Tailscale 暴露。
// TS 对照: server-tailscale.ts startGatewayTailscaleExposure (L9-58)
func StartGatewayTailscaleExposure(params TailscaleExposureParams) (*TailscaleExposure, error) {
	if params.Mode == TailscaleModeOff || params.Mode == "" {
		return nil, nil
	}

	var err error
	switch params.Mode {
	case TailscaleModeServe:
		err = EnableTailscaleServe(params.Port)
	case TailscaleModeFunnel:
		err = EnableTailscaleFunnel(params.Port)
	default:
		return nil, fmt.Errorf("tailscale: unknown mode %q", params.Mode)
	}

	if err != nil {
		if params.Logger != nil {
			params.Logger.Warn(fmt.Sprintf("tailscale %s failed", params.Mode), "error", err)
		}
		return nil, nil // 不阻塞启动
	}

	// 尝试获取 tailnet hostname
	hostname, hostErr := getTailnetHostname()
	if params.Logger != nil {
		if hostErr == nil && hostname != "" {
			uiPath := "/"
			if params.ControlUIBasePath != "" {
				uiPath = params.ControlUIBasePath + "/"
			}
			params.Logger.Info(fmt.Sprintf("tailscale %s enabled: https://%s%s (WS via wss://%s)", params.Mode, hostname, uiPath, hostname))
		} else {
			params.Logger.Info(fmt.Sprintf("tailscale %s enabled", params.Mode))
		}
	}

	if !params.ResetOnExit {
		return nil, nil
	}

	return &TailscaleExposure{params: params}, nil
}

// Stop 停止 Tailscale 暴露（清理）。
func (t *TailscaleExposure) Stop() {
	if t == nil {
		return
	}
	var err error
	switch t.params.Mode {
	case TailscaleModeServe:
		err = DisableTailscaleServe()
	case TailscaleModeFunnel:
		err = DisableTailscaleFunnel()
	}
	if err != nil && t.params.Logger != nil {
		t.params.Logger.Warn(fmt.Sprintf("tailscale %s cleanup failed", t.params.Mode), "error", err)
	}
}
