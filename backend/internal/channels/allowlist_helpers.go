package channels

import (
	"fmt"
	"strings"
)

// ── 白名单辅助工具 (W3-D2) ──

// InterfaceSliceToStringSlice 将 []interface{} 转换为 []string。
// 通用辅助函数，从各子包（imessage.normalizeAllowList / signal.interfaceSliceToStringSlice）
// 提取到父包，消除重复。
func InterfaceSliceToStringSlice(list []interface{}) []string {
	result := make([]string, 0, len(list))
	for _, entry := range list {
		s := strings.TrimSpace(fmt.Sprintf("%v", entry))
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// NormalizeAndDeduplicateAllowlist 从 []interface{} 转换并执行大小写不敏感去重。
// 组合 InterfaceSliceToStringSlice + DeduplicateAllowlist。
func NormalizeAndDeduplicateAllowlist(list []interface{}) []string {
	return DeduplicateAllowlist(InterfaceSliceToStringSlice(list))
}
