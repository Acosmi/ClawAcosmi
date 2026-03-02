package media

// ============================================================================
// media/trending_source_weibo.go — 微博热搜数据源
// 使用微博公开 AJAX 接口拉取实时热搜榜。
// ============================================================================

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// WeiboTrendingSource 微博热搜数据源。
type WeiboTrendingSource struct {
	client *http.Client
}

// NewWeiboTrendingSource 创建微博热搜源。
func NewWeiboTrendingSource() *WeiboTrendingSource {
	return &WeiboTrendingSource{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WeiboTrendingSource) Name() string { return "weibo" }

// weiboAPIResponse 微博热搜 API 响应结构。
type weiboAPIResponse struct {
	OK   int `json:"ok"`
	Data struct {
		Realtime []weiboRealtimeItem `json:"realtime"`
	} `json:"data"`
}

type weiboRealtimeItem struct {
	Word      string  `json:"word"`
	RawHot    float64 `json:"raw_hot"`
	LabelName string  `json:"label_name"`
}

const weiboHotSearchURL = "https://weibo.com/ajax/side/hotSearch"

func (w *WeiboTrendingSource) Fetch(ctx context.Context, category string, limit int) ([]TrendingTopic, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, weiboHotSearchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("weibo: create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("weibo: fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weibo: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("weibo: read body: %w", err)
	}

	var apiResp weiboAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("weibo: parse JSON: %w", err)
	}

	now := time.Now().UTC()
	topics := make([]TrendingTopic, 0, len(apiResp.Data.Realtime))
	for _, item := range apiResp.Data.Realtime {
		if item.Word == "" {
			continue
		}
		cat := "general"
		if item.LabelName != "" {
			cat = item.LabelName
		}
		// 按 category 过滤（空字符串不过滤）
		if category != "" && cat != category {
			continue
		}
		topics = append(topics, TrendingTopic{
			Title:     item.Word,
			Source:    "weibo",
			URL:       "https://s.weibo.com/weibo?q=" + url.QueryEscape(item.Word),
			HeatScore: item.RawHot,
			Category:  cat,
			FetchedAt: now,
		})
	}

	if limit > 0 && len(topics) > limit {
		topics = topics[:limit]
	}
	return topics, nil
}
