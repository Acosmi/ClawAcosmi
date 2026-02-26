//go:build cgo

package services

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// setupDecayTestDB creates an in-memory SQLite database for decay tests.
// Requires CGO for go-sqlite3.
func setupDecayTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	if err := db.AutoMigrate(&models.Memory{}, &models.DecayProfile{}); err != nil {
		t.Fatalf("Failed to auto-migrate: %v", err)
	}
	return db
}

// TestApplyDecayBatch_FSRS6_StabilityDecay verifies FSRS-6 stability is used when available.
func TestApplyDecayBatch_FSRS6_StabilityDecay(t *testing.T) {
	db := setupDecayTestDB(t)

	weekAgo := time.Now().UTC().Add(-7 * 24 * time.Hour)
	m := models.Memory{
		Content:        "test fsrs decay",
		UserID:         "fsrs-user",
		MemoryType:     "observation",
		Category:       "fact",
		DecayFactor:    0.8,
		LastAccessedAt: &weekAgo,
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("Create memory: %v", err)
	}

	profile := models.DecayProfile{
		ID:            "fsrs-user:observation",
		UserID:        "fsrs-user",
		MemoryType:    "observation",
		HalfLife:      30.0,
		FSRSStability: 20.0,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("Create decay profile: %v", err)
	}

	updated, err := ApplyDecayBatch(db, "fsrs-user")
	if err != nil {
		t.Fatalf("ApplyDecayBatch: %v", err)
	}
	if updated < 1 {
		t.Error("Expected at least 1 memory to be updated")
	}
}

// TestUpdateDecayProfiles_WithSQLite tests the full UPSERT flow.
func TestUpdateDecayProfiles_WithSQLite(t *testing.T) {
	db := setupDecayTestDB(t)

	for i := 0; i < 5; i++ {
		now := time.Now().UTC()
		m := models.Memory{
			Content:        "decay test memory",
			UserID:         "decay-user",
			MemoryType:     "observation",
			Category:       "fact",
			DecayFactor:    1.0,
			LastAccessedAt: &now,
		}
		if err := db.Create(&m).Error; err != nil {
			t.Fatalf("Seed memory: %v", err)
		}
	}

	if err := UpdateDecayProfiles(db); err != nil {
		t.Fatalf("UpdateDecayProfiles: %v", err)
	}

	var profile models.DecayProfile
	if err := db.Where("user_id = ? AND memory_type = ?", "decay-user", "observation").
		First(&profile).Error; err != nil {
		t.Fatalf("Profile not found: %v", err)
	}
	if profile.HalfLife <= 0 {
		t.Errorf("HalfLife should be > 0, got %f", profile.HalfLife)
	}
}

// TestApplyDecayBatch_WithSQLite tests batch decay with real DB.
func TestApplyDecayBatch_WithSQLite(t *testing.T) {
	db := setupDecayTestDB(t)

	weekAgo := time.Now().UTC().Add(-7 * 24 * time.Hour)
	m := models.Memory{
		Content:        "old memory",
		UserID:         "batch-user",
		MemoryType:     "episodic",
		Category:       "fact",
		DecayFactor:    0.9,
		LastAccessedAt: &weekAgo,
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("Create memory: %v", err)
	}

	updated, err := ApplyDecayBatch(db, "batch-user")
	if err != nil {
		t.Fatalf("ApplyDecayBatch: %v", err)
	}
	if updated < 1 {
		t.Error("Expected at least 1 updated memory")
	}

	var updatedMem models.Memory
	if err := db.First(&updatedMem, "id = ?", m.ID).Error; err != nil {
		t.Fatalf("Reload memory: %v", err)
	}
	if updatedMem.DecayFactor >= 0.9 {
		t.Errorf("Decay factor should decrease from 0.9, got %f", updatedMem.DecayFactor)
	}
}

// TestApplyDecayBatch_ProtectedSkipped verifies protected memory types are not decayed.
func TestApplyDecayBatch_ProtectedSkipped(t *testing.T) {
	db := setupDecayTestDB(t)

	weekAgo := time.Now().UTC().Add(-7 * 24 * time.Hour)
	m := models.Memory{
		Content:        "imagination memory",
		UserID:         "prot-user",
		MemoryType:     MemoryTypeImagination,
		Category:       CategoryInsight,
		DecayFactor:    1.0,
		LastAccessedAt: &weekAgo,
	}
	if err := db.Create(&m).Error; err != nil {
		t.Fatalf("Create memory: %v", err)
	}

	updated, err := ApplyDecayBatch(db, "prot-user")
	if err != nil {
		t.Fatalf("ApplyDecayBatch: %v", err)
	}
	if updated != 0 {
		t.Errorf("Protected memories should not be decayed, updated=%d", updated)
	}
}
