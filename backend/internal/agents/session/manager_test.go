package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// ---------- TestEnsureSessionFile ----------

func TestEnsureSessionFile_CreatesNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewSessionManager(filepath.Join(tmpDir, "store.json"))

	fp, err := mgr.EnsureSessionFile("test-session-001", "")
	if err != nil {
		t.Fatalf("EnsureSessionFile 失败: %v", err)
	}
	if fp == "" {
		t.Fatal("返回路径为空")
	}

	// 验证文件存在
	data, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("读取文件失败: %v", err)
	}

	// 解析 header
	var header TranscriptHeader
	if err := json.Unmarshal(data[:len(data)-1], &header); err != nil { // -1 去掉换行符
		t.Fatalf("解析 header 失败: %v", err)
	}
	if header.Type != "session" {
		t.Errorf("header.Type = %q, want %q", header.Type, "session")
	}
	if header.ID != "test-session-001" {
		t.Errorf("header.ID = %q, want %q", header.ID, "test-session-001")
	}
}

func TestEnsureSessionFile_ExplicitPath(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewSessionManager("")

	explicit := filepath.Join(tmpDir, "custom.jsonl")
	fp, err := mgr.EnsureSessionFile("any-id", explicit)
	if err != nil {
		t.Fatalf("EnsureSessionFile 失败: %v", err)
	}
	if fp != explicit {
		t.Errorf("path = %q, want %q", fp, explicit)
	}
}

func TestEnsureSessionFile_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewSessionManager(filepath.Join(tmpDir, "store.json"))

	fp1, err := mgr.EnsureSessionFile("idempotent-session", "")
	if err != nil {
		t.Fatalf("首次调用失败: %v", err)
	}

	fp2, err := mgr.EnsureSessionFile("idempotent-session", "")
	if err != nil {
		t.Fatalf("二次调用失败: %v", err)
	}
	if fp1 != fp2 {
		t.Errorf("两次返回路径不同: %q vs %q", fp1, fp2)
	}
}

// ---------- TestAppendMessage + LoadSessionMessages ----------

func TestAppendAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "session.jsonl")
	mgr := NewSessionManager("")

	// 确保文件存在
	if _, err := mgr.EnsureSessionFile("s1", sessionFile); err != nil {
		t.Fatalf("EnsureSessionFile: %v", err)
	}

	// 追加 user 消息
	if err := mgr.AppendMessage("s1", sessionFile, TranscriptEntry{
		Role: "user",
		Content: []ContentBlock{
			{Type: "text", Text: "Hello"},
		},
	}); err != nil {
		t.Fatalf("AppendMessage user: %v", err)
	}

	// 追加 assistant 消息
	if err := mgr.AppendMessage("s1", sessionFile, TranscriptEntry{
		Role: "assistant",
		Content: []ContentBlock{
			{Type: "text", Text: "Hi there!"},
		},
		Model: "claude-3",
	}); err != nil {
		t.Fatalf("AppendMessage assistant: %v", err)
	}

	// 读取
	msgs, err := mgr.LoadSessionMessages("s1", sessionFile)
	if err != nil {
		t.Fatalf("LoadSessionMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("消息数 = %d, want 2", len(msgs))
	}

	// 验证消息 role
	if role, _ := msgs[0]["role"].(string); role != "user" {
		t.Errorf("msgs[0].role = %q, want %q", role, "user")
	}
	if role, _ := msgs[1]["role"].(string); role != "assistant" {
		t.Errorf("msgs[1].role = %q, want %q", role, "assistant")
	}
}

func TestLoadSessionMessages_FileNotFound(t *testing.T) {
	mgr := NewSessionManager("")

	msgs, err := mgr.LoadSessionMessages("nonexistent", "/tmp/does-not-exist.jsonl")
	if err != nil {
		t.Fatalf("不应返回 error: %v", err)
	}
	if msgs != nil {
		t.Errorf("文件不存在时应返回 nil, got %d messages", len(msgs))
	}
}

// ---------- TestConcurrentWrites ----------

func TestConcurrentAppend(t *testing.T) {
	tmpDir := t.TempDir()
	sessionFile := filepath.Join(tmpDir, "concurrent.jsonl")
	mgr := NewSessionManager("")

	if _, err := mgr.EnsureSessionFile("c1", sessionFile); err != nil {
		t.Fatal(err)
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			err := mgr.AppendMessage("c1", sessionFile, TranscriptEntry{
				Role: "user",
				Content: []ContentBlock{
					{Type: "text", Text: "msg"},
				},
			})
			if err != nil {
				t.Errorf("goroutine %d: %v", n, err)
			}
		}(i)
	}
	wg.Wait()

	msgs, err := mgr.LoadSessionMessages("c1", sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != goroutines {
		t.Errorf("消息数 = %d, want %d", len(msgs), goroutines)
	}
}

// ---------- TestResolveFilePath ----------

func TestResolveFilePath(t *testing.T) {
	mgr := NewSessionManager("/data/store.json")

	// 显式路径优先
	got := mgr.ResolveFilePath("sid", "/explicit/path.jsonl", "agent-1")
	if got != "/explicit/path.jsonl" {
		t.Errorf("显式路径未优先: %q", got)
	}

	// 无显式路径 → 从 storePath 推导
	got = mgr.ResolveFilePath("my-session", "", "agent-1")
	want := "/data/my-session.jsonl"
	if got != want {
		t.Errorf("推导路径 = %q, want %q", got, want)
	}

	// 无 storePath 且无显式路径 → 空
	mgr2 := NewSessionManager("")
	got = mgr2.ResolveFilePath("sid", "", "agent-1")
	if got != "" {
		t.Errorf("无 storePath 应返回空, got %q", got)
	}
}
