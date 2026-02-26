// Package services — L1.1 永久记忆归档服务。
// 实现基于 ProMem（Proactive Memory Extraction）思想的主动提问式归档管线。
// 提供 Pin/Unpin 记忆、会话级深度归档以及永久记忆分页查询。
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/middleware"
	"github.com/uhms/go-api/internal/models"
)

// RetentionPolicy 常量 — 记忆保留策略。
const (
	RetentionStandard  = "standard"  // 默认：受衰减和自动归档管控
	RetentionPermanent = "permanent" // 永久：豁免衰减，仅允许手动删除
)

// ArchivalExtraction 主动提问式归档提取结果。
type ArchivalExtraction struct {
	Decisions []string `json:"decisions"` // 关键决策及理由
	Facts     []string `json:"facts"`     // 重要事实和数据点
	Emotions  []string `json:"emotions"`  // 用户情感倾向/态度变化
	Todos     []string `json:"todos"`     // 尚未完成的待办事项或承诺
}

// MemoryArchiver 负责永久记忆归档管理。
// 支持 vector / fs / hybrid 三种存储模式。
type MemoryArchiver struct {
	db          *gorm.DB
	llm         LLMProvider
	vectorStore *VectorStoreService
	fsStore     *FSStoreService // 文件系统存储（可选，Rust VFS）
	storageMode string          // "vector" | "fs" | "hybrid"
}

// NewMemoryArchiver 创建归档器实例。
func NewMemoryArchiver(db *gorm.DB, llm LLMProvider, vs *VectorStoreService) *MemoryArchiver {
	return &MemoryArchiver{
		db:          db,
		llm:         llm,
		vectorStore: vs,
		storageMode: StorageModeVector, // 默认向量模式，保持向后兼容
	}
}

// WithFSStore 挂载文件系统存储并设置存储模式。
func (a *MemoryArchiver) WithFSStore(fs *FSStoreService, mode string) *MemoryArchiver {
	a.fsStore = fs
	if mode != "" {
		a.storageMode = mode
	}
	return a
}

// ProMem 归档 Prompt 模板 — 主动提问式深度提取，而非简单摘要。
const archivalPrompt = `你是一个记忆归档助手。请从以下对话中提取关键信息，确保重要细节不因压缩而丢失。

请提取以下四个维度的信息：
1. 关键决策及其理由（decisions）
2. 重要事实和具体数据点（facts）
3. 用户表达的情感倾向和态度变化（emotions）
4. 尚未完成的待办事项或承诺（todos）

对话内容：
%s

请以 JSON 格式返回，严格遵循以下结构：
{
  "decisions": ["决策1: 理由...", "决策2: 理由..."],
  "facts": ["事实1", "事实2"],
  "emotions": ["态度/情感变化1"],
  "todos": ["待办1", "待办2"]
}

仅返回 JSON，不要附加任何解释。`

// ArchiveSession 对一组旧对话内容执行深度提取后存为永久记忆。
// 使用 ProMem 思想：不做简单摘要，而是主动探测隐含的决策、事实、情感和待办。
func (a *MemoryArchiver) ArchiveSession(
	ctx context.Context, userID, sessionContent string,
) (*models.Memory, error) {
	if strings.TrimSpace(sessionContent) == "" {
		return nil, fmt.Errorf("session content is empty")
	}

	// 1. 使用 LLM 执行主动提问式深度提取
	var extraction ArchivalExtraction
	if a.llm != nil {
		prompt := fmt.Sprintf(archivalPrompt, sessionContent)
		result, err := a.llm.Generate(ctx, prompt)
		if err != nil {
			slog.Warn("ProMem 归档 LLM 提取失败，回退至原文存储", "error", err)
			// 降级：直接使用原始内容
			extraction = ArchivalExtraction{
				Facts: []string{sessionContent},
			}
		} else {
			if parseErr := json.Unmarshal([]byte(strings.TrimSpace(result)), &extraction); parseErr != nil {
				slog.Warn("ProMem JSON 解析失败，回退至原文存储", "error", parseErr, "raw", result)
				extraction = ArchivalExtraction{
					Facts: []string{sessionContent},
				}
			}
		}
	} else {
		// 无 LLM 时直接存储原文
		extraction = ArchivalExtraction{
			Facts: []string{sessionContent},
		}
	}

	// 2. 将提取结果序列化为结构化内容
	content := buildArchivalContent(extraction)

	// 3. 创建永久记忆记录
	now := time.Now().UTC()
	memory := &models.Memory{
		Content:         content,
		UserID:          userID,
		MemoryType:      MemoryTypePermanent,
		Category:        "summary",
		ImportanceScore: 0.7,
		DecayFactor:     MaxDecayFactor, // 永久记忆保持最大衰减因子
		ArchivedAt:      &now,
		RetentionPolicy: RetentionPermanent,
	}

	metadata := map[string]any{
		"archival_type": "proactive_extraction",
		"source":        "session_archive",
	}
	memory.Metadata = &metadata

	if err := a.db.Create(memory).Error; err != nil {
		return nil, fmt.Errorf("create permanent memory: %w", err)
	}

	slog.Info("永久记忆已归档",
		"id", memory.ID,
		"user_id", userID,
		"content_len", len(content),
	)

	// 4. 根据存储模式异步写入
	mode := a.storageMode

	// 4a. 向量存储 (vector / hybrid 模式)
	if (mode == StorageModeVector || mode == StorageModeHybrid) && a.vectorStore != nil {
		go func() {
			if err := a.vectorStore.AddMemory(
				context.Background(), memory.ID, content,
				userID, MemoryTypePermanent, memory.ImportanceScore, nil,
			); err != nil {
				slog.Error("永久记忆向量写入失败", "error", err, "id", memory.ID)
			}
		}()
	}

	// 4b. 文件系统存储 (fs / hybrid 模式)
	if (mode == StorageModeFS || mode == StorageModeHybrid) && a.fsStore != nil {
		go func() {
			// 将 ProMem 提取的各维度分别写入 VFS 对应目录
			a.writeExtractionToFS(context.Background(), userID, memory.ID, extraction)
		}()
	}

	return memory, nil
}

