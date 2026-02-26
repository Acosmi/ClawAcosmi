//go:build e2e
// +build e2e

package e2etest

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
)

// ============================================================================
// Mock FSStore — 简化版 VFS 文件存储
// ============================================================================

const vfsPrefix = "/vfs"

// MockFSStore 提供简化的 VFS 操作
type MockFSStore struct {
	agfs *MockAGFS
}

// NewMockFSStore 创建 Mock 文件存储
func NewMockFSStore() *MockFSStore {
	agfs := NewMockAGFS()
	agfs.Mkdir(vfsPrefix)
	return &MockFSStore{agfs: agfs}
}

func userRoot(tenant, user string) string {
	return fmt.Sprintf("%s/%s/%s", vfsPrefix, tenant, user)
}

// WriteMemory 写入一条记忆到 VFS (L0/L1/L2 三层)
func (s *MockFSStore) WriteMemory(tenant, user, memID, section, category, content, l0, l1 string) error {
	root := userRoot(tenant, user)
	dir := fmt.Sprintf("%s/%s/%s/%s", root, section, category, memID)

	s.agfs.Mkdir(root)
	s.agfs.Mkdir(fmt.Sprintf("%s/%s", root, section))
	s.agfs.Mkdir(fmt.Sprintf("%s/%s/%s", root, section, category))
	s.agfs.Mkdir(dir)

	if err := s.agfs.WriteFile(dir+"/l0.txt", []byte(l0)); err != nil {
		return err
	}
	if err := s.agfs.WriteFile(dir+"/l1.txt", []byte(l1)); err != nil {
		return err
	}
	if err := s.agfs.WriteFile(dir+"/l2.txt", []byte(content)); err != nil {
		return err
	}

	meta := map[string]interface{}{
		"memory_id":  memID,
		"section":    section,
		"category":   category,
		"created_at": memID, // 用 memID 作为时间戳占位
	}
	metaBytes, _ := json.Marshal(meta)
	return s.agfs.WriteFile(dir+"/meta.json", metaBytes)
}

// BatchReadL0 批量读取 L0 摘要
func (s *MockFSStore) BatchReadL0(tenant, user string, uris []string) ([]L0Entry, error) {
	if len(uris) == 0 {
		return nil, nil
	}
	results := make([]L0Entry, 0, len(uris))
	root := userRoot(tenant, user)

	for _, uri := range uris {
		l0Path := fmt.Sprintf("%s/%s/l0.txt", root, uri)
		l0Data, err := s.agfs.ReadFile(l0Path)
		if err != nil {
			continue
		}
		parts := strings.SplitN(uri, "/", 3)
		entry := L0Entry{URI: uri, L0Abstract: string(l0Data)}
		if len(parts) >= 3 {
			entry.MemoryType = parts[0]
			entry.Category = parts[1]
			entry.MemoryID = parts[2]
		}
		results = append(results, entry)
	}
	return results, nil
}

// BatchReadL1 批量读取 L1 概述
func (s *MockFSStore) BatchReadL1(tenant, user string, uris []string) ([]L1Entry, error) {
	if len(uris) == 0 {
		return nil, nil
	}
	results := make([]L1Entry, 0, len(uris))
	root := userRoot(tenant, user)

	for _, uri := range uris {
		l1Path := fmt.Sprintf("%s/%s/l1.txt", root, uri)
		l1Data, err := s.agfs.ReadFile(l1Path)
		if err != nil {
			continue
		}
		parts := strings.SplitN(uri, "/", 3)
		entry := L1Entry{URI: uri, L1Overview: string(l1Data)}
		if len(parts) >= 3 {
			entry.MemoryType = parts[0]
			entry.Category = parts[1]
			entry.MemoryID = parts[2]
		}
		results = append(results, entry)
	}
	return results, nil
}

