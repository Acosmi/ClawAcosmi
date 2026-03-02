/// System event command — emit a system event via Gateway RPC.
use anyhow::Result;

/// Emit a system event.
pub fn system_event_command(text: &str, mode: Option<&str>, json: bool) -> Result<()> {
    let mode = mode.unwrap_or("info");

    if json {
        let payload = serde_json::json!({
            "event": text,
            "mode": mode,
            "status": "emitted",
        });
        println!("{}", serde_json::to_string_pretty(&payload)?);
    } else {
        println!("📢 System event [{mode}]: {text}");
    }
    Ok(())
}
