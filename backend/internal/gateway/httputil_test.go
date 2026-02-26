package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	SendNotFound(w)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestWriteSSEData(t *testing.T) {
	w := httptest.NewRecorder()
	WriteSSEData(w, `{"text":"hello"}`)
	body := w.Body.String()
	expected := "data: {\"text\":\"hello\"}\n\n"
	if body != expected {
		t.Errorf("got %q, want %q", body, expected)
	}
}

func TestResolveAgentIDFromModel(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"openacosmi:myagent", "myagent"},
		{"openacosmi/myagent", "myagent"},
		{"agent:myagent", "myagent"},
		{"OPENCLAW:MyAgent", "myagent"},
		{"gpt-4", ""},
		{"", ""},
		{"openacosmi:", ""},
		{"agent:", ""},
	}
	for _, tt := range tests {
		result := ResolveAgentIDFromModel(tt.model)
		if result != tt.expected {
			t.Errorf("ResolveAgentIDFromModel(%q) = %q, want %q", tt.model, result, tt.expected)
		}
	}
}

func TestResolveAgentIDFromHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-OpenAcosmi-Agent-Id", "TestAgent")
	id := ResolveAgentIDFromHeader(r)
	if id != "testagent" {
		t.Errorf("got %q, want testagent", id)
	}
}

func TestResolveAgentIDFromHeader_Fallback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-OpenAcosmi-Agent", "OtherAgent")
	id := ResolveAgentIDFromHeader(r)
	if id != "otheragent" {
		t.Errorf("got %q, want otheragent", id)
	}
}

func TestResolveAgentIDForRequest_Default(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	id := ResolveAgentIDForRequest(r, "")
	if id != "main" {
		t.Errorf("got %q, want main", id)
	}
}

func TestResolveAgentIDForRequest_FromModel(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	id := ResolveAgentIDForRequest(r, "openacosmi:custom")
	if id != "custom" {
		t.Errorf("got %q, want custom", id)
	}
}

func TestResolveAgentIDForRequest_HeaderPriority(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-OpenAcosmi-Agent-Id", "fromheader")
	id := ResolveAgentIDForRequest(r, "openacosmi:frommodel")
	if id != "fromheader" {
		t.Errorf("got %q, want fromheader (header takes priority)", id)
	}
}
