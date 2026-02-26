// Package database provides PostgreSQL connection pool and session management.
// Mirrors the Python core/database.py module using GORM with pgx driver.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/uhms/go-api/internal/config"
)

// DB holds the global GORM database instance.
var DB *gorm.DB

// Init initializes the PostgreSQL connection pool.
// Called during application startup.
func Init(cfg *config.Config) error {
	dsn := cfg.DatabaseURL()

	// Configure GORM logger level based on debug flag.
	logLevel := logger.Warn
	if cfg.Debug {
		logLevel = logger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool — mirrors Python pool_size=5, max_overflow=10.
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(15)                  // pool_size(5) + max_overflow(10)
	sqlDB.SetMaxIdleConns(5)                   // pool_size
	sqlDB.SetConnMaxLifetime(30 * time.Minute) // prevent stale connections

	// Verify connection (pool_pre_ping equivalent).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	DB = db
	slog.Info("Database initialized", "host", cfg.PostgresHost, "port", cfg.PostgresPort, "db", cfg.PostgresDB)
	return nil
}

// Close gracefully closes the database connection pool.
// Called during application shutdown.
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	slog.Info("Database connection closed")
	return sqlDB.Close()
}

// GetDB returns the global database instance.
// Returns an error if the database has not been initialized.
func GetDB() (*gorm.DB, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized, call database.Init() first")
	}
	return DB, nil
}
