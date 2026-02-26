package gateway

import "testing"

func TestCheckBrowserOrigin_MissingOrigin(t *testing.T) {
	result := CheckBrowserOrigin("example.com", "", nil)
	if result.OK {
		t.Fatal("expected rejection for missing origin")
	}
	if result.Reason != "origin missing or invalid" {
		t.Fatalf("unexpected reason: %s", result.Reason)
	}
}

func TestCheckBrowserOrigin_NullOrigin(t *testing.T) {
	result := CheckBrowserOrigin("example.com", "null", nil)
	if result.OK {
		t.Fatal("expected rejection for null origin")
	}
}

func TestCheckBrowserOrigin_AllowlistMatch(t *testing.T) {
	result := CheckBrowserOrigin("other.com", "https://example.com", []string{"https://example.com"})
	if !result.OK {
		t.Fatalf("expected acceptance for allowlisted origin, got reason=%s", result.Reason)
	}
}

func TestCheckBrowserOrigin_AllowlistCaseInsensitive(t *testing.T) {
	result := CheckBrowserOrigin("other.com", "https://Example.COM", []string{"https://example.com"})
	if !result.OK {
		t.Fatalf("expected case-insensitive allowlist match, got reason=%s", result.Reason)
	}
}

func TestCheckBrowserOrigin_RequestHostMatch(t *testing.T) {
	result := CheckBrowserOrigin("example.com:8080", "https://example.com:8080/path", nil)
	if !result.OK {
		t.Fatalf("expected acceptance when origin host matches request host, got reason=%s", result.Reason)
	}
}

func TestCheckBrowserOrigin_LoopbackBothSides(t *testing.T) {
	result := CheckBrowserOrigin("localhost:3000", "http://127.0.0.1:5000", nil)
	if !result.OK {
		t.Fatalf("expected acceptance for dual loopback, got reason=%s", result.Reason)
	}
}

func TestCheckBrowserOrigin_LoopbackIPv6(t *testing.T) {
	result := CheckBrowserOrigin("[::1]:3000", "http://localhost:5000", nil)
	if !result.OK {
		t.Fatalf("expected acceptance for IPv6 loopback vs localhost, got reason=%s", result.Reason)
	}
}

func TestCheckBrowserOrigin_Rejected(t *testing.T) {
	result := CheckBrowserOrigin("api.example.com", "https://evil.com", []string{"https://safe.com"})
	if result.OK {
		t.Fatal("expected rejection for non-matching origin")
	}
	if result.Reason != "origin not allowed" {
		t.Fatalf("unexpected reason: %s", result.Reason)
	}
}

func TestCheckBrowserOrigin_InvalidOriginURL(t *testing.T) {
	result := CheckBrowserOrigin("example.com", "not-a-url", nil)
	if result.OK {
		t.Fatal("expected rejection for invalid origin URL")
	}
}

func TestIsLoopbackHost(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"::1", true},
		{"", false},
		{"example.com", false},
		{"192.168.1.1", false},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			got := isLoopbackHost(tc.host)
			if got != tc.want {
				t.Errorf("isLoopbackHost(%q) = %v, want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestResolveHostName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"example.com:8080", "example.com"},
		{"[::1]:3000", "::1"},
		{"localhost", "localhost"},
		{"", ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			got := resolveHostName(tc.input)
			if got != tc.want {
				t.Errorf("resolveHostName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
