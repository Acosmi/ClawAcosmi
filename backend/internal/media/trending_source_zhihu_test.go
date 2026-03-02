package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestZhihuTrendingSource_Name(t *testing.T) {
	src := NewZhihuTrendingSource()
	if src.Name() != "zhihu" {
		t.Errorf("Name() = %q, want %q", src.Name(), "zhihu")
	}
}

func TestZhihuTrendingSource_Fetch(t *testing.T) {
	mockResp := zhihuAPIResponse{
		Data: []zhihuHotItem{
			{
				Target: struct {
					Title string `json:"title"`
					URL   string `json:"url"`
				}{Title: "知乎热点1", URL: "https://www.zhihu.com/question/123"},
				DetailText: "1234 万热度",
			},
			{
				Target: struct {
					Title string `json:"title"`
					URL   string `json:"url"`
				}{Title: "知乎热点2", URL: "https://www.zhihu.com/question/456"},
				DetailText: "567 万热度",
			},
			{
				Target: struct {
					Title string `json:"title"`
					URL   string `json:"url"`
				}{Title: "", URL: ""},
				DetailText: "",
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockResp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	src := NewZhihuTrendingSource()
	src.client = srv.Client()
	src.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	topics, err := src.Fetch(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("Fetch() error: %v", err)
	}

	if len(topics) != 2 {
		t.Fatalf("Fetch() returned %d topics, want 2", len(topics))
	}

	if topics[0].Title != "知乎热点1" {
		t.Errorf("topics[0].Title = %q, want %q", topics[0].Title, "知乎热点1")
	}
	if topics[0].Source != "zhihu" {
		t.Errorf("topics[0].Source = %q, want %q", topics[0].Source, "zhihu")
	}
	// 1234 万 = 12340000
	if topics[0].HeatScore != 12340000 {
		t.Errorf("topics[0].HeatScore = %f, want 12340000", topics[0].HeatScore)
	}
	// 567 万 = 5670000
	if topics[1].HeatScore != 5670000 {
		t.Errorf("topics[1].HeatScore = %f, want 5670000", topics[1].HeatScore)
	}
}

func TestZhihuTrendingSource_FetchWithLimit(t *testing.T) {
	mockResp := zhihuAPIResponse{
		Data: []zhihuHotItem{
			{Target: struct {
				Title string `json:"title"`
				URL   string `json:"url"`
			}{Title: "A"}, DetailText: "100 万热度"},
			{Target: struct {
				Title string `json:"title"`
				URL   string `json:"url"`
			}{Title: "B"}, DetailText: "50 万热度"},
			{Target: struct {
				Title string `json:"title"`
				URL   string `json:"url"`
			}{Title: "C"}, DetailText: "10 万热度"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mockResp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	src := NewZhihuTrendingSource()
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

func TestParseZhihuHeatScore(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"1234 万热度", 12340000},
		{"567 万热度", 5670000},
		{"1.5 万热度", 15000},
		{"热度 2345", 2345},
		{"", 0},
		{"无数字内容", 0},
	}
	for _, tt := range tests {
		got := parseZhihuHeatScore(tt.input)
		if got != tt.want {
			t.Errorf("parseZhihuHeatScore(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

func TestZhihuTrendingSource_FetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	src := NewZhihuTrendingSource()
	src.client = srv.Client()
	src.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return http.DefaultTransport.RoundTrip(req)
	})

	_, err := src.Fetch(context.Background(), "", 0)
	if err == nil {
		t.Fatal("expected error for HTTP 403")
	}
}
