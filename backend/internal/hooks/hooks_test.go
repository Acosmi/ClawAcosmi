package hooks

import (
	"net/url"
	"testing"
)

func TestResolveHooksConfig_Disabled(t *testing.T) {
	cfg := &HooksConfig{Enabled: false}
	result, err := ResolveHooksConfig(cfg)
	if err != nil || result != nil {
		t.Errorf("disabled hooks should return nil, got %v, err=%v", result, err)
	}
}

func TestResolveHooksConfig_MissingToken(t *testing.T) {
	cfg := &HooksConfig{Enabled: true}
	_, err := ResolveHooksConfig(cfg)
	if err == nil {
		t.Error("expected error for missing token")
	}
}

func TestResolveHooksConfig_Basic(t *testing.T) {
	cfg := &HooksConfig{Enabled: true, Token: "secret"}
	result, err := ResolveHooksConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BasePath != "/hooks" {
		t.Errorf("basePath = %q, want /hooks", result.BasePath)
	}
	if result.Token != "secret" {
		t.Errorf("token = %q, want secret", result.Token)
	}
	if result.MaxBodyBytes != DefaultHooksMaxBodyBytes {
		t.Errorf("maxBodyBytes = %d", result.MaxBodyBytes)
	}
}

func TestResolveHooksConfig_CustomPath(t *testing.T) {
	cfg := &HooksConfig{Enabled: true, Token: "tk", Path: "webhooks/"}
	result, err := ResolveHooksConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BasePath != "/webhooks" {
		t.Errorf("basePath = %q, want /webhooks", result.BasePath)
	}
}

func TestResolveHooksConfig_SlashPath(t *testing.T) {
	cfg := &HooksConfig{Enabled: true, Token: "tk", Path: "/"}
	_, err := ResolveHooksConfig(cfg)
	if err == nil {
		t.Error("expected error for path='/'")
	}
}

func TestResolveHooksConfig_GmailPreset(t *testing.T) {
	cfg := &HooksConfig{Enabled: true, Token: "tk", Presets: []string{"gmail"}}
	result, err := ResolveHooksConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Mappings) != 1 {
		t.Fatalf("expected 1 mapping, got %d", len(result.Mappings))
	}
	if result.Mappings[0].ID != "gmail" {
		t.Errorf("mapping ID = %q, want gmail", result.Mappings[0].ID)
	}
}

func TestRenderTemplate_Simple(t *testing.T) {
	ctx := &HookMappingContext{
		Payload: map[string]interface{}{"name": "Alice", "value": float64(42)},
		Path:    "test/path",
	}
	out := RenderTemplate("Hello {{name}}, value={{value}}", ctx)
	if out != "Hello Alice, value=42" {
		t.Errorf("got %q", out)
	}
}

func TestRenderTemplate_Nested(t *testing.T) {
	ctx := &HookMappingContext{
		Payload: map[string]interface{}{
			"messages": []interface{}{
				map[string]interface{}{"from": "bob@test.com", "subject": "Hi"},
			},
		},
	}
	out := RenderTemplate("From: {{messages[0].from}}, Subject: {{messages[0].subject}}", ctx)
	if out != "From: bob@test.com, Subject: Hi" {
		t.Errorf("got %q", out)
	}
}

func TestRenderTemplate_PathAndNow(t *testing.T) {
	ctx := &HookMappingContext{Path: "gmail"}
	out := RenderTemplate("path={{path}}", ctx)
	if out != "path=gmail" {
		t.Errorf("got %q", out)
	}
	out = RenderTemplate("time={{now}}", ctx)
	if out == "time=" || out == "time={{now}}" {
		t.Errorf("now should render a timestamp, got %q", out)
	}
}

func TestRenderTemplate_Headers(t *testing.T) {
	ctx := &HookMappingContext{
		Headers: map[string]string{"x-source": "webhook"},
	}
	out := RenderTemplate("source={{headers.x-source}}", ctx)
	if out != "source=webhook" {
		t.Errorf("got %q", out)
	}
}

func TestRenderTemplate_Query(t *testing.T) {
	u, _ := url.Parse("http://example.com?key=val123")
	ctx := &HookMappingContext{URL: u}
	out := RenderTemplate("key={{query.key}}", ctx)
	if out != "key=val123" {
		t.Errorf("got %q", out)
	}
}

func TestGetByPath(t *testing.T) {
	data := map[string]interface{}{
		"a": map[string]interface{}{
			"b": []interface{}{
				map[string]interface{}{"c": "deep"},
			},
		},
	}
	val := getByPath(data, "a.b[0].c")
	if val != "deep" {
		t.Errorf("getByPath a.b[0].c = %v, want deep", val)
	}
	val = getByPath(data, "a.b[99].c")
	if val != nil {
		t.Errorf("out of bounds should return nil, got %v", val)
	}
	val = getByPath(nil, "a.b")
	if val != nil {
		t.Errorf("nil data should return nil, got %v", val)
	}
}

func TestApplyHookMappings_Match(t *testing.T) {
	mappings := []HookMappingResolved{
		{ID: "test", MatchPath: "gmail", Action: "agent", WakeMode: "now", MessageTemplate: "msg: {{text}}"},
	}
	ctx := &HookMappingContext{
		Path:    "gmail",
		Payload: map[string]interface{}{"text": "hello"},
	}
	result := ApplyHookMappings(mappings, ctx)
	if result == nil {
		t.Fatal("expected a match")
	}
	if !result.OK {
		t.Fatalf("expected OK, error=%s", result.Error)
	}
	if result.Action.Message != "msg: hello" {
		t.Errorf("message = %q", result.Action.Message)
	}
}

func TestApplyHookMappings_NoMatch(t *testing.T) {
	mappings := []HookMappingResolved{
		{ID: "test", MatchPath: "slack", Action: "agent", MessageTemplate: "x"},
	}
	ctx := &HookMappingContext{Path: "gmail"}
	result := ApplyHookMappings(mappings, ctx)
	if result != nil {
		t.Error("expected no match (nil)")
	}
}

func TestNormalizeWakePayload(t *testing.T) {
	text, mode, err := NormalizeWakePayload(map[string]interface{}{"text": "wake up"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "wake up" || mode != "now" {
		t.Errorf("text=%q mode=%q", text, mode)
	}
	text, mode, err = NormalizeWakePayload(map[string]interface{}{"text": "later", "mode": "next-heartbeat"})
	if err != nil || mode != "next-heartbeat" {
		t.Errorf("mode=%q err=%v", mode, err)
	}
	_, _, err = NormalizeWakePayload(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestNormalizeAgentPayload(t *testing.T) {
	p, err := NormalizeAgentPayload(map[string]interface{}{"message": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Message != "hello" {
		t.Errorf("message=%q", p.Message)
	}
	if p.Name != "Hook" {
		t.Errorf("name=%q, want Hook", p.Name)
	}
	if p.Channel != "last" {
		t.Errorf("channel=%q, want last", p.Channel)
	}
	// 缺失 message
	_, err = NormalizeAgentPayload(map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing message")
	}
}
