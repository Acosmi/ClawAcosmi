package gateway

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"
)

// ---------- 配置热重载类型 ----------

// ReloadMode 热重载模式。
type ReloadMode string

const (
	ReloadModeOff     ReloadMode = "off"
	ReloadModeRestart ReloadMode = "restart"
	ReloadModeHot     ReloadMode = "hot"
	ReloadModeHybrid  ReloadMode = "hybrid"
)

// ReloadSettings 热重载设定。
type ReloadSettings struct {
	Mode       ReloadMode
	DebounceMs int
}

// DefaultReloadSettings 默认热重载设定。
var DefaultReloadSettings = ReloadSettings{Mode: ReloadModeHybrid, DebounceMs: 300}

// ReloadPlan 热重载计划。
type ReloadPlan struct {
	ChangedPaths          []string
	RestartGateway        bool
	RestartReasons        []string
	HotReasons            []string
	ReloadHooks           bool
	RestartGmailWatcher   bool
	RestartBrowserControl bool
	RestartCron           bool
	RestartHeartbeat      bool
	RestartChannels       map[string]struct{}
	NoopPaths             []string
}

// ---------- 重载规则 ----------

type reloadRuleKind string

const (
	ruleRestart reloadRuleKind = "restart"
	ruleHot     reloadRuleKind = "hot"
	ruleNone    reloadRuleKind = "none"
)

type reloadRule struct {
	prefix  string
	kind    reloadRuleKind
	actions []string
}

var baseReloadRules = []reloadRule{
	{prefix: "gateway.remote", kind: ruleNone},
	{prefix: "gateway.reload", kind: ruleNone},
	{prefix: "hooks.gmail", kind: ruleHot, actions: []string{"restart-gmail-watcher"}},
	{prefix: "hooks", kind: ruleHot, actions: []string{"reload-hooks"}},
	{prefix: "agents.defaults.heartbeat", kind: ruleHot, actions: []string{"restart-heartbeat"}},
	{prefix: "agent.heartbeat", kind: ruleHot, actions: []string{"restart-heartbeat"}},
	{prefix: "cron", kind: ruleHot, actions: []string{"restart-cron"}},
	{prefix: "browser", kind: ruleHot, actions: []string{"restart-browser-control"}},
}

var tailReloadRules = []reloadRule{
	{prefix: "identity", kind: ruleNone},
	{prefix: "wizard", kind: ruleNone},
	{prefix: "logging", kind: ruleNone},
	{prefix: "models", kind: ruleNone},
	{prefix: "agents", kind: ruleNone},
	{prefix: "tools", kind: ruleNone},
	{prefix: "bindings", kind: ruleNone},
	{prefix: "audio", kind: ruleNone},
	{prefix: "agent", kind: ruleNone},
	{prefix: "routing", kind: ruleNone},
	{prefix: "messages", kind: ruleNone},
	{prefix: "session", kind: ruleNone},
	{prefix: "talk", kind: ruleNone},
	{prefix: "skills", kind: ruleNone},
	{prefix: "plugins", kind: ruleRestart},
	{prefix: "ui", kind: ruleNone},
	{prefix: "gateway", kind: ruleRestart},
	{prefix: "discovery", kind: ruleRestart},
	{prefix: "canvasHost", kind: ruleRestart},
}

func matchRule(path string, extraRules []reloadRule) *reloadRule {
	allRules := make([]reloadRule, 0, len(baseReloadRules)+len(extraRules)+len(tailReloadRules))
	allRules = append(allRules, baseReloadRules...)
	allRules = append(allRules, extraRules...)
	allRules = append(allRules, tailReloadRules...)

	for _, rule := range allRules {
		if path == rule.prefix || strings.HasPrefix(path, rule.prefix+".") {
			return &rule
		}
	}
	return nil
}

// BuildReloadPlan 根据变更路径构建重载计划。
func BuildReloadPlan(changedPaths []string, extraRules []reloadRule) *ReloadPlan {
	plan := &ReloadPlan{
		ChangedPaths:    changedPaths,
		RestartChannels: make(map[string]struct{}),
	}
	for _, p := range changedPaths {
		rule := matchRule(p, extraRules)
		if rule == nil {
			plan.RestartGateway = true
			plan.RestartReasons = append(plan.RestartReasons, p)
			continue
		}
		if rule.kind == ruleRestart {
			plan.RestartGateway = true
			plan.RestartReasons = append(plan.RestartReasons, p)
			continue
		}
		if rule.kind == ruleNone {
			plan.NoopPaths = append(plan.NoopPaths, p)
			continue
		}
		plan.HotReasons = append(plan.HotReasons, p)
		for _, action := range rule.actions {
			applyReloadAction(plan, action)
		}
	}
	if plan.RestartGmailWatcher {
		plan.ReloadHooks = true
	}
	return plan
}

