// manage.go — 沙箱容器/浏览器生命周期管理。
//
// TS 对照: agents/sandbox/manage.ts (120L),
//
//	agents/sandbox/prune.ts (102L),
//	agents/sandbox/browser.ts (233L),
//	agents/sandbox/browser-bridges.ts (4L),
//	agents/sandbox/workspace.ts (52L)
//
// 容器列表、删除、剪枝、浏览器管理、工作区创建。
package sandbox

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ---------- 浏览器桥接 ----------
// TS 对照: browser-bridges.ts BROWSER_BRIDGES

// BrowserBridge 活跃的浏览器桥接实例。
type BrowserBridge struct {
	ContainerName string
	BridgeURL     string
	CDPPort       int
	StopFunc      func() // 停止桥接的回调
}

// browserBridges 活跃的浏览器桥接 map。
var browserBridges sync.Map // map[string]*BrowserBridge (key: sessionKey)

// GetBrowserBridge 获取会话的浏览器桥接。
func GetBrowserBridge(sessionKey string) *BrowserBridge {
	if v, ok := browserBridges.Load(sessionKey); ok {
		return v.(*BrowserBridge)
	}
	return nil
}

// SetBrowserBridge 设置会话的浏览器桥接。
func SetBrowserBridge(sessionKey string, bridge *BrowserBridge) {
	browserBridges.Store(sessionKey, bridge)
}

// RemoveBrowserBridge 移除并停止会话的浏览器桥接。
func RemoveBrowserBridge(sessionKey string) {
	if v, ok := browserBridges.LoadAndDelete(sessionKey); ok {
		if bridge := v.(*BrowserBridge); bridge.StopFunc != nil {
			bridge.StopFunc()
		}
	}
}

// ---------- 容器列表 ----------

// ListSandboxContainers 列出所有沙箱容器及其状态。
// TS 对照: manage.ts listSandboxContainers()
func ListSandboxContainers(registryPath string, configuredImage string) ([]ContainerInfo, error) {
	reg, err := ReadRegistry(registryPath)
	if err != nil {
		return nil, fmt.Errorf("reading registry: %w", err)
	}

	results := make([]ContainerInfo, 0, len(reg.Entries))
	for _, entry := range reg.Entries {
		state := DockerContainerState(entry.ContainerName)

		actualImage := entry.Image
		if state.Exists {
			if img, err := ReadContainerImage(entry.ContainerName); err == nil && img != "" {
				actualImage = img
			}
		}

		results = append(results, ContainerInfo{
			RegistryEntry: RegistryEntry{
				ContainerName: entry.ContainerName,
				SessionKey:    entry.SessionKey,
				CreatedAtMs:   entry.CreatedAtMs,
				LastUsedAtMs:  entry.LastUsedAtMs,
				Image:         actualImage,
				ConfigHash:    entry.ConfigHash,
			},
			Running:    state.Running,
			ImageMatch: actualImage == configuredImage,
		})
	}

	return results, nil
}

// ListSandboxBrowserContainers 列出所有沙箱浏览器容器及其状态。
// TS 对照: manage.ts listSandboxBrowsers()
func ListSandboxBrowserContainers(browserRegistryPath string, configuredImage string) ([]BrowserContainerInfo, error) {
	reg, err := ReadBrowserRegistry(browserRegistryPath)
	if err != nil {
		return nil, fmt.Errorf("reading browser registry: %w", err)
	}

	results := make([]BrowserContainerInfo, 0, len(reg.Entries))
	for _, entry := range reg.Entries {
		state := DockerContainerState(entry.ContainerName)

		actualImage := entry.Image
		if state.Exists {
			if img, err := ReadContainerImage(entry.ContainerName); err == nil && img != "" {
				actualImage = img
			}
		}

		// 读取 noVNC 端口映射
		var noVncPort int
		if state.Running {
			if p, err := ReadDockerPort(entry.ContainerName, DefaultNoVNCPort); err == nil {
				noVncPort = p
			}
		}

		results = append(results, BrowserContainerInfo{
			BrowserRegistryEntry: BrowserRegistryEntry{
				ContainerName: entry.ContainerName,
				SessionKey:    entry.SessionKey,
				CreatedAtMs:   entry.CreatedAtMs,
				LastUsedAtMs:  entry.LastUsedAtMs,
				Image:         actualImage,
				CDPPort:       entry.CDPPort,
			},
			Running:    state.Running,
			ImageMatch: actualImage == configuredImage,
			NoVncPort:  noVncPort,
		})
	}

	return results, nil
}

