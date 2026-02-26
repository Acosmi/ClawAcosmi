---
document_type: Tracking
status: Audited
created: 2026-02-25
last_updated: 2026-02-25
audit_report: docs/claude/audit/audit-2026-02-25-skill-store-bridge-p1.md
skill5_verified: true
---

# OpenAcosmi ↔ nexus-v4 技能商店桥接

## Context

OpenAcosmi（本地部署）需要连接 nexus-v4（云端）的技能商店，实现：
1. 从云端浏览已审核通过的技能，批量选择拉取到本地
2. 本地新增技能必须跳转到 chat 端提交审核，通过后才能使用
3. 手动刷新发现新技能

**核心约束**: 安全审核（4 重防护）只在 nexus-v4 端执行，OpenAcosmi 信任已审核内容。

---

## MCP vs REST 连接方案对比

### 两端 MCP 现状

| 维度 | OpenAcosmi | nexus-v4 |
|------|-----------|----------|
| MCP 代码量 | ~200 LOC | **4,110+ LOC** |
| 传输协议 | stdio only | Streamable HTTP + SSE + Stdio + Docker |
| 协议版本 | `2024-11-05` | **`2025-11-25`** (最新) |
| 角色 | Client (仅连 Argus 子进程) | Client + Server (完整) |
| OAuth | 无 | **OAuth 2.1 + PKCE** (生产就绪) |
| 连接池 | 无 | MCPProvider 连接池 + 空闲清理 |
| 容器化 | 无 | Docker ContainerManager |
| Rust 桥接 | 无 | chatacosmi-mcp-v2 FFI |
| 前端 | 无 | MCP Wizard + OAuth Callback |

**结论: nexus-v4 拥有完整的 MCP 架构，OpenAcosmi 只有基础 stdio client。**

### 方案对比

| 方案 | 优点 | 缺点 |
|------|------|------|
| **A: MCP 连接** | 标准协议；未来可扩展到远程工具实时执行；复用 nexus-v4 OAuth 2.1 | 需升级 OpenAcosmi MCP client 支持 HTTP 传输；需 nexus-v4 新增 MCP Server 端点；CRUD 操作用 MCP 有些重 |
| **B: REST API** | 简单直接；OpenAcosmi 只需 HTTP client；nexus-v4 已有 REST 端点 | 非标准协议；未来远程工具执行需另建通道 |
| **C: 混合** (推荐) | REST 做技能商店 CRUD；MCP 留给未来远程工具执行 | 两套通信机制 |

### 推荐: 方案 C — 混合模式

**Phase 1 (本次)**: REST API 连接技能商店 — 快速落地
**Phase 2 (未来)**: MCP Streamable HTTP 连接 — 远程工具实时执行

理由:
- 技能商店操作（浏览/下载/刷新）本质是 CRUD，REST 最合适
- 远程工具实时执行（Agent 调用 nexus-v4 上的工具）才是 MCP 真正的用武之地
- nexus-v4 的 MCP OAuth 2.1 基础设施可在 Phase 2 直接复用
- Phase 1 的 onboard 登录获取的 token 可在 Phase 2 续用

---

## 架构设计 (Phase 1: REST)

```
┌──────────────────────────────────────┐
│       nexus-v4 (云端)                 │
│                                      │
│  /api/v4/skill-store     (Browse)    │
│  /api/v4/skill-store/:id (Detail)    │
│  /api/v4/skill-store/:id/download    │
│  /api/v4/skills          (CRUD)      │
│  前端: /skills 页面                   │
│                                      │
│  JWT Auth: Bearer {token}            │
└───────────────┬──────────────────────┘
                │ HTTPS (REST)
                ▼
┌──────────────────────────────────────┐
│    OpenAcosmi Gateway (本地)          │
│                                      │
│  新增 RPC 方法:                       │
│  skills.store.browse  → 代理浏览      │
│  skills.store.pull    → 下载+安装     │
│  skills.store.refresh → 刷新缓存      │
│  skills.store.link    → 返回跳转 URL  │
│                                      │
│  新增: SkillStoreClient (HTTP 客户端) │
│  配置: skills.store.url + token       │
│                                      │
│  落盘: docs/skills/synced/{name}/     │
└──────────────────────────────────────┘
```

### Phase 2 远景 (MCP 远程工具执行)

