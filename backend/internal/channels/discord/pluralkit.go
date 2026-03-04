package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Acosmi/ClawAcosmi/pkg/types"
)

// PluralKit 集成 — 继承自 src/discord/pluralkit.ts (59L)

const pluralKitAPIBase = "https://api.pluralkit.me/v2"

// PluralKitSystemInfo PluralKit 系统信息
type PluralKitSystemInfo struct {
	ID   string  `json:"id"`
	Name *string `json:"name,omitempty"`
	Tag  *string `json:"tag,omitempty"`
}

// PluralKitMemberInfo PluralKit 成员信息
type PluralKitMemberInfo struct {
	ID          string  `json:"id"`
	Name        *string `json:"name,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
}

// PluralKitMessageInfo PluralKit 消息信息
type PluralKitMessageInfo struct {
	ID       string               `json:"id"`
	Original *string              `json:"original,omitempty"`
	Sender   *string              `json:"sender,omitempty"`
	System   *PluralKitSystemInfo `json:"system,omitempty"`
	Member   *PluralKitMemberInfo `json:"member,omitempty"`
}

// FetchPluralKitMessageInfo 查询消息是否为 PluralKit 代理消息。
// 如果 PluralKit 未启用或消息不是代理消息（404），返回 nil, nil。
// opts 可选，支持通过 opts.Client 注入自定义 HTTP Client（对齐 TS params.fetcher）。
func FetchPluralKitMessageInfo(ctx context.Context, messageID string, config *types.DiscordPluralKitConfig, opts *DiscordFetchOptions) (*PluralKitMessageInfo, error) {
	if config == nil || config.Enabled == nil || !*config.Enabled {
		return nil, nil
	}

	url := fmt.Sprintf("%s/messages/%s", pluralKitAPIBase, messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("pluralkit: create request: %w", err)
	}

	if token := strings.TrimSpace(config.Token); token != "" {
		req.Header.Set("Authorization", token)
	}

	client := http.DefaultClient
	if opts != nil && opts.Client != nil {
		client = opts.Client
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pluralkit: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		detail := strings.TrimSpace(string(body))
		if detail != "" {
			return nil, fmt.Errorf("PluralKit API failed (%d): %s", resp.StatusCode, detail)
		}
		return nil, fmt.Errorf("PluralKit API failed (%d)", resp.StatusCode)
	}

	var info PluralKitMessageInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("pluralkit: decode response: %w", err)
	}
	return &info, nil
}
