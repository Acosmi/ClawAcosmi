package canvas

// Canvas Handler 创建和内部方法 — server.ts 下半部分

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"

	"github.com/openacosmi/claw-acismi/internal/config"
	"github.com/openacosmi/claw-acismi/internal/media"
)

// ---------- 默认 Index HTML ----------
// TS 对照: server.ts L58-150 defaultIndexHTML

func defaultIndexHTML() string {
	return `<!doctype html>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>OpenAcosmi Canvas</title>
<style>
  html, body { height: 100%; margin: 0; background: #000; color: #fff; font: 16px/1.4 -apple-system, BlinkMacSystemFont, system-ui, Segoe UI, Roboto, Helvetica, Arial, sans-serif; }
  .wrap { min-height: 100%; display: grid; place-items: center; padding: 24px; }
  .card { width: min(720px, 100%); background: rgba(255,255,255,0.06); border: 1px solid rgba(255,255,255,0.10); border-radius: 16px; padding: 18px 18px 14px; }
  h1 { margin: 0; font-size: 22px; letter-spacing: 0.2px; }
  .sub { opacity: 0.75; font-size: 13px; }
  .row { display: flex; gap: 10px; flex-wrap: wrap; margin-top: 14px; }
  button { appearance: none; border: 1px solid rgba(255,255,255,0.14); background: rgba(255,255,255,0.10); color: #fff; padding: 10px 12px; border-radius: 12px; font-weight: 600; cursor: pointer; }
  button:active { transform: translateY(1px); }
  .ok { color: #24e08a; }
  .bad { color: #ff5c5c; }
  .log { margin-top: 14px; opacity: 0.85; font: 12px/1.4 ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; white-space: pre-wrap; background: rgba(0,0,0,0.35); border: 1px solid rgba(255,255,255,0.08); padding: 10px; border-radius: 12px; }
</style>
<div class="wrap">
  <div class="card">
    <div style="display: flex; align-items: baseline; gap: 10px;">
      <h1>OpenAcosmi Canvas</h1>
      <div class="sub">Interactive test page (auto-reload enabled)</div>
    </div>
    <div class="row">
      <button id="btn-hello">Hello</button>
      <button id="btn-time">Time</button>
      <button id="btn-photo">Photo</button>
      <button id="btn-dalek">Dalek</button>
    </div>
    <div id="status" class="sub" style="margin-top: 10px;"></div>
    <div id="log" class="log">Ready.</div>
  </div>
</div>
<script>
(() => {
  const logEl = document.getElementById("log");
  const statusEl = document.getElementById("status");
  const log = (msg) => { logEl.textContent = String(msg); };
  const hasHelper = () => typeof window.openacosmiSendUserAction === "function";
  statusEl.innerHTML = "Bridge: " + (hasHelper() ? "<span class='ok'>ready</span>" : "<span class='bad'>missing</span>");
  function send(name, sourceComponentId) {
    if (!hasHelper()) { log("No action bridge found."); return; }
    const ok = window.openacosmiSendUserAction({ name, surfaceId: "main", sourceComponentId, context: { t: Date.now() } });
    log(ok ? ("Sent action: " + name) : ("Failed to send action: " + name));
  }
  document.getElementById("btn-hello").onclick = () => send("hello", "demo.hello");
  document.getElementById("btn-time").onclick = () => send("time", "demo.time");
  document.getElementById("btn-photo").onclick = () => send("photo", "demo.photo");
  document.getElementById("btn-dalek").onclick = () => send("dalek", "demo.dalek");
})();
</script>
`
}

// ---------- Handler 内部方法 ----------

// normalizeBasePath 规范化基路径。
// TS 对照: server.ts L212-219
func normalizeBasePath(rawPath string) string {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		trimmed = CanvasHostPath
	}
	normalized := normalizeURLPath(trimmed)
	if normalized == "/" {
		return "/"
	}
	return strings.TrimRight(normalized, "/")
}