func applyReloadAction(plan *ReloadPlan, action string) {
	if strings.HasPrefix(action, "restart-channel:") {
		ch := action[len("restart-channel:"):]
		plan.RestartChannels[ch] = struct{}{}
		return
	}
	switch action {
	case "reload-hooks":
		plan.ReloadHooks = true
	case "restart-gmail-watcher":
		plan.RestartGmailWatcher = true
	case "restart-browser-control":
		plan.RestartBrowserControl = true
	case "restart-cron":
		plan.RestartCron = true
	case "restart-heartbeat":
		plan.RestartHeartbeat = true
	}
}

// ---------- 配置 Diff ----------

// DiffConfigPaths 深度比较两个配置对象，返回变更路径列表。
func DiffConfigPaths(prev, next interface{}, prefix string) []string {
	if reflect.DeepEqual(prev, next) {
		return nil
	}
	prevMap, prevOk := toMap(prev)
	nextMap, nextOk := toMap(next)
	if prevOk && nextOk {
		keys := mergeKeys(prevMap, nextMap)
		var paths []string
		for _, key := range keys {
			childPrefix := key
			if prefix != "" {
				childPrefix = prefix + "." + key
			}
			childPaths := DiffConfigPaths(prevMap[key], nextMap[key], childPrefix)
			paths = append(paths, childPaths...)
		}
		return paths
	}
	// R-2: 数组元素级别比较
	prevArr, prevArrOk := toArray(prev)
	nextArr, nextArrOk := toArray(next)
	if prevArrOk && nextArrOk {
		var paths []string
		maxLen := len(prevArr)
		if len(nextArr) > maxLen {
			maxLen = len(nextArr)
		}
		for i := 0; i < maxLen; i++ {
			childPrefix := fmt.Sprintf("%s[%d]", prefix, i)
			if prefix == "" {
				childPrefix = fmt.Sprintf("[%d]", i)
			}
			var p, n interface{}
			if i < len(prevArr) {
				p = prevArr[i]
			}
			if i < len(nextArr) {
				n = nextArr[i]
			}
			childPaths := DiffConfigPaths(p, n, childPrefix)
			paths = append(paths, childPaths...)
		}
		return paths
	}
	if prefix == "" {
		return []string{"<root>"}
	}
	return []string{prefix}
}

func toArray(v interface{}) ([]interface{}, bool) {
	if v == nil {
		return nil, false
	}
	a, ok := v.([]interface{})
	return a, ok
}

func toMap(v interface{}) (map[string]interface{}, bool) {
	if v == nil {
		return nil, false
	}
	m, ok := v.(map[string]interface{})
	return m, ok
}

func mergeKeys(a, b map[string]interface{}) []string {
	seen := make(map[string]struct{})
	var keys []string
	for k := range a {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
	}
	for k := range b {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
	}
	return keys
}

// ---------- 文件监视器（防抖） ----------

// ConfigWatcher 配置文件监视器。
type ConfigWatcher struct {
	mu         sync.Mutex
	debounceMs int
	timer      *time.Timer
	stopChan   chan struct{}
	onChange   func()
}

// NewConfigWatcher 创建配置文件监视器。
func NewConfigWatcher(debounceMs int, onChange func()) *ConfigWatcher {
	if debounceMs <= 0 {
		debounceMs = 300
	}
	return &ConfigWatcher{
		debounceMs: debounceMs,
		stopChan:   make(chan struct{}),
		onChange:   onChange,
	}
}

// Notify 通知有文件变更（带防抖）。
func (w *ConfigWatcher) Notify() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(time.Duration(w.debounceMs)*time.Millisecond, func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		select {
		case <-w.stopChan:
			return
		default:
			if w.onChange != nil {
				w.onChange()
			}
		}
	})
}

// Stop 停止监视器。
func (w *ConfigWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.stopChan:
	default:
		close(w.stopChan)
	}
	if w.timer != nil {
		w.timer.Stop()
	}
}

// ---------- 配置快照序列化 ----------

// ConfigSnapshot 配置快照，用于 diff 比较。
func ConfigSnapshot(cfg interface{}) map[string]interface{} {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil
	}
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}

// ---------- 配置解析 ----------

// ReloadSettingsRaw 原始重载配置 (从 JSON/YAML 解析)。
type ReloadSettingsRaw struct {
	Mode       string `json:"mode,omitempty"`
	DebounceMs *int   `json:"debounceMs,omitempty"`
}

// ResolveReloadSettings 从原始配置解析重载设定。
func ResolveReloadSettings(raw *ReloadSettingsRaw) ReloadSettings {
	if raw == nil {
		return DefaultReloadSettings
	}
	settings := DefaultReloadSettings

	switch ReloadMode(strings.ToLower(strings.TrimSpace(raw.Mode))) {
	case ReloadModeOff, ReloadModeRestart, ReloadModeHot, ReloadModeHybrid:
		settings.Mode = ReloadMode(strings.ToLower(strings.TrimSpace(raw.Mode)))
	default:
		// 保持默认 hybrid
	}

	if raw.DebounceMs != nil {
		ms := *raw.DebounceMs
		if ms < 50 {
			ms = 50
		}
		if ms > 10000 {
			ms = 10000
		}
		settings.DebounceMs = ms
	}

	return settings
}

