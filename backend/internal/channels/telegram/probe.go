package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Telegram 探测 — 继承自 src/telegram/probe.ts (115L)

// TelegramProbe Bot 连接探测结果
// 对齐 TS: status/error 使用指针类型输出 null 而非省略字段
type TelegramProbe struct {
	OK        bool    `json:"ok"`
	Status    *int    `json:"status"`
	Error     *string `json:"error"`
	ElapsedMs int64   `json:"elapsedMs"`
	Bot       *struct {
		ID                      *int64  `json:"id,omitempty"`
		Username                *string `json:"username,omitempty"`
		CanJoinGroups           *bool   `json:"canJoinGroups,omitempty"`
		CanReadAllGroupMessages *bool   `json:"canReadAllGroupMessages,omitempty"`
		SupportsInlineQueries   *bool   `json:"supportsInlineQueries,omitempty"`
	} `json:"bot,omitempty"`
	Webhook *struct {
		URL           *string `json:"url,omitempty"`
		HasCustomCert *bool   `json:"hasCustomCert,omitempty"`
	} `json:"webhook,omitempty"`
}

// ProbeTelegram 探测 Telegram Bot API 连接状态
func ProbeTelegram(ctx context.Context, client *http.Client, token string, timeoutMs int) *TelegramProbe {
	started := time.Now()
	if client == nil {
		client = &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	}

	result := &TelegramProbe{}
	base := fmt.Sprintf("%s/bot%s", TelegramAPIBaseURL, token)

	// getMe
	meBody, status, err := doProbeRequest(ctx, client, base+"/getMe", timeoutMs)
	if err != nil {
		errMsg := err.Error()
		result.Error = &errMsg
		result.ElapsedMs = time.Since(started).Milliseconds()
		return result
	}

	var meResp struct {
		OK     bool `json:"ok"`
		Result *struct {
			ID                      int64  `json:"id"`
			Username                string `json:"username"`
			CanJoinGroups           *bool  `json:"can_join_groups"`
			CanReadAllGroupMessages *bool  `json:"can_read_all_group_messages"`
			SupportsInlineQueries   *bool  `json:"supports_inline_queries"`
		} `json:"result"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(meBody, &meResp)

	// 对齐 TS: !meRes.ok 检查 HTTP 2xx 范围（非仅 200）
	if status < 200 || status >= 300 || !meResp.OK {
		result.Status = &status
		var errMsg string
		if meResp.Description != "" {
			errMsg = meResp.Description
		} else {
			errMsg = fmt.Sprintf("getMe failed (%d)", status)
		}
		result.Error = &errMsg
		result.ElapsedMs = time.Since(started).Milliseconds()
		return result
	}

	if meResp.Result != nil {
		r := meResp.Result
		result.Bot = &struct {
			ID                      *int64  `json:"id,omitempty"`
			Username                *string `json:"username,omitempty"`
			CanJoinGroups           *bool   `json:"canJoinGroups,omitempty"`
			CanReadAllGroupMessages *bool   `json:"canReadAllGroupMessages,omitempty"`
			SupportsInlineQueries   *bool   `json:"supportsInlineQueries,omitempty"`
		}{
			ID:                      &r.ID,
			Username:                &r.Username,
			CanJoinGroups:           r.CanJoinGroups,
			CanReadAllGroupMessages: r.CanReadAllGroupMessages,
			SupportsInlineQueries:   r.SupportsInlineQueries,
		}
	}

	// getWebhookInfo（可选，不影响整体探测结果）
	whBody, whStatus, whErr := doProbeRequest(ctx, client, base+"/getWebhookInfo", timeoutMs)
	if whErr == nil && whStatus == http.StatusOK {
		var whResp struct {
			OK     bool `json:"ok"`
			Result *struct {
				URL                  string `json:"url"`
				HasCustomCertificate *bool  `json:"has_custom_certificate"`
			} `json:"result"`
		}
		if json.Unmarshal(whBody, &whResp) == nil && whResp.OK && whResp.Result != nil {
			result.Webhook = &struct {
				URL           *string `json:"url,omitempty"`
				HasCustomCert *bool   `json:"hasCustomCert,omitempty"`
			}{
				URL:           &whResp.Result.URL,
				HasCustomCert: whResp.Result.HasCustomCertificate,
			}
		}
	}

	result.OK = true
	result.ElapsedMs = time.Since(started).Milliseconds()
	return result
}

func doProbeRequest(ctx context.Context, client *http.Client, url string, timeoutMs int) ([]byte, int, error) {
	tCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(tCtx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}
