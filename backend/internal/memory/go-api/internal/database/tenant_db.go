// Package database — 多租户数据库路由器。
// 根据租户 ID 动态路由到对应的数据库连接。
// 未配置自定义 DB 的租户使用默认全局连接。
package database

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/uhms/go-api/internal/models"
)

// TenantDBRouter 根据租户 ID 路由到对应数据库连接。
type TenantDBRouter struct {
	defaultDB *gorm.DB
	mu        sync.RWMutex
	pool      map[string]*tenantConn // tenant_id -> cached connection
}

// tenantConn 缓存的租户数据库连接。
type tenantConn struct {
	db       *gorm.DB
	lastUsed time.Time
	migrated bool // P3-1: AutoMigrate 仅执行一次
}

// NewTenantDBRouter 创建新的租户数据库路由器。
func NewTenantDBRouter(defaultDB *gorm.DB) *TenantDBRouter {
	r := &TenantDBRouter{
		defaultDB: defaultDB,
		pool:      make(map[string]*tenantConn),
	}
	// 启动后台清理过期连接
	go r.cleanupLoop()
	return r
}

// GetDB 获取指定租户的数据库连接。
// 如果租户未配置自定义 DB，返回默认连接。
func (r *TenantDBRouter) GetDB(ctx context.Context, tenantID string) (*gorm.DB, error) {
	if tenantID == "" {
		return r.defaultDB, nil
	}

	// 先查缓存
	r.mu.RLock()
	if conn, ok := r.pool[tenantID]; ok {
		conn.lastUsed = time.Now()
		r.mu.RUnlock()
		return conn.db, nil
	}
	r.mu.RUnlock()

	// 查主库配置
	var config TenantDBConfig
	if err := r.defaultDB.WithContext(ctx).Where("tenant_id = ? AND enabled = ?", tenantID, true).First(&config).Error; err != nil {
		// 未找到配置或查询错误，使用默认 DB
		return r.defaultDB, nil
	}

	// 创建新连接
	db, err := openTenantDB(config)
	if err != nil {
		slog.Error("Failed to connect to tenant DB, falling back to default",
			"tenant_id", tenantID, "error", err)
		return r.defaultDB, nil
	}

	// P3-1: 首次连接时自动建表
	if err := models.AutoMigrate(db); err != nil {
		slog.Warn("AutoMigrate on tenant DB failed",
			"tenant_id", tenantID, "error", err)
		// 不阻塞连接，仅日志警告
	} else {
		// P3-2: 记录 schema 版本
		if svErr := EnsureSchemaVersion(db); svErr != nil {
			slog.Warn("EnsureSchemaVersion failed",
				"tenant_id", tenantID, "error", svErr)
		}
		slog.Info("AutoMigrate completed for tenant DB", "tenant_id", tenantID)
	}

	// 缓存连接
	r.mu.Lock()
	r.pool[tenantID] = &tenantConn{db: db, lastUsed: time.Now(), migrated: true}
	r.mu.Unlock()

	slog.Info("Connected to tenant DB", "tenant_id", tenantID, "db_type", config.DBType)
	return db, nil
}

// InvalidateCache 清除指定租户的连接缓存（配置变更后调用）。
func (r *TenantDBRouter) InvalidateCache(tenantID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if conn, ok := r.pool[tenantID]; ok {
		closeTenantDB(conn.db)
		delete(r.pool, tenantID)
		slog.Info("Invalidated tenant DB cache", "tenant_id", tenantID)
	}
}

// Close 关闭所有租户数据库连接。
func (r *TenantDBRouter) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for tid, conn := range r.pool {
		closeTenantDB(conn.db)
		delete(r.pool, tid)
	}
	return nil
}

// TestConnection 测试租户数据库连接是否可用。
func (r *TenantDBRouter) TestConnection(config TenantDBConfig) error {
	db, err := openTenantDB(config)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer closeTenantDB(db)

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying db: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}

// cleanupLoop 后台定期清理超过 30 分钟未使用的连接。
func (r *TenantDBRouter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		r.mu.Lock()
		for tid, conn := range r.pool {
			if time.Since(conn.lastUsed) > 30*time.Minute {
				closeTenantDB(conn.db)
				delete(r.pool, tid)
				slog.Debug("Cleaned up idle tenant DB connection", "tenant_id", tid)
			}
		}
		r.mu.Unlock()
	}
}

// openTenantDB 根据配置创建并打开数据库连接。
func openTenantDB(config TenantDBConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch config.DBType {
	case "postgres":
		dialector = postgres.Open(config.DSN)
	case "mysql":
		dialector = mysql.Open(config.DSN)
	case "sqlite":
		dialector = sqlite.Open(config.DSN)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.DBType)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	maxConns := config.MaxConns
	if maxConns <= 0 {
		maxConns = 10
	}
	sqlDB.SetMaxOpenConns(maxConns)
	sqlDB.SetMaxIdleConns(maxConns / 2)
	sqlDB.SetConnMaxLifetime(15 * time.Minute)

	return db, nil
}

// closeTenantDB 安全关闭数据库连接。
func closeTenantDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
