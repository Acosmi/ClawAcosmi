package security

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------- CountBySeverity ----------

func TestCountBySeverity(t *testing.T) {
	findings := []SecurityAuditFinding{
		{Severity: SeverityCritical},
		{Severity: SeverityCritical},
		{Severity: SeverityWarn},
		{Severity: SeverityInfo},
		{Severity: SeverityInfo},
		{Severity: SeverityInfo},
	}
	s := CountBySeverity(findings)
	if s.Critical != 2 || s.Warn != 1 || s.Info != 3 {
		t.Errorf("CountBySeverity = %+v, want critical=2 warn=1 info=3", s)
	}
}

func TestCountBySeverity_Empty(t *testing.T) {
	s := CountBySeverity(nil)
	if s.Critical != 0 || s.Warn != 0 || s.Info != 0 {
		t.Errorf("CountBySeverity(nil) = %+v, want all zeros", s)
	}
}

// ---------- normalizeAllowFromList ----------

func TestNormalizeAllowFromList(t *testing.T) {
	out := normalizeAllowFromList([]string{"  alice  ", "", "bob", " "})
	if len(out) != 2 || out[0] != "alice" || out[1] != "bob" {
		t.Errorf("normalizeAllowFromList = %v, want [alice bob]", out)
	}
}

// ---------- classifyChannelWarningSeverity ----------

func TestClassifyChannelWarningSeverity(t *testing.T) {
	tests := []struct {
		msg  string
		want SecurityAuditSeverity
	}{
		{`dms: open`, SeverityCritical},
		{`groupPolicy="open"`, SeverityCritical},
		{`allows any user`, SeverityCritical},
		{`anyone can dm`, SeverityCritical},
		{`public channel`, SeverityCritical},
		{`channel locked`, SeverityInfo},
		{`dm disabled`, SeverityInfo},
		{`some warning text`, SeverityWarn},
	}
	for _, tt := range tests {
		got := classifyChannelWarningSeverity(tt.msg)
		if got != tt.want {
			t.Errorf("classifyChannelWarningSeverity(%q) = %q, want %q", tt.msg, got, tt.want)
		}
	}
}

// ---------- isProbablySyncedPath ----------

func TestIsProbablySyncedPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/Users/foo/Library/Mobile Documents/iCloud~com.app/state", true},
		{"/Users/foo/Dropbox/openacosmi", true},
		{"/Users/foo/Google Drive/state", true},
		{"/Users/foo/OneDrive/openacosmi", true},
		{"/Users/foo/.openacosmi", false},
		{"/home/user/.config/openacosmi", false},
	}
	for _, tt := range tests {
		got := isProbablySyncedPath(tt.path)
		if got != tt.want {
			t.Errorf("isProbablySyncedPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// ---------- CollectSyncedFolderFindings ----------

func TestCollectSyncedFolderFindings_Detected(t *testing.T) {
	findings := CollectSyncedFolderFindings("/Users/x/Dropbox/state", "/local/config.json")
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].CheckID != "fs.synced_dir" {
		t.Errorf("checkId = %q, want fs.synced_dir", findings[0].CheckID)
	}
}

func TestCollectSyncedFolderFindings_Clean(t *testing.T) {
	findings := CollectSyncedFolderFindings("/home/user/.openacosmi", "/home/user/.openacosmi/config.json")
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// ---------- looksLikeEnvRef ----------

func TestLooksLikeEnvRef(t *testing.T) {
	if !looksLikeEnvRef("${MY_SECRET}") {
		t.Error("expected true for ${MY_SECRET}")
	}
	if looksLikeEnvRef("plain-text") {
		t.Error("expected false for plain-text")
	}
	if looksLikeEnvRef("$NOT_REF") {
		t.Error("expected false for $NOT_REF")
	}
}

// ---------- CollectGatewayConfigFindings ----------

func TestCollectGatewayConfigFindings_BindNoAuth(t *testing.T) {
	gw := &GatewayConfigSnapshot{
		Bind:     "0.0.0.0",
		AuthMode: "none",
	}
	findings := CollectGatewayConfigFindings(gw)
	found := false
	for _, f := range findings {
		if f.CheckID == "gateway.bind_no_auth" {
			found = true
			if f.Severity != SeverityCritical {
				t.Errorf("severity = %q, want critical", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected gateway.bind_no_auth finding")
	}
}

func TestCollectGatewayConfigFindings_TokenTooShort(t *testing.T) {
	gw := &GatewayConfigSnapshot{
		Bind:             "loopback",
		AuthMode:         "token",
		AuthToken:        "short",
		ControlUIEnabled: true,
	}
	findings := CollectGatewayConfigFindings(gw)
	found := false
	for _, f := range findings {
		if f.CheckID == "gateway.token_too_short" {
			found = true
		}
	}
	if !found {
		t.Error("expected gateway.token_too_short finding")
	}
}

func TestCollectGatewayConfigFindings_TailscaleFunnel(t *testing.T) {
	gw := &GatewayConfigSnapshot{
		Bind:             "loopback",
		AuthMode:         "token",
		AuthToken:        "a-very-long-token-123456789",
		TailscaleMode:    "funnel",
		ControlUIEnabled: true,
	}
	findings := CollectGatewayConfigFindings(gw)
	found := false
	for _, f := range findings {
		if f.CheckID == "gateway.tailscale_funnel" {
			found = true
			if f.Severity != SeverityCritical {
				t.Errorf("severity = %q, want critical", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected gateway.tailscale_funnel finding")
	}
}

func TestCollectGatewayConfigFindings_Nil(t *testing.T) {
	findings := CollectGatewayConfigFindings(nil)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for nil, got %d", len(findings))
	}
}

// ---------- CollectLoggingFindings ----------

func TestCollectLoggingFindings_RedactOff(t *testing.T) {
	lc := &LoggingConfigSnapshot{RedactSensitive: "off"}
	findings := CollectLoggingFindings(lc)
	if len(findings) != 1 || findings[0].CheckID != "logging.redact_off" {
		t.Errorf("expected logging.redact_off finding, got %v", findings)
	}
}

func TestCollectLoggingFindings_RedactTools(t *testing.T) {
	lc := &LoggingConfigSnapshot{RedactSensitive: "tools"}
	findings := CollectLoggingFindings(lc)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// ---------- CollectElevatedFindings ----------

func TestCollectElevatedFindings_Wildcard(t *testing.T) {
	ec := &ElevatedConfigSnapshot{
		Enabled: true,
		AllowFrom: map[string][]string{
			"discord": {"*"},
		},
	}
	findings := CollectElevatedFindings(ec)
	found := false
	for _, f := range findings {
		if f.CheckID == "tools.elevated.allowFrom.discord.wildcard" {
			found = true
			if f.Severity != SeverityCritical {
				t.Errorf("severity = %q, want critical", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected wildcard finding")
	}
}

func TestCollectElevatedFindings_LargeList(t *testing.T) {
	// 生成 30 个条目
	list := make([]string, 30)
	for i := range list {
		list[i] = "user-" + string(rune('A'+i%26))
	}
	ec := &ElevatedConfigSnapshot{
		Enabled:   true,
		AllowFrom: map[string][]string{"slack": list},
	}
	findings := CollectElevatedFindings(ec)
	found := false
	for _, f := range findings {
		if f.CheckID == "tools.elevated.allowFrom.slack.large" {
			found = true
		}
	}
	if !found {
		t.Error("expected large list finding")
	}
}

// ---------- CollectHooksHardeningFindings ----------

func TestCollectHooksHardeningFindings_ShortToken(t *testing.T) {
	hc := &HooksConfigSnapshot{
		Enabled: true,
		Token:   "short",
		Path:    "/hooks",
	}
	findings := CollectHooksHardeningFindings(hc)
	found := false
	for _, f := range findings {
		if f.CheckID == "hooks.token_too_short" {
			found = true
		}
	}
	if !found {
		t.Error("expected hooks.token_too_short finding")
	}
}

func TestCollectHooksHardeningFindings_TokenReuse(t *testing.T) {
	token := "a-very-long-shared-token-1234567890"
	hc := &HooksConfigSnapshot{
		Enabled:      true,
		Token:        token,
		GatewayToken: token,
		Path:         "/hooks",
	}
	findings := CollectHooksHardeningFindings(hc)
	found := false
	for _, f := range findings {
		if f.CheckID == "hooks.token_reuse_gateway_token" {
			found = true
		}
	}
	if !found {
		t.Error("expected hooks.token_reuse_gateway_token finding")
	}
}

func TestCollectHooksHardeningFindings_RootPath(t *testing.T) {
	hc := &HooksConfigSnapshot{
		Enabled: true,
		Token:   "a-very-long-token-for-hooks-1234567890",
		Path:    "/",
	}
	findings := CollectHooksHardeningFindings(hc)
	found := false
	for _, f := range findings {
		if f.CheckID == "hooks.path_root" {
			found = true
			if f.Severity != SeverityCritical {
				t.Errorf("severity = %q, want critical", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected hooks.path_root finding")
	}
}

// ---------- CollectModelHygieneFindings ----------

func TestCollectModelHygieneFindings_Legacy(t *testing.T) {
	models := []ModelRef{
		{ID: "gpt-3.5-turbo", Source: "agents.defaults.model.primary"},
	}
	findings := CollectModelHygieneFindings(models)
	foundLegacy := false
	for _, f := range findings {
		if f.CheckID == "models.legacy" {
			foundLegacy = true
		}
	}
	if !foundLegacy {
		t.Error("expected models.legacy finding for gpt-3.5-turbo")
	}
}

func TestCollectModelHygieneFindings_WeakTier(t *testing.T) {
	models := []ModelRef{
		{ID: "claude-3-haiku-20240307", Source: "agents.defaults.model.primary"},
	}
	findings := CollectModelHygieneFindings(models)
	foundWeak := false
	for _, f := range findings {
		if f.CheckID == "models.weak_tier" {
			foundWeak = true
		}
	}
	if !foundWeak {
		t.Error("expected models.weak_tier finding for haiku model")
	}
}

func TestCollectModelHygieneFindings_Modern(t *testing.T) {
	models := []ModelRef{
		{ID: "claude-opus-4-5-20250520", Source: "agents.defaults.model.primary"},
	}
	findings := CollectModelHygieneFindings(models)
	// 应当没有遗留模型警告
	for _, f := range findings {
		if f.CheckID == "models.legacy" {
			t.Error("should not flag modern model as legacy")
		}
	}
}

func TestCollectModelHygieneFindings_Empty(t *testing.T) {
	findings := CollectModelHygieneFindings(nil)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for nil, got %d", len(findings))
	}
}

// ---------- CollectSecretsInConfigFindings ----------

func TestCollectSecretsInConfig_PasswordInConfig(t *testing.T) {
	gw := &GatewayConfigSnapshot{
		AuthPassword: "my-secret-password",
	}
	findings := CollectSecretsInConfigFindings(gw, nil)
	found := false
	for _, f := range findings {
		if f.CheckID == "config.secrets.gateway_password_in_config" {
			found = true
		}
	}
	if !found {
		t.Error("expected gateway_password_in_config finding")
	}
}

func TestCollectSecretsInConfig_EnvRef(t *testing.T) {
	gw := &GatewayConfigSnapshot{
		AuthPassword: "${GATEWAY_PASSWORD}",
	}
	findings := CollectSecretsInConfigFindings(gw, nil)
	for _, f := range findings {
		if f.CheckID == "config.secrets.gateway_password_in_config" {
			t.Error("should not flag env ref as leaked secret")
		}
	}
}

// ---------- CollectFilesystemFindings ----------

func TestCollectFilesystemFindings_WorldWritable(t *testing.T) {
	// 创建临时目录并设置 world-writable 权限
	tmp := t.TempDir()
	worldWritableDir := filepath.Join(tmp, "state")
	if err := os.MkdirAll(worldWritableDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// os.MkdirAll respects umask, so explicitly chmod to world-writable
	if err := os.Chmod(worldWritableDir, 0o777); err != nil {
		t.Fatal(err)
	}
	// 创建临时配置文件
	configPath := filepath.Join(tmp, "config.json")
	if err := os.WriteFile(configPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	findings := CollectFilesystemFindings(worldWritableDir, configPath)
	found := false
	for _, f := range findings {
		if f.CheckID == "fs.state_dir.perms_world_writable" {
			found = true
			if f.Severity != SeverityCritical {
				t.Errorf("severity = %q, want critical", f.Severity)
			}
		}
	}
	if !found {
		t.Error("expected fs.state_dir.perms_world_writable finding")
	}
}

// ---------- CollectBrowserControlFindings ----------

func TestCollectBrowserControlFindings_RemoteHTTPCDP(t *testing.T) {
	bc := &BrowserConfigSnapshot{
		Enabled: true,
		Profiles: []BrowserProfileSnapshot{
			{Name: "remote", CDPUrl: "http://192.168.1.1:9222", IsLoopback: false},
		},
	}
	findings := CollectBrowserControlFindings(bc)
	found := false
	for _, f := range findings {
		if f.CheckID == "browser.remote_cdp_http" {
			found = true
		}
	}
	if !found {
		t.Error("expected browser.remote_cdp_http finding")
	}
}

func TestCollectBrowserControlFindings_LoopbackSkipped(t *testing.T) {
	bc := &BrowserConfigSnapshot{
		Enabled: true,
		Profiles: []BrowserProfileSnapshot{
			{Name: "local", CDPUrl: "http://localhost:9222", IsLoopback: true},
		},
	}
	findings := CollectBrowserControlFindings(bc)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for loopback, got %d", len(findings))
	}
}

// ---------- RunSecurityAudit (integration) ----------

func TestRunSecurityAudit_Default(t *testing.T) {
	report, err := RunSecurityAudit(SecurityAuditOptions{
		StateDir:   "/tmp/test-state",
		ConfigPath: "/tmp/test-config.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	if report.Timestamp == 0 {
		t.Error("timestamp should be set")
	}
	// 至少应有攻击面汇总
	foundSummary := false
	for _, f := range report.Findings {
		if f.CheckID == "summary.attack_surface" {
			foundSummary = true
		}
	}
	if !foundSummary {
		t.Error("expected summary.attack_surface finding in default audit")
	}
}

func TestRunSecurityAudit_WithAllChecks(t *testing.T) {
	report, err := RunSecurityAudit(SecurityAuditOptions{
		StateDir:          "/tmp/test-state",
		ConfigPath:        "/tmp/test-config.json",
		IncludeFilesystem: true,
		GatewayConfig: &GatewayConfigSnapshot{
			Bind:     "0.0.0.0",
			AuthMode: "none",
		},
		LoggingConfig: &LoggingConfigSnapshot{
			RedactSensitive: "off",
		},
		HooksConfig: &HooksConfigSnapshot{
			Enabled: true,
			Token:   "short",
			Path:    "/",
		},
		ModelRefs: []ModelRef{
			{ID: "gpt-3.5-turbo", Source: "defaults"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Critical == 0 {
		t.Error("expected critical findings with insecure config")
	}
}
