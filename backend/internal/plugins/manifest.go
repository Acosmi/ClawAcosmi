package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// PluginManifestFilename 默认 manifest 文件名
const PluginManifestFilename = "openacosmi.plugin.json"

// PluginManifest 插件清单
// 对应 TS: manifest.ts PluginManifest
type PluginManifest struct {
	ID           string                        `json:"id"`
	ConfigSchema map[string]interface{}        `json:"configSchema"`
	Kind         PluginKind                    `json:"kind,omitempty"`
	Channels     []string                      `json:"channels,omitempty"`
	Providers    []string                      `json:"providers,omitempty"`
	Skills       []string                      `json:"skills,omitempty"`
	Name         string                        `json:"name,omitempty"`
	Description  string                        `json:"description,omitempty"`
	Version      string                        `json:"version,omitempty"`
	UiHints      map[string]PluginConfigUiHint `json:"uiHints,omitempty"`
}

// PluginManifestLoadResult manifest 加载结果
type PluginManifestLoadResult struct {
	OK           bool
	Manifest     *PluginManifest
	ManifestPath string
	Error        string
}

// ResolvePluginManifestPath 查找 manifest 文件路径
// 对应 TS: manifest.ts resolvePluginManifestPath
func ResolvePluginManifestPath(rootDir string) string {
	candidate := filepath.Join(rootDir, PluginManifestFilename)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return filepath.Join(rootDir, PluginManifestFilename)
}

// LoadPluginManifest 加载并解析 manifest
// 对应 TS: manifest.ts loadPluginManifest
func LoadPluginManifest(rootDir string) PluginManifestLoadResult {
	manifestPath := ResolvePluginManifestPath(rootDir)

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return PluginManifestLoadResult{
			OK:           false,
			ManifestPath: manifestPath,
			Error:        "plugin manifest not found: " + manifestPath,
		}
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return PluginManifestLoadResult{
			OK:           false,
			ManifestPath: manifestPath,
			Error:        "failed to parse plugin manifest: " + err.Error(),
		}
	}

	id := stringField(raw, "id")
	if id == "" {
		return PluginManifestLoadResult{
			OK:           false,
			ManifestPath: manifestPath,
			Error:        "plugin manifest requires id",
		}
	}

	configSchemaRaw, ok := raw["configSchema"]
	if !ok {
		return PluginManifestLoadResult{
			OK:           false,
			ManifestPath: manifestPath,
			Error:        "plugin manifest requires configSchema",
		}
	}
	configSchema, ok := configSchemaRaw.(map[string]interface{})
	if !ok {
		return PluginManifestLoadResult{
			OK:           false,
			ManifestPath: manifestPath,
			Error:        "plugin manifest requires configSchema",
		}
	}

	manifest := &PluginManifest{
		ID:           id,
		ConfigSchema: configSchema,
		Kind:         PluginKind(stringField(raw, "kind")),
		Name:         stringField(raw, "name"),
		Description:  stringField(raw, "description"),
		Version:      stringField(raw, "version"),
		Channels:     normalizeStringListField(raw, "channels"),
		Providers:    normalizeStringListField(raw, "providers"),
		Skills:       normalizeStringListField(raw, "skills"),
	}

	if hints, ok := raw["uiHints"].(map[string]interface{}); ok {
		manifest.UiHints = make(map[string]PluginConfigUiHint)
		for k, v := range hints {
			if hm, ok := v.(map[string]interface{}); ok {
				manifest.UiHints[k] = PluginConfigUiHint{
					Label:       stringField(hm, "label"),
					Help:        stringField(hm, "help"),
					Advanced:    boolField(hm, "advanced"),
					Sensitive:   boolField(hm, "sensitive"),
					Placeholder: stringField(hm, "placeholder"),
				}
			}
		}
	}

	return PluginManifestLoadResult{
		OK:           true,
		Manifest:     manifest,
		ManifestPath: manifestPath,
	}
}

// PluginPackageChannel package.json 渠道元数据
// 对应 TS: manifest.ts PluginPackageChannel
type PluginPackageChannel struct {
	ID                                   string   `json:"id,omitempty"`
	Label                                string   `json:"label,omitempty"`
	SelectionLabel                       string   `json:"selectionLabel,omitempty"`
	DetailLabel                          string   `json:"detailLabel,omitempty"`
	DocsPath                             string   `json:"docsPath,omitempty"`
	DocsLabel                            string   `json:"docsLabel,omitempty"`
	Blurb                                string   `json:"blurb,omitempty"`
	Order                                int      `json:"order,omitempty"`
	Aliases                              []string `json:"aliases,omitempty"`
	PreferOver                           []string `json:"preferOver,omitempty"`
	SystemImage                          string   `json:"systemImage,omitempty"`
	SelectionDocsPrefix                  string   `json:"selectionDocsPrefix,omitempty"`
	SelectionDocsOmitLabel               bool     `json:"selectionDocsOmitLabel,omitempty"`
	SelectionExtras                      []string `json:"selectionExtras,omitempty"`
	ShowConfigured                       bool     `json:"showConfigured,omitempty"`
	QuickstartAllowFrom                  bool     `json:"quickstartAllowFrom,omitempty"`
	ForceAccountBinding                  bool     `json:"forceAccountBinding,omitempty"`
	PreferSessionLookupForAnnounceTarget bool     `json:"preferSessionLookupForAnnounceTarget,omitempty"`
}

// PluginPackageInstall 安装选项
type PluginPackageInstall struct {
	NpmSpec       string `json:"npmSpec,omitempty"`
	LocalPath     string `json:"localPath,omitempty"`
	DefaultChoice string `json:"defaultChoice,omitempty"` // "npm" | "local"
}

// OpenAcosmiPackageManifest package.json openacosmi 字段
type OpenAcosmiPackageManifest struct {
	Extensions []string              `json:"extensions,omitempty"`
	Channel    *PluginPackageChannel `json:"channel,omitempty"`
	Install    *PluginPackageInstall `json:"install,omitempty"`
}

// --- helpers ---

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func boolField(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func normalizeStringListField(m map[string]interface{}, key string) []string {
	arr, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
