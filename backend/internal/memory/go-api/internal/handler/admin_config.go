// Package handler — Admin system configuration and config CRUD routes.
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/uhms/go-api/internal/config"
	"github.com/uhms/go-api/internal/models"
	"github.com/uhms/go-api/internal/services"
)

// ===========================================================================
// System Config
// ===========================================================================

var maskedPattern = regexp.MustCompile(`^(sk-|api-|key-)?\*{3,}$|^[\w-]*\*{3,}[\w-]*$`)

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) < 12 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

func isMasked(value string) bool {
	if value == "" {
		return true
	}
	return strings.Contains(value, "****")
}

func (h *AdminHandler) GetSystemConfig(c *gin.Context) {
	c.JSON(http.StatusOK, buildSystemConfigResponse(h))
}

func (h *AdminHandler) UpdateSystemConfig(c *gin.Context) {
	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	// Field mapping: (json_key, db_key, group, isSecret)
	type fmap struct {
		jsonKey  string
		dbKey    string
		group    string
		isSecret bool
	}
	fieldMapping := []fmap{
		{"embedding_provider", "EMBEDDING_PROVIDER", "embedding", false},
		{"embedding_model", "EMBEDDING_MODEL_NAME", "embedding", false},
		{"embedding_dimension", "EMBEDDING_DIMENSION", "embedding", false},
		{"embedding_api_key", "EMBEDDING_API_KEY", "embedding", true},
		{"embedding_base_url", "EMBEDDING_BASE_URL", "embedding", false},
		{"rerank_enabled", "ENABLE_RERANK", "rerank", false},
		{"rerank_provider", "RERANK_PROVIDER", "rerank", false},
		{"rerank_model", "RERANK_MODEL_NAME", "rerank", false},
		{"rerank_api_key", "RERANK_API_KEY", "rerank", true},
		{"llm_provider", "LLM_PROVIDER", "llm", false},
	}

	var updatedFields []string
	for _, fm := range fieldMapping {
		val, exists := body[fm.jsonKey]
		if !exists || val == nil {
			continue
		}
		strVal := toString(val)
		if fm.isSecret && isMasked(strVal) {
			continue
		}
		isSecret := fm.isSecret
		if err := h.configService.SetValue(getTenantDB(c), fm.dbKey, strVal, fm.group, &isSecret, nil); err != nil {
			slog.Error("Config update failed", "key", fm.dbKey, "error", err)
			continue
		}
		updatedFields = append(updatedFields, fm.jsonKey)
	}

	// Handle LLM provider-specific key/model/url
	llmProvider, _ := body["llm_provider"].(string)
	if llmProvider == "" {
		llmProvider = h.configService.GetValue("LLM_PROVIDER", "openai")
	}
	providerUpper := strings.ToUpper(llmProvider)

	if apiKey, ok := body["llm_api_key"].(string); ok && !isMasked(apiKey) {
		isSecret := true
		_ = h.configService.SetValue(getTenantDB(c), providerUpper+"_API_KEY", apiKey, "llm", &isSecret, nil)
		updatedFields = append(updatedFields, "llm_api_key")
	}
	if model, ok := body["llm_model"].(string); ok && model != "" {
		isSecret := false
		_ = h.configService.SetValue(getTenantDB(c), providerUpper+"_MODEL", model, "llm", &isSecret, nil)
		updatedFields = append(updatedFields, "llm_model")
	}
	if baseURL, ok := body["llm_base_url"].(string); ok && baseURL != "" {
		isSecret := false
		_ = h.configService.SetValue(getTenantDB(c), providerUpper+"_BASE_URL", baseURL, "llm", &isSecret, nil)
		updatedFields = append(updatedFields, "llm_base_url")
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "Updated configurations. AI services may need reload.",
		"updated_fields": updatedFields,
		"config":         buildSystemConfigResponse(h),
	})
}

// buildSystemConfigResponse 组装完整的系统配置响应（复用 GetSystemConfig 逻辑）。
func buildSystemConfigResponse(h *AdminHandler) gin.H {
	cfg := config.Get()
	llmProvider := h.configService.GetValue("LLM_PROVIDER", cfg.LLMProvider)
	return gin.H{
		"embedding_provider":  h.configService.GetValue("EMBEDDING_PROVIDER", cfg.EmbeddingProvider),
		"embedding_model":     h.configService.GetValue("EMBEDDING_MODEL_NAME", cfg.EmbeddingModelName),
		"embedding_dimension": h.configService.GetInt("EMBEDDING_DIMENSION", cfg.EmbeddingDimension),
		"embedding_api_key":   maskAPIKey(h.configService.GetValue("EMBEDDING_API_KEY", "")),
		"embedding_base_url":  h.configService.GetValue("EMBEDDING_BASE_URL", cfg.EmbeddingBaseURL),
		"rerank_enabled":      h.configService.GetBool("ENABLE_RERANK", cfg.EnableRerank),
		"rerank_provider":     h.configService.GetValue("RERANK_PROVIDER", cfg.RerankProvider),
		"rerank_model":        h.configService.GetValue("RERANK_MODEL_NAME", cfg.RerankModelName()),
		"rerank_api_key":      maskAPIKey(h.configService.GetValue("RERANK_API_KEY", "")),
		"llm_provider":        llmProvider,
		"llm_api_key":         maskAPIKey(h.configService.GetValue(strings.ToUpper(llmProvider)+"_API_KEY", "")),
		"version":             "v2.0-go",
		"config_source":       "database",
	}
}

