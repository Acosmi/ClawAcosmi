package runner

import (
	"errors"
	"strings"
	"testing"
)

// ---------- rulePreCheck: nil outcome ----------

func TestQualityReview_NilOutcome(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
	}, nil)

	if result.Approved {
		t.Error("nil outcome should not be approved")
	}
	if len(result.Issues) == 0 {
		t.Error("nil outcome should have issues")
	}
}

// ---------- rulePreCheck: outcome status ----------

func TestQualityReview_OutcomeStatusError(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "error",
			Error:  "connection failed",
		},
	}, nil)

	if result.Approved {
		t.Error("error status should not be approved")
	}
	found := false
	for _, iss := range result.Issues {
		if strings.Contains(iss, "error") {
			found = true
		}
	}
	if !found {
		t.Error("issues should mention error status")
	}
}

func TestQualityReview_OutcomeStatusTimeout(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "timeout",
		},
	}, nil)

	if result.Approved {
		t.Error("timeout status should not be approved")
	}
}

// ---------- rulePreCheck: ThoughtResult 状态 ----------

func TestQualityReview_ThoughtCompleted(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtCompleted,
				Result: "task done successfully",
			},
		},
	}, nil)

	if !result.Approved {
		t.Errorf("completed thought should be approved, issues: %v", result.Issues)
	}
}

func TestQualityReview_ThoughtPartial(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtPartial,
				Result: "partial work",
			},
		},
	}, nil)

	if result.Approved {
		t.Error("partial thought should not be approved")
	}
}

func TestQualityReview_ThoughtFailed(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtFailed,
				Result: "failed",
			},
		},
	}, nil)

	if result.Approved {
		t.Error("failed thought should not be approved")
	}
}

func TestQualityReview_ThoughtNeedsAuth(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtNeedsAuth,
			},
		},
	}, nil)

	if result.Approved {
		t.Error("needs_auth thought should not be approved")
	}
}

func TestQualityReview_ThoughtBlocked(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtBlocked,
				Result: "blocked by firewall",
			},
		},
	}, nil)

	if result.Approved {
		t.Error("blocked thought should not be approved")
	}
}

func TestQualityReview_ThoughtNeedsHelp(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtNeedsHelp,
				Result: "need guidance",
			},
		},
	}, nil)

	if result.Approved {
		t.Error("needs_help thought should not be approved")
	}
}

func TestQualityReview_ThoughtTimeout(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtTimeout,
			},
		},
	}, nil)

	if result.Approved {
		t.Error("timeout thought should not be approved")
	}
}

// ---------- rulePreCheck: scope violations ----------

func TestQualityReview_ScopeViolations(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status:          ThoughtCompleted,
				Result:          "done",
				ScopeViolations: []string{"wrote to /etc/passwd"},
			},
		},
	}, nil)

	if result.Approved {
		t.Error("scope violations should not be approved")
	}
	found := false
	for _, iss := range result.Issues {
		if strings.Contains(iss, "scope") || strings.Contains(iss, "Scope") {
			found = true
		}
	}
	if !found {
		t.Error("issues should mention scope violation")
	}
}

// ---------- rulePreCheck: completed but empty ----------

func TestQualityReview_CompletedEmptyResult(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtCompleted,
				Result: "",
			},
		},
	}, nil)

	if result.Approved {
		t.Error("completed with empty result should not be approved")
	}
}

// ---------- Semantic review: nil fn → rule-only ----------

func TestQualityReview_NoSemanticFn(t *testing.T) {
	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtCompleted,
				Result: "all done",
			},
		},
	}, nil)

	if !result.Approved {
		t.Errorf("rule-pass with no semantic fn should be approved, issues: %v", result.Issues)
	}
}

// ---------- Semantic review: fn returns error → fail-open ----------

func TestQualityReview_SemanticFnError(t *testing.T) {
	failingFn := func(params QualityReviewParams) (*QualityReviewResult, error) {
		return nil, errors.New("LLM unavailable")
	}

	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtCompleted,
				Result: "completed work",
			},
		},
	}, failingFn)

	if !result.Approved {
		t.Error("semantic fn error should fail-open (Approved=true)")
	}
	if !result.RuleChecksPassed {
		t.Error("RuleChecksPassed should be true when rules pass")
	}
	if len(result.Suggestions) == 0 {
		t.Error("should have warning suggestion about semantic fn error")
	}
}

// ---------- Semantic review: fn rejects ----------

func TestQualityReview_SemanticFnRejects(t *testing.T) {
	rejectingFn := func(params QualityReviewParams) (*QualityReviewResult, error) {
		return &QualityReviewResult{
			Approved: false,
			Issues:   []string{"code has bug"},
		}, nil
	}

	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtCompleted,
				Result: "buggy code",
			},
		},
	}, rejectingFn)

	if result.Approved {
		t.Error("semantic rejection should propagate")
	}
	if !result.RuleChecksPassed {
		t.Error("RuleChecksPassed should be true when rules pass")
	}
}

// ---------- Semantic review: fn approves ----------

func TestQualityReview_SemanticFnApproves(t *testing.T) {
	approvingFn := func(params QualityReviewParams) (*QualityReviewResult, error) {
		return &QualityReviewResult{
			Approved:      true,
			ReviewSummary: "looks good",
		}, nil
	}

	result := ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "ok",
			ThoughtResult: &ThoughtResult{
				Status: ThoughtCompleted,
				Result: "clean code",
			},
		},
	}, approvingFn)

	if !result.Approved {
		t.Error("semantic approval should propagate")
	}
	if !result.RuleChecksPassed {
		t.Error("RuleChecksPassed should be true")
	}
}

// ---------- Semantic fn: rule fail skips semantic ----------

func TestQualityReview_RuleFailSkipsSemantic(t *testing.T) {
	called := false
	fn := func(params QualityReviewParams) (*QualityReviewResult, error) {
		called = true
		return &QualityReviewResult{Approved: true}, nil
	}

	ReviewSubagentResult(QualityReviewParams{
		TaskBrief: "test task",
		Outcome: &SubagentRunOutcome{
			Status: "error",
			Error:  "crashed",
		},
	}, fn)

	if called {
		t.Error("semantic fn should NOT be called when rule pre-check fails")
	}
}

// ---------- FormatReviewFailedResult ----------

func TestFormatReviewFailedResult(t *testing.T) {
	contract := &DelegationContract{
		ContractID: "test-contract-123",
		TaskBrief:  "build a REST API",
	}
	outcome := &SubagentRunOutcome{
		Status: "ok",
		ThoughtResult: &ThoughtResult{
			Status: ThoughtCompleted,
			Result: "here is the code",
		},
	}
	review := &QualityReviewResult{
		Approved: false,
		Issues:   []string{"missing error handling", "no tests"},
	}

	result := FormatReviewFailedResult(contract, outcome, review)

	if !strings.Contains(result, "test-contract-123") {
		t.Error("result should contain contract ID")
	}
	if !strings.Contains(result, "missing error handling") {
		t.Error("result should contain issues")
	}
	if !strings.Contains(result, "no tests") {
		t.Error("result should contain all issues")
	}
}
