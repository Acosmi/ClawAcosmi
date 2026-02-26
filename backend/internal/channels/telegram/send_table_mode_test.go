package telegram

import (
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

func TestResolveTableMode_NilConfig(t *testing.T) {
	got := resolveTableMode(nil, nil)
	if got != TableModeDefault {
		t.Errorf("resolveTableMode(nil, nil) = %q, want %q", got, TableModeDefault)
	}
}

func TestResolveTableMode_NilMarkdown(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	got := resolveTableMode(cfg, nil)
	if got != TableModeDefault {
		t.Errorf("resolveTableMode(cfg{}, nil) = %q, want %q", got, TableModeDefault)
	}
}

func TestResolveTableMode_GlobalOff(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Markdown: &types.MarkdownConfig{Tables: types.MarkdownTableOff},
	}
	got := resolveTableMode(cfg, nil)
	if got != TableModeOff {
		t.Errorf("resolveTableMode(off) = %q, want %q", got, TableModeOff)
	}
}

func TestResolveTableMode_GlobalBullets(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Markdown: &types.MarkdownConfig{Tables: types.MarkdownTableBullets},
	}
	got := resolveTableMode(cfg, nil)
	if got != TableModeBullets {
		t.Errorf("resolveTableMode(bullets) = %q, want %q", got, TableModeBullets)
	}
}

func TestResolveTableMode_GlobalCode(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Markdown: &types.MarkdownConfig{Tables: types.MarkdownTableCode},
	}
	got := resolveTableMode(cfg, nil)
	if got != TableModeCode {
		t.Errorf("resolveTableMode(code) = %q, want %q", got, TableModeCode)
	}
}

func TestResolveTableMode_AccountOverridesGlobal(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Markdown: &types.MarkdownConfig{Tables: types.MarkdownTableBullets},
	}
	account := &ResolvedTelegramAccount{
		Config: types.TelegramAccountConfig{
			Markdown: &types.MarkdownConfig{Tables: types.MarkdownTableCode},
		},
	}
	got := resolveTableMode(cfg, account)
	if got != TableModeCode {
		t.Errorf("resolveTableMode(global=bullets, account=code) = %q, want %q (account should override)", got, TableModeCode)
	}
}

func TestResolveTableMode_AccountNilFallsToGlobal(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Markdown: &types.MarkdownConfig{Tables: types.MarkdownTableCode},
	}
	account := &ResolvedTelegramAccount{
		Config: types.TelegramAccountConfig{},
	}
	got := resolveTableMode(cfg, account)
	if got != TableModeCode {
		t.Errorf("resolveTableMode(global=code, account=nil) = %q, want %q", got, TableModeCode)
	}
}

func TestMapGlobalTableMode_Unknown(t *testing.T) {
	got := mapGlobalTableMode("unknown_mode")
	if got != TableModeDefault {
		t.Errorf("mapGlobalTableMode(unknown) = %q, want %q", got, TableModeDefault)
	}
}
