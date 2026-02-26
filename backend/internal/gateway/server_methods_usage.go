package gateway

// server_methods_usage.go — sessions.usage.* 方法处理器
// 对应 TS: src/gateway/server-methods/usage.ts (822L)
//
// 完整实现：session discovery + cost aggregation + 多维度聚合。
// 隐藏依赖 #2: costUsageCache 模块级 Map + TTL 30s

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/anthropic/open-acosmi/internal/agents/models"
	"github.com/anthropic/open-acosmi/internal/agents/scope"
	"github.com/anthropic/open-acosmi/internal/sessions"
)

// UsageHandlers 返回 sessions.usage.* 方法处理器映射。
func UsageHandlers() map[string]GatewayMethodHandler {
	return map[string]GatewayMethodHandler{
		"sessions.usage":            handleSessionsUsage,
		"sessions.usage.timeseries": handleSessionsUsageTimeseries,
		"sessions.usage.logs":       handleSessionsUsageLogs,
		"usage.status":              handleUsageStatus,
		"usage.cost":                handleUsageCost,
	}
}

// ---------- 类型定义 ----------

type usageDateRange struct {
	startMs int64
	endMs   int64
}

type usageTotals struct {
	Input              int     `json:"input"`
	Output             int     `json:"output"`
	CacheRead          int     `json:"cacheRead"`
	CacheWrite         int     `json:"cacheWrite"`
	TotalTokens        int     `json:"totalTokens"`
	TotalCost          float64 `json:"totalCost"`
	InputCost          float64 `json:"inputCost"`
	OutputCost         float64 `json:"outputCost"`
	CacheReadCost      float64 `json:"cacheReadCost"`
	CacheWriteCost     float64 `json:"cacheWriteCost"`
	MissingCostEntries int     `json:"missingCostEntries"`
}

type sessionCostSummary struct {
	usageTotals
	MessageCounts      *messageCounts          `json:"messageCounts,omitempty"`
	ByModel            map[string]*usageTotals `json:"-"`                       // provider::model → totals
	ToolNames          map[string]int          `json:"-"`                       // toolName → callCount
	FirstActivity      int64                   `json:"firstActivity,omitempty"` // Unix ms
	LastActivity       int64                   `json:"lastActivity,omitempty"`  // Unix ms
	DurationMs         int64                   `json:"durationMs,omitempty"`
	ActivityDates      []string                `json:"activityDates,omitempty"` // ["2026-02-26"]
	DailyBreakdown     []dailyUsageEntry       `json:"dailyBreakdown,omitempty"`
	DailyMessageCounts []dailyMsgCountEntry    `json:"dailyMessageCounts,omitempty"`
	DailyModelUsage    []dailyModelEntry       `json:"dailyModelUsage,omitempty"`
	DailyLatency       []dailyLatencyEntry     `json:"dailyLatency,omitempty"`
	rawLatencyData     map[string][]float64    `json:"-"` // 全局聚合用原始数据
}

type dailyUsageEntry struct {
	Date   string  `json:"date"` // "2026-02-26"
	Tokens int     `json:"tokens"`
	Cost   float64 `json:"cost"`
}

type dailyMsgCountEntry struct {
	Date        string `json:"date"`
	Total       int    `json:"total"`
	User        int    `json:"user"`
	Assistant   int    `json:"assistant"`
	ToolCalls   int    `json:"toolCalls"`
	ToolResults int    `json:"toolResults"`
	Errors      int    `json:"errors"`
}

type messageCounts struct {
	Total       int `json:"total"`
	User        int `json:"user"`
	Assistant   int `json:"assistant"`
	ToolCalls   int `json:"toolCalls"`
	ToolResults int `json:"toolResults"`
	Errors      int `json:"errors"`
}

type dailyModelEntry struct {
	Date     string  `json:"date"`
	Provider string  `json:"provider,omitempty"`
	Model    string  `json:"model"`
	Tokens   int     `json:"tokens"`
	Cost     float64 `json:"cost"`
	Count    int     `json:"count"`
}

type dailyLatencyEntry struct {
	Date  string  `json:"date"`
	Count int     `json:"count"`
	AvgMs float64 `json:"avgMs"`
	P95Ms float64 `json:"p95Ms"`
	MinMs float64 `json:"minMs"`
	MaxMs float64 `json:"maxMs"`
}

// ---------- 缓存 (隐依赖 #2: costUsageCache) ----------

const costUsageCacheTTL = 30 * time.Second

type costUsageCacheEntry struct {
	totals    *usageTotals
	updatedAt time.Time
}

var (
	costCacheMu sync.RWMutex
	costCache   = map[string]*costUsageCacheEntry{}
)

// ---------- 日期解析 ----------

var dateRe = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)

func parseDateToMs(raw interface{}) (int64, bool) {
	s, ok := raw.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return 0, false
	}
	m := dateRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, false
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	return t.UnixMilli(), true
}