// ReadMemory 按层级读取记忆内容
func (s *MockFSStore) ReadMemory(tenant, user, path string, level int) (string, error) {
	var filename string
	switch level {
	case 0:
		filename = "l0.txt"
	case 1:
		filename = "l1.txt"
	default:
		filename = "l2.txt"
	}
	fullPath := fmt.Sprintf("%s/%s/%s", userRoot(tenant, user), path, filename)
	data, err := s.agfs.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ============================================================================
// Mock LLM Provider
// ============================================================================

// MockLLM 可配置的 LLM Provider
type MockLLM struct {
	GenerateFunc func(ctx context.Context, prompt string) (string, error)
	CallCount    int
	mu           sync.Mutex
}

// NewMockLLM 创建默认 Mock LLM (返回固定响应)
func NewMockLLM(defaultResponse string) *MockLLM {
	return &MockLLM{
		GenerateFunc: func(_ context.Context, _ string) (string, error) {
			return defaultResponse, nil
		},
	}
}

// NewMockLLMWithError 创建会返回错误的 Mock LLM
func NewMockLLMWithError(err error) *MockLLM {
	return &MockLLM{
		GenerateFunc: func(_ context.Context, _ string) (string, error) {
			return "", err
		},
	}
}

func (m *MockLLM) Generate(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	m.CallCount++
	m.mu.Unlock()
	return m.GenerateFunc(ctx, prompt)
}

// ============================================================================
// Mock VectorStore — 内存向量存储
// ============================================================================

// vectorEntry 内存中的向量条目
type vectorEntry struct {
	MemoryID   string
	Content    string
	UserID     string
	MemoryType string
	Category   string
	Importance float64
	Embedding  []float32
}

// MockVectorStore 简化版内存向量存储
type MockVectorStore struct {
	mu      sync.Mutex
	entries map[string]*vectorEntry // key = memoryID
}

// NewMockVectorStore 创建内存向量存储
func NewMockVectorStore() *MockVectorStore {
	return &MockVectorStore{entries: make(map[string]*vectorEntry)}
}

// Upsert 插入或更新向量
func (vs *MockVectorStore) Upsert(memoryID, content, userID, memoryType, category string, importance float64) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.entries[memoryID] = &vectorEntry{
		MemoryID:   memoryID,
		Content:    content,
		UserID:     userID,
		MemoryType: memoryType,
		Category:   category,
		Importance: importance,
		Embedding:  simpleEmbed(content),
	}
}

// Search 简化的关键词搜索 (模拟向量搜索)
func (vs *MockVectorStore) Search(query, userID string, limit int, memoryTypes []string) []VectorSearchResult {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	queryEmb := simpleEmbed(query)
	var results []VectorSearchResult

	for _, e := range vs.entries {
		if e.UserID != userID {
			continue
		}
		if len(memoryTypes) > 0 && !contains(memoryTypes, e.MemoryType) {
			continue
		}
		score := cosineSim(queryEmb, e.Embedding)
		if score > 0.01 {
			results = append(results, VectorSearchResult{
				MemoryID:        e.MemoryID,
				Content:         e.Content,
				Score:           score,
				MemoryType:      e.MemoryType,
				UserID:          e.UserID,
				Category:        e.Category,
				ImportanceScore: e.Importance,
			})
		}
	}
	// 按 score 降序排序 (简化)
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// Delete 删除向量
func (vs *MockVectorStore) Delete(memoryID string) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	delete(vs.entries, memoryID)
}

// Count 返回条目数
func (vs *MockVectorStore) Count() int {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	return len(vs.entries)
}

// simpleEmbed 简单哈希嵌入 (非真实向量，仅用于测试相似度)
func simpleEmbed(text string) []float32 {
	const dim = 8
	emb := make([]float32, dim)
	lower := strings.ToLower(text)
	for i, r := range lower {
		emb[i%dim] += float32(r) * 0.01
	}
	// 归一化
	var norm float32
	for _, v := range emb {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range emb {
			emb[i] /= norm
		}
	}
	return emb
}