// ---------- 容器删除 ----------

// RemoveSandboxContainer 删除沙箱容器及其注册表条目。
// TS 对照: manage.ts removeSandboxContainer()
func RemoveSandboxContainer(registryPath string, containerName string) error {
	_ = RemoveContainer(containerName)
	return RemoveRegistryEntry(registryPath, containerName)
}

// RemoveSandboxBrowser 删除沙箱浏览器及其注册表条目。
func RemoveSandboxBrowser(browserRegistryPath string, containerName string, sessionKey string) error {
	RemoveBrowserBridge(sessionKey)
	_ = RemoveContainer(containerName)
	return RemoveBrowserRegistryEntry(browserRegistryPath, containerName)
}

// ---------- 容器剪枝 ----------
// TS 对照: prune.ts

// lastPruneAtMs 上次剪枝时间戳（毫秒），用于 5 分钟限流。
// TS 对照: prune.ts let lastPruneAtMs = 0
var lastPruneAtMs atomic.Int64

// PruneSandboxContainers 清理空闲或过期的沙箱容器。
// TS 对照: prune.ts pruneSandboxContainers()
func PruneSandboxContainers(registryPath string, idleHours, maxAgeDays int) error {
	if idleHours == 0 && maxAgeDays == 0 {
		return nil
	}

	now := time.Now().UnixMilli()
	reg, err := ReadRegistry(registryPath)
	if err != nil {
		return err
	}

	for _, entry := range reg.Entries {
		idleMs := now - entry.LastUsedAtMs
		ageMs := now - entry.CreatedAtMs

		shouldPrune := false
		if idleHours > 0 && idleMs > int64(idleHours)*60*60*1000 {
			shouldPrune = true
		}
		if maxAgeDays > 0 && ageMs > int64(maxAgeDays)*24*60*60*1000 {
			shouldPrune = true
		}

		if shouldPrune {
			_ = RemoveContainer(entry.ContainerName)
			_ = RemoveRegistryEntry(registryPath, entry.ContainerName)
		}
	}

	return nil
}

// PruneSandboxBrowsers 清理空闲或过期的沙箱浏览器。
// TS 对照: prune.ts pruneSandboxBrowsers()
func PruneSandboxBrowsers(browserRegistryPath string, idleHours, maxAgeDays int) error {
	if idleHours == 0 && maxAgeDays == 0 {
		return nil
	}

	now := time.Now().UnixMilli()
	reg, err := ReadBrowserRegistry(browserRegistryPath)
	if err != nil {
		return err
	}

	for _, entry := range reg.Entries {
		idleMs := now - entry.LastUsedAtMs
		ageMs := now - entry.CreatedAtMs

		shouldPrune := false
		if idleHours > 0 && idleMs > int64(idleHours)*60*60*1000 {
			shouldPrune = true
		}
		if maxAgeDays > 0 && ageMs > int64(maxAgeDays)*24*60*60*1000 {
			shouldPrune = true
		}

		if shouldPrune {
			_ = RemoveContainer(entry.ContainerName)
			_ = RemoveBrowserRegistryEntry(browserRegistryPath, entry.ContainerName)
			// 清理 bridge 注册表（与 TS 一致）
			RemoveBrowserBridge(entry.SessionKey)
		}
	}

	return nil
}

// MaybePruneSandboxes 带限流的沙箱剪枝入口。
// 每 5 分钟最多执行一次。
// TS 对照: prune.ts maybePruneSandboxes()
func MaybePruneSandboxes(registryPath, browserRegistryPath string, cfg SandboxConfig) {
	now := time.Now().UnixMilli()
	last := lastPruneAtMs.Load()
	if now-last < 5*60*1000 {
		return
	}
	lastPruneAtMs.Store(now)

	if err := PruneSandboxContainers(registryPath, cfg.Prune.IdleHours, cfg.Prune.MaxAgeDays); err != nil {
		slog.Error("sandbox prune containers failed", "error", err)
	}
	if err := PruneSandboxBrowsers(browserRegistryPath, cfg.Prune.IdleHours, cfg.Prune.MaxAgeDays); err != nil {
		slog.Error("sandbox prune browsers failed", "error", err)
	}
}

