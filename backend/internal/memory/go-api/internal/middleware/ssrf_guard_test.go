// Package middleware — SSRF 防护单元测试。
package middleware

import (
	"testing"
)

func TestValidateDSNSafety_SafeAddresses(t *testing.T) {
	safeDSNs := []string{
		"postgres://user:pass@8.8.8.8:5432/mydb", // public IP
		"mysql://user:pass@1.1.1.1:3306/mydb",    // public IP
		"file:./local.db",
		"sqlite:///tmp/test.db",
		"my-local-file.db", // no scheme = sqlite-like
	}
	for _, dsn := range safeDSNs {
		if err := ValidateDSNSafety(dsn); err != nil {
			t.Errorf("ValidateDSNSafety(%q) = %v, want nil", dsn, err)
		}
	}
}

func TestValidateDSNSafety_BlockedAddresses(t *testing.T) {
	blockedDSNs := []struct {
		dsn    string
		reason string
	}{
		{"postgres://user:pass@localhost:5432/db", "localhost"},
		{"postgres://user:pass@127.0.0.1:5432/db", "loopback IPv4"},
		{"mysql://user:pass@10.0.0.1:3306/db", "private 10.x"},
		{"mysql://user:pass@172.16.0.1:3306/db", "private 172.16.x"},
		{"postgres://user:pass@192.168.1.1:5432/db", "private 192.168.x"},
		{"postgres://user:pass@ip6-localhost:5432/db", "IPv6 localhost alias"},
	}
	for _, tt := range blockedDSNs {
		err := ValidateDSNSafety(tt.dsn)
		if err == nil {
			t.Errorf("ValidateDSNSafety(%q) = nil, want error (%s)", tt.dsn, tt.reason)
		}
	}
}

func TestValidateDSNSafety_InvalidDSN(t *testing.T) {
	// DSN with no host should error.
	err := ValidateDSNSafety("postgres://:5432/db")
	if err == nil {
		t.Errorf("ValidateDSNSafety with missing host should error")
	}
}

func TestValidateDSNSafety_SqliteAlwaysSafe(t *testing.T) {
	// sqlite DSNs should always pass (local file, no network).
	sqliteDSNs := []string{
		"file:test.db?mode=memory",
		"sqlite://./data.db",
		"sqlite:///absolute/path.db",
	}
	for _, dsn := range sqliteDSNs {
		if err := ValidateDSNSafety(dsn); err != nil {
			t.Errorf("ValidateDSNSafety(%q) unexpectedly blocked: %v", dsn, err)
		}
	}
}
