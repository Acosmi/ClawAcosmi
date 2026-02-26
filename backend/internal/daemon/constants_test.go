package daemon

import "testing"

func TestNormalizeGatewayProfile(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"default keyword", "default", ""},
		{"default uppercase", "DEFAULT", ""},
		{"default mixed case", "Default", ""},
		{"whitespace", "  ", ""},
		{"custom profile", "myprofile", "myprofile"},
		{"custom with spaces", "  myprofile  ", "myprofile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeGatewayProfile(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeGatewayProfile(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveGatewayProfileSuffix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"default", ""},
		{"myprofile", "-myprofile"},
	}
	for _, tt := range tests {
		result := ResolveGatewayProfileSuffix(tt.input)
		if result != tt.expected {
			t.Errorf("ResolveGatewayProfileSuffix(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestResolveGatewayLaunchAgentLabel(t *testing.T) {
	tests := []struct {
		profile  string
		expected string
	}{
		{"", "ai.openacosmi.gateway"},
		{"default", "ai.openacosmi.gateway"},
		{"staging", "ai.openacosmi.staging"},
	}
	for _, tt := range tests {
		result := ResolveGatewayLaunchAgentLabel(tt.profile)
		if result != tt.expected {
			t.Errorf("ResolveGatewayLaunchAgentLabel(%q) = %q, want %q", tt.profile, result, tt.expected)
		}
	}
}

func TestResolveGatewaySystemdServiceName(t *testing.T) {
	tests := []struct {
		profile  string
		expected string
	}{
		{"", "openacosmi-gateway"},
		{"default", "openacosmi-gateway"},
		{"staging", "openacosmi-gateway-staging"},
	}
	for _, tt := range tests {
		result := ResolveGatewaySystemdServiceName(tt.profile)
		if result != tt.expected {
			t.Errorf("ResolveGatewaySystemdServiceName(%q) = %q, want %q", tt.profile, result, tt.expected)
		}
	}
}

func TestResolveGatewayWindowsTaskName(t *testing.T) {
	tests := []struct {
		profile  string
		expected string
	}{
		{"", "OpenAcosmi Gateway"},
		{"default", "OpenAcosmi Gateway"},
		{"staging", "OpenAcosmi Gateway (staging)"},
	}
	for _, tt := range tests {
		result := ResolveGatewayWindowsTaskName(tt.profile)
		if result != tt.expected {
			t.Errorf("ResolveGatewayWindowsTaskName(%q) = %q, want %q", tt.profile, result, tt.expected)
		}
	}
}

func TestFormatGatewayServiceDescription(t *testing.T) {
	tests := []struct {
		profile  string
		version  string
		expected string
	}{
		{"", "", "OpenAcosmi Gateway"},
		{"", "1.2.3", "OpenAcosmi Gateway (v1.2.3)"},
		{"staging", "", "OpenAcosmi Gateway (profile: staging)"},
		{"staging", "1.2.3", "OpenAcosmi Gateway (profile: staging, v1.2.3)"},
	}
	for _, tt := range tests {
		result := FormatGatewayServiceDescription(tt.profile, tt.version)
		if result != tt.expected {
			t.Errorf("FormatGatewayServiceDescription(%q, %q) = %q, want %q", tt.profile, tt.version, result, tt.expected)
		}
	}
}

func TestNeedsNodeRuntimeMigration(t *testing.T) {
	tests := []struct {
		name     string
		issues   []ServiceConfigIssue
		expected bool
	}{
		{"no issues", nil, false},
		{"unrelated issue", []ServiceConfigIssue{{Code: AuditCodeGatewayPathMissing}}, false},
		{"bun runtime", []ServiceConfigIssue{{Code: AuditCodeGatewayRuntimeBun}}, true},
		{"version manager", []ServiceConfigIssue{{Code: AuditCodeGatewayRuntimeNodeVersionManager}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NeedsNodeRuntimeMigration(tt.issues)
			if result != tt.expected {
				t.Errorf("NeedsNodeRuntimeMigration() = %v, want %v", result, tt.expected)
			}
		})
	}
}
