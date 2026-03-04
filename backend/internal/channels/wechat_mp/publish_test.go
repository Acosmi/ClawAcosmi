package wechat_mp

// ============================================================================
// wechat_mp/publish_test.go — 发布流程单元测试
// 使用 httptest.Server mock 微信发布 API。
//
// Design doc: docs/xinshenji/impl-tracking-media-subagent.md §P2-2
// ============================================================================

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Acosmi/ClawAcosmi/internal/media"
)

// newPublishTestServer 创建 mock 服务器 + Publisher。
func newPublishTestServer(handler http.HandlerFunc) (*httptest.Server, *Publisher) {
	srv, client := newTestServer(handler)
	return srv, NewPublisher(client)
}

// tokenHandler 处理 /cgi-bin/token 请求的通用 handler 片段。
func handleToken(w http.ResponseWriter, r *http.Request) bool {
	if strings.Contains(r.URL.Path, "/cgi-bin/token") {
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "pub_token",
			"expires_in":   7200,
		})
		return true
	}
	return false
}

// ---------- CreateDraft tests ----------

func TestCreateDraft_Success(t *testing.T) {
	srv, pub := newPublishTestServer(func(w http.ResponseWriter, r *http.Request) {
		if handleToken(w, r) {
			return
		}
		if strings.Contains(r.URL.Path, "/cgi-bin/draft/add") {
			json.NewEncoder(w).Encode(map[string]any{
				"errcode":  0,
				"media_id": "draft_media_001",
			})
			return
		}
		t.Errorf("unexpected path: %s", r.URL.Path)
	})
	defer srv.Close()

	draft := &media.ContentDraft{
		Title: "测试文章",
		Body:  "这是测试正文内容。",
	}

	mediaID, err := pub.CreateDraft(context.Background(), draft)
	if err != nil {
		t.Fatalf("CreateDraft: %v", err)
	}
	if mediaID != "draft_media_001" {
		t.Errorf("media_id: got %q, want %q", mediaID, "draft_media_001")
	}
}

func TestCreateDraft_NilDraft(t *testing.T) {
	srv, pub := newPublishTestServer(func(w http.ResponseWriter, r *http.Request) {
		handleToken(w, r)
	})
	defer srv.Close()

	_, err := pub.CreateDraft(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil draft")
	}
}

func TestCreateDraft_APIError(t *testing.T) {
	srv, pub := newPublishTestServer(func(w http.ResponseWriter, r *http.Request) {
		if handleToken(w, r) {
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"errcode": 40001,
			"errmsg":  "invalid credential",
		})
	})
	defer srv.Close()

	draft := &media.ContentDraft{
		Title: "Test",
		Body:  "Body",
	}
	_, err := pub.CreateDraft(context.Background(), draft)
	if err == nil {
		t.Fatal("expected error for API error")
	}
}

// ---------- SubmitPublish tests ----------

func TestSubmitPublish_Success(t *testing.T) {
	srv, pub := newPublishTestServer(func(w http.ResponseWriter, r *http.Request) {
		if handleToken(w, r) {
			return
		}
		if strings.Contains(r.URL.Path, "/cgi-bin/freepublish/submit") {
			json.NewEncoder(w).Encode(map[string]any{
				"errcode":    0,
				"publish_id": "pub_12345",
			})
			return
		}
	})
	defer srv.Close()

	pubID, err := pub.SubmitPublish(context.Background(), "draft_media_001")
	if err != nil {
		t.Fatalf("SubmitPublish: %v", err)
	}
	if pubID != "pub_12345" {
		t.Errorf("publish_id: got %q, want %q", pubID, "pub_12345")
	}
}

// ---------- GetPublishStatus tests ----------

func TestGetPublishStatus_Published(t *testing.T) {
	srv, pub := newPublishTestServer(func(w http.ResponseWriter, r *http.Request) {
		if handleToken(w, r) {
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"errcode":        0,
			"publish_status": 0,
			"article_id":     "art_001",
			"article_url":    "https://mp.weixin.qq.com/s/test",
		})
	})
	defer srv.Close()

	result, err := pub.GetPublishStatus(context.Background(), "pub_12345")
	if err != nil {
		t.Fatalf("GetPublishStatus: %v", err)
	}
	if result.Status != "published" {
		t.Errorf("status: got %q, want %q", result.Status, "published")
	}
	if result.PostID != "art_001" {
		t.Errorf("post_id: got %q, want %q", result.PostID, "art_001")
	}
}

func TestGetPublishStatus_Publishing(t *testing.T) {
	srv, pub := newPublishTestServer(func(w http.ResponseWriter, r *http.Request) {
		if handleToken(w, r) {
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"errcode":        0,
			"publish_status": 1,
		})
	})
	defer srv.Close()

	result, err := pub.GetPublishStatus(context.Background(), "pub_12345")
	if err != nil {
		t.Fatalf("GetPublishStatus: %v", err)
	}
	if result.Status != "publishing" {
		t.Errorf("status: got %q, want %q", result.Status, "publishing")
	}
}

func TestGetPublishStatus_Failed(t *testing.T) {
	srv, pub := newPublishTestServer(func(w http.ResponseWriter, r *http.Request) {
		if handleToken(w, r) {
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"errcode":        0,
			"publish_status": 2,
		})
	})
	defer srv.Close()

	result, err := pub.GetPublishStatus(context.Background(), "pub_12345")
	if err != nil {
		t.Fatalf("GetPublishStatus: %v", err)
	}
	if result.Status != "failed" {
		t.Errorf("status: got %q, want %q", result.Status, "failed")
	}
	if result.Error == "" {
		t.Error("expected error message for failed status")
	}
}

// ---------- Publish full pipeline ----------

func TestPublish_FullPipeline(t *testing.T) {
	srv, pub := newPublishTestServer(func(w http.ResponseWriter, r *http.Request) {
		if handleToken(w, r) {
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/cgi-bin/draft/add"):
			json.NewEncoder(w).Encode(map[string]any{
				"errcode":  0,
				"media_id": "pipe_draft_001",
			})
		case strings.Contains(r.URL.Path, "/cgi-bin/freepublish/submit"):
			json.NewEncoder(w).Encode(map[string]any{
				"errcode":    0,
				"publish_id": "pipe_pub_001",
			})
		case strings.Contains(r.URL.Path, "/cgi-bin/freepublish/get"):
			json.NewEncoder(w).Encode(map[string]any{
				"errcode":        0,
				"publish_status": 0,
				"article_id":     "pipe_art_001",
				"article_url":    "https://mp.weixin.qq.com/s/pipeline",
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	})
	defer srv.Close()

	draft := &media.ContentDraft{
		Title:    "全链路测试",
		Body:     "这是全链路发布测试。",
		Platform: media.PlatformWeChat,
	}

	result, err := pub.Publish(context.Background(), draft)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if result.Status != "published" {
		t.Errorf("status: got %q, want %q", result.Status, "published")
	}
	if result.PostID != "pipe_art_001" {
		t.Errorf("post_id: got %q, want %q", result.PostID, "pipe_art_001")
	}
}

// ---------- formatHTMLContent ----------

func TestFormatHTMLContent(t *testing.T) {
	got := formatHTMLContent("Hello world")
	want := "<p>Hello world</p>"
	if got != want {
		t.Errorf("formatHTMLContent: got %q, want %q", got, want)
	}
	if formatHTMLContent("") != "" {
		t.Error("expected empty string for empty input")
	}
}
