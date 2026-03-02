package config

import (
	"strings"
	"testing"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

func TestNormalizeAgentID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty returns main", "", "main"},
		{"spaces returns main", "   ", "main"},
		{"valid lowercase", "my-agent", "my-agent"},
		{"valid with numbers", "agent1", "agent1"},
		{"valid with underscore", "my_agent", "my_agent"},
		{"uppercase lowered", "MyAgent", "myagent"},
		{"spaces collapsed", "my agent", "my-agent"},
		{"special chars replaced", "my@agent!", "my-agent"},
		{"leading dash removed", "--agent", "agent"},
		{"trailing dash kept (valid regex)", "agent--", "agent--"},
		{"long truncated", strings.Repeat("a", 200), strings.Repeat("a", 64)},
		{"all invalid chars", "!!!", "main"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeAgentID(tc.input)
			if got != tc.want {
				t.Fatalf("NormalizeAgentID(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func boolPtr(v bool) *bool { return &v }

func TestFindDuplicateAgentDirs(t *testing.T) {
	t.Run("no agents no duplicates", func(t *testing.T) {
		cfg := &types.OpenAcosmiConfig{}
		dups := FindDuplicateAgentDirs(cfg)
		if len(dups) != 0 {
			t.Fatalf("expected no duplicates, got %v", dups)
		}
	})

	t.Run("unique agentDirs no duplicates", func(t *testing.T) {
		cfg := &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{
				List: []types.AgentListItemConfig{
					{ID: "agent1", AgentDir: "/tmp/test-agent-dirs-a"},
					{ID: "agent2", AgentDir: "/tmp/test-agent-dirs-b"},
				},
			},
		}
		dups := FindDuplicateAgentDirs(cfg)
		if len(dups) != 0 {
			t.Fatalf("expected no duplicates, got %v", dups)
		}
	})

	t.Run("shared agentDir detected", func(t *testing.T) {
		cfg := &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{
				List: []types.AgentListItemConfig{
					{ID: "agent1", AgentDir: "/tmp/test-agent-dirs-shared"},
					{ID: "agent2", AgentDir: "/tmp/test-agent-dirs-shared"},
				},
			},
		}
		dups := FindDuplicateAgentDirs(cfg)
		if len(dups) != 1 {
			t.Fatalf("expected 1 duplicate, got %d", len(dups))
		}
		if len(dups[0].AgentIDs) != 2 {
			t.Fatalf("expected 2 agent IDs in duplicate, got %d", len(dups[0].AgentIDs))
		}
	})

	t.Run("default agent from flag", func(t *testing.T) {
		cfg := &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{
				List: []types.AgentListItemConfig{
					{ID: "alpha"},
					{ID: "beta", Default: boolPtr(true)},
				},
			},
		}
		ids := collectReferencedAgentIDs(cfg)
		// beta should be first (as default), then alpha
		if ids[0] != "beta" {
			t.Fatalf("first ID should be beta (default), got %q", ids[0])
		}
	})
}

func TestFormatDuplicateAgentDirError(t *testing.T) {
	dups := []DuplicateAgentDir{
		{AgentDir: "/shared/dir", AgentIDs: []string{"a", "b"}},
	}
	msg := FormatDuplicateAgentDirError(dups)
	if !strings.Contains(msg, "Duplicate agentDir") {
		t.Fatal("expected header in message")
	}
	if !strings.Contains(msg, "/shared/dir") {
		t.Fatal("expected agent dir path in message")
	}
	if !strings.Contains(msg, `"a"`) || !strings.Contains(msg, `"b"`) {
		t.Fatal("expected agent IDs in message")
	}
}
