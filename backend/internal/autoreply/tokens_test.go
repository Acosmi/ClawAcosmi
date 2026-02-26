package autoreply

import "testing"

// TS 对照: auto-reply/tokens.test.ts（部分用例）

func TestIsSilentReplyText_ExactToken(t *testing.T) {
	if !IsSilentReplyText("NO_REPLY") {
		t.Error("exact token should match")
	}
}

func TestIsSilentReplyText_WhitespacePrefix(t *testing.T) {
	if !IsSilentReplyText("  NO_REPLY") {
		t.Error("whitespace-prefixed token should match")
	}
}

func TestIsSilentReplyText_Suffix(t *testing.T) {
	if !IsSilentReplyText("some text NO_REPLY") {
		t.Error("token at end should match")
	}
}

func TestIsSilentReplyText_SuffixWithPunctuation(t *testing.T) {
	if !IsSilentReplyText("ok NO_REPLY.") {
		t.Error("token at end with punctuation should match")
	}
}

func TestIsSilentReplyText_EmptyString(t *testing.T) {
	if IsSilentReplyText("") {
		t.Error("empty string should not match")
	}
}

func TestIsSilentReplyText_NoMatch(t *testing.T) {
	if IsSilentReplyText("hello world") {
		t.Error("unrelated text should not match")
	}
}

func TestIsSilentReplyText_SubstringNotMatch(t *testing.T) {
	// "NO_REPLYING" should NOT match — boundary check
	if IsSilentReplyText("NO_REPLYING today") {
		t.Error("substring should not match")
	}
}

func TestIsSilentReplyText_CustomToken(t *testing.T) {
	if !IsSilentReplyText("SKIP_ME", "SKIP_ME") {
		t.Error("custom token should match")
	}
}
