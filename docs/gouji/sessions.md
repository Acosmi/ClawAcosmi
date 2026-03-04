# 会话管理架构文档

> 最后更新：2026-02-26 | 代码级审计完成

## 一、模块概述

| 属性 | 值 |
| ---- | ---- |
| 模块路径 | `backend/internal/sessions/` |
| Go 源文件数 | 7 |
| Go 测试文件数 | 6 |
| 测试函数数 | 26 |
| 总行数 | ~2,400 |

会话管理模块处理会话密钥解析、存储、重置策略、群组会话、元数据和转录文件操作。

## 二、文件索引

| 文件 | 行数 | 职责 | TS 来源 |
|------|------|------|---------|
| `paths.go` | ~105 | `ResolveSessionDir`, `ResolveStorePath`, `ExpandTemplate` | `paths.ts` |
| `reset.go` | ~120 | `EvaluateResetPolicy`, `ClassifyResetType`, `EvaluateFreshness` | `reset.ts` |
| `group.go` | ~85 | `ParseGroupSessionKey`, `BuildGroupDisplayName` | `group.ts` |
| `main_session.go` | ~95 | `ResolveMainSessionKey`, `DeriveAgentSessionKey` | `main-session.ts` |
| `metadata.go` | ~80 | `DeriveSessionOrigin`, `MergeOriginInfo` | `metadata.ts` |
| `transcript.go` | ~110 | `MirrorText`, `HandleMediaURL`, `AppendAssistantMessage` | `transcript.ts` |
| `store.go` | ~100 | 会话存储 (JSON 持久化 + 读写锁) | 新增 |

## 三、测试覆盖

| 测试 | 覆盖范围 |
|------|----------|
| `paths_test.go` | 路径模板展开 |
| `reset_test.go` | 重置策略 (daily/idle) |
| `group_test.go` | 群组键解析 |
| `main_session_test.go` | 主会话键推导 |
| `metadata_test.go` | 元数据合并 |
| `store_test.go` | 存储读写 |
| **合计** | **26 个测试函数** |