func parseDays(raw interface{}) (int, bool) {
	switch v := raw.(type) {
	case float64:
		if !math.IsNaN(v) && !math.IsInf(v, 0) {
			return int(v), true
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n, true
		}
	}
	return 0, false
}

func parseDateRange(params map[string]interface{}) usageDateRange {
	now := time.Now().UTC()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	todayEndMs := todayStart.Add(24*time.Hour).UnixMilli() - 1

	startMs, startOK := parseDateToMs(params["startDate"])
	endMs, endOK := parseDateToMs(params["endDate"])
	if startOK && endOK {
		return usageDateRange{startMs: startMs, endMs: endMs + 24*60*60*1000 - 1}
	}

	if days, ok := parseDays(params["days"]); ok {
		if days < 1 {
			days = 1
		}
		start := todayStart.AddDate(0, 0, -(days - 1)).UnixMilli()
		return usageDateRange{startMs: start, endMs: todayEndMs}
	}

	// 默认 30 天
	defaultStart := todayStart.AddDate(0, 0, -29).UnixMilli()
	return usageDateRange{startMs: defaultStart, endMs: todayEndMs}
}

func formatDateStr(ms int64) string {
	t := time.UnixMilli(ms).UTC()
	return t.Format("2006-01-02")
}

// ---------- Session Discovery ----------

type discoveredSession struct {
	sessionID   string
	sessionFile string
	agentID     string
	mtime       int64 // UnixMilli
}

// discoverSessionsForAgent 扫描指定 agent 的 sessions 目录。
func discoverSessionsForAgent(agentID string, startMs, endMs int64) []discoveredSession {
	dir := sessions.ResolveAgentSessionsDir(agentID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var results []discoveredSession
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		mtimeMs := info.ModTime().UnixMilli()
		// 时间范围过滤
		if mtimeMs < startMs || mtimeMs > endMs {
			continue
		}
		sid := strings.TrimSuffix(e.Name(), ".jsonl")
		// 跳过 topic 文件
		if strings.Contains(sid, "-topic-") {
			continue
		}
		results = append(results, discoveredSession{
			sessionID:   sid,
			sessionFile: filepath.Join(dir, e.Name()),
			agentID:     agentID,
			mtime:       mtimeMs,
		})
	}
	return results
}

// discoverAllSessionsForUsage 遍历所有 agent 发现会话。
// 始终包含 routing.DefaultAgentID ("main")，确保文件系统默认路径不被遗漏。
func discoverAllSessionsForUsage(cfg interface{ GetAgentIds() []string }, startMs, endMs int64, agentIds []string) []discoveredSession {
	// 确保 routing.DefaultAgentID (main) 始终在扫描列表中
	// （scope.DefaultAgentID 是 "default"，但文件实际存在 "main" 目录下）
	seen := make(map[string]bool, len(agentIds)+1)
	var ids []string
	for _, id := range agentIds {
		normalized := strings.ToLower(strings.TrimSpace(id))
		if normalized != "" && !seen[normalized] {
			seen[normalized] = true
			ids = append(ids, id)
		}
	}
	const routingDefault = "main" // routing.DefaultAgentID
	if !seen[routingDefault] {
		ids = append(ids, routingDefault)
	}

	var all []discoveredSession
	for _, id := range ids {
		all = append(all, discoverSessionsForAgent(id, startMs, endMs)...)
	}
	// 按 mtime 降序
	sort.Slice(all, func(i, j int) bool { return all[i].mtime > all[j].mtime })
	return all
}

// ---------- Session Cost 从 JSONL 聚合 ----------

