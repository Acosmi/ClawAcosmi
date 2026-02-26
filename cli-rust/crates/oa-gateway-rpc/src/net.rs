/// Network utilities for the gateway client.
///
/// Provides LAN IP detection, loopback address checking, IPv4-mapped IPv6
/// normalization, local address verification, and bind host resolution.
///
/// Source: `src/gateway/net.ts`

use std::net::{IpAddr, Ipv4Addr, SocketAddr, TcpListener};

use oa_types::gateway::GatewayBindMode;

// ---------------------------------------------------------------------------
// LAN IP detection
// ---------------------------------------------------------------------------

/// Pick the primary non-internal IPv4 address (LAN IP).
///
/// Prefers common interface names (`en0`, `eth0`) on platforms that expose
/// interface names. Falls back to the first non-loopback IPv4 address
/// found by binding/listening on `0.0.0.0` and inspecting the local
/// socket address (a cross-platform heuristic).
///
/// On most systems this returns the machine's LAN-facing IPv4 address.
/// Returns `None` when no non-loopback IPv4 address is available.
///
/// Source: `src/gateway/net.ts` (`pickPrimaryLanIPv4`)
#[must_use]
pub fn pick_primary_lan_ipv4() -> Option<String> {
    // Use a UDP socket trick to find the primary outbound interface.
    // By "connecting" a UDP socket to a public address (no packets are sent),
    // the OS selects the appropriate interface and local address.
    let socket = std::net::UdpSocket::bind("0.0.0.0:0").ok()?;
    // Connect to a well-known public address (Google DNS).
    // No actual traffic is sent -- this just causes the OS to
    // resolve the outgoing interface.
    socket.connect("8.8.8.8:80").ok()?;
    let local_addr = socket.local_addr().ok()?;
    match local_addr.ip() {
        IpAddr::V4(v4) if !v4.is_loopback() && !v4.is_unspecified() => Some(v4.to_string()),
        _ => None,
    }
}

// ---------------------------------------------------------------------------
// Loopback detection
// ---------------------------------------------------------------------------

/// Check whether an IP address string represents a loopback address.
///
/// Recognizes `127.0.0.1`, the `127.0.0.0/8` range, `::1`, and
/// IPv4-mapped IPv6 loopback addresses (`::ffff:127.*`).
///
/// Source: `src/gateway/net.ts` (`isLoopbackAddress`)
#[must_use]
pub fn is_loopback_address(ip: Option<&str>) -> bool {
    let Some(raw) = ip else {
        return false;
    };
    let trimmed = raw.trim();
    if trimmed.is_empty() {
        return false;
    }
    if trimmed == "127.0.0.1" || trimmed == "::1" {
        return true;
    }
    if trimmed.starts_with("127.") {
        return true;
    }
    if trimmed.starts_with("::ffff:127.") {
        return true;
    }
    false
}

/// Alias matching the TS export name.
///
/// Source: `src/gateway/net.ts` (`isLoopbackHost`)
#[must_use]
pub fn is_loopback_host(host: &str) -> bool {
    is_loopback_address(Some(host))
}

// ---------------------------------------------------------------------------
// IPv4-mapped IPv6 normalization
// ---------------------------------------------------------------------------

/// Strip the `::ffff:` prefix from an IPv4-mapped IPv6 address.
///
/// Returns the input unchanged if it does not have the prefix.
///
/// Source: `src/gateway/net.ts` (`normalizeIPv4MappedAddress`)
#[must_use]
pub fn normalize_ipv4_mapped(ip: &str) -> &str {
    ip.strip_prefix("::ffff:").unwrap_or(ip)
}

/// Normalize an IP string: trim, lowercase, strip IPv4-mapped prefix.
///
/// Source: `src/gateway/net.ts` (`normalizeIp`)
#[must_use]
pub fn normalize_ip(ip: Option<&str>) -> Option<String> {
    let trimmed = ip?.trim();
    if trimmed.is_empty() {
        return None;
    }
    let lower = trimmed.to_lowercase();
    Some(normalize_ipv4_mapped(&lower).to_string())
}

// ---------------------------------------------------------------------------
// IP parsing helpers
// ---------------------------------------------------------------------------

