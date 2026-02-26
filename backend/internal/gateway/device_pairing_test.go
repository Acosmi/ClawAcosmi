package gateway

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTempDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func TestRequestAndApproveDevicePairing(t *testing.T) {
	baseDir := setupTempDir(t)

	// 1. 发起配对请求
	result, err := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID:  "device-1",
		PublicKey: "pk-1",
		Role:      "operator",
		Scopes:    []string{"chat"},
	}, baseDir)
	if err != nil {
		t.Fatalf("RequestDevicePairing: %v", err)
	}
	if result.Status != "pending" {
		t.Errorf("status = %q, want pending", result.Status)
	}
	if !result.Created {
		t.Error("expected created = true")
	}

	// 2. 重复请求同一设备 → 返回已有请求
	result2, err := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID:  "device-1",
		PublicKey: "pk-1",
		Role:      "operator",
	}, baseDir)
	if err != nil {
		t.Fatalf("RequestDevicePairing duplicate: %v", err)
	}
	if result2.Created {
		t.Error("duplicate request should not create new")
	}
	if result2.Request.RequestID != result.Request.RequestID {
		t.Error("duplicate should return same request")
	}

	// 3. 批准
	approved, err := ApproveDevicePairing(result.Request.RequestID, baseDir)
	if err != nil {
		t.Fatalf("ApproveDevicePairing: %v", err)
	}
	if approved == nil {
		t.Fatal("expected approve result, got nil")
	}
	if approved.Device.DeviceID != "device-1" {
		t.Errorf("deviceId = %q, want device-1", approved.Device.DeviceID)
	}
	if approved.Device.Tokens == nil || approved.Device.Tokens["operator"] == nil {
		t.Error("expected operator token after approve")
	}

	// 4. 获取已配对设备
	paired, err := GetPairedDevice("device-1", baseDir)
	if err != nil {
		t.Fatalf("GetPairedDevice: %v", err)
	}
	if paired == nil {
		t.Fatal("expected paired device, got nil")
	}
	if paired.PublicKey != "pk-1" {
		t.Errorf("publicKey = %q, want pk-1", paired.PublicKey)
	}
}

func TestRejectDevicePairing(t *testing.T) {
	baseDir := setupTempDir(t)

	result, err := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID:  "device-2",
		PublicKey: "pk-2",
	}, baseDir)
	if err != nil {
		t.Fatal(err)
	}

	rejected, err := RejectDevicePairing(result.Request.RequestID, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if rejected == nil {
		t.Fatal("expected reject result")
	}
	if rejected.DeviceID != "device-2" {
		t.Errorf("deviceId = %q, want device-2", rejected.DeviceID)
	}

	// 列表中不应有 pending
	list, err := ListDevicePairing(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Pending) != 0 {
		t.Errorf("pending count = %d, want 0", len(list.Pending))
	}
}

func TestRejectNonExistent(t *testing.T) {
	baseDir := setupTempDir(t)
	result, err := RejectDevicePairing("nonexistent", baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Error("expected nil for nonexistent request")
	}
}

func TestListDevicePairing(t *testing.T) {
	baseDir := setupTempDir(t)

	// 发起 2 个请求
	r1, _ := RequestDevicePairing(DevicePairingRequestInput{DeviceID: "d1", PublicKey: "k1"}, baseDir)
	RequestDevicePairing(DevicePairingRequestInput{DeviceID: "d2", PublicKey: "k2", Role: "admin"}, baseDir)

	// 批准 d1
	ApproveDevicePairing(r1.Request.RequestID, baseDir)

	list, err := ListDevicePairing(baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Pending) != 1 {
		t.Errorf("pending = %d, want 1", len(list.Pending))
	}
	if len(list.Paired) != 1 {
		t.Errorf("paired = %d, want 1", len(list.Paired))
	}
}

func TestIsRepairFlag(t *testing.T) {
	baseDir := setupTempDir(t)

	r1, _ := RequestDevicePairing(DevicePairingRequestInput{DeviceID: "d1", PublicKey: "k1"}, baseDir)
	if r1.Request.IsRepair {
		t.Error("first request should not be repair")
	}
	ApproveDevicePairing(r1.Request.RequestID, baseDir)

	// 再次请求同一设备 → isRepair = true
	r2, _ := RequestDevicePairing(DevicePairingRequestInput{DeviceID: "d1", PublicKey: "k1-new"}, baseDir)
	if !r2.Request.IsRepair {
		t.Error("second request for same device should be repair")
	}
}