```
┌──────────────────────────────────────┐
│       nexus-v4 MCP Server (云端)      │
│                                      │
│  Streamable HTTP endpoint            │
│  OAuth 2.1 认证                      │
│  tools: 远程技能工具                   │
│  resources: 技能文档/数据             │
└───────────────┬──────────────────────┘
                │ MCP Streamable HTTP
                ▼
┌──────────────────────────────────────┐
│  OpenAcosmi MCP Client (本地)         │
│                                      │
│  升级 mcpclient: 支持 HTTP 传输       │
│  Agent 可直接调用远程工具              │
│  自动发现 tools/list                  │
└──────────────────────────────────────┘
```

---

## 修改文件清单（7 个）

### 1. `pkg/types/types_skills.go` — 新增 Store 配置

在 `SkillsConfig` 中增加 `Store` 字段：

```go
type SkillsStoreConfig struct {
    URL   string `json:"url,omitempty"`   // nexus-v4 基础 URL，如 "https://chat.acosmi.com"
    Token string `json:"token,omitempty"` // JWT Bearer token
}

// SkillsConfig 增加:
type SkillsConfig struct {
    // ...existing fields...
    Store *SkillsStoreConfig `json:"store,omitempty"`
}
```

### 2. `internal/agents/skills/skill_store_client.go` — 新建 HTTP 客户端

与 nexus-v4 skill-store API 通信的客户端：

```go
type SkillStoreClient struct {
    baseURL    string
    token      string
    httpClient *http.Client
}

// 远程技能摘要（从 nexus-v4 API 返回）
type RemoteSkillItem struct {
    ID            string `json:"id"`
    Key           string `json:"key"`
    Name          string `json:"name"`
    Description   string `json:"description"`
    Category      string `json:"category"`
    Version       string `json:"version"`
    SecurityLevel string `json:"securityLevel"`
    SecurityScore int    `json:"securityScore"`
    DownloadCount int64  `json:"downloadCount"`
    Tags          string `json:"tags"`
    Author        string `json:"author"`
    Icon          string `json:"icon"`
    IsInstalled   bool   `json:"isInstalled"` // 本地是否已存在（客户端计算）
}

func NewSkillStoreClient(baseURL, token string) *SkillStoreClient
func (c *SkillStoreClient) Browse(category, keyword string) ([]RemoteSkillItem, error)
func (c *SkillStoreClient) Detail(id string) (*RemoteSkillItem, error)
func (c *SkillStoreClient) Download(id string) ([]byte, string, error)  // content, filename, error
func (c *SkillStoreClient) Available() bool
```

实现要点：
- `Browse` → `GET {baseURL}/api/v4/skill-store?category=X&keyword=Y`
- `Detail` → `GET {baseURL}/api/v4/skill-store/{id}`
- `Download` → `GET {baseURL}/api/v4/skill-store/{id}/download` (返回 ZIP bytes)
- 所有请求带 `Authorization: Bearer {token}` header
- 30s 超时，错误包装

### 3. `internal/agents/skills/skill_store_sync.go` — 新建同步逻辑

将下载的 ZIP 解包为 `docs/skills/synced/{name}/SKILL.md`：

```go
// PullSkillToLocal 从远程下载技能并写入本地 docs/skills/synced/
func PullSkillToLocal(client *SkillStoreClient, skillID string, docsSkillsDir string) (*PullResult, error)

// PullResult 拉取结果
type PullResult struct {
    SkillName string
    Dir       string // 写入的本地目录
    IsNew     bool   // 新增 vs 更新
}

// BatchPull 批量拉取
func BatchPull(client *SkillStoreClient, skillIDs []string, docsSkillsDir string) ([]PullResult, []error)
```

实现要点：
- Download ZIP → 解压到临时目录
- 查找 `manifest.json` 或 `SKILL.md`
- 如果是 nexus-v4 格式（manifest.json），转换为 SKILL.md 格式
- 如果已有 SKILL.md，直接复制
- 写入 `docs/skills/synced/{skill-key}/SKILL.md`
- 返回 PullResult

### 4. `internal/gateway/server_methods_skills.go` — 新增 4 个 RPC 方法

