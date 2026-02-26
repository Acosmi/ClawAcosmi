

# ---

**🛡️ Rust 原生沙箱设计方案（oa-sandbox）**

**核心目标**：替代笨重的 Docker CLI，使用各平台操作系统原生隔离机制（OS-native Sandbox），实现免依赖部署与 10ms 级极速冷启动，构建“开箱即用”的桌面级 AI 安全防线。

## **一、 背景与痛点**

当前沙箱通过 Go 语言调用 docker run 实现，存在以下严重阻碍 C 端桌面智能体落地的痛点：

1. **启动延迟极高**：容器冷启动耗时 500ms \- 2s，且默认无挂载（智能体看不到本地工作区），严重打断 Agent 连续思考链路。  
2. **极高的部署门槛**：强依赖 Docker Desktop，对非技术出身的普通桌面小白用户极其不友好。  
3. **权限流转断裂**：L1 级别的 write\_file 操作在 Go 层被硬拒，未能真正进入沙箱内流转执行。

**破局点**：当前 CLI 层已全面切入 Rust 生态（cli-rust/，包含 22 个 crate）。Rust 对操作系统底层 API 的封装极其成熟，开发纯 Rust 的原生沙箱库，既能共享现有工具链，又能彻底摆脱 Docker 依赖。

## ---

**二、 核心 OS 机制选型与技术排雷 (🚨 关键)**

| 平台 | OS 底层核心机制 | Rust 生态支持 | 落地排雷指南 (Gotchas & Mitigations) |
| :---- | :---- | :---- | :---- |
| **Linux** | Namespace \+ Seccomp \+ Cgroup v2 \+ **Landlock** | sandbox-rs / libc | 现代桌面 Linux 常禁用无特权 CLONE\_NEWUSER。**必须引入 Landlock LSM 作为无特权降级方案**，并充当 PID 1 收割孤儿进程。 |
| **macOS** | Seatbelt (sandbox-exec) | sandbox-exec 动态调用 | Apple 虽将 CLI 标记弃用，但底层内核机制未废弃。**必须在 .sb 中引入 (import "system.sb")**，否则 Python/Node 等解释器会因无法加载系统基础库而直接崩溃 (Killed: 9)。 |
| **Windows** | AppContainer \+ **Job Objects** | windows crate 直调 Win32 | AppContainer 基于 SID 隔离且无挂载概念，**必须动态修改 Workspace 的 NTFS ACL 赋予临时读写权限**；使用 Job Objects (作业对象) 确保子孙进程树被干净收割。 |

## ---

**三、 系统架构设计**

采用 **“Go 并发编排 \+ Rust 底层压榨”** 的进程解耦架构。通过 exec.Command 调用独立编译的 Rust 二进制，并通过\*\*标准流（JSON 格式）\*\*进行高可靠 IPC 通信，彻底避免原生文本流解析错误与跨语言 CGO 内存泄漏。

### **1\. 执行链路架构图**

代码段

graph TB  
    subgraph "Go 调度层（Agent Core 管控与编排）"  
        A\[attempt\_runner.go\] \--\> B\[tool\_executor.go\]  
        B \-.-\>|1. exec.CommandContext\<br/\>(传递参数)| C  
        C \-.-\>|5. stdout 返回 JSON 结果| B  
    end  
      
    subgraph "oa-sandbox (Rust 核心运行层)"  
        C\["oa-sandbox 路由引擎"\] \--\> D{检测当前操作系统}  
        D \--\>|Linux| E\["runner\_linux.rs\<br/\>Namespace / Landlock \+ Cgroup"\]  
        D \--\>|macOS| F\["runner\_macos.rs\<br/\>Seatbelt profile 动态生成"\]  
        D \--\>|Windows| G\["runner\_windows.rs\<br/\>AppContainer / Job Objects \+ ACL"\]  
        D \--\>|Fallback| J\["runner\_docker.rs\<br/\>原生API受限时无缝降级 Docker"\]  
    end  
      
    E \--\> H\[隔离进程树执行 Agent 命令\]  
    F \--\> H  
    G \--\> H  
    J \--\> H  
      
    H \--\> I\[僵尸进程收割 / 资源清理 / 撤销临时 ACL\]  
    I \--\> C