func TestVerifyDeviceToken(t *testing.T) {
	baseDir := setupTempDir(t)

	req, _ := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID: "d1", PublicKey: "k1", Role: "operator", Scopes: []string{"chat"},
	}, baseDir)
	approved, _ := ApproveDevicePairing(req.Request.RequestID, baseDir)
	token := approved.Device.Tokens["operator"].Token

	// 正常验证
	res, err := VerifyDeviceToken(VerifyDeviceTokenParams{
		DeviceID: "d1", Token: token, Role: "operator", Scopes: []string{"chat"}, BaseDir: baseDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("verify failed: %s", res.Reason)
	}

	// 错误 token
	res, _ = VerifyDeviceToken(VerifyDeviceTokenParams{
		DeviceID: "d1", Token: "wrong", Role: "operator", BaseDir: baseDir,
	})
	if res.OK {
		t.Error("wrong token should fail")
	}
	if res.Reason != "token-mismatch" {
		t.Errorf("reason = %q, want token-mismatch", res.Reason)
	}

	// 不存在的设备
	res, _ = VerifyDeviceToken(VerifyDeviceTokenParams{
		DeviceID: "nonexistent", Token: "x", Role: "r", BaseDir: baseDir,
	})
	if res.OK {
		t.Error("nonexistent device should fail")
	}
}

func TestRotateDeviceToken(t *testing.T) {
	baseDir := setupTempDir(t)

	req, _ := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID: "d1", PublicKey: "k1", Role: "operator", Scopes: []string{"admin"},
	}, baseDir)
	approved, _ := ApproveDevicePairing(req.Request.RequestID, baseDir)
	oldToken := approved.Device.Tokens["operator"].Token

	// 轮换 token 并指定新 scopes
	rotated, err := RotateDeviceToken("d1", "operator", []string{"read"}, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if rotated == nil {
		t.Fatal("expected rotated token")
	}
	if rotated.Token == oldToken {
		t.Error("rotated token should be different")
	}
	if rotated.RotatedAtMs == nil {
		t.Error("rotatedAtMs should be set")
	}

	// 验证 device.scopes 也更新了
	paired, _ := GetPairedDevice("d1", baseDir)
	if len(paired.Scopes) != 1 || paired.Scopes[0] != "read" {
		t.Errorf("device scopes = %v, want [read]", paired.Scopes)
	}
}

func TestRotateWithoutScopes(t *testing.T) {
	baseDir := setupTempDir(t)

	req, _ := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID: "d1", PublicKey: "k1", Role: "operator", Scopes: []string{"operator.admin"},
	}, baseDir)
	ApproveDevicePairing(req.Request.RequestID, baseDir)

	// 先用 scopes 轮换
	RotateDeviceToken("d1", "operator", []string{"operator.read"}, baseDir)

	// 再不传 scopes 轮换 → 应保留之前的 scopes
	rotated, _ := RotateDeviceToken("d1", "operator", nil, baseDir)
	if rotated == nil {
		t.Fatal("expected rotated token")
	}
	// 当 scopes 为 nil 时，应该从 existing token 继承
	found := false
	for _, s := range rotated.Scopes {
		if s == "operator.read" {
			found = true
		}
	}
	if !found {
		t.Errorf("scopes = %v, should inherit operator.read", rotated.Scopes)
	}
}

func TestRevokeDeviceToken(t *testing.T) {
	baseDir := setupTempDir(t)

	req, _ := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID: "d1", PublicKey: "k1", Role: "operator", Scopes: []string{"chat"},
	}, baseDir)
	approved, _ := ApproveDevicePairing(req.Request.RequestID, baseDir)
	token := approved.Device.Tokens["operator"].Token

	revoked, err := RevokeDeviceToken("d1", "operator", baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if revoked == nil {
		t.Fatal("expected revoked token")
	}
	if revoked.RevokedAtMs == nil {
		t.Error("revokedAtMs should be set")
	}

	// 验证已撤销的 token 不能通过验证
	res, _ := VerifyDeviceToken(VerifyDeviceTokenParams{
		DeviceID: "d1", Token: token, Role: "operator", BaseDir: baseDir,
	})
	if res.OK {
		t.Error("revoked token should fail verification")
	}
	if res.Reason != "token-revoked" {
		t.Errorf("reason = %q, want token-revoked", res.Reason)
	}
}

func TestEnsureDeviceToken(t *testing.T) {
	baseDir := setupTempDir(t)

	req, _ := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID: "d1", PublicKey: "k1", Role: "operator", Scopes: []string{"chat"},
	}, baseDir)
	approved, _ := ApproveDevicePairing(req.Request.RequestID, baseDir)
	existingToken := approved.Device.Tokens["operator"].Token

	// 已有有效 token → 返回现有 token
	ensured, err := EnsureDeviceToken("d1", "operator", []string{"chat"}, baseDir)
	if err != nil {
		t.Fatal(err)
	}
	if ensured.Token != existingToken {
		t.Error("ensureDeviceToken should return existing valid token")
	}

	// 不同 scope → 创建新 token
	ensured2, _ := EnsureDeviceToken("d1", "operator", []string{"chat", "admin"}, baseDir)
	if ensured2.Token == existingToken {
		t.Error("different scopes should create new token")
	}
}

