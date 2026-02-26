package media

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestDownloadAndSave_WithHeaders 验证自定义 Headers 被正确传递到远程服务器。
// MEDIA-3: 对齐 TS store.ts downloadToFile 的 headers 透传能力。
func TestDownloadAndSave_WithHeaders(t *testing.T) {
	receivedHeaders := make(http.Header)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 捕获所有收到的请求头
		for k, v := range r.Header {
			receivedHeaders[k] = v
		}
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(200)
		// 写入最小有效 PNG (8 bytes)
		w.Write([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	}))
	defer server.Close()

	dir := t.TempDir()
	headers := map[string]string{
		"Authorization": "Bearer test-token-123",
		"X-Custom-Auth": "my-api-key",
	}

	result, err := downloadAndSave(server.URL+"/test-image.png", dir, "test-uuid", headers)
	if err != nil {
		t.Fatalf("downloadAndSave failed: %v", err)
	}

	// 验证 Headers 被正确传递
	if got := receivedHeaders.Get("Authorization"); got != "Bearer test-token-123" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token-123")
	}
	if got := receivedHeaders.Get("X-Custom-Auth"); got != "my-api-key" {
		t.Errorf("X-Custom-Auth header = %q, want %q", got, "my-api-key")
	}

	// 验证文件已保存
	if result.Size != 8 {
		t.Errorf("saved file size = %d, want 8", result.Size)
	}

	// 清理
	os.Remove(result.Path)
}

// TestDownloadAndSave_WithoutHeaders 验证 nil headers 时正常工作（向后兼容）。
func TestDownloadAndSave_WithoutHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		io.WriteString(w, "hello world")
	}))
	defer server.Close()

	dir := t.TempDir()
	result, err := downloadAndSave(server.URL+"/test.txt", dir, "test-uuid2", nil)
	if err != nil {
		t.Fatalf("downloadAndSave with nil headers failed: %v", err)
	}

	if result.Size != 11 {
		t.Errorf("saved file size = %d, want 11", result.Size)
	}

	// 清理
	os.Remove(result.Path)
}

// TestSaveMediaSourceWithHeaders_LocalFile 验证本地文件不受 headers 影响。
func TestSaveMediaSourceWithHeaders_LocalFile(t *testing.T) {
	// 创建临时源文件
	tmpFile, err := os.CreateTemp(t.TempDir(), "media-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.WriteString("test content")
	tmpFile.Close()

	// 设置临时 media dir
	t.Setenv("OPENACOSMI_CONFIG_DIR", t.TempDir())

	result, err := SaveMediaSourceWithHeaders(tmpFile.Name(), map[string]string{
		"Authorization": "should-be-ignored",
	})
	if err != nil {
		t.Fatalf("SaveMediaSourceWithHeaders for local file failed: %v", err)
	}

	if result.Size != 12 {
		t.Errorf("saved file size = %d, want 12", result.Size)
	}

	// 清理
	os.Remove(result.Path)
}