// EnsureDockerContainerIsRunning 确保 Docker 容器正在运行。
// TS 对照: prune.ts ensureDockerContainerIsRunning()
func EnsureDockerContainerIsRunning(containerName string) error {
	state := DockerContainerState(containerName)
	if state.Exists && !state.Running {
		if _, err := ExecDocker([]string{"start", containerName}, nil); err != nil {
			return fmt.Errorf("starting container %s: %w", containerName, err)
		}
	}
	return nil
}

// ---------- 浏览器管理 ----------

// WaitForSandboxCDP 等待沙箱浏览器 CDP 就绪。
// TS 对照: browser.ts waitForSandboxCdp()
func WaitForSandboxCDP(cdpPort, timeoutMs int) bool {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	url := fmt.Sprintf("http://127.0.0.1:%d/json/version", cdpPort)

	client := &http.Client{Timeout: 1 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(CDPWaitInterval)
	}
	return false
}

// EnsureSandboxBrowserImage 确保浏览器镜像存在。
// TS 对照: browser.ts ensureSandboxBrowserImage()
func EnsureSandboxBrowserImage(image string) error {
	result, err := ExecDocker([]string{"image", "inspect", image}, &ExecDockerOpts{AllowFailure: true})
	if err != nil {
		return fmt.Errorf("checking browser image: %w", err)
	}
	if result.Code == 0 {
		return nil
	}
	return fmt.Errorf("sandbox browser image not found: %s. Build it with scripts/sandbox-browser-setup.sh", image)
}

// ReadDockerPort 读取容器端口映射的主机端口。
// TS 对照: docker.ts readDockerPort()
func ReadDockerPort(containerName string, containerPort int) (int, error) {
	result, err := ExecDocker(
		[]string{"port", containerName, fmt.Sprintf("%d", containerPort)},
		&ExecDockerOpts{AllowFailure: true},
	)
	if err != nil || result.Code != 0 {
		return 0, fmt.Errorf("reading port %d from container %s", containerPort, containerName)
	}

	portStr := extractPortFromDockerOutput(result.Stdout)
	if portStr == "" {
		return 0, fmt.Errorf("failed to parse port mapping for %s:%d", containerName, containerPort)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port number %q: %w", portStr, err)
	}
	return port, nil
}

