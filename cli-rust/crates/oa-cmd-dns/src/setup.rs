/// DNS setup command — perform DNS lookups and network configuration.
use anyhow::Result;

/// DNS setup: verify DNS resolution and report network configuration.
pub async fn dns_setup_command(apply: bool) -> Result<()> {
    use tokio::net::lookup_host;

    println!("🌐 DNS Configuration Check:");

    // Test several important hostnames
    let hosts = ["openacosmi.local", "localhost", "api.openai.com"];
    for host in &hosts {
        let lookup_target = format!("{host}:443");
        match lookup_host(&lookup_target).await {
            Ok(addrs) => {
                let addrs: Vec<_> = addrs.map(|a| a.ip().to_string()).collect();
                println!("  ✅ {host} → {}", addrs.join(", "));
            }
            Err(e) => {
                println!("  ❌ {host} → {e}");
            }
        }
    }

    // Detect local IP
    if let Some(ip) = oa_gateway_rpc::net::pick_primary_lan_ipv4() {
        println!("\n📡 Local LAN IP: {ip}");
    }

    if apply {
        println!("\n⚙️  Apply mode: DNS configuration would be persisted.");
    }

    Ok(())
}