func TestPruneExpiredPending(t *testing.T) {
	pending := map[string]*DevicePairingPendingRequest{
		"old":    {RequestID: "old", Ts: time.Now().UnixMilli() - 6*60*1000},
		"recent": {RequestID: "recent", Ts: time.Now().UnixMilli()},
	}
	pruneExpiredPending(pending, time.Now().UnixMilli())
	if _, ok := pending["old"]; ok {
		t.Error("old request should be pruned")
	}
	if _, ok := pending["recent"]; !ok {
		t.Error("recent request should remain")
	}
}

func TestSummarizeDeviceTokens(t *testing.T) {
	now := time.Now().UnixMilli()
	tokens := map[string]*DeviceAuthToken{
		"operator": {Token: "secret", Role: "operator", Scopes: []string{"chat"}, CreatedAtMs: now},
		"admin":    {Token: "secret2", Role: "admin", Scopes: []string{"all"}, CreatedAtMs: now},
	}
	summaries := SummarizeDeviceTokens(tokens)
	if len(summaries) != 2 {
		t.Fatalf("summaries = %d, want 2", len(summaries))
	}
	// 按 role 排序
	if summaries[0].Role != "admin" {
		t.Errorf("first role = %q, want admin", summaries[0].Role)
	}

	// nil tokens
	if SummarizeDeviceTokens(nil) != nil {
		t.Error("nil tokens should return nil")
	}
}

func TestUpdatePairedDeviceMetadata(t *testing.T) {
	baseDir := setupTempDir(t)

	req, _ := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID: "d1", PublicKey: "k1", Role: "operator",
	}, baseDir)
	ApproveDevicePairing(req.Request.RequestID, baseDir)

	err := UpdatePairedDeviceMetadata("d1", &PairedDeviceMetadataPatch{
		DisplayName: "My Phone",
		Platform:    "ios",
	}, baseDir)
	if err != nil {
		t.Fatal(err)
	}

	paired, _ := GetPairedDevice("d1", baseDir)
	if paired.DisplayName != "My Phone" {
		t.Errorf("displayName = %q, want My Phone", paired.DisplayName)
	}
	if paired.Platform != "ios" {
		t.Errorf("platform = %q, want ios", paired.Platform)
	}
}

func TestAtomicFileWrite(t *testing.T) {
	baseDir := setupTempDir(t)

	req, _ := RequestDevicePairing(DevicePairingRequestInput{
		DeviceID: "d1", PublicKey: "k1",
	}, baseDir)

	// 验证文件存在
	pendingPath := filepath.Join(baseDir, "devices", "pending.json")
	if _, err := os.Stat(pendingPath); os.IsNotExist(err) {
		t.Error("pending.json should exist after request")
	}

	ApproveDevicePairing(req.Request.RequestID, baseDir)

	pairedPath := filepath.Join(baseDir, "devices", "paired.json")
	if _, err := os.Stat(pairedPath); os.IsNotExist(err) {
		t.Error("paired.json should exist after approve")
	}

	// 验证文件权限
	info, _ := os.Stat(pairedPath)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("file permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestMergeRolesAndScopes(t *testing.T) {
	// mergeRoles
	roles := mergeRoles("admin", []string{"operator", "admin"}, "viewer")
	if len(roles) != 3 {
		t.Errorf("merged roles = %v, want 3 items", roles)
	}

	emptyRoles := mergeRoles("", []string{})
	if emptyRoles != nil {
		t.Errorf("empty merge should return nil, got %v", emptyRoles)
	}

	// mergeScopes
	scopes := mergeScopes([]string{"read", "write"}, []string{"read", "admin"})
	if len(scopes) != 3 {
		t.Errorf("merged scopes = %v, want 3 items", scopes)
	}

	// scopesAllow
	if !scopesAllow([]string{}, []string{"any"}) {
		t.Error("empty requested should be allowed")
	}
	if scopesAllow([]string{"admin"}, []string{}) {
		t.Error("no allowed should deny")
	}
	if !scopesAllow([]string{"read"}, []string{"read", "write"}) {
		t.Error("subset should be allowed")
	}
	if scopesAllow([]string{"admin"}, []string{"read"}) {
		t.Error("non-subset should be denied")
	}
}
