package channels

import (
	"fmt"
	"testing"
)

type mockPlugin struct {
	id      ChannelID
	started map[string]bool
	stopped map[string]bool
}

func newMockPlugin(id ChannelID) *mockPlugin {
	return &mockPlugin{id: id, started: map[string]bool{}, stopped: map[string]bool{}}
}

func (p *mockPlugin) ID() ChannelID { return p.id }
func (p *mockPlugin) Start(accountID string) error {
	p.started[accountID] = true
	return nil
}
func (p *mockPlugin) Stop(accountID string) error {
	p.stopped[accountID] = true
	return nil
}

func TestManager_StartStop(t *testing.T) {
	m := NewManager()
	plugin := newMockPlugin(ChannelWebchat)
	m.RegisterPlugin(plugin)

	if err := m.StartChannel(ChannelWebchat, ""); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !plugin.started[DefaultAccountID] {
		t.Error("plugin should be started")
	}
	// 重复启动应无操作
	if err := m.StartChannel(ChannelWebchat, ""); err != nil {
		t.Fatalf("re-start: %v", err)
	}

	if err := m.StopChannel(ChannelWebchat, ""); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !plugin.stopped[DefaultAccountID] {
		t.Error("plugin should be stopped")
	}
}

func TestManager_UnknownChannel(t *testing.T) {
	m := NewManager()
	err := m.StartChannel("nonexistent", "")
	if err == nil {
		t.Error("unknown channel should error")
	}
}

func TestManager_HasPlugin(t *testing.T) {
	m := NewManager()

	// 未注册时应返回 false
	if m.HasPlugin(ChannelFeishu) {
		t.Error("HasPlugin should be false before registration")
	}

	// 注册后应返回 true
	m.RegisterPlugin(newMockPlugin(ChannelFeishu))
	if !m.HasPlugin(ChannelFeishu) {
		t.Error("HasPlugin should be true after registration")
	}

	// 其他频道不受影响
	if m.HasPlugin(ChannelDingTalk) {
		t.Error("HasPlugin should be false for unregistered channel")
	}
}

func TestManager_Snapshot(t *testing.T) {
	m := NewManager()
	m.RegisterPlugin(newMockPlugin(ChannelDiscord))
	m.StartChannel(ChannelDiscord, "acc1")

	snap := m.GetSnapshot()
	if snap.Accounts[ChannelDiscord] == nil || snap.Accounts[ChannelDiscord]["acc1"] == nil {
		t.Error("snapshot should contain discord:acc1")
	}
	if snap.Accounts[ChannelDiscord]["acc1"].Status != "running" {
		t.Errorf("status = %q", snap.Accounts[ChannelDiscord]["acc1"].Status)
	}
}

func TestManager_MarkLoggedOut(t *testing.T) {
	m := NewManager()
	m.RegisterPlugin(newMockPlugin(ChannelSlack))
	m.StartChannel(ChannelSlack, "")
	m.MarkLoggedOut(ChannelSlack, true, "")

	snap := m.GetSnapshot()
	if snap.Channels[ChannelSlack].LoggedIn {
		t.Error("should be logged out")
	}
	if snap.Channels[ChannelSlack].Status != "stopped" {
		t.Errorf("status = %q, want stopped", snap.Channels[ChannelSlack].Status)
	}
}

type errorPlugin struct {
	id ChannelID
}

func (p *errorPlugin) ID() ChannelID      { return p.id }
func (p *errorPlugin) Start(string) error { return fmt.Errorf("start failed") }
func (p *errorPlugin) Stop(string) error  { return nil }

func TestManager_StartError(t *testing.T) {
	m := NewManager()
	m.RegisterPlugin(&errorPlugin{id: ChannelTelegram})
	err := m.StartChannel(ChannelTelegram, "")
	if err == nil {
		t.Error("should return error")
	}
	snap := m.GetSnapshot()
	if snap.Channels[ChannelTelegram].Status != "error" {
		t.Errorf("status = %q, want error", snap.Channels[ChannelTelegram].Status)
	}
}

// configUpdaterPlugin 用于测试 ConfigUpdater 接口的 mock 插件。
type configUpdaterPlugin struct {
	id      ChannelID
	lastCfg interface{}
	started int
	stopped int
}

func (p *configUpdaterPlugin) ID() ChannelID                { return p.id }
func (p *configUpdaterPlugin) Start(string) error           { p.started++; return nil }
func (p *configUpdaterPlugin) Stop(string) error            { p.stopped++; return nil }
func (p *configUpdaterPlugin) UpdateConfig(cfg interface{}) { p.lastCfg = cfg }

func TestManager_ReloadChannel_WithConfigUpdater(t *testing.T) {
	m := NewManager()
	plugin := &configUpdaterPlugin{id: ChannelFeishu}
	m.RegisterPlugin(plugin)
	m.StartChannel(ChannelFeishu, "")

	sentinel := struct{ V string }{V: "new-creds"}
	err := m.ReloadChannel(ChannelFeishu, sentinel, "")
	if err != nil {
		t.Fatalf("ReloadChannel: %v", err)
	}
	if plugin.lastCfg != sentinel {
		t.Error("UpdateConfig was not called with the new config")
	}
	// Stop+Start 应各执行一次（Start 初始 1 次 + Reload 内 1 次 = 2）
	if plugin.started != 2 {
		t.Errorf("Start called %d times, want 2", plugin.started)
	}
	if plugin.stopped != 1 {
		t.Errorf("Stop called %d times, want 1", plugin.stopped)
	}
}

func TestManager_ReloadChannel_NoConfigUpdater(t *testing.T) {
	// 不实现 ConfigUpdater 的插件，ReloadChannel 仍应完成 Stop+Start
	m := NewManager()
	plugin := newMockPlugin(ChannelDingTalk)
	m.RegisterPlugin(plugin)
	m.StartChannel(ChannelDingTalk, "")

	err := m.ReloadChannel(ChannelDingTalk, nil, "")
	if err != nil {
		t.Fatalf("ReloadChannel (no updater): %v", err)
	}
	if !plugin.started["default"] {
		t.Error("plugin should be started after reload")
	}
}

func TestManager_ReloadChannel_NotRegistered(t *testing.T) {
	m := NewManager()
	err := m.ReloadChannel(ChannelWeCom, nil, "")
	if err == nil {
		t.Error("ReloadChannel on unregistered channel should return error")
	}
}

func TestIsAccountEnabled(t *testing.T) {
	if !IsAccountEnabled(nil) {
		t.Error("nil should be enabled")
	}
	f := false
	if IsAccountEnabled(&f) {
		t.Error("false should be disabled")
	}
	tr := true
	if !IsAccountEnabled(&tr) {
		t.Error("true should be enabled")
	}
}
