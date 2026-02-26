// Package models provides GORM ORM models for all database tables.
// Mirrors Python models/orm.py — all 10 table models.
package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// --- Memory ---

// Memory represents the memories table — core storage for episodic memory.
type Memory struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	Content         string          `gorm:"type:text;not null" json:"content"`
	UserID          string          `gorm:"type:varchar(100);not null;index" json:"user_id"`
	MemoryType      string          `gorm:"type:varchar(50);not null;default:'observation'" json:"memory_type"`
	Category        string          `gorm:"type:varchar(50);not null;default:'fact';index" json:"category"`
	ImportanceScore float64         `gorm:"type:float;not null;default:0.5" json:"importance_score"`
	EmbeddingRef    *uuid.UUID      `gorm:"type:uuid" json:"embedding_ref,omitempty"`
	Metadata        *map[string]any `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       *time.Time      `gorm:"autoUpdateTime" json:"updated_at,omitempty"`
	AccessCount     int             `gorm:"not null;default:0" json:"access_count"`
	LastAccessedAt  *time.Time      `json:"last_accessed_at,omitempty"`
	DecayFactor     float64         `gorm:"type:float;not null;default:1.0" json:"decay_factor"`
	ArchivedAt      *time.Time      `json:"archived_at,omitempty"`
	RetentionPolicy string          `gorm:"type:varchar(30);not null;default:'standard'" json:"retention_policy"`
	EventTime       *time.Time      `gorm:"index" json:"event_time,omitempty"`
	IngestedAt      time.Time       `gorm:"autoCreateTime;index" json:"ingested_at"`
}

func (Memory) TableName() string { return "memories" }

// --- API Key ---

// ApiKey represents the api_keys table — API key authentication records.
type ApiKey struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	Key            string     `gorm:"type:varchar(100);uniqueIndex;not null" json:"key"`
	Name           string     `gorm:"type:varchar(100);not null" json:"name"`
	UserID         string     `gorm:"type:varchar(100);not null;index" json:"user_id"`
	IsActive       bool       `gorm:"not null;default:true" json:"is_active"`
	Role           string     `gorm:"type:varchar(20);not null;default:'user'" json:"role"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	AllowedIPs     *[]string  `gorm:"type:jsonb" json:"allowed_ips,omitempty"`
	AllowedDomains *[]string  `gorm:"type:jsonb" json:"allowed_domains,omitempty"`
}

func (ApiKey) TableName() string { return "api_keys" }

// --- Usage Log ---

// UsageLog represents the usage_logs table — API usage tracking.
type UsageLog struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	ApiKeyID     *uuid.UUID `gorm:"type:uuid;index" json:"api_key_id,omitempty"`
	UserID       string     `gorm:"type:varchar(100);not null;index" json:"user_id"`
	Endpoint     string     `gorm:"type:varchar(200);not null" json:"endpoint"`
	Method       string     `gorm:"type:varchar(20);not null" json:"method"`
	StatusCode   int        `gorm:"not null" json:"status_code"`
	LatencyMS    int        `gorm:"not null;default:0" json:"latency_ms"`
	ErrorMessage *string    `gorm:"type:text" json:"error_message,omitempty"`
	Timestamp    time.Time  `gorm:"autoCreateTime;index" json:"timestamp"`
}

func (UsageLog) TableName() string { return "usage_logs" }

// --- Billing Account ---

