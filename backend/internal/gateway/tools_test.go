package gateway

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseToolInvokeRequest_Valid(t *testing.T) {
	body := `{"toolName":"search","arguments":{"query":"hello"}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	result, err := ParseToolInvokeRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ToolName != "search" {
		t.Errorf("toolName = %q", result.ToolName)
	}
	q, _ := result.Arguments["query"].(string)
	if q != "hello" {
		t.Errorf("query = %q", q)
	}
}

func TestParseToolInvokeRequest_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/tools/invoke", nil)
	_, err := ParseToolInvokeRequest(req)
	if err == nil {
		t.Error("GET should be rejected")
	}
}

func TestParseToolInvokeRequest_MissingToolName(t *testing.T) {
	body := `{"arguments":{}}`
	req := httptest.NewRequest(http.MethodPost, "/tools/invoke", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	_, err := ParseToolInvokeRequest(req)
	if err == nil {
		t.Error("missing toolName should error")
	}
}

func TestToolRegistry(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&ToolRegistration{Name: "search", Policy: ToolPolicyAllow})
	r.Register(&ToolRegistration{Name: "execute", Policy: ToolPolicyPrompt})

	if r.Get("search") == nil {
		t.Error("search should exist")
	}
	if r.Get("nonexistent") != nil {
		t.Error("nonexistent should be nil")
	}
	if len(r.List()) != 2 {
		t.Errorf("expected 2 tools, got %d", len(r.List()))
	}
}

func TestResolveToolPolicy(t *testing.T) {
	if ResolveToolPolicy(nil, ToolPolicyAllow) != ToolPolicyDeny {
		t.Error("nil tool should deny")
	}
	if ResolveToolPolicy(&ToolRegistration{Policy: ToolPolicyPrompt}, ToolPolicyAllow) != ToolPolicyPrompt {
		t.Error("tool policy should take precedence")
	}
	if ResolveToolPolicy(&ToolRegistration{}, ToolPolicyPrompt) != ToolPolicyPrompt {
		t.Error("should fallback to default")
	}
	if ResolveToolPolicy(&ToolRegistration{}, "") != ToolPolicyAllow {
		t.Error("no policy should default to allow")
	}
}

func TestSendToolResponse(t *testing.T) {
	w := httptest.NewRecorder()
	SendToolResponse(w, http.StatusOK, "result-data", "")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Errorf("body = %s", w.Body.String())
	}
}
