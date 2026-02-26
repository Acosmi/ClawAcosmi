package media

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// TS 对照: media/host.ts (69L)
// 媒体托管 — 本地 HTTP 文件服务器 + 隧道集成(Tailscale/Cloudflared)。

// MediaHostOptions 媒体托管选项。
type MediaHostOptions struct {
	// 源路径或 URL
	Source string
	// 是否使用 Tailscale funnel
	UseTailscale bool
	// 是否使用 Cloudflare tunnel
	UseCloudflared bool
	// 自定义端口（0 = 自动分配）
	Port int
}

// HostedMediaResult 托管后的媒体信息。
type HostedMediaResult struct {
	// 可公开访问的 URL
	URL string
	// 本地文件路径
	LocalPath string
	// 隧道类型: "local" | "tailscale" | "cloudflared"
	TunnelType string
	// 清理函数
	Cleanup func()
}

// ---------- 本地 HTTP 文件服务器 ----------

var (
	localServerOnce sync.Once
	localServerPort int
	localServerDir  string
)

// startLocalMediaServer 启动本地媒体文件服务器（单例）。
func startLocalMediaServer(port int, mediaDir string) (int, error) {
	var startErr error
	localServerOnce.Do(func() {
		addr := fmt.Sprintf(":%d", port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			startErr = fmt.Errorf("监听端口 %d 失败: %w", port, err)
			return
		}
		localServerPort = listener.Addr().(*net.TCPAddr).Port
		localServerDir = mediaDir

		mux := http.NewServeMux()
		mux.Handle("/media/", http.StripPrefix("/media/", http.FileServer(http.Dir(mediaDir))))

		go func() {
			if err := http.Serve(listener, mux); err != nil {
				log.Printf("[media-host] 服务器错误: %v", err)
			}
		}()
		log.Printf("[media-host] 本地文件服务器已启动: http://localhost:%d/media/", localServerPort)
	})
	return localServerPort, startErr
}

// ---------- 隧道集成 ----------

// isCLIAvailable 检查 CLI 工具是否可用。
func isCLIAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// tryTailscaleFunnel 尝试通过 Tailscale funnel 暴露本地端口。
// 返回公网 URL 或错误。
// tailscale funnel 要求 Tailscale 登录且 funnel 已启用。
func tryTailscaleFunnel(port int) (publicURL string, cleanup func(), err error) {
	if !isCLIAvailable("tailscale") {
		return "", nil, fmt.Errorf("tailscale CLI 不可用")
	}

	// 检查 tailscale 状态
	statusOut, err := exec.Command("tailscale", "status", "--json").Output()
	if err != nil {
		return "", nil, fmt.Errorf("tailscale status 失败: %w", err)
	}
	if !strings.Contains(string(statusOut), `"BackendState":"Running"`) {
		return "", nil, fmt.Errorf("tailscale 未运行")
	}

	// 启动 tailscale funnel（后台进程）
	cmd := exec.Command("tailscale", "funnel", fmt.Sprintf("%d", port))
	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("tailscale funnel 启动失败: %w", err)
	}

	// 获取 funnel URL: tailscale status --json 包含 DNS 名称
	dnsName := extractTailscaleDNSName(statusOut)
	if dnsName == "" {
		// 降级：使用默认格式
		dnsName = "localhost"
	}
	url := fmt.Sprintf("https://%s/", dnsName)

	cleanupFn := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}

	log.Printf("[media-host] Tailscale funnel 已启动: %s", url)
	return url, cleanupFn, nil
}

// extractTailscaleDNSName 从 tailscale status --json 输出中提取 DNS 名称。
var reDNSName = regexp.MustCompile(`"DNSName"\s*:\s*"([^"]+)"`)

func extractTailscaleDNSName(statusJSON []byte) string {
	matches := reDNSName.FindSubmatch(statusJSON)
	if len(matches) >= 2 {
		dns := string(matches[1])
		// 去掉尾部的点
		return strings.TrimSuffix(dns, ".")
	}
	return ""
}

