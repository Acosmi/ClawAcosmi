// authprofile/store.go — Profile 存储读写（骨架 + 类型 + 辅助函数）
// 对应 TS 文件: src/agents/auth-profiles/store.ts
package authprofile

import (
	"encoding/json"
	"log"
	"strings"
	"sync"

	"github.com/Acosmi/ClawAcosmi/internal/goproviders/common"
	"github.com/Acosmi/ClawAcosmi/internal/goproviders/types"
)

// CredentialRejectReason 凭证拒绝原因。
type CredentialRejectReason string

const (
	RejectReasonNonObject       CredentialRejectReason = "non_object"
	RejectReasonInvalidType     CredentialRejectReason = "invalid_type"
	RejectReasonMissingProvider CredentialRejectReason = "missing_provider"
)

// RejectedCredentialEntry 被拒绝的凭证条目。
type RejectedCredentialEntry struct {
	Key    string
	Reason CredentialRejectReason
}

// LoadAuthProfileStoreOptions 加载选项。
type LoadAuthProfileStoreOptions struct {
	AllowKeychainPrompt bool
	ReadOnly            bool
}

// authProfileTypes 有效的凭证类型集合。
var authProfileTypes = map[string]bool{
	"api_key": true,
	"oauth":   true,
	"token":   true,
}

// runtimeAuthStoreSnapshots 运行时快照缓存。
var (
	runtimeAuthStoreSnapshots = make(map[string]*types.AuthProfileStore)
	runtimeMu                 sync.RWMutex
)

// resolveRuntimeStoreKey 解析运行时快照键。
func resolveRuntimeStoreKey(agentDir string) string {
	return common.ResolveAuthStorePath(agentDir)
}

// cloneAuthProfileStore 深拷贝 AuthProfileStore。
func cloneAuthProfileStore(store *types.AuthProfileStore) *types.AuthProfileStore {
	data, err := json.Marshal(store)
	if err != nil {
		return store
	}
	var cloned types.AuthProfileStore
	if err := json.Unmarshal(data, &cloned); err != nil {
		return store
	}
	return &cloned
}

// resolveRuntimeAuthProfileStore 从运行时快照获取 Profile Store。
func resolveRuntimeAuthProfileStore(agentDir string) *types.AuthProfileStore {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()

	if len(runtimeAuthStoreSnapshots) == 0 {
		return nil
	}

	mainKey := resolveRuntimeStoreKey("")
	requestedKey := resolveRuntimeStoreKey(agentDir)
	mainStore := runtimeAuthStoreSnapshots[mainKey]
	requestedStore := runtimeAuthStoreSnapshots[requestedKey]

	if agentDir == "" || requestedKey == mainKey {
		if mainStore == nil {
			return nil
		}
		return cloneAuthProfileStore(mainStore)
	}

	if mainStore != nil && requestedStore != nil {
		return MergeAuthProfileStores(
			cloneAuthProfileStore(mainStore),
			cloneAuthProfileStore(requestedStore),
		)
	}
	if requestedStore != nil {
		return cloneAuthProfileStore(requestedStore)
	}
	if mainStore != nil {
		return cloneAuthProfileStore(mainStore)
	}
	return nil
}

// ReplaceRuntimeAuthProfileStoreSnapshots 替换运行时快照。
// 对应 TS: replaceRuntimeAuthProfileStoreSnapshots()
func ReplaceRuntimeAuthProfileStoreSnapshots(entries []struct {
	AgentDir string
	Store    *types.AuthProfileStore
}) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	runtimeAuthStoreSnapshots = make(map[string]*types.AuthProfileStore)
	for _, entry := range entries {
		runtimeAuthStoreSnapshots[resolveRuntimeStoreKey(entry.AgentDir)] = cloneAuthProfileStore(entry.Store)
	}
}

// ClearRuntimeAuthProfileStoreSnapshots 清空运行时快照。
// 对应 TS: clearRuntimeAuthProfileStoreSnapshots()
func ClearRuntimeAuthProfileStoreSnapshots() {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	runtimeAuthStoreSnapshots = make(map[string]*types.AuthProfileStore)
}

// normalizeRawCredentialEntry 规范化原始凭证条目。
// 处理 mode → type 别名、apiKey → key 别名。
func normalizeRawCredentialEntry(raw map[string]interface{}) map[string]interface{} {
	entry := make(map[string]interface{})
	for k, v := range raw {
		entry[k] = v
	}
	// mode → type 别名
	if _, hasType := entry["type"]; !hasType {
		if mode, ok := entry["mode"].(string); ok {
			entry["type"] = mode
		}
	}
	// apiKey → key 别名
	if _, hasKey := entry["key"]; !hasKey {
		if apiKey, ok := entry["apiKey"].(string); ok {
			entry["key"] = apiKey
		}
	}
	return entry
}

// parseCredentialEntry 解析凭证条目。
func parseCredentialEntry(raw interface{}, fallbackProvider string) (map[string]interface{}, CredentialRejectReason) {
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil, RejectReasonNonObject
	}
	typed := normalizeRawCredentialEntry(rawMap)
	credType, _ := typed["type"].(string)
	if !authProfileTypes[credType] {
		return nil, RejectReasonInvalidType
	}
	provider, _ := typed["provider"].(string)
	if provider == "" {
		provider = fallbackProvider
	}
	if strings.TrimSpace(provider) == "" {
		return nil, RejectReasonMissingProvider
	}
	typed["provider"] = provider
	return typed, ""
}

// warnRejectedCredentialEntries 日志报告被拒绝的条目。
func warnRejectedCredentialEntries(source string, rejected []RejectedCredentialEntry) {
	if len(rejected) == 0 {
		return
	}
	keys := make([]string, 0, 10)
	for i, entry := range rejected {
		if i >= 10 {
			break
		}
		keys = append(keys, entry.Key)
	}
	log.Printf("[auth-profiles] 加载时忽略了 %d 个无效条目 (source=%s, keys=%v)", len(rejected), source, keys)
}