// PinMemory 将指定记忆标记为永久保留（用户主动钉选）。
func (a *MemoryArchiver) PinMemory(db *gorm.DB, memoryID uuid.UUID) error {
	result := db.Model(&models.Memory{}).
		Where("id = ?", memoryID).
		Updates(map[string]any{
			"retention_policy": RetentionPermanent,
			"decay_factor":     MaxDecayFactor,
		})
	if result.Error != nil {
		return fmt.Errorf("pin memory: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("memory not found: %s", memoryID)
	}

	slog.Info("记忆已标记为永久", "memory_id", memoryID)
	return nil
}

// UnpinMemory 取消指定记忆的永久保留标记。
func (a *MemoryArchiver) UnpinMemory(db *gorm.DB, memoryID uuid.UUID) error {
	result := db.Model(&models.Memory{}).
		Where("id = ?", memoryID).
		Updates(map[string]any{
			"retention_policy": RetentionStandard,
		})
	if result.Error != nil {
		return fmt.Errorf("unpin memory: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("memory not found: %s", memoryID)
	}

	slog.Info("记忆已取消永久标记", "memory_id", memoryID)
	return nil
}

// GetPermanentMemories 分页查询某用户的永久记忆。
func (a *MemoryArchiver) GetPermanentMemories(
	db *gorm.DB, userID string, page, pageSize int,
) ([]models.Memory, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := db.Model(&models.Memory{}).
		Where("user_id = ? AND retention_policy = ?", userID, RetentionPermanent).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count permanent memories: %w", err)
	}

	var memories []models.Memory
	if err := db.Where("user_id = ? AND retention_policy = ?", userID, RetentionPermanent).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&memories).Error; err != nil {
		return nil, 0, fmt.Errorf("list permanent memories: %w", err)
	}

	return memories, total, nil
}

// writeExtractionToFS 将 ProMem 提取结果按维度写入 VFS 对应目录。
// 每个维度的每条记录以子 UUID 写入，L0 取前50字符，L1 取完整内容。
func (a *MemoryArchiver) writeExtractionToFS(
	ctx context.Context, userID string, parentID uuid.UUID, ext ArchivalExtraction,
) {
	tenantID := middleware.TenantFromCtx(ctx)

	type catItems struct {
		category string
		items    []string
	}

	groups := []catItems{
		{"decisions", ext.Decisions},
		{"facts", ext.Facts},
		{"emotions", ext.Emotions},
		{"todos", ext.Todos},
	}

	for _, g := range groups {
		for _, item := range g.items {
			subID := uuid.New()
			l0 := item
			if len(l0) > 80 {
				l0 = l0[:80] + "…"
			}
			if err := a.fsStore.WriteMemory(
				ctx, tenantID, userID, subID,
				g.category, item, l0, item,
			); err != nil {
				slog.Error("VFS 记忆写入失败",
					"error", err,
					"parent_id", parentID,
					"category", g.category,
				)
			}
		}
	}

	slog.Info("ProMem 提取结果已写入 VFS",
		"parent_id", parentID,
		"user_id", userID,
		"decisions", len(ext.Decisions),
		"facts", len(ext.Facts),
		"emotions", len(ext.Emotions),
		"todos", len(ext.Todos),
	)
}

// --- Internal ---

// buildArchivalContent 将 ProMem 提取结果组装为人类可读的结构化文本。
func buildArchivalContent(ext ArchivalExtraction) string {
	var sb strings.Builder

	if len(ext.Decisions) > 0 {
		sb.WriteString("【关键决策】\n")
		for _, d := range ext.Decisions {
			sb.WriteString("- " + d + "\n")
		}
		sb.WriteString("\n")
	}

	if len(ext.Facts) > 0 {
		sb.WriteString("【重要事实】\n")
		for _, f := range ext.Facts {
			sb.WriteString("- " + f + "\n")
		}
		sb.WriteString("\n")
	}

	if len(ext.Emotions) > 0 {
		sb.WriteString("【情感倾向】\n")
		for _, e := range ext.Emotions {
			sb.WriteString("- " + e + "\n")
		}
		sb.WriteString("\n")
	}

	if len(ext.Todos) > 0 {
		sb.WriteString("【待办事项】\n")
		for _, t := range ext.Todos {
			sb.WriteString("- " + t + "\n")
		}
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "(empty archival extraction)"
	}
	return result
}