```go
// skills.store.browse — 浏览远程技能商店
func handleSkillsStoreBrowse(ctx *MethodHandlerContext)
// 参数: { category?, keyword? }
// 返回: { skills: []RemoteSkillItem }
// 逻辑: 调用 SkillStoreClient.Browse()，标记本地已存在的为 isInstalled=true

// skills.store.pull — 批量拉取技能到本地
func handleSkillsStorePull(ctx *MethodHandlerContext)
// 参数: { skillIds: []string }
// 返回: { results: []PullResult, errors: []string }
// 逻辑: 调用 BatchPull()，写入 docs/skills/synced/

// skills.store.refresh — 刷新本地技能缓存
func handleSkillsStoreRefresh(ctx *MethodHandlerContext)
// 参数: {}
// 返回: { count: int, skills: []SkillSummary }
// 逻辑: 重新扫描 docs/skills/ 并返回最新列表

// skills.store.link — 返回 chat 端技能管理页面 URL
func handleSkillsStoreLink(ctx *MethodHandlerContext)
// 参数: { page?: "create" | "browse" | "manage" }
// 返回: { url: string }
// 逻辑: 拼接 store.URL + 前端路由路径
```

### 5. `internal/gateway/server_methods_skills.go` — 注册新方法

在 `SkillsHandlers()` 中追加 4 个方法映射。

### 6. `internal/gateway/server.go` — 注入 SkillStoreClient

在 Gateway 初始化时从 config 创建 `SkillStoreClient`，注入到 handler context。

需要在 `GatewayContext` 或 `MethodHandlerContext` 中增加 `SkillStoreClient` 字段。

### 7. `internal/gateway/server_methods_skills.go` — handleSkillsStatus 增加 synced 来源

source 判定增加 `"synced"`：

```go
} else if syncedDir != "" && strings.HasPrefix(e.Skill.Dir, syncedDir) {
    source = "synced"  // 从 chat 端同步的技能
}
```

---

## 实现清单

- [x] 1. `types_skills.go` — 新增 Store 配置结构
- [x] 2. `skill_store_client.go` — HTTP 客户端
- [x] 3. `skill_store_sync.go` — ZIP 解包 + 本地写入
- [x] 4. `server_methods_skills.go` — 4 个新 RPC 方法
- [x] 5. `server_methods.go` — GatewayMethodContext 增加 SkillStoreClient 字段
- [x] 6. `server.go` — 注入 SkillStoreClient
- [x] 7. `server_methods_skills.go` — handleSkillsStatus 增加 synced 来源
- [x] 8. 权限注册 — readMethods / adminExactMethods 更新
- [x] 9. 构建 + 测试 (`go build ./...` ✅, `go vet ./...` ✅, `go test ./internal/agents/skills/...` ✅)
- [x] 10. 审计修复: F-01 ZIP path traversal (CRITICAL → FIXED)
- [x] 11. 审计修复: F-02 HTTPS 强制校验 (CRITICAL → FIXED)
- [x] 12. 审计修复: F-03 ZIP 大小限制 (HIGH → FIXED)
- [x] 13. 审计报告已生成 → `docs/claude/audit/audit-2026-02-25-skill-store-bridge-p1.md` (PASS)

---

## 配置示例

用户在 `openacosmi.config.json` 中配置：

```json
{
  "skills": {
    "store": {
      "url": "https://chat.acosmi.com",
      "token": "eyJhbGciOiJIUzI1NiIs..."
    }
  }
}
```

也可通过 `openacosmi onboard` 向导设置。

---

## 前端交互流程

### 流程 1: 从商店拉取技能

```
用户点击「技能商店」
  → 前端调用 skills.store.browse RPC
  → 显示远程技能列表（卡片式，标记已安装）
  → 用户勾选多个技能
  → 点击「批量添加」
  → 前端调用 skills.store.pull RPC（传入 skillIds）
  → 后端下载 ZIP → 解包 → 写入 docs/skills/synced/
  → 前端刷新技能列表
```

### 流程 2: 新增自定义技能（必须通过 chat 审核）

```
用户点击「创建新技能」
  → 前端调用 skills.store.link RPC（page="create"）
  → 返回 URL: "https://chat.acosmi.com/skills/create"
  → 前端打开新窗口/iframe 跳转到 chat 端
  → 用户在 chat 端创建技能 → 4 重防护审核
  → 审核通过后，用户回到 OpenAcosmi
  → 点击「刷新」→ skills.store.refresh
  → 新技能出现在列表中
```

### 流程 3: 手动刷新

```
用户点击「刷新」按钮
  → 前端调用 skills.store.refresh RPC
  → 后端重新扫描 docs/skills/ 全部目录
  → 返回最新技能数量和列表
```

