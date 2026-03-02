package gateway

// server_methods_docconv.go — 文档转换（DocConv）RPC 方法（Phase D 新增）
// 提供 docconv.config.get / docconv.config.set / docconv.test / docconv.formats 方法
// 纯新增文件，不修改任何已有方法

import (
	"context"
	"encoding/json"
	"time"

	"github.com/openacosmi/claw-acismi/internal/media"
	"github.com/openacosmi/claw-acismi/pkg/types"
)

// DocConvHandlers 返回文档转换 RPC 方法处理器。
func DocConvHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"docconv.config.get": handleDocConvConfigGet,
		"docconv.config.set": handleDocConvConfigSet,
		"docconv.test":       handleDocConvTest,
		"docconv.formats":    handleDocConvFormats,
	}
}

// ---------- docconv.config.get ----------

// DocConvConfigGetResult docconv.config.get 响应
type DocConvConfigGetResult struct {
	Configured    bool                  `json:"configured"`
	Provider      string                `json:"provider,omitempty"`
	MCPServerName string                `json:"mcpServerName,omitempty"`
	MCPTransport  string                `json:"mcpTransport,omitempty"`
	MCPCommand    string                `json:"mcpCommand,omitempty"`
	MCPURL        string                `json:"mcpUrl,omitempty"`
	PandocPath    string                `json:"pandocPath,omitempty"`
	Providers     []DocConvProviderInfo `json:"providers"`
	MCPPresets    []DocConvMCPPreset    `json:"mcpPresets"`
}

// DocConvProviderInfo 可选 DocConv Provider 描述
type DocConvProviderInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
}

// DocConvMCPPreset MCP Server 预设
type DocConvMCPPreset struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Command   string `json:"command"`
	Transport string `json:"transport"`
	Hint      string `json:"hint,omitempty"`
}

func handleDocConvConfigGet(ctx *MethodHandlerContext) {
	result := DocConvConfigGetResult{
		Providers: []DocConvProviderInfo{
			{ID: "mcp", Label: "MCP 工具", Hint: "标准 MCP 协议，支持多种文档转换服务器"},
			{ID: "builtin", Label: "内置", Hint: "pandoc CLI + 文本直读"},
			{ID: "", Label: "禁用", Hint: "不使用文档转换"},
		},
		MCPPresets: []DocConvMCPPreset{
			{
				Name:      "mcp-pandoc",
				Label:     "mcp-pandoc",
				Command:   "npx -y mcp-pandoc",
				Transport: "stdio",
				Hint:      "基于 Pandoc，支持 MD/HTML/PDF/DOCX/LaTeX/TXT",
			},
			{
				Name:      "mcp-document-converter",
				Label:     "mcp-document-converter",
				Command:   "npx -y @xt765/mcp-document-converter",
				Transport: "stdio",
				Hint:      "25 种格式组合，语法高亮，CSS 样式",
			},
			{
				Name:      "doc-ops-mcp",
				Label:     "doc-ops-mcp (Tele-AI)",
				Command:   "npx -y doc-ops-mcp",
				Transport: "stdio",
				Hint:      "智能转换规划，OOXML 解析，水印/二维码",
			},
		},
	}

	cfg := loadDocConvConfigFromCtx(ctx)
	if cfg != nil && cfg.Provider != "" {
		result.Configured = true
		result.Provider = cfg.Provider
		result.MCPServerName = cfg.MCPServerName
		result.MCPTransport = cfg.MCPTransport
		result.MCPCommand = cfg.MCPCommand
		result.MCPURL = cfg.MCPURL
		result.PandocPath = cfg.PandocPath
	}

	ctx.Respond(true, result, nil)
}

// ---------- docconv.config.set ----------

func handleDocConvConfigSet(ctx *MethodHandlerContext) {
	paramsJSON, err := json.Marshal(ctx.Params)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "invalid params"))
		return
	}
	var params types.DocConvConfig
	if err := json.Unmarshal(paramsJSON, &params); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "parse params: "+err.Error()))
		return
	}

	cfgLoader := ctx.Context.ConfigLoader
	if cfgLoader == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config loader not available"))
		return
	}

	currentCfg, err := cfgLoader.LoadConfig()
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "load config: "+err.Error()))
		return
	}
	if currentCfg == nil {
		currentCfg = &types.OpenAcosmiConfig{}
	}

	currentCfg.DocConv = &params

	if err := cfgLoader.WriteConfigFile(currentCfg); err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "save config: "+err.Error()))
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"saved":    true,
		"provider": params.Provider,
	}, nil)
}

// ---------- docconv.test ----------

func handleDocConvTest(ctx *MethodHandlerContext) {
	cfg := loadDocConvConfigFromCtx(ctx)
	if cfg == nil || cfg.Provider == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "DocConv not configured"))
		return
	}

	converter, err := media.NewDocConverter(cfg)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "create converter: "+err.Error()))
		return
	}

	testCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := converter.TestConnection(testCtx); err != nil {
		ctx.Respond(true, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}, nil)
		return
	}

	ctx.Respond(true, map[string]interface{}{
		"success":  true,
		"provider": converter.Name(),
		"formats":  converter.SupportedFormats(),
	}, nil)
}

// ---------- docconv.formats ----------

func handleDocConvFormats(ctx *MethodHandlerContext) {
	all := media.AllSupportedExtensions()
	ctx.Respond(true, map[string]interface{}{
		"formats": all,
	}, nil)
}

// ---------- helpers ----------

func loadDocConvConfigFromCtx(ctx *MethodHandlerContext) *types.DocConvConfig {
	cfgLoader := ctx.Context.ConfigLoader
	if cfgLoader == nil {
		return nil
	}
	cfg, err := cfgLoader.LoadConfig()
	if err != nil || cfg == nil {
		return nil
	}
	return cfg.DocConv
}
