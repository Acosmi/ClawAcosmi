package sandbox

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Sandbox 单元测试 — 不依赖 Docker 的纯逻辑测试
// ============================================================================

// ---------- SlugifySessionKey ----------

func TestSlugifySessionKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"Hello World", "hello-world"},
		{"test:session:123", "test-session-123"},
		{"a/b/c", "a-b-c"},
		{"UPPER CASE", "upper-case"},
		{"  spaces  ", "spaces"},
		{"---dashes---", "dashes"},
		{"", "default"},
		{"abc-def", "abc-def"},
		{"special!@#chars", "special-chars"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SlugifySessionKey(tt.input)
			if got != tt.want {
				t.Errorf("SlugifySessionKey(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- ResolveSandboxWorkspaceDir ----------

func TestResolveSandboxWorkspaceDir(t *testing.T) {
	tests := []struct {
		stateDir, agentID string
		scope             SandboxScope
		sessionKey        string
		wantContains      string
	}{
		{"/state", "agent-1", ScopeSession, "session-1", "workspaces"},
		{"/state", "agent-1", ScopeAgent, "session-1", "agent-1"},
		{"/state", "agent-2", ScopeShared, "session-1", "shared"},
	}
	for _, tt := range tests {
		t.Run(string(tt.scope), func(t *testing.T) {
			got := ResolveSandboxWorkspaceDir(tt.stateDir, tt.agentID, tt.scope, tt.sessionKey)
			if got == "" {
				t.Fatal("expected non-empty path")
			}
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("expected path to contain %q, got %q", tt.wantContains, got)
			}
		})
	}
}

// ---------- ResolveSandboxAgentId ----------

func TestResolveSandboxAgentId(t *testing.T) {
	tests := []struct {
		sessionKey string
		want       string
	}{
		{"agent-1/session-abc", "agent-1"},
		{"default/sess", "default"},
		{"simple", "simple"},
		{"a/b/c", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.sessionKey, func(t *testing.T) {
			got := ResolveSandboxAgentId(tt.sessionKey)
			if got != tt.want {
				t.Errorf("ResolveSandboxAgentId(%q) = %q, want %q", tt.sessionKey, got, tt.want)
			}
		})
	}
}

// ---------- NormalizeAndHashConfig ----------

func TestNormalizeAndHashConfig(t *testing.T) {
	cfg1 := SandboxConfig{
		Enabled: true,
		Scope:   ScopeSession,
		Docker: SandboxDockerConfig{
			Image:    "test-image:latest",
			MemoryMB: 512,
			CPUs:     1.0,
		},
	}

	// 相同配置应产生相同哈希
	hash1 := NormalizeAndHashConfig(cfg1)
	hash2 := NormalizeAndHashConfig(cfg1)
	if hash1 != hash2 {
		t.Errorf("same config produced different hashes: %q vs %q", hash1, hash2)
	}
	if hash1 == "" {
		t.Error("hash should not be empty")
	}

	// 不同配置应产生不同哈希
	cfg2 := cfg1
	cfg2.Docker.MemoryMB = 1024
	hash3 := NormalizeAndHashConfig(cfg2)
	if hash1 == hash3 {
		t.Error("different configs should produce different hashes")
	}
}

// ---------- BuildCreateArgs ----------

func TestBuildCreateArgs(t *testing.T) {
	cfg := SandboxConfig{
		Docker: SandboxDockerConfig{
			Image:        "test-image:v1",
			Workdir:      "/custom",
			Network:      "host",
			User:         "testuser",
			ReadOnlyRoot: true,
			MemoryMB:     256,
			CPUs:         0.5,
			Tmpfs:        []string{"/tmp:rw,size=64m"},
			Capabilities: []string{"NET_ADMIN"},
			Env:          map[string]string{"FOO": "bar"},
		},
	}

	args := BuildCreateArgs(cfg, "test-container", "test-session", "hash123")

	assertContains := func(flag string) {
		t.Helper()
		for _, a := range args {
			if a == flag {
				return
			}
		}
		t.Errorf("expected args to contain %q, got %v", flag, args)
	}

	assertContainsPair := func(flag, value string) {
		t.Helper()
		for i, a := range args {
			if a == flag && i+1 < len(args) && args[i+1] == value {
				return
			}
		}
		t.Errorf("expected args to contain %q %q, got %v", flag, value, args)
	}

	// Verify key arguments
	assertContains("create")
	assertContainsPair("--name", "test-container")
	assertContainsPair("-w", "/custom")
	assertContainsPair("--network", "host")
	assertContainsPair("--user", "testuser")
	assertContains("--read-only")
	assertContainsPair("--memory", "256m")
	assertContainsPair("--cpus", "0.50")
	assertContainsPair("--cap-add", "NET_ADMIN")
	assertContains("--cap-drop")
	assertContains("test-image:v1")

	// Labels
	found := false
	for _, a := range args {
		if strings.HasPrefix(a, "openacosmi.config-hash=") {
			found = true
		}
	}
	if !found {
		t.Error("expected config-hash label")
	}
}

func TestBuildCreateArgs_Defaults(t *testing.T) {
	cfg := SandboxConfig{}

	args := BuildCreateArgs(cfg, "c1", "s1", "")

	// Should use default workdir
	found := false
	for i, a := range args {
		if a == "-w" && i+1 < len(args) && args[i+1] == DefaultWorkdir {
			found = true
		}
	}
	if !found {
		t.Errorf("expected default workdir %q", DefaultWorkdir)
	}

	// Should use default network=none
	for i, a := range args {
		if a == "--network" && i+1 < len(args) && args[i+1] == "none" {
			return
		}
	}
	t.Error("expected --network none for default config")
}

// ---------- CompileToolPolicy / IsToolAllowed ----------

func TestCompileToolPolicy(t *testing.T) {
	policy := SandboxToolPolicy{
		Allow: []string{"Read", "Write", "Bash*"},
		Deny:  []string{"BrowserNavigate", "Computer"},
	}

	compiled := CompileToolPolicy(policy)

	if compiled == nil {
		t.Fatal("expected non-nil compiled policy")
	}
	if !compiled.AllowExact["Read"] {
		t.Error("expected Read in AllowExact")
	}
	if !compiled.AllowExact["Write"] {
		t.Error("expected Write in AllowExact")
	}
	if len(compiled.AllowPrefixes) != 1 || compiled.AllowPrefixes[0] != "Bash" {
		t.Errorf("expected AllowPrefixes=[Bash], got %v", compiled.AllowPrefixes)
	}
	if !compiled.DenyExact["BrowserNavigate"] {
		t.Error("expected BrowserNavigate in DenyExact")
	}
}

func TestCompileToolPolicy_Wildcard(t *testing.T) {
	policy := SandboxToolPolicy{
		Allow: []string{"*"},
		Deny:  []string{"Computer"},
	}
	compiled := CompileToolPolicy(policy)
	if !compiled.AllowAll {
		t.Error("expected AllowAll=true")
	}
}

func TestIsToolAllowed(t *testing.T) {
	compiled := CompileToolPolicy(SandboxToolPolicy{
		Allow: []string{"Read", "Write", "Bash*"},
		Deny:  []string{"BrowserNavigate", "Computer"},
	})

	tests := []struct {
		tool string
		want bool
	}{
		{"Read", true},
		{"Write", true},
		{"Bash", true},
		{"BashExec", true},
		{"BrowserNavigate", false},
		{"Computer", false},
		{"Unknown", false}, // not in allow list
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			got := IsToolAllowed(compiled, tt.tool)
			if got != tt.want {
				t.Errorf("IsToolAllowed(%q) = %v, want %v", tt.tool, got, tt.want)
			}
		})
	}
}

