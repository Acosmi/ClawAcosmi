//go:build e2e
// +build e2e

// Package e2etest — E2E 集成测试基础设施。
// 独立包模式，避免 CGO/FFI 链接依赖。
// 复制核心类型定义 + 内存 Mock 实现全链路验证。
package e2etest

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// 常量定义 — 复制自 services 包
// ============================================================================

// 记忆类型常量
const (
	MemoryTypeEpisodic    = "episodic"
	MemoryTypeSemantic    = "semantic"
	MemoryTypeProcedural  = "procedural"
	MemoryTypePermanent   = "permanent"
	MemoryTypeImagination = "imagination"
	MemoryTypeReflection  = "reflection"
)

// 记忆分类常量
const (
	CategoryPreference = "preference"
	CategoryHabit      = "habit"
	CategoryFact       = "fact"
	CategoryEvent      = "event"
	CategorySkill      = "skill"
	CategoryInsight    = "insight"
)

// 受保护记忆类型 — 不受衰减影响
var ProtectedMemoryTypes = []string{
	MemoryTypePermanent,
	MemoryTypeImagination,
}

// TierPolicy 定义渐进式加载策略
type TierPolicy int

const (
	TierStandard TierPolicy = iota
	TierAlwaysL1
	TierL0Only
)

// ClassifyMemoryTier 根据记忆类型返回加载策略
func ClassifyMemoryTier(memoryType string) TierPolicy {
	switch memoryType {
	case MemoryTypePermanent:
		return TierAlwaysL1
	case MemoryTypeImagination:
		return TierL0Only
	default:
		return TierStandard
	}
}

// ============================================================================
// 核心类型 — 复制自 services 包
// ============================================================================

// L0Entry 轻量 L0 摘要条目
type L0Entry struct {
	URI        string `json:"uri"`
	MemoryID   string `json:"memory_id"`
	L0Abstract string `json:"l0_abstract"`
	MemoryType string `json:"memory_type"`
	Category   string `json:"category"`
	CreatedAt  int64  `json:"created_at"`
}

// L1Entry L1 概述条目
type L1Entry struct {
	URI        string `json:"uri"`
	MemoryID   string `json:"memory_id"`
	L1Overview string `json:"l1_overview"`
	MemoryType string `json:"memory_type"`
	Category   string `json:"category"`
	CreatedAt  int64  `json:"created_at"`
}

// VectorSearchResult 向量搜索结果
type VectorSearchResult struct {
	MemoryID        string         `json:"memory_id"`
	Content         string         `json:"content"`
	Score           float64        `json:"score"`
	MemoryType      string         `json:"memory_type"`
	UserID          string         `json:"user_id"`
	Category        string         `json:"category"`
	ImportanceScore float64        `json:"importance_score"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	L0Abstract      string         `json:"l0_abstract,omitempty"`
	L1Overview      string         `json:"l1_overview,omitempty"`
	AvailableLevels []int          `json:"available_levels,omitempty"`
}

// ============================================================================
// Mock AGFS — 内存文件系统
// ============================================================================

type mockFile struct {
	data    []byte
	isDir   bool
	modTime time.Time
}

// MockAGFS 内存文件系统，用于替代真实 AGFS Server
type MockAGFS struct {
	mu    sync.Mutex
	files map[string]*mockFile
}

// NewMockAGFS 创建内存文件系统
func NewMockAGFS() *MockAGFS {
	return &MockAGFS{files: make(map[string]*mockFile)}
}

func (m *MockAGFS) Mkdir(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = &mockFile{isDir: true, modTime: time.Now()}
	return nil
}

func (m *MockAGFS) WriteFile(path string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	parts := strings.Split(path, "/")
	for i := 1; i < len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		if dir == "" {
			continue
		}
		if _, ok := m.files[dir]; !ok {
			m.files[dir] = &mockFile{isDir: true, modTime: time.Now()}
		}
	}
	m.files[path] = &mockFile{data: data, modTime: time.Now()}
	return nil
}

func (m *MockAGFS) ReadFile(path string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[path]
	if !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return f.data, nil
}

type MockFileInfo struct {
	Name    string
	IsDir   bool
	ModTime time.Time
}

func (m *MockAGFS) ListDir(path string) ([]MockFileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := path + "/"
	seen := make(map[string]bool)
	var result []MockFileInfo
	for p, f := range m.files {
		if !strings.HasPrefix(p, prefix) {
			continue
		}
		rest := p[len(prefix):]
		if idx := strings.Index(rest, "/"); idx >= 0 {
			name := rest[:idx]
			if !seen[name] {
				seen[name] = true
				result = append(result, MockFileInfo{Name: name, IsDir: true, ModTime: f.modTime})
			}
		} else {
			if !seen[rest] {
				seen[rest] = true
				result = append(result, MockFileInfo{Name: rest, IsDir: f.isDir, ModTime: f.modTime})
			}
		}
	}
	return result, nil
}

func (m *MockAGFS) RemoveAll(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	prefix := path + "/"
	for p := range m.files {
		if p == path || strings.HasPrefix(p, prefix) {
			delete(m.files, p)
		}
	}
	return nil
}

func (m *MockAGFS) FileExists(path string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.files[path]
	return ok
}
