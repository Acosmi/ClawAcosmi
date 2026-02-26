package agfs

import (
	"os"
	"testing"
)

// 集成测试需要运行中的 AGFS Server。
// 设置环境变量 AGFS_URL 指向 AGFS Server 地址后运行:
//   AGFS_URL=http://localhost:8090 go test ./internal/agfs/ -v -run TestAGFSIntegration

func getAGFSURL(t *testing.T) string {
	url := os.Getenv("AGFS_URL")
	if url == "" {
		t.Skip("AGFS_URL not set, skipping integration test")
	}
	return url
}

func TestAGFSIntegration_Health(t *testing.T) {
	client := NewAGFSClient(getAGFSURL(t))
	if err := client.Health(); err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	t.Log("✅ AGFS Server health check passed")
}

func TestAGFSIntegration_QueueRoundtrip(t *testing.T) {
	client := NewAGFSClient(getAGFSURL(t))

	queueName := "integration_test_queue"
	payload := []byte(`{"test":"queue_roundtrip"}`)

	// Enqueue
	if err := client.QueueEnqueue(queueName, payload); err != nil {
		t.Fatalf("QueueEnqueue failed: %v", err)
	}
	t.Log("✅ Queue enqueue succeeded")

	// Dequeue
	data, err := client.QueueDequeue(queueName)
	if err != nil {
		t.Fatalf("QueueDequeue failed: %v", err)
	}
	if string(data) != string(payload) {
		t.Errorf("QueueDequeue = %q, want %q", data, payload)
	}
	t.Log("✅ Queue dequeue roundtrip succeeded")
}

func TestAGFSIntegration_KVRoundtrip(t *testing.T) {
	client := NewAGFSClient(getAGFSURL(t))

	key := "integration_test_key"
	value := []byte(`{"test":"kv_roundtrip"}`)

	// Set
	if err := client.KVSet(key, value); err != nil {
		t.Fatalf("KVSet failed: %v", err)
	}
	t.Log("✅ KV set succeeded")

	// Get
	data, err := client.KVGet(key)
	if err != nil {
		t.Fatalf("KVGet failed: %v", err)
	}
	if string(data) != string(value) {
		t.Errorf("KVGet = %q, want %q", data, value)
	}
	t.Log("✅ KV get roundtrip succeeded")
}

func TestAGFSIntegration_FileRoundtrip(t *testing.T) {
	client := NewAGFSClient(getAGFSURL(t))

	content := []byte("integration test file content")

	// Write
	if err := client.WriteFile("/integration_test.txt", content); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	t.Log("✅ File write succeeded")

	// Read
	data, err := client.ReadFile("/integration_test.txt")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("ReadFile = %q, want %q", data, content)
	}
	t.Log("✅ File read roundtrip succeeded")
}
