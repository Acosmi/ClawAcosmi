// Package handler — Health check endpoints for production readiness.
// Provides liveness and readiness probes per Kubernetes/Docker convention.
package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/uhms/go-api/internal/config"
)

// HealthHandler provides liveness and readiness endpoints.
type HealthHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

// NewHealthHandler creates a HealthHandler with injected dependencies.
func NewHealthHandler(db *gorm.DB, cfg *config.Config) *HealthHandler {
	return &HealthHandler{db: db, cfg: cfg}
}

// Liveness returns 200 if the process is alive. Used by orchestrators
// to decide whether to restart the container. No dependency checks.
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Readiness returns 200 only if all critical dependencies are reachable.
// Used by load balancers to decide whether to route traffic.
func (h *HealthHandler) Readiness(c *gin.Context) {
	checks := make(map[string]string)
	healthy := true

	// Check database connectivity
	if h.db != nil {
		sqlDB, err := h.db.DB()
		if err != nil {
			checks["database"] = "error: " + err.Error()
			healthy = false
		} else if err := sqlDB.Ping(); err != nil {
			checks["database"] = "unreachable: " + err.Error()
			healthy = false
		} else {
			checks["database"] = "ok"
		}
	} else {
		checks["database"] = "not configured"
	}

	// Check AGFS connectivity (if configured)
	if h.cfg.AGFSServerURL != "" {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(h.cfg.AGFSServerURL + "/health")
		if err != nil {
			checks["agfs"] = "unreachable: " + err.Error()
			// AGFS is non-fatal for readiness
		} else {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				checks["agfs"] = "ok"
			} else {
				checks["agfs"] = "unhealthy"
			}
		}
	} else {
		checks["agfs"] = "not configured"
	}

	status := http.StatusOK
	statusText := "ready"
	if !healthy {
		status = http.StatusServiceUnavailable
		statusText = "not ready"
	}

	c.JSON(status, gin.H{
		"status":    statusText,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks":    checks,
	})
}
