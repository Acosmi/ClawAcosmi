package gateway

import (
	"testing"
)

func TestGatewayState_Lifecycle(t *testing.T) {
	s := NewGatewayState()
	if s.Phase() != BootPhaseInit {
		t.Errorf("initial phase = %q", s.Phase())
	}
	s.SetPhase(BootPhaseStarting)
	if s.Phase() != BootPhaseStarting {
		t.Errorf("phase = %q after SetPhase", s.Phase())
	}
	s.SetPhase(BootPhaseReady)
	if s.Phase() != BootPhaseReady {
		t.Error("should be ready")
	}
}

func TestGatewayState_Subsystems(t *testing.T) {
	s := NewGatewayState()
	if s.Broadcaster() == nil {
		t.Error("Broadcaster nil")
	}
	if s.ChatState() == nil {
		t.Error("ChatState nil")
	}
	if s.ToolRegistry() == nil {
		t.Error("ToolRegistry nil")
	}
	if s.EventDispatcher() == nil {
		t.Error("EventDispatcher nil")
	}
}

func TestGetHealthStatus(t *testing.T) {
	s := NewGatewayState()

	h := GetHealthStatus(s, "1.0.0")
	if h.Status != "starting" {
		t.Errorf("init: status = %q", h.Status)
	}

	s.SetPhase(BootPhaseReady)
	h = GetHealthStatus(s, "1.0.0")
	if h.Status != "ok" {
		t.Errorf("ready: status = %q", h.Status)
	}

	s.SetPhase(BootPhaseStopping)
	h = GetHealthStatus(s, "1.0.0")
	if h.Status != "stopping" {
		t.Errorf("stopping: status = %q", h.Status)
	}
	if h.Version != "1.0.0" {
		t.Errorf("version = %q", h.Version)
	}
}

func TestValidateBootConfig(t *testing.T) {
	// 有效配置
	cfg := BootConfig{
		Server: ServerConfig{Port: 3777},
		Auth:   ResolvedGatewayAuth{Mode: AuthModeToken, Token: "tk"},
	}
	if err := ValidateBootConfig(cfg); err != nil {
		t.Errorf("valid config: %v", err)
	}
	// 无效端口
	cfg.Server.Port = 0
	if err := ValidateBootConfig(cfg); err == nil {
		t.Error("port=0 should fail")
	}
	// 无效 auth
	cfg.Server.Port = 3777
	cfg.Auth = ResolvedGatewayAuth{Mode: AuthModeToken}
	if err := ValidateBootConfig(cfg); err == nil {
		t.Error("token mode without token should fail")
	}
}
