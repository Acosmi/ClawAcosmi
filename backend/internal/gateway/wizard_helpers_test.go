package gateway

import (
	"strings"
	"testing"
)

func TestRandomToken(t *testing.T) {
	tok := RandomToken()
	if len(tok) != 48 { // 24 bytes → 48 hex chars
		t.Fatalf("expected 48 chars, got %d", len(tok))
	}
	tok2 := RandomToken()
	if tok == tok2 {
		t.Fatal("two tokens should not be equal")
	}
}

func TestNormalizeGatewayTokenInput(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  abc  ", "abc"},
		{"", ""},
		{"  ", ""},
		{"token123", "token123"},
	}
	for _, c := range cases {
		got := NormalizeGatewayTokenInput(c.in)
		if got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsValidIPv4_Helpers(t *testing.T) {
	valid := []string{"192.168.1.1", "0.0.0.0", "255.255.255.255", "10.0.0.1"}
	for _, ip := range valid {
		if !IsValidIPv4(ip) {
			t.Errorf("expected %q valid", ip)
		}
	}
	invalid := []string{"256.1.1.1", "01.1.1.1", "1.1.1", "abc", ""}
	for _, ip := range invalid {
		if IsValidIPv4(ip) {
			t.Errorf("expected %q invalid", ip)
		}
	}
}

func TestResolveControlUiLinks_Loopback(t *testing.T) {
	links := ResolveControlUiLinks(ResolveControlUiLinksParams{
		Port: 19001,
		Bind: "loopback",
	})
	if !strings.Contains(links.HttpURL, "127.0.0.1:19001") {
		t.Errorf("expected loopback URL, got %s", links.HttpURL)
	}
	if !strings.HasPrefix(links.WsURL, "ws://127.0.0.1") {
		t.Errorf("unexpected ws URL: %s", links.WsURL)
	}
}

func TestResolveControlUiLinks_Custom(t *testing.T) {
	links := ResolveControlUiLinks(ResolveControlUiLinksParams{
		Port:           8080,
		Bind:           "custom",
		CustomBindHost: "10.0.0.5",
	})
	if !strings.Contains(links.HttpURL, "10.0.0.5:8080") {
		t.Errorf("expected custom host, got %s", links.HttpURL)
	}
}

func TestResolveControlUiLinks_BasePath(t *testing.T) {
	links := ResolveControlUiLinks(ResolveControlUiLinksParams{
		Port:     19001,
		Bind:     "loopback",
		BasePath: "/ui",
	})
	if !strings.Contains(links.HttpURL, "/ui/") {
		t.Errorf("expected /ui/ in URL, got %s", links.HttpURL)
	}
	if !strings.Contains(links.WsURL, "/ui") {
		t.Errorf("expected /ui in WS URL, got %s", links.WsURL)
	}
}

func TestNormalizeControlUiBasePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"/", ""},
		{"ui", "/ui"},
		{"/ui/", "/ui"},
		{"/ui", "/ui"},
	}
	for _, c := range cases {
		got := normalizeControlUiBasePath(c.in)
		if got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatControlUiSshHint(t *testing.T) {
	hint := FormatControlUiSshHint(19001, "", "tok123")
	if !strings.Contains(hint, "19001") {
		t.Error("expected port in hint")
	}
	if !strings.Contains(hint, "tok123") {
		t.Error("expected token in hint")
	}
}
