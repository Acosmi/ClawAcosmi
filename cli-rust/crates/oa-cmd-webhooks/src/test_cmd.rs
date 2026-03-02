/// Webhooks test command.
use anyhow::Result;

pub fn webhooks_test_command(url: Option<&str>) -> Result<()> {
    let u = url.unwrap_or("(none)");
    println!("🧪 Webhooks test (url={u}) not yet implemented");
    Ok(())
}