---

## 目录结构变化

```
docs/skills/
├── tools/      # 本地工具技能 (已有)
├── providers/  # 本地供应商技能 (已有)
├── general/    # 本地通用技能 (已有)
├── official/   # Claude 官方技能 (已有)
└── synced/     # ← 新增：从 chat 端同步的技能
    ├── web-search/
    │   └── SKILL.md
    ├── data-analysis/
    │   └── SKILL.md
    └── ...
```

---

## 安全考虑

| 风险 | 缓解措施 |
|------|----------|
| Token 泄露 | Token 存储在本地 config，不传输到前端；config 文件权限 600 |
| 中间人攻击 | 强制 HTTPS（`url` 必须以 `https://` 开头） |
| 恶意技能绕过审核 | OpenAcosmi 只从 `/skill-store` 接口拉取（仅返回 APPROVED 状态的技能） |
| 本地篡改 | synced/ 目录的技能可被用户自定义覆盖（.agent/skills/ 优先级更高） |

---

## 延迟项: Phase 2 MCP 远程工具执行

| ID | 描述 | 前置条件 |
|:--:|:-----|:---------|
| MCP-1 | 升级 OpenAcosmi `mcpclient` 支持 Streamable HTTP 传输 | Phase 1 完成 |
| MCP-2 | nexus-v4 新增 MCP Server 端点暴露已审核技能为 MCP tools | nexus-v4 侧开发 |
| MCP-3 | OpenAcosmi Agent 可通过 MCP 直接调用 nexus-v4 远程工具 | MCP-1 + MCP-2 |
| MCP-4 | 复用 nexus-v4 OAuth 2.1 (PKCE) 替代简单 JWT token | MCP-1 |

**关键文件参考** (nexus-v4 MCP 基础设施，Phase 2 可复用):
- `nexus-v4/backend/pkg/mcp/client.go` — 多传输 MCP 客户端
- `nexus-v4/backend/pkg/mcp/oauth.go` — OAuth 2.1 Token Manager
- `nexus-v4/backend/pkg/mcp/types.go` — 完整协议类型定义 (2025-11-25)

---

## P1 完成汇总 (2026-02-25)

### 交付物

| 文件 | 变更类型 | 行数 | 描述 |
|------|----------|------|------|
| `pkg/types/types_skills.go` | 修改 | +8 | `SkillsStoreConfig` + `Store` 字段 |
| `internal/agents/skills/skill_store_client.go` | **新建** | 220 | REST API 客户端 (Browse/Detail/Download) |
| `internal/agents/skills/skill_store_sync.go` | **新建** | 256 | ZIP 解包 + manifest→SKILL.md 转换 + BatchPull |
| `internal/gateway/server_methods_skills.go` | 修改 | +200 | 4 个 RPC (browse/pull/refresh/link) + synced source |
| `internal/gateway/server_methods.go` | 修改 | +6 | `SkillStoreClient` 字段 + 权限注册 |
| `internal/gateway/ws_server.go` | 修改 | +4 | WsServerConfig + methodCtx 传递 |
| `internal/gateway/server.go` | 修改 | +10 | SkillStoreClient 创建与注入 |

**新增代码量:** ~500 LOC (2 新文件 + 5 文件修改)

### 新增 RPC 方法

| 方法 | 权限 | 描述 |
|------|------|------|
| `skills.store.browse` | read | 浏览远程技能商店，自动标记已安装 |
| `skills.store.pull` | admin | 批量下载技能 ZIP → 解包到 synced/ |
| `skills.store.refresh` | read | 重新扫描本地 docs/skills/ 全目录 |
| `skills.store.link` | read | 返回 chat 端 URL (create/browse/manage) |

### 安全加固 (审计修复)

| 编号 | 级别 | 修复内容 |
|------|------|----------|
| F-01 | CRITICAL | ZIP path traversal — `filepath.IsLocal()` + `filepath.Clean()` |
| F-02 | CRITICAL | HTTPS 强制 — `Available()` 校验 `https://` 前缀 |
| F-03 | HIGH | ZIP 大小限制 — 10 MB 总包 + 5 MB 单文件 + `io.LimitReader` |

### 验证结果

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./internal/agents/skills/...` 5/5 PASS ✅
- 审计报告: PASS (3 Critical/High fixed, 2 Low accepted)
