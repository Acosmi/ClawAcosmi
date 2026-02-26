package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// BrowserExecutableKind classifies the Chromium variant.
type BrowserExecutableKind string

const (
	KindChrome   BrowserExecutableKind = "chrome"
	KindCanary   BrowserExecutableKind = "canary"
	KindChromium BrowserExecutableKind = "chromium"
	KindBrave    BrowserExecutableKind = "brave"
	KindEdge     BrowserExecutableKind = "edge"
	KindCustom   BrowserExecutableKind = "custom"
)

// BrowserExecutable holds a discovered browser executable.
type BrowserExecutable struct {
	Kind BrowserExecutableKind
	Path string
}

// chromiumExeNames lists known executable names for Chromium-based browsers.
var chromiumExeNames = map[string]BrowserExecutableKind{
	"google-chrome":          KindChrome,
	"google-chrome-stable":   KindChrome,
	"google-chrome-beta":     KindChrome,
	"google-chrome-dev":      KindChrome,
	"google-chrome-unstable": KindChrome,
	"chromium":               KindChromium,
	"chromium-browser":       KindChromium,
	"brave-browser":          KindBrave,
	"brave-browser-stable":   KindBrave,
	"microsoft-edge":         KindEdge,
	"microsoft-edge-beta":    KindEdge,
	"microsoft-edge-dev":     KindEdge,
}

// macAppPaths lists known macOS application paths.
var macAppPaths = []struct {
	path string
	kind BrowserExecutableKind
}{
	{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", KindChrome},
	{"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary", KindCanary},
	{"/Applications/Chromium.app/Contents/MacOS/Chromium", KindChromium},
	{"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser", KindBrave},
	{"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge", KindEdge},
}

// InferKindFromExecutableName guesses the browser kind from exec name.
func InferKindFromExecutableName(name string) BrowserExecutableKind {
	lower := strings.ToLower(filepath.Base(name))
	if kind, ok := chromiumExeNames[lower]; ok {
		return kind
	}
	if strings.Contains(lower, "canary") {
		return KindCanary
	}
	if strings.Contains(lower, "brave") {
		return KindBrave
	}
	if strings.Contains(lower, "edge") {
		return KindEdge
	}
	if strings.Contains(lower, "chromium") {
		return KindChromium
	}
	if strings.Contains(lower, "chrome") {
		return KindChrome
	}
	return KindCustom
}

// fileExists checks if a file exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// FindChromeExecutableMac discovers Chrome on macOS.
func FindChromeExecutableMac() *BrowserExecutable {
	for _, app := range macAppPaths {
		if fileExists(app.path) {
			return &BrowserExecutable{Kind: app.kind, Path: app.path}
		}
	}
	// Try home directory.
	home, err := os.UserHomeDir()
	if err == nil {
		for _, app := range macAppPaths {
			homePath := filepath.Join(home, app.path)
			if fileExists(homePath) {
				return &BrowserExecutable{Kind: app.kind, Path: homePath}
			}
		}
	}
	return nil
}

// FindChromeExecutableLinux discovers Chrome on Linux.
func FindChromeExecutableLinux() *BrowserExecutable {
	names := []struct {
		cmd  string
		kind BrowserExecutableKind
	}{
		{"google-chrome-stable", KindChrome},
		{"google-chrome", KindChrome},
		{"chromium-browser", KindChromium},
		{"chromium", KindChromium},
		{"brave-browser", KindBrave},
		{"brave-browser-stable", KindBrave},
		{"microsoft-edge", KindEdge},
		{"microsoft-edge-beta", KindEdge},
	}
	for _, n := range names {
		if path, err := exec.LookPath(n.cmd); err == nil {
			return &BrowserExecutable{Kind: n.kind, Path: path}
		}
	}
	return nil
}

// FindChromeExecutableWindows discovers Chrome on Windows.
func FindChromeExecutableWindows() *BrowserExecutable {
	progFiles := []string{
		os.Getenv("PROGRAMFILES"),
		os.Getenv("PROGRAMFILES(X86)"),
		os.Getenv("LOCALAPPDATA"),
	}
	subpaths := []struct {
		sub  string
		kind BrowserExecutableKind
	}{
		{`Google\Chrome\Application\chrome.exe`, KindChrome},
		{`Google\Chrome SxS\Application\chrome.exe`, KindCanary},
		{`BraveSoftware\Brave-Browser\Application\brave.exe`, KindBrave},
		{`Microsoft\Edge\Application\msedge.exe`, KindEdge},
		{`Chromium\Application\chrome.exe`, KindChromium},
	}
	for _, pf := range progFiles {
		if pf == "" {
			continue
		}
		for _, sp := range subpaths {
			full := filepath.Join(pf, sp.sub)
			if fileExists(full) {
				return &BrowserExecutable{Kind: sp.kind, Path: full}
			}
		}
	}
	return nil
}

// ResolveBrowserExecutable finds a browser executable for the current platform,
// preferring a custom executable if specified.
func ResolveBrowserExecutable(cfg *ResolvedBrowserConfig) *BrowserExecutable {
	if cfg != nil && cfg.CustomExecutable != "" {
		return &BrowserExecutable{
			Kind: InferKindFromExecutableName(cfg.CustomExecutable),
			Path: cfg.CustomExecutable,
		}
	}
	switch runtime.GOOS {
	case "darwin":
		return FindChromeExecutableMac()
	case "linux":
		return FindChromeExecutableLinux()
	case "windows":
		return FindChromeExecutableWindows()
	default:
		return nil
	}
}
