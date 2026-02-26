package browser

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ProfileNameRegex validates browser profile names.
var ProfileNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// IsValidProfileName checks a profile name against naming rules.
func IsValidProfileName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	return ProfileNameRegex.MatchString(name)
}

// PortRange defines a CDP port allocation range.
type PortRange struct {
	Start int
	End   int
}

// AllocateCDPPort finds the first unused port in the given range.
func AllocateCDPPort(usedPorts map[int]struct{}, portRange *PortRange) (int, error) {
	start := CDPPortRangeStart
	end := CDPPortRangeEnd
	if portRange != nil {
		start = portRange.Start
		end = portRange.End
	}
	if start <= 0 || end <= 0 || start > end {
		return 0, fmt.Errorf("invalid port range: %d-%d", start, end)
	}
	for port := start; port <= end; port++ {
		if _, used := usedPorts[port]; !used {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available CDP port in range %d-%d", start, end)
}

// ProfilePortInfo holds port info for a browser profile.
type ProfilePortInfo struct {
	CDPPort int
	CDPURL  string
}

// GetUsedPorts collects all ports used by existing profiles.
func GetUsedPorts(profiles map[string]ProfilePortInfo) map[int]struct{} {
	used := make(map[int]struct{})
	for _, p := range profiles {
		if p.CDPPort > 0 {
			used[p.CDPPort] = struct{}{}
			continue
		}
		raw := strings.TrimSpace(p.CDPURL)
		if raw == "" {
			continue
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}
		port := 0
		if parsed.Port() != "" {
			if v, err := strconv.Atoi(parsed.Port()); err == nil && v > 0 {
				port = v
			}
		} else if parsed.Scheme == "https" || parsed.Scheme == "wss" {
			port = 443
		} else {
			port = 80
		}
		if port > 0 && port <= 65535 {
			used[port] = struct{}{}
		}
	}
	return used
}

// ProfileColors is the default color palette for browser profiles.
var ProfileColors = []string{
	"#FF4500", // Orange-red (openacosmi default)
	"#0066CC", // Blue
	"#00AA00", // Green
	"#9933FF", // Purple
	"#FF6699", // Pink
	"#00CCCC", // Cyan
	"#FF9900", // Orange
	"#6666FF", // Indigo
	"#CC3366", // Magenta
	"#339966", // Teal
}

// AllocateColor picks the first unused color from the palette.
func AllocateColor(usedColors map[string]struct{}) string {
	for _, color := range ProfileColors {
		upper := strings.ToUpper(color)
		if _, used := usedColors[upper]; !used {
			return color
		}
	}
	idx := len(usedColors) % len(ProfileColors)
	return ProfileColors[idx]
}

// GetUsedColors collects all colors from existing profiles.
func GetUsedColors(profiles map[string]string) map[string]struct{} {
	used := make(map[string]struct{})
	for _, color := range profiles {
		used[strings.ToUpper(color)] = struct{}{}
	}
	return used
}
