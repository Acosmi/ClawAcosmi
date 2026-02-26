package slack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================================================
// UploadFileV2 单元测试
// 验证 3 阶段 files.uploadV2 API 流程（httptest mock）
// ============================================================================

// setupUploadV2Server 创建模拟 Slack 3 阶段上传 API 的 httptest 服务器。
// receivedData 用于验证上传的文件内容。
func setupUploadV2Server(t *testing.T, failStep int) (*httptest.Server, *[]byte) {
	t.Helper()
	var receivedData []byte
	fileID := "F_TEST_123"
	step2Path := "/upload-target"

	mux := http.NewServeMux()

	// Step 1: files.getUploadURLExternal
	mux.HandleFunc("/api/files.getUploadURLExternal", func(w http.ResponseWriter, r *http.Request) {
		if failStep == 1 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "invalid_auth",
			})
			return
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if _, ok := body["filename"]; !ok {
			t.Error("Step 1: missing filename param")
		}
		if _, ok := body["length"]; !ok {
			t.Error("Step 1: missing length param")
		}

		// 返回上传 URL（指向同一个 httptest 服务器）
		uploadURL := "PLACEHOLDER_URL" + step2Path
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":         true,
			"upload_url": uploadURL,
			"file_id":    fileID,
		})
	})

	// Step 2: PUT 文件内容
	mux.HandleFunc(step2Path, func(w http.ResponseWriter, r *http.Request) {
		if failStep == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		data, _ := io.ReadAll(r.Body)
		receivedData = data
		w.WriteHeader(http.StatusOK)
	})

	// Step 3: files.completeUploadExternal
	mux.HandleFunc("/api/files.completeUploadExternal", func(w http.ResponseWriter, r *http.Request) {
		if failStep == 3 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": "channel_not_found",
			})
			return
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		// 验证 files 数组
		files, ok := body["files"].([]interface{})
		if !ok || len(files) == 0 {
			t.Error("Step 3: missing or empty files array")
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok": true,
		})
	})

	ts := httptest.NewServer(mux)

	return ts, &receivedData
}

// patchUploadURL 替换 getUploadURLExternal handler 中的 PLACEHOLDER_URL。
// 由于 httptest 服务器 URL 在创建后才知道，需要用 RoundTripper 包装来替换。
type uploadV2Transport struct {
	base    http.RoundTripper
	baseURL string
}

func (t *uploadV2Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// 拦截 Step 1 响应，替换 PLACEHOLDER_URL
	if strings.Contains(req.URL.Path, "getUploadURLExternal") {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		replaced := strings.ReplaceAll(string(body), "PLACEHOLDER_URL", t.baseURL)
		resp.Body = io.NopCloser(strings.NewReader(replaced))
		resp.ContentLength = int64(len(replaced))
	}
	return resp, nil
}

// createTestUploadClient 创建连接到 httptest 服务器的 SlackWebClient。
func createTestUploadClient(ts *httptest.Server) *SlackWebClient {
	client := NewSlackWebClient("xoxb-test-token")
	// 替换 httpClient 以使用自定义 transport
	client.httpClient = &http.Client{
		Transport: &uploadV2Transport{
			base:    ts.Client().Transport,
			baseURL: ts.URL,
		},
	}
	return client
}

// ---------- 成功路径 ----------

func TestUploadFileV2_Success(t *testing.T) {
	ts, receivedData := setupUploadV2Server(t, 0)
	defer ts.Close()

	// 临时覆盖 slackAPIBaseURL
	origBase := slackAPIBaseURL
	defer func() { setSlackAPIBaseURL(origBase) }()
	setSlackAPIBaseURL(ts.URL + "/api/")

	client := createTestUploadClient(ts)

	content := "hello world file content"
	err := client.UploadFileV2(context.Background(), UploadFileParams{
		Channel:        "C123",
		ThreadTs:       "1234567890.123456",
		Filename:       "test.txt",
		Content:        strings.NewReader(content),
		InitialComment: "Here is a file",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if string(*receivedData) != content {
		t.Errorf("uploaded data = %q, want %q", string(*receivedData), content)
	}
}

func TestUploadFileV2_DefaultFilename(t *testing.T) {
	ts, _ := setupUploadV2Server(t, 0)
	defer ts.Close()

	origBase := slackAPIBaseURL
	defer func() { setSlackAPIBaseURL(origBase) }()
	setSlackAPIBaseURL(ts.URL + "/api/")

	client := createTestUploadClient(ts)

	err := client.UploadFileV2(context.Background(), UploadFileParams{
		Channel:  "C123",
		Filename: "", // should default to "file"
		Content:  strings.NewReader("data"),
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// ---------- 失败路径 ----------

func TestUploadFileV2_Step1Fails(t *testing.T) {
	ts, _ := setupUploadV2Server(t, 1)
	defer ts.Close()

	origBase := slackAPIBaseURL
	defer func() { setSlackAPIBaseURL(origBase) }()
	setSlackAPIBaseURL(ts.URL + "/api/")

	client := createTestUploadClient(ts)

	err := client.UploadFileV2(context.Background(), UploadFileParams{
		Channel:  "C123",
		Filename: "test.txt",
		Content:  strings.NewReader("data"),
	})
	if err == nil {
		t.Fatal("expected error from Step 1 failure")
	}
	if !strings.Contains(err.Error(), "invalid_auth") {
		t.Errorf("expected invalid_auth error, got: %v", err)
	}
}

func TestUploadFileV2_Step2Fails(t *testing.T) {
	ts, _ := setupUploadV2Server(t, 2)
	defer ts.Close()

	origBase := slackAPIBaseURL
	defer func() { setSlackAPIBaseURL(origBase) }()
	setSlackAPIBaseURL(ts.URL + "/api/")

	client := createTestUploadClient(ts)

	err := client.UploadFileV2(context.Background(), UploadFileParams{
		Channel:  "C123",
		Filename: "test.txt",
		Content:  strings.NewReader("data"),
	})
	if err == nil {
		t.Fatal("expected error from Step 2 failure")
	}
	if !strings.Contains(err.Error(), "unexpected status 500") {
		t.Errorf("expected status 500 error, got: %v", err)
	}
}

func TestUploadFileV2_Step3Fails(t *testing.T) {
	ts, _ := setupUploadV2Server(t, 3)
	defer ts.Close()

	origBase := slackAPIBaseURL
	defer func() { setSlackAPIBaseURL(origBase) }()
	setSlackAPIBaseURL(ts.URL + "/api/")

	client := createTestUploadClient(ts)

	err := client.UploadFileV2(context.Background(), UploadFileParams{
		Channel:  "C123",
		Filename: "test.txt",
		Content:  strings.NewReader("data"),
	})
	if err == nil {
		t.Fatal("expected error from Step 3 failure")
	}
	if !strings.Contains(err.Error(), "channel_not_found") {
		t.Errorf("expected channel_not_found error, got: %v", err)
	}
}

func TestUploadFileV2_NoChannel(t *testing.T) {
	ts, _ := setupUploadV2Server(t, 0)
	defer ts.Close()

	origBase := slackAPIBaseURL
	defer func() { setSlackAPIBaseURL(origBase) }()
	setSlackAPIBaseURL(ts.URL + "/api/")

	client := createTestUploadClient(ts)

	// 不指定 Channel — 文件保持私有
	err := client.UploadFileV2(context.Background(), UploadFileParams{
		Filename: "private.txt",
		Content:  strings.NewReader("private data"),
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}
