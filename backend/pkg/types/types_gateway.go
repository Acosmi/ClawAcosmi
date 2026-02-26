package types

// 网关配置类型 — 继承自 src/config/types.gateway.ts

// GatewayBindMode 网关绑定模式
type GatewayBindMode string

const (
	GatewayBindAuto     GatewayBindMode = "auto"
	GatewayBindLAN      GatewayBindMode = "lan"
	GatewayBindLoopback GatewayBindMode = "loopback"
	GatewayBindCustom   GatewayBindMode = "custom"
	GatewayBindTailnet  GatewayBindMode = "tailnet"
)

// GatewayTlsConfig TLS 配置
type GatewayTlsConfig struct {
	Enabled      *bool  `json:"enabled,omitempty"`
	AutoGenerate *bool  `json:"autoGenerate,omitempty"` // 自动生成自签名证书（默认 true）
	CertPath     string `json:"certPath,omitempty"`
	KeyPath      string `json:"keyPath,omitempty"`
	CAPath       string `json:"caPath,omitempty"` // mTLS 或自定义 CA
}

// WideAreaDiscoveryConfig 广域网发现配置
type WideAreaDiscoveryConfig struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Domain  string `json:"domain,omitempty"` // 单播 DNS-SD 域名
}

// MdnsDiscoveryMode mDNS 发现模式
type MdnsDiscoveryMode string

const (
	MdnsOff     MdnsDiscoveryMode = "off"
	MdnsMinimal MdnsDiscoveryMode = "minimal"
	MdnsFull    MdnsDiscoveryMode = "full"
)

// MdnsDiscoveryConfig mDNS/Bonjour 发现配置
type MdnsDiscoveryConfig struct {
	Mode MdnsDiscoveryMode `json:"mode,omitempty"`
}

// DiscoveryConfig 服务发现总配置
type DiscoveryConfig struct {
	WideArea *WideAreaDiscoveryConfig `json:"wideArea,omitempty"`
	Mdns     *MdnsDiscoveryConfig     `json:"mdns,omitempty"`
}

// CanvasHostConfig Canvas 主机配置
type CanvasHostConfig struct {
	Enabled    *bool  `json:"enabled,omitempty"`
	Root       string `json:"root,omitempty"` // 默认 ~/.openacosmi/workspace/canvas
	Port       *int   `json:"port,omitempty"` // 默认 18793
	LiveReload *bool  `json:"liveReload,omitempty"`
}

// TalkConfig Talk 模式配置
type TalkConfig struct {
	VoiceID           string            `json:"voiceId,omitempty"`
	VoiceAliases      map[string]string `json:"voiceAliases,omitempty"`
	ModelID           string            `json:"modelId,omitempty"`
	OutputFormat      string            `json:"outputFormat,omitempty"`
	APIKey            string            `json:"apiKey,omitempty"`
	InterruptOnSpeech *bool             `json:"interruptOnSpeech,omitempty"`
}

// GatewayControlUiConfig 控制面板 UI 配置
type GatewayControlUiConfig struct {
	Enabled                      *bool    `json:"enabled,omitempty"`
	BasePath                     string   `json:"basePath,omitempty"`
	Root                         string   `json:"root,omitempty"`
	AllowedOrigins               []string `json:"allowedOrigins,omitempty"`
	AllowInsecureAuth            *bool    `json:"allowInsecureAuth,omitempty"`
	DangerouslyDisableDeviceAuth *bool    `json:"dangerouslyDisableDeviceAuth,omitempty"`
}

// GatewayAuthMode 网关认证模式
type GatewayAuthMode string

const (
	GatewayAuthToken    GatewayAuthMode = "token"
	GatewayAuthPassword GatewayAuthMode = "password"
)

// GatewayAuthConfig 网关认证配置
type GatewayAuthConfig struct {
	Mode           GatewayAuthMode `json:"mode,omitempty"`
	Token          string          `json:"token,omitempty"`
	Password       string          `json:"password,omitempty"`
	AllowTailscale *bool           `json:"allowTailscale,omitempty"`
}

// GatewayTailscaleMode Tailscale 暴露模式
type GatewayTailscaleMode string

const (
	TailscaleOff    GatewayTailscaleMode = "off"
	TailscaleServe  GatewayTailscaleMode = "serve"
	TailscaleFunnel GatewayTailscaleMode = "funnel"
)

// GatewayTailscaleConfig Tailscale 配置
type GatewayTailscaleConfig struct {
	Mode        GatewayTailscaleMode `json:"mode,omitempty"`
	ResetOnExit *bool                `json:"resetOnExit,omitempty"`
}

// GatewayRemoteConfig 远程网关连接配置
type GatewayRemoteConfig struct {
	URL            string `json:"url,omitempty"`       // ws:// 或 wss://
	Transport      string `json:"transport,omitempty"` // "ssh"|"direct"
	Token          string `json:"token,omitempty"`
	Password       string `json:"password,omitempty"`
	TLSFingerprint string `json:"tlsFingerprint,omitempty"` // sha256
	SSHTarget      string `json:"sshTarget,omitempty"`      // user@host
	SSHIdentity    string `json:"sshIdentity,omitempty"`    // SSH 密钥路径
}

