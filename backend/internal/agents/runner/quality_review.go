package runner

// quality_review.go — 三级指挥体系 Phase 2: 质量审核门控
//
// 子智能体（Open Coder / 灵瞳）返回结果后，主智能体审核质量 → 通过后才交付用户。
//
// R3 混合审核模式（v2 计划修订）:
//   - 规则预检（rule pre-check）: scope violations、status、artifacts 完整度
//   - LLM 语义审核（semantic review）: 结果与原始任务对齐度、代码质量
//
// 行业对标:
//   - LangGraph: evaluate() node in review graph
//   - CrewAI: task callback / quality check
//   - Anthropic: Oversight Paradox — 审核成本 < 重做成本

import (
	"fmt"
	"log/slog"
	"strings"
)

// ---------- 审核结果类型 ----------

// QualityReviewResult 质量审核结果。
type QualityReviewResult struct {
	// Approved 审核是否通过。
	Approved bool `json:"approved"`
	// Issues 发现的问题列表（审核失败时非空）。
	Issues []string `json:"issues,omitempty"`
	// Suggestions 改进建议（审核通过时也可能有）。
	Suggestions []string `json:"suggestions,omitempty"`
	// RuleChecksPassed 规则预检是否通过。
	RuleChecksPassed bool `json:"ruleChecksPassed"`
	// ReviewSummary 审核摘要（供 Phase 3 最终交付门展示）。
	ReviewSummary string `json:"reviewSummary,omitempty"`
}

// QualityReviewParams 质量审核参数。
type QualityReviewParams struct {
	// Contract 委托合约（含 scope、constraints 等约束信息）。
	Contract *DelegationContract
	// Outcome 子智能体执行结果。
	Outcome *SubagentRunOutcome
	// TaskBrief 原始任务描述（用于语义审核时对比）。
	TaskBrief string
	// SuccessCriteria 验收标准（可选，用于语义审核评判）。
	SuccessCriteria string
}

// QualityReviewFunc 质量审核回调函数类型。
// 由 gateway 注入 LLM 语义审核实现（Phase 2 预留）。
// 返回 nil, nil 表示跳过审核。
type QualityReviewFunc func(params QualityReviewParams) (*QualityReviewResult, error)

// ---------- 审核入口 ----------

// ReviewSubagentResult 审核子智能体执行结果（R3 混合模式）。
//
// 流程:
//  1. 规则预检 — 快速发现明确违规（scope violations、失败状态、空结果）
//  2. LLM 语义审核 — 深度评估结果与任务的对齐度（可选，由 semanticReviewFn 提供）
//
// 任一阶段失败即返回 Approved=false。
func ReviewSubagentResult(params QualityReviewParams, semanticReviewFn QualityReviewFunc) *QualityReviewResult {
	log := slog.Default().With("subsystem", "quality-review")

	// Phase 1: 规则预检
	ruleResult := rulePreCheck(params)
	if !ruleResult.Approved {
		log.Info("quality review: rule pre-check failed",
			"issues", ruleResult.Issues,
			"contractID", contractID(params.Contract),
		)
		return ruleResult
	}

	log.Debug("quality review: rule pre-check passed",
		"contractID", contractID(params.Contract),
	)

	// Phase 2: LLM 语义审核（可选）
	if semanticReviewFn != nil {
		semanticResult, err := semanticReviewFn(params)
		if err != nil {
			log.Warn("quality review: semantic review error, passing with warning",
				"error", err,
				"contractID", contractID(params.Contract),
			)
			// 语义审核出错时不阻塞（fail-open），但记录 warning
			return &QualityReviewResult{
				Approved:         true,
				RuleChecksPassed: true,
				Suggestions:      []string{fmt.Sprintf("semantic review skipped due to error: %v", err)},
				ReviewSummary:    "Rule checks passed. Semantic review skipped (error).",
			}
		}
		if semanticResult != nil {
			semanticResult.RuleChecksPassed = true // 规则预检已通过
			if !semanticResult.Approved {
				log.Info("quality review: semantic review rejected",
					"issues", semanticResult.Issues,
					"contractID", contractID(params.Contract),
				)
			}
			return semanticResult
		}
	}

	// 无语义审核 — 规则预检通过即通过
	return &QualityReviewResult{
		Approved:         true,
		RuleChecksPassed: true,
		ReviewSummary:    buildRulePassSummary(params),
	}
}

// ---------- 规则预检 ----------

