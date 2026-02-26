package memory

import (
	"encoding/json"
	"sort"
	"strings"
)

// FingerprintHeaderNames returns sorted header names for cache-key fingerprinting,
// excluding auth-related headers that change per-request.
func FingerprintHeaderNames(headers map[string]string) []string {
	skip := map[string]bool{
		"authorization":  true,
		"x-goog-api-key": true,
		"x-api-key":      true,
	}
	var names []string
	for k := range headers {
		if !skip[strings.ToLower(k)] {
			names = append(names, k)
		}
	}
	sort.Strings(names)
	return names
}

// ComputeEmbeddingProviderKey generates a stable cache key for an embedding provider.
func ComputeEmbeddingProviderKey(providerID, providerModel string, openAI, gemini *struct {
	BaseURL string
	Model   string
	Headers map[string]string
}) string {
	if openAI != nil {
		headerNames := FingerprintHeaderNames(openAI.Headers)
		data, _ := json.Marshal(map[string]any{
			"provider":    "openai",
			"baseUrl":     openAI.BaseURL,
			"model":       openAI.Model,
			"headerNames": headerNames,
		})
		return HashText(string(data))
	}
	if gemini != nil {
		headerNames := FingerprintHeaderNames(gemini.Headers)
		data, _ := json.Marshal(map[string]any{
			"provider":    "gemini",
			"baseUrl":     gemini.BaseURL,
			"model":       gemini.Model,
			"headerNames": headerNames,
		})
		return HashText(string(data))
	}
	data, _ := json.Marshal(map[string]any{
		"provider": providerID,
		"model":    providerModel,
	})
	return HashText(string(data))
}

// ComputeMemoryManagerCacheKey generates a stable cache key for a MemoryManager
// instance based on agent config and settings.
func ComputeMemoryManagerCacheKey(agentID, workspaceDir string, settingsJSON string) string {
	fingerprint := HashText(settingsJSON)
	return agentID + ":" + workspaceDir + ":" + fingerprint
}
