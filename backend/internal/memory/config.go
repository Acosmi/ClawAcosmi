package memory

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// MemoryBackend selects the indexing backend.
type MemoryBackend string

const (
	BackendBuiltin MemoryBackend = "builtin"
	BackendQMD     MemoryBackend = "qmd"
)

// MemoryCitationsMode controls citation inclusion.
type MemoryCitationsMode string

const (
	CitationsAuto MemoryCitationsMode = "auto"
	CitationsOn   MemoryCitationsMode = "on"
	CitationsOff  MemoryCitationsMode = "off"
)

// ResolvedMemoryBackendConfig is the fully-resolved memory backend config.
type ResolvedMemoryBackendConfig struct {
	Backend   MemoryBackend       `json:"backend"`
	Citations MemoryCitationsMode `json:"citations"`
	QMD       *ResolvedQmdConfig  `json:"qmd,omitempty"`
}

// ResolvedQmdCollection describes one index collection.
type ResolvedQmdCollection struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
	Kind    string `json:"kind"` // "memory" | "custom" | "sessions"
}

// ResolvedQmdUpdateConfig holds timing parameters for QMD updates.
type ResolvedQmdUpdateConfig struct {
	IntervalMs       int64 `json:"intervalMs"`
	DebounceMs       int64 `json:"debounceMs"`
	OnBoot           bool  `json:"onBoot"`
	WaitForBootSync  bool  `json:"waitForBootSync"`
	EmbedIntervalMs  int64 `json:"embedIntervalMs"`
	CommandTimeoutMs int64 `json:"commandTimeoutMs"`
	UpdateTimeoutMs  int64 `json:"updateTimeoutMs"`
	EmbedTimeoutMs   int64 `json:"embedTimeoutMs"`
}

// ResolvedQmdLimitsConfig holds search limit parameters.
type ResolvedQmdLimitsConfig struct {
	MaxResults       int   `json:"maxResults"`
	MaxSnippetChars  int   `json:"maxSnippetChars"`
	MaxInjectedChars int   `json:"maxInjectedChars"`
	TimeoutMs        int64 `json:"timeoutMs"`
}

// ResolvedQmdSessionConfig controls session transcript indexing.
type ResolvedQmdSessionConfig struct {
	Enabled       bool   `json:"enabled"`
	ExportDir     string `json:"exportDir,omitempty"`
	RetentionDays *int   `json:"retentionDays,omitempty"`
}

// ResolvedQmdConfig is the fully-resolved QMD backend config.
type ResolvedQmdConfig struct {
	Command              string                   `json:"command"`
	Collections          []ResolvedQmdCollection  `json:"collections"`
	Sessions             ResolvedQmdSessionConfig `json:"sessions"`
	Update               ResolvedQmdUpdateConfig  `json:"update"`
	Limits               ResolvedQmdLimitsConfig  `json:"limits"`
	IncludeDefaultMemory bool                     `json:"includeDefaultMemory"`
}

// Config defaults.
const (
	defaultBackend                   = BackendBuiltin
	defaultCitations                 = CitationsAuto
	defaultQmdIntervalMs             = int64(5 * time.Minute / time.Millisecond)
	defaultQmdDebounceMs       int64 = 15_000
	defaultQmdTimeoutMs        int64 = 4_000
	defaultQmdEmbedIntervalMs        = int64(60 * time.Minute / time.Millisecond)
	defaultQmdCommandTimeoutMs int64 = 30_000
	defaultQmdUpdateTimeoutMs  int64 = 120_000
	defaultQmdEmbedTimeoutMs   int64 = 120_000
)

var defaultQmdLimits = ResolvedQmdLimitsConfig{
	MaxResults:       6,
	MaxSnippetChars:  700,
	MaxInjectedChars: 4_000,
	TimeoutMs:        defaultQmdTimeoutMs,
}

// MemoryConfigInput is a simplified config shape used by ResolveMemoryBackendConfig.
// The full OpenAcosmiConfig type lives in pkg/types; this avoids an import cycle.
type MemoryConfigInput struct {
	Backend   string          `json:"backend,omitempty"`
	Citations string          `json:"citations,omitempty"`
	QMD       *QmdConfigInput `json:"qmd,omitempty"`
}

// QmdConfigInput is raw user config for QMD.
type QmdConfigInput struct {
	Command              string              `json:"command,omitempty"`
	Paths                []QmdIndexPathInput `json:"paths,omitempty"`
	Sessions             *QmdSessionInput    `json:"sessions,omitempty"`
	Update               *QmdUpdateInput     `json:"update,omitempty"`
	Limits               *QmdLimitsInput     `json:"limits,omitempty"`
	IncludeDefaultMemory *bool               `json:"includeDefaultMemory,omitempty"`
}

