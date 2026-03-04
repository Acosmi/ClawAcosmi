package gateway

// ============================================================================
// Contract VFS Persistence — 合约 VFS 持久化适配器
// Phase 5: 设计文档 §4.3-§4.4, 跟踪文档 Phase 5
// ============================================================================

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/agents/runner"
	"github.com/Acosmi/ClawAcosmi/internal/memory/uhms"
)

const (
	// contractVFSNamespace VFS 命名空间。
	contractVFSNamespace = "contracts"
)

// allContractStatuses 用于遍历查找（含 pending，确保全生命周期可发现）。
var allContractStatuses = []runner.ContractStatus{
	runner.ContractPending,
	runner.ContractActive,
	runner.ContractSuspended,
	runner.ContractCompleted,
	runner.ContractFailed,
	runner.ContractCancelled,
}

// VFSContractPersistence 基于 UHMS VFS 的合约持久化实现。
// 复用 WriteSystemEntry/ReadSystemL1/ListSystemEntries/DeleteSystemEntry 接口。
type VFSContractPersistence struct {
	vfs *uhms.LocalVFS
}

// NewVFSContractPersistence 创建 VFS 合约持久化适配器。
func NewVFSContractPersistence(vfs *uhms.LocalVFS) *VFSContractPersistence {
	return &VFSContractPersistence{vfs: vfs}
}

// SaveContract 保存合约到 VFS。
// l0 = 任务摘要（≤80字符，搜索索引用）
// l1 = 合约 JSON
// l2 = ThoughtResult JSON（挂起时，可空）
func (p *VFSContractPersistence) SaveContract(c *runner.DelegationContract, thought *runner.ThoughtResult) error {
	if c == nil {
		return fmt.Errorf("contract is nil")
	}

	// l0: 截断任务摘要
	l0 := truncateContractBrief(c.TaskBrief, 80)

	// l1: 合约 JSON
	l1Bytes, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal contract: %w", err)
	}

	// l2: ThoughtResult JSON（可为空）
	l2 := ""
	if thought != nil {
		l2Bytes, err := json.Marshal(thought)
		if err != nil {
			return fmt.Errorf("marshal thought result: %w", err)
		}
		l2 = string(l2Bytes)
	}

	// meta: 便于搜索和清理
	meta := map[string]interface{}{
		"status":         string(c.Status),
		"issuedBy":       c.IssuedBy,
		"issuedAt":       c.IssuedAt.Format(time.RFC3339),
		"parentContract": c.ParentContract,
		"updatedAt":      time.Now().Format(time.RFC3339),
	}

	category := c.Status.StatusToCategory()
	return p.vfs.WriteSystemEntry(contractVFSNamespace, category, c.ContractID, l0, string(l1Bytes), l2, meta)
}

// LoadContract 按 ID 加载合约，自动遍历所有状态目录查找。
func (p *VFSContractPersistence) LoadContract(contractID string) (*runner.DelegationContract, *runner.ThoughtResult, error) {
	for _, status := range allContractStatuses {
		category := status.StatusToCategory()
		if !p.vfs.SystemEntryExists(contractVFSNamespace, category, contractID) {
			continue
		}

		// 读取 l1（合约 JSON）
		l1, err := p.vfs.ReadSystemL1(contractVFSNamespace, category, contractID)
		if err != nil {
			return nil, nil, fmt.Errorf("read contract l1 from %s/%s: %w", category, contractID, err)
		}

		var contract runner.DelegationContract
		if err := json.Unmarshal([]byte(l1), &contract); err != nil {
			return nil, nil, fmt.Errorf("unmarshal contract from %s/%s: %w", category, contractID, err)
		}

		// 读取 l2（ThoughtResult JSON，可空）
		var thought *runner.ThoughtResult
		l2, err := p.vfs.ReadSystemL2(contractVFSNamespace, category, contractID)
		if err == nil && l2 != "" {
			var tr runner.ThoughtResult
			if json.Unmarshal([]byte(l2), &tr) == nil {
				thought = &tr
			}
		}

		return &contract, thought, nil
	}

	return nil, nil, fmt.Errorf("contract %s not found in any status directory", contractID)
}

