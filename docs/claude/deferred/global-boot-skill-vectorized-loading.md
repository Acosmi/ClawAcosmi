---
document_type: Deferred
status: In Progress
created: 2026-02-26
last_updated: 2026-02-27
audit_report: Pending
skill5_verified: false
---

# 全局 Boot 启动 + Qdrant 按需检索提取 + VFS 分级按需加载

> **这是智能体运行时的全局架构，不是某个子系统的优化。**
>
> 核心循环：**智能体需要信息 → Qdrant 检索定位 → VFS 提取分级内容 → 智能体如需更详细 → 提取更详细的**。
>
> 向量只是辅助，增强记忆的手段，跟这套检索提取架构是两件事。

---

## 1. 问题陈述

### 1.1 当前：各子系统各管各的，全量加载

| 子系统 | 当前做法 | 问题 |
|--------|---------|------|
| **技能** | 启动扫描 69 个 SKILL.md → 全量缓存 → 69 条索引注入 prompt | O(n) token，全量 I/O |
| **记忆** | UHMS 已有 VFS + Qdrant，但与技能/插件独立 | 检索能力没复用 |
| **插件** | 硬编码注册表，config 激活 | 无法按需发现 |
| **会话上下文** | 全量 messages 数组 | 无分级，压缩粗暴 |

**共同问题**：没有统一的"需要什么就检索什么、需要多少就给多少"的机制。

### 1.2 目标：全局统一的按需检索提取

```
智能体运行时（不管在做什么）:

  需要技能？    ─┐
  需要记忆？    ─┤
  需要插件信息？ ─┤→ 同一套流程：Qdrant 检索 → VFS 提取分级 → 按需深入
  需要历史上下文？┘

  每次只给智能体需要的，需要更多时再提取更详细的。
```

### 1.3 技能加载（当前最突出的问题）

```
启动 → 扫描 5 个来源 → 读取 69 个 SKILL.md 文件（~200-500ms I/O）
     → 全部缓存 skillsCache map[name]→content
     → 生成 69 条索引注入 System Prompt（~1500 tokens 固定开销）
     → LLM 自己从 69 条中匹配 → lookup_skill(name) 取内容
```

核心文件：
- `workspace_skills.go` — `LoadSkillEntries()` 扫描/索引
- `attempt_runner.go:569-591` — 缓存构建 + prompt 注入
- `tool_executor.go:613-644` — `lookup_skill` 执行

---

## 2. 全局架构设计

### 2.1 核心循环：智能体 → 检索 → 提取分级 → 按需深入

这是**整个智能体运行时的全局模式**，不是某个子系统的专属优化：

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  智能体正在工作（任何场景）                                       │
│       │                                                         │
│       ▼                                                         │
│  "我需要某个信息"（技能 / 记忆 / 插件 / 历史上下文）              │
│       │                                                         │
│       ▼                                                         │
│  ┌──────────────┐                                               │
│  │ Qdrant 检索   │ ← 找到相关数据的位置 + 元数据                  │
│  └──────┬───────┘                                               │
│         ▼                                                       │
│  ┌──────────────┐                                               │
│  │ VFS 提取 L0   │ ← 摘要（~100tk），快速预览，低成本              │
│  └──────┬───────┘                                               │
│         ▼                                                       │
│  智能体判断："这个够了" → 继续工作                                 │
│         │                                                       │
│         ▼ "需要更详细"                                           │
│  ┌──────────────┐                                               │
│  │ VFS 提取 L1   │ ← 概览（~2K tk），足够做决策                   │
│  └──────┬───────┘                                               │
│         ▼                                                       │
│  智能体判断："还需要完整内容" → 很少走到这步                       │
│         │                                                       │
│         ▼                                                       │
│  ┌──────────────┐                                               │
│  │ VFS 提取 L2   │ ← 完整内容，按需读取                           │
│  └──────────────┘                                               │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**这个循环适用于所有数据类型**：

