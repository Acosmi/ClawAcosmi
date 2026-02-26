// Package services — Agent 注册/发现服务（P4-5）。
// 管理本地 Agent 的注册、发现和生命周期。
package services

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// AgentRegistry 管理已注册的本地 Agent 实例。
// 内存缓存 + DB 持久化双层设计。
type AgentRegistry struct {
	db    *gorm.DB
	mu    sync.RWMutex
	cache map[string]*models.Agent // tenantID -> online agent
}

// NewAgentRegistry 创建 Agent 注册中心。
func NewAgentRegistry(db *gorm.DB) *AgentRegistry {
	r := &AgentRegistry{
		db:    db,
		cache: make(map[string]*models.Agent),
	}
	// 启动时加载在线 Agent 到缓存
	r.loadOnlineAgents()
	return r
}

// RegisterAgent 注册或更新 Agent。
func (r *AgentRegistry) RegisterAgent(
	tenantID, name, endpoint string,
	metadata *map[string]any,
) (*models.Agent, error) {
	now := time.Now()
	agent := &models.Agent{
		TenantID: tenantID,
		Name:     name,
		Status:   models.AgentStatusOnline,
		Endpoint: endpoint,
		LastSeen: &now,
		Metadata: metadata,
	}

	// Upsert: 按 tenant_id 查找已有记录
	var existing models.Agent
	err := r.db.Where("tenant_id = ?", tenantID).First(&existing).Error
	if err == nil {
		// 更新已有记录
		agent.ID = existing.ID
		if dbErr := r.db.Save(agent).Error; dbErr != nil {
			return nil, fmt.Errorf("update agent: %w", dbErr)
		}
	} else if err == gorm.ErrRecordNotFound {
		// 新建记录
		if dbErr := r.db.Create(agent).Error; dbErr != nil {
			return nil, fmt.Errorf("create agent: %w", dbErr)
		}
	} else {
		return nil, fmt.Errorf("query agent: %w", err)
	}

	// 更新缓存
	r.mu.Lock()
	r.cache[tenantID] = agent
	r.mu.Unlock()

	slog.Info("Agent registered",
		"tenant_id", tenantID, "agent_id", agent.ID, "name", name)
	return agent, nil
}

// GetAgent 获取指定租户的在线 Agent。
func (r *AgentRegistry) GetAgent(tenantID string) (*models.Agent, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	agent, ok := r.cache[tenantID]
	if ok && agent.Status == models.AgentStatusOnline {
		return agent, true
	}
	return nil, false
}

// GetAgentByID 通过 ID 查找 Agent。
func (r *AgentRegistry) GetAgentByID(agentID uuid.UUID) (*models.Agent, error) {
	var agent models.Agent
	if err := r.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

// DeregisterAgent Agent 下线/断连时清理。
func (r *AgentRegistry) DeregisterAgent(tenantID string) {
	r.mu.Lock()
	delete(r.cache, tenantID)
	r.mu.Unlock()

	// 更新 DB 状态
	r.db.Model(&models.Agent{}).
		Where("tenant_id = ?", tenantID).
		Update("status", models.AgentStatusOffline)

	slog.Info("Agent deregistered", "tenant_id", tenantID)
}

// UpdateHeartbeat 更新 Agent 心跳时间。
func (r *AgentRegistry) UpdateHeartbeat(tenantID string) {
	now := time.Now()
	r.mu.Lock()
	if agent, ok := r.cache[tenantID]; ok {
		agent.LastSeen = &now
	}
	r.mu.Unlock()

	r.db.Model(&models.Agent{}).
		Where("tenant_id = ?", tenantID).
		Update("last_seen", now)
}

// ListAgents 列出所有已注册 Agent。
func (r *AgentRegistry) ListAgents() ([]models.Agent, error) {
	var agents []models.Agent
	if err := r.db.Order("created_at DESC").Find(&agents).Error; err != nil {
		return nil, err
	}
	return agents, nil
}

// loadOnlineAgents 启动时从 DB 加载在线 Agent 到缓存。
func (r *AgentRegistry) loadOnlineAgents() {
	var agents []models.Agent
	if err := r.db.Where("status = ?", models.AgentStatusOnline).
		Find(&agents).Error; err != nil {
		slog.Warn("Failed to load online agents", "error", err)
		return
	}
	r.mu.Lock()
	for i := range agents {
		r.cache[agents[i].TenantID] = &agents[i]
	}
	r.mu.Unlock()
	slog.Info("Loaded online agents", "count", len(agents))
}
