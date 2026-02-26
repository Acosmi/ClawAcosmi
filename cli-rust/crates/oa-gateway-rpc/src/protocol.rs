/// Gateway RPC protocol types and constants.
///
/// Defines the wire-level types exchanged between gateway clients and the
/// gateway server over WebSocket, including connect handshake parameters,
/// request/response/event frames, client identity metadata, and close-code
/// hints.
///
/// Source: `src/gateway/protocol/client-info.ts`, `src/gateway/protocol/schema/frames.ts`,
///         `src/gateway/protocol/schema/protocol-schemas.ts`, `src/gateway/protocol/schema/error-codes.ts`,
///         `src/gateway/protocol/schema/snapshot.ts`

use std::collections::HashMap;

use serde::{Deserialize, Serialize};

// ---------------------------------------------------------------------------
// Protocol version
// ---------------------------------------------------------------------------

/// Current gateway protocol version.
///
/// Both client and server must agree on a protocol version during the
/// connect handshake. This constant tracks the latest version supported
/// by this crate.
///
/// Source: `src/gateway/protocol/schema/protocol-schemas.ts`
pub const PROTOCOL_VERSION: u32 = 3;

// ---------------------------------------------------------------------------
// Client identity
// ---------------------------------------------------------------------------

/// Known gateway client identifiers.
///
/// Each variant maps to a well-known string value used in the connect
/// handshake to identify the type of client connecting.
///
/// Source: `src/gateway/protocol/client-info.ts` (`GATEWAY_CLIENT_IDS`)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum GatewayClientId {
    /// Web chat user interface.
    #[serde(rename = "webchat-ui")]
    WebchatUi,
    /// Control panel user interface.
    #[serde(rename = "openacosmi-control-ui")]
    ControlUi,
    /// Webchat client.
    #[serde(rename = "webchat")]
    Webchat,
    /// Command-line interface client.
    #[serde(rename = "cli")]
    Cli,
    /// Generic gateway client.
    #[serde(rename = "gateway-client")]
    GatewayClient,
    /// macOS desktop application.
    #[serde(rename = "openacosmi-macos")]
    MacosApp,
    /// iOS application.
    #[serde(rename = "openacosmi-ios")]
    IosApp,
    /// Android application.
    #[serde(rename = "openacosmi-android")]
    AndroidApp,
    /// Node host process.
    #[serde(rename = "node-host")]
    NodeHost,
    /// Test client.
    #[serde(rename = "test")]
    Test,
    /// Fingerprint client.
    #[serde(rename = "fingerprint")]
    Fingerprint,
    /// Probe client.
    #[serde(rename = "openacosmi-probe")]
    Probe,
}

impl GatewayClientId {
    /// Returns the wire-format string for this client ID.
    ///
    /// Source: `src/gateway/protocol/client-info.ts`
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            Self::WebchatUi => "webchat-ui",
            Self::ControlUi => "openacosmi-control-ui",
            Self::Webchat => "webchat",
            Self::Cli => "cli",
            Self::GatewayClient => "gateway-client",
            Self::MacosApp => "openacosmi-macos",
            Self::IosApp => "openacosmi-ios",
            Self::AndroidApp => "openacosmi-android",
            Self::NodeHost => "node-host",
            Self::Test => "test",
            Self::Fingerprint => "fingerprint",
            Self::Probe => "openacosmi-probe",
        }
    }

    /// All known client IDs as a static slice.
    ///
    /// Source: `src/gateway/protocol/client-info.ts`
    #[must_use]
    pub const fn all() -> &'static [GatewayClientId] {
        &[
            Self::WebchatUi,
            Self::ControlUi,
            Self::Webchat,
            Self::Cli,
            Self::GatewayClient,
            Self::MacosApp,
            Self::IosApp,
            Self::AndroidApp,
            Self::NodeHost,
            Self::Test,
            Self::Fingerprint,
            Self::Probe,
        ]
    }
}

/// Gateway client operational modes.
///
/// Describes how the connected client will interact with the gateway
/// (e.g., interactive CLI, background backend, one-shot probe).
///
/// Source: `src/gateway/protocol/client-info.ts` (`GATEWAY_CLIENT_MODES`)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum GatewayClientMode {
    /// Interactive web chat mode.
    Webchat,
    /// Command-line interface mode.
    Cli,
    /// Graphical UI mode.
    Ui,
    /// Headless backend mode.
    Backend,
    /// Node host mode.
    Node,
    /// Probe / health-check mode.
    Probe,
    /// Automated test mode.
    Test,
}

