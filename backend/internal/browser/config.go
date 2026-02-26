package browser

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ResolvedBrowserConfig is the fully-resolved browser configuration.
type ResolvedBrowserConfig struct {
	Enabled          bool                               `json:"enabled"`
	Profiles         map[string]*ResolvedBrowserProfile `json:"profiles,omitempty"`
	DefaultProfile   string                             `json:"defaultProfile,omitempty"`
	CustomExecutable string                             `json:"customExecutable,omitempty"`
}

// ResolvedBrowserProfile is the resolved config for a single browser profile.
type ResolvedBrowserProfile struct {
	Name     string `json:"name"`
	CDPPort  int    `json:"cdpPort"`
	CDPURL   string `json:"cdpUrl,omitempty"`
	Color    string `json:"color"`
	DataDir  string `json:"dataDir,omitempty"`
	Headless bool   `json:"headless"`
}

// BrowserConfigInput is the raw user config for the browser module.
type BrowserConfigInput struct {
	Enabled    *bool                    `json:"enabled,omitempty"`
	Profiles   map[string]*ProfileInput `json:"profiles,omitempty"`
	Default    string                   `json:"default,omitempty"`
	Executable string                   `json:"executable,omitempty"`
}

// ProfileInput is the raw user config for a single profile.
type ProfileInput struct {
	CDPPort  *int   `json:"cdpPort,omitempty"`
	CDPURL   string `json:"cdpUrl,omitempty"`
	Color    string `json:"color,omitempty"`
	DataDir  string `json:"dataDir,omitempty"`
	Headless *bool  `json:"headless,omitempty"`
}

// ResolveBrowserConfig resolves raw browser config into final form.
func ResolveBrowserConfig(input *BrowserConfigInput) *ResolvedBrowserConfig {
	cfg := &ResolvedBrowserConfig{
		Enabled:  true,
		Profiles: make(map[string]*ResolvedBrowserProfile),
	}

	if input != nil && input.Enabled != nil {
		cfg.Enabled = *input.Enabled
	}
	if input != nil && strings.TrimSpace(input.Executable) != "" {
		cfg.CustomExecutable = strings.TrimSpace(input.Executable)
	}
	if input == nil || !cfg.Enabled {
		return cfg
	}

	usedPorts := make(map[int]struct{})
	usedColors := make(map[string]struct{})

	for name, pi := range input.Profiles {
		if !IsValidProfileName(name) {
			continue
		}
		profile := &ResolvedBrowserProfile{
			Name:  name,
			Color: "#FF4500", // default
		}

		if pi.CDPPort != nil && *pi.CDPPort > 0 {
			profile.CDPPort = *pi.CDPPort
		} else {
			port, err := AllocateCDPPort(usedPorts, nil)
			if err == nil {
				profile.CDPPort = port
			}
		}
		usedPorts[profile.CDPPort] = struct{}{}

		if strings.TrimSpace(pi.CDPURL) != "" {
			profile.CDPURL = strings.TrimSpace(pi.CDPURL)
		}
		if strings.TrimSpace(pi.Color) != "" {
			profile.Color = strings.TrimSpace(pi.Color)
		} else {
			profile.Color = AllocateColor(usedColors)
		}
		usedColors[strings.ToUpper(profile.Color)] = struct{}{}

		if strings.TrimSpace(pi.DataDir) != "" {
			profile.DataDir = strings.TrimSpace(pi.DataDir)
		}
		if pi.Headless != nil {
			profile.Headless = *pi.Headless
		}

		cfg.Profiles[name] = profile
	}

	if input.Default != "" {
		cfg.DefaultProfile = input.Default
	} else if len(cfg.Profiles) == 1 {
		for name := range cfg.Profiles {
			cfg.DefaultProfile = name
		}
	}

	return cfg
}

// ParsedHTTPURL holds a parsed and validated HTTP(S) URL.
type ParsedHTTPURL struct {
	URL        *url.URL
	Port       int
	Normalized string
}

// ParseHttpURL parses and validates an HTTP(S) URL, extracting port info.
// Aligns with TS config.ts parseHttpUrl().
func ParseHttpURL(raw string, label string) (*ParsedHTTPURL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%s: URL is empty", label)
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid URL %q: %w", label, raw, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("%s: expected http or https, got %q", label, parsed.Scheme)
	}

	port := 0
	if p := parsed.Port(); p != "" {
		port, _ = strconv.Atoi(p)
	}
	if port <= 0 {
		if parsed.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	if port > 65535 {
		return nil, fmt.Errorf("%s: port %d out of range", label, port)
	}

	normalized := strings.TrimRight(parsed.String(), "/")

	return &ParsedHTTPURL{
		URL:        parsed,
		Port:       port,
		Normalized: normalized,
	}, nil
}

// NOTE: IsLoopbackHost is defined in cdp_helpers.go (shared by config, cdp, chrome).

// ResolveProfile looks up a profile by name and returns it, or nil if not found.
// Aligns with TS config.ts resolveProfile().
func ResolveProfile(cfg *ResolvedBrowserConfig, profileName string) *ResolvedBrowserProfile {
	if cfg == nil || cfg.Profiles == nil {
		return nil
	}
	name := strings.TrimSpace(profileName)
	if name == "" {
		name = cfg.DefaultProfile
	}
	if name == "" {
		return nil
	}
	return cfg.Profiles[name]
}

// DefaultProfileName is the default browser profile name.
const DefaultProfileName = "openacosmi"

// DefaultChromeExtensionProfileName is the extension relay profile name.
const DefaultChromeExtensionProfileName = "chrome"

// EnsureDefaultProfile ensures the default "openacosmi" profile exists.
// Aligns with TS config.ts ensureDefaultProfile().
func EnsureDefaultProfile(profiles map[string]*ProfileInput, defaultColor string) map[string]*ProfileInput {
	if profiles == nil {
		profiles = make(map[string]*ProfileInput)
	}
	if _, ok := profiles[DefaultProfileName]; !ok {
		port := CDPPortRangeStart
		profiles[DefaultProfileName] = &ProfileInput{
			CDPPort: &port,
			Color:   defaultColor,
		}
	}
	return profiles
}

// EnsureDefaultChromeExtensionProfile ensures the "chrome" extension relay profile exists.
// Aligns with TS config.ts ensureDefaultChromeExtensionProfile().
func EnsureDefaultChromeExtensionProfile(profiles map[string]*ProfileInput, controlPort int) map[string]*ProfileInput {
	if profiles == nil {
		profiles = make(map[string]*ProfileInput)
	}
	if _, ok := profiles[DefaultChromeExtensionProfileName]; ok {
		return profiles
	}

	relayPort := controlPort + 1
	if relayPort <= 0 || relayPort > 65535 {
		return profiles
	}

	// Check if relayPort is already used
	for _, p := range profiles {
		if p.CDPPort != nil && *p.CDPPort == relayPort {
			return profiles
		}
	}

	profiles[DefaultChromeExtensionProfileName] = &ProfileInput{
		CDPURL: fmt.Sprintf("http://127.0.0.1:%d", relayPort),
		Color:  "#00AA00",
	}
	return profiles
}

// NormalizeHexColor validates and normalizes a hex color string.
func NormalizeHexColor(raw string) string {
	s := strings.TrimSpace(raw)
	if len(s) == 7 && s[0] == '#' {
		for _, c := range s[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return "#FF4500"
			}
		}
		return strings.ToUpper(s)
	}
	return "#FF4500"
}

// EnsurePortAvailable checks if a port is available for listening.
func EnsurePortAvailable(port int) error {
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return err
	}
	ln.Close()
	return nil
}
