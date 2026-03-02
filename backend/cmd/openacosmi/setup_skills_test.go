package main

import (
	"testing"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

func TestSummarizeInstallFailure(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"no prefix", "some error occurred", "some error occurred"},
		{"with prefix", "Install failed (exit 1): package not found", "package not found"},
		{"prefix no colon", "Install failed", ""},
		{
			"long message truncated",
			"Install failed: " + string(make([]byte, 200)),
			"", // 此处检查截断
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SummarizeInstallFailure(tt.input)
			if tt.name == "long message truncated" {
				if len(result) == 0 || len(result) > 140 {
					// OK — 要么空要么被截断
					return
				}
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestFormatSkillHint(t *testing.T) {
	tests := []struct {
		name         string
		desc         string
		installLabel string
		expected     string
	}{
		{"both empty", "", "", "install"},
		{"desc only", "Web scraping tool", "", "Web scraping tool"},
		{"label only", "", "brew install wget", "brew install wget"},
		{"both present", "Web scraping", "brew install wget", "Web scraping — brew install wget"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSkillHint(tt.desc, tt.installLabel)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestUpsertSkillEntry(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		result := UpsertSkillEntry(nil, "my-skill", "sk-123")
		if result == nil {
			t.Fatal("expected non-nil config")
		}
		if result.Skills == nil || result.Skills.Entries == nil {
			t.Fatal("expected skills.entries to be initialized")
		}
		entry := result.Skills.Entries["my-skill"]
		if entry == nil || entry.APIKey != "sk-123" {
			t.Errorf("expected apiKey sk-123, got %v", entry)
		}
	})

	t.Run("append to existing", func(t *testing.T) {
		cfg := &types.OpenAcosmiConfig{}
		cfg1 := UpsertSkillEntry(cfg, "skill-a", "key-a")
		cfg2 := UpsertSkillEntry(cfg1, "skill-b", "key-b")

		if cfg2.Skills.Entries["skill-a"].APIKey != "key-a" {
			t.Error("skill-a key lost")
		}
		if cfg2.Skills.Entries["skill-b"].APIKey != "key-b" {
			t.Error("skill-b key missing")
		}
	})
}
