// Package di provides dependency injection container for services.
package di

import (
	"log/slog"
	"os"

	"github.com/uhms/go-api/internal/agfs"
	"github.com/uhms/go-api/internal/cache"
	"github.com/uhms/go-api/internal/config"
	"github.com/uhms/go-api/internal/services"
	"gorm.io/gorm"
)

// Container holds all initialized services with proper dependency injection.
// This replaces the Singleton pattern used throughout the codebase.
type Container struct {
	// Core services
	VectorStore   *services.VectorStoreService
	GraphStore    *services.GraphStoreService
	LLMClient     *services.LLMClient
	MemoryManager *services.MemoryManager
	TreeManager   *services.TreeManager

	// Platform services
	ConfigService    *services.DynamicConfigService
	BillingService   *services.BillingService
	Metrics          *services.MemoryMetrics
	RerankService    *services.RerankService
	EmbeddingService services.EmbeddingService
	SessionCommitter *services.SessionCommitter

	// L1.1 永久记忆归档 + L4 想象记忆引擎
	MemoryArchiver    *services.MemoryArchiver
	ImaginationEngine *services.ImaginationEngine

	// Caching
	MemoryCache *services.MemoryCache
}

// NewContainer initializes all services with proper dependency injection.
// Services are initialized in dependency order to ensure correct wiring.
// Note: This uses a hybrid approach - some services still use Singleton pattern
// and will be migrated incrementally.
func NewContainer(cfg *config.Config, db *gorm.DB) *Container {
	// Initialize services in dependency order

	// 1. Use existing Singletons for services without New constructors
	graphStore := services.GetGraphStore()
	treeManager := services.GetTreeManager()
	metrics := services.GetMetrics()
	configService := services.GetDynamicConfigService()
	billingService := services.GetBillingService()
	rerankService := services.GetRerankService()
	embeddingService := services.GetEmbeddingService()

	// 2. LLM Client (has New constructor)
	llmClient := services.NewLLMClient(cfg)

	// 2b. MemoryCache — Redis + local fallback（Redis 不可用时降级本地缓存）
	memoryCache := services.NewMemoryCache(cache.GetClient())

	// 3. VectorStore (uses Singleton but with Init)
	services.InitVectorStore(cfg)
	vectorStore := services.GetVectorStore()

	// 4. MemoryManager (has New constructor with DI)
	// WithCache: read-through Redis/local cache for SearchMemories results.
	memoryManager := services.NewMemoryManager(vectorStore, graphStore, llmClient).
		WithCache(memoryCache)

	// 6. SessionCommitter（依赖 LLM + VectorStore + DB）
	sessionCommitter := services.NewSessionCommitter(db, llmClient, vectorStore)

	// 6b. L1.1 MemoryArchiver（依赖 DB + LLM + VectorStore）
	memoryArchiver := services.NewMemoryArchiver(db, llmClient, vectorStore)

	// 6c. L4 ImaginationEngine（依赖 GraphStore + VectorStore + LLM + WebSearch）
	imaginationEngine := services.NewImaginationEngine(
		graphStore, vectorStore, llmClient, services.GetWebSearchProvider(),
	)

	// 7. FSStoreService — Phase C: 使用 AGFS localfs 共享存储
	agfsURL := cfg.AGFSServerURL
	if agfsURL == "" {
		agfsURL = os.Getenv("AGFS_URL")
	}
	if agfsURL != "" {
		agfsClient := agfs.NewAGFSClient(agfsURL)
		fsStore, err := services.NewFSStoreService(agfsClient)
		if err == nil {
			// Initialize distributed VFS path lock with the same AGFS client.
			// Without this call the global lock falls back to local-only mode
			// and provides no cross-instance serialization.
			services.InitVFSLock(agfsClient)
			slog.Info("VFSPathLock 已初始化 (AGFS 分布式锁)", "agfs_url", agfsURL)

			mode := os.Getenv("MEMFS_STORAGE_MODE")
			if mode == "" {
				mode = services.StorageModeHybrid
			}
			sessionCommitter.WithFSStore(fsStore, mode)
			memoryManager.WithFSStore(fsStore, mode)
			slog.Info("SessionCommitter 已挂载 FSStoreService (AGFS)",
				"agfs_url", agfsURL, "mode", mode)

			// 8. VFS Semantic Index — Phase D: 挂载到 MemoryManager
			if segStore := vectorStore.GetSegmentStore(); segStore != nil {
				if embSvc := vectorStore.GetEmbeddingSvc(); embSvc != nil {
					vfsSemIdx := services.NewVFSSemanticIndex(segStore, embSvc)
					if err := vfsSemIdx.EnsureCollection(vectorStore.Dimension()); err != nil {
						slog.Warn("VFS 语义索引初始化失败", "error", err)
					} else {
						memoryManager.WithVFSSemanticIndex(vfsSemIdx)
						slog.Info("VFS 语义索引已挂载到 MemoryManager (Phase D)")
					}
				}
			}
		} else {
			slog.Warn("FSStoreService 初始化失败（降级为纯 vector 模式）", "error", err)
		}
	}

	return &Container{
		VectorStore:       vectorStore,
		GraphStore:        graphStore,
		LLMClient:         llmClient,
		MemoryManager:     memoryManager,
		TreeManager:       treeManager,
		ConfigService:     configService,
		BillingService:    billingService,
		Metrics:           metrics,
		RerankService:     rerankService,
		EmbeddingService:  embeddingService,
		SessionCommitter:  sessionCommitter,
		MemoryArchiver:    memoryArchiver,
		ImaginationEngine: imaginationEngine,
		MemoryCache:       memoryCache,
	}
}
