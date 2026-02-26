package config

// merge_patch.go — RFC 7396 JSON Merge Patch 实现
// TS 参考: src/config/merge-patch.ts (29L)
//
// 提供 config.patch 方法所需的深度合并逻辑。
// 规则:
//   - patch 不是 object → 直接替换 base
//   - patch 中 key 值为 nil → 从 result 中删除该 key
//   - patch 中 key 值为 object → 与 base 对应 key 递归合并
//   - 其他值类型 → 直接覆盖

// ApplyMergePatch 对 base 应用 RFC 7396 JSON Merge Patch。
// 对齐 TS: src/config/merge-patch.ts applyMergePatch()
func ApplyMergePatch(base, patch interface{}) interface{} {
	patchMap, ok := patch.(map[string]interface{})
	if !ok {
		// patch 不是 object，直接返回 patch（替换 base）
		return patch
	}

	// base 是 object 则浅拷贝，否则从空 object 开始
	result := make(map[string]interface{})
	if baseMap, ok := base.(map[string]interface{}); ok {
		for k, v := range baseMap {
			result[k] = v
		}
	}

	for key, value := range patchMap {
		if value == nil {
			// null → 删除键
			delete(result, key)
			continue
		}
		if isPlainObject(value) {
			// 子 object → 递归合并
			baseValue := result[key]
			if isPlainObject(baseValue) {
				result[key] = ApplyMergePatch(baseValue, value)
			} else {
				result[key] = ApplyMergePatch(nil, value)
			}
			continue
		}
		// 其他值类型 → 覆盖
		result[key] = value
	}

	return result
}

// isPlainObject 检查值是否为 map[string]interface{}。
func isPlainObject(value interface{}) bool {
	_, ok := value.(map[string]interface{})
	return ok
}
