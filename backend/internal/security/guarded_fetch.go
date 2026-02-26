package security

// TS 对照: 无直接单文件对应，但 TS 中通过 ssrf.ts 的 fetch + createPinnedDispatcher 组合实现
// GuardedFetch — 高层 SSRF 安全 HTTP 请求封装。
// 整合 DNS pinning + SSRF 检测 + 超时 + 重定向控制于一体。

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// GuardedFetchOptions GuardedFetch 选项。
type GuardedFetchOptions struct {
	// Method HTTP 方法（默认 GET）。
	Method string
	// Headers 额外请求头。
	Headers map[string]string
	// Body 请求体（仅 POST/PUT/PATCH）。
	Body io.Reader
	// Timeout 请求超时（默认 30s）。
	Timeout time.Duration
	// Policy SSRF 策略（nil 表示默认策略）。
	Policy *SsrfPolicy
	// MaxResponseBytes 最大响应体大小（0 表示不限制）。
	MaxResponseBytes int64
}

// GuardedFetchResult GuardedFetch 结果。
type GuardedFetchResult struct {
	Response   *http.Response
	StatusCode int
}

// GuardedFetch 执行带 SSRF 防护的安全 HTTP 请求。
// 使用 CreatePinnedHTTPClient 确保 DNS pinning，防止 DNS rebinding。
// 每次重定向目标都检查私有 IP 和阻止主机名。
func GuardedFetch(ctx context.Context, rawURL string, opts *GuardedFetchOptions) (*GuardedFetchResult, error) {
	if opts == nil {
		opts = &GuardedFetchOptions{}
	}

	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultSafeFetchTimeout
	}

	policy := opts.Policy
	allowPrivate := policy != nil && policy.AllowPrivateNetwork

	// 预检：URL 解析 + 主机名/IP 检查
	if !allowPrivate {
		if err := validateURLForSSRF(rawURL, policy); err != nil {
			return nil, err
		}
	}

	// 使用 DNS pinning 客户端
	client := CreatePinnedHTTPClient(timeout)

	// 构建请求
	req, err := http.NewRequestWithContext(ctx, method, rawURL, opts.Body)
	if err != nil {
		return nil, fmt.Errorf("guarded_fetch: invalid request: %w", err)
	}

	// 设置请求头
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("guarded_fetch: request failed: %w", err)
	}

	// 限制响应体大小
	if opts.MaxResponseBytes > 0 && resp.Body != nil {
		resp.Body = http.MaxBytesReader(nil, resp.Body, opts.MaxResponseBytes)
	}

	return &GuardedFetchResult{
		Response:   resp,
		StatusCode: resp.StatusCode,
	}, nil
}

// validateURLForSSRF 预检 URL 是否通过 SSRF 检查（不发送请求）。
func validateURLForSSRF(rawURL string, policy *SsrfPolicy) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("guarded_fetch: invalid URL: %w", err)
	}

	hostname := u.Hostname()
	if hostname == "" {
		return &SsrfBlockedError{Message: "guarded_fetch: empty hostname"}
	}

	isExplicitAllowed := policy != nil && isHostnameAllowed(hostname, policy.AllowedHostnames)
	if !isExplicitAllowed {
		if IsBlockedHostname(hostname) {
			return &SsrfBlockedError{Message: fmt.Sprintf("guarded_fetch: blocked hostname: %s", hostname)}
		}
		if IsPrivateIP(hostname) {
			return &SsrfBlockedError{Message: "guarded_fetch: blocked private IP"}
		}
	}

	return nil
}
