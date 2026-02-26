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