| 数据类型 | Qdrant Collection | VFS 路径 | 循环举例 |
|----------|-------------------|----------|----------|
| 技能 | `sys_skills` | `_system/skills/` | 用户提到"调试网页" → 检索 → L0 摘要 → 选中 browser → L1/L2 |
| 记忆 | `mem_*` | `{userID}/{memType}/` | 智能体需要回忆 → 检索 → L0 摘要 → 需要细节 → L1/L2 |
| 插件 | `sys_plugins` | `_system/plugins/` | 需要发消息 → 检索 → L0 摘要 → 选中飞书 → L1 配置 |
| 会话归档 | `sys_sessions` | `{userID}/archives/` | "上次讨论了什么" → 检索 → L0 摘要 → L1 详情 |

### 2.2 三个角色

```
┌───────────────────────────────────────────────────────────┐
│  角色             │  谁               │  做什么             │
├───────────────────┼───────────────────┼─────────────────────┤
│  快速启动         │  Boot 文件         │  告诉系统一切在哪    │
│  检索 + 提取      │  Qdrant Segment   │  找到需要的数据      │
│  分级存储         │  VFS (L0/L1/L2)   │  按需提供内容        │
└───────────────────────────────────────────────────────────┘
```

**Qdrant Segment 是检索提取引擎**。
基于 Qdrant 源码（`qdrant-engine/lib/segment/`），in-process 运行，
mmap 持久化到本地磁盘，零网络开销，占用资源少，可靠性高。
它的职责：**接收查询 → 定位数据 → 返回元数据和 VFS 位置**。
所有数据类型共用同一个 Segment store，按 Collection 隔离。

**VFS 是存储主力**。
所有内容按 L0/L1/L2 三级分级存储在本地文件系统中，零依赖，始终可用。
L0 (~100tk 摘要) → L1 (~2K 概览) → L2 (完整内容)。逐级按需，绝不多给。

**Boot 文件是启动加速器**。
一个文件，包含上次会话摘要、系统全局地图、检索指南。
重启后只读这一个文件就知道一切在哪。

**向量是另一回事**。
向量（Embedding）是 UHMS 记忆系统的可选增强手段，
用于提升记忆搜索的语义理解。跟这套检索提取架构完全独立。

### 2.3 入库 + 检索数据流（以技能为例）

```
═══════════════════════════════════════════════
  入库（一键 VFS 分级）
═══════════════════════════════════════════════

  源文件（SKILL.md / 记忆 / 插件配置 / 会话摘要）
       ↓ 解析
  ┌─────────────────┐     ┌──────────────────┐
  │ Qdrant Segment   │     │  VFS 文件系统     │
  │ (检索索引)        │     │  (分级内容)       │
  │                  │     │                  │
  │ 存入: id,        │     │  L0: 摘要        │
  │   payload {      │     │  L1: 概览        │
  │     name,        │     │  L2: 完整内容     │
  │     category,    │     │                  │
  │     description, │     │  meta.json       │
  │     tags,        │     │                  │
  │     vfs_path     │     │                  │
  │   }              │     │                  │
  └─────────────────┘     └──────────────────┘
  源文件不动。

═══════════════════════════════════════════════
  运行时（全局按需循环）
═══════════════════════════════════════════════

  智能体: "帮我调试网页上的按钮"
       ↓
  Qdrant 检索 sys_skills → 匹配 top 3
       ↓
  VFS 读 L0（3 条摘要 ~300tk）→ 注入 prompt
       ↓
  智能体判断 → "需要 browser 的完整指令"
       ↓
  VFS 读 L1 或 L2 → 返回详细内容
       ↓
  智能体继续工作
```

---

## 3. Boot 文件

### 3.1 结构

```
路径: ~/.openacosmi/memory/boot.json
大小: ~2-5 KB
```

