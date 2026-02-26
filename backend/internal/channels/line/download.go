package line

// TS 对照: src/line/download.ts (120L)
// LINE 媒体下载 — 通过 LINE Messaging API 下载消息附件并写入临时文件

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const lineMediaBaseURL = "https://api-data.line.me/v2/bot/message"

// DownloadResult 媒体下载结果。
type DownloadResult struct {
	Path        string
	ContentType string
	Size        int64
}

// DownloadLineMedia 从 LINE 下载消息媒体，写入临时文件。
// TS: downloadLineMedia(messageId, channelAccessToken, maxBytes)
func DownloadLineMedia(messageID, channelAccessToken string, maxBytes int64) (*DownloadResult, error) {
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}

	url := fmt.Sprintf("%s/%s/content", lineMediaBaseURL, messageID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("line: create media request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+channelAccessToken)

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("line: media download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("line: media download HTTP %d: %s", resp.StatusCode, string(body))
	}

	// 有限读取，防止超限
	limitedReader := io.LimitReader(resp.Body, maxBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("line: reading media body: %w", err)
	}
	if int64(len(data)) > maxBytes {
		limitMB := maxBytes / (1024 * 1024)
		return nil, fmt.Errorf("line: media exceeds %dMB limit", limitMB)
	}

	// 检测内容类型
	contentType := detectLineContentType(data)
	ext := lineExtForContentType(contentType)

	// 写入临时文件
	tempDir := os.TempDir()
	fileName := fmt.Sprintf("line-media-%s-%d%s", messageID, time.Now().UnixMilli(), ext)
	filePath := filepath.Join(tempDir, fileName)

	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return nil, fmt.Errorf("line: write media temp file: %w", err)
	}

	return &DownloadResult{
		Path:        filePath,
		ContentType: contentType,
		Size:        int64(len(data)),
	}, nil
}

// detectLineContentType 通过魔术字节检测内容类型。
// TS: detectContentType()
func detectLineContentType(data []byte) string {
	if len(data) < 4 {
		return "application/octet-stream"
	}

	// JPEG: FF D8
	if data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}
	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}
	// GIF: 47 49 46
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}
	// WebP: RIFF....WEBP
	if len(data) >= 12 &&
		data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}
	// MP4: offset 4 = ftyp
	if len(data) >= 8 && data[4] == 0x66 && data[5] == 0x74 && data[6] == 0x79 && data[7] == 0x70 {
		return "video/mp4"
	}
	// M4A/AAC: 00 00 00 xx ftyp
	if data[0] == 0x00 && data[1] == 0x00 && data[2] == 0x00 &&
		len(data) >= 8 && data[4] == 0x66 && data[5] == 0x74 && data[6] == 0x79 && data[7] == 0x70 {
		return "audio/mp4"
	}

	return "application/octet-stream"
}

// lineExtForContentType 根据内容类型返回文件扩展名。
// TS: getExtensionForContentType()
func lineExtForContentType(ct string) string {
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "audio/mp4":
		return ".m4a"
	case "audio/mpeg":
		return ".mp3"
	default:
		return ".bin"
	}
}
