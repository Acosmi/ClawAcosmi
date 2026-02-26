// tools/image_tool.go — 图片工具。
// TS 参考：src/agents/tools/image-tool.ts (449L) + image-tool.helpers.ts (88L)
package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// ImageAction 图片操作类型。
type ImageAction string

const (
	ImageActionGenerate ImageAction = "generate"
	ImageActionEdit     ImageAction = "edit"
	ImageActionAnalyze  ImageAction = "analyze"
	ImageActionResize   ImageAction = "resize"
	ImageActionConvert  ImageAction = "convert"
)

// ImageProvider 图片生成/编辑 provider 接口。
type ImageProvider interface {
	GenerateImage(ctx context.Context, prompt string, opts ImageGenerateOpts) ([]byte, string, error)
	EditImage(ctx context.Context, imageData []byte, prompt string, opts ImageEditOpts) ([]byte, string, error)
}

// ImageGenerateOpts 图片生成选项。
type ImageGenerateOpts struct {
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Format string `json:"format,omitempty"`
}

// ImageEditOpts 图片编辑选项。
type ImageEditOpts struct {
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
	Format string `json:"format,omitempty"`
}

// CreateImageTool 创建图片工具。
// TS 参考: image-tool.ts
func CreateImageTool(provider ImageProvider, workspaceDir string) *AgentTool {
	return &AgentTool{
		Name:        "image",
		Label:       "Image",
		Description: "Generate, edit, analyze, resize, or convert images.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []any{"generate", "edit", "analyze", "resize", "convert"},
					"description": "The image action to perform",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "Text prompt for generate/edit actions",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the source image (for edit/analyze/resize/convert)",
				},
				"output_path": map[string]any{
					"type":        "string",
					"description": "Output path for the result image",
				},
				"width": map[string]any{
					"type":        "number",
					"description": "Target width for resize",
				},
				"height": map[string]any{
					"type":        "number",
					"description": "Target height for resize",
				},
				"format": map[string]any{
					"type":        "string",
					"enum":        []any{"png", "jpeg", "webp", "gif"},
					"description": "Target format for convert",
				},
			},
			"required": []any{"action"},
		},
		Execute: func(ctx context.Context, toolCallID string, args map[string]any) (*AgentToolResult, error) {
			action, err := ReadStringParam(args, "action", &StringParamOptions{Required: true})
			if err != nil {
				return nil, err
			}

			switch ImageAction(action) {
			case ImageActionGenerate:
				return executeImageGenerate(provider, workspaceDir, args)
			case ImageActionEdit:
				return executeImageEdit(provider, workspaceDir, args)
			case ImageActionAnalyze:
				return executeImageAnalyze(args)
			case ImageActionResize:
				return executeImageResize(args)
			case ImageActionConvert:
				return executeImageConvert(args)
			default:
				return nil, fmt.Errorf("unknown image action: %s", action)
			}
		},
	}
}