// QmdIndexPathInput is a single custom index path entry.
type QmdIndexPathInput struct {
	Name    string `json:"name,omitempty"`
	Path    string `json:"path,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// QmdSessionInput is raw session config.
type QmdSessionInput struct {
	Enabled       *bool  `json:"enabled,omitempty"`
	ExportDir     string `json:"exportDir,omitempty"`
	RetentionDays *int   `json:"retentionDays,omitempty"`
}

// QmdUpdateInput is raw update config.
type QmdUpdateInput struct {
	Interval         string `json:"interval,omitempty"`
	DebounceMs       *int64 `json:"debounceMs,omitempty"`
	OnBoot           *bool  `json:"onBoot,omitempty"`
	WaitForBootSync  *bool  `json:"waitForBootSync,omitempty"`
	EmbedInterval    string `json:"embedInterval,omitempty"`
	CommandTimeoutMs *int64 `json:"commandTimeoutMs,omitempty"`
	UpdateTimeoutMs  *int64 `json:"updateTimeoutMs,omitempty"`
	EmbedTimeoutMs   *int64 `json:"embedTimeoutMs,omitempty"`
}

// QmdLimitsInput is raw limits config.
type QmdLimitsInput struct {
	MaxResults       *int   `json:"maxResults,omitempty"`
	MaxSnippetChars  *int   `json:"maxSnippetChars,omitempty"`
	MaxInjectedChars *int   `json:"maxInjectedChars,omitempty"`
	TimeoutMs        *int64 `json:"timeoutMs,omitempty"`
}

// ResolveMemoryBackendConfig resolves raw memory config into final form.
func ResolveMemoryBackendConfig(input *MemoryConfigInput, workspaceDir string) *ResolvedMemoryBackendConfig {
	backend := defaultBackend
	citations := defaultCitations
	if input != nil {
		if input.Backend == string(BackendQMD) {
			backend = BackendQMD
		}
		switch MemoryCitationsMode(input.Citations) {
		case CitationsOn:
			citations = CitationsOn
		case CitationsOff:
			citations = CitationsOff
		}
	}

	if backend != BackendQMD {
		return &ResolvedMemoryBackendConfig{Backend: BackendBuiltin, Citations: citations}
	}

	qmdCfg := input.QMD
	includeDefault := true
	if qmdCfg != nil && qmdCfg.IncludeDefaultMemory != nil {
		includeDefault = *qmdCfg.IncludeDefaultMemory
	}

	nameSet := make(map[string]struct{})
	var collections []ResolvedQmdCollection
	if includeDefault {
		collections = append(collections, resolveDefaultCollections(workspaceDir, nameSet)...)
	}
	if qmdCfg != nil {
		collections = append(collections, resolveCustomPaths(qmdCfg.Paths, workspaceDir, nameSet)...)
	}

	command := "qmd"
	if qmdCfg != nil && strings.TrimSpace(qmdCfg.Command) != "" {
		parts := strings.Fields(strings.TrimSpace(qmdCfg.Command))
		if len(parts) > 0 {
			command = parts[0]
		}
	}

	resolved := &ResolvedQmdConfig{
		Command:              command,
		Collections:          collections,
		IncludeDefaultMemory: includeDefault,
		Sessions:             resolveSessionConfig(qmdCfg, workspaceDir),
		Update:               resolveUpdateConfig(qmdCfg),
		Limits:               resolveLimitsConfig(qmdCfg),
	}

	return &ResolvedMemoryBackendConfig{
		Backend:   BackendQMD,
		Citations: citations,
		QMD:       resolved,
	}
}

func sanitizeName(input string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(input) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('-')
		}
	}
	result := strings.Trim(sb.String(), "-")
	if result == "" {
		return "collection"
	}
	return result
}

func ensureUniqueName(base string, existing map[string]struct{}) string {
	name := sanitizeName(base)
	if _, ok := existing[name]; !ok {
		existing[name] = struct{}{}
		return name
	}
	suffix := 2
	for {
		candidate := name + "-" + strings.Repeat("", 0) + itoa(suffix)
		if _, ok := existing[candidate]; !ok {
			existing[candidate] = struct{}{}
			return candidate
		}
		suffix++
	}
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func resolveDefaultCollections(workspaceDir string, existing map[string]struct{}) []ResolvedQmdCollection {
	entries := []struct {
		path, pattern, base string
	}{
		{workspaceDir, "MEMORY.md", "memory-root"},
		{workspaceDir, "memory.md", "memory-alt"},
		{filepath.Join(workspaceDir, "memory"), "**/*.md", "memory-dir"},
	}
	var result []ResolvedQmdCollection
	for _, e := range entries {
		result = append(result, ResolvedQmdCollection{
			Name:    ensureUniqueName(e.base, existing),
			Path:    e.path,
			Pattern: e.pattern,
			Kind:    "memory",
		})
	}
	return result
}

func resolveCustomPaths(paths []QmdIndexPathInput, workspaceDir string, existing map[string]struct{}) []ResolvedQmdCollection {
	var result []ResolvedQmdCollection
	for i, entry := range paths {
		trimmed := strings.TrimSpace(entry.Path)
		if trimmed == "" {
			continue
		}
		var resolved string
		if filepath.IsAbs(trimmed) {
			resolved = filepath.Clean(trimmed)
		} else {
			resolved = filepath.Clean(filepath.Join(workspaceDir, trimmed))
		}
		pattern := strings.TrimSpace(entry.Pattern)
		if pattern == "" {
			pattern = "**/*.md"
		}
		baseName := strings.TrimSpace(entry.Name)
		if baseName == "" {
			baseName = "custom-" + itoa(i+1)
		}
		result = append(result, ResolvedQmdCollection{
			Name:    ensureUniqueName(baseName, existing),
			Path:    resolved,
			Pattern: pattern,
			Kind:    "custom",
		})
	}
	return result
}

func resolveSessionConfig(qmd *QmdConfigInput, workspaceDir string) ResolvedQmdSessionConfig {
	if qmd == nil || qmd.Sessions == nil {
		return ResolvedQmdSessionConfig{}
	}
	s := qmd.Sessions
	cfg := ResolvedQmdSessionConfig{}
	if s.Enabled != nil {
		cfg.Enabled = *s.Enabled
	}
	if ed := strings.TrimSpace(s.ExportDir); ed != "" {
		if filepath.IsAbs(ed) {
			cfg.ExportDir = filepath.Clean(ed)
		} else {
			cfg.ExportDir = filepath.Clean(filepath.Join(workspaceDir, ed))
		}
	}
	if s.RetentionDays != nil && *s.RetentionDays > 0 {
		v := *s.RetentionDays
		cfg.RetentionDays = &v
	}
	return cfg
}

func resolveUpdateConfig(qmd *QmdConfigInput) ResolvedQmdUpdateConfig {
	cfg := ResolvedQmdUpdateConfig{
		IntervalMs:       defaultQmdIntervalMs,
		DebounceMs:       defaultQmdDebounceMs,
		OnBoot:           true,
		EmbedIntervalMs:  defaultQmdEmbedIntervalMs,
		CommandTimeoutMs: defaultQmdCommandTimeoutMs,
		UpdateTimeoutMs:  defaultQmdUpdateTimeoutMs,
		EmbedTimeoutMs:   defaultQmdEmbedTimeoutMs,
	}
	if qmd == nil || qmd.Update == nil {
		return cfg
	}
	u := qmd.Update
	if u.DebounceMs != nil && *u.DebounceMs >= 0 {
		cfg.DebounceMs = *u.DebounceMs
	}
	if u.OnBoot != nil {
		cfg.OnBoot = *u.OnBoot
	}
	if u.WaitForBootSync != nil {
		cfg.WaitForBootSync = *u.WaitForBootSync
	}
	if u.CommandTimeoutMs != nil && *u.CommandTimeoutMs > 0 {
		cfg.CommandTimeoutMs = *u.CommandTimeoutMs
	}
	if u.UpdateTimeoutMs != nil && *u.UpdateTimeoutMs > 0 {
		cfg.UpdateTimeoutMs = *u.UpdateTimeoutMs
	}
	if u.EmbedTimeoutMs != nil && *u.EmbedTimeoutMs > 0 {
		cfg.EmbedTimeoutMs = *u.EmbedTimeoutMs
	}
	return cfg
}

func resolveLimitsConfig(qmd *QmdConfigInput) ResolvedQmdLimitsConfig {
	cfg := defaultQmdLimits
	if qmd == nil || qmd.Limits == nil {
		return cfg
	}
	l := qmd.Limits
	if l.MaxResults != nil && *l.MaxResults > 0 {
		cfg.MaxResults = *l.MaxResults
	}
	if l.MaxSnippetChars != nil && *l.MaxSnippetChars > 0 {
		cfg.MaxSnippetChars = *l.MaxSnippetChars
	}
	if l.MaxInjectedChars != nil && *l.MaxInjectedChars > 0 {
		cfg.MaxInjectedChars = *l.MaxInjectedChars
	}
	if l.TimeoutMs != nil && *l.TimeoutMs > 0 {
		cfg.TimeoutMs = *l.TimeoutMs
	}
	return cfg
}