// BillingAccount represents the billing_accounts table.
type BillingAccount struct {
	UserID    string          `gorm:"type:varchar(100);primaryKey" json:"user_id"`
	Balance   decimal.Decimal `gorm:"type:numeric(12,6);not null;default:0" json:"balance"`
	Currency  string          `gorm:"type:varchar(10);not null;default:'USD'" json:"currency"`
	UpdatedAt time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

func (BillingAccount) TableName() string { return "billing_accounts" }

// --- Transaction ---

// Transaction represents the transactions table — billing records.
type Transaction struct {
	ID              uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	UserID          string          `gorm:"type:varchar(100);not null;index" json:"user_id"`
	Amount          decimal.Decimal `gorm:"type:numeric(12,6);not null" json:"amount"`
	TransactionType string          `gorm:"type:varchar(50);not null" json:"transaction_type"`
	Description     *string         `gorm:"type:varchar(255)" json:"description,omitempty"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"created_at"`
}

func (Transaction) TableName() string { return "transactions" }

// --- Entity (Knowledge Graph Node) ---

// Entity represents the entities table — knowledge graph nodes.
type Entity struct {
	ID          uuid.UUID       `gorm:"type:uuid;primaryKey" json:"id"`
	Name        string          `gorm:"type:varchar(200);not null;index" json:"name"`
	EntityType  string          `gorm:"type:varchar(100);not null;index" json:"entity_type"`
	Description *string         `gorm:"type:text" json:"description,omitempty"`
	UserID      string          `gorm:"type:varchar(100);not null;index" json:"user_id"`
	CommunityID *int            `gorm:"index" json:"community_id,omitempty"`
	Metadata    *map[string]any `gorm:"type:jsonb" json:"metadata,omitempty"`
	CreatedAt   time.Time       `gorm:"autoCreateTime" json:"created_at"`
	ValidFrom   *time.Time      `gorm:"index" json:"valid_from,omitempty"`
	ValidUntil  *time.Time      `json:"valid_until,omitempty"`
	EventTime   *time.Time      `gorm:"index" json:"event_time,omitempty"`
	IngestedAt  time.Time       `gorm:"autoCreateTime;index" json:"ingested_at"` // Bitemporal: 数据摄入时间

	// Relationships
	OutgoingRelations []Relation `gorm:"foreignKey:SourceID" json:"outgoing_relations,omitempty"`
	IncomingRelations []Relation `gorm:"foreignKey:TargetID" json:"incoming_relations,omitempty"`
}

func (Entity) TableName() string { return "entities" }

// --- Relation (Knowledge Graph Edge) ---

// Relation represents the relations table — knowledge graph edges.
type Relation struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	SourceID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"source_id"`
	TargetID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"target_id"`
	RelationType string     `gorm:"type:varchar(100);not null;index" json:"relation_type"`
	Weight       float64    `gorm:"type:float;not null;default:1.0" json:"weight"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
	ValidFrom    *time.Time `json:"valid_from,omitempty"`
	ValidUntil   *time.Time `json:"valid_until,omitempty"`
	IngestedAt   time.Time  `gorm:"autoCreateTime;index" json:"ingested_at"` // Bitemporal: 数据摄入时间

	// Relationships
	Source *Entity `gorm:"foreignKey:SourceID" json:"source,omitempty"`
	Target *Entity `gorm:"foreignKey:TargetID" json:"target,omitempty"`
}

func (Relation) TableName() string { return "relations" }

// --- Memory-Entity Link ---

// MemoryEntityLink represents the memory_entity_links junction table.
type MemoryEntityLink struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	MemoryID  uuid.UUID `gorm:"type:uuid;not null;index" json:"memory_id"`
	EntityID  uuid.UUID `gorm:"type:uuid;not null;index" json:"entity_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (MemoryEntityLink) TableName() string { return "memory_entity_links" }

// --- Memory Tree Node ---

// MemoryTreeNode represents a node in the MemTree hierarchical memory structure.
// Implements arXiv:2410.14052 — dynamic tree memory representation.
type MemoryTreeNode struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID        string     `gorm:"type:varchar(100);not null;index" json:"user_id"`
	ParentID      *uuid.UUID `gorm:"type:uuid;index" json:"parent_id,omitempty"`
	MemoryID      *uuid.UUID `gorm:"type:uuid;index" json:"memory_id,omitempty"`
	Content       string     `gorm:"type:text;not null;default:''" json:"content"`
	Category      string     `gorm:"type:varchar(50);not null;default:'fact'" json:"category"`
	Depth         int        `gorm:"not null;default:0" json:"depth"`
	IsLeaf        bool       `gorm:"not null;default:true" json:"is_leaf"`
	NodeType      string     `gorm:"type:varchar(20);not null;default:'leaf'" json:"node_type"`
	ChildrenCount int        `gorm:"not null;default:0" json:"children_count"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`

	// GORM relationships
	Parent   *MemoryTreeNode  `gorm:"foreignKey:ParentID" json:"-"`
	Children []MemoryTreeNode `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Memory   *Memory          `gorm:"foreignKey:MemoryID" json:"memory,omitempty"`
}

func (MemoryTreeNode) TableName() string { return "memory_tree_nodes" }

// --- Notification Preference ---

// NotificationPreference represents user notification settings.
type NotificationPreference struct {
	UserID           string          `gorm:"type:varchar(100);primaryKey" json:"user_id"`
	Email            *string         `gorm:"type:varchar(100)" json:"email,omitempty"`
	EmailEnabled     bool            `gorm:"not null;default:true" json:"email_enabled"`
	SMSEnabled       bool            `gorm:"not null;default:false" json:"sms_enabled"`
	Phone            *string         `gorm:"type:varchar(20)" json:"phone,omitempty"`
	BalanceThreshold decimal.Decimal `gorm:"type:numeric(12,2);not null;default:10.00" json:"balance_threshold"`
	QuietHoursStart  *int            `json:"quiet_hours_start,omitempty"`
	QuietHoursEnd    *int            `json:"quiet_hours_end,omitempty"`
	CreatedAt        time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        *time.Time      `gorm:"autoUpdateTime" json:"updated_at,omitempty"`
}

func (NotificationPreference) TableName() string { return "notification_preferences" }

// --- Notification Log ---

// NotificationLog represents notification sending records.
type NotificationLog struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID           string     `gorm:"type:varchar(100);not null;index" json:"user_id"`
	NotificationType string     `gorm:"type:varchar(50);not null;index" json:"notification_type"`
	Channel          string     `gorm:"type:varchar(20);not null" json:"channel"`
	Status           string     `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	Recipient        string     `gorm:"type:varchar(100);not null" json:"recipient"`
	Subject          *string    `gorm:"type:varchar(200)" json:"subject,omitempty"`
	Content          string     `gorm:"type:text;not null" json:"content"`
	ErrorMessage     *string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt        time.Time  `gorm:"autoCreateTime;index" json:"created_at"`
	SentAt           *time.Time `json:"sent_at,omitempty"`
}

