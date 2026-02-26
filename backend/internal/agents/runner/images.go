package runner

// ============================================================================
// 图片引用检测与加载
// 对应 TS: pi-embedded-runner/run/images.ts (448L)
// ============================================================================

import (
	"encoding/base64"
	"log/slog"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ---------- 类型定义 ----------

// ImageRefType 图片引用类型。
type ImageRefType string

const (
	ImageRefPath ImageRefType = "path"
	ImageRefURL  ImageRefType = "url"
)

// DetectedImageRef 检测到的图片引用。
// TS 对照: images.ts → DetectedImageRef
type DetectedImageRef struct {
	Raw          string       `json:"raw"`
	Type         ImageRefType `json:"type"`
	Resolved     string       `json:"resolved"`
	MessageIndex int          `json:"messageIndex,omitempty"` // -1 表示无关联
}

// ImageContent 加载后的图片内容。
// TS 对照: images.ts → ImageContent
type ImageContent struct {
	Type     string `json:"type"`     // "image"
	Data     string `json:"data"`     // base64 编码数据
	MimeType string `json:"mimeType"` // e.g. "image/png"
}

// ---------- 图片扩展名 ----------

// imageExtensions 支持的图片文件扩展名。
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".bmp": true, ".ico": true, ".svg": true,
	".tiff": true, ".tif": true, ".avif": true, ".heic": true, ".heif": true,
}

// isImageExtension 检查路径是否为图片扩展名。
func isImageExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return imageExtensions[ext]
}

// ---------- 正则模式 ----------

// mediaAttachedRE [media attached: path (type) | url] or [media attached N/M: path (type) | url]
var mediaAttachedRE = regexp.MustCompile(`\[media attached(?:\s+\d+/\d+)?:\s*([^\]]+)\]`)

// nFilesHeaderRE 跳过 "N files" 汇总头行
var nFilesHeaderRE = regexp.MustCompile(`^\d+\s+files?$`)

// imageSourceRE [Image: source: /path/...]
var imageSourceRE = regexp.MustCompile(`\[Image:\s*source:\s*(.+?)\]`)

// fileURLRE file:///path/... (也匹配 file://localhost/path)
var fileURLRE = regexp.MustCompile(`file://[^\s<>"'` + "`" + `\]]+\.(?:png|jpe?g|gif|webp|bmp|tiff?|heic|heif)`)

// absolutePathRE 绝对路径 (以 / 开头的图片文件)
var absolutePathRE = regexp.MustCompile(`(?:^|\s)(/[^\s"'\]>]+\.(?:png|jpg|jpeg|gif|webp|bmp|ico|svg|tiff|tif|avif|heic|heif))(?:\s|$)`)

// tildePathRE ~/path... (用户主目录相对路径)
var tildePathRE = regexp.MustCompile(`(?:^|\s)(~/[^\s"'\]>]+\.(?:png|jpg|jpeg|gif|webp|bmp|ico|svg|tiff|tif|avif|heic|heif))(?:\s|$)`)

// ---------- 图片引用检测 ----------

// DetectImageReferences 从文本中检测图片引用。
// TS 对照: images.ts → detectImageReferences()
func DetectImageReferences(prompt string) []DetectedImageRef {
	var refs []DetectedImageRef
	seen := map[string]bool{}

	addPathRef := func(raw, resolved string) {
		if resolved == "" || seen[resolved] || !isImageExtension(resolved) {
			return
		}
		seen[resolved] = true
		refs = append(refs, DetectedImageRef{
			Raw:          raw,
			Type:         ImageRefPath,
			Resolved:     resolved,
			MessageIndex: -1,
		})
	}

	// 1. [media attached: path ...]
	for _, match := range mediaAttachedRE.FindAllStringSubmatch(prompt, -1) {
		if len(match) > 1 {
			content := strings.TrimSpace(match[1])
			// 跳过 "N files" 汇总头行
			if nFilesHeaderRE.MatchString(content) {
				continue
			}
			// 提取路径: 在 (mime/type) 或 | 分隔符之前
			p := extractMediaAttachedPath(content)
			if p != "" {
				addPathRef(match[0], p)
			}
		}
	}

	// 2. [Image: source: /path/...]
	for _, match := range imageSourceRE.FindAllStringSubmatch(prompt, -1) {
		if len(match) > 1 {
			p := strings.TrimSpace(match[1])
			addPathRef(match[0], p)
		}
	}

	// 3. file:///path/... or file://localhost/path/...
	for _, match := range fileURLRE.FindAllString(prompt, -1) {
		raw := strings.TrimSpace(match)
		if seen[strings.ToLower(raw)] {
			continue
		}
		resolved := fileURLToPath(raw)
		if resolved != "" {
			addPathRef(raw, resolved)
		}
	}

	// 4. 绝对路径
	for _, match := range absolutePathRE.FindAllStringSubmatch(prompt, -1) {
		if len(match) > 1 {
			addPathRef(match[1], strings.TrimSpace(match[1]))
		}
	}

	// 5. ~/path
	for _, match := range tildePathRE.FindAllStringSubmatch(prompt, -1) {
		if len(match) > 1 {
			raw := strings.TrimSpace(match[1])
			resolved := resolveHomePath(raw)
			addPathRef(raw, resolved)
		}
	}

	return refs
}