// prepareCanvasRoot 准备 canvas 根目录并确保 index.html 存在。
// TS 对照: server.ts L221-235
func prepareCanvasRoot(rootDir string) (string, error) {
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return "", err
	}
	rootReal, err := filepath.EvalSymlinks(rootDir)
	if err != nil {
		return "", err
	}

	indexPath := filepath.Join(rootReal, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		_ = os.WriteFile(indexPath, []byte(defaultIndexHTML()), 0644)
	}
	return rootReal, nil
}

// resolveDefaultCanvasRoot 获取默认 canvas 根目录。
// TS 对照: server.ts L237-247
func resolveDefaultCanvasRoot() string {
	return filepath.Join(config.ResolveStateDir(), "canvas")
}

// resolveCanvasFilePath 安全地解析 canvas 文件路径（使用 Lstat 拒绝 symlink）。
// TS 对照: server.ts L158-194
func resolveCanvasFilePath(rootReal, urlPath string) string {
	normalized := normalizeURLPath(urlPath)
	rel := strings.TrimLeft(normalized, "/")

	// 拒绝 ".." 组件
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return ""
		}
	}

	if strings.HasSuffix(normalized, "/") {
		return tryOpenSafe(rootReal, path.Join(rel, "index.html"))
	}

	candidate := filepath.Join(rootReal, rel)
	linfo, err := os.Lstat(candidate)
	if err != nil {
		return ""
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	if linfo.IsDir() {
		return tryOpenSafe(rootReal, path.Join(rel, "index.html"))
	}

	return tryOpenSafe(rootReal, rel)
}

// tryOpenSafe 尝试以安全方式打开文件（确保在 root 内）。
func tryOpenSafe(rootReal, relPath string) string {
	candidate := filepath.Join(rootReal, relPath)
	linfo, err := os.Lstat(candidate)
	if err != nil || linfo.IsDir() {
		return ""
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return ""
	}
	real, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return ""
	}
	rootPrefix := rootReal
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}
	if !strings.HasPrefix(real, rootPrefix) && real != rootReal {
		return ""
	}
	return real
}

// ---------- NewCanvasHandler ----------
// TS 对照: server.ts L249-434

// NewCanvasHandler 创建 Canvas handler。
func NewCanvasHandler(opts CanvasHandlerOpts) (*CanvasHandler, error) {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	basePath := normalizeBasePath(opts.BasePath)
	liveReload := opts.LiveReload == nil || *opts.LiveReload

	rootDir := opts.RootDir
	if rootDir == "" {
		rootDir = resolveDefaultCanvasRoot()
	}

	rootReal, err := prepareCanvasRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("canvas: 准备根目录失败: %w", err)
	}

	h := &CanvasHandler{
		RootDir:    rootDir,
		BasePath:   basePath,
		rootReal:   rootReal,
		liveReload: liveReload,
		logger:     logger,
		sockets:    make(map[*websocket.Conn]struct{}),
	}

	// 设置 fsnotify watcher（仅 liveReload 模式）
	if liveReload {
		watcher, watchErr := fsnotify.NewWatcher()
		if watchErr != nil {
			logger.Warn("canvas: fsnotify 创建失败，live-reload 已禁用", "error", watchErr)
		} else {
			h.watcher = watcher
			if addErr := watcher.Add(rootReal); addErr != nil {
				logger.Warn("canvas: 监视目录失败", "error", addErr, "dir", rootReal)
			}
			go h.watchLoop()
		}
	}

	return h, nil
}

// watchLoop fsnotify 事件循环。
func (h *CanvasHandler) watchLoop() {
	if h.watcher == nil {
		return
	}
	for {
		select {
		case event, ok := <-h.watcher.Events:
			if !ok {
				return
			}
			_ = event // 任何文件事件都触发 reload
			h.scheduleReload()
		case err, ok := <-h.watcher.Errors:
			if !ok {
				return
			}
			h.mu.Lock()
			if !h.watcherClosed {
				h.watcherClosed = true
				h.logger.Error("canvas: watcher 错误，live-reload 已禁用", "error", err)
				_ = h.watcher.Close()
			}
			h.mu.Unlock()
			return
		}
	}
}

