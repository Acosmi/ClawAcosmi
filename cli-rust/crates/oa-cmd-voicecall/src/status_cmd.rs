/// Voicecall status command.
use anyhow::Result;

pub fn voicecall_status_command(call_id: &str, json: bool) -> Result<()> {
    if json {
        println!(r#"{{"call_id":"{call_id}","status":"not_implemented"}}"#);
    } else {
        println!("📊 Voicecall status call_id='{call_id}' not yet implemented");
    }
    Ok(())
}
