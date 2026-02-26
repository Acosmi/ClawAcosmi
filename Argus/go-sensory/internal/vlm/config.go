package vlm

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// ProviderConfig represents the configuration for a single VLM provider.
type ProviderConfig struct {
	Name     string `json:"name"`     // display name, e.g. "openai-main"
	Type     string `json:"type"`     // "openai" or "gemini"
	Endpoint string `json:"endpoint"` // API base URL
	APIKey   string `json:"api_key"`  // API key
	Model    string `json:"model"`    // default model name
	Active   bool   `json:"active"`   // whether this provider is the active default
}

// VLMConfig holds all VLM-related configuration with CRUD and persistence.
type VLMConfig struct {
	Providers []ProviderConfig `json:"providers"`

	mu       sync.RWMutex `json:"-"`
	filePath string       `json:"-"` // path for JSON persistence
}

// --- Load ---

// LoadConfigFromFile loads VLM configuration from a JSON file.
func LoadConfigFromFile(path string) (*VLMConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading VLM config file: %w", err)
	}

	var cfg VLMConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing VLM config file: %w", err)
	}
	cfg.filePath = path

	return &cfg, nil
}

// LoadConfigFromEnv builds VLM configuration from environment variables.
// This provides backward compatibility with legacy env vars.
func LoadConfigFromEnv() *VLMConfig {
	cfg := &VLMConfig{}

	// Primary VLM provider (OpenAI-compatible, covers Qwen/OpenAI/Claude)
	vlmBase := getEnv("VLM_API_BASE", "")
	vlmKey := getEnv("VLM_API_KEY", "")
	vlmModel := getEnv("VLM_MODEL", "")

	if vlmBase != "" {
		cfg.Providers = append(cfg.Providers, ProviderConfig{
			Name:     "default",
			Type:     "openai",
			Endpoint: vlmBase,
			APIKey:   vlmKey,
			Model:    vlmModel,
			Active:   true,
		})
		log.Printf("[VLM] Loaded default provider from env: endpoint=%s model=%s", vlmBase, vlmModel)
	}

	// Gemini provider (if configured)
	geminiKey := getEnv("GEMINI_API_KEY", "")
	if geminiKey != "" {
		geminiModel := getEnv("GEMINI_MODEL", "gemini-2.0-flash-exp")
		isActive := len(cfg.Providers) == 0 // active only if no other provider
		cfg.Providers = append(cfg.Providers, ProviderConfig{
			Name:     "gemini",
			Type:     "gemini",
			Endpoint: "https://generativelanguage.googleapis.com/v1beta",
			APIKey:   geminiKey,
			Model:    geminiModel,
			Active:   isActive,
		})
		log.Printf("[VLM] Loaded Gemini provider from env: model=%s active=%v", geminiModel, isActive)
	}

	// Ollama local provider (auto-detect if no other provider is configured)
	ollamaEndpoint := getEnv("OLLAMA_ENDPOINT", "http://localhost:11434")
	ollamaModel := getEnv("OLLAMA_MODEL", "")

	if ollamaModel != "" && len(cfg.Providers) == 0 {
		cfg.Providers = append(cfg.Providers, ProviderConfig{
			Name:     "ollama-local",
			Type:     "ollama",
			Endpoint: ollamaEndpoint,
			APIKey:   "",
			Model:    ollamaModel,
			Active:   true,
		})
		log.Printf("[VLM] Loaded Ollama provider from env: endpoint=%s model=%s", ollamaEndpoint, ollamaModel)
	}

	// Set default persistence path
	cfg.filePath = getEnv("VLM_CONFIG_PATH", "vlm-config.json")

	return cfg
}

// --- Query ---

// ActiveProvider returns the first active provider config, or nil.
func (c *VLMConfig) ActiveProvider() *ProviderConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i := range c.Providers {
		if c.Providers[i].Active {
			return &c.Providers[i]
		}
	}
	if len(c.Providers) > 0 {
		return &c.Providers[0]
	}
	return nil
}

// FindProvider returns a provider config by name, or nil.
func (c *VLMConfig) FindProvider(name string) *ProviderConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i]
		}
	}
	return nil
}

// ListProviders returns a copy of all provider configs.
func (c *VLMConfig) ListProviders() []ProviderConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]ProviderConfig, len(c.Providers))
	copy(result, c.Providers)
	return result
}

// --- CRUD ---

// AddProvider adds a new provider configuration.
// Returns error if a provider with the same name already exists.
func (c *VLMConfig) AddProvider(pc ProviderConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, existing := range c.Providers {
		if existing.Name == pc.Name {
			return fmt.Errorf("provider %q already exists", pc.Name)
		}
	}

	// If this is the first provider or it's marked active, deactivate others
	if pc.Active || len(c.Providers) == 0 {
		pc.Active = true
		for i := range c.Providers {
			c.Providers[i].Active = false
		}
	}

	c.Providers = append(c.Providers, pc)
	log.Printf("[VLM Config] Added provider: %s (type=%s)", pc.Name, pc.Type)
	return c.saveLocked()
}

// UpdateProvider updates an existing provider configuration.
func (c *VLMConfig) UpdateProvider(name string, updated ProviderConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	idx := -1
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("provider %q not found", name)
	}

	// If setting this as active, deactivate others
	if updated.Active {
		for i := range c.Providers {
			c.Providers[i].Active = false
		}
	}

	updated.Name = name // prevent name change via update
	c.Providers[idx] = updated

	log.Printf("[VLM Config] Updated provider: %s", name)
	return c.saveLocked()
}

// DeleteProvider removes a provider by name.
func (c *VLMConfig) DeleteProvider(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	idx := -1
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("provider %q not found", name)
	}

	wasActive := c.Providers[idx].Active
	c.Providers = append(c.Providers[:idx], c.Providers[idx+1:]...)

	// If we deleted the active provider, activate the first remaining
	if wasActive && len(c.Providers) > 0 {
		c.Providers[0].Active = true
	}

	log.Printf("[VLM Config] Deleted provider: %s", name)
	return c.saveLocked()
}

// SetActive sets a provider as the active provider and deactivates all others.
func (c *VLMConfig) SetActive(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	found := false
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			c.Providers[i].Active = true
			found = true
		} else {
			c.Providers[i].Active = false
		}
	}
	if !found {
		return fmt.Errorf("provider %q not found", name)
	}

	log.Printf("[VLM Config] Set active provider: %s", name)
	return c.saveLocked()
}

// --- Persistence ---

// saveLocked persists the config to disk. Must be called with mu held.
func (c *VLMConfig) saveLocked() error {
	if c.filePath == "" {
		return nil // no persistence configured
	}

	dir := filepath.Dir(c.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(c.filePath, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	log.Printf("[VLM Config] Saved to %s (%d providers)", c.filePath, len(c.Providers))
	return nil
}

// SetFilePath sets the JSON persistence file path.
func (c *VLMConfig) SetFilePath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.filePath = path
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
