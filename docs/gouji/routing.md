# Routing 模块架构文档

> 最后更新：2026-02-26 | 代码级审计完成

## 一、模块概述

| 属性 | 值 |
| ---- | ---- |
| 模块路径 | `backend/internal/routing/` |
| Go 源文件数 | 2 |
| Go 测试文件数 | 1 |
| 测试函数数 | 12 |
| 总行数 | ~700 |

会话路由和 session key 管理，所有频道入站消息通过 session key 路由到正确的 agent 会话。

## 二、文件索引

| 文件 | 行数 | 职责 | TS 来源 |
|------|------|------|---------|
| `session_key.go` | ~460 | 18 个核心函数: `ParseAgentSessionKey`, `NormalizeAgentID`, `BuildAgentPeerSessionKey`, `ClassifySessionKeyShape`, `ResolveThreadSessionKeys` 等 | `session-key.ts` + `session-key-utils.ts` |
| `bindings.go` | ~240 | 频道绑定解析: `ResolveBindings`, `MatchBinding`, `NormalizeBindingKey` | 新增 |

## 三、核心类型与函数

```go
type SessionKeyShape string  // "missing"|"agent"|"legacy_or_alias"|"malformed_agent"
type ParsedAgentSessionKey struct { AgentID, Rest string }
type PeerSessionKeyParams struct { ... }  // 8 字段

// 18 个核心函数
func ParseAgentSessionKey(sessionKey string) *ParsedAgentSessionKey
func NormalizeAgentID(value string) string
func BuildAgentMainSessionKey(agentID, mainKey string) string
func BuildAgentPeerSessionKey(p PeerSessionKeyParams) string
func ClassifySessionKeyShape(sessionKey string) SessionKeyShape
func ResolveThreadSessionKeys(...) ThreadSessionKeyResult
// ... 等
```

## 四、测试覆盖

| 测试文件 | 测试数 | 覆盖范围 |
|----------|--------|----------|
| `session_key_test.go` | 12 | 全函数覆盖 |
