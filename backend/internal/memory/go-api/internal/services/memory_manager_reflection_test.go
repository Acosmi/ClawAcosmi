//go:build cgo

// Package services — Unit tests for MemoryManager Reflection functionality.
package services

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Auto-migrate models
	if err := db.AutoMigrate(&models.Memory{}); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	return db
}

func TestTriggerReflection_Success(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Create test LLM
	mockLLM := &mockLLMProvider{
		reflectionResult: "用户最近频繁提到编程和 Go 语言，显示出对技术学习的浓厚兴趣。",
	}

	memManager := &MemoryManager{
		vectorStore: &VectorStoreService{},
		graphStore:  &GraphStoreService{},
		treeManager: &TreeManager{},
		llmClient:   mockLLM,
	}

	// Insert test memories
	userID := "test_user_123"
	for i := 0; i < 5; i++ {
		memory := &models.Memory{
			Content:         "用户喜欢学习 Go 语言",
			UserID:          userID,
			MemoryType:      MemoryTypeObservation,
			Category:        CategoryFact,
			ImportanceScore: 0.8,
		}
		db.Create(memory)
	}

	// Trigger reflection
	reflection, err := memManager.TriggerReflection(ctx, db, userID)

	if err != nil {
		t.Fatalf("TriggerReflection failed: %v", err)
	}

	if reflection == nil {
		t.Fatal("Expected reflection to be created, got nil")
	}

	// Verify reflection properties
	if reflection.MemoryType != MemoryTypeReflection {
		t.Errorf("Expected memory_type=%s, got %s", MemoryTypeReflection, reflection.MemoryType)
	}

	if reflection.Category != CategoryInsight {
		t.Errorf("Expected category=%s, got %s", CategoryInsight, reflection.Category)
	}

	if reflection.ImportanceScore != 0.8 {
		t.Errorf("Expected importance_score=0.8, got %.2f", reflection.ImportanceScore)
	}

	// Verify reflection was saved to database
	var savedReflection models.Memory
	result := db.Where("id = ?", reflection.ID).First(&savedReflection)
	if result.Error != nil {
		t.Fatalf("Reflection not found in database: %v", result.Error)
	}
}

func TestTriggerReflection_NotEnoughMemories(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	mockLLM := &mockLLMProvider{reflectionResult: "test"}
	memManager := &MemoryManager{
		llmClient: mockLLM,
	}

	// Insert only 2 memories (less than threshold of 3)
	userID := "test_user_456"
	for i := 0; i < 2; i++ {
		memory := &models.Memory{
			Content:         "Test memory",
			UserID:          userID,
			MemoryType:      MemoryTypeObservation,
			ImportanceScore: 0.8,
		}
		db.Create(memory)
	}

	// Should return nil without error
	reflection, err := memManager.TriggerReflection(ctx, db, userID)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if reflection != nil {
		t.Fatalf("Expected nil reflection, got: %v", reflection)
	}
}

func TestTriggerReflection_LLMError(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// Test LLM with error
	mockLLM := &mockLLMProvider{
		reflectionError: errors.New("LLM API failure"),
	}

	memManager := &MemoryManager{
		llmClient: mockLLM,
	}

	// Insert sufficient memories
	userID := "test_user_789"
	for i := 0; i < 5; i++ {
		memory := &models.Memory{
			Content:         "Test memory",
			UserID:          userID,
			MemoryType:      MemoryTypeObservation,
			ImportanceScore: 0.8,
		}
		db.Create(memory)
	}

	// Should return error
	reflection, err := memManager.TriggerReflection(ctx, db, userID)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if reflection != nil {
		t.Fatalf("Expected nil reflection on error, got: %v", reflection)
	}
}

func TestTriggerReflection_NoLLMClient(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	// MemoryManager without LLM client
	memManager := &MemoryManager{
		llmClient: nil,
	}

	// Insert memories
	userID := "test_user_000"
	for i := 0; i < 5; i++ {
		memory := &models.Memory{
			Content:         "Test memory",
			UserID:          userID,
			MemoryType:      MemoryTypeObservation,
			ImportanceScore: 0.8,
		}
		db.Create(memory)
	}

	// Should return nil without error
	reflection, err := memManager.TriggerReflection(ctx, db, userID)

	if err != nil {
		t.Fatalf("Expected no error when LLM is nil, got: %v", err)
	}

	if reflection != nil {
		t.Fatalf("Expected nil reflection when LLM is nil, got: %v", reflection)
	}
}
