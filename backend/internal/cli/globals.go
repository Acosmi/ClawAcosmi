package cli

import "sync/atomic"

// 对应 TS src/globals.ts — CLI 全局状态管理
// 审计修复项: H2-1 (globalVerbose + globalYes)

var (
	globalVerbose atomic.Bool
	globalYes     atomic.Bool
)

// SetVerbose 设置全局 verbose 模式。
// 对应 TS setVerbose()。
func SetVerbose(v bool) {
	globalVerbose.Store(v)
}

// IsVerbose 返回当前 verbose 状态。
// 对应 TS isVerbose()。
func IsVerbose() bool {
	return globalVerbose.Load()
}

// SetYes 设置全局 --yes 非交互模式。
// 对应 TS setYes()。
func SetYes(v bool) {
	globalYes.Store(v)
}

// IsYes 返回当前 --yes 状态。
// 对应 TS isYes()。
func IsYes() bool {
	return globalYes.Load()
}