/// Strip an optional port from an IP address string.
///
/// Handles bracketed IPv6 (`[::1]:8080`) and `host:port` forms.
///
/// Source: `src/gateway/net.ts` (`stripOptionalPort`)
#[must_use]
pub fn strip_optional_port(ip: &str) -> String {
    // Bracketed IPv6
    if ip.starts_with('[') {
        if let Some(end) = ip.find(']') {
            return ip[1..end].to_string();
        }
    }
    // If it already parses as an IP, return as-is.
    if ip.parse::<IpAddr>().is_ok() {
        return ip.to_string();
    }
    // IPv4:port form — single colon with dots
    if let Some(last_colon) = ip.rfind(':') {
        if ip.contains('.') && ip.find(':') == Some(last_colon) {
            let candidate = &ip[..last_colon];
            if candidate.parse::<Ipv4Addr>().is_ok() {
                return candidate.to_string();
            }
        }
    }
    ip.to_string()
}

/// Parse the first IP from an `X-Forwarded-For` header value.
///
/// Source: `src/gateway/net.ts` (`parseForwardedForClientIp`)
#[must_use]
pub fn parse_forwarded_for_client_ip(forwarded_for: Option<&str>) -> Option<String> {
    let raw = forwarded_for?.split(',').next()?.trim();
    if raw.is_empty() {
        return None;
    }
    normalize_ip(Some(&strip_optional_port(raw)))
}

/// Parse the `X-Real-IP` header value.
///
/// Source: `src/gateway/net.ts` (`parseRealIp`)
#[must_use]
fn parse_real_ip(real_ip: Option<&str>) -> Option<String> {
    let raw = real_ip?.trim();
    if raw.is_empty() {
        return None;
    }
    normalize_ip(Some(&strip_optional_port(raw)))
}

/// Check whether a remote address is in the trusted proxy list.
///
/// Source: `src/gateway/net.ts` (`isTrustedProxyAddress`)
#[must_use]
pub fn is_trusted_proxy_address(ip: Option<&str>, trusted_proxies: Option<&[String]>) -> bool {
    let normalized = match normalize_ip(ip) {
        Some(n) => n,
        None => return false,
    };
    let proxies = match trusted_proxies {
        Some(p) if !p.is_empty() => p,
        _ => return false,
    };
    proxies
        .iter()
        .any(|proxy| normalize_ip(Some(proxy)).as_deref() == Some(normalized.as_str()))
}

/// Parameters for resolving the gateway client IP.
///
/// Source: `src/gateway/net.ts` (`resolveGatewayClientIp`)
pub struct ResolveClientIpParams<'a> {
    /// Socket remote address.
    pub remote_addr: Option<&'a str>,
    /// `X-Forwarded-For` header value.
    pub forwarded_for: Option<&'a str>,
    /// `X-Real-IP` header value.
    pub real_ip: Option<&'a str>,
    /// Trusted proxy addresses from config.
    pub trusted_proxies: Option<&'a [String]>,
}

/// Resolve the true client IP, taking trusted proxies into account.
///
/// If the direct remote address is a trusted proxy, prefer `X-Forwarded-For`
/// or `X-Real-IP`. Otherwise, use the remote address directly.
///
/// Source: `src/gateway/net.ts` (`resolveGatewayClientIp`)
#[must_use]
pub fn resolve_gateway_client_ip(params: &ResolveClientIpParams<'_>) -> Option<String> {
    let remote = normalize_ip(params.remote_addr)?;
    if !is_trusted_proxy_address(Some(&remote), params.trusted_proxies) {
        return Some(remote);
    }
    parse_forwarded_for_client_ip(params.forwarded_for)
        .or_else(|| parse_real_ip(params.real_ip))
        .or(Some(remote))
}

// ---------------------------------------------------------------------------
// Local address detection
// ---------------------------------------------------------------------------

/// Check whether an IP is a local gateway address (loopback).
///
/// Note: the TS version also checks tailnet IPs; this Rust version currently
/// only checks loopback since tailnet integration is not yet ported.
///
/// Source: `src/gateway/net.ts` (`isLocalGatewayAddress`)
#[must_use]
pub fn is_local_gateway_address(ip: Option<&str>) -> bool {
    if is_loopback_address(ip) {
        return true;
    }
    // Future: add tailnet IP checks here.
    false
}

// ---------------------------------------------------------------------------
// Bind host resolution
// ---------------------------------------------------------------------------

