package media

import (
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/openacosmi/claw-acismi/internal/security"
)

// TS 对照: media/fetch.ts (220L)

// ---------- 错误类型 ----------

// MediaFetchErrorCode 媒体获取错误码。
// TS 对照: fetch.ts L12
type MediaFetchErrorCode string

const (
	ErrCodeMaxBytes    MediaFetchErrorCode = "max_bytes"
	ErrCodeHTTPError   MediaFetchErrorCode = "http_error"
	ErrCodeFetchFailed MediaFetchErrorCode = "fetch_failed"
)

// MediaFetchError 媒体获取错误。
// TS 对照: fetch.ts L14-22
type MediaFetchError struct {
	Code    MediaFetchErrorCode
	Message string
}

func (e *MediaFetchError) Error() string {
	return e.Message
}

// ---------- 获取结果 ----------

// FetchMediaResult 媒体获取结果。
// TS 对照: fetch.ts L6-10
type FetchMediaResult struct {
	Buffer      []byte
	ContentType string
	FileName    string
}

// FetchMediaOptions 媒体获取选项。
// TS 对照: fetch.ts L26-34
type FetchMediaOptions struct {
	URL          string
	FilePathHint string
	MaxBytes     int64
	MaxRedirects int
}

// ---------- 辅助函数 ----------

// stripQuotes 移除首尾引号。
// TS 对照: fetch.ts L36-38
func stripQuotes(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

// parseContentDispositionFileName 从 Content-Disposition 头解析文件名。
// TS 对照: fetch.ts L40-59
func parseContentDispositionFileName(header string) string {
	if header == "" {
		return ""
	}
	// 尝试 filename* (RFC 5987)
	parts := strings.Split(header, ";")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "filename*") {
			idx := strings.IndexByte(trimmed, '=')
			if idx < 0 {
				continue
			}
			cleaned := stripQuotes(strings.TrimSpace(trimmed[idx+1:]))
			// 尝试 RFC 5987 编码: charset''value
			if apostIdx := strings.Index(cleaned, "''"); apostIdx >= 0 {
				encoded := cleaned[apostIdx+2:]
				decoded, err := url.PathUnescape(encoded)
				if err == nil {
					return filepath.Base(decoded)
				}
				return filepath.Base(encoded)
			}
			return filepath.Base(cleaned)
		}
	}
	// 尝试 filename
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "filename") && !strings.HasPrefix(lower, "filename*") {
			idx := strings.IndexByte(trimmed, '=')
			if idx < 0 {
				continue
			}
			return filepath.Base(stripQuotes(strings.TrimSpace(trimmed[idx+1:])))
		}
	}
	return ""
}

// ---------- 公开函数 ----------

// FetchRemoteMedia 从远程 URL 获取媒体。
// 集成 SSRF 防护（P7B-3）。
// TS 对照: fetch.ts L80-171
func FetchRemoteMedia(opts FetchMediaOptions) (*FetchMediaResult, error) {
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = MediaMaxBytes
	}

	resp, err := security.SafeFetchURL(opts.URL, nil)
	if err != nil {
		return nil, &MediaFetchError{
			Code:    ErrCodeFetchFailed,
			Message: fmt.Sprintf("获取媒体失败 %s: %v", opts.URL, err),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, &MediaFetchError{
			Code:    ErrCodeHTTPError,
			Message: fmt.Sprintf("获取媒体 HTTP %d: %s", resp.StatusCode, opts.URL),
		}
	}

	// 检查 Content-Length
	if resp.ContentLength > 0 && resp.ContentLength > opts.MaxBytes {
		return nil, &MediaFetchError{
			Code:    ErrCodeMaxBytes,
			Message: fmt.Sprintf("媒体大小 %d 超过限制 %d: %s", resp.ContentLength, opts.MaxBytes, opts.URL),
		}
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBytes+1))
	if err != nil {
		return nil, &MediaFetchError{
			Code:    ErrCodeFetchFailed,
			Message: fmt.Sprintf("读取媒体数据失败: %v", err),
		}
	}
	if int64(len(data)) > opts.MaxBytes {
		return nil, &MediaFetchError{
			Code:    ErrCodeMaxBytes,
			Message: fmt.Sprintf("媒体超过 %d 字节限制: %s", opts.MaxBytes, opts.URL),
		}
	}

	// 解析文件名
	fileNameFromURL := ""
	if u, err := url.Parse(resp.Request.URL.String()); err == nil {
		base := filepath.Base(u.Path)
		if base != "" && base != "." && base != "/" {
			fileNameFromURL = base
		}
	}
	headerFileName := parseContentDispositionFileName(resp.Header.Get("Content-Disposition"))
	fileName := headerFileName
	if fileName == "" {
		fileName = fileNameFromURL
	}
	if fileName == "" && opts.FilePathHint != "" {
		fileName = filepath.Base(opts.FilePathHint)
	}

	// 检测 MIME
	filePathForMime := opts.URL
	if headerFileName != "" && filepath.Ext(headerFileName) != "" {
		filePathForMime = headerFileName
	} else if opts.FilePathHint != "" {
		filePathForMime = opts.FilePathHint
	}
	contentType := DetectMime(DetectMimeOpts{
		Buffer:     data,
		HeaderMime: resp.Header.Get("Content-Type"),
		FilePath:   filePathForMime,
	})

	// 补充扩展名
	if fileName != "" && filepath.Ext(fileName) == "" && contentType != "" {
		ext := ExtensionForMime(contentType)
		if ext != "" {
			fileName += ext
		}
	}

	return &FetchMediaResult{
		Buffer:      data,
		ContentType: contentType,
		FileName:    fileName,
	}, nil
}