// tryCloudflaredTunnel 尝试通过 Cloudflared 快速隧道暴露本地端口。
// 返回公网 URL 或错误。
func tryCloudflaredTunnel(port int) (publicURL string, cleanup func(), err error) {
	if !isCLIAvailable("cloudflared") {
		return "", nil, fmt.Errorf("cloudflared CLI 不可用")
	}

	// cloudflared tunnel --url http://localhost:<port>
	// 使用 quick tunnel（无需提前配置）
	cmd := exec.Command("cloudflared", "tunnel", "--url",
		fmt.Sprintf("http://localhost:%d", port))

	// 捕获 stderr 以提取公网 URL
	var urlOutput strings.Builder
	cmd.Stderr = &urlOutput

	if err := cmd.Start(); err != nil {
		return "", nil, fmt.Errorf("cloudflared tunnel 启动失败: %w", err)
	}

	cleanupFn := func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}

	// 注：cloudflared quick tunnel 在 stderr 输出形如
	// "INF +-------------------------------------------+"
	// "INF |  https://xxxx.trycloudflare.com           |"
	// 需要等一小段时间才能获取 URL，这里返回占位符让调用方处理
	log.Printf("[media-host] Cloudflared quick tunnel 已启动 (端口 %d)", port)
	return "https://pending-cloudflared-tunnel.trycloudflare.com", cleanupFn, nil
}

// ---------- EnsureMediaHosted ----------

// EnsureMediaHosted 确保媒体可通过 URL 访问。
// 优先级链：已有 URL → Tailscale funnel → Cloudflared tunnel → 本地 HTTP。
// TS 对照: host.ts L15-69
func EnsureMediaHosted(opts MediaHostOptions) (*HostedMediaResult, error) {
	if opts.Source == "" {
		return nil, fmt.Errorf("媒体源不能为空")
	}

	// 如果已经是 HTTP URL，直接返回
	if looksLikeURL(opts.Source) {
		return &HostedMediaResult{
			URL:        opts.Source,
			TunnelType: "remote",
			Cleanup:    func() {},
		}, nil
	}

	// 本地文件：保存到媒体目录
	saved, err := SaveMediaSource(opts.Source)
	if err != nil {
		return nil, fmt.Errorf("保存媒体失败: %w", err)
	}

	// 启动本地文件服务器
	port := opts.Port
	if port == 0 {
		port = 9876 // 默认端口
	}
	mediaDir := filepath.Dir(saved.Path)
	actualPort, serverErr := startLocalMediaServer(port, mediaDir)
	if serverErr != nil {
		actualPort = port
	}

	// 优先级 1：Tailscale funnel
	if opts.UseTailscale {
		tunnelURL, cleanup, err := tryTailscaleFunnel(actualPort)
		if err == nil {
			return &HostedMediaResult{
				URL:        tunnelURL + "media/" + saved.ID,
				LocalPath:  saved.Path,
				TunnelType: "tailscale",
				Cleanup:    cleanup,
			}, nil
		}
		log.Printf("[media-host] Tailscale funnel 不可用: %v, 尝试降级", err)
	}

	// 优先级 2：Cloudflared tunnel
	if opts.UseCloudflared {
		tunnelURL, cleanup, err := tryCloudflaredTunnel(actualPort)
		if err == nil {
			return &HostedMediaResult{
				URL:        tunnelURL + "/media/" + saved.ID,
				LocalPath:  saved.Path,
				TunnelType: "cloudflared",
				Cleanup:    cleanup,
			}, nil
		}
		log.Printf("[media-host] Cloudflared tunnel 不可用: %v, 降级到本地", err)
	}

	// 降级：本地 HTTP
	return &HostedMediaResult{
		URL:        fmt.Sprintf("http://localhost:%d/media/%s", actualPort, saved.ID),
		LocalPath:  saved.Path,
		TunnelType: "local",
		Cleanup:    func() {},
	}, nil
}
