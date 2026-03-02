package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWeiboTrendingSource_Name(t *testing.T) {
	src := NewWeiboTrendingSource()
	if src.Name() != "weibo" {
		t.Errorf("Name() = %q, want %q", src.Name(), "weibo")
	}
}

func TestWeiboTrendingSource_Fetch(t *testing.T) {
	mockResp := weiboAPIResponse{OK: 1}
	mockResp.Data.Realtime = []weiboRealtimeItem{
		{Word: "热搜话题1", RawHot: 999999, LabelName: ""},
		{Word: "热搜话题2", RawHot: 888888, LabelName: "娱乐"},
		{Word: "", RawHot: 100, LabelName: ""},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockResp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	src := NewWeiboTrendingSource()
	src.client = srv.Client()

	// 替换 URL — 通过自定义 transport 劫持请求到 mock server
	origURL := weiboHotSearchURL
	_ = origURL // 无法直接替换 const，用 httptest 模式

	// 使用 RoundTripper 重定向到 mock server
	src.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	topics, err := src.Fetch(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	// 空 word 条目应被过滤
	if len(topics) != 2 {
		t.Fatalf("Fetch() returned %d topics, want 2", len(topics))
	}

	if topics[0].Title != "热搜话题1" {
		t.Errorf("topics[0].Title = %q, want %q", topics[0].Title, "热搜话题1")
	}
	if topics[0].Source != "weibo" {
		t.Errorf("topics[0].Source = %q, want %q", topics[0].Source, "weibo")
	}
	if topics[0].HeatScore != 999999 {
		t.Errorf("topics[0].HeatScore = %f, want 999999", topics[0].HeatScore)
	}
	if topics[0].URL == "" {
		t.Error("topics[0].URL is empty")
	}
}

func TestWeiboTrendingSource_FetchWithCategory(t *testing.T) {
	mockResp := weiboAPIResponse{OK: 1}
	mockResp.Data.Realtime = []weiboRealtimeItem{
		{Word: "话题1", RawHot: 100, LabelName: "科技"},
		{Word: "话题2", RawHot: 200, LabelName: "娱乐"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockResp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	src := NewWeiboTrendingSource()
	src.client = srv.Client()
	src.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	topics, err := src.Fetch(context.Background(), "娱乐", 0)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(topics) != 1 {
		t.Fatalf("Fetch() with category returned %d, want 1", len(topics))
	}
	if topics[0].Title != "话题2" {
		t.Errorf("filtered topic = %q, want %q", topics[0].Title, "话题2")
	}
}

func TestWeiboTrendingSource_FetchWithLimit(t *testing.T) {
	mockResp := weiboAPIResponse{OK: 1}
	mockResp.Data.Realtime = []weiboRealtimeItem{
		{Word: "A", RawHot: 300},
		{Word: "B", RawHot: 200},
		{Word: "C", RawHot: 100},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockResp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	src := NewWeiboTrendingSource()
	src.client = srv.Client()
	src.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	topics, err := src.Fetch(context.Background(), "", 2)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}
	if len(topics) != 2 {
		t.Fatalf("Fetch() with limit=2 returned %d, want 2", len(topics))
	}
}

func TestWeiboTrendingSource_FetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	src := NewWeiboTrendingSource()
	src.client = srv.Client()
	src.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	_, err := src.Fetch(context.Background(), "", 0)
	if err == nil {
		t.Fatal("expected error for HTTP 503")
	}
}

// roundTripFunc 用于测试中劫持 HTTP 请求。
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