// scheduleReload debounce 75ms 后广播 reload。
// TS 对照: server.ts L289-298
func (h *CanvasHandler) scheduleReload() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.debounceTimer != nil {
		h.debounceTimer.Stop()
	}
	h.debounceTimer = time.AfterFunc(75*time.Millisecond, func() {
		h.broadcastReload()
	})
}

// broadcastReload 向所有 WS 客户端发送 "reload"。
// TS 对照: server.ts L277-288
func (h *CanvasHandler) broadcastReload() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ws := range h.sockets {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("reload"))
	}
}

// HandleUpgrade 处理 WS Upgrade 请求。
// TS 对照: server.ts L324-336
func (h *CanvasHandler) HandleUpgrade(w http.ResponseWriter, r *http.Request) bool {
	if !h.liveReload {
		return false
	}
	u, err := url.Parse(r.RequestURI)
	if err != nil {
		return false
	}
	if u.Path != CanvasWSPath {
		return false
	}
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return false
	}
	h.mu.Lock()
	h.sockets[conn] = struct{}{}
	h.mu.Unlock()
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.sockets, conn)
			h.mu.Unlock()
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
	return true
}

// HandleHTTP 处理 Canvas HTTP 请求。
// TS 对照: server.ts L338-416
func (h *CanvasHandler) HandleHTTP(w http.ResponseWriter, r *http.Request) bool {
	urlPath := r.URL.Path

	// WS 路径的 HTTP 请求
	if urlPath == CanvasWSPath {
		if h.liveReload {
			http.Error(w, "upgrade required", http.StatusUpgradeRequired)
		} else {
			http.Error(w, "not found", http.StatusNotFound)
		}
		return true
	}

	// 基路径匹配
	localPath := urlPath
	if h.BasePath != "/" {
		if urlPath != h.BasePath && !strings.HasPrefix(urlPath, h.BasePath+"/") {
			return false
		}
		if urlPath == h.BasePath {
			localPath = "/"
		} else {
			localPath = strings.TrimPrefix(urlPath, h.BasePath)
			if localPath == "" {
				localPath = "/"
			}
		}
	}

	// 方法检查
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return true
	}

	// 文件解析
	filePath := resolveCanvasFilePath(h.rootReal, localPath)
	if filePath == "" {
		if localPath == "/" || strings.HasSuffix(localPath, "/") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `<!doctype html><meta charset="utf-8" /><title>OpenAcosmi Canvas</title><pre>Missing file.
Create %s/index.html</pre>`, h.RootDir)
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			http.Error(w, "not found", http.StatusNotFound)
		}
		return true
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		h.logger.Error("canvas: 读取文件失败", "error", err, "path", filePath)
		http.Error(w, "error", http.StatusInternalServerError)
		return true
	}

	lower := strings.ToLower(filePath)
	mime := ""
	if strings.HasSuffix(lower, ".html") || strings.HasSuffix(lower, ".htm") {
		mime = "text/html"
	} else {
		mime = media.DetectMime(media.DetectMimeOpts{FilePath: filePath})
		if mime == "" {
			mime = "application/octet-stream"
		}
	}

	w.Header().Set("Cache-Control", "no-store")
	if mime == "text/html" {
		html := string(data)
		if h.liveReload {
			html = InjectCanvasLiveReload(html)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(html))
		return true
	}

	w.Header().Set("Content-Type", mime)
	w.Write(data)
	return true
}

// Close 关闭 handler（停止 watcher、关闭 WS 连接）。
// TS 对照: server.ts L423-433
func (h *CanvasHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	h.closed = true

	if h.debounceTimer != nil {
		h.debounceTimer.Stop()
	}
	h.watcherClosed = true
	if h.watcher != nil {
		_ = h.watcher.Close()
	}
	for ws := range h.sockets {
		_ = ws.Close()
		delete(h.sockets, ws)
	}
	return nil
}