// loadSessionCostFromFile 从 JSONL 转录文件聚合 token 和消息统计。
func loadSessionCostFromFile(sessionFile string) *sessionCostSummary {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return nil
	}

	counts := &messageCounts{}
	totals := usageTotals{}
	byModel := map[string]*usageTotals{}
	toolNames := map[string]int{}
	var firstTs, lastTs int64
	dateSet := map[string]struct{}{}
	dailyMap := map[string]*dailyUsageEntry{}
	dailyMsgMap := map[string]*dailyMsgCountEntry{}
	dailyModelMap := map[string]*dailyModelEntry{} // key: date::provider::model
	latencyByDate := map[string][]float64{}        // date → latency values in ms
	var lastUserTs int64                           // 延迟计算: user→assistant 时间差
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		role, _ := entry["role"].(string)
		switch role {
		case "user":
			counts.Total++
			counts.User++
		case "assistant":
			counts.Total++
			counts.Assistant++
		}

		// 提取 timestamp (提前到 role 计数后、usage 提取前，供后续按日聚合使用)
		var msgTs int64
		if ts, ok := entry["timestamp"].(float64); ok && ts > 0 {
			msgTs = int64(ts) // Unix ms
			if firstTs == 0 || msgTs < firstTs {
				firstTs = msgTs
			}
			if msgTs > lastTs {
				lastTs = msgTs
			}
			dateStr := time.UnixMilli(msgTs).Format("2006-01-02")
			dateSet[dateStr] = struct{}{}
		}

		// 延迟近似计算 (D-03): user ts → first assistant ts
		if role == "user" && msgTs > 0 {
			lastUserTs = msgTs
		} else if role == "assistant" && msgTs > 0 && lastUserTs > 0 {
			latencyMs := float64(msgTs - lastUserTs)
			if latencyMs > 0 && latencyMs < 600_000 { // <10 分钟为有效延迟
				dateStr := time.UnixMilli(msgTs).Format("2006-01-02")
				latencyByDate[dateStr] = append(latencyByDate[dateStr], latencyMs)
			}
			lastUserTs = 0 // 只取 user 后第一条 assistant
		}

		// 提取 model / provider
		model, _ := entry["model"].(string)
		provider, _ := entry["provider"].(string)
		modelKey := ""
		if model != "" {
			if provider != "" {
				modelKey = provider + "::" + model
			} else {
				modelKey = model
			}
		}

		// 提取 usage
		var msgInput, msgOutput, msgCacheRead, msgCacheWrite int
		if u, ok := entry["usage"].(map[string]interface{}); ok {
			if v, ok := u["input_tokens"].(float64); ok {
				msgInput = int(v)
				totals.Input += msgInput
				totals.TotalTokens += msgInput
			}
			if v, ok := u["output_tokens"].(float64); ok {
				msgOutput = int(v)
				totals.Output += msgOutput
				totals.TotalTokens += msgOutput
			}
			if v, ok := u["cache_read_input_tokens"].(float64); ok {
				msgCacheRead = int(v)
				totals.CacheRead += msgCacheRead
			}
			if v, ok := u["cache_creation_input_tokens"].(float64); ok {
				msgCacheWrite = int(v)
				totals.CacheWrite += msgCacheWrite
			}
		}

		// 计算 cost (D-02)
		var msgCost, msgInputCost, msgOutputCost, msgCacheReadCost, msgCacheWriteCost float64
		if model != "" && (msgInput > 0 || msgOutput > 0 || msgCacheRead > 0 || msgCacheWrite > 0) {
			if mc := models.LookupZenModelCost(model); mc != nil {
				msgInputCost = float64(msgInput) * mc.Input / 1_000_000
				msgOutputCost = float64(msgOutput) * mc.Output / 1_000_000
				msgCacheReadCost = float64(msgCacheRead) * mc.CacheRead / 1_000_000
				msgCacheWriteCost = float64(msgCacheWrite) * mc.CacheWrite / 1_000_000
				msgCost = msgInputCost + msgOutputCost + msgCacheReadCost + msgCacheWriteCost
				totals.InputCost += msgInputCost
				totals.OutputCost += msgOutputCost
				totals.CacheReadCost += msgCacheReadCost
				totals.CacheWriteCost += msgCacheWriteCost
				totals.TotalCost += msgCost
			} else {
				totals.MissingCostEntries++
			}
		}

		// 按 model 聚合 (含 cache + cost)
		if modelKey != "" && (msgInput > 0 || msgOutput > 0) {
			mt := byModel[modelKey]
			if mt == nil {
				mt = &usageTotals{}
				byModel[modelKey] = mt
			}
			mt.Input += msgInput
			mt.Output += msgOutput
			mt.CacheRead += msgCacheRead
			mt.CacheWrite += msgCacheWrite
			mt.TotalTokens += msgInput + msgOutput
			mt.InputCost += msgInputCost
			mt.OutputCost += msgOutputCost
			mt.CacheReadCost += msgCacheReadCost
			mt.CacheWriteCost += msgCacheWriteCost
			mt.TotalCost += msgCost
		}

		// 按日聚合 token + cost
		if msgTs > 0 && (msgInput > 0 || msgOutput > 0) {
			dateStr := time.UnixMilli(msgTs).Format("2006-01-02")
			dt := dailyMap[dateStr]
			if dt == nil {
				dt = &dailyUsageEntry{Date: dateStr}
				dailyMap[dateStr] = dt
			}
			dt.Tokens += msgInput + msgOutput
			dt.Cost += msgCost
		}

		// 按日+模型聚合 (D-01)
		if msgTs > 0 && modelKey != "" && (msgInput > 0 || msgOutput > 0) {
			dateStr := time.UnixMilli(msgTs).Format("2006-01-02")
			dmKey := dateStr + "::" + modelKey
			dm := dailyModelMap[dmKey]
			if dm == nil {
				prov, mod := "", model
				if parts := strings.SplitN(modelKey, "::", 2); len(parts) == 2 {
					prov = parts[0]
					mod = parts[1]
				}
				dm = &dailyModelEntry{Date: dateStr, Provider: prov, Model: mod}
				dailyModelMap[dmKey] = dm
			}
			dm.Tokens += msgInput + msgOutput
			dm.Cost += msgCost
			dm.Count++
		}

		// 工具调用计数
		var msgToolCalls, msgToolResults, msgErrors int
		if content, ok := entry["content"].([]interface{}); ok {
			for _, block := range content {
				if bm, ok := block.(map[string]interface{}); ok {
					if tp, ok := bm["type"].(string); ok {
						if tp == "tool_use" {
							counts.ToolCalls++
							msgToolCalls++
							if tn, ok := bm["name"].(string); ok && tn != "" {
								toolNames[tn]++
							}
						} else if tp == "tool_result" {
							counts.ToolResults++
							msgToolResults++
							if isErr, _ := bm["is_error"].(bool); isErr {
								counts.Errors++
								msgErrors++
							}
						}
					}
				}
			}
		}

		// 按日聚合消息计数
		if msgTs > 0 && role != "" {
			dateStr := time.UnixMilli(msgTs).Format("2006-01-02")
			dm := dailyMsgMap[dateStr]
			if dm == nil {
				dm = &dailyMsgCountEntry{Date: dateStr}
				dailyMsgMap[dateStr] = dm
			}
			switch role {
			case "user":
				dm.Total++
				dm.User++
			case "assistant":
				dm.Total++
				dm.Assistant++
			}
			dm.ToolCalls += msgToolCalls
			dm.ToolResults += msgToolResults
			dm.Errors += msgErrors
		}
	}

	// 构建 activityDates (排序)
	activityDates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		activityDates = append(activityDates, d)
	}
	sort.Strings(activityDates)

	// 构建 dailyBreakdown (排序)
	dailyBreakdown := make([]dailyUsageEntry, 0, len(dailyMap))
	for _, dt := range dailyMap {
		dailyBreakdown = append(dailyBreakdown, *dt)
	}
	sort.Slice(dailyBreakdown, func(i, j int) bool {
		return dailyBreakdown[i].Date < dailyBreakdown[j].Date
	})

	// 构建 dailyMessageCounts (排序)
	dailyMsgCounts := make([]dailyMsgCountEntry, 0, len(dailyMsgMap))
	for _, dm := range dailyMsgMap {
		dailyMsgCounts = append(dailyMsgCounts, *dm)
	}
	sort.Slice(dailyMsgCounts, func(i, j int) bool {
		return dailyMsgCounts[i].Date < dailyMsgCounts[j].Date
	})

	// 构建 dailyModelUsage (D-01: 按 date 然后 model 排序)
	dailyModelUsage := make([]dailyModelEntry, 0, len(dailyModelMap))
	for _, dm := range dailyModelMap {
		dailyModelUsage = append(dailyModelUsage, *dm)
	}
	sort.Slice(dailyModelUsage, func(i, j int) bool {
		if dailyModelUsage[i].Date != dailyModelUsage[j].Date {
			return dailyModelUsage[i].Date < dailyModelUsage[j].Date
		}
		return dailyModelUsage[i].Model < dailyModelUsage[j].Model
	})

	// 构建 dailyLatency (D-03)
	dailyLatency := make([]dailyLatencyEntry, 0, len(latencyByDate))
	for dateStr, vals := range latencyByDate {
		if len(vals) == 0 {
			continue
		}
		sort.Float64s(vals)
		n := len(vals)
		sum := 0.0
		for _, v := range vals {
			sum += v
		}
		p95Idx := int(math.Ceil(0.95*float64(n))) - 1
		if p95Idx < 0 {
			p95Idx = 0
		}
		if p95Idx >= n {
			p95Idx = n - 1
		}
		dailyLatency = append(dailyLatency, dailyLatencyEntry{
			Date:  dateStr,
			Count: n,
			AvgMs: math.Round(sum/float64(n)*100) / 100,
			P95Ms: vals[p95Idx],
			MinMs: vals[0],
			MaxMs: vals[n-1],
		})
	}
	sort.Slice(dailyLatency, func(i, j int) bool {
		return dailyLatency[i].Date < dailyLatency[j].Date
	})

	var durationMs int64
	if lastTs > firstTs {
		durationMs = lastTs - firstTs
	}

	return &sessionCostSummary{
		usageTotals:        totals,
		MessageCounts:      counts,
		ByModel:            byModel,
		ToolNames:          toolNames,
		FirstActivity:      firstTs,
		LastActivity:       lastTs,
		DurationMs:         durationMs,
		ActivityDates:      activityDates,
		DailyBreakdown:     dailyBreakdown,
		DailyMessageCounts: dailyMsgCounts,
		DailyModelUsage:    dailyModelUsage,
		DailyLatency:       dailyLatency,
		rawLatencyData:     latencyByDate,
	}
}

