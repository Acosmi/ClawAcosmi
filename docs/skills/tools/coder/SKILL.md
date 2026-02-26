---
name: coder
description: "Programming sub-agent: 9-layer fuzzy edit, file tools, sandboxed bash. Use coder_* tools for all code editing tasks."
metadata: |
  { "openacosmi": { "emoji": "💻" } }
---

# Coder — 编程子智能体使用指南

你拥有一组 `coder_*` 前缀的编程工具。**当用户请求编写、修改、搜索代码或执行命令时，必须优先使用 coder 工具**而非内置的 bash/read/write/edit 工具。

## 核心规则

1. **代码编辑 → 用 `coder_edit`**，不要用内置 `edit` 或 `write_file`
2. **读取代码文件 → 用 `coder_read`**，不要用内置 `read_file`
3. **创建新文件 → 用 `coder_write`**，不要用内置 `write_file`
4. **执行命令 → 用 `coder_bash`**（沙箱隔离，更安全），除非需要网络访问才用内置 `bash`
5. **代码搜索 → 用 `coder_grep` / `coder_glob`**，不要用内置 `search` / `glob`

## 什么时候用 Coder

- 用户要求编写、修改、重构、修复代码
- 用户要求搜索代码、查找文件
- 用户要求运行测试、构建、lint 等开发命令
- 用户要求创建新文件（代码、配置、脚本）
- 任何涉及工作区内文件操作的编程任务

## 什么时候不用 Coder

- 纯对话/问答（不涉及文件操作）
- 需要网络访问的命令（`coder_bash` 无网络）
- 操作工作区外的系统文件

## 工具清单

| 工具 | 用途 | 关键参数 |
|------|------|----------|
| `coder_edit` | 精确编辑文件（9 层模糊匹配） | `filePath`, `oldString`, `newString` |
| `coder_write` | 写入/创建文件 | `filePath`, `content` |
| `coder_read` | 读取文件内容 | `filePath`, `offset?`, `limit?` |
| `coder_bash` | 沙箱内执行命令 | `command`, `timeout?` |
| `coder_grep` | 正则搜索（ripgrep） | `pattern`, `path?`, `include?` |
| `coder_glob` | 文件名模式匹配 | `pattern`, `path?` |

## 编程工作流

### 典型流程

```
1. coder_grep/coder_glob → 定位目标文件和代码位置
2. coder_read → 理解上下文
3. coder_edit → 精确修改（每次一处）
4. coder_read → 验证修改结果
5. coder_bash → 运行测试/构建确认
```

### 最佳实践

- **先搜索后编辑**: 用 `coder_grep` 定位精确位置，避免盲改
- **原子操作**: 每次 `coder_edit` 只改一处，不要一次性大块替换
- **验证修改**: 编辑后用 `coder_read` 确认结果正确
- **分步执行**: 复杂修改分成多个小步骤，每步验证

### `coder_edit` 特性

支持 9 层模糊匹配，即使你提供的 `oldString` 有轻微缩进差异也能匹配成功：
精确匹配 → 忽略前导空白 → 忽略所有空白 → Levenshtein 距离 → 标准化空白 → 空行折叠 → 部分行匹配 → 块级相似度 → 子序列对齐

### `coder_bash` 安全特性

- 在沙箱内执行（macOS Seatbelt / Linux Landlock+Seccomp / Docker）
- 无网络访问（`NetworkPolicy::None`）
- 120 秒默认超时（可通过 `timeout` 参数调整）
- 适合运行: 测试、构建、lint、格式化、git 状态查询等

## 用户确认

`coder_edit`、`coder_write`、`coder_bash` 操作可能需要用户在聊天界面中确认后才会执行。如果操作被用户拒绝，不要重试，询问用户原因。
