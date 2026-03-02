package runner

import (
	"testing"
)

// ---------- TransitionStatus 状态机测试 ----------

func TestTransitionStatus_LegalTransitions(t *testing.T) {
	tests := []struct {
		name string
		from ContractStatus
		to   ContractStatus
	}{
		{"pending→active", ContractPending, ContractActive},
		{"active→suspended", ContractActive, ContractSuspended},
		{"active→completed", ContractActive, ContractCompleted},
		{"active→failed", ContractActive, ContractFailed},
		{"suspended→active", ContractSuspended, ContractActive},
		{"suspended→cancelled", ContractSuspended, ContractCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &DelegationContract{
				ContractID: "test-001",
				TaskBrief:  "test task",
				Status:     tt.from,
			}
			if err := c.TransitionStatus(tt.to); err != nil {
				t.Errorf("TransitionStatus(%q → %q) returned error: %v", tt.from, tt.to, err)
			}
			if c.Status != tt.to {
				t.Errorf("after transition, status = %q, want %q", c.Status, tt.to)
			}
		})
	}
}

func TestTransitionStatus_IllegalTransitions(t *testing.T) {
	tests := []struct {
		name string
		from ContractStatus
		to   ContractStatus
	}{
		{"pending→completed", ContractPending, ContractCompleted},
		{"pending→suspended", ContractPending, ContractSuspended},
		{"active→active", ContractActive, ContractActive},
		{"active→cancelled", ContractActive, ContractCancelled},
		{"completed→active", ContractCompleted, ContractActive},
		{"failed→active", ContractFailed, ContractActive},
		{"cancelled→active", ContractCancelled, ContractActive},
		{"suspended→completed", ContractSuspended, ContractCompleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &DelegationContract{
				ContractID: "test-001",
				TaskBrief:  "test task",
				Status:     tt.from,
			}
			if err := c.TransitionStatus(tt.to); err == nil {
				t.Errorf("TransitionStatus(%q → %q) should return error for illegal transition", tt.from, tt.to)
			}
			// 确认状态未变更
			if c.Status != tt.from {
				t.Errorf("after failed transition, status changed to %q, want %q", c.Status, tt.from)
			}
		})
	}
}

// ---------- ResourceBudget 测试 ----------

func TestResourceBudget_IsExhausted(t *testing.T) {
	tests := []struct {
		name      string
		budget    *ResourceBudget
		exhausted bool
	}{
		{"nil budget", nil, false},
		{"zero limits", &ResourceBudget{}, false},
		{"bash under limit", &ResourceBudget{MaxBashCalls: 5, UsedBashCalls: 3}, false},
		{"bash at limit", &ResourceBudget{MaxBashCalls: 5, UsedBashCalls: 5}, true},
		{"bash over limit", &ResourceBudget{MaxBashCalls: 3, UsedBashCalls: 4}, true},
		{"time under limit", &ResourceBudget{MaxTimeMs: 60000, UsedTimeMs: 30000}, false},
		{"time at limit", &ResourceBudget{MaxTimeMs: 60000, UsedTimeMs: 60000}, true},
		{"both ok", &ResourceBudget{MaxBashCalls: 5, UsedBashCalls: 2, MaxTimeMs: 60000, UsedTimeMs: 30000}, false},
		{"bash exhausted time ok", &ResourceBudget{MaxBashCalls: 3, UsedBashCalls: 3, MaxTimeMs: 60000, UsedTimeMs: 30000}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := tt.budget.IsExhausted()
			if got != tt.exhausted {
				t.Errorf("IsExhausted() = %v, want %v", got, tt.exhausted)
			}
		})
	}
}

func TestStatusToCategory(t *testing.T) {
	tests := []struct {
		status   ContractStatus
		expected string
	}{
		{ContractActive, "active"},
		{ContractSuspended, "suspended"},
		{ContractCompleted, "completed"},
		{ContractFailed, "failed"},
		{ContractCancelled, "cancelled"},
	}

	for _, tt := range tests {
		if got := tt.status.StatusToCategory(); got != tt.expected {
			t.Errorf("StatusToCategory(%q) = %q, want %q", tt.status, got, tt.expected)
		}
	}
}
