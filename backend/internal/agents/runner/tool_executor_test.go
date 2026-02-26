package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic/open-acosmi/pkg/types"
)

// ============================================================================
// Tool Executor 集成测试
// 验证 bash, read_file, write_file, list_dir 工具的完整执行链路。
// ============================================================================

// testToolParams 返回测试用的 ToolExecParams（权限全开）。
func testToolParams(workspaceDir string) ToolExecParams {
	return ToolExecParams{
		WorkspaceDir: workspaceDir,
		AllowExec:    true,
		AllowWrite:   true,
	}
}

func testToolParamsWithTimeout(workspaceDir string, timeoutMs int64) ToolExecParams {
	p := testToolParams(workspaceDir)
	p.TimeoutMs = timeoutMs
	return p
}

// ---------- bash ----------

func TestBash_SimpleEcho(t *testing.T) {
	result, err := ExecuteToolCall(context.Background(), "bash",
		json.RawMessage(`{"command":"echo hello world"}`),
		testToolParams(t.TempDir()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected 'hello world' in output, got %q", result)
	}
}

func TestBash_ExitCode(t *testing.T) {
	result, err := ExecuteToolCall(context.Background(), "bash",
		json.RawMessage(`{"command":"exit 42"}`),
		testToolParams(t.TempDir()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "exit code: 42") {
		t.Errorf("expected exit code 42, got %q", result)
	}
}

func TestBash_EmptyCommand(t *testing.T) {
	_, err := ExecuteToolCall(context.Background(), "bash",
		json.RawMessage(`{"command":""}`),
		testToolParams(t.TempDir()))
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestBash_WorkspaceDir(t *testing.T) {
	dir := t.TempDir()
	result, err := ExecuteToolCall(context.Background(), "bash",
		json.RawMessage(`{"command":"pwd"}`),
		testToolParams(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, dir) {
		t.Errorf("expected workspace dir %q in output, got %q", dir, result)
	}
}

func TestBash_Timeout(t *testing.T) {
	result, err := ExecuteToolCall(context.Background(), "bash",
		json.RawMessage(`{"command":"sleep 10"}`),
		testToolParamsWithTimeout(t.TempDir(), 200))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "timed out") {
		t.Errorf("expected timeout message, got %q", result)
	}
}

func TestBash_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately
	_, err := ExecuteToolCall(ctx, "bash",
		json.RawMessage(`{"command":"sleep 10"}`),
		testToolParams(t.TempDir()))
	// Should not hang — context already cancelled
	if err != nil {
		t.Logf("got error (expected): %v", err)
	}
}

// ---------- read_file ----------

func TestReadFile_Simple(t *testing.T) {
	dir := t.TempDir()
	content := "hello from test file\nline 2\n"
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0644)

	result, err := ExecuteToolCall(context.Background(), "read_file",
		json.RawMessage(`{"path":"test.txt"}`),
		testToolParams(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != content {
		t.Errorf("expected %q, got %q", content, result)
	}
}

func TestReadFile_AbsolutePath(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "abs.txt")
	os.WriteFile(absPath, []byte("absolute"), 0644)

	inputJSON, _ := json.Marshal(map[string]string{"path": absPath})
	result, err := ExecuteToolCall(context.Background(), "read_file",
		inputJSON, testToolParams(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "absolute" {
		t.Errorf("expected 'absolute', got %q", result)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	result, err := ExecuteToolCall(context.Background(), "read_file",
		json.RawMessage(`{"path":"nonexistent.txt"}`),
		testToolParams(t.TempDir()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Error reading file") {
		t.Errorf("expected error message, got %q", result)
	}
}

// ---------- write_file ----------

func TestWriteFile_Simple(t *testing.T) {
	dir := t.TempDir()
	result, err := ExecuteToolCall(context.Background(), "write_file",
		json.RawMessage(`{"path":"output.txt","content":"test content"}`),
		testToolParams(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("expected success message, got %q", result)
	}

	// Verify file contents
	data, _ := os.ReadFile(filepath.Join(dir, "output.txt"))
	if string(data) != "test content" {
		t.Errorf("expected 'test content', got %q", string(data))
	}
}

func TestWriteFile_CreatesSubdirectory(t *testing.T) {
	dir := t.TempDir()
	result, err := ExecuteToolCall(context.Background(), "write_file",
		json.RawMessage(`{"path":"sub/dir/file.txt","content":"nested"}`),
		testToolParams(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Successfully wrote") {
		t.Errorf("expected success message, got %q", result)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "sub", "dir", "file.txt"))
	if string(data) != "nested" {
		t.Errorf("expected 'nested', got %q", string(data))
	}
}

func TestWriteFile_ThenReadFile(t *testing.T) {
	dir := t.TempDir()

	// Write
	_, err := ExecuteToolCall(context.Background(), "write_file",
		json.RawMessage(`{"path":"roundtrip.txt","content":"round trip works"}`),
		testToolParams(dir))
	if err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Read back
	result, err := ExecuteToolCall(context.Background(), "read_file",
		json.RawMessage(`{"path":"roundtrip.txt"}`),
		testToolParams(dir))
	if err != nil {
		t.Fatalf("read error: %v", err)
	}
	if result != "round trip works" {
		t.Errorf("expected 'round trip works', got %q", result)
	}
}

// ---------- list_dir ----------

func TestListDir_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	result, err := ExecuteToolCall(context.Background(), "list_dir",
		json.RawMessage(`{"path":"."}`),
		testToolParams(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty output, got %q", result)
	}
}

func TestListDir_WithFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	result, err := ExecuteToolCall(context.Background(), "list_dir",
		json.RawMessage(`{"path":"."}`),
		testToolParams(dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "a.txt") {
		t.Errorf("expected a.txt in output, got %q", result)
	}
	if !strings.Contains(result, "b.txt") {
		t.Errorf("expected b.txt in output, got %q", result)
	}
	if !strings.Contains(result, "d subdir") {
		t.Errorf("expected 'd subdir' in output, got %q", result)
	}
}

func TestListDir_NotFound(t *testing.T) {
	result, err := ExecuteToolCall(context.Background(), "list_dir",
		json.RawMessage(`{"path":"nonexistent"}`),
		testToolParams(t.TempDir()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Error listing directory") {
		t.Errorf("expected error message, got %q", result)
	}
}

// ---------- unknown tool ----------

func TestUnknownTool(t *testing.T) {
	result, err := ExecuteToolCall(context.Background(), "browser",
		json.RawMessage(`{"url":"http://example.com"}`),
		testToolParams(t.TempDir()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "not yet implemented") {
		t.Errorf("expected 'not yet implemented', got %q", result)
	}
}

// ---------- invalid JSON ----------

func TestBash_InvalidJSON(t *testing.T) {
	_, err := ExecuteToolCall(context.Background(), "bash",
		json.RawMessage(`{invalid`),
		testToolParams(t.TempDir()))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestReadFile_InvalidJSON(t *testing.T) {
	_, err := ExecuteToolCall(context.Background(), "read_file",
		json.RawMessage(`not json`),
		testToolParams(t.TempDir()))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------- bash + write_file + read_file 集成 ----------

func TestToolChain_BashWriteRead(t *testing.T) {
	dir := t.TempDir()
	params := testToolParams(dir)

	// Use bash to write a file
	_, err := ExecuteToolCall(context.Background(), "bash",
		json.RawMessage(`{"command":"echo 'generated content' > gen.txt"}`),
		params)
	if err != nil {
		t.Fatalf("bash error: %v", err)
	}

	// Read it back with read_file
	result, err := ExecuteToolCall(context.Background(), "read_file",
		json.RawMessage(`{"path":"gen.txt"}`),
		params)
	if err != nil {
		t.Fatalf("read_file error: %v", err)
	}
	if !strings.Contains(result, "generated content") {
		t.Errorf("expected 'generated content' in output, got %q", result)
	}

	// List directory to verify
	listing, err := ExecuteToolCall(context.Background(), "list_dir",
		json.RawMessage(`{"path":"."}`),
		params)
	if err != nil {
		t.Fatalf("list_dir error: %v", err)
	}
	if !strings.Contains(listing, "gen.txt") {
		t.Errorf("expected gen.txt in listing, got %q", listing)
	}
}

// ============================================================================
// P0 权限守卫测试
// 验证 AllowExec=false / AllowWrite=false 时工具被正确拒绝，
// 以及 resolveAllowWrite / resolveAllowExec 的安全级别映射。
// ============================================================================

// ---------- 权限拒绝测试 ----------

func TestBash_PermissionDenied(t *testing.T) {
	var callbackTool, callbackDetail string
	params := ToolExecParams{
		WorkspaceDir: t.TempDir(),
		AllowExec:    false,
		AllowWrite:   true,
		OnPermissionDenied: func(tool, detail string) {
			callbackTool = tool
			callbackDetail = detail
		},
	}
	result, err := ExecuteToolCall(context.Background(), "bash",
		json.RawMessage(`{"command":"echo should not run"}`), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "权限不足") {
		t.Errorf("expected permission denied message, got %q", result)
	}
	if callbackTool != "bash" {
		t.Errorf("expected callback tool=bash, got %q", callbackTool)
	}
	if callbackDetail != "echo should not run" {
		t.Errorf("expected callback detail, got %q", callbackDetail)
	}
}

func TestWriteFile_PermissionDenied(t *testing.T) {
	var callbackTool string
	params := ToolExecParams{
		WorkspaceDir: t.TempDir(),
		AllowExec:    true,
		AllowWrite:   false,
		OnPermissionDenied: func(tool, detail string) {
			callbackTool = tool
		},
	}
	result, err := ExecuteToolCall(context.Background(), "write_file",
		json.RawMessage(`{"path":"test.txt","content":"blocked"}`), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "权限不足") {
		t.Errorf("expected permission denied message, got %q", result)
	}
	if callbackTool != "write_file" {
		t.Errorf("expected callback tool=write_file, got %q", callbackTool)
	}
}

// ---------- resolveAllowWrite / resolveAllowExec 映射测试 ----------

func TestResolvePermissions_Full(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Exec: &types.ExecToolConfig{Security: "full"},
		},
	}
	if !resolveAllowWrite(cfg) {
		t.Error("expected AllowWrite=true for security=full")
	}
	if !resolveAllowExec(cfg) {
		t.Error("expected AllowExec=true for security=full")
	}
}

func TestResolvePermissions_Allowlist(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Exec: &types.ExecToolConfig{Security: "allowlist"},
		},
	}
	if resolveAllowWrite(cfg) {
		t.Error("expected AllowWrite=false for security=allowlist")
	}
	if !resolveAllowExec(cfg) {
		t.Error("expected AllowExec=true for security=allowlist")
	}
}

func TestResolvePermissions_Deny(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Exec: &types.ExecToolConfig{Security: "deny"},
		},
	}
	if resolveAllowWrite(cfg) {
		t.Error("expected AllowWrite=false for security=deny")
	}
	if resolveAllowExec(cfg) {
		t.Error("expected AllowExec=false for security=deny")
	}
}

func TestResolvePermissions_Empty(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Exec: &types.ExecToolConfig{Security: ""},
		},
	}
	if resolveAllowWrite(cfg) {
		t.Error("expected AllowWrite=false for empty security")
	}
	if resolveAllowExec(cfg) {
		t.Error("expected AllowExec=false for empty security")
	}
}

func TestResolvePermissions_NilConfig(t *testing.T) {
	if resolveAllowWrite(nil) {
		t.Error("expected AllowWrite=false for nil config")
	}
	if resolveAllowExec(nil) {
		t.Error("expected AllowExec=false for nil config")
	}
}

func TestResolvePermissions_Sandbox(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{
		Tools: &types.ToolsConfig{
			Exec: &types.ExecToolConfig{Security: "sandbox"},
		},
	}
	if resolveAllowWrite(cfg) {
		t.Error("expected AllowWrite=false for security=sandbox")
	}
	if !resolveAllowExec(cfg) {
		t.Error("expected AllowExec=true for security=sandbox (alias for allowlist)")
	}
}

func TestResolvePermissions_NilTools(t *testing.T) {
	cfg := &types.OpenAcosmiConfig{}
	if resolveAllowWrite(cfg) {
		t.Error("expected AllowWrite=false for nil Tools")
	}
	if resolveAllowExec(cfg) {
		t.Error("expected AllowExec=false for nil Tools")
	}
}

// ============================================================================
// 路径逃逸防护测试
// 验证 validateToolPath 及各工具的路径边界检查。
// ============================================================================

// ---------- validateToolPath 单元测试 ----------

func TestValidateToolPath_AllowsInside(t *testing.T) {
	workspace := t.TempDir()
	// 工作空间内的相对路径
	innerPath := filepath.Join(workspace, "sub", "file.txt")
	if err := validateToolPath(innerPath, workspace); err != nil {
		t.Errorf("expected nil for inside path, got %v", err)
	}
	// 工作空间本身
	if err := validateToolPath(workspace, workspace); err != nil {
		t.Errorf("expected nil for workspace itself, got %v", err)
	}
}

func TestValidateToolPath_BlocksEscape(t *testing.T) {
	workspace := t.TempDir()
	escapePath := filepath.Join(workspace, "..", "escape.txt")
	err := validateToolPath(escapePath, workspace)
	if err == nil {
		t.Error("expected error for ../ escape path, got nil")
	}
	if !strings.Contains(err.Error(), "outside workspace") {
		t.Errorf("expected 'outside workspace' in error, got %q", err.Error())
	}
}

func TestValidateToolPath_BlocksAbsOutside(t *testing.T) {
	workspace := t.TempDir()
	err := validateToolPath("/tmp/evil.txt", workspace)
	if err == nil {
		t.Error("expected error for absolute path outside workspace, got nil")
	}
	if !strings.Contains(err.Error(), "outside workspace") {
		t.Errorf("expected 'outside workspace' in error, got %q", err.Error())
	}
}

func TestValidateToolPath_EmptyWorkspace(t *testing.T) {
	// 无工作空间约束时应放行
	if err := validateToolPath("/any/path", ""); err != nil {
		t.Errorf("expected nil for empty workspace, got %v", err)
	}
}

// ---------- 工具级路径逃逸测试 ----------

func TestWriteFile_PathEscapeBlocked(t *testing.T) {
	workspace := t.TempDir()
	outsidePath := filepath.Join(workspace, "..", "escape.txt")
	var callbackTool, callbackDetail string
	inputJSON, _ := json.Marshal(map[string]string{
		"path":    outsidePath,
		"content": "should not be written",
	})
	params := testToolParams(workspace)
	params.OnPermissionDenied = func(tool, detail string) {
		callbackTool = tool
		callbackDetail = detail
	}
	result, err := ExecuteToolCall(context.Background(), "write_file",
		inputJSON, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "权限不足") {
		t.Errorf("expected permission denied message, got %q", result)
	}
	if callbackTool != "write_file" {
		t.Errorf("expected OnPermissionDenied callback tool=write_file, got %q", callbackTool)
	}
	if callbackDetail != outsidePath {
		t.Errorf("expected OnPermissionDenied callback detail=%q, got %q", outsidePath, callbackDetail)
	}
	// 确认文件没有被创建
	if _, statErr := os.Stat(outsidePath); statErr == nil {
		os.Remove(outsidePath)
		t.Error("file was created outside workspace — security breach!")
	}
}

func TestReadFile_GlobalReadAllowed(t *testing.T) {
	// 全局可读: 读取工作空间外的文件应当成功（L0/L1/L2 均允许）
	workspace := t.TempDir()
	var callbackTool string
	inputJSON, _ := json.Marshal(map[string]string{"path": "/etc/hosts"})
	params := testToolParams(workspace)
	params.OnPermissionDenied = func(tool, detail string) {
		callbackTool = tool
	}
	result, err := ExecuteToolCall(context.Background(), "read_file",
		inputJSON, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 应当返回文件内容而非权限拒绝
	if strings.Contains(result, "权限不足") {
		t.Errorf("read_file should allow reading outside workspace, got permission denied")
	}
	if strings.Contains(result, "Error reading file") {
		t.Logf("file not readable (ok on some OS), got: %q", result)
	}
	if callbackTool != "" {
		t.Errorf("OnPermissionDenied should NOT be called for reads, got tool=%q", callbackTool)
	}
}

func TestListDir_GlobalReadAllowed(t *testing.T) {
	// 全局可读: 列出工作空间外的目录应当成功（L0/L1/L2 均允许）
	workspace := t.TempDir()
	var callbackTool string
	inputJSON, _ := json.Marshal(map[string]string{"path": "/tmp"})
	params := testToolParams(workspace)
	params.OnPermissionDenied = func(tool, detail string) {
		callbackTool = tool
	}
	result, err := ExecuteToolCall(context.Background(), "list_dir",
		inputJSON, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 应当返回目录内容而非权限拒绝
	if strings.Contains(result, "权限不足") {
		t.Errorf("list_dir should allow listing outside workspace, got permission denied")
	}
	if callbackTool != "" {
		t.Errorf("OnPermissionDenied should NOT be called for reads, got tool=%q", callbackTool)
	}
}
