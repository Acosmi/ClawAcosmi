package media

import (
	"testing"
)

// ---------- isCLIAvailable ----------

func TestIsCLIAvailable_Exists(t *testing.T) {
	// "ls" should always be available on macOS/Linux
	if !isCLIAvailable("ls") {
		t.Error("expected ls to be available")
	}
}

func TestIsCLIAvailable_NotExists(t *testing.T) {
	if isCLIAvailable("nonexistent-tool-xyz-12345") {
		t.Error("expected nonexistent tool to not be available")
	}
}

// ---------- extractTailscaleDNSName ----------

func TestExtractTailscaleDNSName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "valid DNS name",
			input: `{"Self":{"DNSName":"myhost.tail12345.ts.net.","HostName":"myhost"}}`,
			want:  "myhost.tail12345.ts.net",
		},
		{
			name:  "no DNS name",
			input: `{"Status":"Running"}`,
			want:  "",
		},
		{
			name:  "empty json",
			input: `{}`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTailscaleDNSName([]byte(tt.input))
			if got != tt.want {
				t.Errorf("extractTailscaleDNSName = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------- EnsureMediaHosted ----------

func TestEnsureMediaHosted_URLPassthrough(t *testing.T) {
	result, err := EnsureMediaHosted(MediaHostOptions{
		Source: "https://example.com/image.jpg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "https://example.com/image.jpg" {
		t.Errorf("URL = %q, want https://example.com/image.jpg", result.URL)
	}
	if result.TunnelType != "remote" {
		t.Errorf("TunnelType = %q, want remote", result.TunnelType)
	}
}

func TestEnsureMediaHosted_EmptySource(t *testing.T) {
	_, err := EnsureMediaHosted(MediaHostOptions{})
	if err == nil {
		t.Error("expected error for empty source")
	}
}
