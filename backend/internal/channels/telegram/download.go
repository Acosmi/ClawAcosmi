package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"github.com/anthropic/open-acosmi/internal/media"
)

// Telegram 文件下载 — 继承自 src/telegram/download.ts (58L)

// TelegramFileInfo Telegram 文件信息（getFile API 返回）
type TelegramFileInfo struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
	FilePath     string `json:"file_path,omitempty"`
}

type getFileResponse struct {
	OK     bool              `json:"ok"`
	Result *TelegramFileInfo `json:"result,omitempty"`
}

// GetTelegramFile 调用 Telegram Bot API getFile 获取文件信息。
func GetTelegramFile(ctx context.Context, client *http.Client, token, fileID string) (*TelegramFileInfo, error) {
	if client == nil {
		client = http.DefaultClient
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	apiURL := fmt.Sprintf("%s/bot%s/getFile?file_id=%s",
		TelegramAPIBaseURL, token, url.QueryEscape(fileID))

	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("getFile request build failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getFile request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getFile failed: HTTP %d — %s", resp.StatusCode, string(body))
	}

	var result getFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("getFile response decode failed: %w", err)
	}
	if !result.OK || result.Result == nil || result.Result.FilePath == "" {
		return nil, fmt.Errorf("getFile returned no file_path")
	}
	return result.Result, nil
}

// DownloadedFile 下载后的文件数据
type DownloadedFile struct {
	Data        []byte
	ContentType string
	FileName    string
	FilePath    string // 原始 file_path
}

// DownloadTelegramFile 从 Telegram 服务器下载文件内容。
func DownloadTelegramFile(ctx context.Context, client *http.Client, token string, info *TelegramFileInfo, maxBytes int64) (*DownloadedFile, error) {
	if info.FilePath == "" {
		return nil, fmt.Errorf("file_path missing")
	}
	if client == nil {
		client = http.DefaultClient
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	fileURL := fmt.Sprintf("%s/file/bot%s/%s", TelegramAPIBaseURL, token, info.FilePath)
	req, err := http.NewRequestWithContext(timeoutCtx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("download request build failed: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	var reader io.Reader = resp.Body
	if maxBytes > 0 {
		// 读取 maxBytes+1 以检测超限（对齐 TS: 下载完整后检查大小并报错）
		reader = io.LimitReader(resp.Body, maxBytes+1)
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("download read failed: %w", err)
	}

	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("media file exceeds size limit (%d bytes)", maxBytes)
	}

	// 对齐 TS: 三级 MIME 检测（sniff > 扩展名 > header）
	contentType := media.DetectMime(media.DetectMimeOpts{
		Buffer:     data,
		HeaderMime: resp.Header.Get("Content-Type"),
		FilePath:   info.FilePath,
	})

	fileName := filepath.Base(info.FilePath)

	return &DownloadedFile{
		Data:        data,
		ContentType: contentType,
		FileName:    fileName,
		FilePath:    info.FilePath,
	}, nil
}
