package gateway

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropic/open-acosmi/internal/agents/workspace"
	"github.com/anthropic/open-acosmi/pkg/types"
)

// ---------- agents.files.list ----------

func TestAgentFilesHandlers_ListWithConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 2 bootstrap files
	os.WriteFile(filepath.Join(tmpDir, workspace.DefaultSoulFilename), []byte("soul content"), 0o644)
	os.WriteFile(filepath.Join(tmpDir, workspace.DefaultMemoryFilename), []byte("memory"), 0o644)

	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			List: []types.AgentListItemConfig{
				{ID: "test-agent", Workspace: tmpDir},
			},
		},
	}

	r := NewMethodRegistry()
	r.RegisterAll(AgentFilesHandlers())

	req := &RequestFrame{Method: "agents.files.list", Params: map[string]interface{}{
		"agentId": "test-agent",
	}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{Config: cfg}, respond)
	if !gotOK {
		t.Fatal("agents.files.list should succeed")
	}
	result, ok := gotPayload.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", gotPayload)
	}
	files, ok := result["files"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map, got %T", result["files"])
	}

	// Should have 7 bootstrap files + 1 memory = 8
	if len(files) != 8 {
		t.Errorf("expected 8 files, got %d", len(files))
	}

	// SOUL.md should exist (missing=false)
	found := false
	for _, f := range files {
		if f["name"] == workspace.DefaultSoulFilename {
			found = true
			if f["missing"] != false {
				t.Error("SOUL.md should not be missing")
			}
		}
	}
	if !found {
		t.Error("SOUL.md not found in file list")
	}
}

func TestAgentFilesHandlers_ListMissingAgent(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			List: []types.AgentListItemConfig{
				{ID: "other-agent"},
			},
		},
	}

	r := NewMethodRegistry()
	r.RegisterAll(AgentFilesHandlers())

	req := &RequestFrame{Method: "agents.files.list", Params: map[string]interface{}{
		"agentId": "nonexistent",
	}}
	var gotOK bool
	var gotErr *ErrorShape
	respond := func(ok bool, _ interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{Config: cfg}, respond)
	if gotOK {
		t.Error("should fail for unknown agent")
	}
	if gotErr == nil || gotErr.Code != ErrCodeBadRequest {
		t.Errorf("expected bad_request, got %v", gotErr)
	}
}

// ---------- agents.files.get ----------

func TestAgentFilesHandlers_GetContent(t *testing.T) {
	tmpDir := t.TempDir()
	testContent := "# Soul file\nHello world"
	os.WriteFile(filepath.Join(tmpDir, workspace.DefaultSoulFilename), []byte(testContent), 0o644)

	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			List: []types.AgentListItemConfig{
				{ID: "test-agent", Workspace: tmpDir},
			},
		},
	}

	r := NewMethodRegistry()
	r.RegisterAll(AgentFilesHandlers())

	req := &RequestFrame{Method: "agents.files.get", Params: map[string]interface{}{
		"agentId": "test-agent",
		"name":    workspace.DefaultSoulFilename,
	}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{Config: cfg}, respond)
	if !gotOK {
		t.Fatal("agents.files.get should succeed")
	}
	result := gotPayload.(map[string]interface{})
	file := result["file"].(map[string]interface{})
	if file["content"] != testContent {
		t.Errorf("expected content %q, got %q", testContent, file["content"])
	}
	if file["missing"] != false {
		t.Error("file should not be missing")
	}
}

func TestAgentFilesHandlers_GetMissingFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			List: []types.AgentListItemConfig{
				{ID: "test-agent", Workspace: tmpDir},
			},
		},
	}

	r := NewMethodRegistry()
	r.RegisterAll(AgentFilesHandlers())

	req := &RequestFrame{Method: "agents.files.get", Params: map[string]interface{}{
		"agentId": "test-agent",
		"name":    workspace.DefaultSoulFilename,
	}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{Config: cfg}, respond)
	if !gotOK {
		t.Fatal("agents.files.get should succeed (missing file returns ok with missing=true)")
	}
	result := gotPayload.(map[string]interface{})
	file := result["file"].(map[string]interface{})
	if file["missing"] != true {
		t.Error("file should be missing")
	}
}

func TestAgentFilesHandlers_GetDisallowed(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			List: []types.AgentListItemConfig{
				{ID: "test-agent", Workspace: tmpDir},
			},
		},
	}

	r := NewMethodRegistry()
	r.RegisterAll(AgentFilesHandlers())

	req := &RequestFrame{Method: "agents.files.get", Params: map[string]interface{}{
		"agentId": "test-agent",
		"name":    "evil.sh",
	}}
	var gotOK bool
	var gotErr *ErrorShape
	respond := func(ok bool, _ interface{}, err *ErrorShape) {
		gotOK = ok
		gotErr = err
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{Config: cfg}, respond)
	if gotOK {
		t.Error("should fail for disallowed file name")
	}
	if gotErr == nil || gotErr.Code != ErrCodeBadRequest {
		t.Errorf("expected bad_request, got %v", gotErr)
	}
}

// ---------- agents.files.set ----------

func TestAgentFilesHandlers_SetContent(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &types.OpenAcosmiConfig{
		Agents: &types.AgentsConfig{
			List: []types.AgentListItemConfig{
				{ID: "test-agent", Workspace: tmpDir},
			},
		},
	}

	r := NewMethodRegistry()
	r.RegisterAll(AgentFilesHandlers())

	newContent := "# Updated soul\nNew content here"
	req := &RequestFrame{Method: "agents.files.set", Params: map[string]interface{}{
		"agentId": "test-agent",
		"name":    workspace.DefaultSoulFilename,
		"content": newContent,
	}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{Config: cfg}, respond)
	if !gotOK {
		t.Fatal("agents.files.set should succeed")
	}
	result := gotPayload.(map[string]interface{})
	if result["ok"] != true {
		t.Error("expected ok=true")
	}
	file := result["file"].(map[string]interface{})
	if file["content"] != newContent {
		t.Errorf("expected content %q, got %q", newContent, file["content"])
	}

	// Verify file on disk
	diskContent, err := os.ReadFile(filepath.Join(tmpDir, workspace.DefaultSoulFilename))
	if err != nil {
		t.Fatalf("file should exist on disk: %v", err)
	}
	if string(diskContent) != newContent {
		t.Errorf("disk content mismatch: got %q", string(diskContent))
	}
}