// ---------- usage.status ----------

func handleUsageStatus(ctx *MethodHandlerContext) {
	ctx.Respond(true, map[string]interface{}{
		"ok":      true,
		"message": "usage status not yet fully implemented",
	}, nil)
}

// ---------- usage.cost ----------

func handleUsageCost(ctx *MethodHandlerContext) {
	dr := parseDateRange(ctx.Params)
	cacheKey := fmt.Sprintf("%d-%d", dr.startMs, dr.endMs)

	// 检查缓存
	costCacheMu.RLock()
	cached := costCache[cacheKey]
	costCacheMu.RUnlock()

	if cached != nil && time.Since(cached.updatedAt) < costUsageCacheTTL {
		ctx.Respond(true, cached.totals, nil)
		return
	}

	// 重新计算
	cfg := resolveConfigFromContext(ctx)
	agentIds := []string{"default"}
	if cfg != nil {
		agentIds = scope.ListAgentIds(cfg)
	}

	discovered := discoverAllSessionsForUsage(nil, dr.startMs, dr.endMs, agentIds)
	totals := &usageTotals{}
	for _, d := range discovered {
		cost := loadSessionCostFromFile(d.sessionFile)
		if cost != nil {
			mergeTotalsInto(totals, &cost.usageTotals)
		}
	}

	costCacheMu.Lock()
	costCache[cacheKey] = &costUsageCacheEntry{totals: totals, updatedAt: time.Now()}
	costCacheMu.Unlock()

	ctx.Respond(true, totals, nil)
}

