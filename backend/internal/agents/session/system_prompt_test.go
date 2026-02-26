package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------- TestResolveContextFiles ----------

func TestResolveContextFiles_WithSOUL(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建 SOUL.md
	if err := os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte("You are a pirate assistant."), 0o644); err != nil {
		t.Fatal(err)
	}
	// 创建 TOOLS.md
	if err := os.WriteFile(filepath.Join(tmpDir, "TOOLS.md"), []byte("Available tools:\n- search\n- calculate"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := ResolveContextFiles(tmpDir)
	if len(files) != 2 {
		t.Fatalf("文件数 = %d, want 2", len(files))
	}

	// SOUL.md 优先级最高，应该排第一
	if files[0].Path != "SOUL.md" {
		t.Errorf("第一个文件 = %q, want %q", files[0].Path, "SOUL.md")
	}
	if files[1].Path != "TOOLS.md" {
		t.Errorf("第二个文件 = %q, want %q", files[1].Path, "TOOLS.md")
	}
}

func TestResolveContextFiles_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	files := ResolveContextFiles(tmpDir)
	if len(files) != 0 {
		t.Errorf("空目录应返回 0 个文件, got %d", len(files))
	}
}

func TestResolveContextFiles_EmptyDir(t *testing.T) {
	files := ResolveContextFiles("")
	if files != nil {
		t.Errorf("空路径应返回 nil, got %v", files)
	}
}

func TestResolveContextFiles_EmptyFileSkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建空 SOUL.md
	if err := os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	// 创建非空 TOOLS.md
	if err := os.WriteFile(filepath.Join(tmpDir, "TOOLS.md"), []byte("tool data"), 0o644); err != nil {
		t.Fatal(err)
	}

	files := ResolveContextFiles(tmpDir)
	if len(files) != 1 {
		t.Fatalf("应跳过空文件, got %d files", len(files))
	}
	if files[0].Path != "TOOLS.md" {
		t.Errorf("file = %q, want %q", files[0].Path, "TOOLS.md")
	}
}

// ---------- TestApplyContextPruning ----------

func TestApplyContextPruning_WithinBudget(t *testing.T) {
	files := []ContextFile{
		{Path: "SOUL.md", Content: "short", Priority: 0},
		{Path: "TOOLS.md", Content: "also short", Priority: 1},
	}

	result := ApplyContextPruning(files, 100000) // 很大的预算
	if len(result) != 2 {
		t.Fatalf("预算内应全部保留, got %d", len(result))
	}
}

func TestApplyContextPruning_ExceedsBudget(t *testing.T) {
	// 创建一个大文件
	bigContent := strings.Repeat("This is a long line of text. ", 1000)
	files := []ContextFile{
		{Path: "SOUL.md", Content: "important persona", Priority: 0},
		{Path: "BIG.md", Content: bigContent, Priority: 5},
	}

	result := ApplyContextPruning(files, 50) // 很小的预算
	// SOUL.md 应完整保留（内容很短），BIG.md 应被截断或移除
	if len(result) == 0 {
		t.Fatal("至少保留高优先级文件")
	}
	if result[0].Path != "SOUL.md" {
		t.Errorf("高优先级文件应保留: %q", result[0].Path)
	}
}

func TestApplyContextPruning_ZeroBudget(t *testing.T) {
	files := []ContextFile{
		{Path: "SOUL.md", Content: "data", Priority: 0},
	}
	result := ApplyContextPruning(files, 0)
	if len(result) != 1 {
		t.Errorf("zero budget should return all files unchanged, got %d", len(result))
	}
}

// ---------- TestEstimatePromptTokens ----------

func TestEstimatePromptTokens_English(t *testing.T) {
	text := "Hello world this is a test sentence with several words"
	tokens := EstimatePromptTokens(text)
	if tokens < 5 || tokens > 30 {
		t.Errorf("英文 token 估算异常: %d (text len=%d)", tokens, len(text))
	}
}

func TestEstimatePromptTokens_Chinese(t *testing.T) {
	text := "你好世界这是一个测试句子"
	tokens := EstimatePromptTokens(text)
	if tokens < 3 || tokens > 20 {
		t.Errorf("中文 token 估算异常: %d", tokens)
	}
}

func TestEstimatePromptTokens_Empty(t *testing.T) {
	if got := EstimatePromptTokens(""); got != 0 {
		t.Errorf("空字符串应返回 0, got %d", got)
	}
}

func TestEstimatePromptTokens_Mixed(t *testing.T) {
	text := "Hello 你好 World 世界"
	tokens := EstimatePromptTokens(text)
	if tokens < 2 {
		t.Errorf("混合文本 token 估算异常: %d", tokens)
	}
}

// ---------- TestBuildDynamicSystemPrompt ----------

func TestBuildDynamicSystemPrompt_NoContextFiles(t *testing.T) {
	params := DynamicPromptParams{
		WorkspaceDir: "/nonexistent",
	}

	result := BuildDynamicSystemPrompt(params)
	if result == "" {
		t.Error("应至少包含基础提示词")
	}
	if strings.Contains(result, "Project Context") {
		t.Error("无 context files 时不应包含 Project Context 段")
	}
}

func TestBuildDynamicSystemPrompt_WithContextFiles(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "SOUL.md"), []byte("Be a helpful pirate!"), 0o644)

	params := DynamicPromptParams{
		WorkspaceDir: tmpDir,
	}

	result := BuildDynamicSystemPrompt(params)
	if !strings.Contains(result, "Project Context") {
		t.Error("应包含 Project Context 段")
	}
	if !strings.Contains(result, "Be a helpful pirate!") {
		t.Error("应包含 SOUL.md 内容")
	}
	if !strings.Contains(result, "SOUL.md") {
		t.Error("应包含 SOUL.md 文件路径")
	}
}

func TestBuildDynamicSystemPrompt_WithPresetContextFiles(t *testing.T) {
	params := DynamicPromptParams{
		ContextFiles: []ContextFile{
			{Path: "TEST.md", Content: "Custom context", Priority: 0},
		},
	}

	result := BuildDynamicSystemPrompt(params)
	if !strings.Contains(result, "Custom context") {
		t.Error("应包含预设 context file 内容")
	}
}

func TestBuildDynamicSystemPrompt_SoulPersona(t *testing.T) {
	params := DynamicPromptParams{
		ContextFiles: []ContextFile{
			{Path: "SOUL.md", Content: "Be funny and witty", Priority: 0},
		},
	}
	result := BuildDynamicSystemPrompt(params)
	if !strings.Contains(result, "SOUL.md is present") {
		t.Error("应包含 SOUL.md persona 指导文本")
	}
}