func TestIsToolAllowed_NilPolicy(t *testing.T) {
	if !IsToolAllowed(nil, "anything") {
		t.Error("nil policy should allow all tools")
	}
}

func TestIsToolAllowed_DenyAll(t *testing.T) {
	compiled := CompileToolPolicy(SandboxToolPolicy{
		Deny: []string{"*"},
	})
	if IsToolAllowed(compiled, "Read") {
		t.Error("DenyAll should block everything")
	}
}

func TestIsToolAllowed_AllowAll_ExceptDeny(t *testing.T) {
	compiled := CompileToolPolicy(SandboxToolPolicy{
		Allow: []string{"*"},
		Deny:  []string{"Computer"},
	})
	if !IsToolAllowed(compiled, "Read") {
		t.Error("AllowAll with specific deny should allow Read")
	}
	if IsToolAllowed(compiled, "Computer") {
		t.Error("AllowAll with Computer deny should block Computer")
	}
}

// ---------- ResolveSandboxConfigForAgent ----------

func TestResolveSandboxConfigForAgent(t *testing.T) {
	global := SandboxConfig{
		Enabled:   true,
		Scope:     ScopeSession,
		Workspace: AccessReadWrite,
		Docker: SandboxDockerConfig{
			Image:    "global-image:latest",
			MemoryMB: 512,
		},
	}

	// Without override
	result := ResolveSandboxConfigForAgent(global, nil)
	if !result.Enabled {
		t.Error("expected enabled=true")
	}
	if result.Docker.Image != "global-image:latest" {
		t.Errorf("expected global image, got %q", result.Docker.Image)
	}

	// With override
	override := &SandboxConfig{
		Docker: SandboxDockerConfig{
			Image:    "override-image:v2",
			MemoryMB: 1024,
		},
	}
	result2 := ResolveSandboxConfigForAgent(global, override)
	if result2.Docker.Image != "override-image:v2" {
		t.Errorf("expected override image, got %q", result2.Docker.Image)
	}
}