// ---------- 动态频道规则注入 ----------

var (
	channelRulesMu sync.RWMutex
	channelRules   []reloadRule
)

// RegisterChannelReloadRules 注册频道插件的重载规则。
// 例如: RegisterChannelReloadRules("slack", "restart-channel:slack")
func RegisterChannelReloadRules(channelPrefix, action string) {
	channelRulesMu.Lock()
	defer channelRulesMu.Unlock()
	channelRules = append(channelRules, reloadRule{
		prefix:  "channels." + channelPrefix,
		kind:    ruleHot,
		actions: []string{action},
	})
}

// getChannelRules 获取当前注册的频道规则（线程安全）。
func getChannelRules() []reloadRule {
	channelRulesMu.RLock()
	defer channelRulesMu.RUnlock()
	rules := make([]reloadRule, len(channelRules))
	copy(rules, channelRules)
	return rules
}

// ---------- 配置重载器 ----------

// ConfigReloaderCallbacks 重载器回调。
type ConfigReloaderCallbacks struct {
	// LoadConfig 重新加载配置并返回新快照。
	LoadConfig func() (map[string]interface{}, error)
	// OnHotReload 热重载回调，接收重载计划。
	OnHotReload func(plan *ReloadPlan)
	// OnRestart 需要完全重启时的回调。
	OnRestart func(plan *ReloadPlan)
	// OnError 加载/处理出错时的回调。
	OnError func(err error)
	// ResolveSettings R-R4: 从新配置重新解析 reload settings (可选)。
	ResolveSettings func(newConfig map[string]interface{}) ReloadSettings
}

// ConfigReloader 配置重载器，整合文件监视、diff、plan 构建。
type ConfigReloader struct {
	mu            sync.Mutex
	settings      ReloadSettings
	watcher       *ConfigWatcher
	snapshot      map[string]interface{}
	cb            ConfigReloaderCallbacks
	running       bool
	pending       bool
	stopped       bool
	restartQueued bool // R-1: 防重复重启标志
}

// StartConfigReloader 启动配置重载器。
func StartConfigReloader(
	settings ReloadSettings,
	initialSnapshot map[string]interface{},
	cb ConfigReloaderCallbacks,
) *ConfigReloader {
	r := &ConfigReloader{
		settings: settings,
		snapshot: initialSnapshot,
		cb:       cb,
	}

	if settings.Mode == ReloadModeOff {
		return r
	}

	r.watcher = NewConfigWatcher(settings.DebounceMs, func() {
		r.handleChange()
	})

	return r
}

// Notify 通知配置文件变更。
func (r *ConfigReloader) Notify() {
	if r.watcher != nil {
		r.watcher.Notify()
	}
}

// Stop 停止重载器。
func (r *ConfigReloader) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stopped = true
	if r.watcher != nil {
		r.watcher.Stop()
	}
}

func (r *ConfigReloader) handleChange() {
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return
	}
	if r.running {
		r.pending = true
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.running = false
		shouldRunAgain := r.pending
		r.pending = false
		r.mu.Unlock()
		if shouldRunAgain {
			r.handleChange()
		}
	}()

	// 加载新配置
	newSnapshot, err := r.cb.LoadConfig()
	if err != nil {
		if r.cb.OnError != nil {
			r.cb.OnError(err)
		}
		return
	}

	// Diff
	r.mu.Lock()
	oldSnapshot := r.snapshot
	r.mu.Unlock()

	changedPaths := DiffConfigPaths(oldSnapshot, newSnapshot, "")
	if len(changedPaths) == 0 {
		return
	}

	// 构建 plan
	channelExtraRules := getChannelRules()
	plan := BuildReloadPlan(changedPaths, channelExtraRules)

	// R-R4: 更新快照和 settings (对齐 TS L311-312: settings = resolveGatewayReloadSettings(nextConfig))
	r.mu.Lock()
	r.snapshot = newSnapshot
	if r.cb.ResolveSettings != nil {
		r.settings = r.cb.ResolveSettings(newSnapshot)
	}
	r.mu.Unlock()

	// 根据 mode 决定动作
	switch r.settings.Mode {
	case ReloadModeRestart:
		// R-1: 防重复重启
		r.mu.Lock()
		queued := r.restartQueued
		r.restartQueued = true
		r.mu.Unlock()
		if !queued && r.cb.OnRestart != nil {
			r.cb.OnRestart(plan)
		}
	case ReloadModeHot:
		if !plan.RestartGateway && r.cb.OnHotReload != nil {
			r.cb.OnHotReload(plan)
		}
	case ReloadModeHybrid:
		if plan.RestartGateway {
			// R-1: 防重复重启
			r.mu.Lock()
			queued := r.restartQueued
			r.restartQueued = true
			r.mu.Unlock()
			if !queued && r.cb.OnRestart != nil {
				r.cb.OnRestart(plan)
			}
		} else if r.cb.OnHotReload != nil {
			r.cb.OnHotReload(plan)
		}
	}
}
