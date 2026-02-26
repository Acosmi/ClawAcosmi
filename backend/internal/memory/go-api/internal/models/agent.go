// Package models — Agent 模型（P4-5: Agent 注册/发现）。
// 追踪已注册的本地 Agent 实例及其连接状态。
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AgentStatus 定义 Agent 在线状态。
const (
	AgentStatusOnline  = "online"
	AgentStatusOffline = "offline"
)

// Agent 表示本地 Agent 注册记录。
// 每个租户可注册一个或多个 Agent，通过安全隧道与云端通信。
type Agent struct {
	ID        uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	TenantID  string          `gorm:"type:varchar(100);not null;index" json:"tenant_id"`
	Name      string          `gorm:"type:varchar(200);not null" json:"name"`
	Status    string          `gorm:"type:varchar(20);not null;default:'offline'" json:"status"`
	Endpoint  string          `gorm:"type:varchar(500);not null" json:"endpoint"`
	LastSeen  *time.Time      `gorm:"index" json:"last_seen,omitempty"`
	Metadata  *map[string]any `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt *time.Time      `gorm:"autoUpdateTime" json:"updated_at,omitempty"`
}

func (Agent) TableName() string { return "agents" }

func (a *Agent) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
