# OpenAcosmi OS Keyring 敏感密钥保护架构设计

本文档详细介绍了 OpenAcosmi 系统自重构后所采用的新一代系统级敏感凭据存储架构，主要涵盖各大模型 API Key 以及各渠道 (如飞书、企业微信等) App Secret 的安全存储机制。

---

## 1. 核心面临痛点与前置架构

在原有的配置架构中，所有的全局配置（包含 UI 主题、环境变量、各模型驱动的 API 凭据、以及渠道 SDK 凭据）被统一存储在运行机器本地的 `openacosmi.json` 中。

* **存储介质**：纯文本 JSON / JSON5 文件。
* **安全级别**：基础操作系统文件权限（通常依赖 0600/0700 文件系统权限隔离）。
* **痛点**：此模式虽然部署轻量，但在企业级部署或多租户环境下，任何能读取该文件的守护进程、备份软件或越权恶意软件都可以直接以明文获取所有核心凭据。

## 2. 新一代 OS Keyring 架构机制

### 2.1 架构设计理念

新架构并没有完全抛弃基于 JSON 的全生命周期配置管理体系，而是采用了一种 **“占位符+底层脱壳” (Placeholder & Unwrapping)** 的非侵入式代理架构。

我们将系统底层（如 Mac 的 `Keychain`，Windows 的 `Credential Manager`，Linux 的 `Secret Service/KWallet`）视为最高级别的“凭据保险箱”。

### 2.2 敏感数据 “雷达测向” 拦截器

系统在保存配置之前，并没有采用硬编码的方式去死记硬背“哪个字段是密码”，而是采用了一种智能拦截器 (`internal/config/redact.go`)：

```go
var sensitiveKeyPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)token`),     // 匹配各种渠道的 bot/app/gateway Token
    regexp.MustCompile(`(?i)password`),  // 匹配所有密码
    regexp.MustCompile(`(?i)secret`),    // 匹配渠道的各种 Secret，如飞书 AppSecret
    regexp.MustCompile(`(?i)api.?key`),  // 匹配所有大模型的 apiKey
}
```

* **准入处理**：当这组正则特征在 JSON 的 `Key` 树中被匹配命中时，它的 `Value` 会被立即定性为敏感数据，并触发剥离机制。

### 2.3 工作流：写入阶段 (Write Pipeline)

1. **收集**：前端 UI/CLI 下发了包含明文密码（如飞书新配置的 `appSecret` 或 OpenAI 的 `apiKey`）的完整 `OpenAcosmiConfig` 配置结构。
2. **拦截并存入金库**：配置在落盘之前，系统执行 `MapStructToMapForKeyring` 和 `StoreSensitiveToKeyring`。系统会自动将提取出的明文密钥交给操作系统的底层安全框架保管。
    * *存储服务名示例*: `openacosmi_secrets`
    * *存储键名示例*: 路径特征组合 `channels_feishu_accounts_default_appsecret`
3. **占位符落盘**：将提取了密钥的对应字段强行修改为常量 `__OPENACOSMI_KEYRING_REF__` 占位符。
4. **生成 JSON**：此时序列化生成的 `openacosmi.json` 中，原本放置密码的位置全都是占位符。文件落盘。

### 2.4 工作流：读取与运行阶段 (Read Pipeline)

1. **加载配置**：系统的 Loader (`internal/config/loader.go`) 读取 `openacosmi.json`（里面充满占位符）。
2. **底层反向脱壳 (Unwrapping)**：在配置被反序列化绑定为强类型 `struct` 之前，通过调用 `RestoreFromKeyring()`。该函数根据特定的特征路径，向 OS Keyring 发起提取请求。
3. **静默重组**：从 Keychain 拿出的明文密钥，在系统进程的运行时**动态内存 (In-Memory)** 中被原地拼接放回原本的结构树节点上。
4. **安全传递**：内存中已经拥有了完整真实密码的配置树，被安全地传递给 Gateway 底层的网络客户端去建立连接。

---

## 3. 架构优势总结

1. **热更新 0 阻碍**：由于配置本身存在仅数百毫秒的内存刷新缓存，前端任何对于 API 密钥的最新修改，都能瞬间映射到 OS 底层钥串并立即被引擎采用，完全无需重启进程。
2. **合规性满足**：硬盘中物理遗留的数据永远不含密码明文。符合严苛的企业合规审查（不可明文存储机密）。
3. **泛化适用性极强 (飞书/钉钉等)**：只要插件设计者或渠道（如飞书的 Channel Provider）将凭据命名为合法合规的名字（如 `appSecret` 或 `botToken`），系统就能在开发者毫无感知的情况下，**自动**赋予它 OS 级别的存储防护，无需针对每一种新集成的通信 IM 单独写加密适配器。
4. **前端透明**：这套逻辑被封装在了最靠近文件 I/O 的抽象层。对外暴露的 API（获取配置预览等）时，经过原有 `redact.go` 机制兜底，前端只会认为密码被设置为了普通的“已隐藏”。从而极大地释放了前端的心智负担。