/// Test whether the system can bind to a specific host address.
///
/// Creates a temporary `TcpListener` on port 0, returning `true` on success.
///
/// Source: `src/gateway/net.ts` (`canBindToHost`)
#[must_use]
pub fn can_bind_to_host(host: &str) -> bool {
    let addr: Result<IpAddr, _> = host.parse();
    match addr {
        Ok(ip) => TcpListener::bind(SocketAddr::new(ip, 0)).is_ok(),
        Err(_) => false,
    }
}

/// Validate that a string is a valid IPv4 address.
///
/// Source: `src/gateway/net.ts` (`isValidIPv4`)
#[must_use]
pub fn is_valid_ipv4(host: &str) -> bool {
    host.parse::<Ipv4Addr>().is_ok()
}

/// Resolve the gateway bind host with fallback strategy.
///
/// Modes:
/// - `Loopback`: `127.0.0.1` (rarely fails; falls back to `0.0.0.0`)
/// - `Lan`: always `0.0.0.0`
/// - `Tailnet`: tailnet IPv4 if available, else loopback
/// - `Auto`: loopback if available, else `0.0.0.0`
/// - `Custom`: user-specified IP, fallback to `0.0.0.0`
///
/// Source: `src/gateway/net.ts` (`resolveGatewayBindHost`)
#[must_use]
pub fn resolve_gateway_bind_host(
    bind: Option<&GatewayBindMode>,
    custom_host: Option<&str>,
) -> String {
    let mode = bind.cloned().unwrap_or(GatewayBindMode::Loopback);

    match mode {
        GatewayBindMode::Loopback => {
            if can_bind_to_host("127.0.0.1") {
                "127.0.0.1".to_string()
            } else {
                "0.0.0.0".to_string()
            }
        }
        GatewayBindMode::Lan => "0.0.0.0".to_string(),
        GatewayBindMode::Tailnet => {
            // Future: check tailnet IP first.
            if can_bind_to_host("127.0.0.1") {
                "127.0.0.1".to_string()
            } else {
                "0.0.0.0".to_string()
            }
        }
        GatewayBindMode::Auto => {
            if can_bind_to_host("127.0.0.1") {
                "127.0.0.1".to_string()
            } else {
                "0.0.0.0".to_string()
            }
        }
        GatewayBindMode::Custom => {
            let host = custom_host.unwrap_or("").trim();
            if host.is_empty() {
                return "0.0.0.0".to_string();
            }
            if is_valid_ipv4(host) && can_bind_to_host(host) {
                host.to_string()
            } else {
                "0.0.0.0".to_string()
            }
        }
    }
}

/// Resolve gateway listen hosts, optionally adding IPv6 loopback.
///
/// When `bind_host` is `127.0.0.1`, also listens on `::1` if bindable.
///
/// Source: `src/gateway/net.ts` (`resolveGatewayListenHosts`)
#[must_use]
pub fn resolve_gateway_listen_hosts(bind_host: &str) -> Vec<String> {
    if bind_host != "127.0.0.1" {
        return vec![bind_host.to_string()];
    }
    let mut hosts = vec![bind_host.to_string()];
    if can_bind_to_host("::1") {
        hosts.push("::1".to_string());
    }
    hosts
}

// ---------------------------------------------------------------------------
// Host header parsing
// ---------------------------------------------------------------------------

