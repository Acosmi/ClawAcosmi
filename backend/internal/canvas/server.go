package canvas

// Canvas Host 服务器 — 对应 src/canvas-host/server.ts (516L)
//
// 提供独立 HTTP 服务器启动入口。Handler 逻辑在 handler.go 中。

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
)

// ---------- 类型定义 ----------

// CanvasHandlerOpts handler 创建选项。
// TS 对照: server.ts L42-48
type CanvasHandlerOpts struct {
	RootDir      string // Canvas 目录（默认 ~/.openacosmi/canvas）
	BasePath     string // URL 基路径（默认 /__openacosmi__/canvas）
	LiveReload   *bool  // 是否启用 live-reload（默认 true）
	AllowInTests bool   // 测试模式下仍然启用
	Logger       *slog.Logger
}

// CanvasHandler canvas HTTP + WS handler。
// TS 对照: server.ts L50-56
type CanvasHandler struct {
	RootDir  string
	BasePath string

	rootReal   string
	liveReload bool
	logger     *slog.Logger

	// WS live-reload
	mu      sync.Mutex
	sockets map[*websocket.Conn]struct{}

	// fsnotify
	watcher       *fsnotify.Watcher
	watcherClosed bool
	debounceTimer *time.Timer

	closed bool
}

// CanvasHostServerOpts 独立服务器选项。
// TS 对照: server.ts L31-34
type CanvasHostServerOpts struct {
	CanvasHandlerOpts
	Port       int
	ListenHost string
	Handler    *CanvasHandler // 复用已有 handler（nil 则新建）
}

// CanvasHostServer 独立 Canvas 服务器。
// TS 对照: server.ts L36-40
type CanvasHostServer struct {
	Port    int
	RootDir string

	handler    *CanvasHandler
	httpServer *http.Server
	listener   net.Listener
}

// ---------- WS upgrader ----------

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ---------- StartCanvasHost ----------
// TS 对照: server.ts L436-515

// StartCanvasHost 启动独立 Canvas HTTP 服务器。
func StartCanvasHost(opts CanvasHostServerOpts) (*CanvasHostServer, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}

	handler := opts.Handler
	if handler == nil {
		var err error
		handler, err = NewCanvasHandler(opts.CanvasHandlerOpts)
		if err != nil {
			return nil, err
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// WS upgrade
		if r.Header.Get("Upgrade") == "websocket" {
			if handler.HandleUpgrade(w, r) {
				return
			}
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		// A2UI
		if HandleA2UIRequest(w, r) {
			return
		}
		// Canvas
		if handler.HandleHTTP(w, r) {
			return
		}
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	bindHost := opts.ListenHost
	if bindHost == "" {
		bindHost = "0.0.0.0"
	}
	listenPort := opts.Port
	if listenPort <= 0 {
		listenPort = 0
	}

	addr := fmt.Sprintf("%s:%d", bindHost, listenPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("canvas: 监听失败 %s: %w", addr, err)
	}
	boundPort := listener.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("canvas: 服务器错误", "error", err)
		}
	}()

	logger.Info("canvas host 已启动",
		"addr", fmt.Sprintf("http://%s:%d", bindHost, boundPort),
		"root", handler.RootDir,
	)

	return &CanvasHostServer{
		Port:       boundPort,
		RootDir:    handler.RootDir,
		handler:    handler,
		httpServer: srv,
		listener:   listener,
	}, nil
}

// Close 关闭 Canvas 服务器。
func (s *CanvasHostServer) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return err
	}
	return s.handler.Close()
}
