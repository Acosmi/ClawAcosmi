package agents

import (
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

func intPtr(v int) *int { return &v }

func TestResolveAgentMaxConcurrent(t *testing.T) {
	tests := []struct {
		name string
		cfg  *types.OpenAcosmiConfig
		want int
	}{
		{"nil config", nil, DefaultAgentMaxConcurrent},
		{"nil agents", &types.OpenAcosmiConfig{}, DefaultAgentMaxConcurrent},
		{"nil defaults", &types.OpenAcosmiConfig{Agents: &types.AgentsConfig{}}, DefaultAgentMaxConcurrent},
		{"nil maxConcurrent", &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{Defaults: &types.AgentDefaultsConfig{}},
		}, DefaultAgentMaxConcurrent},
		{"zero value", &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{Defaults: &types.AgentDefaultsConfig{MaxConcurrent: intPtr(0)}},
		}, DefaultAgentMaxConcurrent},
		{"negative", &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{Defaults: &types.AgentDefaultsConfig{MaxConcurrent: intPtr(-5)}},
		}, DefaultAgentMaxConcurrent},
		{"valid=10", &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{Defaults: &types.AgentDefaultsConfig{MaxConcurrent: intPtr(10)}},
		}, 10},
		{"valid=1", &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{Defaults: &types.AgentDefaultsConfig{MaxConcurrent: intPtr(1)}},
		}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAgentMaxConcurrent(tt.cfg)
			if got != tt.want {
				t.Errorf("ResolveAgentMaxConcurrent() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestResolveSubagentMaxConcurrent(t *testing.T) {
	tests := []struct {
		name string
		cfg  *types.OpenAcosmiConfig
		want int
	}{
		{"nil config", nil, DefaultSubagentMaxConcurrent},
		{"nil subagents", &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{Defaults: &types.AgentDefaultsConfig{}},
		}, DefaultSubagentMaxConcurrent},
		{"nil maxConcurrent", &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{Defaults: &types.AgentDefaultsConfig{
				Subagents: &types.SubagentDefaultsConfig{},
			}},
		}, DefaultSubagentMaxConcurrent},
		{"valid=16", &types.OpenAcosmiConfig{
			Agents: &types.AgentsConfig{Defaults: &types.AgentDefaultsConfig{
				Subagents: &types.SubagentDefaultsConfig{MaxConcurrent: intPtr(16)},
			}},
		}, 16},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSubagentMaxConcurrent(tt.cfg)
			if got != tt.want {
				t.Errorf("ResolveSubagentMaxConcurrent() = %d, want %d", got, tt.want)
			}
		})
	}
}
