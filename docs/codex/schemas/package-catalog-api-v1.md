# Package Catalog API v1 — RPC 接口契约

> Phase 0 冻结文档。定义所有后续 Phase 依赖的 RPC 方法签名。
> 数据类型定义见 `backend/pkg/types/types_packages.go`、`types_skills.go`、`types_models.go`。

## 约定

- 传输层: WebSocket JSON-RPC 2.0（与现有 Gateway RPC 一致）
- 请求/响应字段采用 camelCase
- 可选字段用 `?` 标记
- 所有方法共享 Gateway 现有的认证和错误处理机制

---

## 1. Packages — 统一应用中心 (Phase 3)

### packages.catalog.browse

浏览统一目录（聚合 skill + plugin + bundle）。

**请求:**
```json
{
  "kind": "skill|plugin|bundle",  // 可选，过滤包类型
  "keyword": "string",            // 可选，关键词搜索
  "category": "string",           // 可选，分类过滤
  "page": 1,                      // 可选，页码，默认 1
  "pageSize": 20                  // 可选，每页条数，默认 20
}
```

**响应:**
```json
{
  "items": [PackageCatalogItem],
  "total": 42
}
```

### packages.catalog.detail

获取单个包详情。

**请求:**
```json
{
  "id": "string"
}
```

**响应:**
```json
{
  "item": PackageCatalogItem
}
```

### packages.install

安装指定包。

**请求:**
```json
{
  "id": "string",
  "kind": "skill|plugin|bundle"
}
```

**响应:**
```json
{
  "record": PackageInstallRecord
}
```

### packages.update

更新已安装的包到最新版本。

**请求:**
```json
{
  "id": "string"
}
```

**响应:**
```json
{
  "record": PackageInstallRecord
}
```

### packages.remove

卸载已安装的包。

**请求:**
```json
{
  "id": "string"
}
```

**响应:**
```json
{
  "success": true
}
```

### packages.installed

列出已安装的包。

**请求:**
```json
{
  "kind": "skill|plugin|bundle"  // 可选，过滤包类型
}
```

**响应:**
```json
{
  "records": [PackageInstallRecord]
}
```

---

## 2. Auth — 登录与授权 (Phase 2)

### auth.state

查询当前登录态。

**请求:**
```json
{}
```

**响应:**
```json
{
  "state": AuthState
}
```

### auth.login.start

启动 OAuth 2.1 + PKCE 登录流程。

**请求:**
```json
{
  "provider": "string"  // 可选，默认 "acosmi"
}
```

**响应:**
```json
{
  "authURL": "https://nexus.openacosmi.ai/auth/authorize?..."
}
```

### auth.login.exchange

用授权码交换 token。

**请求:**
```json
{
  "code": "string"
}
```

**响应:**
```json
{
  "state": AuthState
}
```

### auth.logout

登出，清除本地 token 和权益缓存。

**请求:**
```json
{}
```

**响应:**
```json
{
  "success": true
}
```

---

## 3. Models — 托管模型 (Phase 4)

### models.managed.list

列出平台可用的托管模型。

**请求:**
```json
{}
```

**响应:**
```json
{
  "models": [ManagedModelEntry]
}
```

---

## 4. Wallet — 钱包与用量 (Phase 5)

### models.wallet.balance

查询钱包余额。

**请求:**
```json
{}
```

**响应:**
```json
{
  "balance": 42.50,
  "currency": "TK"
}
```

### models.wallet.usage

查询用量记录。

**请求:**
```json
{
  "range": "7d|30d|all"  // 可选，默认 "7d"
}
```

**响应:**
```json
{
  "records": [
    {
      "id": "string",
      "modelId": "string",
      "tokensUsed": 1500,
      "cost": 0.03,
      "timestamp": "2026-03-08T12:00:00Z"
    }
  ]
}
```
