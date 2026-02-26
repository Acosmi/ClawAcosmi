package autoreply

import "testing"

func TestExtractModelDirective_Basic(t *testing.T) {
	result := ExtractModelDirective("/model gpt-4", nil)
	if !result.HasDirective {
		t.Fatal("should detect /model directive")
	}
	if result.RawModel != "gpt-4" {
		t.Errorf("RawModel = %q, want %q", result.RawModel, "gpt-4")
	}
	if result.Cleaned != "" {
		t.Errorf("Cleaned = %q, want empty", result.Cleaned)
	}
}

func TestExtractModelDirective_WithBody(t *testing.T) {
	result := ExtractModelDirective("hello /model gpt-4 world", nil)
	if !result.HasDirective {
		t.Fatal("should detect /model directive in body")
	}
	if result.RawModel != "gpt-4" {
		t.Errorf("RawModel = %q, want %q", result.RawModel, "gpt-4")
	}
	if result.Cleaned != "hello world" {
		t.Errorf("Cleaned = %q, want %q", result.Cleaned, "hello world")
	}
}

func TestExtractModelDirective_WithProfile(t *testing.T) {
	result := ExtractModelDirective("/model gpt-4@creative", nil)
	if !result.HasDirective {
		t.Fatal("should detect directive")
	}
	if result.RawModel != "gpt-4" {
		t.Errorf("RawModel = %q, want %q", result.RawModel, "gpt-4")
	}
	if result.RawProfile != "creative" {
		t.Errorf("RawProfile = %q, want %q", result.RawProfile, "creative")
	}
}

func TestExtractModelDirective_NoDirective(t *testing.T) {
	result := ExtractModelDirective("just hello", nil)
	if result.HasDirective {
		t.Error("should not detect directive")
	}
	if result.Cleaned != "just hello" {
		t.Errorf("Cleaned = %q, want %q", result.Cleaned, "just hello")
	}
}

func TestExtractModelDirective_EmptyBody(t *testing.T) {
	result := ExtractModelDirective("", nil)
	if result.HasDirective {
		t.Error("empty body should not have directive")
	}
}

func TestExtractModelDirective_ProviderSlashModel(t *testing.T) {
	result := ExtractModelDirective("/model openai/gpt-4", nil)
	if !result.HasDirective {
		t.Fatal("should detect directive")
	}
	if result.RawModel != "openai/gpt-4" {
		t.Errorf("RawModel = %q, want %q", result.RawModel, "openai/gpt-4")
	}
}

func TestExtractModelDirective_WithColon(t *testing.T) {
	result := ExtractModelDirective("/model: gpt-4", nil)
	if !result.HasDirective {
		t.Fatal("should detect /model: directive")
	}
	if result.RawModel != "gpt-4" {
		t.Errorf("RawModel = %q, want %q", result.RawModel, "gpt-4")
	}
}