impl GatewayClientMode {
    /// Returns the wire-format string for this mode.
    ///
    /// Source: `src/gateway/protocol/client-info.ts`
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Webchat => "webchat",
            Self::Cli => "cli",
            Self::Ui => "ui",
            Self::Backend => "backend",
            Self::Node => "node",
            Self::Probe => "probe",
            Self::Test => "test",
        }
    }

    /// All known client modes as a static slice.
    ///
    /// Source: `src/gateway/protocol/client-info.ts`
    #[must_use]
    pub const fn all() -> &'static [GatewayClientMode] {
        &[
            Self::Webchat,
            Self::Cli,
            Self::Ui,
            Self::Backend,
            Self::Node,
            Self::Probe,
            Self::Test,
        ]
    }
}

/// Gateway client capability flags.
///
/// Advertised during connect to declare optional features the client supports.
///
/// Source: `src/gateway/protocol/client-info.ts` (`GATEWAY_CLIENT_CAPS`)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
pub enum GatewayClientCap {
    /// Client supports tool event streaming.
    #[serde(rename = "tool-events")]
    ToolEvents,
}

impl GatewayClientCap {
    /// Returns the wire-format string for this capability.
    ///
    /// Source: `src/gateway/protocol/client-info.ts`
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            Self::ToolEvents => "tool-events",
        }
    }
}

/// Metadata describing the connecting client.
///
/// Included in the `ConnectParams.client` field during the WebSocket handshake.
///
/// Source: `src/gateway/protocol/client-info.ts` (`GatewayClientInfo`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct GatewayClientInfo {
    /// Client identifier (e.g., `"cli"`, `"webchat-ui"`).
    pub id: GatewayClientId,
    /// Optional human-readable display name.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub display_name: Option<String>,
    /// Client software version string.
    pub version: String,
    /// Operating system / platform (e.g., `"darwin"`, `"linux"`).
    pub platform: String,
    /// Device family (e.g., `"iPhone"`, `"Mac"`).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub device_family: Option<String>,
    /// Device model identifier (e.g., `"MacBookPro18,3"`).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_identifier: Option<String>,
    /// Operational mode.
    pub mode: GatewayClientMode,
    /// Unique instance identifier for this client session.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub instance_id: Option<String>,
}

// ---------------------------------------------------------------------------
// Connect handshake
// ---------------------------------------------------------------------------

/// Device authentication credentials sent during connect.
///
/// Contains a device keypair-based identity proof. The gateway verifies the
/// signature against the advertised public key.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`ConnectParamsSchema.device`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConnectDeviceAuth {
    /// Device identifier.
    pub id: String,
    /// Base64url-encoded public key.
    pub public_key: String,
    /// Base64url-encoded signature over the authentication payload.
    pub signature: String,
    /// Signing timestamp (milliseconds since epoch).
    pub signed_at: u64,
    /// Optional server-issued challenge nonce.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub nonce: Option<String>,
}

/// Token / password credentials sent during connect.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`ConnectParamsSchema.auth`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConnectAuth {
    /// Bearer token.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub token: Option<String>,
    /// Password credential.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub password: Option<String>,
}

/// Parameters sent by the client in the initial `connect` RPC method.
///
/// Negotiates protocol version, advertises client identity and capabilities,
/// and provides authentication credentials.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`ConnectParamsSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConnectParams {
    /// Minimum protocol version the client supports.
    pub min_protocol: u32,
    /// Maximum protocol version the client supports.
    pub max_protocol: u32,
    /// Client identity metadata.
    pub client: GatewayClientInfo,
    /// Optional capability flags advertised by the client.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub caps: Option<Vec<String>>,
    /// Optional list of CLI commands the client supports.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub commands: Option<Vec<String>>,
    /// Optional permission flags.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub permissions: Option<HashMap<String, bool>>,
    /// Optional PATH environment variable.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub path_env: Option<String>,
    /// Requested role (e.g., `"operator"`).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub role: Option<String>,
    /// Requested authorization scopes.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scopes: Option<Vec<String>>,
    /// Device authentication credentials.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub device: Option<ConnectDeviceAuth>,
    /// Token / password authentication credentials.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auth: Option<ConnectAuth>,
    /// Locale hint (e.g., `"en-US"`).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub locale: Option<String>,
    /// User-Agent string.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub user_agent: Option<String>,
}

