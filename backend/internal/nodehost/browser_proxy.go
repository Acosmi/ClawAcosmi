package nodehost

// browser_proxy.go — browser.proxy 命令处理
// 对应 TS: runner.ts L748-856 (handleInvoke browser.proxy 分支)
//
// 通过注入的 BrowserProxyHandler 接口与 internal/browser 解耦。
// 支持 profile 白名单、请求超时、响应文件收集。

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// BrowserProxyHandler 是浏览器代理处理器接口。
// 具体实现由 internal/browser.BrowserServer 提供。
type BrowserProxyHandler interface {
	// Dispatch 分发浏览器代理请求，返回 HTTP 状态码和响应体。
	Dispatch(ctx context.Context, method, path string, query map[string]interface{}, body interface{}) (status int, result interface{}, err error)
}

// BrowserProxyConfig 浏览器代理配置。
type BrowserProxyConfig struct {
	Enabled        bool
	AllowProfiles  []string
	DefaultProfile string // 默认浏览器 profile 名称
}

// handleBrowserProxy 处理 browser.proxy 命令。
func (s *NodeHostService) handleBrowserProxy(frame *NodeInvokeRequest) {
	if s.browserProxy == nil || !s.browserProxyConfig.Enabled {
		s.sendInvokeResult(frame, false, "", &InvokeErrorShape{
			Code: "UNAVAILABLE", Message: "node browser proxy disabled",
		})
		return
	}

	var params BrowserProxyParams
	if err := DecodeParams(frame.ParamsJSON, &params); err != nil {
		s.sendInvokeResult(frame, false, "", &InvokeErrorShape{
			Code: "INVALID_REQUEST", Message: err.Error(),
		})
		return
	}

	pathValue := strings.TrimSpace(params.Path)
	if pathValue == "" {
		s.sendInvokeResult(frame, false, "", &InvokeErrorShape{
			Code: "INVALID_REQUEST", Message: "path required",
		})
		return
	}

	// Profile 白名单检查
	allowProfiles := s.browserProxyConfig.AllowProfiles
	requestedProfile := ""
	if params.Profile != nil {
		requestedProfile = strings.TrimSpace(*params.Profile)
	}
	if len(allowProfiles) > 0 {
		if pathValue != "/profiles" {
			profileToCheck := requestedProfile
			if profileToCheck == "" {
				profileToCheck = s.browserProxyConfig.DefaultProfile
			}
			if !isProfileAllowed(allowProfiles, profileToCheck) {
				s.sendInvokeResult(frame, false, "", &InvokeErrorShape{
					Code: "INVALID_REQUEST", Message: "browser profile not allowed",
				})
				return
			}
		} else if requestedProfile != "" {
			if !isProfileAllowed(allowProfiles, requestedProfile) {
				s.sendInvokeResult(frame, false, "", &InvokeErrorShape{
					Code: "INVALID_REQUEST", Message: "browser profile not allowed",
				})
				return
			}
		}
	}

	// 构造请求
	method := "GET"
	if params.Method != "" {
		method = strings.ToUpper(strings.TrimSpace(params.Method))
	}
	if !strings.HasPrefix(pathValue, "/") {
		pathValue = "/" + pathValue
	}

	query := make(map[string]interface{})
	if requestedProfile != "" {
		query["profile"] = requestedProfile
	}
	if params.Query != nil {
		for k, v := range params.Query {
			if v == nil {
				continue
			}
			query[k] = v
		}
	}

	// 超时
	timeoutMs := 30000 // 默认 30 秒
	if params.TimeoutMs != nil && *params.TimeoutMs > 0 {
		timeoutMs = *params.TimeoutMs
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	status, result, err := s.browserProxy.Dispatch(ctx, method, pathValue, query, params.Body)
	if err != nil {
		s.sendInvokeResult(frame, false, "", &InvokeErrorShape{
			Code: "INVALID_REQUEST", Message: err.Error(),
		})
		return
	}

	if status >= 400 {
		msg := fmt.Sprintf("HTTP %d", status)
		if resultMap, ok := result.(map[string]interface{}); ok {
			if errMsg, ok := resultMap["error"].(string); ok {
				msg = errMsg
			}
		}
		s.sendInvokeResult(frame, false, "", &InvokeErrorShape{
			Code: "INVALID_REQUEST", Message: msg,
		})
		return
	}

	// 过滤 /profiles 响应中的非白名单 profile
	if len(allowProfiles) > 0 && pathValue == "/profiles" {
		result = filterProfilesResult(result, allowProfiles)
	}

	// 收集文件路径
	filePaths := collectBrowserProxyPaths(result)
	var files []*BrowserProxyFile
	for _, fp := range filePaths {
		f, ferr := readBrowserProxyFile(fp)
		if ferr != nil {
			s.sendInvokeResult(frame, false, "", &InvokeErrorShape{
				Code: "INVALID_REQUEST", Message: fmt.Sprintf("browser proxy file read failed for %s: %v", fp, ferr),
			})
			return
		}
		if f != nil {
			files = append(files, f)
		}
	}

	proxyResult := &BrowserProxyResult{Result: result}
	if len(files) > 0 {
		proxyResult.Files = files
	}
	data, _ := json.Marshal(proxyResult)
	s.sendInvokeResult(frame, true, string(data), nil)
}

// isProfileAllowed 检查 profile 是否在白名单中。
func isProfileAllowed(allowProfiles []string, profile string) bool {
	if len(allowProfiles) == 0 {
		return true
	}
	if profile == "" {
		return false
	}
	trimmed := strings.TrimSpace(profile)
	for _, p := range allowProfiles {
		if strings.TrimSpace(p) == trimmed {
			return true
		}
	}
	return false
}

// collectBrowserProxyPaths 从响应中收集文件路径。
func collectBrowserProxyPaths(payload interface{}) []string {
	obj, ok := payload.(map[string]interface{})
	if !ok || obj == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var paths []string
	addPath := func(key string) {
		if v, ok := obj[key].(string); ok {
			v = strings.TrimSpace(v)
			if v != "" {
				if _, exists := seen[v]; !exists {
					seen[v] = struct{}{}
					paths = append(paths, v)
				}
			}
		}
	}
	addPath("path")
	addPath("imagePath")

	if dl, ok := obj["download"].(map[string]interface{}); ok {
		if v, ok := dl["path"].(string); ok {
			v = strings.TrimSpace(v)
			if v != "" {
				if _, exists := seen[v]; !exists {
					seen[v] = struct{}{}
					paths = append(paths, v)
				}
			}
		}
	}
	return paths
}

// readBrowserProxyFile 读取文件并编码为 base64。
func readBrowserProxyFile(filePath string) (*BrowserProxyFile, error) {
	info, err := os.Stat(filePath)
	if err != nil || info.IsDir() {
		return nil, nil
	}
	if info.Size() > BrowserProxyMaxFileBytes {
		return nil, fmt.Errorf("browser proxy file exceeds %dMB", BrowserProxyMaxFileBytes/(1024*1024))
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	mime := detectMimeByExtension(filePath)
	return &BrowserProxyFile{
		Path:     filePath,
		Base64:   b64,
		MimeType: mime,
	}, nil
}

// detectMimeByExtension 根据文件扩展名推断 MIME 类型。
func detectMimeByExtension(filePath string) string {
	lower := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".json"):
		return "application/json"
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		return "text/html"
	case strings.HasSuffix(lower, ".txt"):
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}

// filterProfilesResult 过滤 /profiles 响应中的非白名单 profile。
func filterProfilesResult(result interface{}, allowProfiles []string) interface{} {
	obj, ok := result.(map[string]interface{})
	if !ok {
		return result
	}
	profiles, ok := obj["profiles"].([]interface{})
	if !ok {
		return result
	}
	filtered := make([]interface{}, 0, len(profiles))
	for _, entry := range profiles {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := entryMap["name"].(string)
		if !ok {
			continue
		}
		if isProfileAllowed(allowProfiles, name) {
			filtered = append(filtered, entry)
		}
	}
	obj["profiles"] = filtered
	return obj
}