func executeImageGenerate(provider ImageProvider, workspaceDir string, args map[string]any) (*AgentToolResult, error) {
	prompt, err := ReadStringParam(args, "prompt", &StringParamOptions{Required: true})
	if err != nil {
		return nil, err
	}

	width, _, _ := ReadNumberParam(args, "width", &NumberParamOptions{Integer: true})
	height, _, _ := ReadNumberParam(args, "height", &NumberParamOptions{Integer: true})
	format, _ := ReadStringParam(args, "format", nil)

	if provider == nil {
		return nil, fmt.Errorf("image provider not configured")
	}

	data, mimeType, err := provider.GenerateImage(context.Background(), prompt, ImageGenerateOpts{
		Width:  int(width),
		Height: int(height),
		Format: format,
	})
	if err != nil {
		return nil, fmt.Errorf("image generation failed: %w", err)
	}

	// 保存到文件
	outputPath, _ := ReadStringParam(args, "output_path", nil)
	if outputPath == "" {
		ext := ".png"
		if strings.Contains(mimeType, "jpeg") {
			ext = ".jpg"
		} else if strings.Contains(mimeType, "webp") {
			ext = ".webp"
		}
		outputPath = filepath.Join(workspaceDir, "generated_image"+ext)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("save image: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	return ImageResult("Generated Image", outputPath, b64, mimeType, "", nil), nil
}

func executeImageEdit(provider ImageProvider, workspaceDir string, args map[string]any) (*AgentToolResult, error) {
	path, err := ReadStringParam(args, "path", &StringParamOptions{Required: true})
	if err != nil {
		return nil, err
	}
	prompt, err := ReadStringParam(args, "prompt", &StringParamOptions{Required: true})
	if err != nil {
		return nil, err
	}

	imageData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source image: %w", err)
	}

	if provider == nil {
		return nil, fmt.Errorf("image provider not configured")
	}

	data, mimeType, err := provider.EditImage(context.Background(), imageData, prompt, ImageEditOpts{})
	if err != nil {
		return nil, fmt.Errorf("image edit failed: %w", err)
	}

	outputPath, _ := ReadStringParam(args, "output_path", nil)
	if outputPath == "" {
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(filepath.Base(path), ext)
		outputPath = filepath.Join(filepath.Dir(path), base+"_edited"+ext)
	}

	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return nil, fmt.Errorf("save edited image: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	return ImageResult("Edited Image", outputPath, b64, mimeType, "", nil), nil
}

func executeImageAnalyze(args map[string]any) (*AgentToolResult, error) {
	path, err := ReadStringParam(args, "path", &StringParamOptions{Required: true})
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read image: %w", err)
	}

	mimeType := detectMimeFromBytes(data)

	return JsonResult(map[string]any{
		"path":     path,
		"size":     info.Size(),
		"mimeType": mimeType,
		"ext":      filepath.Ext(path),
	}), nil
}

func executeImageResize(args map[string]any) (*AgentToolResult, error) {
	path, err := ReadStringParam(args, "path", &StringParamOptions{Required: true})
	if err != nil {
		return nil, err
	}

	width, hasWidth, _ := ReadNumberParam(args, "width", &NumberParamOptions{Integer: true})
	height, hasHeight, _ := ReadNumberParam(args, "height", &NumberParamOptions{Integer: true})
	if !hasWidth && !hasHeight {
		return nil, fmt.Errorf("at least one of width or height is required for resize")
	}

	// 读取源图像
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source image: %w", err)
	}

	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	origBounds := src.Bounds()
	origW := origBounds.Dx()
	origH := origBounds.Dy()

	// 计算目标尺寸（保持宽高比）
	targetW := int(width)
	targetH := int(height)
	if targetW <= 0 && targetH > 0 {
		targetW = origW * targetH / origH
	} else if targetH <= 0 && targetW > 0 {
		targetH = origH * targetW / origW
	}
	if targetW <= 0 || targetH <= 0 {
		return nil, fmt.Errorf("invalid target dimensions: %dx%d", targetW, targetH)
	}

	// 使用 Lanczos3 高质量缩放
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, origBounds, xdraw.Over, nil)

	// 输出路径
	outputPath, _ := ReadStringParam(args, "output_path", nil)
	if outputPath == "" {
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(filepath.Base(path), ext)
		outputPath = filepath.Join(filepath.Dir(path), base+"_resized"+ext)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	// 编码输出
	outExt := strings.ToLower(filepath.Ext(outputPath))
	var buf bytes.Buffer
	switch outExt {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 90})
	case ".png":
		err = png.Encode(&buf, dst)
	default:
		// 默认 PNG
		err = png.Encode(&buf, dst)
	}
	if err != nil {
		return nil, fmt.Errorf("encode resized image: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("save resized image: %w", err)
	}

	return JsonResult(map[string]any{
		"path":       outputPath,
		"origWidth":  origW,
		"origHeight": origH,
		"width":      targetW,
		"height":     targetH,
		"size":       buf.Len(),
	}), nil
}

func executeImageConvert(args map[string]any) (*AgentToolResult, error) {
	path, err := ReadStringParam(args, "path", &StringParamOptions{Required: true})
	if err != nil {
		return nil, err
	}
	format, err := ReadStringParam(args, "format", &StringParamOptions{Required: true})
	if err != nil {
		return nil, err
	}

	// 读取源图像
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read source image: %w", err)
	}

	src, srcFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}

	// 输出路径
	outputPath, _ := ReadStringParam(args, "output_path", nil)
	if outputPath == "" {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		ext := "." + format
		if format == "jpeg" {
			ext = ".jpg"
		}
		outputPath = filepath.Join(filepath.Dir(path), base+ext)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	// 编码到目标格式
	var buf bytes.Buffer
	switch strings.ToLower(format) {
	case "png":
		err = png.Encode(&buf, src)
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90})
	default:
		return nil, fmt.Errorf("unsupported target format: %s (supported: png, jpeg)", format)
	}
	if err != nil {
		return nil, fmt.Errorf("encode to %s: %w", format, err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return nil, fmt.Errorf("save converted image: %w", err)
	}

	return JsonResult(map[string]any{
		"path":      outputPath,
		"srcFormat": srcFormat,
		"dstFormat": format,
		"size":      buf.Len(),
	}), nil
}
