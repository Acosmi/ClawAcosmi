package feishu

// resource.go — 飞书消息资源下载（Phase B）
// API: GET /open-apis/im/v1/messages/{message_id}/resources/{file_key}?type={image|file}
// 需要权限: im:resource
// 限制: 文件大小 ≤ 100MB

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

// ResourceType 资源类型常量
const (
	ResourceTypeImage = "image" // 图片（消息图片 + 富文本图片）
	ResourceTypeFile  = "file"  // 文件（普通文件 + 音频 + 视频）
)

// DownloadResource 通过飞书 API 下载消息资源。
func (c *FeishuClient) DownloadResource(ctx context.Context, messageID, fileKey, resourceType string) ([]byte, error) {
	if messageID == "" || fileKey == "" {
		return nil, fmt.Errorf("feishu: messageID and fileKey required")
	}
	if resourceType == "" {
		resourceType = ResourceTypeFile
	}

	token, err := c.getTenantAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("feishu: get token: %w", err)
	}

	baseURL := "https://open.feishu.cn"
	if IsLarkDomain(c.Domain) {
		baseURL = "https://open.larksuite.com"
	}
	url := fmt.Sprintf("%s/open-apis/im/v1/messages/%s/resources/%s?type=%s",
		baseURL, messageID, fileKey, resourceType)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("feishu: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("feishu: download resource: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("feishu: download failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	const maxResourceDownloadSize = 50 * 1024 * 1024 // 50 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResourceDownloadSize+1))
	if err != nil {
		return nil, fmt.Errorf("feishu: read resource body: %w", err)
	}
	if int64(len(data)) > maxResourceDownloadSize {
		return nil, fmt.Errorf("feishu: resource too large (>50 MB)")
	}

	slog.Info("feishu: resource downloaded",
		"message_id", messageID,
		"file_key", fileKey,
		"type", resourceType,
		"size", len(data),
	)
	return data, nil
}

// DownloadImage 下载图片资源（便捷方法）
func (c *FeishuClient) DownloadImage(ctx context.Context, messageID, imageKey string) ([]byte, error) {
	return c.DownloadResource(ctx, messageID, imageKey, ResourceTypeImage)
}

// DownloadFile 下载文件/音频/视频资源（便捷方法）
func (c *FeishuClient) DownloadFile(ctx context.Context, messageID, fileKey string) ([]byte, error) {
	return c.DownloadResource(ctx, messageID, fileKey, ResourceTypeFile)
}

// ---------- 上传 ----------

// UploadImage 上传图片到飞书，返回 image_key。
// API: POST /open-apis/im/v1/images (multipart/form-data)
// 限制: 10 MB, 支持 JPEG/PNG/WEBP/GIF/TIFF/BMP/ICO
func (c *FeishuClient) UploadImage(ctx context.Context, imageData []byte, imageType string) (string, error) {
	if len(imageData) == 0 {
		return "", fmt.Errorf("feishu: imageData is empty")
	}
	const maxImageSize = 10 * 1024 * 1024
	if len(imageData) > maxImageSize {
		return "", fmt.Errorf("feishu: image too large (%d bytes, max 10 MB)", len(imageData))
	}
	if imageType == "" {
		imageType = "message"
	}

	token, err := c.getTenantAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("feishu: get token: %w", err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("image_type", imageType); err != nil {
		return "", fmt.Errorf("feishu: write image_type field: %w", err)
	}
	part, err := w.CreateFormFile("image", "upload.png")
	if err != nil {
		return "", fmt.Errorf("feishu: create form file: %w", err)
	}
	if _, err := part.Write(imageData); err != nil {
		return "", fmt.Errorf("feishu: write image data: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("feishu: close multipart: %w", err)
	}

	baseURL := c.apiBaseURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/open-apis/im/v1/images", &buf)
	if err != nil {
		return "", fmt.Errorf("feishu: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("feishu: upload image: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ImageKey string `json:"image_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("feishu: parse upload image response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("feishu: upload image failed: code=%d msg=%s", result.Code, result.Msg)
	}

	slog.Info("feishu: image uploaded", "image_key", result.Data.ImageKey, "size", len(imageData))
	return result.Data.ImageKey, nil
}

// UploadFile 上传文件到飞书，返回 file_key。
// API: POST /open-apis/im/v1/files (multipart/form-data)
// 限制: 30 MB
// file_type: "opus" | "mp4" | "pdf" | "doc" | "xls" | "ppt" | "stream"
func (c *FeishuClient) UploadFile(ctx context.Context, fileData []byte, fileName, fileType string, durationMs int) (string, error) {
	if len(fileData) == 0 {
		return "", fmt.Errorf("feishu: fileData is empty")
	}
	const maxFileSize = 30 * 1024 * 1024
	if len(fileData) > maxFileSize {
		return "", fmt.Errorf("feishu: file too large (%d bytes, max 30 MB)", len(fileData))
	}
	if fileType == "" {
		fileType = FeishuFileType("", fileName)
	}
	if fileName == "" {
		fileName = "upload"
	}

	token, err := c.getTenantAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("feishu: get token: %w", err)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("file_type", fileType); err != nil {
		return "", fmt.Errorf("feishu: write file_type field: %w", err)
	}
	if err := w.WriteField("file_name", fileName); err != nil {
		return "", fmt.Errorf("feishu: write file_name field: %w", err)
	}
	if durationMs > 0 {
		if err := w.WriteField("duration", strconv.Itoa(durationMs)); err != nil {
			return "", fmt.Errorf("feishu: write duration field: %w", err)
		}
	}
	part, err := w.CreateFormFile("file", fileName)
	if err != nil {
		return "", fmt.Errorf("feishu: create form file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return "", fmt.Errorf("feishu: write file data: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("feishu: close multipart: %w", err)
	}

	baseURL := c.apiBaseURL()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/open-apis/im/v1/files", &buf)
	if err != nil {
		return "", fmt.Errorf("feishu: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("feishu: upload file: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			FileKey string `json:"file_key"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("feishu: parse upload file response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("feishu: upload file failed: code=%d msg=%s", result.Code, result.Msg)
	}

	slog.Info("feishu: file uploaded", "file_key", result.Data.FileKey, "name", fileName, "size", len(fileData))
	return result.Data.FileKey, nil
}

// FeishuFileType 根据 MIME 类型或文件名推断飞书 file_type。
func FeishuFileType(mimeType, fileName string) string {
	// 优先 MIME
	if strings.HasPrefix(mimeType, "audio/") {
		return "opus"
	}
	if strings.HasPrefix(mimeType, "video/") {
		return "mp4"
	}
	if mimeType == "application/pdf" {
		return "pdf"
	}

	// 文件名推断
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".opus", ".ogg", ".mp3", ".wav", ".m4a", ".flac":
		return "opus"
	case ".mp4", ".avi", ".mov", ".webm":
		return "mp4"
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "doc"
	case ".xls", ".xlsx":
		return "xls"
	case ".ppt", ".pptx":
		return "ppt"
	default:
		return "stream"
	}
}

// apiBaseURL 返回飞书 API 基础 URL。
func (c *FeishuClient) apiBaseURL() string {
	if IsLarkDomain(c.Domain) {
		return "https://open.larksuite.com"
	}
	return "https://open.feishu.cn"
}

// getTenantAccessToken 获取 tenant_access_token。
// 使用应用凭证获取租户级别令牌。
func (c *FeishuClient) getTenantAccessToken(ctx context.Context) (string, error) {
	baseURL := "https://open.feishu.cn"
	if IsLarkDomain(c.Domain) {
		baseURL = "https://open.larksuite.com"
	}

	tokenReqBody, _ := json.Marshal(map[string]string{"app_id": c.AppID, "app_secret": c.AppSecret})
	body := string(tokenReqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/open-apis/auth/v3/tenant_access_token/internal",
		strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}

	var tr struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(respBody, &tr); err != nil {
		return "", fmt.Errorf("feishu: parse token response: %w", err)
	}
	if tr.Code != 0 {
		return "", fmt.Errorf("feishu: get token failed: code=%d msg=%s", tr.Code, tr.Msg)
	}
	return tr.TenantAccessToken, nil
}
