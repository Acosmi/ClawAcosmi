// Package services — Dynamic Configuration Service.
// Mirrors Python services/config_service.py — DB-backed cache with typed getters.
package services

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// ConfigMetadata stores metadata about a cached config entry.
type ConfigMetadata struct {
	Group       string
	IsSecret    bool
	Description string
}

// ConfigEntry represents a config item for API responses.
type ConfigEntry struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Group       string `json:"group"`
	IsSecret    bool   `json:"is_secret"`
	Description string `json:"description,omitempty"`
}

// DynamicConfigService manages runtime configuration with DB persistence + memory cache.
type DynamicConfigService struct {
	mu          sync.RWMutex
	cache       map[string]string
	metadata    map[string]ConfigMetadata
	initialized bool
}

// IsInitialized returns whether configs have been loaded.
func (s *DynamicConfigService) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

// LoadConfigs loads all configurations from the database into memory cache.
// Should be called during application startup.
func (s *DynamicConfigService) LoadConfigs(db *gorm.DB) (int, error) {
	var configs []models.SystemConfig
	if err := db.Find(&configs).Error; err != nil {
		return 0, fmt.Errorf("load configs: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.cache = make(map[string]string, len(configs))
	s.metadata = make(map[string]ConfigMetadata, len(configs))

	for _, cfg := range configs {
		val := cfg.Value
		if cfg.IsSecret {
			if dec, err := DecryptConfigValue(val); err == nil {
				val = dec
			} else {
				slog.Warn("Config decrypt failed, using raw", "key", cfg.Key, "error", err)
			}
		}
		s.cache[cfg.Key] = val
		desc := ""
		if cfg.Description != nil {
			desc = *cfg.Description
		}
		s.metadata[cfg.Key] = ConfigMetadata{
			Group:       cfg.Group,
			IsSecret:    cfg.IsSecret,
			Description: desc,
		}
	}

	s.initialized = true
	slog.Info("Loaded configurations into cache", "count", len(configs))
	return len(configs), nil
}

// --- Typed Getters ---

// GetValue returns a config value from cache. Returns def if not found.
func (s *DynamicConfigService) GetValue(key, def string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.cache[key]; ok {
		return v
	}
	return def
}

// GetInt returns a config value as int.
func (s *DynamicConfigService) GetInt(key string, def int) int {
	v := s.GetValue(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// GetFloat returns a config value as float64.
func (s *DynamicConfigService) GetFloat(key string, def float64) float64 {
	v := s.GetValue(key, "")
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

// GetBool returns a config value as bool.
func (s *DynamicConfigService) GetBool(key string, def bool) bool {
	v := s.GetValue(key, "")
	if v == "" {
		return def
	}
	lower := strings.ToLower(v)
	return lower == "true" || lower == "1" || lower == "yes" || lower == "on"
}

// --- Write Operations ---

// SetValue creates or updates a config value. Writes to DB and updates cache.
func (s *DynamicConfigService) SetValue(
	db *gorm.DB,
	key, value, group string,
	isSecret *bool,
	description *string,
) error {
	// Auto-detect secret
	secret := false
	if isSecret != nil {
		secret = *isSecret
	} else {
		secret = models.SecretKeys[key]
	}

	if group == "" {
		group = models.ConfigGroupGeneral
	}

	// Upsert in DB
	var existing models.SystemConfig
	result := db.Where("key = ?", key).First(&existing)

	// 加密 secret 值后存 DB
	dbValue := value
	if secret {
		if enc, err := EncryptConfigValue(value); err == nil {
			dbValue = enc
		} else {
			slog.Warn("Config encrypt failed, storing raw", "key", key, "error", err)
		}
	}

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// Create
		cfg := models.SystemConfig{
			Key:         key,
			Value:       dbValue,
			Group:       group,
			IsSecret:    secret,
			Description: description,
		}
		if err := db.Create(&cfg).Error; err != nil {
			return fmt.Errorf("create config: %w", err)
		}
	} else if result.Error != nil {
		return fmt.Errorf("query config: %w", result.Error)
	} else {
		// Update
		updates := map[string]any{
			"value":     dbValue,
			"is_secret": secret,
			"group":     group,
		}
		if description != nil {
			updates["description"] = *description
		}
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			return fmt.Errorf("update config: %w", err)
		}
	}

	// Update cache
	s.mu.Lock()
	s.cache[key] = value
	desc := ""
	if description != nil {
		desc = *description
	}
	s.metadata[key] = ConfigMetadata{
		Group: group, IsSecret: secret, Description: desc,
	}
	s.mu.Unlock()

	slog.Debug("Config updated", "key", key)
	return nil
}

// DeleteValue removes a config from DB and cache. Returns true if found.
func (s *DynamicConfigService) DeleteValue(db *gorm.DB, key string) (bool, error) {
	result := db.Where("key = ?", key).Delete(&models.SystemConfig{})
	if result.Error != nil {
		return false, fmt.Errorf("delete config: %w", result.Error)
	}

	s.mu.Lock()
	delete(s.cache, key)
	delete(s.metadata, key)
	s.mu.Unlock()

	return result.RowsAffected > 0, nil
}

// --- Query Operations ---

// GetAll returns all cached configs, optionally filtered by group.
func (s *DynamicConfigService) GetAll(group string) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]string)
	for k, v := range s.cache {
		if group != "" {
			if meta, ok := s.metadata[k]; ok && meta.Group != group {
				continue
			}
		}
		result[k] = v
	}
	return result
}

