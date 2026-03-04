package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Acosmi/ClawAcosmi/internal/config"
)

// ---------- 设备配对管理 ----------
// 对齐 TS infra/device-pairing.ts

// pendingTTLMs 待处理配对请求的过期时间（5 分钟）。
const pendingTTLMs = 5 * 60 * 1000

// ---------- 类型定义 ----------

// DevicePairingPendingRequest 待处理的配对请求。
type DevicePairingPendingRequest struct {
	RequestID   string   `json:"requestId"`
	DeviceID    string   `json:"deviceId"`
	PublicKey   string   `json:"publicKey"`
	DisplayName string   `json:"displayName,omitempty"`
	Platform    string   `json:"platform,omitempty"`
	ClientID    string   `json:"clientId,omitempty"`
	ClientMode  string   `json:"clientMode,omitempty"`
	Role        string   `json:"role,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	RemoteIP    string   `json:"remoteIp,omitempty"`
	Silent      bool     `json:"silent,omitempty"`
	IsRepair    bool     `json:"isRepair,omitempty"`
	Ts          int64    `json:"ts"`
}

// DeviceAuthToken 设备认证 token。
type DeviceAuthToken struct {
	Token        string   `json:"token"`
	Role         string   `json:"role"`
	Scopes       []string `json:"scopes"`
	CreatedAtMs  int64    `json:"createdAtMs"`
	RotatedAtMs  *int64   `json:"rotatedAtMs,omitempty"`
	RevokedAtMs  *int64   `json:"revokedAtMs,omitempty"`
	LastUsedAtMs *int64   `json:"lastUsedAtMs,omitempty"`
}

// DeviceAuthTokenSummary token 摘要（不含 token 值）。
type DeviceAuthTokenSummary struct {
	Role         string   `json:"role"`
	Scopes       []string `json:"scopes"`
	CreatedAtMs  int64    `json:"createdAtMs"`
	RotatedAtMs  *int64   `json:"rotatedAtMs,omitempty"`
	RevokedAtMs  *int64   `json:"revokedAtMs,omitempty"`
	LastUsedAtMs *int64   `json:"lastUsedAtMs,omitempty"`
}

// PairedDevice 已配对设备。
type PairedDevice struct {
	DeviceID     string                      `json:"deviceId"`
	PublicKey    string                      `json:"publicKey"`
	DisplayName  string                      `json:"displayName,omitempty"`
	Platform     string                      `json:"platform,omitempty"`
	ClientID     string                      `json:"clientId,omitempty"`
	ClientMode   string                      `json:"clientMode,omitempty"`
	Role         string                      `json:"role,omitempty"`
	Roles        []string                    `json:"roles,omitempty"`
	Scopes       []string                    `json:"scopes,omitempty"`
	RemoteIP     string                      `json:"remoteIp,omitempty"`
	Tokens       map[string]*DeviceAuthToken `json:"tokens,omitempty"`
	CreatedAtMs  int64                       `json:"createdAtMs"`
	ApprovedAtMs int64                       `json:"approvedAtMs"`
}

// DevicePairingList 配对列表。
type DevicePairingList struct {
	Pending []*DevicePairingPendingRequest `json:"pending"`
	Paired  []*PairedDevice                `json:"paired"`
}

// ---------- 工具函数 ----------

func normalizeDeviceID(deviceID string) string {
	return strings.TrimSpace(deviceID)
}

func normalizeRole(role string) string {
	return strings.TrimSpace(role)
}

func mergeRoles(items ...interface{}) []string {
	roles := make(map[string]struct{})
	for _, item := range items {
		switch v := item.(type) {
		case string:
			if t := strings.TrimSpace(v); t != "" {
				roles[t] = struct{}{}
			}
		case []string:
			for _, r := range v {
				if t := strings.TrimSpace(r); t != "" {
					roles[t] = struct{}{}
				}
			}
		}
	}
	if len(roles) == 0 {
		return nil
	}
	out := make([]string, 0, len(roles))
	for r := range roles {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func mergeScopes(items ...[]string) []string {
	scopes := make(map[string]struct{})
	for _, item := range items {
		for _, s := range item {
			if t := strings.TrimSpace(s); t != "" {
				scopes[t] = struct{}{}
			}
		}
	}
	if len(scopes) == 0 {
		return nil
	}
	out := make([]string, 0, len(scopes))
	for s := range scopes {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func normalizeScopes(scopes []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, s := range scopes {
		if t := strings.TrimSpace(s); t != "" {
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				out = append(out, t)
			}
		}
	}
	sort.Strings(out)
	return out
}

func scopesAllow(requested, allowed []string) bool {
	if len(requested) == 0 {
		return true
	}
	if len(allowed) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(allowed))
	for _, s := range allowed {
		set[s] = struct{}{}
	}
	for _, s := range requested {
		if _, ok := set[s]; !ok {
			return false
		}
	}
	return true
}

func newDeviceToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 回退到时间戳
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ---------- 文件 I/O ----------

func resolvePairingPaths(baseDir string) (dir, pendingPath, pairedPath string) {
	if baseDir == "" {
		baseDir = config.ResolveStateDir()
	}
	dir = filepath.Join(baseDir, "devices")
	pendingPath = filepath.Join(dir, "pending.json")
	pairedPath = filepath.Join(dir, "paired.json")
	return
}

func readJSONFile(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在返回 nil（空状态）
		}
		return err
	}
	return json.Unmarshal(data, v)
}

func writeJSONAtomic(filePath string, v interface{}) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	tmp := filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, filePath); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// ---------- 状态管理 ----------

type devicePairingState struct {
	PendingByID      map[string]*DevicePairingPendingRequest `json:"-"`
	PairedByDeviceID map[string]*PairedDevice                `json:"-"`
}

func pruneExpiredPending(pending map[string]*DevicePairingPendingRequest, nowMs int64) {
	for id, req := range pending {
		if nowMs-req.Ts > pendingTTLMs {
			delete(pending, id)
		}
	}
}

var devicePairingMu sync.Mutex

func loadPairingState(baseDir string) (*devicePairingState, error) {
	_, pendingPath, pairedPath := resolvePairingPaths(baseDir)

	pending := make(map[string]*DevicePairingPendingRequest)
	paired := make(map[string]*PairedDevice)

	if err := readJSONFile(pendingPath, &pending); err != nil {
		return nil, fmt.Errorf("read pending: %w", err)
	}
	if err := readJSONFile(pairedPath, &paired); err != nil {
		return nil, fmt.Errorf("read paired: %w", err)
	}

	pruneExpiredPending(pending, time.Now().UnixMilli())

	return &devicePairingState{
		PendingByID:      pending,
		PairedByDeviceID: paired,
	}, nil
}

func persistPairingState(state *devicePairingState, baseDir string) error {
	_, pendingPath, pairedPath := resolvePairingPaths(baseDir)
	if err := writeJSONAtomic(pendingPath, state.PendingByID); err != nil {
		return err
	}
	return writeJSONAtomic(pairedPath, state.PairedByDeviceID)
}

// ---------- 配对核心逻辑 ----------

// ListDevicePairing 列出所有待处理和已配对设备。
func ListDevicePairing(baseDir string) (*DevicePairingList, error) {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(baseDir)
	if err != nil {
		return nil, err
	}

	pending := make([]*DevicePairingPendingRequest, 0, len(state.PendingByID))
	for _, req := range state.PendingByID {
		pending = append(pending, req)
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].Ts > pending[j].Ts })

	paired := make([]*PairedDevice, 0, len(state.PairedByDeviceID))
	for _, dev := range state.PairedByDeviceID {
		paired = append(paired, dev)
	}
	sort.Slice(paired, func(i, j int) bool { return paired[i].ApprovedAtMs > paired[j].ApprovedAtMs })

	return &DevicePairingList{Pending: pending, Paired: paired}, nil
}

// GetPairedDevice 获取已配对设备。
func GetPairedDevice(deviceID, baseDir string) (*PairedDevice, error) {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(baseDir)
	if err != nil {
		return nil, err
	}
	return state.PairedByDeviceID[normalizeDeviceID(deviceID)], nil
}

// DevicePairingRequestInput 配对请求输入。
type DevicePairingRequestInput struct {
	DeviceID    string
	PublicKey   string
	DisplayName string
	Platform    string
	ClientID    string
	ClientMode  string
	Role        string
	Scopes      []string
	RemoteIP    string
	Silent      bool
}

// DevicePairingRequestResult 配对请求结果。
type DevicePairingRequestResult struct {
	Status  string                       `json:"status"`
	Request *DevicePairingPendingRequest `json:"request"`
	Created bool                         `json:"created"`
}

// RequestDevicePairing 发起配对请求。
func RequestDevicePairing(req DevicePairingRequestInput, baseDir string) (*DevicePairingRequestResult, error) {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(baseDir)
	if err != nil {
		return nil, err
	}

	deviceID := normalizeDeviceID(req.DeviceID)
	if deviceID == "" {
		return nil, fmt.Errorf("deviceId required")
	}

	// 检查是否已有 pending 请求
	for _, p := range state.PendingByID {
		if p.DeviceID == deviceID {
			return &DevicePairingRequestResult{Status: "pending", Request: p, Created: false}, nil
		}
	}

	_, isRepair := state.PairedByDeviceID[deviceID]

	var roles []string
	if req.Role != "" {
		roles = []string{req.Role}
	}

	b := make([]byte, 16)
	rand.Read(b)
	requestID := hex.EncodeToString(b)

	request := &DevicePairingPendingRequest{
		RequestID:   requestID,
		DeviceID:    deviceID,
		PublicKey:   req.PublicKey,
		DisplayName: req.DisplayName,
		Platform:    req.Platform,
		ClientID:    req.ClientID,
		ClientMode:  req.ClientMode,
		Role:        req.Role,
		Roles:       roles,
		Scopes:      req.Scopes,
		RemoteIP:    req.RemoteIP,
		Silent:      req.Silent,
		IsRepair:    isRepair,
		Ts:          time.Now().UnixMilli(),
	}

	state.PendingByID[requestID] = request
	if err := persistPairingState(state, baseDir); err != nil {
		return nil, err
	}

	return &DevicePairingRequestResult{Status: "pending", Request: request, Created: true}, nil
}

// ApproveDevicePairingResult 配对批准结果。
type ApproveDevicePairingResult struct {
	RequestID string        `json:"requestId"`
	Device    *PairedDevice `json:"device"`
}

// ApproveDevicePairing 批准配对请求。
func ApproveDevicePairing(requestID, baseDir string) (*ApproveDevicePairingResult, error) {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(baseDir)
	if err != nil {
		return nil, err
	}

	pending, ok := state.PendingByID[requestID]
	if !ok {
		return nil, nil
	}

	now := time.Now().UnixMilli()
	existing := state.PairedByDeviceID[pending.DeviceID]

	var existingRoles []string
	var existingRole string
	var existingScopes []string
	var existingTokens map[string]*DeviceAuthToken

	if existing != nil {
		existingRoles = existing.Roles
		existingRole = existing.Role
		existingScopes = existing.Scopes
		existingTokens = existing.Tokens
	}

	roles := mergeRoles(existingRoles, existingRole, pending.Roles, pending.Role)
	scopes := mergeScopes(existingScopes, pending.Scopes)

	tokens := make(map[string]*DeviceAuthToken)
	for k, v := range existingTokens {
		cp := *v
		tokens[k] = &cp
	}

	roleForToken := normalizeRole(pending.Role)
	if roleForToken != "" {
		nextScopes := normalizeScopes(pending.Scopes)
		existingToken := tokens[roleForToken]

		createdAt := now
		if existingToken != nil {
			createdAt = existingToken.CreatedAtMs
		}

		var rotatedAt *int64
		if existingToken != nil {
			rotatedAt = &now
		}

		var lastUsed *int64
		if existingToken != nil {
			lastUsed = existingToken.LastUsedAtMs
		}

		tokens[roleForToken] = &DeviceAuthToken{
			Token:        newDeviceToken(),
			Role:         roleForToken,
			Scopes:       nextScopes,
			CreatedAtMs:  createdAt,
			RotatedAtMs:  rotatedAt,
			RevokedAtMs:  nil,
			LastUsedAtMs: lastUsed,
		}
	}

	createdAtMs := now
	if existing != nil {
		createdAtMs = existing.CreatedAtMs
	}

	device := &PairedDevice{
		DeviceID:     pending.DeviceID,
		PublicKey:    pending.PublicKey,
		DisplayName:  pending.DisplayName,
		Platform:     pending.Platform,
		ClientID:     pending.ClientID,
		ClientMode:   pending.ClientMode,
		Role:         pending.Role,
		Roles:        roles,
		Scopes:       scopes,
		RemoteIP:     pending.RemoteIP,
		Tokens:       tokens,
		CreatedAtMs:  createdAtMs,
		ApprovedAtMs: now,
	}

	delete(state.PendingByID, requestID)
	state.PairedByDeviceID[device.DeviceID] = device

	if err := persistPairingState(state, baseDir); err != nil {
		return nil, err
	}

	return &ApproveDevicePairingResult{RequestID: requestID, Device: device}, nil
}

// RejectDevicePairingResult 拒绝结果。
type RejectDevicePairingResult struct {
	RequestID string `json:"requestId"`
	DeviceID  string `json:"deviceId"`
}

// RejectDevicePairing 拒绝配对请求。
func RejectDevicePairing(requestID, baseDir string) (*RejectDevicePairingResult, error) {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(baseDir)
	if err != nil {
		return nil, err
	}

	pending, ok := state.PendingByID[requestID]
	if !ok {
		return nil, nil
	}

	delete(state.PendingByID, requestID)
	if err := persistPairingState(state, baseDir); err != nil {
		return nil, err
	}

	return &RejectDevicePairingResult{RequestID: requestID, DeviceID: pending.DeviceID}, nil
}

// UpdatePairedDeviceMetadata 更新已配对设备元数据。
func UpdatePairedDeviceMetadata(deviceID string, patch *PairedDeviceMetadataPatch, baseDir string) error {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(baseDir)
	if err != nil {
		return err
	}

	existing := state.PairedByDeviceID[normalizeDeviceID(deviceID)]
	if existing == nil {
		return nil
	}

	roles := mergeRoles(existing.Roles, existing.Role, patch.Role)
	scopes := mergeScopes(existing.Scopes, patch.Scopes)

	if patch.PublicKey != "" {
		existing.PublicKey = patch.PublicKey
	}
	if patch.DisplayName != "" {
		existing.DisplayName = patch.DisplayName
	}
	if patch.Platform != "" {
		existing.Platform = patch.Platform
	}
	if patch.ClientID != "" {
		existing.ClientID = patch.ClientID
	}
	if patch.ClientMode != "" {
		existing.ClientMode = patch.ClientMode
	}
	if patch.RemoteIP != "" {
		existing.RemoteIP = patch.RemoteIP
	}
	if patch.Role != "" {
		existing.Role = patch.Role
	}
	existing.Roles = roles
	existing.Scopes = scopes

	return persistPairingState(state, baseDir)
}

// PairedDeviceMetadataPatch 元数据更新补丁。
type PairedDeviceMetadataPatch struct {
	PublicKey   string
	DisplayName string
	Platform    string
	ClientID    string
	ClientMode  string
	Role        string
	Scopes      []string
	RemoteIP    string
}

// SummarizeDeviceTokens 生成 token 摘要（隐藏 token 值）。
func SummarizeDeviceTokens(tokens map[string]*DeviceAuthToken) []*DeviceAuthTokenSummary {
	if len(tokens) == 0 {
		return nil
	}
	summaries := make([]*DeviceAuthTokenSummary, 0, len(tokens))
	for _, t := range tokens {
		summaries = append(summaries, &DeviceAuthTokenSummary{
			Role:         t.Role,
			Scopes:       t.Scopes,
			CreatedAtMs:  t.CreatedAtMs,
			RotatedAtMs:  t.RotatedAtMs,
			RevokedAtMs:  t.RevokedAtMs,
			LastUsedAtMs: t.LastUsedAtMs,
		})
	}
	sort.Slice(summaries, func(i, j int) bool { return summaries[i].Role < summaries[j].Role })
	return summaries
}

// ---------- Token CRUD ----------

// VerifyDeviceTokenParams token 验证参数。
type VerifyDeviceTokenParams struct {
	DeviceID string
	Token    string
	Role     string
	Scopes   []string
	BaseDir  string
}

// VerifyDeviceTokenResult token 验证结果。
type VerifyDeviceTokenResult struct {
	OK     bool   `json:"ok"`
	Reason string `json:"reason,omitempty"`
}

// VerifyDeviceToken 验证设备 token。
func VerifyDeviceToken(params VerifyDeviceTokenParams) (*VerifyDeviceTokenResult, error) {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(params.BaseDir)
	if err != nil {
		return nil, err
	}

	device := state.PairedByDeviceID[normalizeDeviceID(params.DeviceID)]
	if device == nil {
		return &VerifyDeviceTokenResult{OK: false, Reason: "device-not-paired"}, nil
	}

	role := normalizeRole(params.Role)
	if role == "" {
		return &VerifyDeviceTokenResult{OK: false, Reason: "role-missing"}, nil
	}

	entry := device.Tokens[role]
	if entry == nil {
		return &VerifyDeviceTokenResult{OK: false, Reason: "token-missing"}, nil
	}

	if entry.RevokedAtMs != nil {
		return &VerifyDeviceTokenResult{OK: false, Reason: "token-revoked"}, nil
	}

	if entry.Token != params.Token {
		return &VerifyDeviceTokenResult{OK: false, Reason: "token-mismatch"}, nil
	}

	requested := normalizeScopes(params.Scopes)
	if !scopesAllow(requested, entry.Scopes) {
		return &VerifyDeviceTokenResult{OK: false, Reason: "scope-mismatch"}, nil
	}

	// 更新 lastUsedAtMs
	now := time.Now().UnixMilli()
	entry.LastUsedAtMs = &now
	if err := persistPairingState(state, params.BaseDir); err != nil {
		return nil, err
	}

	return &VerifyDeviceTokenResult{OK: true}, nil
}

// EnsureDeviceToken 确保设备 token 存在（已有且有效则返回，否则创建新的）。
func EnsureDeviceToken(deviceID, role string, scopes []string, baseDir string) (*DeviceAuthToken, error) {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(baseDir)
	if err != nil {
		return nil, err
	}

	device := state.PairedByDeviceID[normalizeDeviceID(deviceID)]
	if device == nil {
		return nil, nil
	}

	r := normalizeRole(role)
	if r == "" {
		return nil, nil
	}

	requested := normalizeScopes(scopes)

	if device.Tokens != nil {
		existing := device.Tokens[r]
		if existing != nil && existing.RevokedAtMs == nil {
			if scopesAllow(requested, existing.Scopes) {
				return existing, nil
			}
		}
	}

	now := time.Now().UnixMilli()
	var createdAt int64 = now
	var rotatedAt *int64
	var lastUsed *int64

	if device.Tokens != nil {
		if existing := device.Tokens[r]; existing != nil {
			createdAt = existing.CreatedAtMs
			rotatedAt = &now
			lastUsed = existing.LastUsedAtMs
		}
	}

	next := &DeviceAuthToken{
		Token:        newDeviceToken(),
		Role:         r,
		Scopes:       requested,
		CreatedAtMs:  createdAt,
		RotatedAtMs:  rotatedAt,
		LastUsedAtMs: lastUsed,
	}

	if device.Tokens == nil {
		device.Tokens = make(map[string]*DeviceAuthToken)
	}
	device.Tokens[r] = next

	if err := persistPairingState(state, baseDir); err != nil {
		return nil, err
	}

	return next, nil
}

// RotateDeviceToken 轮换设备 token。
func RotateDeviceToken(deviceID, role string, scopes []string, baseDir string) (*DeviceAuthToken, error) {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(baseDir)
	if err != nil {
		return nil, err
	}

	device := state.PairedByDeviceID[normalizeDeviceID(deviceID)]
	if device == nil {
		return nil, nil
	}

	r := normalizeRole(role)
	if r == "" {
		return nil, nil
	}

	var existing *DeviceAuthToken
	if device.Tokens != nil {
		existing = device.Tokens[r]
	}

	// scopes 为 nil 时使用 existing 或 device 级别 scopes
	resolvedScopes := scopes
	if resolvedScopes == nil {
		if existing != nil {
			resolvedScopes = existing.Scopes
		} else {
			resolvedScopes = device.Scopes
		}
	}
	requested := normalizeScopes(resolvedScopes)

	now := time.Now().UnixMilli()
	var createdAt int64 = now
	var lastUsed *int64

	if existing != nil {
		createdAt = existing.CreatedAtMs
		lastUsed = existing.LastUsedAtMs
	}

	next := &DeviceAuthToken{
		Token:        newDeviceToken(),
		Role:         r,
		Scopes:       requested,
		CreatedAtMs:  createdAt,
		RotatedAtMs:  &now,
		LastUsedAtMs: lastUsed,
	}

	if device.Tokens == nil {
		device.Tokens = make(map[string]*DeviceAuthToken)
	}
	device.Tokens[r] = next

	// 如果显式传了 scopes，更新 device 级别
	if scopes != nil {
		device.Scopes = requested
	}

	if err := persistPairingState(state, baseDir); err != nil {
		return nil, err
	}

	return next, nil
}

// RevokeDeviceToken 撤销设备 token。
func RevokeDeviceToken(deviceID, role, baseDir string) (*DeviceAuthToken, error) {
	devicePairingMu.Lock()
	defer devicePairingMu.Unlock()

	state, err := loadPairingState(baseDir)
	if err != nil {
		return nil, err
	}

	device := state.PairedByDeviceID[normalizeDeviceID(deviceID)]
	if device == nil {
		return nil, nil
	}

	r := normalizeRole(role)
	if r == "" {
		return nil, nil
	}

	if device.Tokens == nil || device.Tokens[r] == nil {
		return nil, nil
	}

	now := time.Now().UnixMilli()
	entry := *device.Tokens[r]
	entry.RevokedAtMs = &now
	device.Tokens[r] = &entry

	if err := persistPairingState(state, baseDir); err != nil {
		return nil, err
	}

	return &entry, nil
}