// TransitionStatus 执行合约状态转换。
// 步骤: Load from 'from' → validate → Write to 'to' → Delete from 'from'
func (p *VFSContractPersistence) TransitionStatus(contractID string, from, to runner.ContractStatus) error {
	fromCategory := from.StatusToCategory()
	toCategory := to.StatusToCategory()

	// 读取现有合约
	l1, err := p.vfs.ReadSystemL1(contractVFSNamespace, fromCategory, contractID)
	if err != nil {
		return fmt.Errorf("read contract for transition: %w", err)
	}

	var contract runner.DelegationContract
	if err := json.Unmarshal([]byte(l1), &contract); err != nil {
		return fmt.Errorf("unmarshal contract for transition: %w", err)
	}

	// 校验状态转换合法性
	if contract.Status != from {
		return fmt.Errorf("contract %s status mismatch: expected %q, got %q", contractID, from, contract.Status)
	}
	if err := contract.TransitionStatus(to); err != nil {
		return err
	}

	// 读取 l2（保留）
	l2, _ := p.vfs.ReadSystemL2(contractVFSNamespace, fromCategory, contractID)

	// 写入新状态目录
	if err := p.SaveContract(&contract, nil); err != nil {
		return fmt.Errorf("save contract to %s: %w", toCategory, err)
	}

	// 如果有 l2 但 SaveContract 传了 nil thought，需要保留 l2
	if l2 != "" {
		// 重新写入完整数据（含 l2）
		l0 := truncateContractBrief(contract.TaskBrief, 80)
		l1Bytes, _ := json.Marshal(&contract)
		meta := map[string]interface{}{
			"status":         string(contract.Status),
			"issuedBy":       contract.IssuedBy,
			"issuedAt":       contract.IssuedAt.Format(time.RFC3339),
			"parentContract": contract.ParentContract,
			"updatedAt":      time.Now().Format(time.RFC3339),
		}
		if err := p.vfs.WriteSystemEntry(contractVFSNamespace, toCategory, contractID, l0, string(l1Bytes), l2, meta); err != nil {
			slog.Warn("contract: failed to preserve l2 during transition (ThoughtResult may be lost)",
				"contractID", contractID,
				"to", to,
				"error", err,
			)
		}
	}

	// 删除旧状态目录
	if err := p.vfs.DeleteSystemEntry(contractVFSNamespace, fromCategory, contractID); err != nil {
		slog.Warn("contract: failed to delete old status entry (non-fatal)",
			"contractID", contractID,
			"from", from,
			"error", err,
		)
	}

	return nil
}

// ListContracts 列出指定状态的所有合约。
func (p *VFSContractPersistence) ListContracts(status runner.ContractStatus) ([]*runner.DelegationContract, error) {
	category := status.StatusToCategory()
	refs, err := p.vfs.ListSystemEntries(contractVFSNamespace, category)
	if err != nil {
		return nil, fmt.Errorf("list contracts in %s: %w", category, err)
	}

	contracts := make([]*runner.DelegationContract, 0, len(refs))
	for _, ref := range refs {
		l1, err := p.vfs.ReadSystemL1(contractVFSNamespace, category, ref.ID)
		if err != nil {
			slog.Warn("contract: failed to read contract during list (skipping)",
				"id", ref.ID,
				"category", category,
				"error", err,
			)
			continue
		}
		var c runner.DelegationContract
		if err := json.Unmarshal([]byte(l1), &c); err != nil {
			slog.Warn("contract: failed to unmarshal contract during list (skipping)",
				"id", ref.ID,
				"error", err,
			)
			continue
		}
		contracts = append(contracts, &c)
	}

	return contracts, nil
}

// CleanupCompleted 清理超过 olderThan 的已完成合约。
func (p *VFSContractPersistence) CleanupCompleted(olderThan time.Duration) (int, error) {
	category := runner.ContractCompleted.StatusToCategory()
	refs, err := p.vfs.ListSystemEntries(contractVFSNamespace, category)
	if err != nil {
		return 0, fmt.Errorf("list completed contracts: %w", err)
	}

	cutoff := time.Now().Add(-olderThan)
	deleted := 0

	for _, ref := range refs {
		meta, err := p.vfs.ReadSystemMeta(contractVFSNamespace, category, ref.ID)
		if err != nil {
			continue
		}
		updatedAtStr, ok := meta["updatedAt"].(string)
		if !ok {
			continue
		}
		updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			continue
		}
		if updatedAt.Before(cutoff) {
			if err := p.vfs.DeleteSystemEntry(contractVFSNamespace, category, ref.ID); err != nil {
				slog.Warn("contract: failed to cleanup completed contract",
					"id", ref.ID,
					"error", err,
				)
				continue
			}
			deleted++
		}
	}

	if deleted > 0 {
		slog.Info("contract: TTL cleanup completed",
			"deleted", deleted,
			"olderThan", olderThan.String(),
		)
	}

	return deleted, nil
}

// DeleteEntry 删除指定状态目录下的合约条目。
func (p *VFSContractPersistence) DeleteEntry(status runner.ContractStatus, contractID string) error {
	return p.vfs.DeleteSystemEntry(contractVFSNamespace, status.StatusToCategory(), contractID)
}

// truncateContractBrief 截断合约任务摘要到指定长度。
func truncateContractBrief(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}