func (h *AdminHandler) ReloadServices(c *gin.Context) {
	// 1) 刷新配置缓存
	h.configService.Invalidate("")
	count, err := h.configService.LoadConfigs(getTenantDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Config reload failed"})
		return
	}
	// 2) 重建 Embedding + LLM 服务单例
	services.ReloadEmbeddingService()
	services.ReloadLLMClient()

	slog.Info("Services hot-reloaded", "configs", count)
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          "Services reloaded with new config",
		"configs_reloaded": count,
	})
}

// ===========================================================================
// Config CRUD
// ===========================================================================

// Config group display names
var configGroupNames = map[string]string{
	"general":   "通用设置",
	"embedding": "向量嵌入",
	"rerank":    "重排序",
	"llm":       "大语言模型",
	"security":  "安全设置",
	"database":  "数据库",
}

func (h *AdminHandler) ListConfigs(c *gin.Context) {
	var configs []models.SystemConfig
	getTenantDB(c).Order("config_group ASC, key ASC").Find(&configs)

	// Group by config_group
	groupsMap := make(map[string][]gin.H)
	for _, cfg := range configs {
		value := cfg.Value
		if cfg.IsSecret {
			value = maskSecretValue(value)
		}
		item := gin.H{
			"key":         cfg.Key,
			"value":       value,
			"is_secret":   cfg.IsSecret,
			"description": cfg.Description,
		}
		groupsMap[cfg.Group] = append(groupsMap[cfg.Group], item)
	}

	groupOrder := []string{"general", "embedding", "rerank", "llm", "security", "database"}
	groups := make([]gin.H, 0)
	seen := make(map[string]bool)
	for _, gid := range groupOrder {
		if items, ok := groupsMap[gid]; ok {
			groups = append(groups, gin.H{
				"group": gid,
				"name":  configGroupNames[gid],
				"items": items,
			})
			seen[gid] = true
		}
	}
	// Remaining groups
	for gid, items := range groupsMap {
		if !seen[gid] {
			name := gid
			if n, ok := configGroupNames[gid]; ok {
				name = n
			}
			groups = append(groups, gin.H{
				"group": gid,
				"name":  name,
				"items": items,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"groups": groups, "total": len(configs)})
}

func maskSecretValue(value string) string {
	if value == "" {
		return "****"
	}
	if strings.HasPrefix(value, "sk-") || strings.HasPrefix(value, "api-") || strings.HasPrefix(value, "key-") {
		return value[:3] + "****"
	}
	return "****"
}

func (h *AdminHandler) BatchUpdateConfigs(c *gin.Context) {
	var req struct {
		Configs []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"configs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	updated, skipped := 0, 0
	for _, item := range req.Configs {
		if maskedPattern.MatchString(item.Value) {
			skipped++
			continue
		}
		// Preserve existing group/secret metadata
		var existing models.SystemConfig
		if err := getTenantDB(c).First(&existing, "key = ?", item.Key).Error; err == nil {
			_ = h.configService.SetValue(getTenantDB(c), item.Key, item.Value, existing.Group, &existing.IsSecret, existing.Description)
		}
		updated++
	}

	c.JSON(http.StatusOK, gin.H{
		"updated": updated,
		"skipped": skipped,
		"message": "Batch update complete",
	})
}

func (h *AdminHandler) RefreshConfigs(c *gin.Context) {
	h.configService.Invalidate("")
	count, err := h.configService.LoadConfigs(getTenantDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Reload failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"reloaded": count,
		"message":  "Configurations reloaded",
	})
}

func (h *AdminHandler) ListConfigGroups(c *gin.Context) {
	var groups []gin.H
	for id, name := range configGroupNames {
		groups = append(groups, gin.H{"id": id, "name": name})
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

func (h *AdminHandler) GetConfigValue(c *gin.Context) {
	key := c.Param("key")
	value := h.configService.GetValue(key, "")
	if value == "" {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Configuration not found"})
		return
	}

	// Check if secret
	var cfg models.SystemConfig
	isSecret := false
	if getTenantDB(c).First(&cfg, "key = ?", key).Error == nil {
		isSecret = cfg.IsSecret
	}

	displayValue := value
	if isSecret {
		includeSecret := c.Query("include_secret") == "true"
		if !includeSecret {
			displayValue = maskSecretValue(value)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"key":         key,
		"value":       displayValue,
		"is_secret":   isSecret,
		"description": cfg.Description,
	})
}

func (h *AdminHandler) UpdateSingleConfig(c *gin.Context) {
	key := c.Param("key")
	var req struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Invalid request body"})
		return
	}

	if maskedPattern.MatchString(req.Value) {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Cannot update with masked value"})
		return
	}

	if h.configService.GetValue(key, "") == "" {
		c.JSON(http.StatusNotFound, gin.H{"detail": "Configuration not found"})
		return
	}

	var existing models.SystemConfig
	if err := getTenantDB(c).First(&existing, "key = ?", key).Error; err == nil {
		_ = h.configService.SetValue(getTenantDB(c), key, req.Value, existing.Group, &existing.IsSecret, existing.Description)
	}

	c.JSON(http.StatusOK, gin.H{"key": key, "updated": true})
}

func (h *AdminHandler) InitializeDefaults(c *gin.Context) {
	created, err := h.configService.InitializeDefaults(getTenantDB(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Seed defaults failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"created": created, "message": "Default configs initialized"})
}

// ===========================================================================
// Helpers
// ===========================================================================

func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		return json.Number(strings.TrimRight(strings.TrimRight(
			strings.Replace(
				json.Number(strings.TrimRight(
					strings.TrimRight(
						strings.Replace(decimal.NewFromFloat(val).String(), ".", ".", 1), "0"), ".")).String(),
				".", ".", 1), "0"), ".")).String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
