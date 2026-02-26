// Package config — application context for dependency injection.
// Aggregates all shared dependencies to avoid scattered singletons.
package config

import (
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// AppContext aggregates all shared dependencies (DB, Cache, Services).
// Created once in main() and passed to handlers/services via constructor injection.
type AppContext struct {
	DB    *gorm.DB
	Redis *redis.Client
	Cfg   *Config
}
