// Package cache provides Redis connection pool and lifecycle management.
// Mirrors the Python core/redis.py module using go-redis.
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/uhms/go-api/internal/config"
)

// Client holds the global Redis client instance.
// May be nil if Redis is unavailable (fail-open behavior).
var Client *redis.Client

// Init initializes the Redis connection pool.
// Non-fatal on failure: Client remains nil, allowing fail-open behavior.
func Init(cfg *config.Config) error {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.RedisHost, cfg.RedisPort),
		DB:           cfg.RedisDB,
		PoolSize:     20,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		slog.Warn("Redis connection failed (non-fatal)", "error", err)
		Client = nil
		return nil // non-fatal, matches Python behavior
	}

	Client = client
	slog.Info("Redis connected", "host", cfg.RedisHost, "port", cfg.RedisPort)
	return nil
}

// Close gracefully shuts down the Redis connection.
func Close() error {
	if Client == nil {
		return nil
	}

	if err := Client.Close(); err != nil {
		return fmt.Errorf("redis close error: %w", err)
	}

	slog.Info("Redis connection closed")
	Client = nil
	return nil
}

// GetClient returns the global Redis client.
// May return nil if Redis is unavailable.
func GetClient() *redis.Client {
	return Client
}

// HealthCheck checks Redis connection health.
// Returns a status map compatible with the health endpoint.
func HealthCheck(ctx context.Context) map[string]interface{} {
	if Client == nil {
		return map[string]interface{}{
			"status":  "disconnected",
			"message": "Redis client not initialized",
		}
	}

	start := time.Now()
	if err := Client.Ping(ctx).Err(); err != nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}

	latency := time.Since(start).Milliseconds()
	cfg := config.Get()
	return map[string]interface{}{
		"status":     "healthy",
		"latency_ms": latency,
		"host":       cfg.RedisHost,
		"port":       cfg.RedisPort,
	}
}
