// Package main is the entry point for the UHMS Go API server.
// Mirrors the Python main.py: lifecycle management, CORS, exception handling, router mounting.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/uhms/go-api/internal/algo"
	"github.com/uhms/go-api/internal/cache"
	"github.com/uhms/go-api/internal/config"
	"github.com/uhms/go-api/internal/database"
	"github.com/uhms/go-api/internal/di"
	"github.com/uhms/go-api/internal/handler"
	"github.com/uhms/go-api/internal/mcp"
	"github.com/uhms/go-api/internal/middleware"
	"github.com/uhms/go-api/internal/models"
	"github.com/uhms/go-api/internal/services"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Configure log level
	logLevel := slog.LevelInfo
	if cfg.Debug {
		logLevel = slog.LevelDebug
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})))

	slog.Info("Starting UHMS API (Go)...")

	// --- Startup: Initialize dependencies ---

	// Database
	if err := database.Init(cfg); err != nil {
		slog.Error("Database initialization failed", "error", err)
		os.Exit(1)
	}
	db, _ := database.GetDB()

	// Auto-migrate all model tables (api_keys, billing_accounts, usage_logs, etc.)
	if err := models.AutoMigrate(db); err != nil {
		slog.Warn("AutoMigrate failed (non-fatal)", "error", err)
	}

	// Redis (non-fatal)
	if err := cache.Init(cfg); err != nil {
		slog.Warn("Redis initialization failed (non-fatal)", "error", err)
	}

	// --- Initialize services via DI Container ---
	container := di.NewContainer(cfg, db)

	// Initialize VectorStore connection
	if err := container.VectorStore.Initialize(cfg); err != nil {
		slog.Warn("VectorStore initialization failed (non-fatal, search degraded)", "error", err)
	}

	// Load dynamic configs from DB (non-fatal)
	if _, err := container.ConfigService.LoadConfigs(db); err != nil {
		slog.Warn("Dynamic config loading failed (non-fatal)", "error", err)
	}

	// --- MCP mode: run stdio server and exit ---
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		slog.Info("Starting MCP server (stdio transport)")
		mcpServer := mcp.NewServer(db, container.MemoryManager, container.GraphStore)
		if err := mcpServer.Run(context.Background(), &sdkmcp.StdioTransport{}); err != nil {
			slog.Error("MCP server error", "error", err)
			os.Exit(1)
		}
		return
	}

	// --- Create Gin engine ---
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS middleware — mirrors Python CORSMiddleware configuration.
	r.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Content-Type", "Authorization", "X-API-Key",
			"X-Requested-With", "Accept", "Origin",
		},
		ExposeHeaders: []string{
			"X-RateLimit-Remaining", "X-RateLimit-Reset",
			"Content-Length", "Content-Type",
		},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// API Logging middleware — records all /api/v1/* requests.
	r.Use(middleware.APILogging())

	// Dev auth bypass — parses "Bearer dev_{user_id}_{role}" tokens.
	r.Use(middleware.DevAuthBypass())

	// Billing middleware — API key validation + balance checks.
	r.Use(middleware.Billing())

	// Rate limiting middleware — 120 req/min per API key, fail-open if Redis down.
	r.Use(middleware.RateLimit())

	// --- Routes ---

	// --- Prometheus Metrics ---
	httpRequestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	httpRequestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)
	prometheus.MustRegister(httpRequestDuration, httpRequestsTotal)

	// Prometheus middleware — records request duration and count
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		httpRequestDuration.WithLabelValues(c.Request.Method, path, status).Observe(duration)
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
	})

	// Root endpoint
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":    cfg.AppName,
			"version": "2.0.0-go",
			"status":  "running",
			"docs":    "/docs",
		})
	})

	// --- Health + Readiness + Metrics ---
	healthHandler := handler.NewHealthHandler(db, cfg)
	r.GET("/health", healthHandler.Liveness)
	r.GET("/ready", healthHandler.Readiness)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// --- Tenant DB Router (shared across handlers and middleware) ---
	tenantRouter := database.NewTenantDBRouter(db)

	// API v1 route group
	v1 := r.Group(cfg.APIPrefix)

	// TenantDB middleware — injects per-tenant *gorm.DB into context
	v1.Use(middleware.TenantDB(tenantRouter))
	{
		// 8.1 Memory CRUD (5 routes)
		memHandler := handler.NewMemoryHandler(
			container.MemoryManager,
			handler.WithMemoryArchiver(container.MemoryArchiver),
			handler.WithImaginationEngine(container.ImaginationEngine),
		)
		memHandler.RegisterRoutes(v1)

		// 8.2 Graph (6 routes)
		graphHandler := handler.NewGraphHandler(container.GraphStore)
		graphHandler.RegisterRoutes(v1)

		// 8.3 SSE Events (1 route)
		eventsHandler := handler.NewEventsHandler()
		eventsHandler.RegisterRoutes(v1)

		// 8.4 Platform Management (10 routes)
		platformHandler := handler.NewPlatformHandler()
		platformHandler.RegisterRoutes(v1)

		// 8.4b Tenant DB Configuration (4 routes)
		dbConfigHandler := handler.NewDBConfigHandler(tenantRouter)
		dbConfigHandler.RegisterRoutes(v1)

		// 8.4c Tenant Memory Config (2 routes)
		memConfigHandler := handler.NewMemoryConfigHandler()
		memConfigHandler.RegisterRoutes(v1)

		// 8.5 Admin (18 routes)
		adminHandler := handler.NewAdminHandler(container.ConfigService)
		adminHandler.RegisterRoutes(v1)

		// 8.5b Algorithm API (6 routes — cloud algo for local-proxy)
		// Protected by API key (set ALGO_API_KEYS env var, comma-separated)
		algoService := algo.NewService(
			container.EmbeddingService,
			container.LLMClient,
			container.RerankService,
		)
		algoGroup := v1.Group("/")
		algoGroup.Use(algo.APIKeyAuth(algo.APIKeyConfig{
			Keys: cfg.AlgoAPIKeys(),
		}))
		algoHandler := algo.NewHandler(algoService)
		algoHandler.RegisterRoutes(algoGroup)

		// 8.5c Algo WebSocket (Phase 7A — long connection for local-proxy)
		algoWSHandler := algo.NewWSHandler(algoService, cfg.AlgoAPIKeys())
		algoWSHandler.RegisterRoutes(v1)
		// 8.6 MCP over HTTP (Streamable HTTP transport)
		// OAuth 2.1 protection for MCP endpoints
		mcpGroup := v1.Group("/")
		mcpGroup.Use(middleware.MCPOAuth(middleware.MCPOAuthConfig{
			JWTSecret:   cfg.JWTSecretKey,
			ResourceURL: cfg.APIPrefix + "/mcp",
			AuthServer:  cfg.APIPrefix + "/oauth",
		}))
		mcpHandler := handler.NewMCPHandler(tenantRouter, container.MemoryManager, container.GraphStore)
		mcpHandler.RegisterRoutes(mcpGroup)

		// 8.7 Agent WebSocket (P4-4: 安全隧道)
		tunnelPool := mcp.NewTunnelPool()
		agentRegistry := services.NewAgentRegistry(db)
		agentWSHandler := handler.NewAgentWSHandler(tunnelPool, agentRegistry, cfg.CORSOrigins)
		agentWSHandler.RegisterRoutes(v1)

		// P4-4: 启动隧道心跳检测
		go tunnelPool.HeartbeatLoop(context.Background(), 30*time.Second)
	}

	// RFC 9728 — OAuth Protected Resource Metadata (public, no auth)
	r.GET("/.well-known/oauth-protected-resource",
		middleware.ProtectedResourceMetadata(
			"https://your-domain.com"+cfg.APIPrefix+"/mcp",
			"https://your-domain.com/oauth",
		))

	// Auto-migrate tenant DB config table
	_ = db.AutoMigrate(&database.TenantDBConfig{})

	// --- FSStoreService (Phase C: AGFS localfs) ---
	// Initialization handled in DI container (NewContainer).
	// Log status here for visibility.
	if cfg.AGFSServerURL != "" {
		slog.Info("FSStoreService 已通过 AGFS localfs 启用", "agfs_url", cfg.AGFSServerURL)
	}

	// --- Background Tasks ---

	// Background context for all background goroutines — cancelled on shutdown.
	bgCtx, bgCancel := context.WithCancel(context.Background())

	// Tree Rebalance: 每小时检查过载节点并重新聚类（防树劣化）
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				if err := container.TreeManager.RebalanceTask(bgCtx, db); err != nil {
					slog.Warn("TreeManager rebalance error (non-fatal)", "error", err)
				}
			}
		}
	}()

	// OPT-5 Decay Profile Update: 1分钟延迟启动 → 首次执行 → 24h ticker
	go func() {
		select {
		case <-bgCtx.Done():
			return
		case <-time.After(1 * time.Minute):
		}
		if err := services.UpdateDecayProfiles(db); err != nil {
			slog.Warn("Initial decay profile update failed", "error", err)
		}
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				if err := services.UpdateDecayProfiles(db); err != nil {
					slog.Warn("Decay profile update failed", "error", err)
				}
			}
		}
	}()

	// OPT-5 Decay Batch Apply: 2分钟延迟启动 → 6h ticker（新记忆需更快衰减）
	go func() {
		select {
		case <-bgCtx.Done():
			return
		case <-time.After(2 * time.Minute):
		}
		applyDecayForAllUsers := func() {
			var userIDs []string
			if err := db.Model(&models.Memory{}).Distinct("user_id").Pluck("user_id", &userIDs).Error; err != nil {
				slog.Warn("Decay batch: failed to query users", "error", err)
				return
			}
			for _, uid := range userIDs {
				if _, err := services.ApplyDecayBatch(db, uid); err != nil {
					slog.Warn("ApplyDecayBatch failed", "user_id", uid, "error", err)
				}
			}
		}
		applyDecayForAllUsers()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				applyDecayForAllUsers()
			}
		}
	}()

	// L4 Imagination Engine: 注册事件驱动触发器
	container.ImaginationEngine.RegisterTrigger(services.NewActivityBurstTrigger())
	container.ImaginationEngine.RegisterTrigger(services.NewEntityClusterTrigger())
	container.ImaginationEngine.RegisterTrigger(services.NewTopicDriftTrigger())

	// L4 Imagination Engine: 5分钟延迟启动 → 72h ticker（兜底定时器）
	go func() {
		select {
		case <-bgCtx.Done():
			return
		case <-time.After(5 * time.Minute):
		}
		runImagination := func() {
			var userIDs []string
			if err := db.Model(&models.Entity{}).Distinct("user_id").Pluck("user_id", &userIDs).Error; err != nil {
				slog.Warn("Imagination scheduler: failed to query users", "error", err)
				return
			}
			for _, uid := range userIDs {
				if _, err := container.ImaginationEngine.Run(bgCtx, db, uid); err != nil {
					slog.Warn("ImaginationEngine.Run failed", "user_id", uid, "error", err)
				}
			}
		}
		runImagination()
		ticker := time.NewTicker(72 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				runImagination()
			}
		}
	}()

	// L4 Imagination: 事件驱动触发器轮询（10分钟延迟 → 15分钟 ticker）
	go func() {
		select {
		case <-bgCtx.Done():
			return
		case <-time.After(10 * time.Minute):
		}
		checkTriggers := func() {
			var userIDs []string
			if err := db.Model(&models.Memory{}).Distinct("user_id").Pluck("user_id", &userIDs).Error; err != nil {
				slog.Warn("Imagination trigger check: failed to query users", "error", err)
				return
			}
			for _, uid := range userIDs {
				triggerName := container.ImaginationEngine.CheckTriggers(db, uid)
				if triggerName != "" {
					slog.Info("Imagination event trigger fired",
						"user_id", uid, "trigger", triggerName)
					if _, err := container.ImaginationEngine.Run(bgCtx, db, uid); err != nil {
						slog.Warn("ImaginationEngine.Run (event-driven) failed",
							"user_id", uid, "trigger", triggerName, "error", err)
					}
				}
			}
		}
		checkTriggers()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-bgCtx.Done():
				return
			case <-ticker.C:
				checkTriggers()
			}
		}
	}()

	slog.Info("Registered API routes", "prefix", cfg.APIPrefix, "total", 42)

	// --- Graceful shutdown ---
	addr := fmt.Sprintf(":%d", cfg.ServerPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("UHMS API started successfully", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down UHMS API...")

	// Cancel all background goroutines first
	bgCancel()

	// Graceful shutdown with 10-second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown error", "error", err)
	}

	// Close dependencies
	if err := cache.Close(); err != nil {
		slog.Warn("Redis close error", "error", err)
	}
	if err := database.Close(); err != nil {
		slog.Warn("Database close error", "error", err)
	}

	slog.Info("UHMS API shutdown complete")
}