func (NotificationLog) TableName() string { return "notification_logs" }

// DocEndpoint represents API documentation entries managed via admin CMS.
type DocEndpoint struct {
	ID              string          `gorm:"type:varchar(100);primaryKey" json:"id"`
	Category        string          `gorm:"type:varchar(50);not null;index" json:"category"`
	Name            string          `gorm:"type:varchar(200);not null" json:"name"`
	Method          string          `gorm:"type:varchar(10);not null" json:"method"`
	Path            string          `gorm:"type:varchar(500);not null" json:"path"`
	Description     string          `gorm:"type:text;not null;default:''" json:"description"`
	PricingInfo     *string         `gorm:"type:varchar(100)" json:"pricing_info,omitempty"`
	IsMock          bool            `gorm:"not null;default:false" json:"is_mock"`
	IsPublic        bool            `gorm:"not null;default:true;index" json:"is_public"`
	DisplayLabel    *string         `gorm:"type:varchar(300)" json:"display_label,omitempty"`
	Parameters      json.RawMessage `gorm:"type:jsonb" json:"parameters,omitempty"`
	RequestBody     json.RawMessage `gorm:"type:jsonb" json:"request_body,omitempty"`
	ResponseExample json.RawMessage `gorm:"type:jsonb" json:"response_example,omitempty"`
	CodeExamples    json.RawMessage `gorm:"type:jsonb" json:"code_examples,omitempty"`
	SortOrder       int             `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       *time.Time      `gorm:"autoUpdateTime" json:"updated_at,omitempty"`
}

func (DocEndpoint) TableName() string { return "doc_endpoints" }

// ============================================================================
// BeforeCreate hooks — UUID auto-generation.
// GORM sends Go's zero-value UUID on INSERT, overriding PG's gen_random_uuid().
// These hooks generate UUIDs in Go before INSERT.
// ============================================================================

func (m *Memory) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

func (a *ApiKey) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

func (u *UsageLog) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (e *Entity) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

func (r *Relation) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	// Bi-temporal: 自动设置 valid_from 为当前时间（关系生效起始时间）
	if r.ValidFrom == nil {
		now := time.Now()
		r.ValidFrom = &now
	}
	return nil
}

func (m *MemoryEntityLink) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

func (n *NotificationLog) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (t *MemoryTreeNode) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// --- Provider Constants (from core/models.py) ---

// EmbeddingModelDefaults maps provider names to their default model names.
var EmbeddingModelDefaults = map[string]string{
	"local":      "BAAI/bge-small-zh-v1.5",
	"openai":     "text-embedding-3-small",
	"google":     "text-embedding-004",
	"azure":      "text-embedding-3-small",
	"aliyun":     "text-embedding-v3",
	"tencent":    "hunyuan-embedding",
	"volcengine": "doubao-embedding",
}

// EmbeddingDimensionDefaults maps model names to their vector dimensions.
var EmbeddingDimensionDefaults = map[string]int{
	"BAAI/bge-small-zh-v1.5": 512,
	"text-embedding-3-small": 1536,
	"text-embedding-3-large": 3072,
	"text-embedding-004":     768,
	"text-embedding-v3":      1024,
	"hunyuan-embedding":      1024,
	"doubao-embedding":       2560,
}

// ProviderBaseURLs maps provider names to their OpenAI-compatible API base URLs.
var ProviderBaseURLs = map[string]string{
	"openai":     "https://api.openai.com/v1",
	"aliyun":     "https://dashscope.aliyuncs.com/compatible-mode/v1",
	"volcengine": "https://ark.cn-beijing.volces.com/api/v3",
}

// RerankModelDefaults maps rerank provider names to their default models.
var RerankModelDefaults = map[string]string{
	"cohere":      "rerank-english-v3.0",
	"siliconflow": "BAAI/bge-reranker-v2-m3",
	"aliyun":      "gte-rerank",
	"volcengine":  "doubao-rerank",
}

// --- SystemConfig (Dynamic Configuration) ---

// ConfigGroup defines standard configuration groups.
const (
	ConfigGroupGeneral   = "general"
	ConfigGroupLLM       = "llm"
	ConfigGroupEmbedding = "embedding"
	ConfigGroupRerank    = "rerank"
	ConfigGroupSecurity  = "security"
	ConfigGroupDatabase  = "database"
)

// SecretKeys defines keys whose values should always be encrypted.
var SecretKeys = map[string]bool{
	"EMBEDDING_API_KEY": true,
	"RERANK_API_KEY":    true,
	"OPENAI_API_KEY":    true,
	"ANTHROPIC_API_KEY": true,
	"GEMINI_API_KEY":    true,
	"DEEPSEEK_API_KEY":  true,
	"QWEN_API_KEY":      true,
	"DOUBAO_API_KEY":    true,
	"JWT_SECRET_KEY":    true,
}

// SystemConfig represents the system_configs table — dynamic configuration storage.
type SystemConfig struct {
	Key         string     `gorm:"type:varchar(255);primaryKey" json:"key"`
	Value       string     `gorm:"type:text;not null;default:''" json:"value"`
	Group       string     `gorm:"type:varchar(50);not null;default:'general';index" json:"group"`
	IsSecret    bool       `gorm:"not null;default:false" json:"is_secret"`
	Description *string    `gorm:"type:varchar(500)" json:"description,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   *time.Time `gorm:"autoUpdateTime" json:"updated_at,omitempty"`
}

