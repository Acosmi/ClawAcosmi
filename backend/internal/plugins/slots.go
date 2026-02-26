package plugins

import "fmt"

// SlotKeyForPluginKind 根据插件 Kind 返回对应的槽位 key
// 对应 TS: slots.ts slotKeyForPluginKind
func SlotKeyForPluginKind(kind PluginKind) string {
	switch kind {
	case PluginKindMemory:
		return "memory"
	}
	return ""
}

// DefaultSlotIdForKey 返回给定槽位 key 的默认插件 ID
// 对应 TS: slots.ts defaultSlotIdForKey
func DefaultSlotIdForKey(slotKey string) string {
	switch slotKey {
	case "memory":
		return "memory-core"
	}
	return ""
}

// SlotSelectionResult 槽位选择结果
type SlotSelectionResult struct {
	Config   map[string]interface{}
	Warnings []string
	Changed  bool
}

// ApplyExclusiveSlotSelection 互斥槽位选择
// 对应 TS: slots.ts applyExclusiveSlotSelection
func ApplyExclusiveSlotSelection(
	config map[string]interface{},
	selectedID string,
	selectedKind PluginKind,
	registryPlugins []SlotPluginRecord,
) SlotSelectionResult {
	slotKey := SlotKeyForPluginKind(selectedKind)
	if slotKey == "" {
		return SlotSelectionResult{Config: config, Changed: false}
	}

	var warnings []string
	pluginsConfig, _ := config["plugins"].(map[string]interface{})
	if pluginsConfig == nil {
		pluginsConfig = make(map[string]interface{})
	}

	slotsMap, _ := pluginsConfig["slots"].(map[string]interface{})
	if slotsMap == nil {
		slotsMap = make(map[string]interface{})
	}
	prevSlot, _ := slotsMap[slotKey].(string)

	newSlots := make(map[string]interface{})
	for k, v := range slotsMap {
		newSlots[k] = v
	}
	newSlots[slotKey] = selectedID

	inferredPrevSlot := prevSlot
	if inferredPrevSlot == "" {
		inferredPrevSlot = DefaultSlotIdForKey(slotKey)
	}
	if inferredPrevSlot != "" && inferredPrevSlot != selectedID {
		warnings = append(warnings, fmt.Sprintf(
			"Exclusive slot %q switched from %q to %q.",
			slotKey, inferredPrevSlot, selectedID,
		))
	}

	entriesMap, _ := pluginsConfig["entries"].(map[string]interface{})
	newEntries := make(map[string]interface{})
	for k, v := range entriesMap {
		newEntries[k] = v
	}

	var disabledIDs []string
	for _, p := range registryPlugins {
		if p.ID == selectedID {
			continue
		}
		if p.Kind != selectedKind {
			continue
		}
		entry, _ := newEntries[p.ID].(map[string]interface{})
		if entry == nil {
			entry = make(map[string]interface{})
		}
		if enabled, ok := entry["enabled"].(bool); ok && !enabled {
			continue
		}
		entryClone := make(map[string]interface{})
		for k, v := range entry {
			entryClone[k] = v
		}
		entryClone["enabled"] = false
		newEntries[p.ID] = entryClone
		disabledIDs = append(disabledIDs, p.ID)
	}

	if len(disabledIDs) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"Disabled other %q slot plugins: %s.",
			slotKey, joinSorted(disabledIDs),
		))
	}

	changed := prevSlot != selectedID || len(disabledIDs) > 0
	if !changed {
		return SlotSelectionResult{Config: config, Changed: false}
	}

	newPluginsConfig := make(map[string]interface{})
	for k, v := range pluginsConfig {
		newPluginsConfig[k] = v
	}
	newPluginsConfig["slots"] = newSlots
	newPluginsConfig["entries"] = newEntries

	newConfig := make(map[string]interface{})
	for k, v := range config {
		newConfig[k] = v
	}
	newConfig["plugins"] = newPluginsConfig

	return SlotSelectionResult{
		Config:   newConfig,
		Warnings: warnings,
		Changed:  true,
	}
}

// SlotPluginRecord 简化的插件记录（用于槽位计算）
type SlotPluginRecord struct {
	ID   string
	Kind PluginKind
}

// joinSorted 排序后 join
func joinSorted(ids []string) string {
	// 简单实现: sort + join
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	// 冒泡排序（列表通常很短）
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	result := ""
	for i, s := range sorted {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}
