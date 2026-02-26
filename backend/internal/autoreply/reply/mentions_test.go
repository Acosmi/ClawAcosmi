package reply

import (
	"regexp"
	"testing"
)

func TestStripStructuralPrefixes(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"empty", "", ""},
		{"plain text", "hello world", "hello world"},
		{"with bracket label", "[12:34] hello", "hello"},
		{"with sender prefix", "Alice: hello", "hello"},
		{"with marker", "[Chat messages since your last reply - for context]\nAlice: old msg\n\n[Current message - respond to this]\n/think high", "/think high"},
		{"multiple brackets", "[foo] [bar] text", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StripStructuralPrefixes(tt.input)
			if result != tt.expect {
				t.Errorf("StripStructuralPrefixes(%q) = %q, want %q", tt.input, result, tt.expect)
			}
		})
	}
}

func TestNormalizeMentionText(t *testing.T) {
	if NormalizeMentionText("Hello\u200bWorld") != "helloworld" {
		t.Error("failed to strip zero-width and lowercase")
	}
	if NormalizeMentionText("") != "" {
		t.Error("empty should stay empty")
	}
}

func TestMatchesMentionPatterns(t *testing.T) {
	re := regexp.MustCompile(`(?i)\bbot\b`)
	if !MatchesMentionPatterns("hello @bot", []*regexp.Regexp{re}) {
		t.Error("should match bot mention")
	}
	if MatchesMentionPatterns("hello world", []*regexp.Regexp{re}) {
		t.Error("should not match without bot")
	}
	if MatchesMentionPatterns("", []*regexp.Regexp{re}) {
		t.Error("empty should not match")
	}
}

func TestMatchesMentionWithExplicit(t *testing.T) {
	re := regexp.MustCompile(`(?i)\bbot\b`)
	// 显式提及
	if !MatchesMentionWithExplicit("hello", nil, &ExplicitMentionSignal{IsExplicitlyMentioned: true, CanResolveExplicit: true, HasAnyMention: true}) {
		t.Error("explicit mention should match")
	}
	// 模式匹配
	if !MatchesMentionWithExplicit("@bot", []*regexp.Regexp{re}, nil) {
		t.Error("pattern mention should match")
	}
}

func TestStripMentions(t *testing.T) {
	result := StripMentions("hello @bot world @123456789", []string{`@?bot\b`})
	if result != "hello world" {
		t.Errorf("StripMentions = %q, want %q", result, "hello world")
	}
}

func TestBuildMentionRegexes(t *testing.T) {
	regexes := BuildMentionRegexes([]string{`\bbot\b`, `(invalid[`})
	if len(regexes) != 1 {
		t.Errorf("expected 1 valid regex, got %d", len(regexes))
	}
}