// ---------------------------------------------------------------------------
// Hello-ok (server acknowledgment)
// ---------------------------------------------------------------------------

/// Server information returned in the `hello-ok` response.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`HelloOkSchema.server`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HelloOkServer {
    /// Server software version.
    pub version: String,
    /// Optional git commit hash.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub commit: Option<String>,
    /// Optional hostname.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub host: Option<String>,
    /// Unique connection identifier assigned by the server.
    pub conn_id: String,
}

/// Supported methods and events the server declares.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`HelloOkSchema.features`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HelloOkFeatures {
    /// RPC methods the server supports.
    pub methods: Vec<String>,
    /// Event types the server may emit.
    pub events: Vec<String>,
}

/// Device-token auth info returned in the `hello-ok` response.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`HelloOkSchema.auth`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HelloOkAuth {
    /// Server-issued device token.
    pub device_token: String,
    /// Assigned role.
    pub role: String,
    /// Granted authorization scopes.
    pub scopes: Vec<String>,
    /// Token issuance timestamp (milliseconds since epoch).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub issued_at_ms: Option<u64>,
}

/// Policy limits the server enforces on this connection.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`HelloOkSchema.policy`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HelloOkPolicy {
    /// Maximum payload size in bytes.
    pub max_payload: u64,
    /// Maximum buffered bytes before back-pressure kicks in.
    pub max_buffered_bytes: u64,
    /// Server tick interval in milliseconds.
    pub tick_interval_ms: u64,
}

/// Presence entry in the gateway snapshot.
///
/// Describes a single connected client visible in the presence list.
///
/// Source: `src/gateway/protocol/schema/snapshot.ts` (`PresenceEntrySchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PresenceEntry {
    /// Hostname of the client.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub host: Option<String>,
    /// IP address of the client.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub ip: Option<String>,
    /// Client software version.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub version: Option<String>,
    /// Client platform.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub platform: Option<String>,
    /// Device family.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub device_family: Option<String>,
    /// Device model identifier.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub model_identifier: Option<String>,
    /// Operational mode.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub mode: Option<String>,
    /// Seconds since last user input.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub last_input_seconds: Option<u64>,
    /// Connection reason.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
    /// Descriptive tags.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tags: Option<Vec<String>>,
    /// Human-readable text.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub text: Option<String>,
    /// Timestamp (milliseconds since epoch).
    pub ts: u64,
    /// Device identifier.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub device_id: Option<String>,
    /// Roles assigned to this client.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub roles: Option<Vec<String>>,
    /// Scopes granted to this client.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scopes: Option<Vec<String>>,
    /// Instance identifier.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub instance_id: Option<String>,
}

/// State version counters for optimistic concurrency.
///
/// Source: `src/gateway/protocol/schema/snapshot.ts` (`StateVersionSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct StateVersion {
    /// Presence state version counter.
    pub presence: u64,
    /// Health state version counter.
    pub health: u64,
}

/// Session defaults from the gateway snapshot.
///
/// Source: `src/gateway/protocol/schema/snapshot.ts` (`SessionDefaultsSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SessionDefaults {
    /// Default agent identifier.
    pub default_agent_id: String,
    /// Main session key prefix.
    pub main_key: String,
    /// Main session key with scope.
    pub main_session_key: String,
    /// Optional scope.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub scope: Option<String>,
}

/// Gateway state snapshot delivered in the `hello-ok` response.
///
/// Contains the current presence list, health state, and metadata about
/// the running gateway instance.
///
/// Source: `src/gateway/protocol/schema/snapshot.ts` (`SnapshotSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Snapshot {
    /// Currently connected clients.
    pub presence: Vec<PresenceEntry>,
    /// Health data (opaque JSON).
    pub health: serde_json::Value,
    /// State version for optimistic concurrency.
    pub state_version: StateVersion,
    /// Server uptime in milliseconds.
    pub uptime_ms: u64,
    /// Config file path on the server.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub config_path: Option<String>,
    /// State directory on the server.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub state_dir: Option<String>,
    /// Default session resolution info.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub session_defaults: Option<SessionDefaults>,
}