```jsonc
{
  "version": "1.0",
  "updated_at": "2026-02-27T10:30:00Z",

  // ─── 上次会话 ───
  "last_session": {
    "summary": "讨论了飞书审批卡片回调，修复了 cardActionFunc 注入。",
    "active_tasks": ["飞书跨会话通知", "channelMeta 类型修复"],
    "ended_at": "2026-02-27T15:45:00Z"
  },

  // ─── 系统地图：告诉系统一切在哪 ───
  "system_map": {
    "skills": {
      "source_dir": "docs/skills/",
      "vfs_dir": "_system/skills/",
      "categories": ["general", "tools", "official", "providers"],
      "total_count": 71,
      "indexed": true,
      "qdrant_collection": "sys_skills",
      "last_indexed_at": "2026-02-27T10:30:00Z"
    },
    "plugins": {
      "registered": ["feishu", "dingtalk", "wecom"],
      "active": ["feishu"]
    },
    "memory": {
      "vfs_root": "~/.openacosmi/memory/vfs/",
      "segment_data": "~/.openacosmi/memory/segment/"
    }
  },

  // ─── 按需检索指南：智能体统一循环 ───
  "search_guide": {
    "pattern": "需要任何信息 → Qdrant 检索对应 collection → VFS 读 L0 摘要 → 需要更多再读 L1/L2",
    "skills": "需要技能 → 检索 sys_skills → L0 预览 → lookup_skill(name) → L1/L2",
    "memory": "需要回忆 → 检索 mem_* → L0 摘要 → 需要细节 → L1/L2",
    "plugins": "需要外部通道 → 检索 sys_plugins → L0 → L1 配置",
    "sessions": "需要历史 → 检索 sys_sessions → L0 摘要 → L1 详情"
  }
}
```

### 3.2 启动流程

```
Gateway.Start()
       ↓
  读取 boot.json
       ↓
  ┌─ 存在且合法 ────────────────────────────┐
  │ 1. 打开 Qdrant Segment store (~50ms)     │
  │ 2. 知道技能在哪 → 不扫描文件系统          │
  │ 3. 恢复上次会话上下文                     │
  │ 4. 就绪 ✓                               │
  └──────────────────────────────────────────┘
       ↓
  ┌─ 不存在 / 损坏 ─────────────────────────┐
  │ 退回现有逻辑:                             │
  │ LoadSkillEntries() 全量扫描 → 生成 Boot   │
  │ 现有代码 100% 保留，零风险                 │
  └──────────────────────────────────────────┘
```

### 3.3 生命周期

| 事件 | 操作 |
|------|------|
| 首次安装 | 不存在 → 全量扫描 → 生成 Boot |
| 正常重启 | 读取 Boot → 就绪（不扫描） |
| 会话结束 | 更新 `last_session` |
| 一键分布式 | 更新 `skills.indexed` + 索引时间 |
| 技能变更 | content_hash 变化 → 增量更新 |
| Boot 损坏 | 校验失败 → 删除 → 退回全量扫描 |

---

## 4. Qdrant 检索 + VFS 存储

### 4.1 VFS 技能存储

技能作为 `_system` 命名空间存入 VFS，与用户记忆隔离：

```
{vfsRoot}/_system/skills/{category}/{skillName}/
  ├── l0.txt       ← "browser: Integrated browser control..."（~100tk 摘要）
  ├── l1.txt       ← 概览 + tags + 使用场景（~2K tokens）
  ├── l2.txt       ← 完整 SKILL.md 原文
  └── meta.json    ← { name, category, source_path, content_hash }
```

源文件 `docs/skills/tools/browser/SKILL.md` **不动**。

### 4.2 Qdrant 检索索引

Collection: `sys_skills`

Qdrant 存储每个技能的**检索元数据**（不存内容本身）：

```
Point {
  id:      确定性 UUID
  payload: {
    "name":         "browser",
    "category":     "tools",
    "description":  "Integrated browser control service + action commands",
    "tags":         "web,automation,chrome",
    "vfs_path":     "_system/skills/tools/browser",
    "content_hash": "a1b2c3",
    "distributed":  true
  }
}
```

对常用字段建立 payload field index（`segment_store.rs:276-299` 已支持）：

