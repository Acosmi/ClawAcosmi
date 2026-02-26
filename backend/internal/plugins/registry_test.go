package plugins

import (
	"testing"
)

func TestNewPluginRegistry(t *testing.T) {
	rt := &NullPluginRuntime{VersionStr: "1.0.0"}
	logger := PluginLogger{
		Info:  func(_ string) {},
		Warn:  func(_ string) {},
		Error: func(_ string) {},
	}
	r := NewPluginRegistry(rt, logger, nil)

	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if len(r.Plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(r.Plugins))
	}
	if len(r.Tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(r.Tools))
	}
}

func TestRegisterTool(t *testing.T) {
	r := newTestRegistry()
	rec := newTestRecord("test-plugin")

	factory := func(ctx PluginToolContext) interface{} { return nil }
	r.RegisterTool(rec, factory, &PluginToolOptions{Names: []string{"my-tool", "  another  "}})

	if len(r.Tools) != 1 {
		t.Fatalf("expected 1 tool registration, got %d", len(r.Tools))
	}
	if len(r.Tools[0].Names) != 2 {
		t.Errorf("expected 2 names, got %d", len(r.Tools[0].Names))
	}
	if r.Tools[0].Names[1] != "another" {
		t.Errorf("expected trimmed name 'another', got %q", r.Tools[0].Names[1])
	}
	if len(rec.ToolNames) != 2 {
		t.Errorf("expected 2 tool names on record, got %d", len(rec.ToolNames))
	}
}

func TestRegisterGatewayMethod_Duplicate(t *testing.T) {
	r := newTestRegistry()
	rec := newTestRecord("test-plugin")

	handler := func(params map[string]interface{}) (interface{}, error) { return nil, nil }
	r.RegisterGatewayMethod(rec, "foo.bar", handler)
	r.RegisterGatewayMethod(rec, "foo.bar", handler)

	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(r.Diagnostics))
	}
	if r.Diagnostics[0].Level != "error" {
		t.Errorf("expected error level, got %s", r.Diagnostics[0].Level)
	}
}

func TestRegisterChannel_EmptyID(t *testing.T) {
	r := newTestRegistry()
	rec := newTestRecord("test-plugin")

	r.RegisterChannel(rec, ChannelPlugin{ID: ""})

	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(r.Diagnostics))
	}
}

func TestRegisterProvider_Duplicate(t *testing.T) {
	r := newTestRegistry()
	rec := newTestRecord("test-plugin")

	p := ProviderPlugin{ID: "openai", Label: "OpenAI"}
	r.RegisterProvider(rec, p)
	r.RegisterProvider(rec, p)

	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic for dup provider, got %d", len(r.Diagnostics))
	}
	if len(r.Providers) != 1 {
		t.Errorf("expected 1 provider, got %d", len(r.Providers))
	}
}

func TestRegisterHook_MissingName(t *testing.T) {
	r := newTestRegistry()
	rec := newTestRecord("test-plugin")

	r.RegisterHook(rec, []string{"event.foo"}, "", rec.Source)

	if len(r.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic for missing name, got %d", len(r.Diagnostics))
	}
}

func TestCreateAPI(t *testing.T) {
	r := newTestRegistry()
	rec := newTestRecord("api-test")

	api := r.CreateAPI(rec, nil)
	if api.ID != "api-test" {
		t.Errorf("expected id 'api-test', got %q", api.ID)
	}
	if api.Runtime == nil {
		t.Error("expected non-nil runtime")
	}
}

// --- helpers ---

func newTestRegistry() *PluginRegistry {
	return NewPluginRegistry(
		&NullPluginRuntime{VersionStr: "1.0.0"},
		PluginLogger{
			Info:  func(_ string) {},
			Warn:  func(_ string) {},
			Error: func(_ string) {},
		},
		nil,
	)
}

func newTestRecord(id string) *PluginRecord {
	return &PluginRecord{
		ID:     id,
		Name:   id,
		Source: "/test/" + id,
		Status: "loaded",
	}
}