// ---------- sessions.usage ----------

func handleSessionsUsage(ctx *MethodHandlerContext) {
	cfg := resolveConfigFromContext(ctx)
	if cfg == nil {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeInternalError, "config not available"))
		return
	}

	dr := parseDateRange(ctx.Params)
	limit := 50
	if v, ok := ctx.Params["limit"].(float64); ok && !math.IsNaN(v) {
		limit = int(v)
	}

	agentIds := scope.ListAgentIds(cfg)
	defaultAgentId := scope.ResolveDefaultAgentId(cfg)

	// 加载 session store
	store := ctx.Context.SessionStore
	var storeEntries map[string]*SessionEntry
	if store != nil {
		storeEntries = store.LoadCombinedStore(defaultAgentId)
	}

	// Session discovery + store 合并
	type mergedEntry struct {
		key         string
		sessionID   string
		sessionFile string
		label       string
		updatedAt   int64
		agentID     string
		storeEntry  *SessionEntry
	}

	var merged []mergedEntry

	// 检查是否请求特定 key
	specificKey := ""
	if v, ok := ctx.Params["key"].(string); ok {
		specificKey = strings.TrimSpace(v)
	}

	if specificKey != "" {
		// 单 key 查询
		parsed := scope.ParseAgentSessionKey(specificKey)
		var agentID, rawSessionID string
		if parsed != nil {
			agentID = parsed.AgentID
			rawSessionID = parsed.Rest
		} else {
			rawSessionID = specificKey
		}

		var storeEntry *SessionEntry
		if storeEntries != nil {
			storeEntry = storeEntries[specificKey]
		}

		sessionID := rawSessionID
		if storeEntry != nil && storeEntry.SessionId != "" {
			sessionID = storeEntry.SessionId
		}

		sessionFile := sessions.ResolveSessionFilePath(sessionID, nil, agentID)
		if _, err := os.Stat(sessionFile); err == nil {
			label := ""
			updAt := time.Now().UnixMilli()
			if storeEntry != nil {
				label = storeEntry.Label
				if storeEntry.UpdatedAt > 0 {
					updAt = storeEntry.UpdatedAt
				}
			}
			merged = append(merged, mergedEntry{
				key: specificKey, sessionID: sessionID, sessionFile: sessionFile,
				label: label, updatedAt: updAt, agentID: agentID, storeEntry: storeEntry,
			})
		}
	} else {
		// 全量 discovery
		discovered := discoverAllSessionsForUsage(nil, dr.startMs, dr.endMs, agentIds)

		// 构建 sessionID → store entry 映射
		storeBySessionID := map[string]struct {
			key   string
			entry *SessionEntry
		}{}
		for k, e := range storeEntries {
			if e != nil && e.SessionId != "" {
				storeBySessionID[e.SessionId] = struct {
					key   string
					entry *SessionEntry
				}{key: k, entry: e}
			}
		}

		for _, d := range discovered {
			if match, ok := storeBySessionID[d.sessionID]; ok {
				updAt := d.mtime
				if match.entry.UpdatedAt > 0 {
					updAt = match.entry.UpdatedAt
				}
				merged = append(merged, mergedEntry{
					key: match.key, sessionID: d.sessionID, sessionFile: d.sessionFile,
					label: match.entry.Label, updatedAt: updAt, agentID: d.agentID,
					storeEntry: match.entry,
				})
			} else {
				merged = append(merged, mergedEntry{
					key:         fmt.Sprintf("agent:%s:%s", d.agentID, d.sessionID),
					sessionID:   d.sessionID,
					sessionFile: d.sessionFile,
					updatedAt:   d.mtime,
					agentID:     d.agentID,
				})
			}
		}
	}

	// 按 updatedAt 降序
	sort.Slice(merged, func(i, j int) bool { return merged[i].updatedAt > merged[j].updatedAt })

	// 限制
	if len(merged) > limit {
		merged = merged[:limit]
	}

	// 聚合
	aggTotals := &usageTotals{}
	aggMsgs := &messageCounts{}
	// byModel: provider::model → *usageTotals
	globalByModel := map[string]*usageTotals{}
	// byAgent: agentID → *usageTotals
	globalByAgent := map[string]*usageTotals{}
	// tools: toolName → callCount
	globalToolNames := map[string]int{}
	// daily: date → aggregated entry
	type globalDailyEntry struct {
		Tokens    int     `json:"tokens"`
		Cost      float64 `json:"cost"`
		Messages  int     `json:"messages"`
		ToolCalls int     `json:"toolCalls"`
		Errors    int     `json:"errors"`
	}
	globalDaily := map[string]*globalDailyEntry{}
	globalDailyModel := map[string]*dailyModelEntry{}
	globalLatencyByDate := map[string][]float64{}
	globalByChannel := map[string]*usageTotals{}

	sessionsOut := make([]map[string]interface{}, 0, len(merged))

	for _, m := range merged {
		cost := loadSessionCostFromFile(m.sessionFile)

		if cost != nil {
			mergeTotalsInto(aggTotals, &cost.usageTotals)
			if cost.MessageCounts != nil {
				aggMsgs.Total += cost.MessageCounts.Total
				aggMsgs.User += cost.MessageCounts.User
				aggMsgs.Assistant += cost.MessageCounts.Assistant
				aggMsgs.ToolCalls += cost.MessageCounts.ToolCalls
				aggMsgs.ToolResults += cost.MessageCounts.ToolResults
				aggMsgs.Errors += cost.MessageCounts.Errors
			}
			// 合并 byModel
			for mk, mt := range cost.ByModel {
				if globalByModel[mk] == nil {
					globalByModel[mk] = &usageTotals{}
				}
				mergeTotalsInto(globalByModel[mk], mt)
			}
			// 合并 toolNames
			for tn, cnt := range cost.ToolNames {
				globalToolNames[tn] += cnt
			}
			// 合并 byAgent
			if m.agentID != "" {
				if globalByAgent[m.agentID] == nil {
					globalByAgent[m.agentID] = &usageTotals{}
				}
				mergeTotalsInto(globalByAgent[m.agentID], &cost.usageTotals)
			}
			// 合并 daily tokens
			for _, d := range cost.DailyBreakdown {
				gd := globalDaily[d.Date]
				if gd == nil {
					gd = &globalDailyEntry{}
					globalDaily[d.Date] = gd
				}
				gd.Tokens += d.Tokens
				gd.Cost += d.Cost
			}
			// 合并 daily message counts
			for _, dm := range cost.DailyMessageCounts {
				gd := globalDaily[dm.Date]
				if gd == nil {
					gd = &globalDailyEntry{}
					globalDaily[dm.Date] = gd
				}
				gd.Messages += dm.Total
				gd.ToolCalls += dm.ToolCalls
				gd.Errors += dm.Errors
			}
			// 合并 dailyModelUsage (D-01)
			for _, dm := range cost.DailyModelUsage {
				key := dm.Date + "::" + dm.Provider + "::" + dm.Model
				gm := globalDailyModel[key]
				if gm == nil {
					gm = &dailyModelEntry{Date: dm.Date, Provider: dm.Provider, Model: dm.Model}
					globalDailyModel[key] = gm
				}
				gm.Tokens += dm.Tokens
				gm.Cost += dm.Cost
				gm.Count += dm.Count
			}
			// 合并 rawLatencyData (D-03)
			for dateStr, vals := range cost.rawLatencyData {
				globalLatencyByDate[dateStr] = append(globalLatencyByDate[dateStr], vals...)
			}
		}

		// 按频道聚合 (D-04)
		ch := "direct"
		if m.storeEntry != nil && m.storeEntry.Channel != "" {
			ch = m.storeEntry.Channel
		}
		if cost != nil {
			if globalByChannel[ch] == nil {
				globalByChannel[ch] = &usageTotals{}
			}
			mergeTotalsInto(globalByChannel[ch], &cost.usageTotals)
		}

		entry := map[string]interface{}{
			"key":       m.key,
			"sessionId": m.sessionID,
			"updatedAt": m.updatedAt,
			"agentId":   m.agentID,
			"usage":     cost,
		}
		if m.label != "" {
			entry["label"] = m.label
		}
		// Session 元数据 (D-05)
		if m.storeEntry != nil {
			if m.storeEntry.Subject != "" {
				entry["subject"] = m.storeEntry.Subject
			}
			if m.storeEntry.GroupChannel != "" {
				entry["room"] = m.storeEntry.GroupChannel
			}
			if m.storeEntry.Space != "" {
				entry["space"] = m.storeEntry.Space
			}
			if m.storeEntry.Channel != "" {
				entry["channel"] = m.storeEntry.Channel
			}
		}
		sessionsOut = append(sessionsOut, entry)
	}

	// 构建 byModel 数组
	byModelArr := make([]map[string]interface{}, 0, len(globalByModel))
	for mk, mt := range globalByModel {
		parts := strings.SplitN(mk, "::", 2)
		provider := ""
		model := mk
		if len(parts) == 2 {
			provider = parts[0]
			model = parts[1]
		}
		byModelArr = append(byModelArr, map[string]interface{}{
			"model":    model,
			"provider": provider,
			"totals":   mt,
		})
	}

	// 构建 byProvider 数组 (从 byModel 聚合)
	byProviderMap := map[string]*usageTotals{}
	for mk, mt := range globalByModel {
		parts := strings.SplitN(mk, "::", 2)
		p := "unknown"
		if len(parts) == 2 {
			p = parts[0]
		}
		if byProviderMap[p] == nil {
			byProviderMap[p] = &usageTotals{}
		}
		mergeTotalsInto(byProviderMap[p], mt)
	}
	byProviderArr := make([]map[string]interface{}, 0, len(byProviderMap))
	for p, t := range byProviderMap {
		byProviderArr = append(byProviderArr, map[string]interface{}{
			"provider": p,
			"totals":   t,
		})
	}

	// 构建 byAgent 数组
	byAgentArr := make([]map[string]interface{}, 0, len(globalByAgent))
	for aid, t := range globalByAgent {
		byAgentArr = append(byAgentArr, map[string]interface{}{
			"agentId": aid,
			"totals":  t,
		})
	}

	// 构建 tools 数组
	toolsArr := make([]map[string]interface{}, 0, len(globalToolNames))
	for tn, cnt := range globalToolNames {
		toolsArr = append(toolsArr, map[string]interface{}{
			"name":  tn,
			"calls": cnt,
		})
	}

	// 构建 daily 数组 (排序)
	dailyArr := make([]map[string]interface{}, 0, len(globalDaily))
	for d, gd := range globalDaily {
		dailyArr = append(dailyArr, map[string]interface{}{
			"date":      d,
			"tokens":    gd.Tokens,
			"cost":      gd.Cost,
			"messages":  gd.Messages,
			"toolCalls": gd.ToolCalls,
			"errors":    gd.Errors,
		})
	}
	sort.Slice(dailyArr, func(i, j int) bool {
		di, _ := dailyArr[i]["date"].(string)
		dj, _ := dailyArr[j]["date"].(string)
		return di < dj
	})

	// 构建 modelDaily 数组 (D-01)
	modelDailyArr := make([]map[string]interface{}, 0, len(globalDailyModel))
	for _, gm := range globalDailyModel {
		modelDailyArr = append(modelDailyArr, map[string]interface{}{
			"date":     gm.Date,
			"provider": gm.Provider,
			"model":    gm.Model,
			"tokens":   gm.Tokens,
			"cost":     gm.Cost,
			"count":    gm.Count,
		})
	}
	sort.Slice(modelDailyArr, func(i, j int) bool {
		di, _ := modelDailyArr[i]["date"].(string)
		dj, _ := modelDailyArr[j]["date"].(string)
		if di != dj {
			return di < dj
		}
		mi, _ := modelDailyArr[i]["model"].(string)
		mj, _ := modelDailyArr[j]["model"].(string)
		return mi < mj
	})

	// 构建全局 dailyLatency (D-03)
	aggDailyLatency := make([]map[string]interface{}, 0, len(globalLatencyByDate))
	for dateStr, vals := range globalLatencyByDate {
		if len(vals) == 0 {
			continue
		}
		sort.Float64s(vals)
		n := len(vals)
		sum := 0.0
		for _, v := range vals {
			sum += v
		}
		p95Idx := int(math.Ceil(0.95*float64(n))) - 1
		if p95Idx < 0 {
			p95Idx = 0
		}
		if p95Idx >= n {
			p95Idx = n - 1
		}
		aggDailyLatency = append(aggDailyLatency, map[string]interface{}{
			"date":  dateStr,
			"count": n,
			"avgMs": math.Round(sum/float64(n)*100) / 100,
			"p95Ms": vals[p95Idx],
			"minMs": vals[0],
			"maxMs": vals[n-1],
		})
	}
	sort.Slice(aggDailyLatency, func(i, j int) bool {
		di, _ := aggDailyLatency[i]["date"].(string)
		dj, _ := aggDailyLatency[j]["date"].(string)
		return di < dj
	})

	// 构建 byChannel 数组 (D-04)
	byChannelArr := make([]map[string]interface{}, 0, len(globalByChannel))
	for ch, t := range globalByChannel {
		byChannelArr = append(byChannelArr, map[string]interface{}{
			"channel": ch,
			"totals":  t,
		})
	}

	now := time.Now()
	ctx.Respond(true, map[string]interface{}{
		"updatedAt": now.UnixMilli(),
		"startDate": formatDateStr(dr.startMs),
		"endDate":   formatDateStr(dr.endMs),
		"sessions":  sessionsOut,
		"totals":    aggTotals,
		"aggregates": map[string]interface{}{
			"messages": aggMsgs,
			"tools": map[string]interface{}{
				"totalCalls":  aggMsgs.ToolCalls,
				"uniqueTools": len(globalToolNames),
				"tools":       toolsArr,
			},
			"byModel":      byModelArr,
			"byProvider":   byProviderArr,
			"byAgent":      byAgentArr,
			"byChannel":    byChannelArr,
			"dailyLatency": aggDailyLatency,
			"modelDaily":   modelDailyArr,
			"daily":        dailyArr,
		},
	}, nil)
}

