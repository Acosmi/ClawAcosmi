// Package main — UHMS Local Agent 入口（P4-1/P4-2/P4-6）。
// 编译为独立二进制 `uhms-local-agent`，在租户本地运行 MCP Server。
// 功能: 本地 DB 连接 + AutoMigrate + MCP Server + 健康检查 + 隧道连接。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/uhms/go-api/internal/database"
	"github.com/uhms/go-api/internal/mcp"
	"github.com/uhms/go-api/internal/models"
	"github.com/uhms/go-api/internal/services"
)

func main() {
	// CLI 参数
	dsn := flag.String("dsn", "", "数据库连接字符串 (必须)")
	dbType := flag.String("db-type", "sqlite", "数据库类型: postgres|mysql|sqlite")
	port := flag.Int("port", 9100, "HTTP 服务端口")
	token := flag.String("token", "", "Agent 鉴权 token")
	cloudURL := flag.String("cloud-url", "", "云端 WebSocket 隧道地址 (如 wss://api.uhms.dev/api/v1/agent/ws)")
	agentName := flag.String("name", "local-agent", "Agent 名称")
	flag.Parse()

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "错误: --dsn 参数必须指定")
		flag.Usage()
		os.Exit(1)
	}

	slog.SetDefault(slog.New(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}),
	))
	slog.Info("Starting UHMS Local Agent",
		"name", *agentName, "port", *port, "db_type", *dbType)

	// --- 1. 连接本地 DB ---
	db, err := openDB(*dbType, *dsn)
	if err != nil {
		slog.Error("Database connection failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Database connected", "type", *dbType)

	// --- 2. P4-6: AutoMigrate + Schema 版本管理 ---
	if err := models.AutoMigrate(db); err != nil {
		slog.Error("AutoMigrate failed", "error", err)
		os.Exit(1)
	}
	if err := database.EnsureSchemaVersion(db); err != nil {
		slog.Warn("EnsureSchemaVersion failed", "error", err)
	}
	slog.Info("Schema migration completed")

	// --- 3. P4-2: 初始化服务 + MCP Server ---
	graphStore := services.GetGraphStore()
	vectorStore := services.GetVectorStore()
	llmClient := services.NewLLMClient(nil)
	memManager := services.NewMemoryManager(vectorStore, graphStore, llmClient)

	mcpServer := mcp.NewServer(db, memManager, graphStore)

	// --- 4. 创建 HTTP Server ---
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if pingErr := pingDB(db); pingErr != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unhealthy","error":"%s"}`, pingErr)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"healthy"}`)
	})

	// MCP HTTP 端点（Streamable HTTP）
	mcpHTTP := sdkmcp.NewStreamableHTTPHandler(
		func(r *http.Request) *sdkmcp.Server {
			return mcpServer.Inner()
		},
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)
	mux.Handle("/mcp", mcpHTTP)

	// Agent 信息端点
	mux.HandleFunc("/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w,
			`{"name":"%s","version":"2.0.0","token_set":%t}`,
			*agentName, *token != "")
	})

	// --- 统一 Context（隧道+健康检查共用）---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- WebSocket 反向隧道（DEF-AGENT-01）---
	if *cloudURL != "" && *token != "" {
		localMCPAddr := fmt.Sprintf("http://localhost:%d/mcp", *port)
		dialer := NewTunnelDialer(*cloudURL, *token, *agentName, localMCPAddr)
		go dialer.Run(ctx)
		slog.Info("Tunnel dialer started",
			"cloud_url", *cloudURL, "agent", *agentName)
	} else if *cloudURL != "" {
		slog.Warn("--cloud-url specified but --token is empty, tunnel disabled")
	}

	addr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// --- 5. P4-6: 周期性健康检查 ---
	go healthCheckLoop(ctx, db)

	// --- OPT-5: 后台定时更新自适应衰减参数 (每 24h) ---
	go decayProfileLoop(ctx, db)

	// --- 6. 启动 HTTP Server ---
	go func() {
		slog.Info("UHMS Local Agent started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// --- 优雅关闭 ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down Local Agent...")
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	slog.Info("Local Agent shutdown complete")
}

// openDB 根据类型打开数据库连接。
func openDB(dbType, dsn string) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch dbType {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "mysql":
		dialector = mysql.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn)
	default:
		return nil, fmt.Errorf("unsupported db type: %s", dbType)
	}
	return gorm.Open(dialector, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
}

// pingDB 检查数据库连接是否正常。
func pingDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return sqlDB.PingContext(ctx)
}

// healthCheckLoop 每 30 秒检查一次 DB 健康状态。
func healthCheckLoop(ctx context.Context, db *gorm.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := pingDB(db); err != nil {
				slog.Warn("Health check failed", "error", err)
			}
		}
	}
}

// decayProfileLoop 每 24h 更新一次自适应衰减参数 (OPT-5)。
// 启动后延迟 1 分钟运行首次更新，之后每 24h 执行一次。
func decayProfileLoop(ctx context.Context, db *gorm.DB) {
	// 启动延迟：等待 1 分钟让其他初始化完成
	select {
	case <-ctx.Done():
		return
	case <-time.After(1 * time.Minute):
	}

	// 立即执行首次更新
	if err := services.UpdateDecayProfiles(db); err != nil {
		slog.Warn("Initial decay profile update failed", "error", err)
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := services.UpdateDecayProfiles(db); err != nil {
				slog.Warn("Decay profile update failed", "error", err)
			}
		}
	}
}