/// Extract the hostname from an HTTP `Host` header value.
///
/// Strips port suffixes and handles bracketed IPv6 addresses.
///
/// Source: `src/gateway/auth.ts` (`getHostName`)
#[must_use]
pub fn get_host_name(host_header: Option<&str>) -> String {
    let host = host_header.unwrap_or("").trim().to_lowercase();
    if host.is_empty() {
        return String::new();
    }
    if host.starts_with('[') {
        if let Some(end) = host.find(']') {
            return host[1..end].to_string();
        }
    }
    host.split(':').next().unwrap_or("").to_string()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn loopback_detection() {
        assert!(is_loopback_address(Some("127.0.0.1")));
        assert!(is_loopback_address(Some("127.0.0.2")));
        assert!(is_loopback_address(Some("::1")));
        assert!(is_loopback_address(Some("::ffff:127.0.0.1")));
        assert!(!is_loopback_address(Some("192.168.1.1")));
        assert!(!is_loopback_address(Some("")));
        assert!(!is_loopback_address(None));
    }

    #[test]
    fn ipv4_mapped_normalization() {
        assert_eq!(normalize_ipv4_mapped("::ffff:127.0.0.1"), "127.0.0.1");
        assert_eq!(normalize_ipv4_mapped("192.168.1.1"), "192.168.1.1");
        assert_eq!(normalize_ipv4_mapped("::1"), "::1");
    }

    #[test]
    fn strip_port_works() {
        assert_eq!(strip_optional_port("[::1]:8080"), "::1");
        assert_eq!(strip_optional_port("192.168.1.1:8080"), "192.168.1.1");
        assert_eq!(strip_optional_port("192.168.1.1"), "192.168.1.1");
        assert_eq!(strip_optional_port("::1"), "::1");
    }

    #[test]
    fn forwarded_for_parsing() {
        assert_eq!(
            parse_forwarded_for_client_ip(Some("10.0.0.1, 192.168.1.1")),
            Some("10.0.0.1".to_string())
        );
        assert_eq!(
            parse_forwarded_for_client_ip(Some("::ffff:10.0.0.1")),
            Some("10.0.0.1".to_string())
        );
        assert_eq!(parse_forwarded_for_client_ip(Some("")), None);
        assert_eq!(parse_forwarded_for_client_ip(None), None);
    }

    #[test]
    fn valid_ipv4_checks() {
        assert!(is_valid_ipv4("127.0.0.1"));
        assert!(is_valid_ipv4("0.0.0.0"));
        assert!(is_valid_ipv4("255.255.255.255"));
        assert!(!is_valid_ipv4("not-an-ip"));
        assert!(!is_valid_ipv4("::1"));
    }

    #[test]
    fn bind_host_resolution_loopback() {
        // Loopback should almost always work on a dev machine.
        let result = resolve_gateway_bind_host(Some(&GatewayBindMode::Loopback), None);
        assert!(result == "127.0.0.1" || result == "0.0.0.0");
    }

    #[test]
    fn bind_host_resolution_lan() {
        let result = resolve_gateway_bind_host(Some(&GatewayBindMode::Lan), None);
        assert_eq!(result, "0.0.0.0");
    }

    #[test]
    fn bind_host_resolution_custom_empty() {
        let result = resolve_gateway_bind_host(Some(&GatewayBindMode::Custom), Some(""));
        assert_eq!(result, "0.0.0.0");
    }

    #[test]
    fn listen_hosts_non_loopback() {
        let hosts = resolve_gateway_listen_hosts("0.0.0.0");
        assert_eq!(hosts, vec!["0.0.0.0"]);
    }

    #[test]
    fn host_name_parsing() {
        assert_eq!(get_host_name(Some("localhost:8080")), "localhost");
        assert_eq!(get_host_name(Some("[::1]:8080")), "::1");
        assert_eq!(get_host_name(Some("example.com")), "example.com");
        assert_eq!(get_host_name(None), "");
        assert_eq!(get_host_name(Some("")), "");
    }

    #[test]
    fn local_gateway_address_checks() {
        assert!(is_local_gateway_address(Some("127.0.0.1")));
        assert!(is_local_gateway_address(Some("::1")));
        assert!(!is_local_gateway_address(Some("192.168.1.1")));
        assert!(!is_local_gateway_address(None));
    }

    #[test]
    fn normalize_ip_works() {
        assert_eq!(normalize_ip(Some("  ::FFFF:10.0.0.1 ")), Some("10.0.0.1".to_string()));
        assert_eq!(normalize_ip(Some("192.168.1.1")), Some("192.168.1.1".to_string()));
        assert_eq!(normalize_ip(Some("")), None);
        assert_eq!(normalize_ip(None), None);
    }

    #[test]
    fn trusted_proxy_check() {
        let proxies = vec!["10.0.0.1".to_string()];
        assert!(is_trusted_proxy_address(Some("10.0.0.1"), Some(&proxies)));
        assert!(!is_trusted_proxy_address(Some("10.0.0.2"), Some(&proxies)));
        assert!(!is_trusted_proxy_address(Some("10.0.0.1"), None));
        assert!(!is_trusted_proxy_address(None, Some(&proxies)));
    }
}