```
create_field_index("sys_skills", "category", Keyword)
create_field_index("sys_skills", "name", Keyword)
create_field_index("sys_skills", "tags", Keyword)
```

Qdrant 的职责：**接收查询 → 检索匹配的技能 → 返回 ID + 元数据 → 告诉系统去 VFS 哪里读内容**。

### 4.3 按需加载流水线

```
用户消息 → Qdrant 检索 sys_skills → 匹配 top 3
                ↓
  VFS 读 L0（3 条摘要 ~300tk）→ 注入 prompt
                ↓
  LLM 判断 → 调用 lookup_skill("browser")
                ↓
  VFS 读 L1（概览 ~2K）或 L2（完整内容）→ 返回给 LLM
```

**层级按需**：
- 先读 L0（列表预览，极低成本）
- LLM 感兴趣才读 L1（上下文够用）
- 真正需要完整指令才读 L2（完整 SKILL.md）

### 4.4 降级策略

```
Level 0 (正常): Boot → Qdrant 检索 → VFS 读取
                  ↓ Qdrant 不可用
Level 1 (降级): FTS5 关键词匹配 → VFS 读取
                  ↓ FTS5 也失败
Level 2 (兜底): LoadSkillEntries() 全量扫描（现有逻辑不动）
```

---

## 5. "一键 VFS 分级" 功能

### 5.1 技能中心 UI 改造

改动范围小——在现有技能中心增加两个元素：

#### 5.1.1 全局操作栏：一键 VFS 分级按钮

在技能中心顶部（刷新按钮旁）新增一个按钮：

```
┌──────────────────────────────────────────────────┐
│  技能中心                                         │
│  管理和配置已安装的技能                              │
│                                                   │
│            [刷新]  [一键 VFS 分级]                  │
│                                                   │
│  搜索技能...                    显示 71 个          │
│                                                   │
│  ▶ 工作区技能 (5)                                  │
│  ▶ 内置技能 (41)                                   │
│  ...                                              │
└──────────────────────────────────────────────────┘
```

按钮行为：
- 点击 → 调用 `skills.distribute` RPC → 进度态 "分级中..."
- 完成 → 刷新列表 → 提示 "已完成 VFS 分级: 71 个技能"
- 出错 → 显示错误信息

#### 5.1.2 技能卡片：VFS 分级角标

在每个技能的 chip-row 中新增一个角标：

```
已分级:
┌─────────────────────────────────────────────┐
│ 🌐 browser                                  │
│ Integrated browser control service...       │
│ [openacosmi-bundled] [eligible] [已VFS分级]  │
│                               [Enable]      │
└─────────────────────────────────────────────┘

未分级:
┌─────────────────────────────────────────────┐
│ 📅 date-time                                │
│ Date, time and timezone utilities...        │
│ [openacosmi-bundled] [eligible] [未分级]     │
│                               [Enable]      │
└─────────────────────────────────────────────┘
```

- **已分级**: `chip chip-ok` 样式，显示 "已VFS分级"
- **未分级**: `chip chip-warn` 样式，显示 "未分级"（提示用户可以执行一键分级）

#### 5.1.3 前端改动文件

**`ui/src/ui/types.ts`** — `SkillStatusEntry` 新增字段：

```typescript
// 新增：
distributed?: boolean;    // 是否已 VFS 分级
distributedAt?: string;   // 分级时间 ISO string
```

**`ui/src/ui/views/skills.ts`** — 改动：

1. `SkillsProps` 新增：
```typescript
distributeLoading: boolean;        // 分级进行中
distributeResult: string | null;   // 结果消息
onDistribute: () => void;          // 一键分级回调
```

2. `renderSkills()` — 顶部按钮区新增 "一键 VFS 分级" 按钮
3. `renderSkill()` — chip-row 中根据 `skill.distributed` 显示角标

**`ui/src/ui/controllers/skills.ts`** — 新增：

```typescript
// SkillsState 新增：
distributeLoading: boolean;
distributeResult: string | null;

// 新函数：
export async function distributeSkills(state: SkillsState) {
  // state.client.request("skills.distribute", {})
  // 完成后 loadSkills(state) 刷新
}
```