// cosineSim 计算余弦相似度
func cosineSim(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := float32(math.Sqrt(float64(normA * normB)))
	if denom == 0 {
		return 0
	}
	return float64(dot / denom)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ============================================================================
// 测试数据工厂
// ============================================================================

// TestMemory 测试用记忆数据
type TestMemory struct {
	MemoryID   string
	Content    string
	L0Abstract string
	L1Overview string
	MemoryType string
	Category   string
	UserID     string
}

// SeedTestMemories 返回预定义的多类型测试记忆
func SeedTestMemories() []TestMemory {
	return []TestMemory{
		{
			MemoryID: "mem-ep-001", MemoryType: MemoryTypeEpisodic, Category: CategoryPreference,
			Content:    "用户喜欢喝拿铁咖啡，每天早上都会去星巴克",
			L0Abstract: "咖啡偏好：拿铁", L1Overview: "用户偏好：每天早上喝星巴克拿铁",
			UserID: "user-1",
		},
		{
			MemoryID: "mem-ep-002", MemoryType: MemoryTypeEpisodic, Category: CategoryEvent,
			Content:    "今天去了上海博物馆，看了青铜器展览",
			L0Abstract: "上海博物馆参观", L1Overview: "参观了上海博物馆的青铜器展览",
			UserID: "user-1",
		},
		{
			MemoryID: "mem-sem-001", MemoryType: MemoryTypeSemantic, Category: CategorySkill,
			Content:    "Rust 的所有权系统通过编译期检查消除内存安全问题",
			L0Abstract: "Rust 所有权系统", L1Overview: "Rust 通过所有权和借用检查实现编译期内存安全",
			UserID: "user-1",
		},
		{
			MemoryID: "mem-perm-001", MemoryType: MemoryTypePermanent, Category: CategoryFact,
			Content:    "用户是全栈工程师，精通 Go 和 Next.js",
			L0Abstract: "职业：全栈工程师", L1Overview: "用户为全栈工程师，主要技术栈为 Go 和 Next.js",
			UserID: "user-1",
		},
		{
			MemoryID: "mem-imag-001", MemoryType: MemoryTypeImagination, Category: CategoryInsight,
			Content:    "基于用户对 Rust 的持续关注，预测用户可能会将项目核心模块迁移到 Rust",
			L0Abstract: "预测：Rust 迁移趋势", L1Overview: "用户可能将核心模块迁移到 Rust",
			UserID: "user-1",
		},
	}
}

// SeedToFSStore 将测试数据写入 MockFSStore
func SeedToFSStore(store *MockFSStore, tenant string, memories []TestMemory) {
	for _, m := range memories {
		_ = store.WriteMemory(tenant, m.UserID, m.MemoryID, m.MemoryType, m.Category,
			m.Content, m.L0Abstract, m.L1Overview)
	}
}

// SeedToVectorStore 将测试数据写入 MockVectorStore
func SeedToVectorStore(vs *MockVectorStore, memories []TestMemory) {
	for _, m := range memories {
		vs.Upsert(m.MemoryID, m.Content, m.UserID, m.MemoryType, m.Category, 0.7)
	}
}

// ============================================================================
// Mock SegmentStore — 镜像 ffi.SegmentStore (多集合向量存储)
// ============================================================================

type segPoint struct {
	vector  []float32
	payload map[string]interface{}
}

type segCollection struct {
	dim    int
	points map[string]*segPoint
}

// MockSegmentStore 模拟 Rust FFI SegmentStore
type MockSegmentStore struct {
	mu          sync.Mutex
	collections map[string]*segCollection
}

// SegmentSearchHit 模拟 FFI 搜索结果
type SegmentSearchHit struct {
	ID      string                 `json:"id"`
	Score   float32                `json:"score"`
	Payload map[string]interface{} `json:"payload"`
}

// NewMockSegmentStore 创建 Mock FFI SegmentStore
func NewMockSegmentStore() *MockSegmentStore {
	return &MockSegmentStore{collections: make(map[string]*segCollection)}
}

func (s *MockSegmentStore) CreateCollection(name string, dim int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.collections[name]; ok {
		return nil // already exists
	}
	s.collections[name] = &segCollection{dim: dim, points: make(map[string]*segPoint)}
	return nil
}

func (s *MockSegmentStore) Upsert(collection, pointID string, denseVec []float32, payloadJSON []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	col, ok := s.collections[collection]
	if !ok {
		return fmt.Errorf("collection not found: %s", collection)
	}
	var payload map[string]interface{}
	if len(payloadJSON) > 0 {
		if err := json.Unmarshal(payloadJSON, &payload); err != nil {
			return fmt.Errorf("payload parse: %w", err)
		}
	}
	vec := make([]float32, len(denseVec))
	copy(vec, denseVec)
	col.points[pointID] = &segPoint{vector: vec, payload: payload}
	return nil
}

func (s *MockSegmentStore) Search(collection string, queryVec []float32, limit int) ([]SegmentSearchHit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	col, ok := s.collections[collection]
	if !ok {
		return nil, fmt.Errorf("collection not found: %s", collection)
	}
	type scored struct {
		id    string
		score float32
		pt    *segPoint
	}
	var results []scored
	for id, pt := range col.points {
		sim := segCosine(queryVec, pt.vector)
		results = append(results, scored{id: id, score: sim, pt: pt})
	}
	// Sort descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	hits := make([]SegmentSearchHit, len(results))
	for i, r := range results {
		hits[i] = SegmentSearchHit{ID: r.id, Score: r.score, Payload: r.pt.payload}
	}
	return hits, nil
}

func (s *MockSegmentStore) Delete(collection, pointID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	col, ok := s.collections[collection]
	if !ok {
		return fmt.Errorf("collection not found: %s", collection)
	}
	delete(col.points, pointID)
	return nil
}

func (s *MockSegmentStore) Flush(collection string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.collections[collection]; !ok {
		return fmt.Errorf("collection not found: %s", collection)
	}
	return nil
}

func (s *MockSegmentStore) Count(collection string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	col, ok := s.collections[collection]
	if !ok {
		return 0
	}
	return len(col.points)
}

// --- VFS Semantic 索引 (复用 MockSegmentStore) ---

const vfsSemanticCol = "vfs_semantic"

func (s *MockSegmentStore) VFSSemanticInit(dim int) error {
	return s.CreateCollection(vfsSemanticCol, dim)
}

func (s *MockSegmentStore) VFSSemanticIndex(pointID string, denseVec []float32, payloadJSON []byte) error {
	return s.Upsert(vfsSemanticCol, pointID, denseVec, payloadJSON)
}

func (s *MockSegmentStore) VFSSemanticSearch(queryVec []float32, limit int) ([]SegmentSearchHit, error) {
	return s.Search(vfsSemanticCol, queryVec, limit)
}

func (s *MockSegmentStore) VFSSemanticDelete(pointID string) error {
	return s.Delete(vfsSemanticCol, pointID)
}

func segCosine(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return float32(dot / denom)
}

// ============================================================================
// Mock MemFS — 镜像 ffi.MemFS (VFS 记忆文件系统 + Trace)
// ============================================================================

// MemFSTraceStep 下探轨迹步骤
type MemFSTraceStep struct {
	Path             string  `json:"path"`
	NodeType         string  `json:"node_type"`
	Score            float64 `json:"score"`
	ChildrenExplored int     `json:"children_explored"`
	Matched          bool    `json:"matched"`
}

// MemFSSearchTrace 搜索轨迹
type MemFSSearchTrace struct {
	Query            string           `json:"query"`
	Keywords         []string         `json:"keywords"`
	Steps            []MemFSTraceStep `json:"steps"`
	TotalDirsVisited int              `json:"total_dirs_visited"`
	TotalFilesScored int              `json:"total_files_scored"`
	Hits             []MemFSSearchHit `json:"hits"`
}

// MemFSSearchHit 搜索命中
type MemFSSearchHit struct {
	Path       string  `json:"path"`
	MemoryID   string  `json:"memory_id"`
	Category   string  `json:"category"`
	Score      float64 `json:"score"`
	L0Abstract string  `json:"l0_abstract"`
}

type memFSEntry struct {
	memoryID   string
	category   string
	content    string
	l0Abstract string
	l1Overview string
}

// MockMemFS 内存文件系统 (镜像 Rust MemFS)
type MockMemFS struct {
	mu      sync.Mutex
	entries map[string]map[string][]memFSEntry // [tenant/user] -> entries
}

// NewMockMemFS 创建 Mock MemFS
func NewMockMemFS() *MockMemFS {
	return &MockMemFS{entries: make(map[string]map[string][]memFSEntry)}
}

func memFSKey(tenant, user string) string { return tenant + "/" + user }

func (m *MockMemFS) WriteMemory(tenant, user, memoryID, category, content, l0, l1 string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memFSKey(tenant, user)
	if m.entries[key] == nil {
		m.entries[key] = make(map[string][]memFSEntry)
	}
	m.entries[key][memoryID] = append(m.entries[key][memoryID][:0], memFSEntry{
		memoryID: memoryID, category: category,
		content: content, l0Abstract: l0, l1Overview: l1,
	})
	return nil
}

func (m *MockMemFS) Read(tenant, user, memoryID string, level int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memFSKey(tenant, user)
	entries, ok := m.entries[key]
	if !ok {
		return "", fmt.Errorf("tenant/user not found: %s", key)
	}
	arr, ok := entries[memoryID]
	if !ok || len(arr) == 0 {
		return "", fmt.Errorf("memory not found: %s", memoryID)
	}
	e := arr[0]
	switch level {
	case 0:
		return e.l0Abstract, nil
	case 1:
		return e.l1Overview, nil
	default:
		return e.content, nil
	}
}

func (m *MockMemFS) Delete(tenant, user, memoryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memFSKey(tenant, user)
	if entries, ok := m.entries[key]; ok {
		delete(entries, memoryID)
	}
	return nil
}

func (m *MockMemFS) MemoryCount(tenant, user string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memFSKey(tenant, user)
	return len(m.entries[key])
}

func (m *MockMemFS) Search(tenant, user, query string, maxResults int) []MemFSSearchHit {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := memFSKey(tenant, user)
	entries, ok := m.entries[key]
	if !ok {
		return nil
	}
	var hits []MemFSSearchHit
	queryLower := strings.ToLower(query)
	for _, arr := range entries {
		for _, e := range arr {
			if strings.Contains(strings.ToLower(e.content), queryLower) ||
				strings.Contains(strings.ToLower(e.l0Abstract), queryLower) {
				hits = append(hits, MemFSSearchHit{
					Path: e.category + "/" + e.memoryID, MemoryID: e.memoryID,
					Category: e.category, Score: 0.8, L0Abstract: e.l0Abstract,
				})
			}
		}
	}
	if len(hits) > maxResults {
		hits = hits[:maxResults]
	}
	return hits
}

func (m *MockMemFS) SearchWithTrace(tenant, user, query string, maxResults int) *MemFSSearchTrace {
	hits := m.Search(tenant, user, query, maxResults)
	// 构造下探轨迹
	m.mu.Lock()
	key := memFSKey(tenant, user)
	totalFiles := len(m.entries[key])
	m.mu.Unlock()

	keywords := strings.Fields(query)
	var steps []MemFSTraceStep
	steps = append(steps, MemFSTraceStep{
		Path: "/", NodeType: "directory", Score: 0, ChildrenExplored: totalFiles, Matched: false,
	})
	for _, h := range hits {
		steps = append(steps, MemFSTraceStep{
			Path: h.Path, NodeType: "file", Score: h.Score, ChildrenExplored: 0, Matched: true,
		})
	}
	return &MemFSSearchTrace{
		Query:            query,
		Keywords:         keywords,
		Steps:            steps,
		TotalDirsVisited: 1,
		TotalFilesScored: totalFiles,
		Hits:             hits,
	}
}

// ============================================================================
// Mock AGFS Queue — FIFO 队列 (镜像 AGFS queuefs)
// ============================================================================

// MockAGFSQueue 内存 FIFO 队列
type MockAGFSQueue struct {
	mu   sync.Mutex
	data [][]byte
}

// NewMockAGFSQueue 创建 Mock 队列
func NewMockAGFSQueue() *MockAGFSQueue {
	return &MockAGFSQueue{}
}

func (q *MockAGFSQueue) Enqueue(data []byte) {
	q.mu.Lock()
	defer q.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	q.data = append(q.data, cp)
}

func (q *MockAGFSQueue) EnqueueJSON(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	q.Enqueue(b)
	return nil
}

func (q *MockAGFSQueue) Dequeue() ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.data) == 0 {
		return nil, false
	}
	item := q.data[0]
	q.data = q.data[1:]
	return item, true
}

