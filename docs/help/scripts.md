---
summary: "仓库脚本：用途、范围和安全说明"
read_when:
  - 运行仓库中的脚本
  - 在 ./scripts 目录下添加或修改脚本
title: "脚本"
---

> [!NOTE]
> 本文档已更新以适配 **Rust CLI + Go Gateway** 混合架构。

# 脚本

`scripts/` 目录包含用于本地工作流和运维任务的辅助脚本。
当任务明确与脚本关联时使用这些脚本；否则请优先使用 CLI。

## 约定

- 脚本是**可选的**，除非在文档或发布检查清单中被引用。
- 当 CLI 已提供对应功能时，优先使用 CLI（例如：认证监控使用 `openacosmi models status --check`）。
- 假定脚本可能依赖特定主机环境；在新机器上运行前请先阅读脚本内容。

## 认证监控脚本

认证监控脚本的文档位于：
[/automation/auth-monitoring](/automation/auth-monitoring)

## 添加脚本时

- 保持脚本功能聚焦且有文档说明。
- 在相关文档中添加简短条目（如没有相关文档则创建一个）。
