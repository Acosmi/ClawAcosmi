/// Webhooks list command.
use anyhow::Result;

pub fn webhooks_list_command(json: bool) -> Result<()> {
    if json {
        println!(r#"{{"webhooks":[],"status":"not_implemented"}}"#);
    } else {
        println!("📋 Webhooks list not yet implemented");
    }
    Ok(())
}
