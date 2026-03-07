package runner

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/agents/llmclient"
	"github.com/Acosmi/ClawAcosmi/internal/agents/session"
)

// TestPersistToTranscript_WithAttachments 验证 persistToTranscript 将附件 blocks 写入 user 消息。
func TestPersistToTranscript_WithAttachments(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session-att"
	sessionFile := filepath.Join(tmpDir, sessionID+".jsonl")

	// 写入 header
	mgr := session.NewSessionManager("")
	if _, err := mgr.EnsureSessionFile(sessionID, sessionFile); err != nil {
		t.Fatalf("ensure session file: %v", err)
	}

	r := &EmbeddedAttemptRunner{}
	params := AttemptParams{
		SessionID:   sessionID,
		SessionFile: sessionFile,
		Prompt:      "look at this image",
		Attachments: []session.ContentBlock{
			{
				Type:     "image",
				FileName: "test.png",
				MimeType: "image/png",
				Source: &session.MediaSource{
					Type:      "base64",
					MediaType: "image/png",
					Data:      "iVBORw0KGgo=",
				},
			},
			{
				Type:     "document",
				FileName: "readme.md",
				FileSize: 256,
				MimeType: "text/markdown",
			},
		},
	}

	messages := []llmclient.ChatMessage{
		{Role: "user", Content: []llmclient.ContentBlock{{Type: "text", Text: "look at this image"}}},
		{Role: "assistant", Content: []llmclient.ContentBlock{{Type: "text", Text: "I see the image."}}},
	}

	log := slog.Default()
	r.persistToTranscript(params, messages, log)

	// 读取 transcript 验证
	entries, err := mgr.LoadSessionMessages(sessionID, sessionFile)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}

	if len(entries) < 1 {
		t.Fatalf("expected at least 1 entry, got %d", len(entries))
	}

	// 验证 user 消息
	userEntry := entries[0]
	role, _ := userEntry["role"].(string)
	if role != "user" {
		t.Fatalf("expected user role, got %q", role)
	}

	content, ok := userEntry["content"].([]interface{})
	if !ok {
		t.Fatalf("expected content array, got %T", userEntry["content"])
	}

	// 应有 3 个 blocks: text + image + document
	if len(content) != 3 {
		t.Fatalf("expected 3 content blocks (text+image+document), got %d", len(content))
	}

	// 验证第一个 block 是 text
	block0, _ := content[0].(map[string]interface{})
	if blockType, _ := block0["type"].(string); blockType != "text" {
		t.Fatalf("expected first block type=text, got %q", blockType)
	}

	// 验证第二个 block 是 image
	block1, _ := content[1].(map[string]interface{})
	if blockType, _ := block1["type"].(string); blockType != "image" {
		t.Fatalf("expected second block type=image, got %q", blockType)
	}
	if fn, _ := block1["fileName"].(string); fn != "test.png" {
		t.Fatalf("expected fileName=test.png, got %q", fn)
	}

	// 验证第三个 block 是 document
	block2, _ := content[2].(map[string]interface{})
	if blockType, _ := block2["type"].(string); blockType != "document" {
		t.Fatalf("expected third block type=document, got %q", blockType)
	}

	// 清理
	os.Remove(sessionFile)
}

// TestPersistToTranscript_NoAttachments 验证无附件时行为不变。
func TestPersistToTranscript_NoAttachments(t *testing.T) {
	tmpDir := t.TempDir()
	sessionID := "test-session-noatt"
	sessionFile := filepath.Join(tmpDir, sessionID+".jsonl")

	mgr := session.NewSessionManager("")
	if _, err := mgr.EnsureSessionFile(sessionID, sessionFile); err != nil {
		t.Fatalf("ensure session file: %v", err)
	}

	r := &EmbeddedAttemptRunner{}
	params := AttemptParams{
		SessionID:   sessionID,
		SessionFile: sessionFile,
		Prompt:      "hello",
	}

	messages := []llmclient.ChatMessage{
		{Role: "user", Content: []llmclient.ContentBlock{{Type: "text", Text: "hello"}}},
		{Role: "assistant", Content: []llmclient.ContentBlock{{Type: "text", Text: "hi"}}},
	}

	log := slog.Default()
	r.persistToTranscript(params, messages, log)

	entries, err := mgr.LoadSessionMessages(sessionID, sessionFile)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}

	if len(entries) < 1 {
		t.Fatalf("expected at least 1 entry, got %d", len(entries))
	}

	userEntry := entries[0]
	content, _ := userEntry["content"].([]interface{})
	// 仅 1 个 text block
	if len(content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(content))
	}
}
