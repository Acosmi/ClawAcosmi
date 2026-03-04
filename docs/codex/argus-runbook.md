# Argus 运维 Runbook（ARGUS-010）

> 灵瞳（Argus）安装、诊断、恢复、升级后处理标准手册。

---

## 1. 安装

### 1.1 从源码编译

```bash
cd Argus/go-sensory
go build -o argus-sensory ./cmd/sensory-server
```

### 1.2 安装到标准位置

```bash
# 方式 A: 拷贝到用户 bin 目录
mkdir -p ~/.openacosmi/bin
cp argus-sensory ~/.openacosmi/bin/
chmod +x ~/.openacosmi/bin/argus-sensory

# 方式 B: 配置指定路径（推荐）
# 编辑 ~/.openacosmi/openacosmi.json:
{
  "subAgents": {
    "screenObserver": {
      "binaryPath": "/absolute/path/to/argus-sensory"
    }
  }
}
```

### 1.3 打包安装（.app bundle）

```bash
cd Argus && make app
# 安装到 /Applications/Argus.app 或 ~/.openacosmi/Argus.app
```

---

## 2. 诊断

### 2.1 一键诊断 RPC

```json
// WebSocket 发送:
{"method": "argus.diagnose", "params": {}}

// 返回字段:
// - resolvedPath: 解析到的二进制路径
// - exists: 文件是否存在
// - executable: 是否可执行
// - codesign: 签名状态 (valid/unsigned_or_invalid/unknown)
// - tcc: TCC 权限状态
// - trace: 解析链路追踪 (每层 layer/path/found)
// - recovery: 恢复建议
```

### 2.2 命令行诊断

```bash
# 检查二进制路径解析
ls -la ~/.openacosmi/bin/argus-sensory

# 检查签名状态
codesign --verify --verbose=2 $(which argus-sensory)

# 检查 TCC 权限
# 屏幕录制
sqlite3 ~/Library/Application\ Support/com.apple.TCC/TCC.db \
  "SELECT * FROM access WHERE service='kTCCServiceScreenCapture'"

# 辅助功能
sqlite3 ~/Library/Application\ Support/com.apple.TCC/TCC.db \
  "SELECT * FROM access WHERE service='kTCCServiceAccessibility'"
```

### 2.3 日志检查

```bash
# 网关日志中搜索 argus 关键词
grep -i argus ~/.openacosmi/logs/gateway.log | tail -20
```

---

## 3. 恢复

### 3.1 `binary_not_found` 错误

**根因**: resolver 未找到可用二进制。

**恢复步骤**:

1. 确认二进制已编译且可执行：

   ```bash
   file /path/to/argus-sensory
   chmod +x /path/to/argus-sensory
   ```

2. 使用任一方式注册路径：
   - `export ARGUS_BINARY_PATH=/path/to/argus-sensory`（临时）
   - 在 `openacosmi.json` 中配置 `subAgents.screenObserver.binaryPath`（持久）
   - 拷贝到 `~/.openacosmi/bin/argus-sensory`
3. 重启网关或通过 UI 启用 Argus

### 3.2 `env_path_invalid` 错误

**根因**: `$ARGUS_BINARY_PATH` 指向的路径不存在。

**恢复**: 修正环境变量或移除（让 resolver 自动发现）。

### 3.3 TCC 权限被拒绝

**恢复步骤**:

1. 打开系统设置 > 隐私与安全性 > 屏幕录制/辅助功能
2. 添加或勾选 argus-sensory / Argus.app
3. 调用 `argus.permission.check` RPC 验证
4. 通过 `subagent.ctl` 的 `set_enabled` 操作重试启动

### 3.4 因签名失效重新授权

**根因**: 重新编译后二进制哈希变化，TCC 授权按哈希追踪。

**恢复步骤**:

1. 确保 `Argus Dev` 证书存在：

   ```bash
   security find-identity -v -p codesigning | grep "Argus Dev"
   ```

2. 如果不存在，创建：

   ```bash
   cd Argus/scripts/package && ./create-dev-cert.sh
   ```

3. 重启网关，`EnsureCodeSigned()` 将自动签名。

---

## 4. macOS 升级后处理

**症状**: 升级 macOS 后 Argus 无法截屏。

**原因**: 系统升级重置 TCC 权限数据库。

**恢复步骤**:

1. 调用 `argus.permission.check` 检查权限状态
2. 按照 §3.3 重新授权
3. 如果使用 Sequoia (15+)，注意月度重授权机制
   - `argus.permission.check` 返回的 `screen_recording_days_left` 字段可查看剩余天数
   - 建议在到期前主动重授权

---

## 5. Resolver 优先级参考

| 优先级 | 层 | 来源 | 说明 |
|--------|-----|------|------|
| 1 | env | `$ARGUS_BINARY_PATH` | 环境变量覆盖 |
| 2 | config | `subAgents.screenObserver.binaryPath` | 配置文件持久化 |
| 3 | app_bundle | `.app/Contents/MacOS/argus-sensory` | 签名 bundle |
| 4 | user_bin | `~/.openacosmi/bin/argus-sensory` | 用户级安装/符号链接 |
| 5 | path | `PATH` 搜索 | exec.LookPath |
