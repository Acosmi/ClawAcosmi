---
summary: "在 OpenAcosmi 中使用 Amazon Bedrock（Converse API）模型"
read_when:
  - 使用 Amazon Bedrock 模型
  - 需要 AWS 凭据/区域配置
title: "Amazon Bedrock"
status: active
arch: rust-cli+go-gateway
---

# Amazon Bedrock

> [!IMPORTANT]
> **架构状态**：系统采用 **Rust CLI + Go Gateway 双二进制架构**。
>
> - Bedrock 模型发现与调用由 **Go Gateway** 处理（`backend/internal/agents/models/`）
> - Onboard 流程由 **Rust CLI** 实现（`cli-rust/crates/oa-cmd-onboard/`）
> - AWS 凭据链在 Gateway 主机环境中解析

OpenAcosmi 支持通过 pi‑ai 的 **Bedrock Converse** 流式供应商使用 **Amazon Bedrock** 模型。Bedrock 认证使用 **AWS SDK 默认凭据链**，不需要 API Key。

## pi‑ai 支持内容

- 供应商：`amazon-bedrock`
- API：`bedrock-converse-stream`
- 认证：AWS 凭据（环境变量、共享配置或实例角色）
- 区域：`AWS_REGION` 或 `AWS_DEFAULT_REGION`（默认：`us-east-1`）

## 自动模型发现

如果检测到 AWS 凭据，OpenAcosmi 可以自动发现支持**流式传输**和**文本输出**的 Bedrock 模型。发现使用 `bedrock:ListFoundationModels`，结果将被缓存（默认 1 小时）。

配置选项位于 `models.bedrockDiscovery` 下：

```json5
{
  models: {
    bedrockDiscovery: {
      enabled: true,
      region: "us-east-1",
      providerFilter: ["anthropic", "amazon"],
      refreshInterval: 3600,
      defaultContextWindow: 32000,
      defaultMaxTokens: 4096,
    },
  },
}
```

说明：

- `enabled` 在检测到 AWS 凭据时默认为 `true`。
- `region` 默认使用 `AWS_REGION` 或 `AWS_DEFAULT_REGION`，然后是 `us-east-1`。
- `providerFilter` 匹配 Bedrock 供应商名称（例如 `anthropic`）。
- `refreshInterval` 单位为秒；设为 `0` 禁用缓存。
- `defaultContextWindow`（默认 `32000`）和 `defaultMaxTokens`（默认 `4096`）用于发现的模型（如果知道模型限制可覆盖）。

## 手动设置

1. 确保 AWS 凭据在 **Gateway 主机**上可用：

```bash
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_REGION="us-east-1"
# 可选：
export AWS_SESSION_TOKEN="..."
export AWS_PROFILE="your-profile"
# 可选（Bedrock API Key / Bearer Token）：
export AWS_BEARER_TOKEN_BEDROCK="..."
```

1. 在配置中添加 Bedrock 供应商和模型（不需要 `apiKey`）：

```json5
{
  models: {
    providers: {
      "amazon-bedrock": {
        baseUrl: "https://bedrock-runtime.us-east-1.amazonaws.com",
        api: "bedrock-converse-stream",
        auth: "aws-sdk",
        models: [
          {
            id: "us.anthropic.claude-opus-4-6-v1:0",
            name: "Claude Opus 4.6 (Bedrock)",
            reasoning: true,
            input: ["text", "image"],
            cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 },
            contextWindow: 200000,
            maxTokens: 8192,
          },
        ],
      },
    },
  },
  agents: {
    defaults: {
      model: { primary: "amazon-bedrock/us.anthropic.claude-opus-4-6-v1:0" },
    },
  },
}
```

## EC2 实例角色

在附加了 IAM 角色的 EC2 实例上运行 OpenAcosmi 时，AWS SDK 会自动使用实例元数据服务（IMDS）进行认证。但 OpenAcosmi 的凭据检测目前只检查环境变量，不检查 IMDS 凭据。

**解决方法：** 设置 `AWS_PROFILE=default` 表示 AWS 凭据可用。实际认证仍通过 IMDS 使用实例角色。

```bash
# 添加到 ~/.bashrc 或 shell 配置
export AWS_PROFILE=default
export AWS_REGION=us-east-1
```

**所需 IAM 权限**（EC2 实例角色）：

- `bedrock:InvokeModel`
- `bedrock:InvokeModelWithResponseStream`
- `bedrock:ListFoundationModels`（用于自动发现）

或附加托管策略 `AmazonBedrockFullAccess`。

**快速设置：**

```bash
# 1. 创建 IAM 角色和实例配置文件
aws iam create-role --role-name EC2-Bedrock-Access \
  --assume-role-policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Principal": {"Service": "ec2.amazonaws.com"},
      "Action": "sts:AssumeRole"
    }]
  }'

aws iam attach-role-policy --role-name EC2-Bedrock-Access \
  --policy-arn arn:aws:iam::aws:policy/AmazonBedrockFullAccess

aws iam create-instance-profile --instance-profile-name EC2-Bedrock-Access
aws iam add-role-to-instance-profile \
  --instance-profile-name EC2-Bedrock-Access \
  --role-name EC2-Bedrock-Access

# 2. 附加到 EC2 实例
aws ec2 associate-iam-instance-profile \
  --instance-id i-xxxxx \
  --iam-instance-profile Name=EC2-Bedrock-Access

# 3. 在 EC2 实例上启用发现
openacosmi config set models.bedrockDiscovery.enabled true
openacosmi config set models.bedrockDiscovery.region us-east-1

# 4. 设置解决方法环境变量
echo 'export AWS_PROFILE=default' >> ~/.bashrc
echo 'export AWS_REGION=us-east-1' >> ~/.bashrc
source ~/.bashrc

# 5. 验证模型是否被发现
openacosmi models list
```

## 注意事项

- Bedrock 需要在 AWS 账户/区域中启用**模型访问权限**。
- 自动发现需要 `bedrock:ListFoundationModels` 权限。
- 如果使用 profile，请在 Gateway 主机上设置 `AWS_PROFILE`。
- OpenAcosmi 按以下顺序解析凭据来源：`AWS_BEARER_TOKEN_BEDROCK` → `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` → `AWS_PROFILE` → 默认 AWS SDK 链。
- Reasoning 支持取决于模型；请查看 Bedrock 模型卡了解当前能力。
- 如果你更喜欢托管密钥流程，也可以在 Bedrock 前放置一个 OpenAI 兼容的代理，然后作为 OpenAI 供应商进行配置。