### **2\. Workspace 工程代码布局**

Plaintext

cli-rust/crates/  
├── oa-sandbox/              ← \[NEW\] 沙箱运行时核心库  
│   ├── Cargo.toml  
│   └── src/  
│       ├── lib.rs           \# 统一 SandboxConfig, SandboxRunner Trait, JSON Output  
│       ├── config.rs        \# SecurityLevel, MountSpec, ResourceLimits, NetworkPolicy  
│       ├── runner\_linux.rs  \# Linux: sandbox-rs (namespace) / Landlock 兜底机制  
│       ├── runner\_macos.rs  \# macOS: Seatbelt profile 动态生成器  
│       ├── runner\_windows.rs\# Windows: SID 隔离 \+ ACL 赋权 \+ Job Objects 收割  
│       └── runner\_docker.rs \# Fallback: 保底调用 docker CLI（保障 SLA 高可用）  
│  
├── oa-cmd-sandbox/          ← \[MODIFY\] CLI 入口层  
│   └── src/  
│       ├── run.rs           ← \[NEW\] \`openacosmi sandbox run\` 子命令解析  
│       └── ...

## ---

**四、 安全级别映射与网络策略**

\[\!WARNING\]

**修复了原方案的网络悖论**：Agent 执行代码（如 npm install, pip install）强依赖网络，L1 一刀切断网会导致代码解释器失灵。必须引入 **受控网络 (Restricted Network)** 策略。

| 级别 | 文件系统 (工作区/自身系统区) | 命令执行 | 网络限制策略 (--net) 🚨 | 资源限制与僵尸进程收割 |
| :---- | :---- | :---- | :---- | :---- |
| **L0 (Deny)** | 工作区: RO / 系统区: 拒绝 | ✅ 允许 (沙箱内) | ❌ **彻底禁止** (none) *防静默窃取私钥外发* | 256MB / 1 CPU / 强制收割 |
| **L1 (Sandbox)** | 工作区: **RW** / 系统区: RO | ✅ 允许 (沙箱内) | 🛡️ **受控出站** (restricted) *放行公网拉包，系统级拦截局域网及 127.0.0.0/8 防 SSRF* | 512MB / 2 CPU / 强制收割 |
| **L2 (Full)** | 宿主机无限制直接读写 | ✅ **需人类审批 (Dry Run)** | 🌐 **无限制** (host) | 无限制 |

*(RO \= 只读 Read-Only，RW \= 读写 Read-Write)*

## ---

**五、 统一 CLI 接口与 Go 调度集成**

**架构优化**：放弃纯文本输出，强制使用 \--format json，让 Rust 将沙箱内的 stdout、stderr、exit\_code 和错误信息结构化，避免 Go 层产生截断。

### **1\. CLI 调用示例**

Bash

\# L1 沙箱读写模式 (Agent 核心生产环境：受控网络，允许拉包，禁止内网扫描)  
openacosmi sandbox run \\  
  \--security sandbox \\  
  \--workspace /path/to/project \\  
  \--skills \~/.openacosmi/skills \\  
  \--net restricted \\  
  \--timeout 120 \\  
  \--format json \\  
  \-- sh \-c "npm install && npm test"

### **2\. Go 端接入规范 (tool\_executor.go)**

Go

type SandboxResult struct {  
    Stdout   string \`json:"stdout"\`  
    Stderr   string \`json:"stderr"\`  
    ExitCode int    \`json:"exit\_code"\`  
    Error    string \`json:"error,omitempty"\`  
}

func executeBashNativeSandbox(ctx context.Context, input bashInput, params ToolExecParams) (\*SandboxResult, error) {  
    args := \[\]string{"sandbox", "run",  
        "--security", params.SecurityLevel,  
        "--workspace", params.WorkspaceDir,  
        "--net", getNetworkPolicy(params.SecurityLevel), // 返回 "restricted" 或 "none"  
        "--timeout", fmt.Sprintf("%d", params.TimeoutMs/1000),  
        "--format", "json", // 强制标准化 JSON 通信  
        "--", "sh", "-c", input.Command,  
    }  
      
    // Rust 二进制崩溃不影响 Go 主进程  
    cmd := exec.CommandContext(ctx, "openacosmi", args...)  
    outputBytes, err := cmd.CombinedOutput()  
      
    var res SandboxResult  
    if parseErr := json.Unmarshal(outputBytes, \&res); parseErr \!= nil {  
        return nil, fmt.Errorf("sandbox crash or invalid output: %v, raw: %s", parseErr, string(outputBytes))  
    }  
    return \&res, err  
}

## ---

**六、 三大平台底层核心代码落地骨架 (排雷版)**

### **🐧 1\. Linux（主战平台：双保险机制）**

结合 sandbox-rs (Namespace) 与 Landlock，必须处理 UID 映射与 PID 1 孤儿进程收割。

Rust

// runner\_linux.rs 核心逻辑概览  
pub fn run(config: \&SandboxConfig) \-\> Result\<ExecResult\> {  
    let mut builder \= sandbox::SandboxBuilder::new()  
        .memory\_limit(config.memory\_mb \* 1024 \* 1024)  
        .seccomp\_profile(SeccompProfile::Default);

    // 🚨 僵尸进程收割保障 (作为 PID 1 启动) & UID 防 nobody 错乱  
    builder \= builder.uid\_mapping(HostUid, ContainerUid).init\_process(true);

    match builder.command(\&config.command).run() {  
        Ok(result) \=\> Ok(result),  
        Err(e) if is\_unprivileged\_error(\&e) \=\> {  
            // 🚨 降级方案：普通用户 Namespace 被禁时，无缝降级到 Linux 5.13+ 的 Landlock LSM  
            log::warn\!("Namespace unprivileged, downgrading to Landlock");  
            apply\_landlock\_rules(\&config.mounts)?;  
            run\_with\_cgroup\_limits(config)  
        },  
        Err(e) \=\> Err(e),  
    }  
}

### **🍎 2\. macOS（Seatbelt 动态规避崩溃）**

避免因屏蔽系统库导致 Node/Python 解释器启动即崩溃。

Rust

// runner\_macos.rs 核心逻辑概览  
fn generate\_seatbelt\_profile(config: \&SandboxConfig) \-\> String {  
    let mut rules \= vec\!\[  
        "(version 1)",   
        "(deny default)",  
        "(import \\"system.sb\\")", // 🚨 极其重要：引入 macOS 系统核心基础允许规则  
    \];  
      
    // 允许解释器读取基础依赖、动态链接库和随机数池  
    rules.push("(allow process-exec\* (subpath \\"/bin\\") (subpath \\"/usr/bin\\") (subpath \\"/usr/local\\") (subpath \\"/opt/homebrew\\"))");  
    rules.push("(allow process-fork)");  
    rules.push("(allow sysctl-read)");  
    rules.push("(allow file-read\* (subpath \\"/usr/lib\\") (subpath \\"/System\\") (literal \\"/dev/urandom\\"))");  
      
    // 注入 Workspace 挂载逻辑 (略)  
      
    // 网络控制 (Restricted 模式：外网放行，内网拦截)  
    if config.network \== NetworkPolicy::Restricted {  
        rules.push("(allow network-outbound (remote ip))");  
        rules.push("(deny network-outbound (remote ip \\"127.0.0.0/8\\") (remote ip \\"192.168.0.0/16\\") (remote ip \\"10.0.0.0/8\\"))");  
    }  
      
    rules.join("\\n")  
}

### **🪟 3\. Windows（AppContainer 与 ACL 动态提权）**

突破 Windows 没有 Bind Mount 的局限，解决 Access Denied 问题。

Rust

// runner\_windows.rs 核心逻辑概览  
pub fn run(config: \&SandboxConfig) \-\> Result\<ExecResult\> {  
    // 1\. 创建 AppContainer SID  
    let sid \= create\_app\_container\_profile("acosmi-sandbox")?;  
      
    // 2\. 🚨 核心跨越：动态修改工作区目录的 NTFS ACL，临时赋予该受限 SID 读写权限  
    grant\_ntfs\_acl\_to\_sid(\&config.workspace, \&sid, config.is\_readonly)?;  
      
    // 3\. 🚨 进程树管控：创建 Job Object，配置 JOB\_OBJECT\_LIMIT\_KILL\_ON\_JOB\_CLOSE  
    // 确保超时或被 Kill 后，能够一键连根拔除所有衍生的后台孙进程 (\`nohup\` 等)  
    let job \= create\_job\_object\_with\_limits(config.memory\_mb)?;  
      
    let process \= create\_sandboxed\_process(\&config.command, \&sid)?;  
    assign\_process\_to\_job(\&process, \&job)?;  
      
    let result \= wait\_and\_collect(process, config.timeout);  
      
    // 4\. 清理善后：必须撤销注入的临时 NTFS ACL，防止提权漏洞  
    revoke\_ntfs\_acl\_from\_sid(\&config.workspace, \&sid)?;  
      
    result  
}

## ---

**七、 分阶段实施计划**

### **Phase 1：核心骨架 \+ Linux 后端验证（约 2 周）**

* 新建 oa-sandbox crate，定义标准化 JSON IPC 通信契约。  
* 完成 Linux Namespace 后端与 Landlock Fallback 双通道机制。  
* 验证 L1 写入文件到主宿主机时的 UID/GID 映射正常，跑通 PID 1 僵尸进程收割。

### **Phase 2：macOS 适配 \+ Docker 兜底方案（约 1 周）**

* 开发 macOS Seatbelt profile 生成器，跑通 Python/Node 环境调用测试。  
* 开发 **Docker 降级模式 (runner\_docker.rs)**：当检测到系统原生沙箱拉起失败（如被杀毒软件拦截）时，无缝切回原有 Docker 方案，**死守产品可用性底线 (SLA)**。

### **Phase 3：Windows 攻坚与全平台 CI（约 2 周）**

* 攻坚 Windows AppContainer SID 权限分配及 NTFS ACL 动态修改。  
* 引入 Windows Job Objects，模拟极端情况（如后台死循环脚本），验证进程树能被 100% 收割。  
* 全平台 CI 交叉编译流水线搭建与端到端集成测试。

### **Phase 4：Go 调度层全面切流上线（约 1 周）**

* 修复 Go 端 attempt\_runner.go L1 权限解除硬拒，将 payload 真正转交 Rust CLI。  
* L2 全局权限执行前，引入 **Dry Run（动作预览）弹窗** 拦截机制。  
* 内部模拟 Prompt 越狱注入攻击，验证 \--net restricted 防内网探测能力。

## ---

**八、 风险评估与缓解策略**

| 风险类别 | 风险描述 | 危害等级 | 缓解策略 (Mitigation) |
| :---- | :---- | :---- | :---- |
| **资源泄漏** | Agent 启动后台驻留进程导致 CLI 退出后进程持续占用 CPU | 🔴 高 | **必须严格落实 Phase 3 的收割机制**：Linux 启用 PID 1 / Cgroup；Windows 强行挂载 Job Objects。 |
| **安全绕过** | L1 放开网络后，恶意 NPM 包触发本地环回扫描攻击 (SSRF) | 🔴 高 | 强制执行 \--net restricted，利用 OS 规则硬拦截 127.0.0.0/8 和 192.168.x.x。 |
| **系统兼容** | Windows AppContainer 对传统 .exe 兼容性极差 | 🟡 中 | 若开发受阻，立刻降级为 Job Objects (限制资源) \+ Restricted Tokens (受限令牌剥夺管理员权限) 组合拳，或直接默认走 Docker Fallback。 |
| **平台演进** | macOS 彻底物理移除 sandbox-exec | 🟢 低 | Apple 自身核心服务及 Chrome 重度依赖 MACF 内核，短期（3-5年）无忧。远期技术债可迁移至 macOS 原生 Virtualization.framework。 |

