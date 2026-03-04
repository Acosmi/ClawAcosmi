---
summary: "macOS 应用技能集成"
read_when:
  - 在 macOS 上加载或管理技能
title: "技能"
---

# macOS 应用技能集成

## 概览

技能通过 Go Gateway 的配置系统维护。macOS 应用不直接管理技能文件，
而是通过 Gateway WebSocket 读取/写入技能配置。

## 技能存储

技能文件位于 Gateway 主机上：

- 全局技能：`~/.openacosmi/skills/`
- 工作区技能：`~/.openacosmi/workspace/skills/`

## CLI 管理

```bash
openacosmi skills list
openacosmi skills install <skill-name>
openacosmi skills remove <skill-name>
```

## 说明

- macOS 应用的设置面板可能显示已加载的技能列表。
- 技能的实际加载和执行由 Go Gateway 处理。