/// Successful server acknowledgment after the connect handshake.
///
/// The server responds with `hello-ok` when the client's `connect` call
/// succeeds, confirming the negotiated protocol version and returning
/// the initial state snapshot.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`HelloOkSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct HelloOk {
    /// Always `"hello-ok"`.
    #[serde(rename = "type")]
    pub frame_type: String,
    /// Negotiated protocol version.
    pub protocol: u32,
    /// Server identity.
    pub server: HelloOkServer,
    /// Supported methods and events.
    pub features: HelloOkFeatures,
    /// Initial state snapshot.
    pub snapshot: Snapshot,
    /// Optional canvas host URL.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub canvas_host_url: Option<String>,
    /// Device token auth info (if device auth succeeded).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub auth: Option<HelloOkAuth>,
    /// Connection policy limits.
    pub policy: HelloOkPolicy,
}

// ---------------------------------------------------------------------------
// Error shape
// ---------------------------------------------------------------------------

/// Structured error returned in response frames.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`ErrorShapeSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ErrorShape {
    /// Machine-readable error code.
    pub code: String,
    /// Human-readable error message.
    pub message: String,
    /// Optional additional details (opaque JSON).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub details: Option<serde_json::Value>,
    /// Whether the request can be retried.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub retryable: Option<bool>,
    /// Suggested retry delay in milliseconds.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub retry_after_ms: Option<u64>,
}

/// Well-known gateway error codes.
///
/// Source: `src/gateway/protocol/schema/error-codes.ts`
pub mod error_codes {
    /// Client is not linked to the gateway.
    pub const NOT_LINKED: &str = "NOT_LINKED";
    /// Client is not paired.
    pub const NOT_PAIRED: &str = "NOT_PAIRED";
    /// Agent timed out.
    pub const AGENT_TIMEOUT: &str = "AGENT_TIMEOUT";
    /// Invalid request.
    pub const INVALID_REQUEST: &str = "INVALID_REQUEST";
    /// Service unavailable.
    pub const UNAVAILABLE: &str = "UNAVAILABLE";
}

/// Build an `ErrorShape` from a code and message.
///
/// Source: `src/gateway/protocol/schema/error-codes.ts` (`errorShape`)
#[must_use]
pub fn error_shape(code: &str, message: &str) -> ErrorShape {
    ErrorShape {
        code: code.to_string(),
        message: message.to_string(),
        details: None,
        retryable: None,
        retry_after_ms: None,
    }
}

// ---------------------------------------------------------------------------
// Request / Response / Event frames
// ---------------------------------------------------------------------------

/// Client-to-server RPC request frame.
///
/// The client sends a request with a unique `id` and awaits a `ResponseFrame`
/// with the same `id`.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`RequestFrameSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RequestFrame {
    /// Always `"req"`.
    #[serde(rename = "type")]
    pub frame_type: String,
    /// Unique request identifier (UUID).
    pub id: String,
    /// RPC method name.
    pub method: String,
    /// Optional parameters (opaque JSON).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub params: Option<serde_json::Value>,
}

/// Server-to-client RPC response frame.
///
/// Corresponds to a prior `RequestFrame` with the matching `id`.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`ResponseFrameSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ResponseFrame {
    /// Always `"res"`.
    #[serde(rename = "type")]
    pub frame_type: String,
    /// Request identifier this response corresponds to.
    pub id: String,
    /// Whether the request succeeded.
    pub ok: bool,
    /// Response payload (on success).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub payload: Option<serde_json::Value>,
    /// Error details (on failure).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub error: Option<ErrorShape>,
}

/// Server-to-client push event frame.
///
/// Delivered asynchronously by the server. May carry a monotonically
/// increasing sequence number for gap detection.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`EventFrameSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EventFrame {
    /// Always `"event"`.
    #[serde(rename = "type")]
    pub frame_type: String,
    /// Event type name.
    pub event: String,
    /// Event payload (opaque JSON).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub payload: Option<serde_json::Value>,
    /// Monotonically increasing sequence number.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub seq: Option<u64>,
    /// State version at the time of the event.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub state_version: Option<StateVersion>,
}

/// Discriminated union of all top-level gateway frames.
///
/// Used for parsing incoming WebSocket messages when the frame type is
/// not yet known.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`GatewayFrameSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type")]
pub enum GatewayFrame {
    /// Client-to-server request.
    #[serde(rename = "req")]
    Request(RequestFrame),
    /// Server-to-client response.
    #[serde(rename = "res")]
    Response(ResponseFrame),
    /// Server-to-client event.
    #[serde(rename = "event")]
    Event(EventFrame),
}