func (q *MockAGFSQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.data)
}

// Snapshot 返回当前队列数据的快照 (模拟持久化)
func (q *MockAGFSQueue) Snapshot() [][]byte {
	q.mu.Lock()
	defer q.mu.Unlock()
	snap := make([][]byte, len(q.data))
	for i, d := range q.data {
		cp := make([]byte, len(d))
		copy(cp, d)
		snap[i] = cp
	}
	return snap
}

// Restore 从快照恢复 (模拟 AGFS 重启)
func (q *MockAGFSQueue) Restore(snap [][]byte) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.data = make([][]byte, len(snap))
	for i, d := range snap {
		cp := make([]byte, len(d))
		copy(cp, d)
		q.data[i] = cp
	}
}

// ============================================================================
// Mock AGFS KV — 键值存储 (镜像 AGFS kvfs)
// ============================================================================

// MockAGFSKV 内存 KV 存储
type MockAGFSKV struct {
	mu   sync.Mutex
	data map[string][]byte
}

// NewMockAGFSKV 创建 Mock KV 存储
func NewMockAGFSKV() *MockAGFSKV {
	return &MockAGFSKV{data: make(map[string][]byte)}
}

func (kv *MockAGFSKV) Set(key string, value []byte) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	cp := make([]byte, len(value))
	copy(cp, value)
	kv.data[key] = cp
}

