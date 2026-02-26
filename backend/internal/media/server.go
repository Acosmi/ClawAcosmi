package media

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// TS 对照: media/server.ts (107L)
// 媒体 HTTP 服务器 — CORS + 认证中间件集成。

// MediaServerConfig 媒体服务器配置。
type MediaServerConfig struct {
	Port    int
	TTLMs   int64
	BaseDir string
	Auth    *MediaServerAuthConfig
}

// MediaServerAuthConfig 媒体服务器认证配置。
type MediaServerAuthConfig struct {
	// Token Bearer token 认证（空字符串 = 不启用认证）
	Token string
	// AllowOrigin CORS 允许来源（空 = 不设置 CORS 头，"*" = 全部允许）
	AllowOrigin string
}

// AttachMediaRoutes 将媒体路由注册到 HTTP 多路复用器。
// TS 对照: server.ts L20-85
func AttachMediaRoutes(mux *http.ServeMux, dir string, auth *MediaServerAuthConfig) {
	if dir == "" {
		dir = GetMediaDir()
	}

	mux.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) {
		// CORS 支持
		if auth != nil && auth.AllowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", auth.AllowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		// CORS 预飞请求
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Bearer token 认证
		if auth != nil && auth.Token != "" {
			token := extractBearerToken(r)
			if token != auth.Token {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
		}

		// 安全检查：路径遍历
		requested := strings.TrimPrefix(r.URL.Path, "/media/")
		if strings.Contains(requested, "..") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		fullPath := filepath.Join(dir, requested)

		// 检查文件存在
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			http.NotFound(w, r)
			return
		}

		// 检测 MIME
		data, err := os.ReadFile(fullPath)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		contentType := DetectMime(DetectMimeOpts{
			Buffer:   data,
			FilePath: fullPath,
		})
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Write(data)
	})
}

// extractBearerToken 从请求头提取 Bearer token。
func extractBearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return ""
	}
	return strings.TrimSpace(auth[7:])
}

// StartMediaServer 启动独立的媒体 HTTP 服务器。
// TS 对照: server.ts L87-107
func StartMediaServer(config MediaServerConfig) (*http.Server, error) {
	if config.Port <= 0 {
		return nil, fmt.Errorf("无效端口: %d", config.Port)
	}
	if config.BaseDir == "" {
		config.BaseDir = GetMediaDir()
	}
	mux := http.NewServeMux()
	AttachMediaRoutes(mux, config.BaseDir, config.Auth)

	addr := fmt.Sprintf("127.0.0.1:%d", config.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	go func() {
		_ = server.ListenAndServe()
	}()
	return server, nil
}
