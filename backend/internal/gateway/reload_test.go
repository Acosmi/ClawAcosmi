package gateway

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDiffConfigPaths_Equal(t *testing.T) {
	a := map[string]interface{}{"key": "val"}
	b := map[string]interface{}{"key": "val"}
	if paths := DiffConfigPaths(a, b, ""); len(paths) != 0 {
		t.Errorf("equal configs should produce no diff, got %v", paths)
	}
}

func TestDiffConfigPaths_Changed(t *testing.T) {
	a := map[string]interface{}{"gateway": map[string]interface{}{"port": float64(3000)}}
	b := map[string]interface{}{"gateway": map[string]interface{}{"port": float64(3001)}}
	paths := DiffConfigPaths(a, b, "")
	if len(paths) != 1 || paths[0] != "gateway.port" {
		t.Errorf("expected [gateway.port], got %v", paths)
	}
}

func TestDiffConfigPaths_Multiple(t *testing.T) {
	a := map[string]interface{}{"hooks": map[string]interface{}{"enabled": true}, "models": "m1"}
	b := map[string]interface{}{"hooks": map[string]interface{}{"enabled": false}, "models": "m2"}
	paths := DiffConfigPaths(a, b, "")
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
	}
}

func TestBuildReloadPlan_HotReload(t *testing.T) {
	plan := BuildReloadPlan([]string{"hooks.token"}, nil)
	if plan.RestartGateway {
		t.Error("hooks change should not restart gateway")
	}
	if !plan.ReloadHooks {
		t.Error("hooks change should reload hooks")
	}
}

func TestBuildReloadPlan_Restart(t *testing.T) {
	plan := BuildReloadPlan([]string{"gateway.port"}, nil)
	if !plan.RestartGateway {
		t.Error("gateway change should restart")
	}
}

func TestBuildReloadPlan_Noop(t *testing.T) {
	plan := BuildReloadPlan([]string{"models.openai"}, nil)
	if plan.RestartGateway {
		t.Error("models change should not restart")
	}
	if len(plan.NoopPaths) != 1 {
		t.Errorf("expected 1 noop, got %d", len(plan.NoopPaths))
	}
}

func TestBuildReloadPlan_GmailWatcher(t *testing.T) {
	plan := BuildReloadPlan([]string{"hooks.gmail.enabled"}, nil)
	if !plan.RestartGmailWatcher {
		t.Error("gmail hook should trigger restart-gmail-watcher")
	}
	if !plan.ReloadHooks {
		t.Error("gmail restart should also reload hooks")
	}
}

func TestBuildReloadPlan_Cron(t *testing.T) {
	plan := BuildReloadPlan([]string{"cron.schedule"}, nil)
	if !plan.RestartCron {
		t.Error("cron change should trigger restart-cron")
	}
}

func TestBuildReloadPlan_Unknown(t *testing.T) {
	plan := BuildReloadPlan([]string{"completely.unknown.path"}, nil)
	if !plan.RestartGateway {
		t.Error("unknown path should trigger restart")
	}
}

// TestBuildReloadPlan_Channels 验证 channels.* 变更通过动态注册的规则触发 RestartChannels，
// 而不是 RestartGateway（B3 修复回归测试）。
func TestBuildReloadPlan_Channels(t *testing.T) {
	// channels.feishu.* 没有动态规则时，回退为 RestartGateway（旧行为，B3 bug 所在）
	planOld := BuildReloadPlan([]string{"channels.feishu.appId"}, nil)
	if !planOld.RestartGateway {
		t.Error("without channel rules, channels change should trigger RestartGateway (old fallback)")
	}

	// 注册动态规则后，触发 RestartChannels 而非 RestartGateway（B3 修复目标状态）
	channelRulesMu.Lock()
	channelRules = nil
	channelRulesMu.Unlock()
	RegisterChannelReloadRules("feishu", "restart-channel:feishu")

	planNew := BuildReloadPlan([]string{"channels.feishu.appId"}, getChannelRules())
	if planNew.RestartGateway {
		t.Error("with channel rule registered, channels change should NOT trigger RestartGateway")
	}
	if _, ok := planNew.RestartChannels["feishu"]; !ok {
		t.Error("channels.feishu change should populate RestartChannels[feishu]")
	}

	// 清理全局状态
	channelRulesMu.Lock()
	channelRules = nil
	channelRulesMu.Unlock()
}