// resolveHomePath 将 ~/path 解析为绝对路径。
func resolveHomePath(p string) string {
	if !strings.HasPrefix(p, "~/") {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[2:])
}

// mediaAttachedPathRE 提取 media attached 中的文件路径（在 (mime/type) 或 | 分隔符之前）
var mediaAttachedPathRE = regexp.MustCompile(`^\s*(.+?\.(?:png|jpe?g|gif|webp|bmp|tiff?|heic|heif))\s*(?:\(|$|\|)`)

// extractMediaAttachedPath 从 media attached 内容中提取文件路径。
// TS 对照: images.ts L112-117 pathMatch 逻辑
func extractMediaAttachedPath(content string) string {
	m := mediaAttachedPathRE.FindStringSubmatch(content)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// fileURLToPath 将 file:// URL 转换为本地路径。
// 支持 file:///path 和 file://localhost/path 格式。
// TS 对照: images.ts L141-146 fileURLToPath()
func fileURLToPath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	p := parsed.Path
	if decoded, err := url.PathUnescape(p); err == nil {
		p = decoded
	}
	return p
}

// ---------- 图片加载 ----------

// LoadImageFromRefOptions 加载图片的选项。
type LoadImageFromRefOptions struct {
	MaxBytes    int64  // 最大文件大小（字节），0 表示不限
	SandboxRoot string // 沙箱根目录，空表示不限
}

// LoadImageFromRef 从引用加载图片内容。
// TS 对照: images.ts → loadImageFromRef()
func LoadImageFromRef(ref DetectedImageRef, workspaceDir string, opts *LoadImageFromRefOptions) *ImageContent {
	// Remote URL → 拒绝 (本地专用)
	// TS 对照: images.ts L188-191
	if ref.Type == ImageRefURL {
		slog.Debug("image: rejecting remote URL (local-only)", "resolved", ref.Resolved)
		return nil
	}

	targetPath := ref.Resolved

	// 相对路径解析 — sandbox 优先，否则 workspaceDir
	if !filepath.IsAbs(targetPath) {
		resolveRoot := workspaceDir
		if opts != nil && opts.SandboxRoot != "" {
			resolveRoot = opts.SandboxRoot
		}
		targetPath = filepath.Join(resolveRoot, targetPath)
	}

	// 沙箱校验
	if opts != nil && opts.SandboxRoot != "" {
		absTarget, err := filepath.Abs(targetPath)
		if err != nil {
			return nil
		}
		absRoot, err := filepath.Abs(opts.SandboxRoot)
		if err != nil {
			return nil
		}
		if !strings.HasPrefix(absTarget, absRoot+string(filepath.Separator)) && absTarget != absRoot {
			slog.Debug("image: path outside sandbox", "path", targetPath, "sandbox", opts.SandboxRoot)
			return nil
		}
	}

	// 文件存在性检查
	info, err := os.Stat(targetPath)
	if err != nil {
		slog.Debug("image: file not found", "path", targetPath)
		return nil
	}

	// 大小检查
	if opts != nil && opts.MaxBytes > 0 && info.Size() > opts.MaxBytes {
		slog.Debug("image: file too large", "path", targetPath, "size", info.Size(), "max", opts.MaxBytes)
		return nil
	}

	// 读取文件
	data, err := os.ReadFile(targetPath)
	if err != nil {
		slog.Debug("image: read failed", "path", targetPath, "err", err)
		return nil
	}

	// MIME 类型检测
	ext := filepath.Ext(targetPath)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "image/jpeg" // 默认
	}

	return &ImageContent{
		Type:     "image",
		Data:     base64.StdEncoding.EncodeToString(data),
		MimeType: mimeType,
	}
}

// ---------- 模型能力检测 ----------

// ModelSupportsImages 检查模型是否支持图片输入。
// TS 对照: images.ts → modelSupportsImages()
func ModelSupportsImages(supportedInputTypes []string) bool {
	for _, t := range supportedInputTypes {
		if t == "image" {
			return true
		}
	}
	return false
}

// ---------- 历史消息图片检测 ----------

