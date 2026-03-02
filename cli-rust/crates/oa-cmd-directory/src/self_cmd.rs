/// Directory self command.
use anyhow::Result;

pub fn directory_self_command(channel: &str, json: bool) -> Result<()> {
    if json {
        println!(r#"{{"channel":"{channel}","status":"not_implemented"}}"#);
    } else {
        println!("👤 Directory self ({channel}) not yet implemented");
    }
    Ok(())
}
