package config

import (
	"encoding/json"
	"testing"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
	"github.com/zalando/go-keyring"
)

func TestStoreAndRestoreSensitiveToKeyring(t *testing.T) {
	// 1. Setup Mock Keyring for testing so we don't pollute the real OS keyring
	keyring.MockInit()

	// 2. Create a mock config with a sensitive field
	cfg := &types.OpenAcosmiConfig{
		SubAgents: &types.SubAgentConfig{
			OpenCoder: &types.OpenCoderSettings{
				APIKey: "sk-real-secret-12345",
			},
		},
		Env: &types.OpenAcosmiEnvConfig{
			Vars: map[string]string{
				"OPENAI_API_KEY": "sk-env-secret-67890",
				"SAFE_VAR":       "im-safe",
			},
		},
	}

	// 3. Convert to map for the generic traverse function
	m, err := MapStructToMapForKeyring(cfg)
	if err != nil {
		t.Fatalf("Failed to convert struct to map: %v", err)
	}

	// 4. Store sensitive fields into the mock keyring
	redactedMap, err := StoreSensitiveToKeyring(m)
	if err != nil {
		t.Fatalf("StoreSensitiveToKeyring failed: %v", err)
	}

	// 5. Verify the map now has sentinels
	redactedJSON, _ := json.Marshal(redactedMap)
	redactedStr := string(redactedJSON)

	if !Contains(redactedStr, KeyringSentinel) {
		t.Errorf("Expected map to contain KeyringSentinel, but got: %s", redactedStr)
	}
	if Contains(redactedStr, "sk-real-secret-12345") {
		t.Errorf("Expected APIKey to be redacted, but it leaked in map: %s", redactedStr)
	}
	if Contains(redactedStr, "sk-env-secret-67890") {
		t.Errorf("Expected env APIKey to be redacted, but it leaked in map: %s", redactedStr)
	}
	if !Contains(redactedStr, "im-safe") {
		t.Errorf("Expected SAFE_VAR to be untouched, but it was lost: %s", redactedStr)
	}

	// 6. Restore from keyring
	err = RestoreFromKeyring(redactedMap)
	if err != nil {
		t.Fatalf("RestoreFromKeyring failed: %v", err)
	}

	// 7. Verify the map has the secrets back
	restoredJSON, _ := json.Marshal(redactedMap)
	restoredStr := string(restoredJSON)

	if Contains(restoredStr, KeyringSentinel) {
		t.Errorf("Expected KeyringSentinel to be completely removed, but got: %s", restoredStr)
	}
	if !Contains(restoredStr, "sk-real-secret-12345") {
		t.Errorf("Expected APIKey to be restored")
	}
	if !Contains(restoredStr, "sk-env-secret-67890") {
		t.Errorf("Expected env APIKey to be restored")
	}
}

// Simple wrapper for strings.Contains to avoid importing strings everywhere just for tests if not needed,
// but import standard strings instead
func Contains(s, substr string) bool {
	return len(s) >= len(substr) && func(s, substr string) bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	}(s, substr)
}
