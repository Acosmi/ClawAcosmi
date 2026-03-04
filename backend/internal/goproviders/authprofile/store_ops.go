// authprofile/store_ops.go — Profile 存储操作（加载、保存、合并）
// 对应 TS 文件: src/agents/auth-profiles/store.ts（续）
package authprofile

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// coerceLegacyStore 尝试将旧格式 auth.json 转换为 LegacyAuthStore。
func coerceLegacyStore(raw map[string]interface{}) map[string]map[string]interface{} {
	if raw == nil {
		return nil
	}
	if _, hasProfiles := raw["profiles"]; hasProfiles {
		return nil
	}
	entries := make(map[string]map[string]interface{})
	var rejected []RejectedCredentialEntry
	for key, value := range raw {
		parsed, reason := parseCredentialEntry(value, key)
		if reason != "" {
			rejected = append(rejected, RejectedCredentialEntry{Key: key, Reason: reason})
			continue
		}
		entries[key] = parsed
	}
	warnRejectedCredentialEntries("auth.json", rejected)
	if len(entries) == 0 {
		return nil
	}
	return entries
}

// coerceAuthStore 将原始 JSON 数据转换为 AuthProfileStore。
func coerceAuthStore(raw map[string]interface{}) *types.AuthProfileStore {
	if raw == nil {
		return nil
	}
	profilesRaw, ok := raw["profiles"]
	if !ok {
		return nil
	}
	profiles, ok := profilesRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	normalized := make(map[string]map[string]interface{})
	var rejected []RejectedCredentialEntry
	for key, value := range profiles {
		parsed, reason := parseCredentialEntry(value, "")
		if reason != "" {
			rejected = append(rejected, RejectedCredentialEntry{Key: key, Reason: reason})
			continue
		}
		normalized[key] = parsed
	}
	warnRejectedCredentialEntries("auth-profiles.json", rejected)

	// 解析 order
	var order map[string][]string
	if orderRaw, ok := raw["order"].(map[string]interface{}); ok {
		order = make(map[string][]string)
		for provider, value := range orderRaw {
			arr, ok := value.([]interface{})
			if !ok {
				continue
			}
			var list []string
			for _, entry := range arr {
				if s, ok := entry.(string); ok {
					trimmed := strings.TrimSpace(s)
					if trimmed != "" {
						list = append(list, trimmed)
					}
				}
			}
			if len(list) > 0 {
				order[provider] = list
			}
		}
	}

	// 解析 version
	version := common.AuthStoreVersion
	if v, ok := raw["version"].(float64); ok {
		version = int(v)
	}

	// 解析 lastGood
	var lastGood map[string]string
	if lg, ok := raw["lastGood"].(map[string]interface{}); ok {
		lastGood = make(map[string]string)
		for k, v := range lg {
			if s, ok := v.(string); ok {
				lastGood[k] = s
			}
		}
	}

	// 解析 usageStats
	var usageStats map[string]types.ProfileUsageStats
	if us, ok := raw["usageStats"].(map[string]interface{}); ok {
		data, _ := json.Marshal(us)
		usageStats = make(map[string]types.ProfileUsageStats)
		_ = json.Unmarshal(data, &usageStats)
	}

	return &types.AuthProfileStore{
		Version:    version,
		Profiles:   normalized,
		Order:      order,
		LastGood:   lastGood,
		UsageStats: usageStats,
	}
}

// mergeRecord 合并两个 string→string 映射。
func mergeStringRecord(base, override map[string]string) map[string]string {
	if base == nil && override == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

// MergeAuthProfileStores 合并两个 AuthProfileStore（override 覆盖 base）。
// 对应 TS: mergeAuthProfileStores()
func MergeAuthProfileStores(base, override *types.AuthProfileStore) *types.AuthProfileStore {
	if len(override.Profiles) == 0 && override.Order == nil && override.LastGood == nil && override.UsageStats == nil {
		return base
	}

	version := base.Version
	if override.Version > version {
		version = override.Version
	}

	mergedProfiles := make(map[string]map[string]interface{})
	for k, v := range base.Profiles {
		mergedProfiles[k] = v
	}
	for k, v := range override.Profiles {
		mergedProfiles[k] = v
	}

	// 合并 order
	var mergedOrder map[string][]string
	if base.Order != nil || override.Order != nil {
		mergedOrder = make(map[string][]string)
		for k, v := range base.Order {
			mergedOrder[k] = v
		}
		for k, v := range override.Order {
			mergedOrder[k] = v
		}
	}

	// 合并 usageStats
	var mergedUsageStats map[string]types.ProfileUsageStats
	if base.UsageStats != nil || override.UsageStats != nil {
		mergedUsageStats = make(map[string]types.ProfileUsageStats)
		for k, v := range base.UsageStats {
			mergedUsageStats[k] = v
		}
		for k, v := range override.UsageStats {
			mergedUsageStats[k] = v
		}
	}

	return &types.AuthProfileStore{
		Version:    version,
		Profiles:   mergedProfiles,
		Order:      mergedOrder,
		LastGood:   mergeStringRecord(base.LastGood, override.LastGood),
		UsageStats: mergedUsageStats,
	}
}

// loadJsonFile 加载 JSON 文件为 map。
func loadJsonFile(path string) map[string]interface{} {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil
	}
	return result
}

