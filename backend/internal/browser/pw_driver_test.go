package browser

import (
	"testing"
)

func TestDriverKindConstants(t *testing.T) {
	if DriverCDP != "cdp" {
		t.Errorf("DriverCDP = %q, want %q", DriverCDP, "cdp")
	}
	if DriverPlaywright != "playwright" {
		t.Errorf("DriverPlaywright = %q, want %q", DriverPlaywright, "playwright")
	}
}

func TestNewDriverManager_DefaultsCDP(t *testing.T) {
	dm := NewDriverManager(DriverManagerConfig{})
	if dm.Kind() != DriverCDP {
		t.Errorf("default driver = %q, want %q", dm.Kind(), DriverCDP)
	}
}

func TestNewDriverManager_CDPTools(t *testing.T) {
	dm := NewDriverManager(DriverManagerConfig{
		Kind:   DriverCDP,
		CDPURL: "ws://localhost:9222",
	})

	tools, err := dm.Tools()
	if err != nil {
		t.Fatalf("Tools() error: %v", err)
	}
	if tools == nil {
		t.Fatal("Tools() returned nil")
	}

	// Should be CDPPlaywrightTools
	_, ok := tools.(*CDPPlaywrightTools)
	if !ok {
		t.Error("expected *CDPPlaywrightTools for CDP driver")
	}
}

func TestNewDriverManager_CachesTools(t *testing.T) {
	dm := NewDriverManager(DriverManagerConfig{
		Kind:   DriverCDP,
		CDPURL: "ws://localhost:9222",
	})

	tools1, _ := dm.Tools()
	tools2, _ := dm.Tools()
	if tools1 != tools2 {
		t.Error("expected cached tools instance to be reused")
	}
}

func TestNewDriverManager_Close(t *testing.T) {
	dm := NewDriverManager(DriverManagerConfig{
		Kind:   DriverCDP,
		CDPURL: "ws://localhost:9222",
	})

	// Get tools to populate cache
	_, _ = dm.Tools()

	if err := dm.Close(); err != nil {
		t.Errorf("Close() error: %v", err)
	}
}

func TestIsPlaywrightAvailable(t *testing.T) {
	// This just checks the function doesn't panic.
	// Result depends on environment.
	_ = IsPlaywrightAvailable()
}
