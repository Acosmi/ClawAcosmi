package nodehost

// allowlist_types.go — 白名单评估相关类型定义
// 对应 TS: src/infra/exec-approvals.ts 类型部分

import "github.com/anthropic/open-acosmi/internal/infra"

// DefaultSafeBins 默认安全命令列表（只读不写的工具）。
var DefaultSafeBins = []string{"jq", "grep", "cut", "sort", "uniq", "head", "tail", "tr", "wc"}

// CommandResolution 命令解析结果。
type CommandResolution struct {
	RawExecutable  string
	ResolvedPath   string
	ExecutableName string
}

// ExecCommandSegment 一个管道段的解析结果。
type ExecCommandSegment struct {
	Raw        string
	Argv       []string
	Resolution *CommandResolution
}

// ExecCommandAnalysis Shell 命令完整解析结果。
type ExecCommandAnalysis struct {
	OK       bool
	Reason   string
	Segments []ExecCommandSegment
	Chains   [][]ExecCommandSegment // 按 &&/||/; 分组
}

// ExecAllowlistEvaluation 白名单评估结果。
type ExecAllowlistEvaluation struct {
	AllowlistSatisfied bool
	AllowlistMatches   []infra.ExecAllowlistEntry
}

// ExecAllowlistAnalysis Shell 命令白名单分析结果。
type ExecAllowlistAnalysis struct {
	AnalysisOk         bool
	AllowlistSatisfied bool
	AllowlistMatches   []infra.ExecAllowlistEntry
	Segments           []ExecCommandSegment
}
