package telegram

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
)

// Telegram HTTP 客户端 — 合并 proxy.ts + fetch.ts
// grammy 框架在 Go 中无直接等价物，通过标准 net/http 客户端
// + HTTP 代理 + 超时配置替代。

// HTTPClientConfig HTTP 客户端配置
type HTTPClientConfig struct {
	ProxyURL       string
	TimeoutSeconds int
	Network        *TelegramNetworkConfig
}

// TelegramAPIBaseURL Telegram Bot API 基础 URL
const TelegramAPIBaseURL = "https://api.telegram.org"

// NewTelegramHTTPClient 创建 Telegram Bot API HTTP 客户端。
func NewTelegramHTTPClient(cfg HTTPClientConfig) (*http.Client, error) {
	timeout := 30 * time.Second
	if cfg.TimeoutSeconds > 0 {
		timeout = time.Duration(cfg.TimeoutSeconds) * time.Second
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if proxyURL := cfg.ProxyURL; proxyURL != "" {
		parsedURL, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", proxyURL, err)
		}
		if parsedURL.Scheme == "socks5" || parsedURL.Scheme == "socks5h" {
			// SOCKS5 代理：使用 golang.org/x/net/proxy
			socksDialer, err := proxy.FromURL(parsedURL, proxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("create SOCKS5 dialer for %q: %w", proxyURL, err)
			}
			// 包装 Dial 为 DialContext（proxy.Dialer 不直接支持 context）
			transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
				// 如果 dialer 支持 ContextDialer 接口，优先使用
				if cd, ok := socksDialer.(proxy.ContextDialer); ok {
					return cd.DialContext(ctx, network, addr)
				}
				return socksDialer.Dial(network, addr)
			}
			slog.Info("telegram: using SOCKS5 proxy", "url", parsedURL.Host)
		} else {
			// HTTP/HTTPS 代理
			transport.Proxy = http.ProxyURL(parsedURL)
		}
	}

	// 应用 autoSelectFamily 决策（Go 默认 Happy Eyeballs，此处仅记录）
	decision := ResolveAutoSelectFamilyDecision(cfg.Network)
	if decision.Value != nil {
		slog.Info("telegram: network family decision",
			"autoSelectFamily", *decision.Value,
			"source", decision.Source)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

// NewDefaultTelegramHTTPClient 使用账户配置创建 HTTP 客户端。
func NewDefaultTelegramHTTPClient(account ResolvedTelegramAccount) (*http.Client, error) {
	cfg := HTTPClientConfig{
		ProxyURL: account.Config.Proxy,
	}
	if account.Config.TimeoutSeconds != nil {
		cfg.TimeoutSeconds = *account.Config.TimeoutSeconds
	}
	if account.Config.Network != nil {
		cfg.Network = &TelegramNetworkConfig{
			AutoSelectFamily: account.Config.Network.AutoSelectFamily,
		}
	}
	return NewTelegramHTTPClient(cfg)
}
