// Package services — Memory Decay & Consolidation.
// Mirrors Python services/memory_decay.py — time-based importance decay + access-frequency boosting.
// RUST_CANDIDATE: decay_algo — 衰减算法后续迁移 Rust (nexus-decay)
package services

import (
	"fmt"
	"log/slog"
	"math"
	"time"

	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/models"
)

// Decay constants — mirrors Python HALF_LIFE_DAYS etc.
const (
	DefaultHalfLifeDays    = 30.0 // Fallback when no adaptive profile exists
	AccessBoost            = 0.05 // Each access boosts decay_factor
	MaxDecayFactor         = 1.0
	MinDecayFactor         = 0.01 // Floor to prevent full disappearance
	ConsolidationThreshold = 0.15 // Below this → eligible for consolidation
	ln2                    = 0.693147180559945
)

// TypeBaseHalfLife maps memory types to their base half-life (days).
// Derived from Duolingo HLR + Anki FSRS + Mem0 category differentiation.
var TypeBaseHalfLife = map[string]float64{
	"episodic":    14.0, // 情景记忆衰减最快
	"observation": 30.0, // 观察记忆中等
	"dialogue":    14.0, // 对话记忆同情景
	"reflection":  60.0, // 反思记忆最持久
	"semantic":    60.0, // 语义记忆同反思
	"procedural":  45.0, // 程序记忆中偏持久
	"plan":        45.0, // 计划同程序
}

// GetAdaptiveHalfLife queries the decay_profiles table for a (user, memoryType) pair.
// Returns the adaptive half-life, falling back to type base or DefaultHalfLifeDays.
func GetAdaptiveHalfLife(db *gorm.DB, userID, memoryType string) float64 {
	var profile models.DecayProfile
	err := db.Where("user_id = ? AND memory_type = ?", userID, memoryType).
		First(&profile).Error
	if err != nil {
		// Fallback: type base → global default
		if base, ok := TypeBaseHalfLife[memoryType]; ok {
			return base
		}
		return DefaultHalfLifeDays
	}
	return profile.HalfLife
}

// computeAdaptiveHalfLife calculates the adaptive half-life using the formula:
//
//	adaptive_half_life = type_base × user_activity_factor
//	user_activity_factor = 1 + log₂(1 + avg_access_interval / 7)
func computeAdaptiveHalfLife(memoryType string, avgIntervalDays float64) float64 {
	base, ok := TypeBaseHalfLife[memoryType]
	if !ok {
		base = DefaultHalfLifeDays
	}

	// user_activity_factor = 1 + log₂(1 + avg_interval / 7)
	activityFactor := 1.0 + math.Log2(1.0+avgIntervalDays/7.0)

	hl := base * activityFactor
	// Clamp to reasonable bounds: [7, 365]
	if hl < 7.0 {
		hl = 7.0
	}
	if hl > 365.0 {
		hl = 365.0
	}
	return hl
}

