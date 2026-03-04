# Gateway 日志审计报告 (2026-02-28 15:04 → 16:05)

**审计对象**: Gemini Flash 智能体在飞书频道的交互日志  
**审计时间**: 2026-02-28  
**审计人**: Antigravity (代码审计辅助)

---

## 总体结论

| 维度 | 状态 | 说明 |
|------|------|------|
| 网关核心 | ✅ 正常 | HTTP、飞书收发、LLM 调用均正常 |
| Argus Bridge | ❌→✅ | 重启前 stopped，重启后 ready |
| Intent Filter | 🔴 严重 | 多次将执行指令误判为 chat |
| Skills 索引 | 🔴 严重 | 73 个 skills 全部 skipped，0 个 indexed |
| Token 浪费 | 🔴 严重 | 死循环消耗 ~82 万 tokens |
| 用户体验 | 🔴 严重 | 删除任务未完成，一次回复丢失 |

---

## 阶段一：Argus Bridge 不可用 (15:04 → 15:33)

### 现象

Argus 处于 stopped 状态，所有 `argus_run_shell` 调用失败。

### 关键日志

```log
argus: bridge not available (state: stopped)
```

### 影响

- 智能体优先选择 `argus_run_shell` → 失败 → 误判整个网关故障
- **但 `bash` 工具可以正常工作**：

```log
tool=bash  sandbox bash exec command="ls -lh \"/Users/fushihua/Desktop/2月27日\""  mode=native
```

- 智能体执行 `rm` 时选择了 `argus_run_shell` 而非 `bash`，导致删除失败

---

## 阶段二：Skills 搜索死循环 (15:06 → 15:08)

### 触发条件

用户问 "能否用浏览器自动化"，intent filter 判定 `tier=chat`，仅给 2 个工具。

### 关键日志

```log
intent filter  tier=chat  toolCount=2

uhms: Qdrant payload search returned 0 hits, falling back to VFS  collection=sys_skills  query="browser automation"
uhms: Qdrant payload search returned 0 hits, falling back to VFS  collection=sys_skills  query=browser
uhms: Qdrant payload search returned 0 hits, falling back to VFS  collection=sys_skills  query="gateway status"
uhms: Qdrant payload search returned 0 hits, falling back to VFS  collection=sys_skills  query=exec_command
uhms: Qdrant payload search returned 0 hits, falling back to VFS  collection=sys_skills  query=status
uhms: Qdrant payload search returned 0 hits, falling back to VFS  collection=sys_skills  query="openacosmi gateway status"
uhms: Qdrant payload search returned 0 hits, falling back to VFS  collection=sys_skills  query="browser status"
```

7 次搜索全部返回空。`sys_skills` 集合在 Qdrant 中为空。

---

## Intent Filter 行为差异

### 判定为 `task` (24 工具)

```log
# 用户: "桌面有个 2 月 27 日的文件夹，里面是视频文件，帮我给它删除"
intent filter  tier=task  toolCount=24

# 用户: "删除"
intent filter  tier=task  toolCount=24
```

### 判定为 `chat` (仅 2 工具)

```log
# 用户: "我们使用浏览器自动化工具不行吗？不用灵瞳"
intent filter  tier=chat  toolCount=2

# 用户: "网关已经从新启动了，请完成未完成的任务"  ← 这是执行指令！
intent filter  tier=chat  toolCount=2

# 用户: "你还在吗？"
intent filter  tier=chat  toolCount=2
```

> **问题**: `"请完成未完成的任务"` 被判定为 chat 是直接导致后续 82 万 tokens 死循环的根因。

---

## 阶段三：重启后死循环 (15:56 → 16:01)

### 重启状态

网关在 15:55:27 关闭，15:56:03 重启成功：

```log
argus: ready  tools=16  pid=98339
native sandbox bridge started  pid=98338
```

但出现两个异常：

```log
gateway: UHMS disabled (enable via config memory.uhms.enabled)
skill_distributor: distribute complete  indexed=0  skipped=73  errors=0
```

### 用户消息与分类

```log
feishu message received  text=网关已经从新启动了，请完成未完成的任务
intent filter  tier=chat  toolCount=2
```

### 死循环轨迹 (27 轮)

智能体从记忆中知道要删除 `2月27日` 文件夹，但只有 `search_skills` 和 `lookup_skill`，疯狂搜索能执行命令的 skill：

