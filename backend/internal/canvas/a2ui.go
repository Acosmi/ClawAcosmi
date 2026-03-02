package canvas

// A2UI 静态文件服务 — 对应 src/canvas-host/a2ui.ts (219L)
//
// 提供 /__openacosmi__/a2ui 路由下的 A2UI 静态资源服务，
// 包含安全路径解析、MIME 检测和 live-reload 脚本注入。

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/openacosmi/claw-acismi/internal/media"
)

// ---------- 路径常量 ----------

const (
	// A2UIPath A2UI 静态资源路径前缀
	A2UIPath = "/__openacosmi__/a2ui"
	// CanvasHostPath Canvas 静态文件路径前缀
	CanvasHostPath = "/__openacosmi__/canvas"
	// CanvasWSPath Canvas WebSocket 路径
	CanvasWSPath = "/__openacosmi__/ws"
)

// ---------- A2UI Root 解析 ----------

var (
	a2uiRootOnce sync.Once
	a2uiRootReal string // 缓存的 a2ui 根目录 realpath（空字符串表示未找到）
)

// resolveA2UIRoot 查找 a2ui 静态资源根目录。
// TS 对照: a2ui.ts L16-43 resolveA2uiRoot
func resolveA2UIRoot() string {
	exe, _ := os.Executable()
	exeDir := ""
	if exe != "" {
		exeDir = filepath.Dir(exe)
	}

	cwd, _ := os.Getwd()

	candidates := []string{}
	if exeDir != "" {
		candidates = append(candidates, filepath.Join(exeDir, "a2ui"))
	}
	// 常见开发/部署路径
	candidates = append(candidates,
		filepath.Join(cwd, "src", "canvas-host", "a2ui"),
		filepath.Join(cwd, "dist", "canvas-host", "a2ui"),
	)

	for _, dir := range candidates {
		indexPath := filepath.Join(dir, "index.html")
		bundlePath := filepath.Join(dir, "a2ui.bundle.js")
		if fileExists(indexPath) && fileExists(bundlePath) {
			real, err := filepath.EvalSymlinks(dir)
			if err == nil {
				return real
			}
			return dir
		}
	}
	return ""
}

// getA2UIRoot 获取 a2ui 根目录（带缓存）。
func getA2UIRoot() string {
	a2uiRootOnce.Do(func() {
		a2uiRootReal = resolveA2UIRoot()
	})
	return a2uiRootReal
}

// ---------- 安全路径解析 ----------

// normalizeURLPath 规范化 URL 路径。
// TS 对照: a2ui.ts L59-63
func normalizeURLPath(rawPath string) string {
	if rawPath == "" {
		rawPath = "/"
	}
	decoded, err := url.PathUnescape(rawPath)
	if err != nil {
		decoded = rawPath
	}
	normalized := path.Clean(decoded)
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	return normalized
}

// resolveA2UIFilePath 安全地解析 a2ui 文件路径。
// 拒绝 symlink、路径遍历和超出 rootReal 的访问。
// TS 对照: a2ui.ts L65-100
func resolveA2UIFilePath(rootReal, urlPath string) string {
	normalized := normalizeURLPath(urlPath)
	rel := strings.TrimLeft(normalized, "/")

	// 拒绝 ".." 组件
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return ""
		}
	}

	candidate := filepath.Join(rootReal, rel)
	if strings.HasSuffix(normalized, "/") {
		candidate = filepath.Join(candidate, "index.html")
	}

	// 检测目录 → index.html
	info, err := os.Stat(candidate)
	if err == nil && info.IsDir() {
		candidate = filepath.Join(candidate, "index.html")
	}

	// 安全检查：拒绝 symlink
	linfo, err := os.Lstat(candidate)
	if err != nil {
		return ""
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return ""
	}

	// 确保 realpath 在 rootReal 内
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

// ---------- Live Reload 注入 ----------

// InjectCanvasLiveReload 向 HTML 注入 live-reload + 原生桥接脚本。
// TS 对照: a2ui.ts L102-159
func InjectCanvasLiveReload(html string) string {
	snippet := `
<script>
(() => {
  const handlerNames = ["openacosmiCanvasA2UIAction"];
  function postToNode(payload) {
    try {
      const raw = typeof payload === "string" ? payload : JSON.stringify(payload);
      for (const name of handlerNames) {
        const iosHandler = globalThis.webkit?.messageHandlers?.[name];
        if (iosHandler && typeof iosHandler.postMessage === "function") {
          iosHandler.postMessage(raw);
          return true;
        }
        const androidHandler = globalThis[name];
        if (androidHandler && typeof androidHandler.postMessage === "function") {
          androidHandler.postMessage(raw);
          return true;
        }
      }
    } catch {}
    return false;
  }
  function sendUserAction(userAction) {
    const id =
      (userAction && typeof userAction.id === "string" && userAction.id.trim()) ||
      (globalThis.crypto?.randomUUID?.() ?? String(Date.now()));
    const action = { ...userAction, id };
    return postToNode({ userAction: action });
  }
  globalThis.OpenAcosmi = globalThis.OpenAcosmi ?? {};
  globalThis.OpenAcosmi.postMessage = postToNode;
  globalThis.OpenAcosmi.sendUserAction = sendUserAction;
  globalThis.openacosmiPostMessage = postToNode;
  globalThis.openacosmiSendUserAction = sendUserAction;

  try {
    const proto = location.protocol === "https:" ? "wss" : "ws";
    const ws = new WebSocket(proto + "://" + location.host + "` + CanvasWSPath + `");
    ws.onmessage = (ev) => {
      if (String(ev.data || "") === "reload") location.reload();
    };
  } catch {}
})();
</script>`

	idx := strings.LastIndex(strings.ToLower(html), "</body>")
	if idx >= 0 {
		return html[:idx] + "\n" + strings.TrimSpace(snippet) + "\n" + html[idx:]
	}
	return html + "\n" + strings.TrimSpace(snippet) + "\n"
}

// ---------- A2UI HTTP Handler ----------

// HandleA2UIRequest 处理 A2UI HTTP 请求。
// TS 对照: a2ui.ts L161-218
func HandleA2UIRequest(w http.ResponseWriter, r *http.Request) bool {
	urlPath := r.URL.Path
	if urlPath != A2UIPath && !strings.HasPrefix(urlPath, A2UIPath+"/") {
		return false
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return true
	}

	rootReal := getA2UIRoot()
	if rootReal == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, "A2UI assets not found", http.StatusServiceUnavailable)
		return true
	}

	rel := strings.TrimPrefix(urlPath, A2UIPath)
	if rel == "" {
		rel = "/"
	}
	filePath := resolveA2UIFilePath(rootReal, rel)
	if filePath == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, "not found", http.StatusNotFound)
		return true
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		http.Error(w, "not found", http.StatusNotFound)
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
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(InjectCanvasLiveReload(string(data))))
		return true
	}

	w.Header().Set("Content-Type", mime)
	w.Write(data)
	return true
}

// ---------- 辅助 ----------

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
