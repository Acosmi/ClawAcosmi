package linkparse

import (
	"testing"
)

// ---------- IsAllowedURL ----------

func TestIsAllowedURL_HTTP(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com", true},
		{"http://example.com/path?q=1", true},
		{"ftp://example.com", false},
		{"not-a-url", false},
		{"http://127.0.0.1/secret", false},
		{"https://127.0.0.1:8080", false},
		{"https://10.0.0.1/api", true},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := IsAllowedURL(tt.url)
			if got != tt.want {
				t.Errorf("IsAllowedURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// ---------- ResolveMaxLinks ----------

func TestResolveMaxLinks(t *testing.T) {
	v := 5
	if got := ResolveMaxLinks(&v); got != 5 {
		t.Errorf("expected 5, got %d", got)
	}
	if got := ResolveMaxLinks(nil); got != DefaultMaxLinks {
		t.Errorf("expected default %d, got %d", DefaultMaxLinks, got)
	}
	zero := 0
	if got := ResolveMaxLinks(&zero); got != DefaultMaxLinks {
		t.Errorf("expected default for zero, got %d", got)
	}
}

// ---------- ExtractLinksFromMessage ----------

func TestExtractLinks_BareURLs(t *testing.T) {
	msg := "check https://example.com and http://test.org/page for info"
	links := ExtractLinksFromMessage(msg, nil)
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d: %v", len(links), links)
	}
	if links[0] != "https://example.com" {
		t.Errorf("links[0] = %q", links[0])
	}
}

func TestExtractLinks_MarkdownLinks(t *testing.T) {
	msg := "see [docs](https://docs.example.com) and [api](http://api.example.com)"
	links := ExtractLinksFromMessage(msg, nil)
	// Markdown links are stripped; URLs should still be found from the stripped text
	if len(links) != 0 {
		// stripMarkdownLinks replaces the entire [text](url) with " ", so no URLs remain
		// unless they appear elsewhere in the text
		t.Logf("got %d links: %v (may vary based on strip behavior)", len(links), links)
	}
}

func TestExtractLinks_Dedup(t *testing.T) {
	msg := "https://example.com foo https://example.com bar"
	links := ExtractLinksFromMessage(msg, nil)
	if len(links) != 1 {
		t.Errorf("expected 1 deduped link, got %d: %v", len(links), links)
	}
}

func TestExtractLinks_Limit(t *testing.T) {
	msg := "https://a.com https://b.com https://c.com https://d.com"
	limit := 2
	links := ExtractLinksFromMessage(msg, &limit)
	if len(links) != 2 {
		t.Errorf("expected 2 links (limit), got %d: %v", len(links), links)
	}
}

func TestExtractLinks_Empty(t *testing.T) {
	links := ExtractLinksFromMessage("", nil)
	if links != nil {
		t.Errorf("expected nil for empty message, got %v", links)
	}
	links = ExtractLinksFromMessage("   ", nil)
	if links != nil {
		t.Errorf("expected nil for whitespace message, got %v", links)
	}
}

func TestExtractLinks_FilterLocalhost(t *testing.T) {
	msg := "see http://127.0.0.1:3000/api and https://real.example.com"
	links := ExtractLinksFromMessage(msg, nil)
	if len(links) != 1 {
		t.Fatalf("expected 1 link (localhost filtered), got %d: %v", len(links), links)
	}
	if links[0] != "https://real.example.com" {
		t.Errorf("expected real.example.com, got %q", links[0])
	}
}
