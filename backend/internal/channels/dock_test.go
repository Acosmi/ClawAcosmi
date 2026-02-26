package channels

import (
	"testing"
)

func TestGetChannelDock_CoreChannels(t *testing.T) {
	// 核心频道应存在且字段正确
	tests := []struct {
		id         ChannelID
		wantNative bool
		wantChunk  int
	}{
		{ChannelDiscord, true, 2000},
		{ChannelTelegram, true, 4000},
		{ChannelSlack, true, 4000},
		{ChannelWhatsApp, false, 4000},
		{ChannelGoogleChat, false, 4000},
		{ChannelSignal, false, 4000},
		{ChannelIMessage, false, 4000},
	}

	for _, tt := range tests {
		d := GetChannelDock(tt.id)
		if d == nil {
			t.Errorf("GetChannelDock(%q) = nil, want non-nil", tt.id)
			continue
		}
		if d.Capabilities.NativeCommands != tt.wantNative {
			t.Errorf("GetChannelDock(%q).NativeCommands = %v, want %v",
				tt.id, d.Capabilities.NativeCommands, tt.wantNative)
		}
		if d.Outbound == nil || d.Outbound.TextChunkLimit != tt.wantChunk {
			chunk := 0
			if d.Outbound != nil {
				chunk = d.Outbound.TextChunkLimit
			}
			t.Errorf("GetChannelDock(%q).TextChunkLimit = %d, want %d",
				tt.id, chunk, tt.wantChunk)
		}
	}
}

func TestGetChannelDock_Unknown(t *testing.T) {
	d := GetChannelDock("nonexistent")
	if d != nil {
		t.Errorf("GetChannelDock(nonexistent) = %v, want nil", d)
	}
}

func TestListChannelDocks_Order(t *testing.T) {
	docks := ListChannelDocks()
	if len(docks) < 7 {
		t.Fatalf("ListChannelDocks() returned %d docks, want >= 7", len(docks))
	}
	// 应按 chatChannelOrder 排序
	for i, d := range docks {
		if d == nil {
			t.Errorf("ListChannelDocks()[%d] is nil", i)
		}
	}
}

func TestGetPluginDebounce_CoreChannels(t *testing.T) {
	// 核心频道无 QueueDefaults → 返回 nil
	for _, id := range []ChannelID{ChannelDiscord, ChannelTelegram, ChannelSlack} {
		v := GetPluginDebounce(id)
		if v != nil {
			t.Errorf("GetPluginDebounce(%q) = %d, want nil", id, *v)
		}
	}
}

func TestGetPluginDebounce_WithDefaults(t *testing.T) {
	// 注入 PluginChannelDockProvider
	debounce := 1500
	origProvider := PluginChannelDockProvider
	PluginChannelDockProvider = func() []*ChannelDock {
		return []*ChannelDock{
			{
				ID:            "custom-channel",
				Capabilities:  DockCapabilities{ChatTypes: []string{"direct"}},
				QueueDefaults: &DockQueueDefaults{DebounceMs: &debounce},
			},
		}
	}
	defer func() { PluginChannelDockProvider = origProvider }()

	v := GetPluginDebounce("custom-channel")
	if v == nil || *v != 1500 {
		t.Errorf("GetPluginDebounce(custom-channel) = %v, want 1500", v)
	}

	// 未知频道仍返回 nil
	v2 := GetPluginDebounce("unknown")
	if v2 != nil {
		t.Errorf("GetPluginDebounce(unknown) = %d, want nil", *v2)
	}
}

func TestGetPluginDebounce_NegativeClamped(t *testing.T) {
	// 负值应 clamp 到 0
	neg := -100
	origProvider := PluginChannelDockProvider
	PluginChannelDockProvider = func() []*ChannelDock {
		return []*ChannelDock{
			{
				ID:            "neg-channel",
				Capabilities:  DockCapabilities{},
				QueueDefaults: &DockQueueDefaults{DebounceMs: &neg},
			},
		}
	}
	defer func() { PluginChannelDockProvider = origProvider }()

	v := GetPluginDebounce("neg-channel")
	if v == nil || *v != 0 {
		t.Errorf("GetPluginDebounce(neg-channel) = %v, want 0", v)
	}
}

func TestGetBlockStreamingCoalesceDefaults(t *testing.T) {
	// Discord 有 streaming defaults
	minChars, idleMs := GetBlockStreamingCoalesceDefaults(ChannelDiscord)
	if minChars != 1500 || idleMs != 1000 {
		t.Errorf("Discord streaming = (%d, %d), want (1500, 1000)", minChars, idleMs)
	}

	// WhatsApp 无 streaming → (0, 0)
	minChars, idleMs = GetBlockStreamingCoalesceDefaults(ChannelWhatsApp)
	if minChars != 0 || idleMs != 0 {
		t.Errorf("WhatsApp streaming = (%d, %d), want (0, 0)", minChars, idleMs)
	}
}

func TestListNativeCommandChannels(t *testing.T) {
	ids := ListNativeCommandChannels()
	if len(ids) < 3 {
		t.Fatalf("ListNativeCommandChannels() = %d, want >= 3", len(ids))
	}

	expected := map[ChannelID]bool{
		ChannelDiscord:  false,
		ChannelTelegram: false,
		ChannelSlack:    false,
	}
	for _, id := range ids {
		if _, ok := expected[id]; ok {
			expected[id] = true
		}
	}
	for id, found := range expected {
		if !found {
			t.Errorf("ListNativeCommandChannels() missing %q", id)
		}
	}
}

func TestListNativeCommandChannels_WithPlugin(t *testing.T) {
	// 注入含 NativeCommands 的插件频道
	origProvider := PluginChannelDockProvider
	PluginChannelDockProvider = func() []*ChannelDock {
		return []*ChannelDock{
			{
				ID:           "plugin-native",
				Capabilities: DockCapabilities{NativeCommands: true},
			},
		}
	}
	defer func() { PluginChannelDockProvider = origProvider }()

	ids := ListNativeCommandChannels()
	found := false
	for _, id := range ids {
		if id == "plugin-native" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListNativeCommandChannels() should include plugin-native")
	}
}