func (SystemConfig) TableName() string { return "system_configs" }

// --- Core Memory (Agent 自编辑) ---

// CoreMemorySection 定义 Core Memory 的分区枚举。
const (
	CoreMemSectionPersona      = "persona"      // 用户画像
	CoreMemSectionPreferences  = "preferences"  // 用户偏好
	CoreMemSectionInstructions = "instructions" // Agent 行为指令
)

// CoreMemory 表示 Agent 可自编辑的核心记忆分区。
// 参考 Letta/MemGPT Core Memory 模型。
type CoreMemory struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string     `gorm:"type:varchar(100);not null;uniqueIndex:idx_core_mem_user_section" json:"user_id"`
	Section   string     `gorm:"type:varchar(50);not null;uniqueIndex:idx_core_mem_user_section" json:"section"`
	Content   string     `gorm:"type:text;not null;default:''" json:"content"`
	UpdatedBy string     `gorm:"type:varchar(20);not null;default:'system'" json:"updated_by"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt *time.Time `gorm:"autoUpdateTime" json:"updated_at,omitempty"`
}

func (CoreMemory) TableName() string { return "core_memories" }

func (c *CoreMemory) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// --- Tenant Memory Config (记忆存储模式配置) ---

// TenantMemoryConfig 租户记忆存储模式配置。
// 允许租户选择 vector / fs / hybrid 三种永久记忆存储方式。
type TenantMemoryConfig struct {
	TenantID          string     `gorm:"type:varchar(100);primaryKey" json:"tenant_id"`
	MemoryStorageMode string     `gorm:"type:varchar(20);not null;default:'vector'" json:"memory_storage_mode"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         *time.Time `gorm:"autoUpdateTime" json:"updated_at,omitempty"`
}

