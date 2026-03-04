# GUI Grounding 升级规划

> ✅ **已完成** | 来源: `docs/renwu/global-health-audit.md` 审计发现 | 日期: 2026-02-15

## 背景

### 当前架构问题

`ui_parser.go` 已实现完整的 SoM (Set-of-Mark) Grounding 管线：

```
截屏 → [VLM 检测UI元素] → [SoM 标注] → [VLM 选编号] → 执行
            ↑ 瓶颈：慢 + 不准
```

**问题**：`DetectElements()` 使用 VLM 检测 UI 元素，每次调用消耗 1-3 秒 + 大量 token，且坐标精度差。

### macOS AXUIElement API 方案

macOS 提供原生 Accessibility API (`AXUIElement`)，可以：

- **零延迟** 获取所有 UI 元素的 bounding box、role、label
- **精确到像素** 的坐标，不依赖任何 ML 模型
- 返回结构化元素树（类似 DOM），包含 button/textfield/menu 等类型

### 技术选型：Rust

**选用 Rust 实现，原因：**

| 因素 | Go (CGO) | Rust | 决策 |
|------|----------|------|------|
| AX API 接入 | 需经 CGO → ObjC 桥接 | `accessibility` crate 直接调用 | ✅ Rust |
| 性能 | CGO 调用有开销 | 与 CoreGraphics 同级，零额外开销 | ✅ Rust |
| 安全性 | unsafe 指针管理复杂 | 类型安全 + 所有权模型 | ✅ Rust |
| 一致性 | 增加 CGO 负担 | 统一进 `libargus_core.dylib` | ✅ Rust |

---

## 升级方案

### 架构变更

```
升级前:  截屏 → [VLM检测] → SoM标注 → VLM选择 → 执行
升级后:  截屏 → [AX原生检测(Rust)] → SoM标注 → VLM选择 → 执行
                     ↑
              0ms, 精确到像素
              VLM检测降级为 fallback
```

### 性能预期

| 指标 | 升级前 (VLM) | 升级后 (AX) | 提升倍数 |
|------|-------------|------------|---------|
| 元素检测延迟 | 1-3s | <5ms | ~500× |
| 坐标精度 | ±30px | 精确 | 质变 |
| Token 消耗 | ~2000/次 | 0 | 100% 节省 |
| 小元素检测 | 经常漏掉 | 全量枚举 | 质变 |

### 实施范围

#### Batch A: Rust AXUIElement 模块 ✅

**新增文件：**

1. `rust-core/src/accessibility.rs` — AX 元素枚举 + JSON 导出
   - `argus_ax_list_elements(pid, out_json, out_len)` — 列出指定进程所有 UI 元素
   - `argus_ax_element_at_position(x, y, out_json, out_len)` — 获取坐标处元素
   - `argus_ax_focused_app(out_json, out_len)` — 获取前台 App 元素树
   - 输出 JSON：`[{role, label, x1, y1, x2, y2, interactable}, ...]`

2. `rust-core/include/argus_core.h` — 追加 AX 函数声明

3. **Rust 依赖**: 直接使用 `core-foundation` crate + ApplicationServices framework FFI

#### Batch B: Go FFI 绑定 + UIParser 改造 ✅

**修改文件：**

1. `go-sensory/internal/agent/rust_accessibility.go` [NEW] — Rust FFI 绑定
2. `go-sensory/internal/agent/ui_parser.go` [MODIFY] — `DetectElements()` 改为：
   - 优先调用 AX API (Rust FFI) 获取元素
   - AX 返回空时 fallback 到 VLM 检测（兼容无 accessibility 权限场景）

#### Batch C: ReAct Loop 集成 ✅

**修改文件：**

1. `go-sensory/internal/agent/react_loop.go` [MODIFY]:
   - `think()` 方法增加 SoM 模式：先用 AX 检测 + SoM 标注 → VLM 只选编号
   - 保留直接坐标回退模式

---

## 验证计划

### 自动化验证

```bash
# 1. Rust 编译
cd rust-core && cargo build --release

# 2. Rust 静态分析
cd rust-core && cargo clippy -- -D warnings

# 3. Go 编译
cd go-sensory && go build ./...

# 4. Go 静态分析
cd go-sensory && go vet ./...

# 5. 现有测试不回归
make test
```

### 验证结果 ✅

| 检查项 | 结果 |
|--------|------|
| `cargo build --release` | ✅ 零错误 |
| `cargo clippy -- -D warnings` | ✅ 零警告 |
| `go build ./...` | ✅ 零错误 |
| `go vet ./...` | ✅ 零警告 |
| `make test` | ✅ 全部通过，零回归 |

### 人工验证

1. 运行 AX 元素检测，确认能获取前台 App 的所有 UI 元素
2. 对比 AX 检测 vs VLM 检测的元素数量和坐标精度
3. 确认 Accessibility 权限弹窗正常出现并可授权

---

## 风险说明

| 风险 | 缓解措施 | 状态 |
|------|----------|------|
| macOS Accessibility 权限门槛 | `AXIsProcessTrusted()` 静默降级；后续 .app 封装 + 统一权限引导 | ⚠️ v2 增强中 |
| 非原生应用 (Electron/Web) 无障碍树不完整 | AX fallback VLM；后续 AXManualAccessibility + Web 深层递归 | ⚠️ v2 增强中 |
| `accessibility` crate 成熟度 | 已规避：直接使用 Core Foundation FFI | ✅ 已解决 |
| 跨平台适配 (Windows/Linux) | 延迟项 P3，远期规划 | 📋 挂起 |

> 后续增强方案详见: `docs/renwu/plan-20260215-gui-grounding-v2.md`
