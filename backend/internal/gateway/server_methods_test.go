package gateway

import (
	"testing"
)

// ---------- AuthorizeGatewayMethod ----------

func TestAuthorizeGatewayMethod_NilClient(t *testing.T) {
	if err := AuthorizeGatewayMethod("sessions.list", nil); err != nil {
		t.Errorf("nil client should be allowed, got %v", err)
	}
}

func TestAuthorizeGatewayMethod_AdminScope(t *testing.T) {
	client := &GatewayClient{Connect: &ConnectParamsFull{
		Role: "operator", Scopes: []string{"operator.admin"},
	}}
	// Admin can do anything
	for _, method := range []string{"sessions.list", "config.get", "chat.send"} {
		if err := AuthorizeGatewayMethod(method, client); err != nil {
			t.Errorf("admin should access %q, got %v", method, err)
		}
	}
}

func TestAuthorizeGatewayMethod_ReadScope(t *testing.T) {
	client := &GatewayClient{Connect: &ConnectParamsFull{
		Role: "operator", Scopes: []string{"operator.read"},
	}}
	if err := AuthorizeGatewayMethod("sessions.list", client); err != nil {
		t.Errorf("read scope should access sessions.list, got %v", err)
	}
	if err := AuthorizeGatewayMethod("config.get", client); err == nil {
		t.Error("read scope should NOT access config.get")
	}
}

func TestAuthorizeGatewayMethod_WriteScope(t *testing.T) {
	client := &GatewayClient{Connect: &ConnectParamsFull{
		Role: "operator", Scopes: []string{"operator.write"},
	}}
	if err := AuthorizeGatewayMethod("chat.send", client); err != nil {
		t.Errorf("write scope should access chat.send, got %v", err)
	}
	if err := AuthorizeGatewayMethod("sessions.delete", client); err == nil {
		t.Error("write scope should NOT access sessions.delete (admin)")
	}
}

func TestAuthorizeGatewayMethod_NodeRole(t *testing.T) {
	client := &GatewayClient{Connect: &ConnectParamsFull{Role: "node"}}
	if err := AuthorizeGatewayMethod("node.invoke.result", client); err != nil {
		t.Errorf("node should access node.invoke.result, got %v", err)
	}
	if err := AuthorizeGatewayMethod("sessions.list", client); err == nil {
		t.Error("node should NOT access sessions.list")
	}
}

// ---------- MethodRegistry ----------

func TestMethodRegistry_RegisterAndGet(t *testing.T) {
	r := NewMethodRegistry()
	called := false
	r.Register("test.method", func(ctx *MethodHandlerContext) {
		called = true
	})

	handler := r.Get("test.method")
	if handler == nil {
		t.Fatal("handler should not be nil")
	}
	handler(&MethodHandlerContext{})
	if !called {
		t.Error("handler was not called")
	}
}

func TestMethodRegistry_UnknownMethod(t *testing.T) {
	r := NewMethodRegistry()
	if r.Get("nonexistent") != nil {
		t.Error("unknown method should return nil")
	}
}

// ---------- HandleGatewayRequest ----------

func TestHandleGatewayRequest_UnknownMethod(t *testing.T) {
	r := NewMethodRegistry()
	req := &RequestFrame{Method: "nonexistent"}
	var gotOK bool
	var gotErr *ErrorShape
	respond := func(ok bool, _ interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{}, respond)
	if gotOK {
		t.Error("should not be ok")
	}
	if gotErr == nil || gotErr.Code != ErrCodeBadRequest {
		t.Errorf("expected bad_request error, got %v", gotErr)
	}
}

func TestHandleGatewayRequest_Success(t *testing.T) {
	r := NewMethodRegistry()
	r.Register("echo", func(ctx *MethodHandlerContext) {
		ctx.Respond(true, ctx.Params, nil)
	})
	req := &RequestFrame{Method: "echo", Params: map[string]interface{}{"key": "val"}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{}, respond)
	if !gotOK {
		t.Error("should be ok")
	}
	m, _ := gotPayload.(map[string]interface{})
	if m["key"] != "val" {
		t.Errorf("expected key=val, got %v", gotPayload)
	}
}

// ---------- SessionsHandlers ----------

func TestSessionsHandlers_ListEmpty(t *testing.T) {
	store := NewSessionStore("")
	r := NewMethodRegistry()
	r.RegisterAll(SessionsHandlers())

	req := &RequestFrame{Method: "sessions.list", Params: map[string]interface{}{}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{SessionStore: store}, respond)
	if !gotOK {
		t.Error("should be ok")
	}
	result, ok := gotPayload.(SessionsListResult)
	if !ok {
		t.Fatalf("expected SessionsListResult, got %T", gotPayload)
	}
	if result.Count != 0 {
		t.Errorf("expected 0 sessions, got %d", result.Count)
	}
}

func TestSessionsHandlers_PatchAndResolve(t *testing.T) {
	store := NewSessionStore("")
	r := NewMethodRegistry()
	r.RegisterAll(SessionsHandlers())

	// Patch
	patchReq := &RequestFrame{Method: "sessions.patch", Params: map[string]interface{}{
		"key": "test-session", "displayName": "My Session",
	}}
	var patchOK bool
	HandleGatewayRequest(r, patchReq, nil, &GatewayMethodContext{SessionStore: store},
		func(ok bool, _ interface{}, _ *ErrorShape) { patchOK = ok })
	if !patchOK {
		t.Error("patch should succeed")
	}

	// Resolve
	resolveReq := &RequestFrame{Method: "sessions.resolve", Params: map[string]interface{}{"key": "test-session"}}
	var resolvePayload interface{}
	HandleGatewayRequest(r, resolveReq, nil, &GatewayMethodContext{SessionStore: store},
		func(_ bool, payload interface{}, _ *ErrorShape) { resolvePayload = payload })
	m, _ := resolvePayload.(map[string]interface{})
	if m["key"] != "test-session" {
		t.Errorf("expected key=test-session, got %v", m)
	}
}

func TestSessionsHandlers_Delete(t *testing.T) {
	store := NewSessionStore("")
	store.Save(&SessionEntry{SessionKey: "to-delete"})
	r := NewMethodRegistry()
	r.RegisterAll(SessionsHandlers())

	req := &RequestFrame{Method: "sessions.delete", Params: map[string]interface{}{"key": "to-delete"}}
	var gotOK bool
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{SessionStore: store},
		func(ok bool, _ interface{}, _ *ErrorShape) { gotOK = ok })
	if !gotOK {
		t.Error("delete should succeed")
	}
	if store.Count() != 0 {
		t.Errorf("store should be empty, got %d", store.Count())
	}
}
