# 第三轮深度审计报告 — Part 3: 协议层 + 根级行为 + 总结

---

## 八、`gateway/protocol/` 协议子系统 — 21 文件，90KB

> [!WARNING]
> 这是 Gateway WebSocket 协议的**完整定义层**，包含所有 RPC 帧格式、schema 校验、错误码。第二轮审计完全未提及。

| 文件 | 大小 | 职责 |
|------|------|------|
| `protocol/index.ts` | **19KB** | 协议主入口：消息分发、帧解析 |
| `protocol/schema/types.ts` | **11KB** | 协议类型定义 |
| `protocol/schema/protocol-schemas.ts` | **8KB** | Schema 注册表 |
| `protocol/schema/cron.ts` | **7KB** | Cron 协议帧 |
| `protocol/schema/frames.ts` | **4KB** | 帧格式定义 |
| `protocol/schema/sessions.ts` | **4KB** | 会话协议帧 |
| `protocol/schema/agents-models-skills.ts` | **5KB** | Agent/模型/技能帧 |
| `protocol/schema/channels.ts` | **3KB** | 频道帧 |
| `protocol/schema/exec-approvals.ts` | **3KB** | 审批帧 |
| 其余 12 文件 | ~26KB | 其他协议定义 |

**Go 端影响**: `internal/gateway/` 的 WebSocket 消息格式必须与此协议层**100% 兼容**。任何帧格式不一致都将导致前端断连。

---

## 九、`logging/` 日志子系统 — 15 文件，57KB

第二轮审计仅在依赖链 4.15 中简要提及 subsystem.ts，但日志系统实际规模远超预估：

| 文件 | 大小 | 职责 |
|------|------|------|
| `subsystem.ts` | **10KB** | 子系统 logger 创建 |
| `diagnostic.ts` | **9KB** | 诊断事件 |
| `console.ts` | **8KB** | 控制台捕获/重定向 |
| `logger.ts` | **7KB** | 核心 logger |
| `redact.ts` | **4KB** | 日志脱敏（API Key 等） |
| `parse-log-line.ts` | **1.7KB** | 日志行解析 |
| 其余 9 文件 | ~17KB | 配置、级别、状态 |

**关键隐式行为**: `redact.ts` 自动在日志输出中**屏蔽 API 密钥**，Go 端 `pkg/log/` 需等价实现。

---

## 十、根级文件补充（第二轮遗漏的隐式行为）

### 10.1 `src/utils.ts` — 9.3KB，第二轮未提及

此文件是**全局工具函数集**，被几乎所有模块依赖：

- `jidToPhoneNumber()` / `phoneNumberToJid()` — WhatsApp JID 转换
- `lidToLegacyId()` — LID 反向映射
- `isGroup()` / `isStatus()` — 群组/状态判断
- `CONFIG_DIR` — 全局配置目录常量
- `sleep()` — 延迟函数
- `pluralize()` — 复数化
- `formatBytes()` — 字节格式化

### 10.2 `src/polls.ts` — 2KB，完全遗漏

投票系统：WhatsApp 投票消息的创建/解析。Go 端需实现。

### 10.3 `src/compat/index.ts` — 兼容层

Node.js 兼容性垫片，Go 端不需要但需确认无业务逻辑。

---

## 十一、第三轮审计总汇总

### 数据对比

| 指标 | 第二轮 | 第三轮 | 增量 |
|------|--------|--------|------|
| 遗漏模块总数 | 19 | **28** | +9 |
| 隐藏依赖链 | 15 | **20** | +5 |
| 文件大小低估 | 10 | **18** | +8 |
| 根级隐式行为 | 11 | **14** | +3 |
| 受影响 Phase | 4-7 | 4-7 | 无变化 |

### 第三轮新增对 Phase 4 的影响（最重要）

Phase 4 原子任务数需从 16 调整为 **20**：

| 新增编号 | 子任务 | 来源 | 复杂度 |
|----------|--------|------|--------|
| 4.16 | **出站消息管线** | `infra/outbound/` 31 文件 | ⭐⭐⭐⭐⭐ |
| 4.17 | **执行审批系统** | `infra/exec-approvals` + forwarder | ⭐⭐⭐⭐ |
| 4.18 | **心跳系统** | `infra/heartbeat-*` 5 文件 | ⭐⭐⭐ |
| 4.19 | **Gateway 协议层验证** | `gateway/protocol/` 90KB | ⭐⭐⭐ |

### Phase 4 修订后总代码量预估

| 类别 | 第二轮预估 | 第三轮修正 |
|------|-----------|-----------|
| PI Runner 核心 | ~200KB | ~200KB（确认准确） |
| 工具系统 | ~90KB | **~500KB**（agents/tools/ 60 文件） |
| 基础设施 | ~60KB | **~400KB**（outbound+审批+心跳+成本+迁移） |
| 配置验证 | 未计入 | **~160KB**（schema+zod+legacy） |
| **合计** | **~350KB** | **~1,260KB** |

> [!CAUTION]
> Phase 4 的实际代码量是第二轮预估的 **3.6 倍**。建议将 Phase 4 拆分为 Phase 4A（核心引擎）和 Phase 4B（基础设施）两个子阶段。

---

## 十二、风险评级修订

| 风险 | 第二轮 | 第三轮 | 原因 |
|------|--------|--------|------|
| Phase 4 代码量 | 🔴 极高 | 🔴🔴🔴 | 实际 1,260KB vs 预估 350KB |
| 出站管线遗漏 | — | 🔴 高 | Agent 回复无法投递 |
| 执行审批遗漏 | — | 🔴 高 | Bash 工具无安全审批 |
| 协议兼容性 | — | 🟡 中 | 90KB 协议定义需对齐 |
| 配置验证遗漏 | — | 🟡 中 | 160KB 验证规则 |

---

> **审计结论**: 建议在执行 Phase 4 前，先完成所有遗漏模块的**代码量清点和子任务拆分**，避免中途发现范围膨胀。
