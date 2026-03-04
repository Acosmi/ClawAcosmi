---
summary: "如何在本地运行测试及使用覆盖率模式"
read_when:
  - 运行或修复测试
title: "测试"
status: active
arch: rust-cli+go-gateway
---

# 测试

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - Go Gateway 测试：`cd backend && go test ./...`
> - Rust CLI 测试：`cd cli-rust && cargo test`
> - UI 测试：`cd ui && npx vitest run`
> - Gateway 默认端口：`19001`

- 完整测试套件：参见 [Testing](/help/testing)

## Go Gateway 测试

```bash
cd backend && go test ./...
```

- 端到端测试（多实例 WS/HTTP/节点配对）：`go test ./internal/gateway/ -run TestE2E`
- 供应商实时测试（minimax/zai）：需要 API Key 和 `LIVE=1`

## Rust CLI 测试

```bash
cd cli-rust && cargo test
```

## UI 测试

```bash
cd ui && npx vitest run
```

- 覆盖率：`npx vitest run --coverage`（全局阈值：70% 行/分支/函数/语句）

## 模型延迟基准测试（本地密钥）

脚本：`scripts/bench-model.ts`

用法：

- `source ~/.profile && pnpm tsx scripts/bench-model.ts --runs 10`
- 可选环境变量：`MINIMAX_API_KEY`、`MINIMAX_BASE_URL`、`MINIMAX_MODEL`、`ANTHROPIC_API_KEY`
- 默认提示："Reply with a single word: ok. No punctuation or extra text."

## Onboarding E2E（Docker）

Docker 可选；仅用于容器化 onboarding 冒烟测试。

完整冷启动流程：

```bash
scripts/e2e/onboard-docker.sh
```

此脚本通过伪终端驱动交互式向导，验证配置/工作区/会话文件，然后启动 Gateway 并运行 `openacosmi health`。
