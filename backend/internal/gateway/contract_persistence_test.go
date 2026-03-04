package gateway

import (
	"testing"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/agents/runner"
	"github.com/Acosmi/ClawAcosmi/internal/memory/uhms"
)

func newTestPersistence(t *testing.T) *VFSContractPersistence {
	t.Helper()
	vfs, err := uhms.NewLocalVFS(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalVFS: %v", err)
	}
	return NewVFSContractPersistence(vfs)
}

func newTestContract(id, brief string, status runner.ContractStatus) *runner.DelegationContract {
	return &runner.DelegationContract{
		ContractID:    id,
		SchemaVersion: "1.0",
		TaskBrief:     brief,
		IssuedBy:      "main-agent",
		IssuedAt:      time.Now(),
		TimeoutMs:     60000,
		Status:        status,
		Scope: []runner.ScopeEntry{
			{Path: "/workspace/src", Permissions: []runner.ScopePermission{runner.PermRead, runner.PermWrite}},
		},
	}
}

func TestContractPersistence_SaveAndLoad(t *testing.T) {
	p := newTestPersistence(t)

	c := newTestContract("ct-001", "Fix the login bug", runner.ContractActive)

	// Save
	if err := p.SaveContract(c, nil); err != nil {
		t.Fatalf("SaveContract: %v", err)
	}

	// Load
	loaded, thought, err := p.LoadContract("ct-001")
	if err != nil {
		t.Fatalf("LoadContract: %v", err)
	}
	if loaded.ContractID != "ct-001" {
		t.Errorf("ContractID = %q, want %q", loaded.ContractID, "ct-001")
	}
	if loaded.TaskBrief != "Fix the login bug" {
		t.Errorf("TaskBrief = %q, want %q", loaded.TaskBrief, "Fix the login bug")
	}
	if loaded.Status != runner.ContractActive {
		t.Errorf("Status = %q, want %q", loaded.Status, runner.ContractActive)
	}
	if thought != nil {
		t.Errorf("thought should be nil, got %+v", thought)
	}
}

func TestContractPersistence_SaveWithThoughtResult(t *testing.T) {
	p := newTestPersistence(t)

	c := newTestContract("ct-002", "Implement feature", runner.ContractSuspended)
	tr := &runner.ThoughtResult{
		Status:     runner.ThoughtNeedsAuth,
		ContractID: "ct-002",
		Result:     "Need access to config files",
		ResumeHint: "Grant read access to /etc/config",
		AuthRequest: &runner.AuthRequest{
			Reason:    "Config file needed",
			RiskLevel: "low",
		},
	}

	if err := p.SaveContract(c, tr); err != nil {
		t.Fatalf("SaveContract: %v", err)
	}

	loaded, thought, err := p.LoadContract("ct-002")
	if err != nil {
		t.Fatalf("LoadContract: %v", err)
	}
	if loaded.Status != runner.ContractSuspended {
		t.Errorf("Status = %q, want %q", loaded.Status, runner.ContractSuspended)
	}
	if thought == nil {
		t.Fatal("thought should not be nil")
	}
	if thought.ResumeHint != "Grant read access to /etc/config" {
		t.Errorf("ResumeHint = %q, want %q", thought.ResumeHint, "Grant read access to /etc/config")
	}
	if thought.AuthRequest == nil || thought.AuthRequest.RiskLevel != "low" {
		t.Errorf("AuthRequest.RiskLevel wrong")
	}
}

func TestContractPersistence_LoadNotFound(t *testing.T) {
	p := newTestPersistence(t)

	_, _, err := p.LoadContract("nonexistent")
	if err == nil {
		t.Error("LoadContract should return error for nonexistent contract")
	}
}

func TestContractPersistence_TransitionStatus(t *testing.T) {
	p := newTestPersistence(t)

	c := newTestContract("ct-003", "Test transition", runner.ContractActive)
	if err := p.SaveContract(c, nil); err != nil {
		t.Fatalf("SaveContract: %v", err)
	}

	// active → completed
	if err := p.TransitionStatus("ct-003", runner.ContractActive, runner.ContractCompleted); err != nil {
		t.Fatalf("TransitionStatus: %v", err)
	}

	// Verify new status
	loaded, _, err := p.LoadContract("ct-003")
	if err != nil {
		t.Fatalf("LoadContract after transition: %v", err)
	}
	if loaded.Status != runner.ContractCompleted {
		t.Errorf("Status = %q, want %q", loaded.Status, runner.ContractCompleted)
	}

	// Verify old status directory is empty
	old, err := p.ListContracts(runner.ContractActive)
	if err != nil {
		t.Fatalf("ListContracts(active): %v", err)
	}
	if len(old) != 0 {
		t.Errorf("active contracts should be empty after transition, got %d", len(old))
	}
}

func TestContractPersistence_TransitionStatus_Illegal(t *testing.T) {
	p := newTestPersistence(t)

	c := newTestContract("ct-004", "Test illegal transition", runner.ContractActive)
	if err := p.SaveContract(c, nil); err != nil {
		t.Fatalf("SaveContract: %v", err)
	}

	// active → cancelled is illegal
	if err := p.TransitionStatus("ct-004", runner.ContractActive, runner.ContractCancelled); err == nil {
		t.Error("TransitionStatus(active→cancelled) should return error")
	}
}

func TestContractPersistence_ListContracts(t *testing.T) {
	p := newTestPersistence(t)

	// Save 3 active contracts
	for i, brief := range []string{"Task A", "Task B", "Task C"} {
		c := newTestContract("ct-list-"+string(rune('0'+i)), brief, runner.ContractActive)
		if err := p.SaveContract(c, nil); err != nil {
			t.Fatalf("SaveContract(%s): %v", brief, err)
		}
	}

	// List active
	contracts, err := p.ListContracts(runner.ContractActive)
	if err != nil {
		t.Fatalf("ListContracts: %v", err)
	}
	if len(contracts) != 3 {
		t.Errorf("ListContracts(active) = %d, want 3", len(contracts))
	}

	// List completed (empty)
	completed, err := p.ListContracts(runner.ContractCompleted)
	if err != nil {
		t.Fatalf("ListContracts(completed): %v", err)
	}
	if len(completed) != 0 {
		t.Errorf("ListContracts(completed) = %d, want 0", len(completed))
	}
}

func TestContractPersistence_CleanupCompleted(t *testing.T) {
	p := newTestPersistence(t)

	// Save a completed contract
	c := newTestContract("ct-cleanup", "Old task", runner.ContractCompleted)
	if err := p.SaveContract(c, nil); err != nil {
		t.Fatalf("SaveContract: %v", err)
	}

	// Cleanup with 0 duration → should delete everything
	n, err := p.CleanupCompleted(0)
	if err != nil {
		t.Fatalf("CleanupCompleted: %v", err)
	}
	if n != 1 {
		t.Errorf("CleanupCompleted = %d, want 1", n)
	}

	// Verify deleted
	contracts, err := p.ListContracts(runner.ContractCompleted)
	if err != nil {
		t.Fatalf("ListContracts: %v", err)
	}
	if len(contracts) != 0 {
		t.Errorf("after cleanup, completed contracts = %d, want 0", len(contracts))
	}
}

func TestContractPersistence_NilContract(t *testing.T) {
	p := newTestPersistence(t)

	if err := p.SaveContract(nil, nil); err == nil {
		t.Error("SaveContract(nil) should return error")
	}
}