// ---------- sessions.usage.timeseries ----------

func handleSessionsUsageTimeseries(ctx *MethodHandlerContext) {
	key, _ := ctx.Params["key"].(string)
	if strings.TrimSpace(key) == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "key is required for timeseries"))
		return
	}

	// 解析 key 获取 session file
	parsed := scope.ParseAgentSessionKey(key)
	var agentID, rawSessionID string
	if parsed != nil {
		agentID = parsed.AgentID
		rawSessionID = parsed.Rest
	} else {
		rawSessionID = key
	}
	sessionFile := sessions.ResolveSessionFilePath(rawSessionID, nil, agentID)

	// 从 JSONL 提取时序数据
	points := loadTimeseriesFromFile(sessionFile)

	ctx.Respond(true, map[string]interface{}{
		"sessionId": rawSessionID,
		"points":    points,
	}, nil)
}

// loadTimeseriesFromFile 从 JSONL 提取每消息 usage 时序。
func loadTimeseriesFromFile(sessionFile string) []map[string]interface{} {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return []map[string]interface{}{}
	}

	var points []map[string]interface{}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		u, ok := entry["usage"].(map[string]interface{})
		if !ok {
			continue
		}

		ts, _ := entry["timestamp"].(float64)
		if ts == 0 {
			continue
		}

		input, _ := u["input_tokens"].(float64)
		output, _ := u["output_tokens"].(float64)

		points = append(points, map[string]interface{}{
			"timestamp":    int64(ts),
			"inputTokens":  int(input),
			"outputTokens": int(output),
			"totalTokens":  int(input + output),
		})
	}

	if points == nil {
		return []map[string]interface{}{}
	}
	// 限制 200 点
	if len(points) > 200 {
		points = points[len(points)-200:]
	}
	return points
}

