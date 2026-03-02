package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/openacosmi/claw-acismi/internal/config"
)

// ---------- system.backup.list ----------

func TestBackupList_NoBackups(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "openacosmi.json")
	os.WriteFile(cfgPath, []byte(`{"foo":"bar"}`), 0600)

	loader := config.NewConfigLoader(config.WithConfigPath(cfgPath))
	r := NewMethodRegistry()
	r.RegisterAll(SystemResetHandlers())

	req := &RequestFrame{Method: "system.backup.list", Params: map[string]interface{}{}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{ConfigLoader: loader}, respond)
	if !gotOK {
		t.Fatal("system.backup.list should succeed")
	}
	result := gotPayload.(map[string]interface{})
	backups := result["backups"]
	// No backups exist yet — should be nil or empty
	if backups != nil {
		if arr, ok := backups.([]backupEntry); ok && len(arr) > 0 {
			t.Errorf("expected empty backups, got %d", len(arr))
		}
	}
}

func TestBackupList_WithBackups(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "openacosmi.json")
	os.WriteFile(cfgPath, []byte(`{"current":true}`), 0600)

	// Create .bak and .bak.1
	os.WriteFile(cfgPath+".bak", []byte(`{"backup":0}`), 0600)
	os.WriteFile(cfgPath+".bak.1", []byte(`not json`), 0600)

	loader := config.NewConfigLoader(config.WithConfigPath(cfgPath))
	r := NewMethodRegistry()
	r.RegisterAll(SystemResetHandlers())

	req := &RequestFrame{Method: "system.backup.list", Params: map[string]interface{}{}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{ConfigLoader: loader}, respond)
	if !gotOK {
		t.Fatal("system.backup.list should succeed")
	}
	result := gotPayload.(map[string]interface{})
	backups := result["backups"].([]backupEntry)
	if len(backups) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(backups))
	}

	// First backup (index 0) should be valid JSON
	if !backups[0].Valid {
		t.Error("backup 0 should be valid JSON")
	}
	// Second backup (index 1) should be invalid
	if backups[1].Valid {
		t.Error("backup 1 should be invalid JSON")
	}
}

// ---------- system.backup.restore ----------

func TestBackupRestore_Success(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "openacosmi.json")
	os.WriteFile(cfgPath, []byte(`{"broken":true}`), 0600)
	os.WriteFile(cfgPath+".bak", []byte(`{"restored":true}`), 0600)

	loader := config.NewConfigLoader(config.WithConfigPath(cfgPath))
	broadcaster := NewBroadcaster()
	r := NewMethodRegistry()
	r.RegisterAll(SystemResetHandlers())

	req := &RequestFrame{Method: "system.backup.restore", Params: map[string]interface{}{"index": float64(0)}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{
		ConfigLoader: loader,
		Broadcaster:  broadcaster,
	}, respond)
	if !gotOK {
		t.Fatal("system.backup.restore should succeed")
	}
	result := gotPayload.(map[string]interface{})
	if result["ok"] != true {
		t.Error("expected ok=true")
	}

	// Verify config file was overwritten
	data, _ := os.ReadFile(cfgPath)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)
	if parsed["restored"] != true {
		t.Errorf("config should contain restored data, got: %s", string(data))
	}
}

func TestBackupRestore_MissingIndex(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "openacosmi.json")
	os.WriteFile(cfgPath, []byte(`{}`), 0600)

	loader := config.NewConfigLoader(config.WithConfigPath(cfgPath))
	r := NewMethodRegistry()
	r.RegisterAll(SystemResetHandlers())

	req := &RequestFrame{Method: "system.backup.restore", Params: map[string]interface{}{}}
	var gotOK bool
	respond := func(ok bool, _ interface{}, _ *ErrorShape) {
		gotOK = ok
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{ConfigLoader: loader}, respond)
	if gotOK {
		t.Fatal("should fail without index")
	}
}

func TestBackupRestore_BackupNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "openacosmi.json")
	os.WriteFile(cfgPath, []byte(`{}`), 0600)
	// No .bak file

	loader := config.NewConfigLoader(config.WithConfigPath(cfgPath))
	r := NewMethodRegistry()
	r.RegisterAll(SystemResetHandlers())

	req := &RequestFrame{Method: "system.backup.restore", Params: map[string]interface{}{"index": float64(0)}}
	var gotOK bool
	respond := func(ok bool, _ interface{}, _ *ErrorShape) {
		gotOK = ok
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{ConfigLoader: loader}, respond)
	if gotOK {
		t.Fatal("should fail when backup not found")
	}
}

