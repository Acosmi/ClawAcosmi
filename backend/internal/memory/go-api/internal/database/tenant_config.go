// Package database — 租户数据库配置模型。
// 存储在主库中，记录每个租户的自定义数据库连接信息。
package database

import (
	"time"
)

// TenantDBConfig 租户自定义数据库配置。
// 存储在主库的 tenant_db_configs 表中。
type TenantDBConfig struct {
	TenantID  string    `gorm:"primaryKey;size:64" json:"tenant_id"`
	DBType    string    `gorm:"size:32;not null;default:postgres" json:"db_type"` // postgres, mysql, sqlite
	DSN       string    `gorm:"size:1024;not null" json:"-"`                      // 加密存储，不暴露给 API
	MaxConns  int       `gorm:"not null;default:10" json:"max_conns"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Display-only fields (not stored in DB, populated by handler)
	Host   string `gorm:"-" json:"host,omitempty"`
	Port   int    `gorm:"-" json:"port,omitempty"`
	DBName string `gorm:"-" json:"db_name,omitempty"`
}

// TableName specifies the GORM table name.
func (TenantDBConfig) TableName() string {
	return "tenant_db_configs"
}