### 5.2 后端 API: `skills.distribute`

WebSocket RPC 方法（与现有 `skills.status` / `skills.update` 同级）：

```
1. 扫描 docs/skills/**/ 所有 SKILL.md
2. 对每个技能:
   a. 解析 frontmatter (name, description, tags, category)
   b. 计算 content_hash → 跳过未变化的（增量）
   c. 写入 VFS: L0 / L1 / L2 + meta.json
   d. Qdrant upsert("sys_skills", id, payload)
   e. create_field_index (首次)
   f. flush() 持久化
3. 更新 boot.json
4. 返回 { indexed: 71, skipped: 0 }
```

### 5.3 "已VFS分级" 含义

- 源文件不动（`docs/skills/` 原位）
- VFS 有分级副本（L0/L1/L2），运行时按需读取
- Qdrant 有检索索引（payload + field index）
- Boot 文件已记录（重启后直接可用）
- 技能中心角标显示"已VFS分级"

---

## 6. 向量与记忆系统（独立于技能加载）

向量是 UHMS 记忆系统的**可选增强手段**，用于提升记忆搜索的语义理解：

```
记忆系统:
  存储 → VFS (L0/L1/L2)
  检索 → Qdrant Segment
  增强 → 向量模型 (可选配置: Ollama/OpenAI 等)
         ↑ 这是辅助手段，增强记忆搜索精度
         ↑ 需要在设置中配置 embeddingProvider + embeddingModel
         ↑ 不配就不用，不影响基本功能
```

技能加载系统**不涉及向量**。Qdrant 通过 payload 元数据检索技能，
VFS 提供分级内容，Boot 文件加速启动。三者配合，按需加载。

---

## 7. 全局落地计划

这是全局架构，按数据类型分阶段落地。每个阶段都遵循同一套循环：
**智能体 → Qdrant 检索 → VFS 提取 L0 → 需要更多 → 提取 L1/L2**

### Phase 1: 技能（本方案核心）

| 项 | 说明 |
|----|------|
| Collection | `sys_skills` |
| VFS 路径 | `_system/skills/{category}/{name}/` |
| 入库方式 | 技能中心"一键 VFS 分级"按钮 |
| 运行时 | 用户消息 → 检索技能 → L0 摘要注入 → 按需 L1/L2 |
| 当前状态 | ❌ 待实现 |

### Phase 2: 插件

| 项 | 说明 |
|----|------|
| Collection | `sys_plugins` |
| VFS 路径 | `_system/plugins/{pluginID}/` |
| 入库方式 | 插件注册时自动生成 L0/L1/L2 |
| 运行时 | 智能体需要外部通道 → 检索插件 → L0 看有哪些 → L1 看怎么用 |
| 当前状态 | ❌ 待实现 |

### Phase 3: 会话上下文归档

| 项 | 说明 |
|----|------|
| Collection | `sys_sessions` |
| VFS 路径 | `{userID}/archives/{sessionID}/` |
| 入库方式 | 会话结束时自动摘要 → L0/L1/L2 |
| 运行时 | "上次讨论了什么" → 检索归档 → L0 摘要 → 需要细节 → L1/L2 |
| 当前状态 | ❌ 待实现（会话归档已有基础） |

### 已有基础: 用户记忆

| 项 | 说明 |
|----|------|
| Collection | `mem_episodic`, `mem_semantic`, ... |
| VFS 路径 | `{userID}/{memType}/{category}/{memID}/` |
| 入库方式 | UHMS 自动（对话中提取） |
| 运行时 | `BuildContextBlock(query, budget)` 按需检索 |
| 当前状态 | ✅ 已实现 |

### 全局统一接口

所有 Phase 共用同一套底层：

```
统一检索: manager.Search(collection, query, topK) → []Hit{ID, Payload, VFSPath}
统一提取: vfs.ReadL0(path) / vfs.ReadL1(path) / vfs.ReadL2(path)
统一入库: manager.Index(collection, id, payload, l0, l1, l2)
```

