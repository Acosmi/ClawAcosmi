package security

import "testing"

func TestIsPrivateIP_IPv4Private(t *testing.T) {
	privates := []string{
		"10.0.0.1",
		"10.255.255.255",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.0.1",
		"192.168.255.255",
		"127.0.0.1",
		"127.255.255.255",
		"169.254.0.1",
		"0.0.0.0",
		"100.64.0.1",
		"100.127.255.255",
	}
	for _, ip := range privates {
		if !IsPrivateIP(ip) {
			t.Errorf("expected %q to be private", ip)
		}
	}
}

func TestIsPrivateIP_IPv4Public(t *testing.T) {
	publics := []string{
		"8.8.8.8",
		"1.1.1.1",
		"203.0.113.1",
		"100.128.0.1",
		"172.32.0.1",
		"172.15.0.1",
		"192.169.0.1",
	}
	for _, ip := range publics {
		if IsPrivateIP(ip) {
			t.Errorf("expected %q to be public", ip)
		}
	}
}

func TestIsPrivateIP_IPv6(t *testing.T) {
	privates := []string{
		"::1",
		"::",
		"fe80::1",
		"fd00::1",
		"fc00::1",
		"fec0::1",
	}
	for _, ip := range privates {
		if !IsPrivateIP(ip) {
			t.Errorf("expected %q to be private IPv6", ip)
		}
	}

	publics := []string{
		"2001:db8::1",
		"2607:f8b0:4004:800::200e",
	}
	for _, ip := range publics {
		if IsPrivateIP(ip) {
			t.Errorf("expected %q to be public IPv6", ip)
		}
	}
}

func TestIsPrivateIP_IPv4MappedIPv6(t *testing.T) {
	cases := []struct {
		addr    string
		private bool
	}{
		{"::ffff:10.0.0.1", true},
		{"::ffff:192.168.1.1", true},
		{"::ffff:127.0.0.1", true},
		{"::ffff:8.8.8.8", false},
		{"::ffff:1.1.1.1", false},
	}
	for _, tc := range cases {
		got := IsPrivateIP(tc.addr)
		if got != tc.private {
			t.Errorf("IsPrivateIP(%q) = %v, want %v", tc.addr, got, tc.private)
		}
	}
}

func TestIsPrivateIP_Brackets(t *testing.T) {
	if !IsPrivateIP("[::1]") {
		t.Error("expected [::1] to be private")
	}
	if !IsPrivateIP("[::ffff:10.0.0.1]") {
		t.Error("expected [::ffff:10.0.0.1] to be private")
	}
}

func TestIsPrivateIP_InvalidInput(t *testing.T) {
	invalids := []string{"", "not-an-ip", "abc.def.ghi.jkl"}
	for _, s := range invalids {
		if IsPrivateIP(s) {
			t.Errorf("expected %q to not be recognized as private", s)
		}
	}
}

func TestIsBlockedHostname(t *testing.T) {
	blocked := []string{
		"localhost",
		"LOCALHOST",
		"foo.localhost",
		"bar.local",
		"metadata.google.internal",
		"some.internal",
	}
	for _, h := range blocked {
		if !IsBlockedHostname(h) {
			t.Errorf("expected %q to be blocked", h)
		}
	}

	allowed := []string{
		"example.com",
		"api.openai.com",
		"google.com",
		"local.example.com",
	}
	for _, h := range allowed {
		if IsBlockedHostname(h) {
			t.Errorf("expected %q to NOT be blocked", h)
		}
	}
}

func TestIsBlockedHostname_Empty(t *testing.T) {
	if IsBlockedHostname("") {
		t.Error("expected empty string to NOT be blocked")
	}
}

func TestNormalizeHostname(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Example.COM.", "example.com"},
		{"[::1]", "::1"},
		{"  HOST  ", "host"},
	}
	for _, tc := range cases {
		got := normalizeHostname(tc.input)
		if got != tc.expected {
			t.Errorf("normalizeHostname(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestIsHostnameAllowed(t *testing.T) {
	allowed := []string{"api.openai.com", "Example.com"}
	if !isHostnameAllowed("api.openai.com", allowed) {
		t.Error("expected api.openai.com to be allowed")
	}
	if !isHostnameAllowed("EXAMPLE.COM", allowed) {
		t.Error("expected EXAMPLE.COM to be allowed (case-insensitive)")
	}
	if isHostnameAllowed("evil.com", allowed) {
		t.Error("expected evil.com to NOT be allowed")
	}
	if isHostnameAllowed("test.com", nil) {
		t.Error("expected nil allowlist to not allow anything")
	}
}

func TestSafeFetchURL_BlocksPrivateIP(t *testing.T) {
	_, err := SafeFetchURL("http://10.0.0.1/secret", nil)
	if err == nil {
		t.Fatal("expected error for private IP")
	}
	if _, ok := err.(*SsrfBlockedError); !ok {
		t.Errorf("expected SsrfBlockedError, got %T: %v", err, err)
	}
}

func TestSafeFetchURL_BlocksLocalhost(t *testing.T) {
	_, err := SafeFetchURL("http://localhost/path", nil)
	if err == nil {
		t.Fatal("expected error for localhost")
	}
	if _, ok := err.(*SsrfBlockedError); !ok {
		t.Errorf("expected SsrfBlockedError, got %T: %v", err, err)
	}
}

func TestSafeFetchURL_AllowsWithPolicy(t *testing.T) {
	policy := &SsrfPolicy{AllowPrivateNetwork: true}
	// This will fail due to connection refused, but should NOT be an SsrfBlockedError
	_, err := SafeFetchURL("http://127.0.0.1:1/test", policy)
	if err == nil {
		return // unlikely but acceptable
	}
	if _, ok := err.(*SsrfBlockedError); ok {
		t.Error("should not be blocked when AllowPrivateNetwork is true")
	}
}

func TestSafeFetchURL_InvalidURL(t *testing.T) {
	_, err := SafeFetchURL("://invalid", nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}