func (TenantMemoryConfig) TableName() string { return "tenant_memory_configs" }

// --- Schema Version (P3-2: 多租户 Schema 版本管理) ---

// SchemaVersion 记录租户 DB 的 schema 版本信息。
// 每次 AutoMigrate 成功后插入一条记录，用于追踪版本一致性。
type SchemaVersion struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Version    int       `gorm:"not null;default:1" json:"version"`
	AppliedAt  time.Time `gorm:"autoCreateTime" json:"applied_at"`
	AppVersion string    `gorm:"type:varchar(50);not null" json:"app_version"`
	Detail     string    `gorm:"type:text;not null;default:''" json:"detail"`
}

func (SchemaVersion) TableName() string { return "schema_versions" }

func (s *SchemaVersion) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// --- Decay Profile (OPT-5: 自适应衰减参数) ---

// DecayProfile 存储 (用户 × 记忆类型) 的自适应半衰期参数。
// 由后台定时任务每 24h 根据用户访问模式更新。
// FSRS-6 扩展：存储 per-(user, type) 的 FSRS 稳定性和难度参数。
type DecayProfile struct {
	ID              string     `gorm:"type:varchar(100);primaryKey" json:"id"`
	UserID          string     `gorm:"type:varchar(100);not null;uniqueIndex:idx_dp_user_type" json:"user_id"`
	MemoryType      string     `gorm:"type:varchar(50);not null;uniqueIndex:idx_dp_user_type" json:"memory_type"`
	HalfLife        float64    `gorm:"type:float;not null;default:30.0" json:"half_life"`
	AccessCount30d  int        `gorm:"not null;default:0" json:"access_count_30d"`
	AvgIntervalDays float64    `gorm:"type:float;not null;default:0" json:"avg_interval_days"`
	FSRSStability   float64    `gorm:"type:float;not null;default:0" json:"fsrs_stability"`  // FSRS-6 稳定性 S
	FSRSDifficulty  float64    `gorm:"type:float;not null;default:0" json:"fsrs_difficulty"` // FSRS-6 难度 D
	FSRSParams      *string    `gorm:"type:text" json:"fsrs_params,omitempty"`               // FSRS-6 自定义参数 (JSON)
	UpdatedAt       *time.Time `gorm:"autoUpdateTime" json:"updated_at,omitempty"`
}

func (DecayProfile) TableName() string { return "decay_profiles" }

// CoreMemoryAuditLog 核心记忆审计日志模型。
// 记录 LLM 自动编辑核心记忆的完整 audit trail。
type CoreMemoryAuditLog struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    string    `gorm:"type:varchar(100);not null;index" json:"user_id"`
	Section   string    `gorm:"type:varchar(50);not null;index" json:"section"`
	Mode      string    `gorm:"type:varchar(20);not null" json:"mode"`
	Source    string    `gorm:"type:varchar(50);not null;index" json:"source"`
	OldValue  string    `gorm:"type:text;not null;default:''" json:"old_value"`
	NewValue  string    `gorm:"type:text;not null" json:"new_value"`
	EditedBy  string    `gorm:"type:varchar(50);not null;default:'llm'" json:"edited_by"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (CoreMemoryAuditLog) TableName() string { return "core_memory_audit_logs" }

func (c *CoreMemoryAuditLog) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// AutoMigrate runs GORM auto-migration for all models.
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Memory{},
		&ApiKey{},
		&UsageLog{},
		&BillingAccount{},
		&Transaction{},
		&Entity{},
		&Relation{},
		&MemoryEntityLink{},
		&MemoryTreeNode{},
		&NotificationPreference{},
		&NotificationLog{},
		&SystemConfig{},
		&CoreMemory{},
		&SchemaVersion{},
		&Agent{},
		&TenantMemoryConfig{},
		&DecayProfile{},
		&CoreMemoryAuditLog{},
	)
}