// GatewayReloadMode 配置重载模式
type GatewayReloadMode string

const (
	ReloadOff     GatewayReloadMode = "off"
	ReloadRestart GatewayReloadMode = "restart"
	ReloadHot     GatewayReloadMode = "hot"
	ReloadHybrid  GatewayReloadMode = "hybrid"
)

// GatewayReloadConfig 配置重载设置
type GatewayReloadConfig struct {
	Mode       GatewayReloadMode `json:"mode,omitempty"`
	DebounceMs *int              `json:"debounceMs,omitempty"` // 默认 300
}

// GatewayHttpChatCompletionsConfig /v1/chat/completions 端点配置
type GatewayHttpChatCompletionsConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// GatewayHttpResponsesPdfConfig PDF 处理配置
type GatewayHttpResponsesPdfConfig struct {
	MaxPages     *int `json:"maxPages,omitempty"`     // 默认 4
	MaxPixels    *int `json:"maxPixels,omitempty"`    // 默认 4M
	MinTextChars *int `json:"minTextChars,omitempty"` // 默认 200
}

// GatewayHttpResponsesFilesConfig 文件输入配置
type GatewayHttpResponsesFilesConfig struct {
	AllowURL     *bool                          `json:"allowUrl,omitempty"`
	AllowedMimes []string                       `json:"allowedMimes,omitempty"`
	MaxBytes     *int                           `json:"maxBytes,omitempty"`     // 默认 5MB
	MaxChars     *int                           `json:"maxChars,omitempty"`     // 默认 200k
	MaxRedirects *int                           `json:"maxRedirects,omitempty"` // 默认 3
	TimeoutMs    *int                           `json:"timeoutMs,omitempty"`    // 默认 10s
	PDF          *GatewayHttpResponsesPdfConfig `json:"pdf,omitempty"`
}

// GatewayHttpResponsesImagesConfig 图片输入配置
type GatewayHttpResponsesImagesConfig struct {
	AllowURL     *bool    `json:"allowUrl,omitempty"`
	AllowedMimes []string `json:"allowedMimes,omitempty"`
	MaxBytes     *int     `json:"maxBytes,omitempty"` // 默认 10MB
	MaxRedirects *int     `json:"maxRedirects,omitempty"`
	TimeoutMs    *int     `json:"timeoutMs,omitempty"`
}

// GatewayHttpResponsesConfig /v1/responses 端点配置
type GatewayHttpResponsesConfig struct {
	Enabled      *bool                             `json:"enabled,omitempty"`
	MaxBodyBytes *int                              `json:"maxBodyBytes,omitempty"` // 默认 20MB
	Files        *GatewayHttpResponsesFilesConfig  `json:"files,omitempty"`
	Images       *GatewayHttpResponsesImagesConfig `json:"images,omitempty"`
}

// GatewayHttpEndpointsConfig HTTP 端点配置
type GatewayHttpEndpointsConfig struct {
	ChatCompletions *GatewayHttpChatCompletionsConfig `json:"chatCompletions,omitempty"`
	Responses       *GatewayHttpResponsesConfig       `json:"responses,omitempty"`
}

// GatewayHttpConfig 网关 HTTP 配置
type GatewayHttpConfig struct {
	Endpoints *GatewayHttpEndpointsConfig `json:"endpoints,omitempty"`
}

// GatewayNodesBrowserConfig 节点浏览器路由配置
type GatewayNodesBrowserConfig struct {
	Mode string `json:"mode,omitempty"` // "auto"|"manual"|"off"
	Node string `json:"node,omitempty"` // 固定到特定节点
}

// GatewayNodesConfig 网关节点配置
type GatewayNodesConfig struct {
	Browser       *GatewayNodesBrowserConfig `json:"browser,omitempty"`
	AllowCommands []string                   `json:"allowCommands,omitempty"`
	DenyCommands  []string                   `json:"denyCommands,omitempty"`
}

// GatewayMode 网关运行模式
type GatewayMode string

const (
	GatewayModeLocal  GatewayMode = "local"
	GatewayModeRemote GatewayMode = "remote"
)

// GatewayConfig 网关总配置
// 原版: export type GatewayConfig (249 行)
type GatewayConfig struct {
	Port           *int                    `json:"port,omitempty"` // 默认 18789
	Mode           GatewayMode             `json:"mode,omitempty"`
	Bind           GatewayBindMode         `json:"bind,omitempty"`
	CustomBindHost string                  `json:"customBindHost,omitempty"`
	ControlUI      *GatewayControlUiConfig `json:"controlUi,omitempty"`
	Auth           *GatewayAuthConfig      `json:"auth,omitempty"`
	Tailscale      *GatewayTailscaleConfig `json:"tailscale,omitempty"`
	Remote         *GatewayRemoteConfig    `json:"remote,omitempty"`
	Reload         *GatewayReloadConfig    `json:"reload,omitempty"`
	TLS            *GatewayTlsConfig       `json:"tls,omitempty"`
	HTTP           *GatewayHttpConfig      `json:"http,omitempty"`
	Nodes          *GatewayNodesConfig     `json:"nodes,omitempty"`
	TrustedProxies []string                `json:"trustedProxies,omitempty"`
}
