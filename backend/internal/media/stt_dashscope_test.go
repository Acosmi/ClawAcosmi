package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/openacosmi/claw-acismi/pkg/types"
)

// TestDashScopeSTT_Name 验证 Provider 名称
func TestDashScopeSTT_Name(t *testing.T) {
	stt := NewDashScopeSTT(&types.STTConfig{
		Provider: "qwen",
		APIKey:   "test-key",
	})
	if stt.Name() != "dashscope" {
		t.Errorf("expected name 'dashscope', got %q", stt.Name())
	}
}

// TestDashScopeSTT_Transcribe_Success 模拟正常转录流程
func TestDashScopeSTT_Transcribe_Success(t *testing.T) {
	taskID := "task_test_123"
	transcriptionJSON := `{"transcripts":[{"channel_id":0,"text":"你好世界"}]}`
	pollCount := 0

	// Mock 转录结果下载服务器
	resultSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(transcriptionJSON))
	}))
	defer resultSrv.Close()

	// Mock DashScope API 服务器
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证 Authorization header
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		switch {
		case strings.HasSuffix(r.URL.Path, "/services/audio/asr/transcription"):
			// 提交任务
			json.NewEncoder(w).Encode(map[string]interface{}{
				"request_id": "req_001",
				"output": map[string]interface{}{
					"task_id":     taskID,
					"task_status": "PENDING",
				},
			})
		case strings.Contains(r.URL.Path, "/tasks/"):
			// 轮询任务
			pollCount++
			status := "RUNNING"
			var results []map[string]interface{}
			if pollCount >= 2 {
				status = "SUCCEEDED"
				results = []map[string]interface{}{
					{
						"file_url":          "data:audio/webm;base64,...",
						"transcription_url": resultSrv.URL + "/result.json",
					},
				}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"request_id": "req_002",
				"output": map[string]interface{}{
					"task_id":     taskID,
					"task_status": status,
					"results":     results,
				},
			})
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	stt := NewDashScopeSTT(&types.STTConfig{
		Provider: "qwen",
		APIKey:   "test-key-123",
		BaseURL:  srv.URL,
		Model:    "sensevoice-v1",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	text, err := stt.Transcribe(ctx, []byte("fake-audio-data"), "audio/webm")
	if err != nil {
		t.Fatalf("Transcribe failed: %v", err)
	}
	if text != "你好世界" {
		t.Errorf("expected '你好世界', got %q", text)
	}
	if pollCount < 2 {
		t.Errorf("expected at least 2 poll attempts, got %d", pollCount)
	}
}

// TestDashScopeSTT_Transcribe_EmptyAudio 空音频应报错
func TestDashScopeSTT_Transcribe_EmptyAudio(t *testing.T) {
	stt := NewDashScopeSTT(&types.STTConfig{
		Provider: "qwen",
		APIKey:   "test-key",
	})
	_, err := stt.Transcribe(context.Background(), nil, "audio/webm")
	if err == nil {
		t.Error("expected error for empty audio")
	}
}

// TestDashScopeSTT_Transcribe_NoAPIKey 无 API Key 应报错
func TestDashScopeSTT_Transcribe_NoAPIKey(t *testing.T) {
	stt := NewDashScopeSTT(&types.STTConfig{
		Provider: "qwen",
	})
	_, err := stt.Transcribe(context.Background(), []byte("data"), "audio/webm")
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

// TestDashScopeSTT_Transcribe_TaskFailed 任务失败返回错误
func TestDashScopeSTT_Transcribe_TaskFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/services/audio/asr/transcription"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"request_id": "req_001",
				"output": map[string]interface{}{
					"task_id":     "task_fail",
					"task_status": "PENDING",
				},
			})
		case strings.Contains(r.URL.Path, "/tasks/"):
			json.NewEncoder(w).Encode(map[string]interface{}{
				"request_id": "req_002",
				"output": map[string]interface{}{
					"task_id":     "task_fail",
					"task_status": "FAILED",
				},
				"message": "audio format not supported",
			})
		}
	}))
	defer srv.Close()

	stt := NewDashScopeSTT(&types.STTConfig{
		Provider: "qwen",
		APIKey:   "test-key",
		BaseURL:  srv.URL,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := stt.Transcribe(ctx, []byte("data"), "audio/webm")
	if err == nil {
		t.Error("expected error for failed task")
	}
	if !strings.Contains(err.Error(), "audio format not supported") {
		t.Errorf("expected error message to contain 'audio format not supported', got: %v", err)
	}
}

// TestDashScopeSTT_TestConnection 测试连接
func TestDashScopeSTT_TestConnection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer good-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models":[]}`))
	}))
	defer srv.Close()

	t.Run("valid key", func(t *testing.T) {
		stt := NewDashScopeSTT(&types.STTConfig{
			Provider: "qwen",
			APIKey:   "good-key",
			BaseURL:  srv.URL,
		})
		if err := stt.TestConnection(context.Background()); err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		stt := NewDashScopeSTT(&types.STTConfig{
			Provider: "qwen",
			APIKey:   "bad-key",
			BaseURL:  srv.URL,
		})
		err := stt.TestConnection(context.Background())
		if err == nil {
			t.Error("expected error for invalid key")
		}
	})

	t.Run("no key", func(t *testing.T) {
		stt := NewDashScopeSTT(&types.STTConfig{Provider: "qwen"})
		err := stt.TestConnection(context.Background())
		if err == nil {
			t.Error("expected error for no key")
		}
	})
}

// TestNewSTTProvider_Qwen 验证工厂方法路由到 DashScopeSTT
func TestNewSTTProvider_Qwen(t *testing.T) {
	provider, err := NewSTTProvider(&types.STTConfig{
		Provider: "qwen",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("NewSTTProvider failed: %v", err)
	}
	if provider.Name() != "dashscope" {
		t.Errorf("expected provider name 'dashscope', got %q", provider.Name())
	}
}