// ---------- system.reset.preview ----------

func TestResetPreview(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create files that would be reset
	ocDir := filepath.Join(tmpHome, ".openacosmi")
	os.MkdirAll(filepath.Join(ocDir, "memory"), 0755)
	os.WriteFile(filepath.Join(ocDir, "exec-approvals.json"), []byte(`{}`), 0600)
	os.WriteFile(filepath.Join(ocDir, "escalation-audit.log"), []byte("log data"), 0600)
	os.WriteFile(filepath.Join(ocDir, "memory", "boot.json"), []byte(`{}`), 0600)

	r := NewMethodRegistry()
	r.RegisterAll(SystemResetHandlers())

	req := &RequestFrame{Method: "system.reset.preview", Params: map[string]interface{}{"level": float64(1)}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{}, respond)
	if !gotOK {
		t.Fatal("system.reset.preview should succeed")
	}
	result := gotPayload.(map[string]interface{})
	targets := result["targets"].([]resetFileEntry)
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
	// All should exist
	for _, tgt := range targets {
		if !tgt.Exists {
			t.Errorf("expected %s to exist", tgt.Path)
		}
	}
}

// ---------- system.reset ----------

func TestReset_Success(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	ocDir := filepath.Join(tmpHome, ".openacosmi")
	os.MkdirAll(filepath.Join(ocDir, "memory"), 0755)
	eaFile := filepath.Join(ocDir, "exec-approvals.json")
	auditFile := filepath.Join(ocDir, "escalation-audit.log")
	bootFile := filepath.Join(ocDir, "memory", "boot.json")
	os.WriteFile(eaFile, []byte(`{"rules":[]}`), 0600)
	os.WriteFile(auditFile, []byte("line1\nline2\n"), 0600)
	os.WriteFile(bootFile, []byte(`{"version":"0.2.0"}`), 0600)

	// Create an EscalationManager to test Reset()
	broadcaster := NewBroadcaster()
	auditLogger := &EscalationAuditLogger{filePath: auditFile}
	escMgr := NewEscalationManager(broadcaster, auditLogger, nil)

	r := NewMethodRegistry()
	r.RegisterAll(SystemResetHandlers())

	req := &RequestFrame{Method: "system.reset", Params: map[string]interface{}{"level": float64(1)}}
	var gotOK bool
	var gotPayload interface{}
	respond := func(ok bool, payload interface{}, _ *ErrorShape) {
		gotOK = ok
		gotPayload = payload
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{
		Broadcaster:   broadcaster,
		EscalationMgr: escMgr,
	}, respond)
	if !gotOK {
		t.Fatal("system.reset should succeed")
	}
	result := gotPayload.(map[string]interface{})
	if result["ok"] != true {
		t.Error("expected ok=true")
	}

	// exec-approvals.json should be deleted
	if _, err := os.Stat(eaFile); !os.IsNotExist(err) {
		t.Error("exec-approvals.json should be deleted")
	}

	// escalation-audit.log should be truncated (exists but empty)
	info, err := os.Stat(auditFile)
	if err != nil {
		t.Fatal("escalation-audit.log should still exist")
	}
	if info.Size() != 0 {
		t.Errorf("escalation-audit.log should be empty, got %d bytes", info.Size())
	}

	// boot.json should be deleted
	if _, err := os.Stat(bootFile); !os.IsNotExist(err) {
		t.Error("boot.json should be deleted")
	}

	// EscalationManager state should be cleared
	status := escMgr.GetStatus()
	if status.HasPending || status.HasActive {
		t.Error("escalation state should be cleared")
	}
}

func TestReset_NoFiles(t *testing.T) {
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// Create .openacosmi dir but no files
	os.MkdirAll(filepath.Join(tmpHome, ".openacosmi", "memory"), 0755)

	r := NewMethodRegistry()
	r.RegisterAll(SystemResetHandlers())

	req := &RequestFrame{Method: "system.reset", Params: map[string]interface{}{"level": float64(1)}}
	var gotOK bool
	respond := func(ok bool, _ interface{}, _ *ErrorShape) {
		gotOK = ok
	}
	HandleGatewayRequest(r, req, nil, &GatewayMethodContext{}, respond)
	if !gotOK {
		t.Fatal("system.reset should succeed even when no files exist")
	}
}