// ---------------------------------------------------------------------------
// Tick / Shutdown events
// ---------------------------------------------------------------------------

/// Server heartbeat tick event payload.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`TickEventSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TickEvent {
    /// Server timestamp (milliseconds since epoch).
    pub ts: u64,
}

/// Server shutdown event payload.
///
/// Source: `src/gateway/protocol/schema/frames.ts` (`ShutdownEventSchema`)
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ShutdownEvent {
    /// Human-readable reason for the shutdown.
    pub reason: String,
    /// Expected time until restart (milliseconds), if any.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub restart_expected_ms: Option<u64>,
}

// ---------------------------------------------------------------------------
// Normalization helpers
// ---------------------------------------------------------------------------

/// Normalize a raw string to a known `GatewayClientId`, or `None` if unrecognized.
///
/// Trims whitespace and lowercases the input before matching against known IDs.
///
/// Source: `src/gateway/protocol/client-info.ts` (`normalizeGatewayClientId`)
#[must_use]
pub fn normalize_gateway_client_id(raw: Option<&str>) -> Option<GatewayClientId> {
    let normalized = raw?.trim().to_lowercase();
    if normalized.is_empty() {
        return None;
    }
    GatewayClientId::all()
        .iter()
        .find(|id| id.as_str() == normalized)
        .copied()
}

/// Normalize a raw string to a known `GatewayClientMode`, or `None` if unrecognized.
///
/// Trims whitespace and lowercases the input before matching against known modes.
///
/// Source: `src/gateway/protocol/client-info.ts` (`normalizeGatewayClientMode`)
#[must_use]
pub fn normalize_gateway_client_mode(raw: Option<&str>) -> Option<GatewayClientMode> {
    let normalized = raw?.trim().to_lowercase();
    if normalized.is_empty() {
        return None;
    }
    GatewayClientMode::all()
        .iter()
        .find(|mode| mode.as_str() == normalized)
        .copied()
}

/// Check whether a capability list includes a given cap.
///
/// Source: `src/gateway/protocol/client-info.ts` (`hasGatewayClientCap`)
#[must_use]
pub fn has_gateway_client_cap(caps: Option<&[String]>, cap: GatewayClientCap) -> bool {
    caps.is_some_and(|list| list.iter().any(|c| c == cap.as_str()))
}

// ---------------------------------------------------------------------------
// Close-code hints
// ---------------------------------------------------------------------------

/// Human-readable descriptions for common WebSocket close codes.
///
/// Source: `src/gateway/client.ts` (`GATEWAY_CLOSE_CODE_HINTS`)
pub fn gateway_close_code_hints() -> HashMap<u16, &'static str> {
    let mut map = HashMap::new();
    map.insert(1000, "normal closure");
    map.insert(1006, "abnormal closure (no close frame)");
    map.insert(1008, "policy violation");
    map.insert(1012, "service restart");
    map
}