// ---------- sessions.usage.logs ----------

func handleSessionsUsageLogs(ctx *MethodHandlerContext) {
	key, _ := ctx.Params["key"].(string)
	if strings.TrimSpace(key) == "" {
		ctx.Respond(false, nil, NewErrorShape(ErrCodeBadRequest, "key is required for logs"))
		return
	}

	limit := 200
	if v, ok := ctx.Params["limit"].(float64); ok && !math.IsNaN(v) {
		l := int(v)
		if l > 0 && l < 1000 {
			limit = l
		}
	}

	parsed := scope.ParseAgentSessionKey(key)
	var agentID, rawSessionID string
	if parsed != nil {
		agentID = parsed.AgentID
		rawSessionID = parsed.Rest
	} else {
		rawSessionID = key
	}
	sessionFile := sessions.ResolveSessionFilePath(rawSessionID, nil, agentID)

	logs := loadLogsFromFile(sessionFile, limit)
	ctx.Respond(true, map[string]interface{}{
		"logs": logs,
	}, nil)
}

// loadLogsFromFile 从 JSONL 文件加载日志条目。
func loadLogsFromFile(sessionFile string, limit int) []map[string]interface{} {
	data, err := os.ReadFile(sessionFile)
	if err != nil {
		return []map[string]interface{}{}
	}

	lines := strings.Split(string(data), "\n")
	var logs []map[string]interface{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		role, _ := entry["role"].(string)
		if role == "" {
			continue
		}
		logEntry := map[string]interface{}{
			"role": role,
		}
		if ts, ok := entry["timestamp"].(float64); ok {
			logEntry["timestamp"] = int64(ts)
		}
		if u, ok := entry["usage"].(map[string]interface{}); ok {
			logEntry["usage"] = u
		}
		if model, ok := entry["model"].(string); ok {
			logEntry["model"] = model
		}
		logs = append(logs, logEntry)
	}

	if logs == nil {
		return []map[string]interface{}{}
	}
	if len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}
	return logs
}

// ---------- 辅助函数 ----------

func mergeTotalsInto(dst, src *usageTotals) {
	dst.Input += src.Input
	dst.Output += src.Output
	dst.CacheRead += src.CacheRead
	dst.CacheWrite += src.CacheWrite
	dst.TotalTokens += src.TotalTokens
	dst.TotalCost += src.TotalCost
	dst.InputCost += src.InputCost
	dst.OutputCost += src.OutputCost
	dst.CacheReadCost += src.CacheReadCost
	dst.CacheWriteCost += src.CacheWriteCost
	dst.MissingCostEntries += src.MissingCostEntries
}

func init() {
	// 静默未使用的 slog import（后续会用于 discovery 错误日志）
	_ = slog.Info
}