// UpdateDecayProfiles recalculates adaptive half-life for all active users.
// Should be called periodically (e.g. every 24h via background goroutine).
func UpdateDecayProfiles(db *gorm.DB) error {
	// 1. Find all active users (who have non-protected memories)
	var userIDs []string
	if err := db.Model(&models.Memory{}).
		Where("memory_type NOT IN ?", ProtectedMemoryTypes).
		Distinct("user_id").
		Pluck("user_id", &userIDs).Error; err != nil {
		return fmt.Errorf("query active users: %w", err)
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -30) // 30-day window
	updated := 0

	// Iterate over memory types that participate in decay
	decayTypes := []string{"episodic", "observation", "dialogue", "reflection", "semantic", "procedural", "plan"}

	for _, userID := range userIDs {
		for _, memType := range decayTypes {
			// Count accesses in last 30 days for this (user, type)
			var accessCount int64
			if err := db.Model(&models.Memory{}).
				Where("user_id = ? AND memory_type = ? AND last_accessed_at >= ?",
					userID, memType, cutoff).
				Count(&accessCount).Error; err != nil {
				slog.Warn("Failed to count accesses", "user_id", userID, "type", memType, "error", err)
				continue
			}

			// Skip if no memories of this type
			if accessCount == 0 {
				// Check if there are any memories of this type at all
				var totalCount int64
				db.Model(&models.Memory{}).
					Where("user_id = ? AND memory_type = ?", userID, memType).
					Count(&totalCount)
				if totalCount == 0 {
					continue // No memories of this type, skip
				}
			}

			// avg_access_interval = 30 / max(access_count, 1)
			effectiveCount := accessCount
			if effectiveCount < 1 {
				effectiveCount = 1
			}
			avgInterval := 30.0 / float64(effectiveCount)

			// Compute adaptive half-life
			halfLife := computeAdaptiveHalfLife(memType, avgInterval)

			// UPSERT into decay_profiles
			profileID := fmt.Sprintf("%s:%s", userID, memType)
			profile := models.DecayProfile{
				ID:              profileID,
				UserID:          userID,
				MemoryType:      memType,
				HalfLife:        halfLife,
				AccessCount30d:  int(accessCount),
				AvgIntervalDays: avgInterval,
			}

			if err := db.Where("id = ?", profileID).
				Assign(models.DecayProfile{
					HalfLife:        halfLife,
					AccessCount30d:  int(accessCount),
					AvgIntervalDays: avgInterval,
				}).
				FirstOrCreate(&profile).Error; err != nil {
				slog.Warn("Failed to upsert decay profile",
					"user_id", userID, "type", memType, "error", err)
				continue
			}
			updated++
		}
	}

	if updated > 0 {
		slog.Info("Updated decay profiles", "count", updated)
	}
	return nil
}

// ComputeEffectiveImportance calculates importance with time decay + access boost.
//
// Formula: effective = base_importance × decay_factor × time_decay × recency_boost
//   - time_decay = exp(-ln2 × days_since_access / halfLifeDays)
//   - recency_boost = 1 + log(1 + access_count) × 0.1
//
// OPT-5: halfLifeDays is now a parameter (was global constant HalfLifeDays=30).
func ComputeEffectiveImportance(
	baseImportance float64,
	decayFactor float64,
	lastAccessedAt *time.Time,
	accessCount int,
	now time.Time,
	halfLifeDays float64,
) float64 {
	if now.IsZero() {
		now = time.Now().UTC()
	}

	// OPT-5: Fallback to default if caller passes 0 or negative
	if halfLifeDays <= 0 {
		halfLifeDays = DefaultHalfLifeDays
	}

	// Time-based decay
	daysSinceAccess := 0.0
	if lastAccessedAt != nil {
		daysSinceAccess = now.Sub(*lastAccessedAt).Seconds() / 86400.0
	}
	// Guard against negative durations (clock skew)
	if daysSinceAccess < 0 {
		daysSinceAccess = 0
	}

	// FSRS-6: 使用幂律遗忘曲线替代指数衰减。
	// 当 halfLifeDays 作为稳定性 S 使用时，Retrievability 提供更符合记忆科学的衰减曲线。
	timeDecay := Retrievability(daysSinceAccess, halfLifeDays, DefaultFSRSParams.W[20])

	// Access frequency boost (logarithmic to prevent gaming)
	recencyBoost := 1.0 + math.Log1p(float64(accessCount))*0.1

	effective := baseImportance * decayFactor * timeDecay * recencyBoost
	return math.Max(MinDecayFactor, math.Min(1.0, effective))
}

// UpdateAccessStats increments access count and boosts decay_factor for a memory.
func UpdateAccessStats(db *gorm.DB, memoryID string) error {
	now := time.Now().UTC()
	return db.Model(&models.Memory{}).
		Where("id = ?", memoryID).
		Updates(map[string]any{
			"access_count":     gorm.Expr("access_count + 1"),
			"last_accessed_at": now,
			"decay_factor":     gorm.Expr("LEAST(decay_factor + ?, ?)", AccessBoost, MaxDecayFactor),
		}).Error
}

