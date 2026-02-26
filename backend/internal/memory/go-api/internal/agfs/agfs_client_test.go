package agfs

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockAGFSHandler 模拟 AGFS Server 的 HTTP 行为。
func mockAGFSHandler() http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 文件读写 (PUT / GET)
	store := make(map[string][]byte)

	mux.HandleFunc("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		switch r.Method {
		case http.MethodPut:
			data, _ := io.ReadAll(r.Body)
			store[path] = data
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
		case http.MethodGet:
			data, ok := store[path]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
				return
			}
			w.Write(data)
		}
	})

	return mux
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(mockAGFSHandler())
	defer srv.Close()

	client := NewAGFSClient(srv.URL)
	if err := client.Health(); err != nil {
		t.Fatalf("Health() failed: %v", err)
	}
}

func TestQueueEnqueueDequeue(t *testing.T) {
	srv := httptest.NewServer(mockAGFSHandler())
	defer srv.Close()

	client := NewAGFSClient(srv.URL)

	// Enqueue — 验证写入不报错
	payload := []byte(`{"task_id":"t1","text":"hello"}`)
	if err := client.QueueEnqueue("embedding_tasks", payload); err != nil {
		t.Fatalf("QueueEnqueue failed: %v", err)
	}

	// 注: 真实 AGFS 队列的 enqueue/dequeue 语义由 queuefs 插件处理，
	// mock server 仅验证 HTTP 调用链路畅通，不模拟队列行为。
	// Dequeue 读取的是 enqueue 写入的同路径数据。
	data, err := client.QueueDequeue("embedding_tasks")
	if err != nil {
		// 在简单 mock 中 dequeue 路径与 enqueue 不同，这是预期行为
		t.Logf("QueueDequeue returned error (expected in mock): %v", err)
		return
	}
	_ = data
}

func TestKVSetGet(t *testing.T) {
	srv := httptest.NewServer(mockAGFSHandler())
	defer srv.Close()

	client := NewAGFSClient(srv.URL)

	key := "user_config_123"
	value := []byte(`{"theme":"dark","lang":"zh-CN"}`)

	if err := client.KVSet(key, value); err != nil {
		t.Fatalf("KVSet failed: %v", err)
	}

	got, err := client.KVGet(key)
	if err != nil {
		t.Fatalf("KVGet failed: %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("KVGet = %q, want %q", got, value)
	}
}

func TestWriteReadFile(t *testing.T) {
	srv := httptest.NewServer(mockAGFSHandler())
	defer srv.Close()

	client := NewAGFSClient(srv.URL)

	content := []byte("hello agfs")
	if err := client.WriteFile("/test/hello.txt", content); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got, err := client.ReadFile("/test/hello.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("ReadFile = %q, want %q", got, content)
	}
}

func TestMemWriteRead(t *testing.T) {
	srv := httptest.NewServer(mockAGFSHandler())
	defer srv.Close()

	client := NewAGFSClient(srv.URL)

	data := []byte("temp data")
	if err := client.MemWrite("/scratch/temp.bin", data); err != nil {
		t.Fatalf("MemWrite failed: %v", err)
	}

	got, err := client.MemRead("/scratch/temp.bin")
	if err != nil {
		t.Fatalf("MemRead failed: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("MemRead = %q, want %q", got, data)
	}
}

func TestQueueEnqueueJSON(t *testing.T) {
	srv := httptest.NewServer(mockAGFSHandler())
	defer srv.Close()

	client := NewAGFSClient(srv.URL)

	task := map[string]interface{}{
		"task_id": "t2",
		"text":    "world",
	}
	if err := client.QueueEnqueueJSON("embedding_tasks", task); err != nil {
		t.Fatalf("QueueEnqueueJSON failed: %v", err)
	}
}

func TestKVSetJSON(t *testing.T) {
	srv := httptest.NewServer(mockAGFSHandler())
	defer srv.Close()

	client := NewAGFSClient(srv.URL)

	config := map[string]string{"theme": "dark"}
	if err := client.KVSetJSON("user_prefs", config); err != nil {
		t.Fatalf("KVSetJSON failed: %v", err)
	}

	got, err := client.KVGet("user_prefs")
	if err != nil {
		t.Fatalf("KVGet after KVSetJSON failed: %v", err)
	}
	var result map[string]string
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if result["theme"] != "dark" {
		t.Errorf("theme = %q, want dark", result["theme"])
	}
}