// ---------- ResolveSandboxMode ----------

func TestResolveSandboxMode(t *testing.T) {
	tests := []struct {
		name string
		cfg  SandboxConfig
		want string
	}{
		{"enabled enforced", SandboxConfig{Enabled: true}, "enforced"},
		{"disabled off", SandboxConfig{Enabled: false}, "off"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveSandboxMode(tt.cfg)
			if got != tt.want {
				t.Errorf("ResolveSandboxMode() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------- ResolveContainerName ----------

func TestResolveContainerName(t *testing.T) {
	name := ResolveContainerName("my:session:key", ScopeSession, "agent-1")
	if name == "" {
		t.Fatal("expected non-empty container name")
	}
	if !strings.HasPrefix(name, DefaultContainerPrefix) {
		t.Errorf("expected prefix %q, got %q", DefaultContainerPrefix, name)
	}
}

func TestResolveBrowserContainerName(t *testing.T) {
	name := ResolveBrowserContainerName("my:session:key")
	if name == "" {
		t.Fatal("expected non-empty browser container name")
	}
	if !strings.HasPrefix(name, DefaultBrowserContainerPrefix) {
		t.Errorf("expected prefix %q, got %q", DefaultBrowserContainerPrefix, name)
	}
}

// ---------- FormatToolBlockedMessage ----------

func TestFormatToolBlockedMessage(t *testing.T) {
	msg := FormatToolBlockedMessage("Computer")
	if !strings.Contains(msg, "Computer") {
		t.Error("expected message to contain tool name")
	}
	if !strings.Contains(msg, "blocked") {
		t.Error("expected message to contain 'blocked'")
	}
}

// ---------- ResolveToolPolicyForAgent ----------

func TestResolveToolPolicyForAgent(t *testing.T) {
	global := SandboxConfig{
		Tools: SandboxToolPolicy{
			Allow: []string{"Read", "Write"},
			Deny:  []string{"Computer"},
		},
	}

	// Without override → use global
	policy := ResolveToolPolicyForAgent(global, nil)
	if len(policy.Allow) != 2 || policy.Allow[0] != "Read" {
		t.Errorf("expected global allow, got %v", policy.Allow)
	}

	// With override → override takes precedence
	override := &SandboxConfig{
		Tools: SandboxToolPolicy{
			Allow: []string{"*"},
		},
	}
	policy2 := ResolveToolPolicyForAgent(global, override)
	if len(policy2.Allow) != 1 || policy2.Allow[0] != "*" {
		t.Errorf("expected override allow, got %v", policy2.Allow)
	}
}

func TestResolveToolPolicyForAgent_Defaults(t *testing.T) {
	global := SandboxConfig{}
	policy := ResolveToolPolicyForAgent(global, nil)

	// Should use DefaultToolAllow
	if len(policy.Allow) != len(DefaultToolAllow) {
		t.Errorf("expected default allow list length %d, got %d", len(DefaultToolAllow), len(policy.Allow))
	}
	if len(policy.Deny) != len(DefaultToolDeny) {
		t.Errorf("expected default deny list length %d, got %d", len(DefaultToolDeny), len(policy.Deny))
	}
}

// ---------- ResolveSandboxRuntimeStatus ----------

func TestResolveSandboxRuntimeStatus(t *testing.T) {
	cfg := SandboxConfig{Enabled: true}
	status := ResolveSandboxRuntimeStatus(cfg, "agent-1", nil)

	if status.AgentID != "agent-1" {
		t.Errorf("expected agentId=agent-1, got %q", status.AgentID)
	}
	if status.Mode != "enforced" {
		t.Errorf("expected mode=enforced, got %q", status.Mode)
	}
	if !status.IsSandboxed {
		t.Error("expected isSandboxed=true")
	}
	if status.ToolPolicy == nil {
		t.Error("expected non-nil tool policy")
	}
}

// ---------- Registry Read/Write ----------

func TestRegistryReadWrite(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "sandbox", RegistryFilename)

	// Write
	entry := RegistryEntry{
		ContainerName: "test-container-1",
		SessionKey:    "session-1",
		CreatedAtMs:   1000,
		LastUsedAtMs:  2000,
		Image:         "test-image:v1",
		ConfigHash:    "abc123",
	}
	if err := UpdateRegistryEntry(regPath, entry); err != nil {
		t.Fatalf("UpdateRegistryEntry failed: %v", err)
	}

	// Read
	reg, err := ReadRegistry(regPath)
	if err != nil {
		t.Fatalf("ReadRegistry failed: %v", err)
	}
	if len(reg.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(reg.Entries))
	}
	if reg.Entries[0].ContainerName != "test-container-1" {
		t.Errorf("expected container name test-container-1, got %q", reg.Entries[0].ContainerName)
	}

	// Update existing
	entry.LastUsedAtMs = 3000
	if err := UpdateRegistryEntry(regPath, entry); err != nil {
		t.Fatalf("UpdateRegistryEntry (update) failed: %v", err)
	}
	reg, _ = ReadRegistry(regPath)
	if len(reg.Entries) != 1 {
		t.Fatalf("expected 1 entry after update, got %d", len(reg.Entries))
	}
	if reg.Entries[0].LastUsedAtMs != 3000 {
		t.Errorf("expected lastUsedAtMs=3000, got %d", reg.Entries[0].LastUsedAtMs)
	}

	// Add second entry
	entry2 := RegistryEntry{
		ContainerName: "test-container-2",
		SessionKey:    "session-2",
		CreatedAtMs:   4000,
		LastUsedAtMs:  5000,
		Image:         "test-image:v2",
	}
	if err := UpdateRegistryEntry(regPath, entry2); err != nil {
		t.Fatalf("UpdateRegistryEntry (add) failed: %v", err)
	}
	reg, _ = ReadRegistry(regPath)
	if len(reg.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(reg.Entries))
	}

	// Remove
	if err := RemoveRegistryEntry(regPath, "test-container-1"); err != nil {
		t.Fatalf("RemoveRegistryEntry failed: %v", err)
	}
	reg, _ = ReadRegistry(regPath)
	if len(reg.Entries) != 1 {
		t.Fatalf("expected 1 entry after remove, got %d", len(reg.Entries))
	}
	if reg.Entries[0].ContainerName != "test-container-2" {
		t.Error("expected remaining entry to be test-container-2")
	}
}

func TestRegistryReadNonExistent(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, "nonexistent", RegistryFilename)

	reg, err := ReadRegistry(regPath)
	if err != nil {
		t.Fatalf("ReadRegistry on non-existent should return empty: %v", err)
	}
	if len(reg.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(reg.Entries))
	}
}

func TestRegistryCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, RegistryFilename)

	// Write corrupted data
	if err := os.WriteFile(regPath, []byte("not json{{{"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Should return empty registry instead of error
	reg, err := ReadRegistry(regPath)
	if err != nil {
		t.Fatalf("corrupted file should return empty registry, got error: %v", err)
	}
	if len(reg.Entries) != 0 {
		t.Error("corrupted file should yield empty entries")
	}
}

// ---------- Browser Registry ----------

func TestBrowserRegistryReadWrite(t *testing.T) {
	dir := t.TempDir()
	regPath := filepath.Join(dir, BrowserRegistryFilename)

	entry := BrowserRegistryEntry{
		ContainerName: "browser-1",
		SessionKey:    "session-1",
		CreatedAtMs:   1000,
		LastUsedAtMs:  2000,
		Image:         "browser-image:v1",
		CDPPort:       9222,
	}
	if err := UpdateBrowserRegistryEntry(regPath, entry); err != nil {
		t.Fatalf("UpdateBrowserRegistryEntry failed: %v", err)
	}

	reg, err := ReadBrowserRegistry(regPath)
	if err != nil {
		t.Fatalf("ReadBrowserRegistry failed: %v", err)
	}
	if len(reg.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(reg.Entries))
	}
	if reg.Entries[0].CDPPort != 9222 {
		t.Errorf("expected CDP port 9222, got %d", reg.Entries[0].CDPPort)
	}

	// Remove
	if err := RemoveBrowserRegistryEntry(regPath, "browser-1"); err != nil {
		t.Fatal(err)
	}
	reg, _ = ReadBrowserRegistry(regPath)
	if len(reg.Entries) != 0 {
		t.Error("expected 0 entries after remove")
	}
}

