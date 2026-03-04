---
summary: "Agent 工作区：位置、文件布局和备份策略"
read_when:
  - 需要了解 Agent 工作区及其文件布局
  - 需要备份或迁移 Agent 工作区
title: "Agent 工作区"
status: active
arch: go-gateway
---

# Agent 工作区

> [!IMPORTANT]
> **架构状态**：工作区由 **Go Gateway** 管理（`backend/internal/agents/`）。
> Rust CLI 的 `openacosmi setup` 命令（`oa-cmd-supporting`）可初始化工作区文件。

工作区是 Agent 的家目录，是文件工具和工作区上下文使用的唯一工作目录。请将其视为私有记忆。

工作区与 `~/.openacosmi/` 是分开的，后者存储配置、凭证和会话。

**重要：** 工作区是**默认 cwd**，不是硬沙箱。工具将相对路径解析到工作区内，但绝对路径仍可到达主机其他位置，除非启用了沙箱。如需隔离，请使用 [`agents.defaults.sandbox`](/gateway/sandboxing)（和/或按 Agent 沙箱配置）。启用沙箱且 `workspaceAccess` 不为 `"rw"` 时，工具在 `~/.openacosmi/sandboxes` 下的沙箱工作区中操作。

## 默认位置

- 默认：`~/.openacosmi/workspace`
- 如果设置了 `OPENACOSMI_PROFILE` 且不为 `"default"`，默认变为 `~/.openacosmi/workspace-<profile>`。
- 在 `~/.openacosmi/openacosmi.json` 中覆盖：

```json5
{
  agent: {
    workspace: "~/.openacosmi/workspace",
  },
}
```

`openacosmi onboard`、`openacosmi configure` 或 `openacosmi setup` 将创建工作区并在缺失时填充 bootstrap 文件。

如果你已自行管理工作区文件，可禁用 bootstrap 文件创建：

```json5
{ agent: { skipBootstrap: true } }
```

## 额外工作区目录

旧版安装可能创建了 `~/openacosmi`。保留多个工作区目录可能导致认证或状态漂移混乱，因为同一时间只有一个工作区处于活动状态。

**建议：** 保留单一活动工作区。如不再使用额外目录，将其归档或移至废纸篓（例如 `trash ~/openacosmi`）。如有意保留多个工作区，确保 `agents.defaults.workspace` 指向活动工作区。

`openacosmi doctor` 在检测到额外工作区目录时会发出警告。

## 工作区文件映射（各文件含义）

OpenAcosmi 在工作区内期望的标准文件：

- `AGENTS.md`
  - Agent 的操作指令及如何使用记忆。
  - 每次会话启动时加载。
  - 适合放置规则、优先级和"行为方式"细节。

- `SOUL.md`
  - 人格、语气和边界。
  - 每次会话加载。

- `USER.md`
  - 用户身份和称呼方式。
  - 每次会话加载。

- `IDENTITY.md`
  - Agent 的名称、风格和表情符号。
  - 在引导仪式期间创建/更新。

- `TOOLS.md`
  - 关于本地工具和约定的说明。
  - 不控制工具可用性；仅为指导。

- `HEARTBEAT.md`
  - 可选的心跳运行检查清单。
  - 保持简短以避免 token 消耗。

- `BOOT.md`
  - 可选的启动检查清单，在启用内部 Hook 时于 Gateway 重启时执行。
  - 保持简短；使用消息工具进行出站发送。

- `BOOTSTRAP.md`
  - 一次性首次运行仪式。
  - 仅为全新工作区创建。
  - 完成仪式后删除。

- `memory/YYYY-MM-DD.md`
  - 每日记忆日志（每天一个文件）。
  - 建议在会话启动时读取今天 + 昨天的日志。

- `MEMORY.md`（可选）
  - 精选的长期记忆。
  - 仅在主私有会话中加载（不在共享/群组上下文中加载）。

参见 [记忆](/concepts/memory)。

- `skills/`（可选）
  - 工作区特定的 Skills。
  - 名称冲突时覆盖托管/捆绑 Skills。

- `canvas/`（可选）
  - Canvas UI 文件用于节点显示（例如 `canvas/index.html`）。

如果任何 bootstrap 文件缺失，OpenAcosmi 将注入"缺失文件"标记到会话中并继续。大文件注入时会被截断；通过 `agents.defaults.bootstrapMaxChars`（默认：20000）调整限制。`openacosmi setup` 可重建缺失的默认文件而不覆盖已有文件。

## 不在工作区内的内容

以下位于 `~/.openacosmi/` 下，**不应**提交到工作区仓库：

- `~/.openacosmi/openacosmi.json`（配置）
- `~/.openacosmi/credentials/`（OAuth 令牌、API 密钥）
- `~/.openacosmi/agents/<agentId>/sessions/`（会话转录 + 元数据）
- `~/.openacosmi/skills/`（托管 Skills）

如需迁移会话或配置，请单独复制并保持在版本控制之外。

## Git 备份（推荐，私有）

将工作区视为私有记忆。放入**私有** Git 仓库以便备份和恢复。

在 Gateway 运行的机器上执行以下步骤（工作区位于那里）。

### 1）初始化仓库

如果安装了 git，全新工作区会自动初始化。如果此工作区尚非仓库，运行：

```bash
cd ~/.openacosmi/workspace
git init
git add AGENTS.md SOUL.md TOOLS.md IDENTITY.md USER.md HEARTBEAT.md memory/
git commit -m "添加 agent 工作区"
```

### 2）添加私有远程仓库

**GitHub Web UI 方式：**

1. 在 GitHub 上创建一个新的**私有**仓库。
2. 不要用 README 初始化（避免合并冲突）。
3. 复制 HTTPS 远程 URL。
4. 添加远程并推送：

```bash
git branch -M main
git remote add origin <https-url>
git push -u origin main
```

**GitHub CLI 方式：**

```bash
gh auth login
gh repo create openacosmi-workspace --private --source . --remote origin --push
```

### 3）日常更新

```bash
git status
git add .
git commit -m "更新记忆"
git push
```

## 不要提交密钥

即使在私有仓库中，也避免在工作区中存储密钥：

- API 密钥、OAuth 令牌、密码或私有凭证。
- `~/.openacosmi/` 下的任何内容。
- 聊天或敏感附件的原始转储。

如必须存储敏感引用，使用占位符并将真实密钥保存在其他地方（密码管理器、环境变量或 `~/.openacosmi/`）。

建议的 `.gitignore`：

```gitignore
.DS_Store
.env
**/*.key
**/*.pem
**/secrets*
```

## 迁移工作区到新机器

1. 将仓库克隆到目标路径（默认 `~/.openacosmi/workspace`）。
2. 在 `~/.openacosmi/openacosmi.json` 中将 `agents.defaults.workspace` 设为该路径。
3. 运行 `openacosmi setup --workspace <path>` 以填充缺失文件。
4. 如需会话，从旧机器单独复制 `~/.openacosmi/agents/<agentId>/sessions/`。

## 高级说明

- 多 Agent 路由可为每个 Agent 使用不同的工作区。参见 [频道路由](/channels/channel-routing)。
- 如果启用了 `agents.defaults.sandbox`，非主会话可使用 `agents.defaults.sandbox.workspaceRoot` 下的按会话沙箱工作区。