智能体不需要知道数据来自哪里。它只需要：
1. **我要什么** → Qdrant 检索
2. **给我摘要** → VFS L0
3. **给我更多** → VFS L1/L2

---

## 8. 资源开销

### Qdrant Segment（in-process 源码级）

| 指标 | 69 技能 | 500 技能 |
|------|---------|----------|
| 磁盘 | ~50KB | ~400KB |
| 内存 | ~1MB | ~3MB |
| 重启加载 | ~50ms | ~100ms |

### VFS 存储

| 指标 | 69 技能 | 500 技能 |
|------|---------|----------|
| L0+L1+L2 | ~2.2MB | ~17MB |

### Token 消耗

| 技能数 | 现有（全量注入） | Boot 模式 | 节省 |
|--------|----------------|-----------|------|
| 69 | ~1500 tk/次 | ~300 tk/次 | **80%** |
| 200 | ~4500 tk/次 | ~300 tk/次 | **93%** |
| 500 | ~11000 tk/次 | ~300 tk/次 | **97%** |

O(n) → O(1)。

---

## 9. 改动文件

### Phase 1（技能）改动清单

| 文件 | 改动 | 说明 |
|------|------|------|
| `uhms/vfs.go` | 扩展 | `_system/` 命名空间支持 |
| `uhms/boot.go` | **新建** | Boot 文件读写 + 全局系统地图 |
| `uhms/manager.go` | 扩展 | 统一接口 `Index()` / `Search()` + 按 collection 分发 |
| `uhms/config.go` | 小改 | `BootFilePath` |
| `vectoradapter/adapter.go` | 扩展 | `sys_skills` collection（后续 Phase 加 `sys_plugins`/`sys_sessions`） |
| `attempt_runner.go` | 改 | Boot 模式分支：检索 → L0 注入 → 按需 L1/L2 |
| `workspace_skills.go` | 改 | `LoadSkillEntries()` Boot 分支 |
| `server_methods_uhms.go` | 扩展 | `skills.distribute` RPC |
| `ui/src/ui/types.ts` | 扩展 | `SkillStatusEntry` 加 `distributed` 字段 |
| `ui/src/ui/views/skills.ts` | 改 | 顶栏加"一键VFS分级"按钮 + 技能卡片加分级胶囊 |
| `ui/src/ui/controllers/skills.ts` | 新增 | `distributeSkills()` + 状态字段 |

### 全局架构（跨 Phase 复用）

| 文件 | 说明 |
|------|------|
| `uhms/boot.go` | Boot 文件管理所有 Phase 的系统地图 |
| `uhms/manager.go` | 统一 `Search(collection, query, topK)` 供所有子系统调用 |
| `uhms/vfs.go` | `ReadL0/ReadL1/ReadL2` 已有，`_system/` 命名空间 Phase 1 加入后通用 |
| `vectoradapter/adapter.go` | 每个 Phase 只需注册新 Collection |

---

## 10. 总结

```
这是全局架构:

  智能体运行时的统一模式:
    智能体需要信息 → Qdrant 检索 → VFS 提取 L0
                  → 需要更多   → VFS 提取 L1
                  → 需要完整   → VFS 提取 L2

  适用于一切:
    技能、记忆、插件、会话上下文——同一套循环。

  三个角色:
    Boot 文件        → 告诉系统一切在哪（~50ms 启动）
    Qdrant Segment   → 检索 + 定位（找到需要的数据）
    VFS (L0/L1/L2)   → 分级按需提供内容

  不相关的:
    向量             → 那是记忆系统的辅助增强，独立的事
    源文件           → 不动

  保障:
    降级             → Qdrant 不可用 → FTS5 → 全量扫描
    成本             → O(1)，不随数据量增长

  落地顺序:
    Phase 1: 技能（一键 VFS 分级）
    Phase 2: 插件
    Phase 3: 会话上下文归档
    已有: 用户记忆（UHMS）
```
