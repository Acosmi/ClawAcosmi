package gateway

// server_methods_contracts.go — contract.list / contract.get / contract.audit
// Phase 8: 合约生命周期仪表盘 RPC。

import (
	"github.com/Acosmi/ClawAcosmi/internal/agents/runner"
)

// ContractHandlers 返回合约方法映射。
func ContractHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"contract.list":  handleContractList,
		"contract.get":   handleContractGet,
		"contract.audit": handleContractAudit,
	}
}

// ---------- contract.list ----------

type contractListEntry struct {
	ContractID     string `json:"contract_id"`
	TaskBrief      string `json:"task_brief"`
	Status         string `json:"status"`
	IssuedAt       string `json:"issued_at"`
	ParentContract string `json:"parent_contract,omitempty"`
}

func handleContractList(ctx *MethodHandlerContext) {
	store := ctx.Context.ContractStore
	if store == nil {
		ctx.Respond(true, map[string]interface{}{"contracts": []interface{}{}}, nil)
		return
	}

	// 可选状态过滤（白名单验证，防止路径遍历）
	statusFilter, _ := ctx.Params["status"].(string)
	limitF, _ := ctx.Params["limit"].(float64)
	limit := int(limitF)
	if limit <= 0 {
		limit = 50
	}

	validStatuses := map[string]bool{
		string(runner.ContractActive):    true,
		string(runner.ContractSuspended): true,
		string(runner.ContractCompleted): true,
		string(runner.ContractFailed):    true,
		string(runner.ContractCancelled): true,
		string(runner.ContractPending):   true,
	}

	var statuses []runner.ContractStatus
	if statusFilter != "" {
		if !validStatuses[statusFilter] {
			ctx.Respond(false, nil, NewErrorShape(ErrCodeInvalidParams, "invalid status filter: "+statusFilter))
			return
		}
		statuses = []runner.ContractStatus{runner.ContractStatus(statusFilter)}
	} else {
		statuses = []runner.ContractStatus{
			runner.ContractActive,
			runner.ContractSuspended,
			runner.ContractCompleted,
			runner.ContractFailed,
		}
	}

	var entries []contractListEntry
	for _, status := range statuses {
		contracts, err := store.ListContracts(status)
		if err != nil {
			continue
		}
		for _, c := range contracts {
			if len(entries) >= limit {
				break
			}
			entries = append(entries, contractListEntry{
				ContractID:     c.ContractID,
				TaskBrief:      c.TaskBrief,
				Status:         string(c.Status),
				IssuedAt:       c.IssuedAt.Format("2006-01-02T15:04:05Z07:00"),
				ParentContract: c.ParentContract,
			})
		}
		if len(entries) >= limit {
			break
		}
	}

	ctx.Respond(true, map[string]interface{}{"contracts": entries}, nil)
}

// ---------- contract.get ----------

func handleContractGet(ctx *MethodHandlerContext) {
	store := ctx.Context.ContractStore
	if store == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "contract store not available"))
		return
	}

	contractID, _ := ctx.Params["contract_id"].(string)
	if contractID == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInvalidParams, "contract_id is required"))
		return
	}

	contract, thought, err := store.LoadContract(contractID)
	if err != nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, err.Error()))
		return
	}

	result := map[string]interface{}{
		"contract": contract,
	}
	if thought != nil {
		result["thought_result"] = thought
	}

	ctx.Respond(true, result, nil)
}

// ---------- contract.audit ----------

func handleContractAudit(ctx *MethodHandlerContext) {
	// 审批审计日志来自 CoderConfirmationManager 的 ApprovalRouter
	confirmMgr := ctx.Context.CoderConfirmMgr
	if confirmMgr == nil {
		ctx.Respond(true, map[string]interface{}{"entries": []interface{}{}}, nil)
		return
	}

	router := confirmMgr.ApprovalRouterRef()
	if router == nil {
		ctx.Respond(true, map[string]interface{}{"entries": []interface{}{}}, nil)
		return
	}

	// 注意: FlushAudit 会清空日志。对于查看场景，后续可改为 GetAudit（非破坏性）。
	// 当前阶段先用 FlushAudit，审计数据在 VFS 中也有备份。
	entries := router.FlushAudit()
	ctx.Respond(true, map[string]interface{}{"entries": entries}, nil)
}
