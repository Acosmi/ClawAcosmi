/// System heartbeat commands — local heartbeat file operations.
use anyhow::Result;

use oa_config::paths::resolve_state_dir;

/// Show last heartbeat timestamp.
pub fn heartbeat_last_command(json: bool) -> Result<()> {
    let state_dir = resolve_state_dir();
    let hb_path = state_dir.join("heartbeat.json");

    if !hb_path.exists() {
        if json {
            println!(r#"{{"error": "no heartbeat file found"}}"#);
        } else {
            println!("💔 No heartbeat file found at {}", hb_path.display());
        }
        return Ok(());
    }

    let content = std::fs::read_to_string(&hb_path)?;
    if json {
        println!("{content}");
    } else {
        let data: serde_json::Value = serde_json::from_str(&content)?;
        let ts = data.get("timestamp").and_then(|v| v.as_u64()).unwrap_or(0);
        let ago = if ts > 0 {
            let now = std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .map(|d| d.as_millis() as u64)
                .unwrap_or(0);
            let diff_s = now.saturating_sub(ts) / 1000;
            format!("{diff_s}s ago")
        } else {
            "unknown".to_string()
        };
        println!("💓 Last heartbeat: {ago}");
    }

    Ok(())
}

/// Enable heartbeat service.
pub fn heartbeat_enable_command() -> Result<()> {
    let state_dir = resolve_state_dir();
    let flag_path = state_dir.join("heartbeat.enabled");
    std::fs::create_dir_all(&state_dir)?;
    std::fs::write(&flag_path, "1")?;
    println!("✅ Heartbeat enabled");
    Ok(())
}

/// Disable heartbeat service.
pub fn heartbeat_disable_command() -> Result<()> {
    let state_dir = resolve_state_dir();
    let flag_path = state_dir.join("heartbeat.enabled");
    if flag_path.exists() {
        std::fs::remove_file(&flag_path)?;
    }
    println!("❌ Heartbeat disabled");
    Ok(())
}