func TestConfigWatcher_Debounce(t *testing.T) {
	var count atomic.Int32
	w := NewConfigWatcher(50, func() {
		count.Add(1)
	})
	defer w.Stop()

	// 快速通知多次，应只触发一次
	for i := 0; i < 5; i++ {
		w.Notify()
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if c := count.Load(); c != 1 {
		t.Errorf("expected 1 callback, got %d", c)
	}
}

func TestConfigWatcher_Stop(t *testing.T) {
	var count atomic.Int32
	w := NewConfigWatcher(50, func() {
		count.Add(1)
	})
	w.Notify()
	w.Stop()
	time.Sleep(100 * time.Millisecond)
	if c := count.Load(); c != 0 {
		t.Errorf("stopped watcher should not fire, got %d", c)
	}
}

func TestConfigSnapshot(t *testing.T) {
	type Config struct {
		Port int    `json:"port"`
		Name string `json:"name"`
	}
	snap := ConfigSnapshot(Config{Port: 3000, Name: "test"})
	if snap["port"] != float64(3000) || snap["name"] != "test" {
		t.Errorf("snapshot = %v", snap)
	}
}

func TestResolveReloadSettings_Nil(t *testing.T) {
	s := ResolveReloadSettings(nil)
	if s.Mode != ReloadModeHybrid || s.DebounceMs != 300 {
		t.Errorf("default settings = %+v", s)
	}
}

func TestResolveReloadSettings_Custom(t *testing.T) {
	ms := 500
	raw := &ReloadSettingsRaw{Mode: "hot", DebounceMs: &ms}
	s := ResolveReloadSettings(raw)
	if s.Mode != ReloadModeHot {
		t.Errorf("mode = %q", s.Mode)
	}
	if s.DebounceMs != 500 {
		t.Errorf("debounceMs = %d", s.DebounceMs)
	}
}

func TestResolveReloadSettings_Clamp(t *testing.T) {
	ms := 10
	raw := &ReloadSettingsRaw{DebounceMs: &ms}
	s := ResolveReloadSettings(raw)
	if s.DebounceMs != 50 {
		t.Errorf("debounceMs should be clamped to 50, got %d", s.DebounceMs)
	}
}

func TestRegisterChannelReloadRules(t *testing.T) {
	// 清理全局状态
	channelRulesMu.Lock()
	channelRules = nil
	channelRulesMu.Unlock()

	RegisterChannelReloadRules("slack", "restart-channel:slack")
	rules := getChannelRules()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].prefix != "channels.slack" {
		t.Errorf("prefix = %q", rules[0].prefix)
	}
}

func TestStartConfigReloader_Off(t *testing.T) {
	r := StartConfigReloader(
		ReloadSettings{Mode: ReloadModeOff},
		nil,
		ConfigReloaderCallbacks{},
	)
	if r.watcher != nil {
		t.Error("mode=off should not create watcher")
	}
	r.Stop()
}

func TestStartConfigReloader_HotReload(t *testing.T) {
	var hotCalled atomic.Int32
	initial := map[string]interface{}{"hooks": map[string]interface{}{"token": "old"}}
	r := StartConfigReloader(
		ReloadSettings{Mode: ReloadModeHybrid, DebounceMs: 50},
		initial,
		ConfigReloaderCallbacks{
			LoadConfig: func() (map[string]interface{}, error) {
				return map[string]interface{}{"hooks": map[string]interface{}{"token": "new"}}, nil
			},
			OnHotReload: func(plan *ReloadPlan) {
				hotCalled.Add(1)
			},
		},
	)
	defer r.Stop()
	r.Notify()
	time.Sleep(200 * time.Millisecond)
	if hotCalled.Load() != 1 {
		t.Errorf("expected 1 hot reload, got %d", hotCalled.Load())
	}
}