// ---------- Type JSON Serialization ----------

func TestSandboxConfigJSON(t *testing.T) {
	cfg := SandboxConfig{
		Enabled:   true,
		Scope:     ScopeSession,
		Workspace: AccessReadWrite,
		Docker: SandboxDockerConfig{
			Image:    "test:v1",
			MemoryMB: 512,
			CPUs:     1.5,
			Env:      map[string]string{"K": "V"},
		},
		Tools: SandboxToolPolicy{
			Allow: []string{"Read", "Write"},
			Deny:  []string{"Computer"},
		},
		Prune: SandboxPruneConfig{
			IdleHours:  12,
			MaxAgeDays: 3,
		},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var decoded SandboxConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Scope != ScopeSession {
		t.Errorf("expected scope=%q, got %q", ScopeSession, decoded.Scope)
	}
	if decoded.Docker.MemoryMB != 512 {
		t.Errorf("expected memoryMb=512, got %d", decoded.Docker.MemoryMB)
	}
	if len(decoded.Tools.Allow) != 2 {
		t.Errorf("expected 2 allow rules, got %d", len(decoded.Tools.Allow))
	}
	if decoded.Docker.Env["K"] != "V" {
		t.Error("expected env K=V")
	}
}

// ---------- extractPortFromDockerOutput ----------

func TestExtractPortFromDockerOutput(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"0.0.0.0:32768\n", "32768"},
		{"0.0.0.0:9222\n[::]:9222\n", "9222"},
		{"", ""},
		{"no-port-here", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractPortFromDockerOutput(tt.input)
			if got != tt.want {
				t.Errorf("extractPortFromDockerOutput(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------- boolToFlag ----------

func TestBoolToFlag(t *testing.T) {
	if boolToFlag(true) != "1" {
		t.Error("boolToFlag(true) should return \"1\"")
	}
	if boolToFlag(false) != "0" {
		t.Error("boolToFlag(false) should return \"0\"")
	}
}
