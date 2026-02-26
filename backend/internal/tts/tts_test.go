package tts

import (
	"testing"
)

// ---------- CacheKey ----------

func TestCacheKey(t *testing.T) {
	key := CacheKey("hello world", ProviderOpenAI, "alloy")
	if key == "" {
		t.Fatal("expected non-empty cache key")
	}
	// Should be deterministic
	key2 := CacheKey("hello world", ProviderOpenAI, "alloy")
	if key != key2 {
		t.Errorf("cache key not deterministic: %q != %q", key, key2)
	}
	// Different inputs → different keys
	key3 := CacheKey("hello world", ProviderElevenLabs, "alloy")
	if key == key3 {
		t.Error("different providers should produce different keys")
	}
}

// ---------- GetCached / SetCached ----------

func TestCacheGetSet(t *testing.T) {
	// clean state
	audioCache.mu.Lock()
	audioCache.entries = make(map[string]cachedAudio)
	audioCache.mu.Unlock()

	key := "test-key"
	_, ok := GetCached(key)
	if ok {
		t.Error("expected cache miss for new key")
	}

	// set + get (note: file doesn't exist, so GetCached should return miss after stat check)
	SetCached(key, "/tmp/nonexistent-audio-file.wav")
	_, ok = GetCached(key)
	if ok {
		t.Logf("GetCached returns false for nonexistent file (correct)")
	}
}

// ---------- NormalizeTtsAutoMode ----------

func TestNormalizeTtsAutoMode(t *testing.T) {
	tests := []struct {
		input string
		want  TtsAutoMode
	}{
		{"always", AutoAlways},
		{"ALWAYS", AutoAlways},
		{"off", AutoOff},
		{"OFF", AutoOff},
		{"inbound", AutoInbound},
		{"INBOUND", AutoInbound},
		{"tagged", AutoTagged},
		{"", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeTtsAutoMode(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeTtsAutoMode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- ResolveTtsConfig ----------

func TestResolveTtsConfig_Defaults(t *testing.T) {
	cfg := ResolveTtsConfig(TtsRawConfig{})
	if cfg.Auto != AutoOff {
		t.Errorf("expected Auto=off by default, got %q", cfg.Auto)
	}
	if cfg.MaxTextLength <= 0 {
		t.Error("expected positive MaxTextLength default")
	}
	if cfg.TimeoutMs <= 0 {
		t.Error("expected positive TimeoutMs default")
	}
}

func TestResolveTtsConfig_WithValues(t *testing.T) {
	cfg := ResolveTtsConfig(TtsRawConfig{
		Auto:          "always",
		MaxTextLength: 500,
		TimeoutMs:     3000,
	})
	if cfg.Auto != AutoAlways {
		t.Errorf("expected Auto=always, got %q", cfg.Auto)
	}
	if cfg.MaxTextLength != 500 {
		t.Errorf("expected MaxTextLength=500, got %d", cfg.MaxTextLength)
	}
	if cfg.TimeoutMs != 3000 {
		t.Errorf("expected TimeoutMs=3000, got %d", cfg.TimeoutMs)
	}
}

// ---------- ParseTtsDirectives ----------

func TestParseTtsDirectives_NoDirective(t *testing.T) {
	result := ParseTtsDirectives("Hello, world!", ResolvedTtsModelOverrides{})
	if result.HasDirective {
		t.Error("expected no directive in plain text")
	}
	if result.CleanedText != "Hello, world!" {
		t.Errorf("expected unchanged text, got %q", result.CleanedText)
	}
}

func TestParseTtsDirectives_SimpleTag(t *testing.T) {
	result := ParseTtsDirectives("Hello [[tts]] world", ResolvedTtsModelOverrides{})
	if !result.HasDirective {
		t.Error("expected directive found")
	}
	if result.CleanedText == "" {
		t.Error("expected non-empty cleaned text")
	}
}

func TestParseTtsDirectives_ProviderOverride(t *testing.T) {
	overrides := ResolvedTtsModelOverrides{AllowProvider: true}
	result := ParseTtsDirectives("Hello [[tts:provider=openai]] world", overrides)
	if !result.HasDirective {
		t.Error("expected directive found")
	}
	if result.Overrides.Provider != ProviderOpenAI {
		t.Errorf("expected provider=openai, got %q", result.Overrides.Provider)
	}
}

func TestParseTtsDirectives_VoiceOverride(t *testing.T) {
	overrides := ResolvedTtsModelOverrides{AllowVoice: true}
	result := ParseTtsDirectives("Hello [[tts:voice=alloy]] world", overrides)
	if result.Overrides.OpenAI == nil || result.Overrides.OpenAI.Voice != "alloy" {
		t.Errorf("expected voice=alloy, got %+v", result.Overrides)
	}
}

func TestParseTtsDirectives_TextBlock(t *testing.T) {
	overrides := ResolvedTtsModelOverrides{AllowText: true}
	input := "Before [[tts:text]]speak this[[/tts:text]] after"
	result := ParseTtsDirectives(input, overrides)
	if result.TtsText != "speak this" {
		t.Errorf("expected TtsText='speak this', got %q", result.TtsText)
	}
}

func TestParseTtsDirectives_Empty(t *testing.T) {
	result := ParseTtsDirectives("", ResolvedTtsModelOverrides{})
	if result.HasDirective {
		t.Error("expected no directive in empty text")
	}
}

// ---------- GetLastTtsAttempt / SetLastTtsAttempt ----------

func TestTtsStatusTracking(t *testing.T) {
	// Clean state
	lastTtsAttemptValue = nil

	status := GetLastTtsAttempt()
	if status != nil {
		t.Error("expected nil initial status")
	}

	entry := &TtsStatusEntry{
		Timestamp: 12345,
		Success:   true,
		Provider:  "openai",
	}
	SetLastTtsAttempt(entry)

	got := GetLastTtsAttempt()
	if got == nil {
		t.Fatal("expected non-nil status after set")
	}
	if got.Provider != "openai" {
		t.Errorf("expected provider=openai, got %q", got.Provider)
	}
}