// GetAllWithMetadata returns config entries for API responses.
func (s *DynamicConfigService) GetAllWithMetadata(group string, maskSecrets bool) []ConfigEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []ConfigEntry
	for k, v := range s.cache {
		meta := s.metadata[k]
		if group != "" && meta.Group != group {
			continue
		}
		displayValue := v
		if maskSecrets && meta.IsSecret {
			displayValue = "******"
		}
		entries = append(entries, ConfigEntry{
			Key:         k,
			Value:       displayValue,
			Group:       meta.Group,
			IsSecret:    meta.IsSecret,
			Description: meta.Description,
		})
	}
	return entries
}

// --- Cache Management ---

// Invalidate clears cache entry (or all if key is empty).
func (s *DynamicConfigService) Invalidate(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if key != "" {
		delete(s.cache, key)
		delete(s.metadata, key)
	} else {
		s.cache = make(map[string]string)
		s.metadata = make(map[string]ConfigMetadata)
		s.initialized = false
	}
	slog.Debug("Config cache invalidated", "key", key)
}

// InitializeDefaults seeds default configurations if they don't exist.
func (s *DynamicConfigService) InitializeDefaults(db *gorm.DB) (int, error) {
	defaults := []struct {
		Key, Value, Group, Desc string
	}{
		{"EMBEDDING_PROVIDER", "local", models.ConfigGroupEmbedding, "Embedding provider"},
		{"EMBEDDING_MODEL_NAME", "BAAI/bge-small-zh-v1.5", models.ConfigGroupEmbedding, "Embedding model name"},
		{"EMBEDDING_DIMENSION", "512", models.ConfigGroupEmbedding, "Embedding vector dimension"},
		{"ENABLE_RERANK", "false", models.ConfigGroupRerank, "Enable reranking"},
		{"RERANK_PROVIDER", "cohere", models.ConfigGroupRerank, "Rerank provider"},
		{"RERANK_TOP_N", "5", models.ConfigGroupRerank, "Rerank top N results"},
		{"LLM_PROVIDER", "openai", models.ConfigGroupLLM, "LLM provider"},
	}

	s.mu.RLock()
	created := 0
	s.mu.RUnlock()

	for _, d := range defaults {
		if s.GetValue(d.Key, "") != "" {
			continue // Already exists
		}
		desc := d.Desc
		if err := s.SetValue(db, d.Key, d.Value, d.Group, nil, &desc); err != nil {
			slog.Warn("Failed to seed default config", "key", d.Key, "error", err)
			continue
		}
		created++
	}

	slog.Info("Default configs initialized", "created", created)
	return created, nil
}

// --- Singleton ---

var (
	dynConfigOnce    sync.Once
	dynConfigService *DynamicConfigService
)

// GetDynamicConfigService returns the singleton DynamicConfigService.
func GetDynamicConfigService() *DynamicConfigService {
	dynConfigOnce.Do(func() {
		dynConfigService = &DynamicConfigService{
			cache:    make(map[string]string),
			metadata: make(map[string]ConfigMetadata),
		}
	})
	return dynConfigService
}