/// Look up a human-readable description for a WebSocket close code.
///
/// Source: `src/gateway/client.ts` (`describeGatewayCloseCode`)
#[must_use]
pub fn describe_gateway_close_code(code: u16) -> Option<&'static str> {
    match code {
        1000 => Some("normal closure"),
        1006 => Some("abnormal closure (no close frame)"),
        1008 => Some("policy violation"),
        1012 => Some("service restart"),
        _ => None,
    }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn protocol_version_is_3() {
        assert_eq!(PROTOCOL_VERSION, 3);
    }

    #[test]
    fn client_id_round_trips() {
        let id = GatewayClientId::Cli;
        assert_eq!(id.as_str(), "cli");
        let json = serde_json::to_string(&id).ok();
        assert_eq!(json.as_deref(), Some("\"cli\""));

        let parsed: GatewayClientId =
            serde_json::from_str("\"cli\"").ok().unwrap_or(GatewayClientId::Test);
        assert_eq!(parsed, GatewayClientId::Cli);
    }

    #[test]
    fn client_mode_round_trips() {
        let mode = GatewayClientMode::Backend;
        assert_eq!(mode.as_str(), "backend");
        let json = serde_json::to_string(&mode).ok();
        assert_eq!(json.as_deref(), Some("\"backend\""));
    }

    #[test]
    fn normalize_client_id_valid() {
        assert_eq!(
            normalize_gateway_client_id(Some("CLI")),
            Some(GatewayClientId::Cli)
        );
        assert_eq!(
            normalize_gateway_client_id(Some(" webchat-ui ")),
            Some(GatewayClientId::WebchatUi)
        );
    }

    #[test]
    fn normalize_client_id_unknown() {
        assert_eq!(normalize_gateway_client_id(Some("unknown")), None);
        assert_eq!(normalize_gateway_client_id(Some("")), None);
        assert_eq!(normalize_gateway_client_id(None), None);
    }

    #[test]
    fn normalize_client_mode_valid() {
        assert_eq!(
            normalize_gateway_client_mode(Some("CLI")),
            Some(GatewayClientMode::Cli)
        );
        assert_eq!(
            normalize_gateway_client_mode(Some(" backend ")),
            Some(GatewayClientMode::Backend)
        );
    }

    #[test]
    fn normalize_client_mode_unknown() {
        assert_eq!(normalize_gateway_client_mode(Some("nope")), None);
        assert_eq!(normalize_gateway_client_mode(None), None);
    }

    #[test]
    fn has_cap_checks_correctly() {
        let caps = vec!["tool-events".to_string()];
        assert!(has_gateway_client_cap(Some(&caps), GatewayClientCap::ToolEvents));
        assert!(!has_gateway_client_cap(Some(&[]), GatewayClientCap::ToolEvents));
        assert!(!has_gateway_client_cap(None, GatewayClientCap::ToolEvents));
    }

    #[test]
    fn close_code_hints_known() {
        assert_eq!(describe_gateway_close_code(1000), Some("normal closure"));
        assert_eq!(
            describe_gateway_close_code(1006),
            Some("abnormal closure (no close frame)")
        );
        assert_eq!(describe_gateway_close_code(9999), None);
    }

    #[test]
    fn request_frame_serializes() {
        let frame = RequestFrame {
            frame_type: "req".to_string(),
            id: "abc-123".to_string(),
            method: "sessions.list".to_string(),
            params: None,
        };
        let json = serde_json::to_value(&frame).ok();
        assert!(json.is_some());
        let val = json.unwrap_or_default();
        assert_eq!(val["type"], "req");
        assert_eq!(val["method"], "sessions.list");
    }

    #[test]
    fn response_frame_deserializes() {
        let json = r#"{"type":"res","id":"abc","ok":true,"payload":{"data":1}}"#;
        let frame: Result<ResponseFrame, _> = serde_json::from_str(json);
        assert!(frame.is_ok());
        let f = frame.unwrap_or_else(|_| ResponseFrame {
            frame_type: String::new(),
            id: String::new(),
            ok: false,
            payload: None,
            error: None,
        });
        assert!(f.ok);
        assert!(f.payload.is_some());
    }

    #[test]
    fn event_frame_deserializes() {
        let json = r#"{"type":"event","event":"tick","payload":{"ts":123},"seq":5}"#;
        let frame: Result<EventFrame, _> = serde_json::from_str(json);
        assert!(frame.is_ok());
        let f = frame.unwrap_or_else(|_| EventFrame {
            frame_type: String::new(),
            event: String::new(),
            payload: None,
            seq: None,
            state_version: None,
        });
        assert_eq!(f.event, "tick");
        assert_eq!(f.seq, Some(5));
    }

    #[test]
    fn error_shape_builder() {
        let err = error_shape(error_codes::INVALID_REQUEST, "bad field");
        assert_eq!(err.code, "INVALID_REQUEST");
        assert_eq!(err.message, "bad field");
        assert!(err.details.is_none());
    }

    #[test]
    fn connect_params_serializes() {
        let params = ConnectParams {
            min_protocol: 3,
            max_protocol: 3,
            client: GatewayClientInfo {
                id: GatewayClientId::Cli,
                display_name: None,
                version: "0.1.0".to_string(),
                platform: "darwin".to_string(),
                device_family: None,
                model_identifier: None,
                mode: GatewayClientMode::Cli,
                instance_id: None,
            },
            caps: Some(vec![]),
            commands: None,
            permissions: None,
            path_env: None,
            role: Some("operator".to_string()),
            scopes: Some(vec!["operator.admin".to_string()]),
            device: None,
            auth: Some(ConnectAuth {
                token: Some("secret".to_string()),
                password: None,
            }),
            locale: None,
            user_agent: None,
        };
        let json = serde_json::to_value(&params).ok();
        assert!(json.is_some());
        let val = json.unwrap_or_default();
        assert_eq!(val["minProtocol"], 3);
        assert_eq!(val["client"]["id"], "cli");
    }
}
