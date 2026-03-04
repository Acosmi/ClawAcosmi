package bridge

import (
	"strings"
	"testing"
)

func TestBuildWizardProviderCatalog_AllProviders(t *testing.T) {
	catalog := BuildWizardProviderCatalog()
	if len(catalog) < 22 {
		t.Errorf("expected at least 22 providers, got %d", len(catalog))
	}
	for _, p := range catalog {
		if p.Name == "" {
			t.Errorf("provider %q has empty Name", p.ID)
		}
		if p.Desc == "" {
			t.Errorf("provider %q has empty Desc", p.ID)
		}
		if len(p.AuthModes) == 0 {
			t.Errorf("provider %q has no AuthModes", p.ID)
		}
	}
}

func TestBuildWizardProviderCatalog_ModelsNotEmpty(t *testing.T) {
	catalog := BuildWizardProviderCatalog()
	for _, p := range catalog {
		// custom-openai 只有一个占位模型，byteplus 复用 volcengine（可能无模型）
		if p.ID == "custom-openai" || p.ID == "byteplus" {
			continue
		}
		if len(p.Models) == 0 {
			t.Errorf("provider %q has no models", p.ID)
		}
		for _, m := range p.Models {
			if m.ID == "" {
				t.Errorf("provider %q has model with empty ID", p.ID)
			}
			if m.Name == "" {
				t.Errorf("provider %q model %q has empty Name", p.ID, m.ID)
			}
		}
	}
}

func TestBuildWizardProviderCatalog_DefaultModelRefValid(t *testing.T) {
	catalog := BuildWizardProviderCatalog()
	for _, p := range catalog {
		// byteplus 可能没有独立后端 ID
		if p.ID == "byteplus" {
			continue
		}
		if p.DefaultModelRef == "" {
			t.Errorf("provider %q has empty DefaultModelRef", p.ID)
		}
		if !strings.Contains(p.DefaultModelRef, "/") {
			t.Errorf("provider %q DefaultModelRef %q should contain '/'", p.ID, p.DefaultModelRef)
		}
	}
}

func TestBuildWizardProviderCatalog_Categories(t *testing.T) {
	catalog := BuildWizardProviderCatalog()
	categories := make(map[string]bool)
	for _, p := range catalog {
		categories[p.Category] = true
	}

	expected := []string{"oauth_priority", "china_major", "international", "emerging", "aggregator", "local_custom"}
	for _, c := range expected {
		if !categories[c] {
			t.Errorf("missing expected category %q", c)
		}
	}
}

func TestBuildWizardProviderCatalog_NilModelsNeverReturned(t *testing.T) {
	catalog := BuildWizardProviderCatalog()
	for _, p := range catalog {
		if p.Models == nil {
			t.Errorf("provider %q has nil Models (would serialize as JSON null)", p.ID)
		}
	}
}

func TestBuildWizardProviderCatalog_FrontendToBackendIDValid(t *testing.T) {
	// 验证 frontendToBackendID 中的目标 ID 确实存在于某个注册表中
	for frontendID, backendID := range frontendToBackendID {
		_, inDefault := defaultModelRefs[backendID]
		_, inRegistry := providerRegistry[backendID]
		if !inDefault && !inRegistry {
			t.Errorf("frontendToBackendID[%q]=%q not found in defaultModelRefs or providerRegistry", frontendID, backendID)
		}
	}
}

func TestBuildWizardProviderCatalog_AllProvidersHaveModelsOrCustomInput(t *testing.T) {
	// 验证所有 provider 要么有模型列表，要么是 requiresBaseUrl（自定义输入）
	catalog := BuildWizardProviderCatalog()
	for _, p := range catalog {
		if len(p.Models) == 0 && !p.RequiresBaseUrl {
			// byteplus 和 custom-openai 是已知的特殊情况
			if p.ID == "byteplus" || p.ID == "custom-openai" {
				continue
			}
			t.Errorf("provider %q has no models and requiresBaseUrl=false (frontend select will be empty)", p.ID)
		}
	}
}

func TestBuildWizardProviderCatalog_SortOrder(t *testing.T) {
	catalog := BuildWizardProviderCatalog()
	// 验证同 category 内按 sortOrder 排列
	lastCategory := ""
	lastSort := 0
	for _, p := range catalog {
		if p.Category != lastCategory {
			lastCategory = p.Category
			lastSort = 0
		}
		if p.SortOrder < lastSort {
			t.Errorf("provider %q sortOrder %d < previous %d in category %q", p.ID, p.SortOrder, lastSort, p.Category)
		}
		lastSort = p.SortOrder
	}
}
