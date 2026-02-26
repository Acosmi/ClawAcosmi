package plugins

import (
	"encoding/json"
	"os"
	"time"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// UpdateChannel 更新渠道类型
// 对应 TS: infra/update-channels.ts UpdateChannel
type UpdateChannel string

const (
	UpdateChannelDev     UpdateChannel = "dev"
	UpdateChannelStable  UpdateChannel = "stable"
	UpdateChannelRelease UpdateChannel = "release"
)

// PluginUpdateStatus 更新状态
type PluginUpdateStatus string

const (
	PluginUpdateStatusUpdated   PluginUpdateStatus = "updated"
	PluginUpdateStatusUnchanged PluginUpdateStatus = "unchanged"
	PluginUpdateStatusSkipped   PluginUpdateStatus = "skipped"
	PluginUpdateStatusError     PluginUpdateStatus = "error"
)

// PluginUpdateOutcome 单个插件更新结果
type PluginUpdateOutcome struct {
	PluginID       string             `json:"pluginId"`
	Status         PluginUpdateStatus `json:"status"`
	Message        string             `json:"message"`
	CurrentVersion string             `json:"currentVersion,omitempty"`
	NextVersion    string             `json:"nextVersion,omitempty"`
}

// PluginUpdateSummary 更新汇总
type PluginUpdateSummary struct {
	Config   *types.OpenAcosmiConfig `json:"config"`
	Changed  bool                    `json:"changed"`
	Outcomes []PluginUpdateOutcome   `json:"outcomes"`
}

// UpdateLogger 更新日志接口
type UpdateLogger struct {
	Info  func(message string)
	Warn  func(message string)
	Error func(message string)
}

// PluginChannelSyncSummary 渠道同步汇总
type PluginChannelSyncSummary struct {
	SwitchedToBundled []string `json:"switchedToBundled"`
	SwitchedToNpm     []string `json:"switchedToNpm"`
	Warnings          []string `json:"warnings"`
	Errors            []string `json:"errors"`
}

// PluginChannelSyncResult 渠道同步结果
type PluginChannelSyncResult struct {
	Config  *types.OpenAcosmiConfig  `json:"config"`
	Changed bool                     `json:"changed"`
	Summary PluginChannelSyncSummary `json:"summary"`
}

// UpdateNpmInstalledPlugins 更新已安装的 npm 插件
// 对应 TS: update.ts updateNpmInstalledPlugins
func UpdateNpmInstalledPlugins(params UpdateNpmParams) PluginUpdateSummary {
	logger := params.Logger
	installs := getInstalls(params.Config)
	targets := params.PluginIDs
	if len(targets) == 0 {
		targets = installKeys(installs)
	}

	outcomes := make([]PluginUpdateOutcome, 0, len(targets))
	next := cloneConfig(params.Config)
	changed := false

	for _, pluginID := range targets {
		if params.SkipIDs != nil && params.SkipIDs[pluginID] {
			outcomes = append(outcomes, PluginUpdateOutcome{
				PluginID: pluginID,
				Status:   PluginUpdateStatusSkipped,
				Message:  "Skipping \"" + pluginID + "\" (already updated).",
			})
			continue
		}

		record := installs[pluginID]
		if record == nil {
			outcomes = append(outcomes, PluginUpdateOutcome{
				PluginID: pluginID,
				Status:   PluginUpdateStatusSkipped,
				Message:  "No install record for \"" + pluginID + "\".",
			})
			continue
		}

		if record.Source != "npm" {
			outcomes = append(outcomes, PluginUpdateOutcome{
				PluginID: pluginID,
				Status:   PluginUpdateStatusSkipped,
				Message:  "Skipping \"" + pluginID + "\" (source: " + record.Source + ").",
			})
			continue
		}

		if record.Spec == "" {
			outcomes = append(outcomes, PluginUpdateOutcome{
				PluginID: pluginID,
				Status:   PluginUpdateStatusSkipped,
				Message:  "Skipping \"" + pluginID + "\" (missing npm spec).",
			})
			continue
		}

		installPath := record.InstallPath
		if installPath == "" {
			resolved, err := ResolvePluginInstallDir(pluginID, "")
			if err != nil {
				outcomes = append(outcomes, PluginUpdateOutcome{
					PluginID: pluginID,
					Status:   PluginUpdateStatusError,
					Message:  "Invalid install path for \"" + pluginID + "\": " + err.Error(),
				})
				continue
			}
			installPath = resolved
		}

		currentVersion := readInstalledPackageVersion(installPath)

		// dry-run 模式
		if params.DryRun {
			probe := InstallPluginFromNpmSpec(InstallFromNpmParams{
				Spec:             record.Spec,
				Mode:             "update",
				DryRun:           true,
				ExpectedPluginID: pluginID,
				Logger:           InstallLogger{Info: logger.Info, Warn: logger.Warn},
			})
			if !probe.OK {
				outcomes = append(outcomes, PluginUpdateOutcome{
					PluginID: pluginID,
					Status:   PluginUpdateStatusError,
					Message:  "Failed to check " + pluginID + ": " + probe.Error,
				})
				continue
			}
			nextVersion := coalesce(probe.Version, "unknown")
			currentLabel := coalesce(currentVersion, "unknown")
			if currentVersion != "" && probe.Version != "" && currentVersion == probe.Version {
				outcomes = append(outcomes, PluginUpdateOutcome{
					PluginID:       pluginID,
					Status:         PluginUpdateStatusUnchanged,
					CurrentVersion: currentVersion,
					NextVersion:    probe.Version,
					Message:        pluginID + " is up to date (" + currentLabel + ").",
				})
			} else {
				outcomes = append(outcomes, PluginUpdateOutcome{
					PluginID:       pluginID,
					Status:         PluginUpdateStatusUpdated,
					CurrentVersion: currentVersion,
					NextVersion:    probe.Version,
					Message:        "Would update " + pluginID + ": " + currentLabel + " -> " + nextVersion + ".",
				})
			}
			continue
		}

		// 实际更新
		result := InstallPluginFromNpmSpec(InstallFromNpmParams{
			Spec:             record.Spec,
			Mode:             "update",
			ExpectedPluginID: pluginID,
			Logger:           InstallLogger{Info: logger.Info, Warn: logger.Warn},
		})
		if !result.OK {
			outcomes = append(outcomes, PluginUpdateOutcome{
				PluginID: pluginID,
				Status:   PluginUpdateStatusError,
				Message:  "Failed to update " + pluginID + ": " + result.Error,
			})
			continue
		}

		nextVersion := result.Version
		if nextVersion == "" {
			nextVersion = readInstalledPackageVersion(result.TargetDir)
		}

		RecordPluginInstall(next, PluginInstallUpdate{
			PluginID:    pluginID,
			Source:      "npm",
			Spec:        record.Spec,
			InstallPath: result.TargetDir,
			Version:     nextVersion,
		})
		changed = true

		currentLabel := coalesce(currentVersion, "unknown")
		nextLabel := coalesce(nextVersion, "unknown")
		if currentVersion != "" && nextVersion != "" && currentVersion == nextVersion {
			outcomes = append(outcomes, PluginUpdateOutcome{
				PluginID:       pluginID,
				Status:         PluginUpdateStatusUnchanged,
				CurrentVersion: currentVersion,
				NextVersion:    nextVersion,
				Message:        pluginID + " already at " + currentLabel + ".",
			})
		} else {
			outcomes = append(outcomes, PluginUpdateOutcome{
				PluginID:       pluginID,
				Status:         PluginUpdateStatusUpdated,
				CurrentVersion: currentVersion,
				NextVersion:    nextVersion,
				Message:        "Updated " + pluginID + ": " + currentLabel + " -> " + nextLabel + ".",
			})
		}
	}

	return PluginUpdateSummary{Config: next, Changed: changed, Outcomes: outcomes}
}

// UpdateNpmParams 更新参数
type UpdateNpmParams struct {
	Config    *types.OpenAcosmiConfig
	Logger    UpdateLogger
	PluginIDs []string
	SkipIDs   map[string]bool
	DryRun    bool
}

// SyncPluginsForUpdateChannel 按渠道同步插件
// 对应 TS: update.ts syncPluginsForUpdateChannel
func SyncPluginsForUpdateChannel(params SyncChannelParams) PluginChannelSyncResult {
	summary := PluginChannelSyncSummary{
		SwitchedToBundled: make([]string, 0),
		SwitchedToNpm:     make([]string, 0),
		Warnings:          make([]string, 0),
		Errors:            make([]string, 0),
	}

	bundled := resolveBundledPluginSources(params.WorkspaceDir)
	if len(bundled) == 0 {
		return PluginChannelSyncResult{Config: params.Config, Changed: false, Summary: summary}
	}

	next := cloneConfig(params.Config)
	installs := getInstalls(next)
	loadHelper := newLoadPathHelper(getLoadPaths(next))
	changed := false

	if params.Channel == UpdateChannelDev {
		for pluginID, record := range installs {
			info, ok := bundled[pluginID]
			if !ok {
				continue
			}
			loadHelper.addPath(info.localPath)

			alreadyBundled := record.Source == "path" && pathsEqual(record.SourcePath, info.localPath)
			if alreadyBundled {
				continue
			}

			RecordPluginInstall(next, PluginInstallUpdate{
				PluginID:    pluginID,
				Source:      "path",
				SourcePath:  info.localPath,
				InstallPath: info.localPath,
				Spec:        coalesce(record.Spec, info.npmSpec),
				Version:     record.Version,
			})
			summary.SwitchedToBundled = append(summary.SwitchedToBundled, pluginID)
			changed = true
		}
	} else {
		for pluginID, record := range installs {
			info, ok := bundled[pluginID]
			if !ok {
				continue
			}

			if record.Source == "npm" {
				loadHelper.removePath(info.localPath)
				continue
			}
			if record.Source != "path" {
				continue
			}
			if !pathsEqual(record.SourcePath, info.localPath) {
				continue
			}

			spec := coalesce(record.Spec, info.npmSpec)
			if spec == "" {
				summary.Warnings = append(summary.Warnings, "Missing npm spec for "+pluginID+"; keeping local path.")
				continue
			}

			result := InstallPluginFromNpmSpec(InstallFromNpmParams{
				Spec:             spec,
				Mode:             "update",
				ExpectedPluginID: pluginID,
				Logger:           InstallLogger{Info: params.Logger.Info, Warn: params.Logger.Warn},
			})
			if !result.OK {
				summary.Errors = append(summary.Errors, "Failed to install "+pluginID+": "+result.Error)
				continue
			}

			RecordPluginInstall(next, PluginInstallUpdate{
				PluginID:    pluginID,
				Source:      "npm",
				Spec:        spec,
				InstallPath: result.TargetDir,
				Version:     result.Version,
			})
			summary.SwitchedToNpm = append(summary.SwitchedToNpm, pluginID)
			changed = true
			loadHelper.removePath(info.localPath)
		}
	}

	if loadHelper.changed {
		if next.Plugins == nil {
			next.Plugins = &types.PluginsConfig{}
		}
		if next.Plugins.Load == nil {
			next.Plugins.Load = &types.PluginsLoadConfig{}
		}
		next.Plugins.Load.Paths = loadHelper.paths
		changed = true
	}

	return PluginChannelSyncResult{Config: next, Changed: changed, Summary: summary}
}

// SyncChannelParams 渠道同步参数
type SyncChannelParams struct {
	Config       *types.OpenAcosmiConfig
	Channel      UpdateChannel
	WorkspaceDir string
	Logger       UpdateLogger
}

// --- RecordPluginInstall ---

// PluginInstallUpdate 安装记录更新
// 对应 TS: installs.ts PluginInstallUpdate
type PluginInstallUpdate struct {
	PluginID    string
	Source      string
	Spec        string
	SourcePath  string
	InstallPath string
	Version     string
}

// RecordPluginInstall 记录插件安装信息到配置
// 对应 TS: installs.ts recordPluginInstall
func RecordPluginInstall(cfg *types.OpenAcosmiConfig, update PluginInstallUpdate) {
	if cfg.Plugins == nil {
		cfg.Plugins = &types.PluginsConfig{}
	}
	if cfg.Plugins.Installs == nil {
		cfg.Plugins.Installs = make(map[string]*types.PluginInstallRecord)
	}

	existing := cfg.Plugins.Installs[update.PluginID]
	installedAt := time.Now().UTC().Format(time.RFC3339)
	if existing != nil && existing.InstalledAt != "" {
		installedAt = existing.InstalledAt
	}

	cfg.Plugins.Installs[update.PluginID] = &types.PluginInstallRecord{
		Source:      update.Source,
		Spec:        update.Spec,
		SourcePath:  update.SourcePath,
		InstallPath: update.InstallPath,
		Version:     update.Version,
		InstalledAt: installedAt,
	}
}

// --- bundled plugin sources ---

type bundledPluginSource struct {
	pluginID  string
	localPath string
	npmSpec   string
}

func resolveBundledPluginSources(workspaceDir string) map[string]bundledPluginSource {
	discovery := DiscoverPlugins(workspaceDir, nil)
	bundled := make(map[string]bundledPluginSource)

	for _, candidate := range discovery.Candidates {
		if candidate.Origin != PluginOriginBundled {
			continue
		}
		manifest := LoadPluginManifest(candidate.RootDir)
		if !manifest.OK {
			continue
		}
		pluginID := manifest.Manifest.ID
		if _, exists := bundled[pluginID]; exists {
			continue
		}

		var npmSpec string
		if candidate.PackageManifest != nil && candidate.PackageManifest.Install != nil {
			npmSpec = candidate.PackageManifest.Install.NpmSpec
		}
		if npmSpec == "" {
			npmSpec = candidate.PackageName
		}

		bundled[pluginID] = bundledPluginSource{
			pluginID:  pluginID,
			localPath: candidate.RootDir,
			npmSpec:   npmSpec,
		}
	}
	return bundled
}

// --- load path helpers ---

type loadPathHelper struct {
	paths    []string
	resolved map[string]bool
	changed  bool
}

func newLoadPathHelper(existing []string) *loadPathHelper {
	h := &loadPathHelper{
		paths:    make([]string, len(existing)),
		resolved: make(map[string]bool),
	}
	copy(h.paths, existing)
	for _, p := range existing {
		h.resolved[resolveUserPath(p)] = true
	}
	return h
}

func (h *loadPathHelper) addPath(value string) {
	normalized := resolveUserPath(value)
	if h.resolved[normalized] {
		return
	}
	h.paths = append(h.paths, value)
	h.resolved[normalized] = true
	h.changed = true
}

func (h *loadPathHelper) removePath(value string) {
	normalized := resolveUserPath(value)
	if !h.resolved[normalized] {
		return
	}
	filtered := make([]string, 0, len(h.paths))
	for _, p := range h.paths {
		if resolveUserPath(p) != normalized {
			filtered = append(filtered, p)
		}
	}
	h.paths = filtered
	delete(h.resolved, normalized)
	h.changed = true
}

// --- helpers ---

func readInstalledPackageVersion(dir string) string {
	data, err := os.ReadFile(dir + "/package.json")
	if err != nil {
		return ""
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Version
}

func getInstalls(cfg *types.OpenAcosmiConfig) map[string]*types.PluginInstallRecord {
	if cfg == nil || cfg.Plugins == nil || cfg.Plugins.Installs == nil {
		return make(map[string]*types.PluginInstallRecord)
	}
	return cfg.Plugins.Installs
}

func getLoadPaths(cfg *types.OpenAcosmiConfig) []string {
	if cfg == nil || cfg.Plugins == nil || cfg.Plugins.Load == nil {
		return nil
	}
	return cfg.Plugins.Load.Paths
}

func installKeys(m map[string]*types.PluginInstallRecord) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func cloneConfig(cfg *types.OpenAcosmiConfig) *types.OpenAcosmiConfig {
	if cfg == nil {
		return &types.OpenAcosmiConfig{}
	}
	data, _ := json.Marshal(cfg)
	var cloned types.OpenAcosmiConfig
	_ = json.Unmarshal(data, &cloned)
	return &cloned
}

func pathsEqual(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return resolveUserPath(left) == resolveUserPath(right)
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