// saveJsonFile 保存数据为 JSON 文件。
func saveJsonFile(path string, data interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, jsonData, 0o644)
}

// applyLegacyStore 将旧版存储的凭证应用到新存储中。
func applyLegacyStore(store *types.AuthProfileStore, legacy map[string]map[string]interface{}) {
	for provider, cred := range legacy {
		profileId := provider + ":default"
		credType, _ := cred["type"].(string)
		credProvider, _ := cred["provider"].(string)
		if credProvider == "" {
			credProvider = provider
		}

		switch credType {
		case "api_key":
			store.Profiles[profileId] = map[string]interface{}{
				"type":     "api_key",
				"provider": credProvider,
				"key":      cred["key"],
			}
			if email, ok := cred["email"].(string); ok && email != "" {
				store.Profiles[profileId]["email"] = email
			}
		case "token":
			entry := map[string]interface{}{
				"type":     "token",
				"provider": credProvider,
				"token":    cred["token"],
			}
			if expires, ok := cred["expires"]; ok {
				entry["expires"] = expires
			}
			if email, ok := cred["email"].(string); ok && email != "" {
				entry["email"] = email
			}
			store.Profiles[profileId] = entry
		default:
			entry := map[string]interface{}{
				"type":     "oauth",
				"provider": credProvider,
				"access":   cred["access"],
				"refresh":  cred["refresh"],
				"expires":  cred["expires"],
			}
			for _, k := range []string{"enterpriseUrl", "projectId", "accountId", "email"} {
				if v, ok := cred[k]; ok {
					if s, ok := v.(string); ok && s != "" {
						entry[k] = s
					}
				}
			}
			store.Profiles[profileId] = entry
		}
	}
}

// LoadAuthProfileStore 加载 Auth Profile Store（主代理版本）。
// 对应 TS: loadAuthProfileStore()
func LoadAuthProfileStore(cliReaders map[string]ExternalCliCredentialReader) *types.AuthProfileStore {
	authPath := common.ResolveAuthStorePath("")
	raw := loadJsonFile(authPath)
	asStore := coerceAuthStore(raw)
	if asStore != nil {
		synced := SyncExternalCliCredentials(asStore, cliReaders)
		if synced {
			_ = saveJsonFile(authPath, asStore)
		}
		return asStore
	}

	legacyRaw := loadJsonFile(common.ResolveLegacyAuthStorePath(""))
	legacy := coerceLegacyStore(legacyRaw)
	if legacy != nil {
		store := &types.AuthProfileStore{
			Version:  common.AuthStoreVersion,
			Profiles: make(map[string]map[string]interface{}),
		}
		applyLegacyStore(store, legacy)
		SyncExternalCliCredentials(store, cliReaders)
		return store
	}

	store := &types.AuthProfileStore{
		Version:  common.AuthStoreVersion,
		Profiles: make(map[string]map[string]interface{}),
	}
	SyncExternalCliCredentials(store, cliReaders)
	return store
}

