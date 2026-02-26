package uhms

import (
	"context"
	"log/slog"
	"math"
	"time"
)

// FSRS-6 inspired decay model:
//   decay_factor = max(factor * e^(-λt), min_decay)
//   λ = ln(2) / half_life
//   half_life adapts per (user, memoryType) based on access patterns.
//
// Access boost: each access multiplies decay_factor by 1.1 (capped at 1.0).

const (
	defaultHalfLifeHours = 168.0 // 7 days
	defaultMinDecay      = 0.01
	accessBoostFactor    = 1.1
	maxDecayFactor       = 1.0
)

// runDecayCycle applies FSRS-6 decay to all non-permanent memories for a user.
func runDecayCycle(ctx context.Context, store *Store, userID string) error {
	slog.Debug("uhms/decay: starting cycle", "userID", userID)
	totalAffected := int64(0)

	for _, mt := range AllMemoryTypes {
		if mt == MemTypePermanent || mt == MemTypeImagination {
			continue // 永久和想象记忆豁免
		}

		// 获取该类型的衰减参数
		profile, err := store.GetDecayProfile(userID, mt)
		halfLife := defaultHalfLifeHours
		minDecay := defaultMinDecay
		if err == nil && profile != nil {
			halfLife = profile.HalfLife
			minDecay = profile.MinDecay
		}

		// 计算衰减系数: e^(-λ * interval)
		// λ = ln(2) / halfLife
		// interval = decayIntervalHours (默认 6 小时, 由 config 控制)
		intervalHours := 6.0 // 单次调用的时间间隔
		lambda := math.Ln2 / halfLife
		factor := math.Exp(-lambda * intervalHours)

		affected, err := store.BatchUpdateDecay(userID, mt, factor, minDecay)
		if err != nil {
			slog.Warn("uhms/decay: batch update failed", "memoryType", mt, "error", err)
			continue
		}
		totalAffected += affected
	}

	slog.Debug("uhms/decay: cycle complete", "userID", userID, "affected", totalAffected)
	return nil
}

// StartDecayTicker starts a background ticker that runs decay cycles periodically.
// Returns a stop function.
func StartDecayTicker(ctx context.Context, store *Store, userID string, intervalHours int) func() {
	if intervalHours <= 0 {
		intervalHours = 6
	}

	tickerCtx, cancel := context.WithCancel(ctx)
	interval := time.Duration(intervalHours) * time.Hour

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-tickerCtx.Done():
				return
			case <-ticker.C:
				if err := runDecayCycle(tickerCtx, store, userID); err != nil {
					slog.Warn("uhms/decay: ticker cycle error", "error", err)
				}
			}
		}
	}()

	return cancel
}

// ApplyAccessBoost increases a memory's decay factor after access.
func ApplyAccessBoost(store *Store, memoryID string) error {
	mem, err := store.GetMemory(memoryID)
	if err != nil {
		return err
	}

	newFactor := mem.DecayFactor * accessBoostFactor
	if newFactor > maxDecayFactor {
		newFactor = maxDecayFactor
	}
	mem.DecayFactor = newFactor

	return store.UpdateMemory(mem)
}
