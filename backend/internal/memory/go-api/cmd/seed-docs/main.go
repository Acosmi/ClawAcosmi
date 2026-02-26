package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/uhms/go-api/internal/config"
	"github.com/uhms/go-api/internal/database"
	"github.com/uhms/go-api/internal/models"
)

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func ptr(s string) *string { return &s }

func main() {
	cfg := config.Load()
	if err := database.Init(cfg); err != nil {
		log.Fatal("DB init failed:", err)
	}
	db, _ := database.GetDB()
	db.AutoMigrate(&models.DocEndpoint{})

	docs := []models.DocEndpoint{
		// ── MCP ──
		{
			ID: "mcp-overview", Category: "mcp", Name: "MCP 协议概述",
			Method: "CONCEPT", Path: "/api/v1/mcp", IsPublic: true, SortOrder: 0,
			DisplayLabel: ptr("MCP 协议概述 / MCP Overview"),
			Description:  "Model Context Protocol 让 AI 助手直接访问 OpenAethel 记忆系统。支持 stdio / HTTP / Agent 隧道三种接入方式，共 10 个 MCP 工具。",
			CodeExamples: mustJSON(map[string]string{
				"curl":       "# Claude Desktop 配置\n{\"mcpServers\":{\"openaethel\":{\"command\":\"/path/to/go-api\",\"args\":[\"mcp\"]}}}",
				"python":     "from openaethel import OpenAethelClient\nclient = OpenAethelClient('key')\nresp = client.send_mcp_message({'method':'tools/list'})",
				"javascript": "const r = await fetch('/api/v1/mcp',{method:'POST',headers:{'Authorization':'Bearer key'},body:JSON.stringify({method:'tools/list'})});",
			}),
		},
		{
			ID: "mcp-send-message", Category: "mcp", Name: "Send MCP Message",
			Method: "POST", Path: "/api/v1/mcp", IsPublic: true, SortOrder: 1,
			DisplayLabel: ptr("发送 MCP 消息 / Send MCP Message"),
			Description:  "发送 MCP 消息到 OpenAethel Memory Server。支持 OAuth 2.1 Bearer Token 认证。",
			Parameters: mustJSON([]map[string]any{
				{"name": "method", "type": "string", "required": true, "description": "MCP 方法名"},
				{"name": "params", "type": "object", "required": false, "description": "方法参数"},
			}),
			CodeExamples: mustJSON(map[string]string{
				"curl":       "curl -X POST /api/v1/mcp -H 'Authorization: Bearer key' -d '{\"method\":\"tools/call\",\"params\":{\"name\":\"recall_memory\"}}'",
				"python":     "client.send_mcp_message({'method':'tools/call','params':{'name':'recall_memory','arguments':{'query':'偏好'}}})",
				"javascript": "await fetch('/api/v1/mcp',{method:'POST',body:JSON.stringify({method:'tools/call',params:{name:'recall_memory'}})})",
			}),
		},
		{
			ID: "agent-tunnel", Category: "mcp", Name: "Agent Tunnel",
			Method: "GET", Path: "/api/v1/agent/ws", IsPublic: true, SortOrder: 2,
			DisplayLabel: ptr("Agent 隧道 / Agent Tunnel"),
			Description:  "WebSocket 反向隧道，本地 Agent 连接云端。30s 心跳保活，断线指数退避重连。",
			Parameters: mustJSON([]map[string]any{
				{"name": "X-Agent-Token", "type": "header", "required": true, "description": "Agent 认证 Token"},
				{"name": "X-Agent-Name", "type": "header", "required": false, "description": "Agent 名称"},
			}),
			CodeExamples: mustJSON(map[string]string{
				"curl":       "curl /api/v1/agent/list -H 'Authorization: Bearer dev_admin_user'",
				"python":     "# websockets 连接\nasync with websockets.connect(uri, extra_headers={'X-Agent-Token':'token'}) as ws: ...",
				"javascript": "const ws = new WebSocket('ws://localhost:8006/api/v1/agent/ws',{headers:{'X-Agent-Token':'token'}});",
			}),
		},
		// ── SDK ──
		{
			ID: "sdk-quickstart", Category: "sdk", Name: "SDK 快速上手",
			Method: "CONCEPT", Path: "https://github.com/openaethel/sdk-go", IsPublic: true, SortOrder: 0,
			DisplayLabel: ptr("SDK 快速上手 / SDK Quickstart"),
			Description:  "Go 和 Python 官方 SDK，3 行代码接入。Go 27/27 测试通过，Python 24/24 测试通过，100% 路由覆盖。",
			CodeExamples: mustJSON(map[string]string{
				"curl":       "go get github.com/openaethel/sdk-go\npip install openaethel",
				"python":     "from openaethel import OpenAethelClient\nclient = OpenAethelClient('key')\nmem = client.add('内容', user_id='u1')",
				"javascript": "import openaethel \"github.com/openaethel/sdk-go\"\nclient := openaethel.NewClient(\"key\")\nmem, _ := client.AddMemory(ctx, input)",
			}),
		},
		{
			ID: "sdk-go-reference", Category: "sdk", Name: "Go SDK API",
			Method: "CONCEPT", Path: "https://pkg.go.dev/github.com/openaethel/sdk-go", IsPublic: true, SortOrder: 1,
			DisplayLabel: ptr("Go SDK API 参考 / Go SDK Reference"),
			Description:  "Go SDK 20 个方法，涵盖 Memory、Graph、MCP 三大模块。零外部依赖，强类型。",
			CodeExamples: mustJSON(map[string]string{
				"curl":       "go get github.com/openaethel/sdk-go",
				"python":     "# 参见 Python SDK",
				"javascript": "client := openaethel.NewClient(\"key\")\nresults, _ := client.SearchMemories(ctx, input)",
			}),
		},
		{
			ID: "sdk-python-reference", Category: "sdk", Name: "Python SDK API",
			Method: "CONCEPT", Path: "https://pypi.org/project/openaethel/", IsPublic: true, SortOrder: 2,
			DisplayLabel: ptr("Python SDK API 参考 / Python SDK Reference"),
			Description:  "Python SDK 同步+异步双客户端，完整 type hints。OpenAethelClient 和 AsyncOpenAethelClient。",
			CodeExamples: mustJSON(map[string]string{
				"curl":       "pip install openaethel",
				"python":     "from openaethel import OpenAethelClient\nclient = OpenAethelClient('key', base_url='http://localhost:8006/api/v1')\nresults = client.search('query', user_id='u1')",
				"javascript": "# 异步用法\nasync with AsyncOpenAethelClient('key') as client:\n    await client.search('query')",
			}),
		},
	}

	for _, d := range docs {
		result := db.Where("id = ?", d.ID).Assign(d).FirstOrCreate(&models.DocEndpoint{})
		if result.Error != nil {
			fmt.Printf("❌ %s: %v\n", d.ID, result.Error)
		} else {
			fmt.Printf("✅ %s (category: %s)\n", d.ID, d.Category)
		}
	}

	var count int64
	db.Model(&models.DocEndpoint{}).Count(&count)
	fmt.Printf("\n📊 doc_endpoints 总计: %d 条\n", count)
	os.Exit(0)
}