// LoadAuthProfileStoreForAgent 为指定代理加载 Profile Store。
func LoadAuthProfileStoreForAgent(agentDir string, options *LoadAuthProfileStoreOptions, cliReaders map[string]ExternalCliCredentialReader) *types.AuthProfileStore {
	readOnly := options != nil && options.ReadOnly
	authPath := common.ResolveAuthStorePath(agentDir)
	raw := loadJsonFile(authPath)
	asStore := coerceAuthStore(raw)
	if asStore != nil {
		synced := SyncExternalCliCredentials(asStore, cliReaders)
		if synced && !readOnly {
			_ = saveJsonFile(authPath, asStore)
		}
		return asStore
	}

	// 回退：从主代理继承
	if agentDir != "" && !readOnly {
		mainAuthPath := common.ResolveAuthStorePath("")
		mainRaw := loadJsonFile(mainAuthPath)
		mainStore := coerceAuthStore(mainRaw)
		if mainStore != nil && len(mainStore.Profiles) > 0 {
			_ = saveJsonFile(authPath, mainStore)
			log.Printf("[auth-profiles] 从主代理继承了 auth-profiles, agentDir=%s", agentDir)
			return mainStore
		}
	}

	legacyRaw := loadJsonFile(common.ResolveLegacyAuthStorePath(agentDir))
	legacy := coerceLegacyStore(legacyRaw)
	store := &types.AuthProfileStore{
		Version:  common.AuthStoreVersion,
		Profiles: make(map[string]map[string]interface{}),
	}
	if legacy != nil {
		applyLegacyStore(store, legacy)
	}

	syncedCli := SyncExternalCliCredentials(store, cliReaders)
	forceReadOnly := os.Getenv("OPENCLAW_AUTH_STORE_READONLY") == "1"
	shouldWrite := !readOnly && !forceReadOnly && (legacy != nil || syncedCli)
	if shouldWrite {
		_ = saveJsonFile(authPath, store)
	}

	// 删除旧版文件（避免重复迁移）
	if shouldWrite && legacy != nil {
		legacyPath := common.ResolveLegacyAuthStorePath(agentDir)
		if err := os.Remove(legacyPath); err != nil && !os.IsNotExist(err) {
			log.Printf("[auth-profiles] 删除旧版 auth.json 失败: %v", err)
		}
	}

	return store
}

// LoadAuthProfileStoreForRuntime 为运行时加载 Profile Store（合并主代理和子代理）。
// 对应 TS: loadAuthProfileStoreForRuntime()
func LoadAuthProfileStoreForRuntime(agentDir string, options *LoadAuthProfileStoreOptions, cliReaders map[string]ExternalCliCredentialReader) *types.AuthProfileStore {
	store := LoadAuthProfileStoreForAgent(agentDir, options, cliReaders)
	authPath := common.ResolveAuthStorePath(agentDir)
	mainAuthPath := common.ResolveAuthStorePath("")
	if agentDir == "" || authPath == mainAuthPath {
		return store
	}
	mainStore := LoadAuthProfileStoreForAgent("", options, cliReaders)
	return MergeAuthProfileStores(mainStore, store)
}

// LoadAuthProfileStoreForSecretsRuntime 为密钥解析运行时加载（只读）。
// 对应 TS: loadAuthProfileStoreForSecretsRuntime()
func LoadAuthProfileStoreForSecretsRuntime(agentDir string, cliReaders map[string]ExternalCliCredentialReader) *types.AuthProfileStore {
	return LoadAuthProfileStoreForRuntime(agentDir, &LoadAuthProfileStoreOptions{ReadOnly: true, AllowKeychainPrompt: false}, cliReaders)
}

// EnsureAuthProfileStore 确保获取可用的 Profile Store（优先使用运行时快照）。
// 对应 TS: ensureAuthProfileStore()
func EnsureAuthProfileStore(agentDir string, cliReaders map[string]ExternalCliCredentialReader) *types.AuthProfileStore {
	runtimeStore := resolveRuntimeAuthProfileStore(agentDir)
	if runtimeStore != nil {
		return runtimeStore
	}

	store := LoadAuthProfileStoreForAgent(agentDir, nil, cliReaders)
	authPath := common.ResolveAuthStorePath(agentDir)
	mainAuthPath := common.ResolveAuthStorePath("")
	if agentDir == "" || authPath == mainAuthPath {
		return store
	}

	mainStore := LoadAuthProfileStoreForAgent("", nil, cliReaders)
	return MergeAuthProfileStores(mainStore, store)
}

// SaveAuthProfileStore 保存 Profile Store 到磁盘。
// 对应 TS: saveAuthProfileStore()
func SaveAuthProfileStore(store *types.AuthProfileStore, agentDir string) error {
	authPath := common.ResolveAuthStorePath(agentDir)

	// 清理含 keyRef/tokenRef 的明文值
	profiles := make(map[string]map[string]interface{})
	for profileId, credential := range store.Profiles {
		entry := make(map[string]interface{})
		for k, v := range credential {
			entry[k] = v
		}
		credType, _ := entry["type"].(string)
		if credType == "api_key" {
			if _, hasRef := entry["keyRef"]; hasRef {
				delete(entry, "key")
			}
		}
		if credType == "token" {
			if _, hasRef := entry["tokenRef"]; hasRef {
				delete(entry, "token")
			}
		}
		profiles[profileId] = entry
	}

	payload := &types.AuthProfileStore{
		Version:    common.AuthStoreVersion,
		Profiles:   profiles,
		Order:      store.Order,
		LastGood:   store.LastGood,
		UsageStats: store.UsageStats,
	}

	return saveJsonFile(authPath, payload)
}