| 轮次 | 工具 | 搜索词 | 结果 |
|------|------|--------|------|
| 0 | search_skills ×2 | `"openacosmi gateway status"` + `"file deletion"` | 空 |
| 1 | search_skills | `"shell command execution"` | 空 |
| 2 | search_skills | `"delete directory bash"` | 空 (503 重试) |
| 3-4 | search_skills | `"list files directory"`, `"exec bash shell"` | 空 |
| 5-6 | lookup_skill | `"exec"`, `"exec-approvals"` + `"elevated"` | 返回无关内容 |
| 7-13 | 混合 | 重复 `"exec command bash"`, `"coder"`, `"bash sh tool"` | 空/无关 |
| 14-26 | **完全循环** | 反复 `"exec-approvals"` + `"elevated"` + 各种变体 | 空/无关 |
| 27 | — | `context deadline exceeded` | 💀 超时 |

### Token 消耗

| 指标 | 数值 |
|------|------|
| LLM 调用次数 | 28 次 (第 28 次超时) |
| 总 input tokens | ≈ 820,000 |
| 总 output tokens | ≈ 720 |
| 请求 body 增长 | 40KB → 218KB |
| 耗时 | 5 分钟 |
| 503 重试 | 3 次 |
| **用户收到回复** | **无 (replyCount=0)** |

### 超时崩溃日志

```log
[GEMINI-DIAG] HTTP error (attempt 0): context deadline exceeded
run cleared  totalActive=0
dispatch_inbound: pipeline complete  replyCount=0
```

---

## 阶段四：自我强化错误认知 (16:04)

### 现象

用户问 "你还在吗？"，智能体回复：**"网关仍然断开，我处于能说话但不能动手的状态"**

### 关键日志

```log
intent filter  tier=chat  toolCount=2
transcript history loaded  messageCount=20
# 智能体回复:
"我在的。由于 Gateway（网关）仍然断开，我目前处于「能说话但不能动手」的状态"
```

### 原因分析

```
用户消息像"闲聊"
  → intent filter 判定 tier=chat (仅 2 工具)
    → 智能体无系统检测工具
      → 只能参考历史 transcript (20 条)
        → 历史全是 "网关坏了" 的记录
          → 复述 "网关仍然断开"
            → 错误写入新记忆 → 恶性循环
```

**事实**: 网关完全正常，Argus ready，native sandbox 正常。

---

## Skills 搜索引擎分析

### 搜索链路

```
SearchSystemEntries("sys_skills", query)
  ├─ [1] Qdrant SearchByPayload → 0 hits (集合为空)
  └─ [2] VFS fallback: searchSystemEntriesVFSFallback
           ├─ 扫描 _system/skills/ 下所有 meta.json
           ├─ tokenizeQuery(query) → 分词
           └─ vfsMetaKeywordScore(meta, terms) → 纯关键词匹配
```

### 为什么 Qdrant 集合为空？

```log
skill_distributor: distribute complete  indexed=0  skipped=73  errors=0
```

73 个 skills 全部 `skipped`。代码注释写着 "Qdrant 索引始终执行"，但实际结果是 0 个 indexed。

可能原因: `vectorMode=off` 导致 `vectorIndex` 为 nil 或不满足 `payloadUpserter` 接口，跳过了 Qdrant upsert。

### 为什么返回无关 skills？

VFS fallback 使用**纯关键词匹配** (`vfsMetaKeywordScore`)：

```go
// 拼接 name, description, tags, category, _l0_content
// 查看是否 strings.Contains(haystack, term)
```

当智能体搜 `"exec-approvals"` 时，VFS 中有名为 `exec-approvals` 的 Argus 工具定义文件，名称完全匹配。但其内容是关于**命令审批权限**，跟"执行 bash 命令"毫无语义关系。

**结论**: 搜索引擎没坏，但 (1) Qdrant 集合为空导致降级，(2) 关键词搜索返回名称匹配但语义无关的结果。

---

## Bug 清单

| # | 问题 | 严重度 | 建议修复 |
|---|------|--------|---------|
| 1 | Intent Filter 将执行指令判为 chat | 🔴 | 改进意图分类，或在 chat tier 保留 bash |
| 2 | chat tier 无迭代上限 | 🔴 | 限制 ≤5 轮，超出后强制回复用户 |
| 3 | Skills 索引全部跳过 | 🔴 | 排查 vectorMode=off 时 upsert 路径 |
| 4 | lookup_skill 无关内容膨胀上下文 | 🟡 | 限制返回内容长度或相关性阈值 |
| 5 | 超时无兜底回复 | 🔴 | replyCount=0 时发送错误提示 |
| 6 | UHMS 重启后被禁用 | 🟡 | 检查配置加载逻辑 |
| 7 | 错误认知自我强化 | 🟡 | 在 system prompt 注入当前系统状态 |
