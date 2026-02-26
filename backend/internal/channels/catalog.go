package channels

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 频道插件目录 — 继承自 src/channels/plugins/catalog.ts (312L)

// ChannelUiMetaEntry UI 展示用频道元数据
type ChannelUiMetaEntry struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	DetailLabel string `json:"detailLabel"`
	SystemImage string `json:"systemImage,omitempty"`
}

// ChannelUiCatalog UI 频道目录
type ChannelUiCatalog struct {
	Entries      []ChannelUiMetaEntry          `json:"entries"`
	Order        []string                      `json:"order"`
	Labels       map[string]string             `json:"labels"`
	DetailLabels map[string]string             `json:"detailLabels"`
	SystemImages map[string]string             `json:"systemImages"`
	ByID         map[string]ChannelUiMetaEntry `json:"byId"`
}

// ChannelPluginCatalogEntry 插件目录条目
type ChannelPluginCatalogEntry struct {
	ID      string           `json:"id"`
	Meta    ChannelMetaEntry `json:"meta"`
	Install struct {
		NpmSpec       string `json:"npmSpec"`
		LocalPath     string `json:"localPath,omitempty"`
		DefaultChoice string `json:"defaultChoice,omitempty"`
	} `json:"install"`
}

// ExternalCatalogEntry 外部目录条目（JSON 文件）
type ExternalCatalogEntry struct {
	Name        string `json:"name,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
}

// PluginOriginPriority 插件来源优先级
var pluginOriginPriority = map[string]int{
	"config":    0,
	"workspace": 1,
	"global":    2,
	"bundled":   3,
}

// DefaultCatalogPaths 默认目录搜索路径
func DefaultCatalogPaths(configDir string) []string {
	return []string{
		filepath.Join(configDir, "mpm", "plugins.json"),
		filepath.Join(configDir, "mpm", "catalog.json"),
		filepath.Join(configDir, "plugins", "catalog.json"),
	}
}

// LoadExternalCatalogEntries 加载外部插件目录
func LoadExternalCatalogEntries(paths []string) []ExternalCatalogEntry {
	var entries []ExternalCatalogEntry
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var parsed []ExternalCatalogEntry
		if err := json.Unmarshal(data, &parsed); err != nil {
			// 尝试解析为单个条目
			var single ExternalCatalogEntry
			if err := json.Unmarshal(data, &single); err == nil && single.Name != "" {
				entries = append(entries, single)
			}
			continue
		}
		entries = append(entries, parsed...)
	}
	return entries
}

// BuildChannelUiCatalog 从频道元数据构建 UI 目录
func BuildChannelUiCatalog(channels []ChannelMetaEntry) ChannelUiCatalog {
	catalog := ChannelUiCatalog{
		Labels:       make(map[string]string),
		DetailLabels: make(map[string]string),
		SystemImages: make(map[string]string),
		ByID:         make(map[string]ChannelUiMetaEntry),
	}
	type sortEntry struct {
		entry ChannelUiMetaEntry
		order int
	}
	var sorted []sortEntry
	for _, ch := range channels {
		uiEntry := ChannelUiMetaEntry{
			ID:          string(ch.ID),
			Label:       ch.Label,
			DetailLabel: ch.DetailLabel,
			SystemImage: ch.SystemImage,
		}
		if uiEntry.DetailLabel == "" {
			uiEntry.DetailLabel = uiEntry.Label
		}
		sorted = append(sorted, sortEntry{entry: uiEntry, order: ch.Order})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].order < sorted[j].order
	})
	for _, s := range sorted {
		catalog.Entries = append(catalog.Entries, s.entry)
		catalog.Order = append(catalog.Order, s.entry.ID)
		catalog.Labels[s.entry.ID] = s.entry.Label
		catalog.DetailLabels[s.entry.ID] = s.entry.DetailLabel
		if s.entry.SystemImage != "" {
			catalog.SystemImages[s.entry.ID] = s.entry.SystemImage
		}
		catalog.ByID[s.entry.ID] = s.entry
	}
	return catalog
}

// SplitEnvPaths 分割环境变量路径
func SplitEnvPaths(value string) []string {
	sep := string(os.PathListSeparator)
	parts := strings.Split(value, sep)
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