// rulePreCheck 快速规则预检（不调 LLM，纯逻辑判断）。
func rulePreCheck(params QualityReviewParams) *QualityReviewResult {
	var issues []string

	outcome := params.Outcome
	if outcome == nil {
		return &QualityReviewResult{
			Approved: false,
			Issues:   []string{"no outcome returned from sub-agent"},
		}
	}

	// R1: 执行状态检查
	if outcome.Status == "error" || outcome.Status == "timeout" {
		issues = append(issues, fmt.Sprintf("sub-agent execution status: %s", outcome.Status))
		if outcome.Error != "" {
			issues = append(issues, fmt.Sprintf("error detail: %s", outcome.Error))
		}
	}

	tr := outcome.ThoughtResult
	if tr == nil {
		// 无结构化结果 — 如果 outcome.Status 不是 ok 则视为失败
		if outcome.Status != "ok" {
			issues = append(issues, "no structured ThoughtResult and status is not ok")
		}
		if len(issues) > 0 {
			return &QualityReviewResult{Approved: false, Issues: issues}
		}
		return &QualityReviewResult{
			Approved:         true,
			RuleChecksPassed: true,
			ReviewSummary:    "No structured result, but execution succeeded.",
		}
	}

	// R2: ThoughtResult 状态检查
	switch tr.Status {
	case ThoughtCompleted:
		// 预期状态，继续检查
	case ThoughtPartial:
		issues = append(issues, "sub-agent returned partial result (incomplete)")
	case ThoughtBlocked:
		issues = append(issues, "sub-agent is blocked, cannot complete")
	case ThoughtNeedsAuth:
		issues = append(issues, "sub-agent needs authorization, cannot complete")
	case ThoughtNeedsHelp:
		issues = append(issues, "sub-agent needs help from parent agent")
	case ThoughtFailed:
		issues = append(issues, "sub-agent reported failure")
		if tr.Result != "" {
			issues = append(issues, fmt.Sprintf("failure detail: %s", truncate(tr.Result, 200)))
		}
	case ThoughtTimeout:
		issues = append(issues, "sub-agent timed out")
	default:
		issues = append(issues, fmt.Sprintf("unknown ThoughtResult status: %q", tr.Status))
	}

	// R3: Scope violations 检查
	if len(tr.ScopeViolations) > 0 {
		issues = append(issues, fmt.Sprintf("scope violations detected: %v", tr.ScopeViolations))
	}

	// R4: 空结果检查（completed 状态但无结果内容）
	if tr.Status == ThoughtCompleted && tr.Result == "" && tr.Artifacts == nil {
		issues = append(issues, "completed status but empty result and no artifacts")
	}

	// R5: Artifacts 基本验证
	if tr.Artifacts != nil {
		if len(tr.Artifacts.FilesModified) == 0 && len(tr.Artifacts.FilesCreated) == 0 && tr.Result == "" {
			issues = append(issues, "artifacts present but no files modified/created and no result text")
		}
	}

	if len(issues) > 0 {
		return &QualityReviewResult{
			Approved: false,
			Issues:   issues,
		}
	}

	return &QualityReviewResult{
		Approved:         true,
		RuleChecksPassed: true,
	}
}

// ---------- 辅助函数 ----------

// buildRulePassSummary 构建规则预检通过时的摘要。
func buildRulePassSummary(params QualityReviewParams) string {
	var parts []string
	parts = append(parts, "Rule checks passed.")

	if params.Outcome == nil {
		return parts[0]
	}
	tr := params.Outcome.ThoughtResult
	if tr != nil {
		if tr.Artifacts != nil {
			var counts []string
			if n := len(tr.Artifacts.FilesModified); n > 0 {
				counts = append(counts, fmt.Sprintf("%d files modified", n))
			}
			if n := len(tr.Artifacts.FilesCreated); n > 0 {
				counts = append(counts, fmt.Sprintf("%d files created", n))
			}
			if len(counts) > 0 {
				parts = append(parts, strings.Join(counts, ", ")+".")
			}
		}
		if tr.ReasoningSummary != "" {
			parts = append(parts, fmt.Sprintf("Reasoning: %s", truncate(tr.ReasoningSummary, 100)))
		}
	}

	return strings.Join(parts, " ")
}

// FormatReviewFailedResult 格式化质量审核失败的结果文本（供主智能体阅读）。
func FormatReviewFailedResult(contract *DelegationContract, outcome *SubagentRunOutcome, review *QualityReviewResult) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("[Quality Review - FAILED]\nContract: %s\n", contractID(contract)))

	if len(review.Issues) > 0 {
		b.WriteString("\nIssues:\n")
		for i, issue := range review.Issues {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, issue))
		}
	}

	if len(review.Suggestions) > 0 {
		b.WriteString("\nSuggestions:\n")
		for _, s := range review.Suggestions {
			b.WriteString(fmt.Sprintf("  - %s\n", s))
		}
	}

	// 附加原始子智能体结果摘要
	if outcome != nil && outcome.ThoughtResult != nil {
		tr := outcome.ThoughtResult
		if tr.Result != "" {
			b.WriteString(fmt.Sprintf("\nOriginal result (truncated): %s\n", truncate(tr.Result, 300)))
		}
	}

	b.WriteString("\n---\n")
	b.WriteString("ACTION: Review the issues above. You may:\n")
	b.WriteString("  1. Re-delegate with adjusted parameters\n")
	b.WriteString("  2. Handle the task directly\n")
	b.WriteString("  3. Report the issues to the user\n")

	return b.String()
}

// contractID 安全获取合约 ID（nil-safe）。
func contractID(c *DelegationContract) string {
	if c == nil {
		return "(no contract)"
	}
	return c.ContractID
}

// truncate 截断字符串到指定长度。
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
