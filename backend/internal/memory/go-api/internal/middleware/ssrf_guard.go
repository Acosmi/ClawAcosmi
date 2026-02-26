// Package middleware — SSRF 防护辅助函数。
// P2-3: 防止 DB 连接测试被用于内网端口扫描。
package middleware

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// privateRanges 是需要拦截的内网 CIDR 列表。
var privateRanges = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.0/8",
	"169.254.0.0/16", // link-local
	"::1/128",        // IPv6 loopback
	"fc00::/7",       // IPv6 unique local
	"fe80::/10",      // IPv6 link-local
}

var privateCIDRs []*net.IPNet

func init() {
	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			privateCIDRs = append(privateCIDRs, network)
		}
	}
}

// ValidateDSNSafety 校验 DSN 是否安全（非内网地址）。
// 返回 nil 表示安全，返回 error 表示被拦截。
// 支持 postgres://, mysql://, sqlite:// 格式。
func ValidateDSNSafety(dsn string) error {
	// sqlite 使用本地文件，不涉及网络
	if strings.HasPrefix(dsn, "file:") || strings.HasPrefix(dsn, "sqlite") || !strings.Contains(dsn, "://") {
		return nil
	}

	u, err := url.Parse(dsn)
	if err != nil {
		return fmt.Errorf("invalid DSN format: %w", err)
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("DSN missing host")
	}

	// 禁止 localhost 别名
	lower := strings.ToLower(host)
	if lower == "localhost" || lower == "ip6-localhost" || lower == "ip6-loopback" {
		return fmt.Errorf("connection to localhost is not allowed")
	}

	// 解析 IP（如果是域名会走 DNS 解析）
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve host %q: %w", host, err)
	}

	for _, ip := range ips {
		for _, cidr := range privateCIDRs {
			if cidr.Contains(ip) {
				return fmt.Errorf("connection to private network address %s (%s) is not allowed", host, ip)
			}
		}
	}

	return nil
}