func (kv *MockAGFSKV) Get(key string) ([]byte, bool) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	v, ok := kv.data[key]
	if !ok {
		return nil, false
	}
	cp := make([]byte, len(v))
	copy(cp, v)
	return cp, true
}

func (kv *MockAGFSKV) SetJSON(key string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	kv.Set(key, b)
	return nil
}

func (kv *MockAGFSKV) GetJSON(key string, out interface{}) error {
	b, ok := kv.Get(key)
	if !ok {
		return fmt.Errorf("key not found: %s", key)
	}
	return json.Unmarshal(b, out)
}

func (kv *MockAGFSKV) Delete(key string) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	delete(kv.data, key)
}

// ============================================================================
// Mock AGFS Lock — 分布式锁 (镜像 VFSPathLock)
// ============================================================================

// MockAGFSLock 内存分布式锁
type MockAGFSLock struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

// NewMockAGFSLock 创建 Mock 分布式锁
func NewMockAGFSLock() *MockAGFSLock {
	return &MockAGFSLock{locks: make(map[string]*sync.Mutex)}
}

// ForUser 获取指定租户+用户的锁
func (l *MockAGFSLock) ForUser(tenantID, userID string) *sync.Mutex {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := tenantID + "/" + userID
	m, ok := l.locks[key]
	if !ok {
		m = &sync.Mutex{}
		l.locks[key] = m
	}
	return m
}
