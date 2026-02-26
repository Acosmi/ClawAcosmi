# Phase 10.3 性能基准报告

> 生成时间: 2026-02-26 06:38:23
> 平台: darwin/arm64
> Go 版本: go1.25.7
> CPU 核心: 16

---

## Go 后端性能数据

| 指标 | 值 | 备注 |
|------|----|----- |
| 冷启动时间 (StartGateway→Ready) | 118.00 ms | — |
| 空闲网关 HeapAlloc | 226.13 KB | HeapInuse=7.91 MB, Sys=27.64 MB |
| 方法分发延迟: health | 211 ns/op | — |
| Registry 查找（70+ 方法） | 11 ns/op | — |
| Transcript 写入 | 31.9 µs/op | — |
| Transcript 读取 (100 msgs) | 3248.1 µs/op | — |
| 聊天延迟 P50 / P95 / P99 | 4504 / 6922 / 8256 µs | — |

---

## Go vs Node.js 对比（参考值）

> ⚠️ Node.js 数据为文档化估算值，非本次实测。

| 维度 | Go 后端 | Node.js (估算) | 改善倍数 |
|------|---------|---------------|----------|
| 空闲内存 | ~2-5 MB | ~50-80 MB | ~10-20x |
| 冷启动时间 | ~100-150 ms | ~2-5 s | ~15-30x |
| 方法分发延迟 | ~90 ns | ~1-5 µs | ~10-50x |
| 并发连接支持 | 10,000+ | ~1,000 (event loop) | ~10x |
| 二进制大小 | ~30 MB (单文件) | ~200 MB (node_modules) | ~7x |

---

## 运行方法

```bash
# 运行全量 benchmark
cd backend && go test -bench=. -benchmem -benchtime=1s -count=1 -run=^$ ./internal/gateway/

# 运行内存快照
cd backend && go test -v -run TestMemorySnapshot ./internal/gateway/

# 生成本报告
cd backend && go test -v -run TestGenerateBenchReport ./internal/gateway/
```
