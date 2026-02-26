package memory

// Tone represents a status severity level for display.
type Tone string

const (
	ToneOK    Tone = "ok"
	ToneWarn  Tone = "warn"
	ToneMuted Tone = "muted"
)

// VectorState describes vector search readiness.
type VectorState struct {
	Tone  Tone   `json:"tone"`
	State string `json:"state"` // "ready" | "unavailable" | "disabled" | "unknown"
}

// ResolveMemoryVectorState determines the display state of vector search.
func ResolveMemoryVectorState(enabled bool, available *bool) VectorState {
	if !enabled {
		return VectorState{Tone: ToneMuted, State: "disabled"}
	}
	if available != nil && *available {
		return VectorState{Tone: ToneOK, State: "ready"}
	}
	if available != nil && !*available {
		return VectorState{Tone: ToneWarn, State: "unavailable"}
	}
	return VectorState{Tone: ToneMuted, State: "unknown"}
}

// FTSState describes FTS search readiness.
type FTSState struct {
	Tone  Tone   `json:"tone"`
	State string `json:"state"` // "ready" | "unavailable" | "disabled"
}

// ResolveMemoryFtsState determines the display state of FTS.
func ResolveMemoryFtsState(enabled, available bool) FTSState {
	if !enabled {
		return FTSState{Tone: ToneMuted, State: "disabled"}
	}
	if available {
		return FTSState{Tone: ToneOK, State: "ready"}
	}
	return FTSState{Tone: ToneWarn, State: "unavailable"}
}

// CacheSummary describes cache state for display.
type CacheSummary struct {
	Tone Tone   `json:"tone"`
	Text string `json:"text"`
}

// ResolveMemoryCacheSummary formats the cache state for display.
func ResolveMemoryCacheSummary(enabled bool, entries *int) CacheSummary {
	if !enabled {
		return CacheSummary{Tone: ToneMuted, Text: "cache off"}
	}
	text := "cache on"
	if entries != nil {
		text += " (" + itoa(*entries) + ")"
	}
	return CacheSummary{Tone: ToneOK, Text: text}
}

// CacheState describes whether caching is on/off for display.
type CacheState struct {
	Tone  Tone   `json:"tone"`
	State string `json:"state"` // "enabled" | "disabled"
}

// ResolveMemoryCacheState returns caching on/off state.
func ResolveMemoryCacheState(enabled bool) CacheState {
	if enabled {
		return CacheState{Tone: ToneOK, State: "enabled"}
	}
	return CacheState{Tone: ToneMuted, State: "disabled"}
}