// EnsureSandboxBrowser 确保沙箱浏览器容器正在运行。
// TS 对照: browser.ts ensureSandboxBrowser()
func EnsureSandboxBrowser(cfg SandboxConfig, sessionKey, workspaceDir, agentWorkspaceDir, stateDir string) (*SandboxBrowserContext, error) {
	if !cfg.Browser.Enabled {
		return nil, nil
	}

	// TS 对照: browser.ts L97-99 — isToolAllowed(params.cfg.tools, "browser")
	compiled := CompileToolPolicy(cfg.Tools)
	if !IsToolAllowed(compiled, "browser") {
		return nil, nil
	}

	// 生成容器名（scope-aware）
	prefix := cfg.Browser.ContainerPrefix
	if prefix == "" {
		prefix = DefaultBrowserContainerPrefix
	}
	slug := SlugifySessionKey(sessionKey)
	if cfg.Scope == ScopeShared {
		slug = "shared"
	}
	containerName := prefix + slug
	if len(containerName) > 63 {
		containerName = containerName[:63]
	}

	// 检查容器状态
	state := DockerContainerState(containerName)
	if !state.Exists {
		// 确保镜像存在
		browserImage := cfg.Browser.Image
		if browserImage == "" {
			browserImage = DefaultBrowserImage
		}
		if err := EnsureSandboxBrowserImage(browserImage); err != nil {
			return nil, err
		}

		// 构建 docker create 参数
		cdpPort := cfg.Browser.CDPPort
		if cdpPort == 0 {
			cdpPort = DefaultCDPPort
		}

		args := BuildSandboxBrowserCreateArgs(cfg, containerName, sessionKey, workspaceDir, agentWorkspaceDir)

		if _, err := ExecDocker(args, nil); err != nil {
			return nil, fmt.Errorf("creating browser container: %w", err)
		}
		if _, err := ExecDocker([]string{"start", containerName}, nil); err != nil {
			return nil, fmt.Errorf("starting browser container: %w", err)
		}
	} else if !state.Running {
		if _, err := ExecDocker([]string{"start", containerName}, nil); err != nil {
			return nil, fmt.Errorf("starting browser container: %w", err)
		}
	}

	// 读取映射的 CDP 端口
	cdpPort := cfg.Browser.CDPPort
	if cdpPort == 0 {
		cdpPort = DefaultCDPPort
	}
	mappedCDP, err := ReadDockerPort(containerName, cdpPort)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve CDP port mapping for %s: %w", containerName, err)
	}

	// 读取映射的 noVNC 端口
	var mappedNoVNC int
	noVNCPort := cfg.Browser.NoVNCPort
	if noVNCPort == 0 {
		noVNCPort = DefaultNoVNCPort
	}
	if cfg.Browser.EnableNoVNC && !cfg.Browser.Headless {
		if p, err := ReadDockerPort(containerName, noVNCPort); err == nil {
			mappedNoVNC = p
		}
	}

	// 检查已有 bridge 是否可复用
	existing := GetBrowserBridge(sessionKey)
	shouldReuse := existing != nil && existing.ContainerName == containerName && existing.CDPPort == mappedCDP
	if existing != nil && !shouldReuse {
		RemoveBrowserBridge(sessionKey)
	}

	// 等待 CDP 就绪（autoStart 模式）
	if cfg.Browser.AutoStart && !shouldReuse {
		timeoutMs := cfg.Browser.AutoStartTimeoutMs
		if timeoutMs == 0 {
			timeoutMs = DefaultBrowserAutoStartTimeoutMs
		}
		if !WaitForSandboxCDP(mappedCDP, timeoutMs) {
			slog.Warn("sandbox browser CDP did not become reachable",
				"container", containerName,
				"cdpPort", mappedCDP,
				"timeoutMs", timeoutMs)
		}
	}

	// 构建 bridge URL
	bridgeURL := fmt.Sprintf("http://127.0.0.1:%d", mappedCDP)

	// 更新 bridge 注册
	if !shouldReuse {
		SetBrowserBridge(sessionKey, &BrowserBridge{
			ContainerName: containerName,
			BridgeURL:     bridgeURL,
			CDPPort:       mappedCDP,
		})
	}

	// 更新浏览器注册表
	browserRegistryPath := filepath.Join(stateDir, "sandbox", BrowserRegistryFilename)
	now := time.Now().UnixMilli()
	_ = UpdateBrowserRegistryEntry(browserRegistryPath, BrowserRegistryEntry{
		ContainerName: containerName,
		SessionKey:    sessionKey,
		CreatedAtMs:   now,
		LastUsedAtMs:  now,
		Image:         cfg.Browser.Image,
		CDPPort:       mappedCDP,
	})

	// 构造 noVNC URL
	var noVNCURL string
	if mappedNoVNC > 0 && cfg.Browser.EnableNoVNC && !cfg.Browser.Headless {
		noVNCURL = fmt.Sprintf("http://127.0.0.1:%d/vnc.html?autoconnect=1&resize=remote", mappedNoVNC)
	}

	return &SandboxBrowserContext{
		ContainerName: containerName,
		BridgeURL:     bridgeURL,
		NoVNCURL:      noVNCURL,
		CDPPort:       mappedCDP,
	}, nil
}