// ApplyDecayBatch applies time-based decay to all memories for a user.
// OPT-5: Uses per-(user, type) adaptive half-life from decay_profiles.
// Returns the number of memories updated.
func ApplyDecayBatch(db *gorm.DB, userID string) (int, error) {
	now := time.Now().UTC()

	var memories []models.Memory
	if err := db.Where("user_id = ? AND decay_factor > ? AND memory_type NOT IN ?",
		userID, MinDecayFactor, ProtectedMemoryTypes).
		Find(&memories).Error; err != nil {
		return 0, err
	}

	// OPT-5: Pre-load all decay profiles for this user into a map
	halfLifeMap := make(map[string]float64)
	stabilityMap := make(map[string]float64) // FSRS-6 稳定性
	var profiles []models.DecayProfile
	if err := db.Where("user_id = ?", userID).Find(&profiles).Error; err != nil {
		slog.Warn("Failed to load decay profiles, using defaults", "user_id", userID, "error", err)
	}
	for _, p := range profiles {
		halfLifeMap[p.MemoryType] = p.HalfLife
		if p.FSRSStability > 0 {
			stabilityMap[p.MemoryType] = p.FSRSStability
		}
	}

	updated := 0
	for i := range memories {
		m := &memories[i]

		// OPT-5: Resolve half-life for this memory's type
		hl, ok := halfLifeMap[m.MemoryType]
		if !ok {
			// Fallback: type base → global default
			if base, bOk := TypeBaseHalfLife[m.MemoryType]; bOk {
				hl = base
			} else {
				hl = DefaultHalfLifeDays
			}
		}

		var days float64
		if m.LastAccessedAt != nil {
			days = now.Sub(*m.LastAccessedAt).Seconds() / 86400.0
		} else {
			days = now.Sub(m.CreatedAt).Seconds() / 86400.0
		}
		// Guard against negative days
		if days < 0 {
			days = 0
		}

		// FSRS-6: 使用幂律遗忘曲线计算衰减。
		// 如果存在 FSRS 稳定性参数则优先使用，否则回退到自适应半衰期。
		stability := hl
		if fsrsS, ok := stabilityMap[m.MemoryType]; ok {
			stability = fsrsS
		}
		newDecay := m.DecayFactor * Retrievability(days, stability, DefaultFSRSParams.W[20])
		newDecay = math.Max(MinDecayFactor, newDecay)

		if math.Abs(newDecay-m.DecayFactor) > 0.001 {
			if err := db.Model(m).Update("decay_factor", newDecay).Error; err != nil {
				slog.Warn("Failed to update decay", "memory_id", m.ID, "error", err)
				continue
			}
			updated++
		}
	}

	if updated > 0 {
		slog.Info("Applied decay batch", "user_id", userID, "updated", updated)
	}
	return updated, nil
}

// GetConsolidationCandidates returns memories eligible for consolidation
// (observation type with low decay_factor).
func GetConsolidationCandidates(db *gorm.DB, userID string) ([]models.Memory, error) {
	var memories []models.Memory
	err := db.Where(
		"user_id = ? AND memory_type = ? AND decay_factor < ?",
		userID, "observation", ConsolidationThreshold,
	).Find(&memories).Error
	return memories, err
}

// GetArchiveCandidates 返回可归档的记忆（极低衰减 + 长期未访问）。
func GetArchiveCandidates(db *gorm.DB, userID string, inactiveDays int) ([]models.Memory, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -inactiveDays)
	var memories []models.Memory
	err := db.Where(
		"user_id = ? AND decay_factor < ? AND (last_accessed_at IS NULL OR last_accessed_at < ?) AND memory_type NOT IN ?",
		userID, MinDecayFactor*5, cutoff, append([]string{"reflection", "archived"}, ProtectedMemoryTypes...),
	).Find(&memories).Error
	return memories, err
}
