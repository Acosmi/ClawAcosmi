package daemon

import "testing"

func TestGetMinimalServicePathParts(t *testing.T) {
	tests := []struct {
		name    string
		opts    MinimalServicePathOptions
		wantLen int // 最小预期长度
		wantNil bool
	}{
		{
			"darwin returns 4 system dirs",
			MinimalServicePathOptions{Platform: "darwin"},
			4,
			false,
		},
		{
			"linux returns 3 system dirs",
			MinimalServicePathOptions{Platform: "linux"},
			3,
			false,
		},
		{
			"windows returns nil",
			MinimalServicePathOptions{Platform: "windows"},
			0,
			true,
		},
		{
			"darwin with extra dirs",
			MinimalServicePathOptions{
				Platform:  "darwin",
				ExtraDirs: []string{"/custom/bin"},
			},
			5,
			false,
		},
		{
			"linux with home adds user dirs",
			MinimalServicePathOptions{
				Platform: "linux",
				Home:     "/home/testuser",
			},
			10, // 3 system + ~7 user dirs
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMinimalServicePathParts(tt.opts)
			if tt.wantNil {
				if result != nil {
					t.Errorf("GetMinimalServicePathParts() = %v, want nil", result)
				}
				return
			}
			if len(result) < tt.wantLen {
				t.Errorf("GetMinimalServicePathParts() len = %d, want >= %d, got %v", len(result), tt.wantLen, result)
			}
		})
	}
}

func TestBuildMinimalServicePath(t *testing.T) {
	result := BuildMinimalServicePath(MinimalServicePathOptions{
		Platform: "darwin",
	})
	if result == "" {
		t.Error("BuildMinimalServicePath(darwin) returned empty string")
	}
	// 应该包含 : 分隔符
	if len(result) > 0 {
		parts := splitPath(result)
		if len(parts) < 2 {
			t.Errorf("BuildMinimalServicePath(darwin) should have multiple parts, got %d", len(parts))
		}
	}
}

func TestBuildServiceEnvironment(t *testing.T) {
	env := map[string]string{
		"HOME":               "/home/testuser",
		"OPENACOSMI_PROFILE": "",
	}
	result := BuildServiceEnvironment(env, 8080, "test-token")

	if result["OPENACOSMI_GATEWAY_PORT"] != "8080" {
		t.Errorf("port = %q, want 8080", result["OPENACOSMI_GATEWAY_PORT"])
	}
	if result["OPENACOSMI_GATEWAY_TOKEN"] != "test-token" {
		t.Errorf("token = %q, want test-token", result["OPENACOSMI_GATEWAY_TOKEN"])
	}
	if result["OPENACOSMI_SERVICE_MARKER"] != GatewayServiceMarker {
		t.Errorf("marker = %q, want %q", result["OPENACOSMI_SERVICE_MARKER"], GatewayServiceMarker)
	}
	if result["OPENACOSMI_SERVICE_KIND"] != GatewayServiceKind {
		t.Errorf("kind = %q, want %q", result["OPENACOSMI_SERVICE_KIND"], GatewayServiceKind)
	}
}

func TestBuildNodeServiceEnvironment(t *testing.T) {
	env := map[string]string{
		"HOME": "/home/testuser",
	}
	result := BuildNodeServiceEnvironment(env)

	if result["OPENACOSMI_SERVICE_KIND"] != NodeServiceKind {
		t.Errorf("kind = %q, want %q", result["OPENACOSMI_SERVICE_KIND"], NodeServiceKind)
	}
	if result["OPENACOSMI_LOG_PREFIX"] != "node" {
		t.Errorf("log prefix = %q, want node", result["OPENACOSMI_LOG_PREFIX"])
	}
	if result["OPENACOSMI_LAUNCHD_LABEL"] != NodeLaunchAgentLabel {
		t.Errorf("label = %q, want %q", result["OPENACOSMI_LAUNCHD_LABEL"], NodeLaunchAgentLabel)
	}
}

func TestWithNodeServiceEnv(t *testing.T) {
	env := map[string]string{"EXISTING": "value"}
	result := WithNodeServiceEnv(env)

	if result["EXISTING"] != "value" {
		t.Error("WithNodeServiceEnv should preserve existing entries")
	}
	if result["OPENACOSMI_SERVICE_MARKER"] != NodeServiceMarker {
		t.Error("WithNodeServiceEnv should inject node service marker")
	}
	if result["OPENACOSMI_SERVICE_KIND"] != NodeServiceKind {
		t.Error("WithNodeServiceEnv should inject node service kind")
	}
}

func TestIsNodeRuntime(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/usr/bin/node", true},
		{"/usr/bin/node.exe", true},
		{"/usr/bin/bun", false},
		{"/usr/bin/python", false},
	}
	for _, tt := range tests {
		if result := IsNodeRuntime(tt.path); result != tt.expected {
			t.Errorf("IsNodeRuntime(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestIsBunRuntime(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/usr/bin/bun", true},
		{"/usr/bin/bun.exe", true},
		{"/usr/bin/node", false},
	}
	for _, tt := range tests {
		if result := IsBunRuntime(tt.path); result != tt.expected {
			t.Errorf("IsBunRuntime(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestIsVersionManagedNodePath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/usr/bin/node", false},
		{"/home/user/.nvm/versions/node/v22/bin/node", true},
		{"/home/user/.fnm/node-versions/v22/bin/node", true},
		{"/home/user/.volta/bin/node", true},
		{"/home/user/.asdf/shims/node", true},
	}
	for _, tt := range tests {
		if result := IsVersionManagedNodePath(tt.path, "linux"); result != tt.expected {
			t.Errorf("IsVersionManagedNodePath(%q) = %v, want %v", tt.path, result, tt.expected)
		}
	}
}

func TestHasGatewaySubcommand(t *testing.T) {
	if !HasGatewaySubcommand([]string{"node", "gateway", "--port", "8080"}) {
		t.Error("should detect 'gateway' subcommand")
	}
	if HasGatewaySubcommand([]string{"node", "--port", "8080"}) {
		t.Error("should not detect gateway when absent")
	}
}