// messageHasImageContent 检查消息是否已包含 image content block。
// TS 对照: images.ts L279-291
func messageHasImageContent(msg HistoryMessage) bool {
	for _, block := range msg.Content {
		if m, ok := block.(map[string]interface{}); ok {
			if t, ok := m["type"].(string); ok && t == "image" {
				return true
			}
		}
	}
	return false
}

// DetectImagesFromHistory 从仅 user 消息中检测图片引用。
// 跳过已包含 image content 的消息（防止每轮重复加载）。
// 返回带有 MessageIndex 字段的引用列表。
// TS 对照: images.ts → detectImagesFromHistory()
func DetectImagesFromHistory(messages []HistoryMessage) []DetectedImageRef {
	var refs []DetectedImageRef
	seen := map[string]bool{}

	for i, msg := range messages {
		if msg.Role != "user" {
			continue
		}
		// 跳过已有 image content 的消息（TS L303-306）
		if messageHasImageContent(msg) {
			continue
		}
		text := ExtractTextFromHistoryMessage(msg)
		if text == "" {
			continue
		}
		detected := DetectImageReferences(text)
		for _, ref := range detected {
			if seen[ref.Resolved] {
				continue
			}
			seen[ref.Resolved] = true
			ref.MessageIndex = i
			refs = append(refs, ref)
		}
	}

	return refs
}

// HistoryMessage 历史消息的简化表示。
type HistoryMessage struct {
	Role    string        `json:"role"`
	Text    string        `json:"text"`
	Content []interface{} `json:"content,omitempty"` // 支持 array content blocks
}

// ExtractTextFromHistoryMessage 从历史消息中提取文本内容。
// 支持 string text 和 array content blocks。
// TS 对照: tui-formatters.ts → extractTextFromMessage()
func ExtractTextFromHistoryMessage(msg HistoryMessage) string {
	if msg.Text != "" {
		return msg.Text
	}
	// 从 content blocks 中提取：遍历 array 找 text 类型
	var parts []string
	for _, block := range msg.Content {
		if m, ok := block.(map[string]interface{}); ok {
			if t, ok := m["type"].(string); ok && t == "text" {
				if text, ok := m["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

// ---------- 图片后处理 ----------

// SanitizeLoadedImages 对加载的图片进行后处理校验。
// 移除无效项（空数据、不支持的 MIME 类型）。
// TS 对照: images.ts → sanitizeImagesWithLog() → sanitizeImageBlocks()
func SanitizeLoadedImages(images []ImageContent, label string) []ImageContent {
	var result []ImageContent
	dropped := 0
	for _, img := range images {
		if img.Data == "" || img.MimeType == "" {
			dropped++
			continue
		}
		result = append(result, img)
	}
	if dropped > 0 {
		slog.Warn("image: dropped images after sanitization",
			"dropped", dropped, "label", label)
	}
	return result
}

// ---------- 主入口 ----------

// DetectAndLoadPromptImagesParams 检测并加载图片的参数。
type DetectAndLoadPromptImagesParams struct {
	Prompt       string
	History      []HistoryMessage
	WorkspaceDir string
	SandboxRoot  string
	MaxBytes     int64
}

// DetectAndLoadPromptImagesResult 检测并加载图片的结果。
type DetectAndLoadPromptImagesResult struct {
	Images []ImageContent
	Refs   []DetectedImageRef
}

// DetectAndLoadPromptImages 从 prompt 和 history 中检测并加载图片。
// TS 对照: images.ts → detectAndLoadPromptImages()
func DetectAndLoadPromptImages(params DetectAndLoadPromptImagesParams) DetectAndLoadPromptImagesResult {
	var allRefs []DetectedImageRef
	seen := map[string]bool{}

	// 从 prompt 检测
	promptRefs := DetectImageReferences(params.Prompt)
	for _, ref := range promptRefs {
		if !seen[ref.Resolved] {
			seen[ref.Resolved] = true
			allRefs = append(allRefs, ref)
		}
	}

	// 从 history 检测
	historyRefs := DetectImagesFromHistory(params.History)
	for _, ref := range historyRefs {
		if !seen[ref.Resolved] {
			seen[ref.Resolved] = true
			allRefs = append(allRefs, ref)
		}
	}

	// 加载
	opts := &LoadImageFromRefOptions{
		MaxBytes:    params.MaxBytes,
		SandboxRoot: params.SandboxRoot,
	}
	var images []ImageContent
	for _, ref := range allRefs {
		img := LoadImageFromRef(ref, params.WorkspaceDir, opts)
		if img != nil {
			images = append(images, *img)
		}
	}

	return DetectAndLoadPromptImagesResult{
		Images: SanitizeLoadedImages(images, "prompt"),
		Refs:   allRefs,
	}
}
