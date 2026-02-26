package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStripFrontMatter(t *testing.T) {
	content := "---\ntitle: test\n---\nBody content"
	result := StripFrontMatter(content)
	if result != "Body content" {
		t.Errorf("got %q, want 'Body content'", result)
	}

	// No front matter
	if StripFrontMatter("plain text") != "plain text" {
		t.Error("plain text should pass through")
	}
}

func TestLoadWorkspaceBootstrapFiles(t *testing.T) {
	dir := t.TempDir()
	// Create AGENTS.md
	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("---\ntype: agents\n---\nAgent instructions"), 0644)

	files := LoadWorkspaceBootstrapFiles(dir)
	if len(files) == 0 {
		t.Fatal("should return files")
	}

	var found bool
	for _, f := range files {
		if f.Name == BootstrapAgents {
			found = true
			if f.Missing {
				t.Error("AGENTS.md should not be missing")
			}
			if f.Content != "Agent instructions" {
				t.Errorf("content = %q", f.Content)
			}
		}
	}
	if !found {
		t.Error("AGENTS.md not found in bootstrap files")
	}
}

func TestFilterBootstrapFilesForSession(t *testing.T) {
	files := []WorkspaceBootstrapFile{
		{Name: BootstrapAgents},
		{Name: BootstrapSoul},
		{Name: BootstrapTools},
	}

	// Non-subagent: all files
	result := FilterBootstrapFilesForSession(files, "session-123")
	if len(result) != 3 {
		t.Errorf("non-subagent got %d, want 3", len(result))
	}

	// Subagent: only AGENTS + TOOLS
	result = FilterBootstrapFilesForSession(files, "session:subagent:123")
	if len(result) != 2 {
		t.Errorf("subagent got %d, want 2", len(result))
	}
}

func TestResolveDocsPath(t *testing.T) {
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	os.MkdirAll(docsDir, 0755)

	result := ResolveDocsPath(dir)
	if result != docsDir {
		t.Errorf("docs path = %q, want %q", result, docsDir)
	}

	// Missing docs
	result = ResolveDocsPath("/nonexistent/path")
	if result != "" {
		t.Errorf("missing docs = %q, want empty", result)
	}
}

func TestEnsureAgentWorkspace_DirOnly(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "myagent", "workspace")

	result, err := EnsureAgentWorkspace(EnsureAgentWorkspaceParams{
		Dir:                  wsDir,
		EnsureBootstrapFiles: false,
	})
	if err != nil {
		t.Fatalf("EnsureAgentWorkspace: %v", err)
	}
	if result.Dir != wsDir {
		t.Errorf("dir = %q, want %q", result.Dir, wsDir)
	}
	// 目录应已创建
	if _, err := os.Stat(wsDir); err != nil {
		t.Errorf("workspace dir not created: %v", err)
	}
	// 不应有引导文件路径
	if result.AgentsPath != "" {
		t.Errorf("agentsPath should be empty, got %q", result.AgentsPath)
	}
}

func TestEnsureAgentWorkspace_WithBootstrapFiles(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "ws")

	// 创建模板目录
	tmplDir := filepath.Join(dir, "templates")
	os.MkdirAll(tmplDir, 0755)
	os.WriteFile(filepath.Join(tmplDir, "AGENTS.md"), []byte("---\ntype: agents\n---\nAgent template"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "SOUL.md"), []byte("Soul template"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "TOOLS.md"), []byte("Tools template"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "IDENTITY.md"), []byte("Identity template"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "USER.md"), []byte("User template"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "HEARTBEAT.md"), []byte("Heartbeat template"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "BOOTSTRAP.md"), []byte("Bootstrap template"), 0644)

	// 覆盖模板目录
	ResetTemplateDirCache()
	SetTemplateDir(tmplDir)
	defer ResetTemplateDirCache()

	result, err := EnsureAgentWorkspace(EnsureAgentWorkspaceParams{
		Dir:                  wsDir,
		EnsureBootstrapFiles: true,
	})
	if err != nil {
		t.Fatalf("EnsureAgentWorkspace: %v", err)
	}

	// 检查引导文件
	if result.AgentsPath == "" {
		t.Error("agentsPath should be set")
	}
	content, err := os.ReadFile(result.AgentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	// AGENTS.md 有 front matter，应被去除
	if string(content) != "Agent template" {
		t.Errorf("AGENTS.md content = %q, want 'Agent template'", string(content))
	}

	// BOOTSTRAP.md 应被写入（全新工作区）
	bsContent, err := os.ReadFile(result.BootstrapPath)
	if err != nil {
		t.Fatalf("read BOOTSTRAP.md: %v", err)
	}
	if string(bsContent) != "Bootstrap template" {
		t.Errorf("BOOTSTRAP.md content = %q", string(bsContent))
	}
}

func TestEnsureAgentWorkspace_ExistingFiles(t *testing.T) {
	dir := t.TempDir()
	wsDir := filepath.Join(dir, "ws")
	os.MkdirAll(wsDir, 0755)

	// 预先创建 AGENTS.md
	existingContent := "My existing agents"
	os.WriteFile(filepath.Join(wsDir, "AGENTS.md"), []byte(existingContent), 0644)

	// 创建模板目录
	tmplDir := filepath.Join(dir, "templates")
	os.MkdirAll(tmplDir, 0755)
	os.WriteFile(filepath.Join(tmplDir, "AGENTS.md"), []byte("New template"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "SOUL.md"), []byte("Soul"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "TOOLS.md"), []byte("Tools"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "IDENTITY.md"), []byte("Identity"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "USER.md"), []byte("User"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "HEARTBEAT.md"), []byte("Heartbeat"), 0644)
	os.WriteFile(filepath.Join(tmplDir, "BOOTSTRAP.md"), []byte("Bootstrap"), 0644)

	ResetTemplateDirCache()
	SetTemplateDir(tmplDir)
	defer ResetTemplateDirCache()

	_, err := EnsureAgentWorkspace(EnsureAgentWorkspaceParams{
		Dir:                  wsDir,
		EnsureBootstrapFiles: true,
	})
	if err != nil {
		t.Fatalf("EnsureAgentWorkspace: %v", err)
	}

	// 已有文件不应被覆盖（O_EXCL 语义）
	content, _ := os.ReadFile(filepath.Join(wsDir, "AGENTS.md"))
	if string(content) != existingContent {
		t.Errorf("existing AGENTS.md was overwritten: got %q", string(content))
	}

	// BOOTSTRAP.md 不应写入（非全新工作区）
	if _, err := os.Stat(filepath.Join(wsDir, "BOOTSTRAP.md")); err == nil {
		t.Error("BOOTSTRAP.md should NOT be written for non-brand-new workspace")
	}
}
