package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BochaSearchProvider 博查搜索 API 实现。
// API 文档: https://open.bochaai.com
// Endpoint: POST https://api.bochaai.com/v1/web-search
// 认证: Authorization: Bearer <apiKey>
type BochaSearchProvider struct {
	APIKey  string
	BaseURL string // 默认 https://api.bochaai.com
	client  *http.Client
}

// NewBochaSearchProvider 创建博查搜索 provider。
func NewBochaSearchProvider(apiKey, baseURL string) *BochaSearchProvider {
	if baseURL == "" {
		baseURL = "https://api.bochaai.com"
	}
	return &BochaSearchProvider{
		APIKey:  apiKey,
		BaseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// bochaRequest 博查搜索请求体。
type bochaRequest struct {
	Query     string `json:"query"`
	Freshness string `json:"freshness,omitempty"` // "noLimit"|"oneDay"|"oneWeek"|"oneMonth"|"oneYear"
	Summary   bool   `json:"summary"`
	Count     int    `json:"count,omitempty"`
}

// bochaResponse 博查搜索响应体。
type bochaResponse struct {
	WebPages *bochaWebPages `json:"webPages"`
}

type bochaWebPages struct {
	TotalEstimatedMatches int         `json:"totalEstimatedMatches"`
	Value                 []bochaPage `json:"value"`
}

type bochaPage struct {
	Name          string `json:"name"`
	URL           string `json:"url"`
	Snippet       string `json:"snippet"`
	Summary       string `json:"summary,omitempty"`
	SiteName      string `json:"siteName,omitempty"`
	DatePublished string `json:"datePublished,omitempty"`
}

// Search 执行博查搜索，返回统一格式的搜索结果。
func (p *BochaSearchProvider) Search(ctx context.Context, query string, maxResults int) ([]WebSearchResult, error) {
	if p.APIKey == "" {
		return nil, fmt.Errorf("bocha: API key not configured")
	}

	if maxResults <= 0 {
		maxResults = 8
	}
	if maxResults > 50 {
		maxResults = 50
	}

	reqBody := bochaRequest{
		Query:   query,
		Summary: true,
		Count:   maxResults,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("bocha: marshal request: %w", err)
	}

	url := p.BaseURL + "/v1/web-search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("bocha: create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Body = io.NopCloser(jsonReader(bodyBytes))

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bocha: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("bocha: API error %d: %s", resp.StatusCode, string(respBody))
	}

	var bochaResp bochaResponse
	if err := json.NewDecoder(resp.Body).Decode(&bochaResp); err != nil {
		return nil, fmt.Errorf("bocha: decode response: %w", err)
	}

	if bochaResp.WebPages == nil {
		return nil, nil
	}

	results := make([]WebSearchResult, 0, len(bochaResp.WebPages.Value))
	for _, page := range bochaResp.WebPages.Value {
		snippet := page.Snippet
		if page.Summary != "" {
			snippet = page.Summary
		}
		results = append(results, WebSearchResult{
			Title:   page.Name,
			URL:     page.URL,
			Snippet: snippet,
		})
	}

	return results, nil
}

// jsonReader 将 []byte 包装为 io.Reader。
func jsonReader(data []byte) io.Reader {
	return &jsonBytesReader{data: data, pos: 0}
}

type jsonBytesReader struct {
	data []byte
	pos  int
}

func (r *jsonBytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