// BuildSandboxBrowserCreateArgs 构建浏览器容器的 docker create 参数。
// TS 对照: browser.ts ensureSandboxBrowser() 中内联参数构建
func BuildSandboxBrowserCreateArgs(cfg SandboxConfig, containerName, sessionKey, workspaceDir, agentWorkspaceDir string) []string {
	args := BuildCreateArgs(cfg, containerName, sessionKey, "")

	// 去除末尾的 image 和 sleep infinity（由 BuildCreateArgs 添加）
	// 重新构造：browser 容器不需要 sleep infinity
	// 使用 docker run 风格：无 entrypoint override（使用镜像默认）
	cdpPort := cfg.Browser.CDPPort
	if cdpPort == 0 {
		cdpPort = DefaultCDPPort
	}
	vncPort := cfg.Browser.VNCPort
	if vncPort == 0 {
		vncPort = DefaultVNCPort
	}
	noVNCPort := cfg.Browser.NoVNCPort
	if noVNCPort == 0 {
		noVNCPort = DefaultNoVNCPort
	}

	// 重新构建参数（不使用 BuildCreateArgs 的默认行尾）
	args = []string{
		"create",
		"--name", containerName,
		"--label", "openacosmi.sandboxBrowser=1",
		"--label", fmt.Sprintf("openacosmi.session-key=%s", sessionKey),
	}

	// 网络
	if cfg.Docker.Network != "" {
		args = append(args, "--network", cfg.Docker.Network)
	}

	// 工作目录
	workdir := cfg.Docker.Workdir
	if workdir == "" {
		workdir = DefaultWorkdir
	}
	args = append(args, "-w", workdir)

	// workspace volume 挂载
	// TS 对照: browser.ts L113-124
	mountSuffix := ""
	if cfg.Workspace == AccessReadOnly && workspaceDir == agentWorkspaceDir {
		mountSuffix = ":ro"
	}
	if workspaceDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:%s%s", workspaceDir, workdir, mountSuffix))
	}
	if cfg.Workspace != AccessNone && workspaceDir != agentWorkspaceDir && agentWorkspaceDir != "" {
		agentMountSuffix := ""
		if cfg.Workspace == AccessReadOnly {
			agentMountSuffix = ":ro"
		}
		args = append(args, "-v", fmt.Sprintf("%s:%s%s", agentWorkspaceDir, SandboxAgentWorkspaceMount, agentMountSuffix))
	}

	// CDP 端口映射（随机主机端口）
	args = append(args, "-p", fmt.Sprintf("127.0.0.1::%d", cdpPort))

	// noVNC 端口映射（仅非 headless 模式）
	if cfg.Browser.EnableNoVNC && !cfg.Browser.Headless {
		args = append(args, "-p", fmt.Sprintf("127.0.0.1::%d", noVNCPort))
	}

	// 浏览器专用环境变量
	args = append(args, "-e", fmt.Sprintf("OPENACOSMI_BROWSER_HEADLESS=%s", boolToFlag(cfg.Browser.Headless)))
	args = append(args, "-e", fmt.Sprintf("OPENACOSMI_BROWSER_ENABLE_NOVNC=%s", boolToFlag(cfg.Browser.EnableNoVNC)))
	args = append(args, "-e", fmt.Sprintf("OPENACOSMI_BROWSER_CDP_PORT=%d", cdpPort))
	args = append(args, "-e", fmt.Sprintf("OPENACOSMI_BROWSER_VNC_PORT=%d", vncPort))
	args = append(args, "-e", fmt.Sprintf("OPENACOSMI_BROWSER_NOVNC_PORT=%d", noVNCPort))

	// Docker env 环境变量
	for k, v := range cfg.Docker.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// 安全限制
	args = append(args, "--cap-drop", "ALL")

	// 浏览器镜像
	browserImage := cfg.Browser.Image
	if browserImage == "" {
		browserImage = DefaultBrowserImage
	}
	args = append(args, browserImage)

	return args
}

// boolToFlag 将 bool 转为 "1"/"0" 字符串。
func boolToFlag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// extractPortFromDockerOutput 从 docker port 输出中提取端口号。
func extractPortFromDockerOutput(output string) string {
	// 格式: "0.0.0.0:12345\n" 或 "127.0.0.1:12345\n"
	output = strings.TrimSpace(output)
	// 可能有多行（IPv4 + IPv6），取第一行
	if idx := strings.Index(output, "\n"); idx >= 0 {
		output = output[:idx]
	}
	output = strings.TrimSpace(output)
	parts := strings.Split(output, ":")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return ""
}

// ---------- 工作区管理 ----------

// EnsureSandboxWorkspace 确保沙箱工作区目录存在并初始化。
// TS 对照: workspace.ts ensureSandboxWorkspace()
func EnsureSandboxWorkspace(workspaceDir, sourceDir string) error {
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return fmt.Errorf("creating workspace dir: %w", err)
	}

	if sourceDir == "" {
		return nil
	}

	// 从源目录种子初始化（仅当工作区为空时）
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		return nil // 忽略读取错误
	}
	if len(entries) > 0 {
		return nil // 已有内容，跳过种子
	}

	return seedWorkspace(workspaceDir, sourceDir)
}

// seedWorkspace 从源目录复制文件到工作区。
func seedWorkspace(dst, src string) error {
	return filepath.WalkDir(src, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		relPath, err := filepath.Rel(src, srcPath)
		if err != nil {
			return nil
		}
		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			return nil // 跳过无法读取的文件
		}

		return os.WriteFile(dstPath, data, 0o644)
	})
}
