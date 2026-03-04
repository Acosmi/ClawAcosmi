package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/agents/auth"
	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// newTempAuthStore 创建临时 auth store 用于测试。
func newTempAuthStore(t *testing.T) *auth.AuthStore {
	t.Helper()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "auth.json")
	store := auth.NewAuthStore(storePath)
	// 初始化空 store
	if _, err := store.Load(); err != nil {
		// 第一次加载文件不存在是正常的，忽略
		_ = err
	}
	return store
}

func TestSetAnthropicApiKey(t *testing.T) {
	store := newTempAuthStore(t)
	if err := SetAnthropicApiKey(store, "sk-ant-test123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cred := store.GetProfile("anthropic:default")
	if cred == nil {
		t.Fatal("profile not found")
	}
	if cred.Provider != "anthropic" {
		t.Errorf("expected provider anthropic, got %s", cred.Provider)
	}
	if cred.Key != "sk-ant-test123" {
		t.Errorf("expected key sk-ant-test123, got %s", cred.Key)
	}
	if cred.Type != auth.CredentialAPIKey {
		t.Errorf("expected type api_key, got %s", cred.Type)
	}
}

func TestSetGeminiApiKey(t *testing.T) {
	store := newTempAuthStore(t)
	if err := SetGeminiApiKey(store, "AIza-test-key"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cred := store.GetProfile("google:default")
	if cred == nil {
		t.Fatal("profile not found")
	}
	if cred.Provider != "google" {
		t.Errorf("expected provider google, got %s", cred.Provider)
	}
}

func TestWriteOAuthCredentials(t *testing.T) {
	store := newTempAuthStore(t)
	err := WriteOAuthCredentials(store, "anthropic", " user@test.com ", "access-tok", "refresh-tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cred := store.GetProfile("anthropic:user@test.com")
	if cred == nil {
		t.Fatal("profile not found")
	}
	if cred.Type != auth.CredentialOAuth {
		t.Errorf("expected type oauth, got %s", cred.Type)
	}
	if cred.Email != "user@test.com" {
		t.Errorf("expected trimmed email, got %q", cred.Email)
	}
	if cred.Metadata["refreshToken"] != "refresh-tok" {
		t.Errorf("expected refreshToken, got %q", cred.Metadata["refreshToken"])
	}
}

func TestWriteOAuthCredentials_EmptyEmail(t *testing.T) {
	store := newTempAuthStore(t)
	err := WriteOAuthCredentials(store, "google", "", "tok", "ref")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cred := store.GetProfile("google:default")
	if cred == nil {
		t.Fatal("profile not found — empty email should default to 'default'")
	}
}

func TestAllSetApiKeyFunctions_NilStore(t *testing.T) {
	// 应该都返回 nil 而不是 panic
	fns := []func(*auth.AuthStore, string) error{
		SetAnthropicApiKey, SetGeminiApiKey, SetMinimaxApiKey,
		SetMoonshotApiKey, SetZaiApiKey, SetAcosmiZenApiKey,
		SetXaiApiKey, SetOpenAIApiKey,
	}
	for i, fn := range fns {
		if err := fn(nil, "test-key"); err != nil {
			t.Errorf("fn[%d] with nil store should return nil, got: %v", i, err)
		}
	}
}

func TestAllSetApiKeyFunctions_Persistence(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "auth.json")
	store := auth.NewAuthStore(storePath)
	store.Load()

	providers := []struct {
		fn       func(*auth.AuthStore, string) error
		provider string
	}{
		{SetAnthropicApiKey, "anthropic"},
		{SetGeminiApiKey, "google"},
		{SetMinimaxApiKey, "minimax"},
		{SetMoonshotApiKey, "moonshot"},
		{SetZaiApiKey, "zai"},
		{SetAcosmiZenApiKey, "openacosmi"},
		{SetXaiApiKey, "xai"},
		{SetOpenAIApiKey, "openai"},
	}

	for _, p := range providers {
		if err := p.fn(store, "key-"+p.provider); err != nil {
			t.Errorf("set %s: %v", p.provider, err)
			continue
		}
		cred := store.GetProfile(p.provider + ":default")
		if cred == nil {
			t.Errorf("%s: profile not found", p.provider)
			continue
		}
		if cred.Provider != p.provider {
			t.Errorf("%s: wrong provider %s", p.provider, cred.Provider)
		}
		if cred.Key != "key-"+p.provider {
			t.Errorf("%s: wrong key %s", p.provider, cred.Key)
		}
	}

	// 验证持久化 — 重新加载
	store2 := auth.NewAuthStore(storePath)
	store2.Load()
	// 检查文件存在
	if _, err := os.Stat(storePath); os.IsNotExist(err) {
		t.Error("store file not persisted")
	}
}

// ---------- 模型定义测试 ----------

func TestBuildMinimaxModelDefinition(t *testing.T) {
	m := BuildMinimaxModelDefinition("MiniMax-M2.1", "MiniMax M2.1", false, types.ModelCostConfig{
		Input: 15, Output: 60, CacheRead: 2, CacheWrite: 10,
	}, 200000, 8192)
	if m.ID != "MiniMax-M2.1" {
		t.Errorf("expected ID MiniMax-M2.1, got %s", m.ID)
	}
	if m.Name != "MiniMax M2.1" {
		t.Errorf("expected name MiniMax M2.1, got %s", m.Name)
	}
	if m.ContextWindow != 200000 {
		t.Errorf("expected ctx 200000, got %d", m.ContextWindow)
	}
}

func TestBuildMinimaxModelDefinition_DefaultName(t *testing.T) {
	m := BuildMinimaxModelDefinition("test-id", "", false, types.ModelCostConfig{}, 1000, 100)
	if m.Name != "MiniMax test-id" {
		t.Errorf("expected auto-generated name, got %s", m.Name)
	}
}

func TestBuildMinimaxApiModelDefinition(t *testing.T) {
	m := BuildMinimaxApiModelDefinition("MiniMax-M2.1")
	if m.Cost.Input != 15 || m.Cost.Output != 60 {
		t.Errorf("unexpected cost: %+v", m.Cost)
	}
	if m.ContextWindow != DefaultMinimaxCtxWindow {
		t.Errorf("expected ctx %d, got %d", DefaultMinimaxCtxWindow, m.ContextWindow)
	}
}

func TestBuildMoonshotModelDefinition(t *testing.T) {
	m := BuildMoonshotModelDefinition()
	if m.ID != MoonshotDefaultModelID {
		t.Errorf("expected %s, got %s", MoonshotDefaultModelID, m.ID)
	}
	if m.Name != "Kimi K2.5" {
		t.Errorf("expected Kimi K2.5, got %s", m.Name)
	}
	if m.ContextWindow != MoonshotDefaultCtxWin {
		t.Errorf("expected ctx %d, got %d", MoonshotDefaultCtxWin, m.ContextWindow)
	}
}

func TestBuildXaiModelDefinition(t *testing.T) {
	m := BuildXaiModelDefinition()
	if m.ID != XaiDefaultModelID {
		t.Errorf("expected %s, got %s", XaiDefaultModelID, m.ID)
	}
	if m.Name != "Grok 4" {
		t.Errorf("expected Grok 4, got %s", m.Name)
	}
	if m.ContextWindow != XaiDefaultCtxWin {
		t.Errorf("expected ctx %d, got %d", XaiDefaultCtxWin, m.ContextWindow)
	}
}

// ---------- 模型目录测试 ----------

func TestBuildProviderModelCatalog_Anthropic(t *testing.T) {
	catalog := BuildProviderModelCatalog("anthropic")
	if len(catalog) != 3 {
		t.Errorf("expected 3 models, got %d", len(catalog))
	}
	if !catalog[0].Default {
		t.Error("first should be default")
	}
}

func TestBuildProviderModelCatalog_Unknown(t *testing.T) {
	catalog := BuildProviderModelCatalog("unknown-provider")
	if catalog != nil {
		t.Errorf("expected nil for unknown, got %v", catalog)
	}
}

func TestBuildProviderModelCatalog_AllProviders(t *testing.T) {
	providers := []string{"anthropic", "openai", "google", "moonshot", "minimax", "xai", "zai"}
	for _, p := range providers {
		catalog := BuildProviderModelCatalog(p)
		if len(catalog) == 0 {
			t.Errorf("%s: expected non-empty catalog", p)
		}
		// 至少一个 default
		hasDefault := false
		for _, entry := range catalog {
			if entry.Default {
				hasDefault = true
				break
			}
		}
		if !hasDefault {
			t.Errorf("%s: no default model", p)
		}
	}
}

func TestPickDefaultModel_SingleModel(t *testing.T) {
	// moonshot has only 1 model → auto-select
	ref, err := PickDefaultModel(nil, "moonshot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != MoonshotDefaultModelRef {
		t.Errorf("expected %s, got %s", MoonshotDefaultModelRef, ref)
	}
}

func TestPickDefaultModel_UnknownProvider(t *testing.T) {
	ref, err := PickDefaultModel(nil, "unknown")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ref != "" {
		t.Errorf("expected empty ref, got %s", ref)
	}
}
