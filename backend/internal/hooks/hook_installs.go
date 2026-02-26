package hooks

import (
	"time"
)

// ============================================================================
// Hook 安装记录管理 — 记录和查询已安装的钩子
// 对应 TS: hooks/installs.ts (31L)
// ============================================================================

// HookInstallRecord 安装记录
// 对应 TS: config/types.hooks.ts HookInstallRecord
type HookInstallRecord struct {
	PackageSpec string `json:"packageSpec,omitempty"`
	Version     string `json:"version,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	TargetDir   string `json:"targetDir,omitempty"`
}

// HookInstallUpdate 安装更新记录（含 hookId）
// 对应 TS: installs.ts HookInstallUpdate
type HookInstallUpdate struct {
	HookID      string `json:"hookId"`
	PackageSpec string `json:"packageSpec,omitempty"`
	Version     string `json:"version,omitempty"`
	InstalledAt string `json:"installedAt,omitempty"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
	TargetDir   string `json:"targetDir,omitempty"`
}

// RecordHookInstall 将安装记录追加到钩子配置中。
// 对应 TS: installs.ts recordHookInstall
//
// 参数 installs 是 hooks.internal.installs map，hookId → record。
// 返回更新后的 map。
func RecordHookInstall(installs map[string]*HookInstallRecord, update HookInstallUpdate) map[string]*HookInstallRecord {
	if installs == nil {
		installs = make(map[string]*HookInstallRecord)
	}

	existing := installs[update.HookID]
	if existing == nil {
		existing = &HookInstallRecord{}
	}

	// 合并更新字段
	if update.PackageSpec != "" {
		existing.PackageSpec = update.PackageSpec
	}
	if update.Version != "" {
		existing.Version = update.Version
	}
	if update.TargetDir != "" {
		existing.TargetDir = update.TargetDir
	}
	if update.InstalledAt != "" {
		existing.InstalledAt = update.InstalledAt
	} else if existing.InstalledAt == "" {
		existing.InstalledAt = time.Now().UTC().Format(time.RFC3339)
	}
	if update.UpdatedAt != "" {
		existing.UpdatedAt = update.UpdatedAt
	}

	installs[update.HookID] = existing
	return installs
}
